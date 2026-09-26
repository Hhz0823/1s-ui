package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util"
	"github.com/Hhz0823/1s-ui/util/common"
)

// The SD-WAN uplink is a dedicated inbound on a managed server that only the
// controller uses. End users keep connecting to the controller, which picks the
// fastest uplink; they never receive these credentials.
const (
	sdwanUplinkTag           = "sdwan-uplink"
	sdwanUplinkClient        = "sdwan-uplink"
	SdwanProtocolShadowsocks = "shadowsocks"
	SdwanProtocolHysteria2   = "hysteria2"
	sdwanShadowsocksMethod   = "2022-blake3-aes-128-gcm"
)

var sdwanUplinkMu sync.Mutex

type SdwanProvisionRequest struct {
	Protocol   string `json:"protocol"`
	Port       int    `json:"port,omitempty"`
	Rotate     bool   `json:"rotate,omitempty"`
	Actor      string `json:"actor"`
	PublicHost string `json:"public_host"`
}

type SdwanProvisionResponse struct {
	Tag      string          `json:"tag"`
	Protocol string          `json:"protocol"`
	Server   string          `json:"server"`
	Port     int             `json:"port"`
	Outbound json.RawMessage `json:"outbound"`
	Revision uint64          `json:"revision"`
}

type SdwanRemoveRequest struct {
	Actor      string `json:"actor"`
	PublicHost string `json:"public_host"`
}

type SdwanRemoveResponse struct {
	Removed  bool   `json:"removed"`
	Revision uint64 `json:"revision"`
}

func normalizeSdwanProtocol(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "shadowsocks", "ss", "ss2022":
		return SdwanProtocolShadowsocks, nil
	case "hysteria2", "hy2":
		return SdwanProtocolHysteria2, nil
	}
	return "", common.NewErrorf("unsupported SD-WAN uplink protocol %q", value)
}

// ProvisionSdwanUplink creates (or returns the existing) SD-WAN uplink and the
// sing-box outbound the controller needs to reach it. It is idempotent: the
// same uplink is reused unless the protocol changes or a rotation is asked for.
func (s *LocalControlService) ProvisionSdwanUplink(request SdwanProvisionRequest) (*SdwanProvisionResponse, error) {
	protocol, err := normalizeSdwanProtocol(request.Protocol)
	if err != nil {
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
		return nil, common.NewError("managed server public host is required for SD-WAN")
	}
	if request.Port < 0 || request.Port > 65535 {
		return nil, common.NewError("SD-WAN uplink port must be between 1 and 65535")
	}

	sdwanUplinkMu.Lock()
	defer sdwanUplinkMu.Unlock()

	revision, err := s.ConfigService.CurrentRevision()
	if err != nil {
		return nil, err
	}
	existing, client, err := findSdwanUplink()
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.Type == protocol && !request.Rotate {
		if outbound, port, buildErr := sdwanOutboundFromUplink(existing, client, publicHost); buildErr == nil {
			return &SdwanProvisionResponse{
				Tag: sdwanUplinkTag, Protocol: protocol, Server: publicHost, Port: port,
				Outbound: outbound, Revision: revision,
			}, nil
		}
	}
	if existing != nil || client != nil {
		if revision, err = s.removeSdwanUplinkLocked(existing, client, revision, actor, publicHost); err != nil {
			return nil, err
		}
	}
	return s.createSdwanUplink(protocol, request.Port, revision, actor, publicHost)
}

// RemoveSdwanUplink deletes the uplink inbound, its internal user and the
// certificate generated for it.
func (s *LocalControlService) RemoveSdwanUplink(request SdwanRemoveRequest) (*SdwanRemoveResponse, error) {
	actor, err := normalizeRemoteActor(request.Actor)
	if err != nil {
		return nil, err
	}
	publicHost, err := normalizeAgentPublicHost(request.PublicHost)
	if err != nil {
		return nil, err
	}
	sdwanUplinkMu.Lock()
	defer sdwanUplinkMu.Unlock()
	revision, err := s.ConfigService.CurrentRevision()
	if err != nil {
		return nil, err
	}
	existing, client, err := findSdwanUplink()
	if err != nil {
		return nil, err
	}
	if existing == nil && client == nil {
		return &SdwanRemoveResponse{Removed: false, Revision: revision}, nil
	}
	revision, err = s.removeSdwanUplinkLocked(existing, client, revision, actor, publicHost)
	if err != nil {
		return nil, err
	}
	return &SdwanRemoveResponse{Removed: true, Revision: revision}, nil
}

func findSdwanUplink() (*model.Inbound, *model.Client, error) {
	db := database.GetDB()
	var inbounds []model.Inbound
	if err := db.Preload("Tls").Where("tag = ?", sdwanUplinkTag).Limit(1).Find(&inbounds).Error; err != nil {
		return nil, nil, err
	}
	var clients []model.Client
	if err := db.Where("name = ?", sdwanUplinkClient).Limit(1).Find(&clients).Error; err != nil {
		return nil, nil, err
	}
	var inbound *model.Inbound
	if len(inbounds) > 0 {
		inbound = &inbounds[0]
	}
	var client *model.Client
	if len(clients) > 0 {
		client = &clients[0]
	}
	return inbound, client, nil
}

func (s *LocalControlService) removeSdwanUplinkLocked(inbound *model.Inbound, client *model.Client, revision uint64, actor, publicHost string) (uint64, error) {
	var err error
	if inbound != nil {
		tag, _ := json.Marshal(inbound.Tag)
		if _, revision, err = s.ConfigService.SaveWithRevision(revision, "inbounds", "del", tag, "", "agent:"+actor, publicHost); err != nil {
			return revision, err
		}
		if inbound.TlsId > 0 && inbound.Tls != nil && strings.HasPrefix(inbound.Tls.Name, generatedTLSNamePrefix) {
			var users int64
			if err = database.GetDB().Model(&model.Inbound{}).Where("tls_id = ?", inbound.TlsId).Count(&users).Error; err != nil {
				return revision, err
			}
			if users == 0 {
				id, _ := json.Marshal(inbound.TlsId)
				if _, revision, err = s.ConfigService.SaveWithRevision(revision, "tls", "del", id, "", "agent:"+actor, publicHost); err != nil {
					return revision, err
				}
			}
		}
	}
	if client != nil {
		id, _ := json.Marshal(client.Id)
		if _, revision, err = s.ConfigService.SaveWithRevision(revision, "clients", "del", id, "", "agent:"+actor, publicHost); err != nil {
			return revision, err
		}
	}
	return revision, nil
}

func (s *LocalControlService) createSdwanUplink(protocol string, preferredPort int, revision uint64, actor, publicHost string) (*SdwanProvisionResponse, error) {
	inbounds, err := s.InboundService.Get("")
	if err != nil {
		return nil, err
	}
	items := []map[string]interface{}{}
	if inbounds != nil {
		items = *inbounds
	}
	if preferredPort == 0 {
		preferredPort = 20000 + common.RandomInt(30000)
	}
	ports, _, err := allocateRemoteQuickAdd(items, preferredPort, 1, sdwanUplinkTag, protocol)
	if err != nil {
		return nil, err
	}
	port := ports[0]

	inbound := map[string]interface{}{
		"id": 0, "core_type": model.CoreTypeSingBox, "type": protocol, "tag": sdwanUplinkTag,
		"listen": quickAddListenAddress(publicHost), "listen_port": port,
		"addrs": []interface{}{}, "out_json": map[string]interface{}{},
	}
	clientConfig := map[string]interface{}{}
	switch protocol {
	case SdwanProtocolShadowsocks:
		inbound["method"] = sdwanShadowsocksMethod
		inbound["password"] = relayShadowsocksKey(sdwanShadowsocksMethod)
		clientConfig["shadowsocks16"] = map[string]interface{}{
			"name": sdwanUplinkClient, "password": relayShadowsocksKey(sdwanShadowsocksMethod),
		}
	case SdwanProtocolHysteria2:
		tlsID, nextRevision, tlsErr := s.createRemoteQuickAddTLS(strings.Trim(publicHost, "[]"), revision, actor, publicHost)
		if tlsErr != nil {
			return nil, tlsErr
		}
		revision = nextRevision
		inbound["tls_id"] = tlsID
		inbound["obfs"] = map[string]interface{}{
			"type": "salamander", "password": base64.StdEncoding.EncodeToString([]byte(common.Random(16))),
		}
		clientConfig["hysteria2"] = map[string]interface{}{"name": sdwanUplinkClient, "password": common.Random(24)}
	}

	client := map[string]interface{}{
		"id": 0, "enable": true, "name": sdwanUplinkClient, "config": clientConfig,
		"inbounds": []uint{}, "links": []interface{}{}, "volume": 0, "expiry": 0,
		"up": 0, "down": 0, "desc": "SD-WAN uplink used by the controller", "group": "sdwan", "remark": "",
	}
	rawClient, err := json.Marshal(client)
	if err != nil {
		return nil, err
	}
	if _, revision, err = s.ConfigService.SaveWithRevision(revision, "clients", "new", rawClient, "", "agent:"+actor, publicHost); err != nil {
		return nil, err
	}
	var savedClient model.Client
	if err = database.GetDB().Where("name = ?", sdwanUplinkClient).Order("id desc").First(&savedClient).Error; err != nil {
		return nil, err
	}
	rawInbound, err := json.Marshal(inbound)
	if err != nil {
		return nil, err
	}
	if _, revision, err = s.ConfigService.SaveWithRevision(
		revision, "inbounds", "new", rawInbound, fmt.Sprintf("%d", savedClient.Id), "agent:"+actor, publicHost,
	); err != nil {
		return nil, err
	}
	created, savedUplinkClient, err := findSdwanUplink()
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, common.NewError("SD-WAN uplink was not saved")
	}
	outbound, port, err := sdwanOutboundFromUplink(created, savedUplinkClient, publicHost)
	if err != nil {
		return nil, err
	}
	return &SdwanProvisionResponse{
		Tag: sdwanUplinkTag, Protocol: protocol, Server: publicHost, Port: port,
		Outbound: outbound, Revision: revision,
	}, nil
}

// sdwanOutboundFromUplink rebuilds the controller-side sing-box outbound from
// the stored uplink, so repeated provisioning returns the same credentials.
func sdwanOutboundFromUplink(inbound *model.Inbound, client *model.Client, publicHost string) (json.RawMessage, int, error) {
	if inbound == nil || client == nil {
		return nil, 0, common.NewError("SD-WAN uplink is incomplete")
	}
	options := map[string]interface{}{}
	if err := json.Unmarshal(inbound.Options, &options); err != nil {
		return nil, 0, err
	}
	port := intFromInterface(options["listen_port"])
	if port < 1 || port > 65535 {
		return nil, 0, common.NewError("SD-WAN uplink has no valid port")
	}
	var clientConfig map[string]map[string]interface{}
	if err := json.Unmarshal(client.Config, &clientConfig); err != nil {
		return nil, 0, err
	}
	server := strings.Trim(publicHost, "[]")
	outbound := map[string]interface{}{"type": inbound.Type, "server": server, "server_port": port}
	switch inbound.Type {
	case SdwanProtocolShadowsocks:
		method, _ := options["method"].(string)
		serverKey, _ := options["password"].(string)
		userKey, _ := clientConfig["shadowsocks16"]["password"].(string)
		if method != sdwanShadowsocksMethod || serverKey == "" || userKey == "" {
			return nil, 0, common.NewError("SD-WAN Shadowsocks uplink is incomplete")
		}
		outbound["method"] = method
		outbound["password"] = serverKey + ":" + userKey
	case SdwanProtocolHysteria2:
		password, _ := clientConfig["hysteria2"]["password"].(string)
		if password == "" || inbound.Tls == nil {
			return nil, 0, common.NewError("SD-WAN Hysteria2 uplink is incomplete")
		}
		var serverTLS map[string]interface{}
		if err := json.Unmarshal(inbound.Tls.Server, &serverTLS); err != nil {
			return nil, 0, err
		}
		certificate := util.CertPEMFromTLS(serverTLS)
		if certificate == "" {
			return nil, 0, common.NewError("SD-WAN Hysteria2 uplink has no certificate")
		}
		serverName, _ := serverTLS["server_name"].(string)
		if serverName == "" {
			serverName = server
		}
		outbound["password"] = password
		if obfs, ok := options["obfs"].(map[string]interface{}); ok {
			outbound["obfs"] = obfs
		}
		// The controller trusts exactly this certificate instead of skipping
		// verification, because both ends are managed by the same panel.
		outbound["tls"] = map[string]interface{}{
			"enabled":     true,
			"server_name": serverName,
			"certificate": strings.Split(strings.TrimSpace(certificate), "\n"),
		}
	default:
		return nil, 0, common.NewErrorf("unsupported SD-WAN uplink type %q", inbound.Type)
	}
	raw, err := json.Marshal(outbound)
	return raw, port, err
}
