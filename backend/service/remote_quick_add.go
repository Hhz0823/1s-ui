package service

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"strings"
	"time"
	"unicode"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util/common"
	boxtls "github.com/sagernet/sing-box/common/tls"
)

const maxRemoteQuickAddCount = 100

var remoteQuickAddProtocols = map[string]map[string]bool{
	model.CoreTypeSingBox: {
		"mixed": true, "socks": true, "http": true, "shadowsocks": true,
		"vmess": true, "trojan": true, "vless": true, "hysteria2": true,
		"shadowtls": true, "tuic": true, "naive": true, "anytls": true,
		"direct": true,
	},
	model.CoreTypeXray: {
		"vless": true, "vmess": true, "trojan": true, "shadowsocks": true,
		"socks": true, "http": true, "mixed": true, "hysteria2": true,
		"dokodemo-door": true,
	},
}

var remoteQuickAddTLSProtocols = map[string]bool{
	"vmess": true, "vless": true, "trojan": true, "hysteria2": true,
	"tuic": true, "naive": true, "anytls": true,
}

var remoteQuickAddClientProtocols = map[string]bool{
	"shadowsocks": true, "vmess": true, "vless": true, "trojan": true,
	"hysteria2": true, "shadowtls": true, "tuic": true, "naive": true,
	"anytls": true,
}

type RemoteQuickAddRequest struct {
	CoreType         string `json:"core_type"`
	Protocol         string `json:"protocol"`
	Tag              string `json:"tag"`
	Count            int    `json:"count"`
	Port             int    `json:"port"`
	Password         string `json:"password"`
	Method           string `json:"method"`
	ObfsPassword     string `json:"obfs_password"`
	HandshakeServer  string `json:"handshake_server"`
	ExpectedRevision uint64 `json:"expected_revision"`
	Actor            string `json:"actor"`
	PublicHost       string `json:"public_host"`
}

type RemoteQuickAddItem struct {
	ID         uint   `json:"id"`
	Tag        string `json:"tag"`
	Port       int    `json:"port"`
	ClientID   uint   `json:"client_id,omitempty"`
	ClientName string `json:"client_name,omitempty"`
}

type RemoteQuickAddResponse struct {
	Revision uint64               `json:"revision"`
	Created  []RemoteQuickAddItem `json:"created"`
}

func (s *LocalControlService) QuickAddInbounds(request RemoteQuickAddRequest) (*RemoteQuickAddResponse, error) {
	if err := validateRemoteQuickAddRequest(&request); err != nil {
		return nil, err
	}
	actor, err := normalizeRemoteActor(request.Actor)
	if err != nil {
		return nil, err
	}
	publicHost, err := normalizeAgentPublicHost(request.PublicHost)
	if err != nil {
		return nil, err
	}
	if publicHost == "" {
		return nil, common.NewError("managed server public host is required before quick add")
	}
	if request.CoreType == model.CoreTypeXray {
		check := s.ConfigService.CheckXray()
		if check.Disabled {
			return nil, common.NewError("Xray-core is disabled on this managed server; use sing-box")
		}
		if !check.BinaryAvailable {
			return nil, common.NewError("Xray-core is not installed on this managed server")
		}
	}

	inbounds, err := s.InboundService.Get("")
	if err != nil {
		return nil, err
	}
	items := []map[string]interface{}{}
	if inbounds != nil {
		items = *inbounds
	}
	ports, tags, err := allocateRemoteQuickAdd(items, request.Port, request.Count, request.Tag, request.Protocol)
	if err != nil {
		return nil, err
	}

	revision := request.ExpectedRevision
	tlsID := uint(0)
	if remoteQuickAddTLSProtocols[request.Protocol] {
		tlsID, revision, err = s.createRemoteQuickAddTLS(tags[0], revision, actor, publicHost)
		if err != nil {
			return nil, err
		}
	}

	created := make([]RemoteQuickAddItem, 0, request.Count)
	for index := 0; index < request.Count; index++ {
		password := quickAddPassword(request, index)
		clientID := uint(0)
		clientName := ""
		if remoteQuickAddClientProtocols[request.Protocol] {
			clientName = "user-" + common.Random(8)
			clientID, revision, err = s.createRemoteQuickAddClient(request, clientName, password, revision, actor, publicHost)
			if err != nil {
				return nil, common.NewErrorf("quick add stopped after %d/%d nodes: %v", len(created), request.Count, err)
			}
		}

		inbound := buildRemoteQuickAddInbound(request, tags[index], ports[index], password, tlsID, publicHost)
		rawInbound, marshalErr := json.Marshal(inbound)
		if marshalErr != nil {
			return nil, marshalErr
		}
		initUsers := ""
		if clientID > 0 {
			initUsers = fmt.Sprintf("%d", clientID)
		}
		_, revision, err = s.ConfigService.SaveWithRevision(
			revision, "inbounds", "new", rawInbound, initUsers,
			"agent:"+actor, publicHost,
		)
		if err != nil {
			return nil, common.NewErrorf("quick add stopped after %d/%d nodes: %v", len(created), request.Count, err)
		}
		var saved model.Inbound
		if err = database.GetDB().Where("tag = ?", tags[index]).First(&saved).Error; err != nil {
			return nil, err
		}
		created = append(created, RemoteQuickAddItem{
			ID: saved.Id, Tag: tags[index], Port: ports[index],
			ClientID: clientID, ClientName: clientName,
		})
	}
	return &RemoteQuickAddResponse{Revision: revision, Created: created}, nil
}

func validateRemoteQuickAddRequest(request *RemoteQuickAddRequest) error {
	request.CoreType = strings.TrimSpace(request.CoreType)
	if request.CoreType == "" {
		request.CoreType = model.CoreTypeSingBox
	}
	request.Protocol = strings.ToLower(strings.TrimSpace(request.Protocol))
	if !remoteQuickAddProtocols[request.CoreType][request.Protocol] {
		return common.NewErrorf("protocol %q is not supported by %s", request.Protocol, request.CoreType)
	}
	if request.Count < 1 || request.Count > maxRemoteQuickAddCount {
		return common.NewErrorf("quick add count must be between 1 and %d", maxRemoteQuickAddCount)
	}
	if request.Port < 1 || request.Port > 65535 {
		return common.NewError("quick add port must be between 1 and 65535")
	}
	request.Tag = strings.TrimSpace(request.Tag)
	if len([]rune(request.Tag)) > 100 {
		return common.NewError("quick add tag is too long")
	}
	for _, value := range request.Tag {
		if unicode.IsControl(value) {
			return common.NewError("quick add tag contains control characters")
		}
	}
	request.Method = strings.TrimSpace(request.Method)
	if request.Method == "" {
		request.Method = "2022-blake3-aes-256-gcm"
	}
	if request.CoreType == model.CoreTypeXray && request.Protocol == "shadowsocks" {
		request.Method = "2022-blake3-aes-256-gcm"
	}
	if request.Protocol == "shadowsocks" && !relayShadowsocksMethods[request.Method] {
		return common.NewErrorf("unsupported Shadowsocks method %q", request.Method)
	}
	return nil
}

func normalizeRemoteActor(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "remote-panel"
	}
	if len([]rune(value)) > 100 {
		return "", common.NewError("actor is too long")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", common.NewError("actor contains control characters")
		}
	}
	return value, nil
}

func allocateRemoteQuickAdd(inbounds []map[string]interface{}, start, count int, baseTag, protocol string) ([]int, []string, error) {
	usedPorts := make(map[int]bool, len(inbounds))
	usedTags := make(map[string]bool, len(inbounds))
	for _, inbound := range inbounds {
		if port := intFromInterface(inbound["listen_port"]); port > 0 {
			usedPorts[port] = true
		}
		if tag, ok := inbound["tag"].(string); ok && tag != "" {
			usedTags[tag] = true
		}
	}
	ports := make([]int, 0, count)
	candidate := start
	for len(ports) < count {
		attempts := 0
		for usedPorts[candidate] && attempts < 65535 {
			candidate++
			if candidate > 65535 {
				candidate = 1
			}
			attempts++
		}
		if attempts >= 65535 {
			return nil, nil, common.NewError("no available ports for quick add")
		}
		ports = append(ports, candidate)
		usedPorts[candidate] = true
		candidate++
		if candidate > 65535 {
			candidate = 1
		}
	}
	if baseTag == "" {
		baseTag = fmt.Sprintf("%s-%d", protocol, ports[0])
	}
	tags := make([]string, 0, count)
	for index := 0; index < count; index++ {
		initial := baseTag
		if count > 1 {
			initial = fmt.Sprintf("%s-%d", baseTag, index+1)
		}
		tag := initial
		for copyIndex := 1; usedTags[tag]; copyIndex++ {
			tag = fmt.Sprintf("%s-copy%d", initial, copyIndex)
		}
		usedTags[tag] = true
		tags = append(tags, tag)
	}
	return ports, tags, nil
}

func intFromInterface(value interface{}) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case uint:
		return int(typed)
	case uint64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		value, _ := typed.Int64()
		return int(value)
	default:
		return 0
	}
}

func quickAddPassword(request RemoteQuickAddRequest, index int) string {
	if index == 0 && strings.TrimSpace(request.Password) != "" {
		return request.Password
	}
	if request.Protocol == "shadowsocks" {
		return relayShadowsocksKey(request.Method)
	}
	return common.Random(20)
}

func quickAddListenAddress(publicHost string) string {
	host := strings.Trim(strings.TrimSpace(publicHost), "[]")
	if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
		return "::"
	}
	return "0.0.0.0"
}

func buildRemoteQuickAddInbound(request RemoteQuickAddRequest, tag string, port int, password string, tlsID uint, publicHost string) map[string]interface{} {
	inbound := map[string]interface{}{
		"id": 0, "core_type": request.CoreType, "type": request.Protocol,
		"tag": tag, "listen": quickAddListenAddress(publicHost), "listen_port": port, "tls_id": tlsID,
		"addrs": []interface{}{}, "out_json": map[string]interface{}{},
	}
	isXray := request.CoreType == model.CoreTypeXray
	switch request.Protocol {
	case "shadowsocks":
		method := request.Method
		if isXray {
			method = "2022-blake3-aes-256-gcm"
		}
		inbound["method"] = method
		inbound["password"] = password
	case "vmess":
		if isXray {
			inbound["transport"] = map[string]interface{}{"type": "ws", "path": "/", "host": publicHost}
		} else {
			inbound["transport"] = map[string]interface{}{"type": "http"}
		}
	case "vless":
		if isXray {
			inbound["transport"] = map[string]interface{}{"type": "xhttp", "path": "/xhttp", "host": publicHost, "mode": "auto"}
		} else {
			inbound["transport"] = map[string]interface{}{}
		}
	case "trojan":
		if isXray {
			inbound["transport"] = map[string]interface{}{"type": "ws", "path": "/", "host": publicHost}
		} else {
			inbound["transport"] = map[string]interface{}{}
		}
	case "hysteria2":
		if isXray {
			inbound["transport"] = map[string]interface{}{"type": "hysteria", "udp_idle_timeout": 60}
		} else {
			obfsPassword := strings.TrimSpace(request.ObfsPassword)
			if obfsPassword == "" {
				obfsPassword = base64.StdEncoding.EncodeToString([]byte(common.Random(16)))
			}
			inbound["obfs"] = map[string]interface{}{"type": "salamander", "password": obfsPassword}
		}
	case "shadowtls":
		handshake := strings.TrimSpace(request.HandshakeServer)
		if handshake == "" {
			handshake = "www.microsoft.com"
		}
		inbound["version"] = 3
		inbound["password"] = password
		inbound["handshake"] = map[string]interface{}{"server": handshake, "server_port": 443}
	case "tuic":
		inbound["congestion_control"] = "cubic"
	case "anytls":
		inbound["padding_scheme"] = []string{
			"stop=8", "0=30-30", "1=100-400",
			"2=400-500,c,500-1000,c,500-1000,c,500-1000,c,500-1000",
			"3=9-9,500-1000", "4=500-1000", "5=500-1000", "6=500-1000", "7=500-1000",
		}
	case "dokodemo-door":
		inbound["network"] = "tcp,udp"
		inbound["follow_redirect"] = true
		inbound["sniffing"] = map[string]interface{}{
			"enabled": true, "destOverride": []string{"http", "tls", "quic"}, "routeOnly": true,
		}
	}
	return inbound
}

func (s *LocalControlService) createRemoteQuickAddClient(request RemoteQuickAddRequest, name, password string, revision uint64, actor, publicHost string) (uint, uint64, error) {
	uuidValue := randomUUID()
	configValue := map[string]interface{}{}
	switch request.Protocol {
	case "shadowsocks":
		configValue["shadowsocks"] = map[string]interface{}{"name": name, "password": password}
	case "vmess":
		configValue["vmess"] = map[string]interface{}{"name": name, "uuid": uuidValue, "alterId": 0}
	case "vless":
		flow := "xtls-rprx-vision"
		if request.CoreType == model.CoreTypeXray {
			flow = ""
		}
		configValue["vless"] = map[string]interface{}{"name": name, "uuid": uuidValue, "flow": flow}
	case "trojan":
		configValue["trojan"] = map[string]interface{}{"name": name, "password": password}
	case "hysteria2":
		configValue["hysteria2"] = map[string]interface{}{"name": name, "password": password}
	case "shadowtls":
		configValue["shadowtls"] = map[string]interface{}{"name": name, "password": password}
	case "tuic":
		configValue["tuic"] = map[string]interface{}{"name": name, "uuid": uuidValue, "password": password}
	case "naive":
		configValue["naive"] = map[string]interface{}{"username": name, "password": password}
	case "anytls":
		configValue["anytls"] = map[string]interface{}{"name": name, "password": password}
	default:
		return 0, revision, common.NewErrorf("protocol %q does not use clients", request.Protocol)
	}
	client := map[string]interface{}{
		"id": 0, "enable": true, "name": name, "config": configValue,
		"inbounds": []uint{}, "links": []interface{}{}, "volume": 0, "expiry": 0,
		"up": 0, "down": 0, "desc": "", "group": "", "remark": "",
	}
	raw, err := json.Marshal(client)
	if err != nil {
		return 0, revision, err
	}
	_, revision, err = s.ConfigService.SaveWithRevision(
		revision, "clients", "new", raw, "", "agent:"+actor, publicHost,
	)
	if err != nil {
		return 0, revision, err
	}
	var saved model.Client
	if err = database.GetDB().Where("name = ?", name).Order("id desc").First(&saved).Error; err != nil {
		return 0, revision, err
	}
	return saved.Id, revision, nil
}

func (s *LocalControlService) createRemoteQuickAddTLS(serverName string, revision uint64, actor, publicHost string) (uint, uint64, error) {
	cleanName := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' {
			return r
		}
		return '-'
	}, serverName)
	if cleanName == "" {
		cleanName = "managed-node"
	}
	privateKey, certificate, err := boxtls.GenerateCertificate(nil, nil, time.Now, cleanName, time.Now().AddDate(0, 12, 0))
	if err != nil {
		return 0, revision, err
	}
	block, _ := pem.Decode(certificate)
	if block == nil {
		return 0, revision, common.NewError("generated TLS certificate is invalid")
	}
	parsed, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return 0, revision, err
	}
	certificateHash := sha256.Sum256(parsed.Raw)
	name := "auto-" + serverName
	for copyIndex := 0; ; copyIndex++ {
		candidate := name
		if copyIndex > 0 {
			candidate = fmt.Sprintf("%s-copy%d", name, copyIndex)
		}
		var count int64
		if err = database.GetDB().Model(&model.Tls{}).Where("name = ?", candidate).Count(&count).Error; err != nil {
			return 0, revision, err
		}
		if count == 0 {
			name = candidate
			break
		}
	}
	tlsConfig := map[string]interface{}{
		"id":   0,
		"name": name,
		"server": map[string]interface{}{
			"enabled":     true,
			"key":         strings.Split(strings.TrimSpace(string(privateKey)), "\n"),
			"certificate": strings.Split(strings.TrimSpace(string(certificate)), "\n"),
		},
		"client": map[string]interface{}{
			"pinned_peer_certificate_sha256": []string{base64.StdEncoding.EncodeToString(certificateHash[:])},
		},
	}
	raw, err := json.Marshal(tlsConfig)
	if err != nil {
		return 0, revision, err
	}
	_, revision, err = s.ConfigService.SaveWithRevision(
		revision, "tls", "new", raw, "", "agent:"+actor, publicHost,
	)
	if err != nil {
		return 0, revision, err
	}
	var saved model.Tls
	if err = database.GetDB().Where("name = ?", name).First(&saved).Error; err != nil {
		return 0, revision, err
	}
	return saved.Id, revision, nil
}
