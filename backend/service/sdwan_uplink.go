package service

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util"
	"github.com/Hhz0823/1s-ui/util/common"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// SD-WAN uplinks are dedicated inbounds on a managed server that only the
// controller uses. End users keep connecting to the controller, which picks the
// best uplink; they never receive these credentials.
//
//   - reality:     VLESS + XTLS Vision over TCP, authenticated by an x25519 key
//     (no certificate to trust) and indistinguishable from a real TLS 1.3 site.
//   - hysteria2:   QUIC with BBR and Salamander obfuscation; fastest on lossy
//     long-distance links, the controller pins the generated certificate.
//   - shadowsocks: Shadowsocks 2022, the lightweight fallback.
const (
	sdwanUplinkClient        = "sdwan-uplink"
	sdwanLegacyUplinkTag     = "sdwan-uplink"
	SdwanProtocolReality     = "reality"
	SdwanProtocolHysteria2   = "hysteria2"
	SdwanProtocolShadowsocks = "shadowsocks"
	sdwanShadowsocksMethod   = "2022-blake3-aes-128-gcm"
	sdwanRealityTLSPrefix    = "sdwan-reality-"
	sdwanRealityFingerprint  = "chrome"
	defaultSdwanRealitySNI   = "www.amazon.com"
)

var sdwanProtocolOrder = []string{SdwanProtocolReality, SdwanProtocolHysteria2, SdwanProtocolShadowsocks}

var sdwanUplinkTags = map[string]string{
	SdwanProtocolReality:     "sdwan-uplink-reality",
	SdwanProtocolHysteria2:   "sdwan-uplink-hy2",
	SdwanProtocolShadowsocks: "sdwan-uplink-ss",
}

var sdwanInboundTypes = map[string]string{
	SdwanProtocolReality:     "vless",
	SdwanProtocolHysteria2:   "hysteria2",
	SdwanProtocolShadowsocks: "shadowsocks",
}

// Large TLS 1.3 + HTTP/2 sites used as Reality handshake targets. Each managed
// server probes them and uses the fastest one it can reach. Apple, iCloud and
// Microsoft sites (and .cn/.ru/.ir domains) are left out: Xray-core warns that
// imitating them makes the GFW more likely to block the server's IP.
var sdwanRealityCandidates = []string{
	"www.amazon.com", "addons.mozilla.org", "www.nvidia.com", "dl.google.com",
	"www.samsung.com", "www.oracle.com",
}

// probeRealityServer is replaceable in tests to avoid network access.
var probeRealityServer = defaultProbeRealityServer

var sdwanUplinkMu sync.Mutex

type SdwanProvisionRequest struct {
	Protocols     []string `json:"protocols"`
	RealityServer string   `json:"reality_server,omitempty"`
	Rotate        bool     `json:"rotate,omitempty"`
	Actor         string   `json:"actor"`
	PublicHost    string   `json:"public_host"`
}

type SdwanUplink struct {
	Protocol string          `json:"protocol"`
	Tag      string          `json:"tag"`
	Port     int             `json:"port"`
	Detail   string          `json:"detail,omitempty"`
	Outbound json.RawMessage `json:"outbound"`
}

type SdwanProvisionResponse struct {
	Server   string        `json:"server"`
	Uplinks  []SdwanUplink `json:"uplinks"`
	Revision uint64        `json:"revision"`
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
	case "reality", "vless", "vless-reality":
		return SdwanProtocolReality, nil
	case "hysteria2", "hy2":
		return SdwanProtocolHysteria2, nil
	case "shadowsocks", "ss", "ss2022":
		return SdwanProtocolShadowsocks, nil
	}
	return "", common.NewErrorf("unsupported SD-WAN uplink protocol %q", value)
}

// localSdwanProtocols lists the uplink protocols this build can serve.
func localSdwanProtocols() []string {
	protocols := []string{}
	if sdwanRealitySupported {
		protocols = append(protocols, SdwanProtocolReality)
	}
	if sdwanHysteria2Supported {
		protocols = append(protocols, SdwanProtocolHysteria2)
	}
	return append(protocols, SdwanProtocolShadowsocks)
}

func sdwanProtocolAvailable(protocol string) bool {
	switch protocol {
	case SdwanProtocolReality:
		return sdwanRealitySupported
	case SdwanProtocolHysteria2:
		return sdwanHysteria2Supported
	case SdwanProtocolShadowsocks:
		return true
	}
	return false
}

func normalizeSdwanProtocolList(values []string) ([]string, error) {
	wanted := map[string]bool{}
	for _, value := range values {
		protocol, err := normalizeSdwanProtocol(value)
		if err != nil {
			return nil, err
		}
		wanted[protocol] = true
	}
	var protocols []string
	for _, protocol := range sdwanProtocolOrder {
		if wanted[protocol] {
			protocols = append(protocols, protocol)
		}
	}
	if len(protocols) == 0 {
		return nil, common.NewError("no SD-WAN uplink protocol requested")
	}
	return protocols, nil
}

func normalizeRealityServer(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if len(value) > 253 || strings.ContainsAny(value, " /?#@:[]\t\r\n") || net.ParseIP(value) != nil || !strings.Contains(value, ".") {
		return "", common.NewErrorf("Reality target %q must be a domain name such as www.amazon.com", value)
	}
	return strings.ToLower(value), nil
}

// ProvisionSdwanUplink makes sure the managed server runs exactly the uplinks
// the controller asked for and returns the sing-box outbound for each one.
// Existing uplinks are reused, so repeated calls keep their credentials.
func (s *LocalControlService) ProvisionSdwanUplink(request SdwanProvisionRequest) (*SdwanProvisionResponse, error) {
	protocols, err := normalizeSdwanProtocolList(request.Protocols)
	if err != nil {
		return nil, err
	}
	for _, protocol := range protocols {
		if !sdwanProtocolAvailable(protocol) {
			return nil, common.NewErrorf("this server's build does not support the %s uplink", protocol)
		}
	}
	realityServer, err := normalizeRealityServer(request.RealityServer)
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

	sdwanUplinkMu.Lock()
	defer sdwanUplinkMu.Unlock()

	revision, err := s.ConfigService.CurrentRevision()
	if err != nil {
		return nil, err
	}
	state, err := loadSdwanUplinks()
	if err != nil {
		return nil, err
	}
	requested := map[string]bool{}
	for _, protocol := range protocols {
		requested[protocol] = true
	}
	// Drop uplinks that were not requested, are broken, use another Reality
	// target or must be rotated; everything else is reused as is.
	for protocol, inbound := range state.inbounds {
		keep := requested[protocol] && !request.Rotate && state.client != nil
		if keep {
			_, _, detail, buildErr := sdwanOutboundFromUplink(protocol, inbound, state.client, publicHost)
			keep = buildErr == nil && (protocol != SdwanProtocolReality || realityServer == "" || detail == realityServer)
		}
		if !keep {
			if revision, err = s.removeSdwanInbound(inbound, revision, actor, publicHost); err != nil {
				return nil, err
			}
			delete(state.inbounds, protocol)
		}
	}
	if state.legacy != nil {
		if revision, err = s.removeSdwanInbound(state.legacy, revision, actor, publicHost); err != nil {
			return nil, err
		}
	}
	if request.Rotate && state.client != nil {
		if revision, err = s.deleteSdwanClient(state.client, revision, actor, publicHost); err != nil {
			return nil, err
		}
		state.client = nil
	}

	client, revision, err := s.ensureSdwanClient(state.client, protocols, revision, actor, publicHost)
	if err != nil {
		return nil, err
	}
	for _, protocol := range protocols {
		if state.inbounds[protocol] != nil {
			continue
		}
		if revision, err = s.createSdwanInbound(protocol, client.Id, realityServer, revision, actor, publicHost); err != nil {
			return nil, err
		}
	}

	state, err = loadSdwanUplinks()
	if err != nil {
		return nil, err
	}
	response := &SdwanProvisionResponse{Server: publicHost, Revision: revision}
	for _, protocol := range protocols {
		inbound := state.inbounds[protocol]
		if inbound == nil {
			return nil, common.NewErrorf("SD-WAN %s uplink was not saved", protocol)
		}
		outbound, port, detail, buildErr := sdwanOutboundFromUplink(protocol, inbound, state.client, publicHost)
		if buildErr != nil {
			return nil, buildErr
		}
		response.Uplinks = append(response.Uplinks, SdwanUplink{
			Protocol: protocol, Tag: inbound.Tag, Port: port, Detail: detail, Outbound: outbound,
		})
	}
	return response, nil
}

// RemoveSdwanUplink deletes every SD-WAN uplink, the internal user and the
// certificates or Reality keys generated for them.
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
	state, err := loadSdwanUplinks()
	if err != nil {
		return nil, err
	}
	removed := false
	for _, inbound := range state.allInbounds() {
		if revision, err = s.removeSdwanInbound(inbound, revision, actor, publicHost); err != nil {
			return nil, err
		}
		removed = true
	}
	if state.client != nil {
		if revision, err = s.deleteSdwanClient(state.client, revision, actor, publicHost); err != nil {
			return nil, err
		}
		removed = true
	}
	return &SdwanRemoveResponse{Removed: removed, Revision: revision}, nil
}

type sdwanUplinkState struct {
	inbounds map[string]*model.Inbound
	legacy   *model.Inbound
	client   *model.Client
}

func (s sdwanUplinkState) allInbounds() []*model.Inbound {
	result := []*model.Inbound{}
	for _, protocol := range sdwanProtocolOrder {
		if inbound := s.inbounds[protocol]; inbound != nil {
			result = append(result, inbound)
		}
	}
	if s.legacy != nil {
		result = append(result, s.legacy)
	}
	return result
}

func isSdwanUplinkTag(tag string) bool {
	if tag == sdwanLegacyUplinkTag {
		return true
	}
	for _, uplinkTag := range sdwanUplinkTags {
		if tag == uplinkTag {
			return true
		}
	}
	return false
}

func loadSdwanUplinks() (sdwanUplinkState, error) {
	state := sdwanUplinkState{inbounds: map[string]*model.Inbound{}}
	db := database.GetDB()
	tags := []string{sdwanLegacyUplinkTag}
	for _, tag := range sdwanUplinkTags {
		tags = append(tags, tag)
	}
	var inbounds []model.Inbound
	if err := db.Preload("Tls").Where("tag IN ?", tags).Find(&inbounds).Error; err != nil {
		return state, err
	}
	for index := range inbounds {
		inbound := &inbounds[index]
		if inbound.Tag == sdwanLegacyUplinkTag {
			state.legacy = inbound
			continue
		}
		for protocol, tag := range sdwanUplinkTags {
			if inbound.Tag == tag {
				state.inbounds[protocol] = inbound
			}
		}
	}
	var clients []model.Client
	if err := db.Where("name = ?", sdwanUplinkClient).Limit(1).Find(&clients).Error; err != nil {
		return state, err
	}
	if len(clients) > 0 {
		state.client = &clients[0]
	}
	return state, nil
}

func (s *LocalControlService) removeSdwanInbound(inbound *model.Inbound, revision uint64, actor, publicHost string) (uint64, error) {
	tag, _ := json.Marshal(inbound.Tag)
	_, revision, err := s.ConfigService.SaveWithRevision(revision, "inbounds", "del", tag, "", "agent:"+actor, publicHost)
	if err != nil {
		return revision, err
	}
	if inbound.TlsId == 0 || inbound.Tls == nil {
		return revision, nil
	}
	if !strings.HasPrefix(inbound.Tls.Name, generatedTLSNamePrefix) && !strings.HasPrefix(inbound.Tls.Name, sdwanRealityTLSPrefix) {
		return revision, nil
	}
	var users int64
	if err = database.GetDB().Model(&model.Inbound{}).Where("tls_id = ?", inbound.TlsId).Count(&users).Error; err != nil || users > 0 {
		return revision, err
	}
	id, _ := json.Marshal(inbound.TlsId)
	_, revision, err = s.ConfigService.SaveWithRevision(revision, "tls", "del", id, "", "agent:"+actor, publicHost)
	return revision, err
}

func (s *LocalControlService) deleteSdwanClient(client *model.Client, revision uint64, actor, publicHost string) (uint64, error) {
	id, _ := json.Marshal(client.Id)
	_, revision, err := s.ConfigService.SaveWithRevision(revision, "clients", "del", id, "", "agent:"+actor, publicHost)
	return revision, err
}

// ensureSdwanClient creates the internal user or adds the credentials that a
// newly requested protocol needs, keeping existing credentials untouched.
func (s *LocalControlService) ensureSdwanClient(client *model.Client, protocols []string, revision uint64, actor, publicHost string) (*model.Client, uint64, error) {
	config := map[string]map[string]interface{}{}
	if client != nil && len(client.Config) > 0 {
		if err := json.Unmarshal(client.Config, &config); err != nil {
			return nil, revision, err
		}
	}
	changed := false
	for _, protocol := range protocols {
		switch protocol {
		case SdwanProtocolReality:
			if uuid, _ := config["vless"]["uuid"].(string); uuid == "" {
				config["vless"] = map[string]interface{}{"name": sdwanUplinkClient, "uuid": randomUUID(), "flow": "xtls-rprx-vision"}
				changed = true
			} else if flow, _ := config["vless"]["flow"].(string); flow != "xtls-rprx-vision" {
				config["vless"]["flow"] = "xtls-rprx-vision"
				changed = true
			}
		case SdwanProtocolHysteria2:
			if password, _ := config["hysteria2"]["password"].(string); password == "" {
				config["hysteria2"] = map[string]interface{}{"name": sdwanUplinkClient, "password": common.Random(24)}
				changed = true
			}
		case SdwanProtocolShadowsocks:
			if password, _ := config["shadowsocks16"]["password"].(string); password == "" {
				config["shadowsocks16"] = map[string]interface{}{"name": sdwanUplinkClient, "password": relayShadowsocksKey(sdwanShadowsocksMethod)}
				changed = true
			}
		}
	}
	if client != nil && !changed {
		return client, revision, nil
	}
	rawConfig, err := json.Marshal(config)
	if err != nil {
		return nil, revision, err
	}
	var raw []byte
	action := "new"
	if client == nil {
		raw, err = json.Marshal(map[string]interface{}{
			"id": 0, "enable": true, "name": sdwanUplinkClient, "config": json.RawMessage(rawConfig),
			"inbounds": []uint{}, "links": []interface{}{}, "volume": 0, "expiry": 0,
			"up": 0, "down": 0, "desc": "SD-WAN uplink used by the controller", "group": "sdwan", "remark": "",
		})
	} else {
		action = "edit"
		updated := *client
		updated.Enable = true
		updated.Config = rawConfig
		raw, err = json.Marshal(updated)
	}
	if err != nil {
		return nil, revision, err
	}
	if _, revision, err = s.ConfigService.SaveWithRevision(revision, "clients", action, raw, "", "agent:"+actor, publicHost); err != nil {
		return nil, revision, err
	}
	var saved model.Client
	if err = database.GetDB().Where("name = ?", sdwanUplinkClient).Order("id desc").First(&saved).Error; err != nil {
		return nil, revision, err
	}
	return &saved, revision, nil
}

func (s *LocalControlService) createSdwanInbound(protocol string, clientID uint, realityServer string, revision uint64, actor, publicHost string) (uint64, error) {
	inbounds, err := s.InboundService.Get("")
	if err != nil {
		return revision, err
	}
	items := []map[string]interface{}{}
	if inbounds != nil {
		items = *inbounds
	}
	port, err := allocateSdwanUplinkPort(items, protocol)
	if err != nil {
		return revision, err
	}
	inbound := map[string]interface{}{
		"id": 0, "core_type": model.CoreTypeSingBox, "type": sdwanInboundTypes[protocol], "tag": sdwanUplinkTags[protocol],
		"listen": quickAddListenAddress(publicHost), "listen_port": port,
		"addrs": []interface{}{}, "out_json": map[string]interface{}{},
	}
	switch protocol {
	case SdwanProtocolReality:
		if realityServer == "" {
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			realityServer = probeRealityServer(ctx)
			cancel()
		}
		tlsID, nextRevision, tlsErr := s.createSdwanRealityTLS(realityServer, revision, actor, publicHost)
		if tlsErr != nil {
			return revision, tlsErr
		}
		revision = nextRevision
		inbound["tls_id"] = tlsID
	case SdwanProtocolHysteria2:
		tlsID, nextRevision, tlsErr := s.createRemoteQuickAddTLS(strings.Trim(publicHost, "[]"), revision, "agent:"+actor, publicHost)
		if tlsErr != nil {
			return revision, tlsErr
		}
		revision = nextRevision
		inbound["tls_id"] = tlsID
		inbound["obfs"] = map[string]interface{}{
			"type": "salamander", "password": base64.StdEncoding.EncodeToString([]byte(common.Random(16))),
		}
	case SdwanProtocolShadowsocks:
		inbound["method"] = sdwanShadowsocksMethod
		inbound["password"] = relayShadowsocksKey(sdwanShadowsocksMethod)
	}
	raw, err := json.Marshal(inbound)
	if err != nil {
		return revision, err
	}
	_, revision, err = s.ConfigService.SaveWithRevision(
		revision, "inbounds", "new", raw, fmt.Sprintf("%d", clientID), "agent:"+actor, publicHost,
	)
	return revision, err
}

// sdwanUplinkPortRange keeps uplink ports below the kernel's ephemeral port
// range: a listener cannot bind a port that an outgoing connection (or its
// TIME_WAIT) holds, which would stop sing-box from starting on a busy server.
func sdwanUplinkPortRange() (int, int) {
	low, high := 20000, 32767
	data, err := os.ReadFile("/proc/sys/net/ipv4/ip_local_port_range")
	if err != nil {
		return low, high
	}
	fields := strings.Fields(string(data))
	if len(fields) != 2 {
		return low, high
	}
	start, startErr := strconv.Atoi(fields[0])
	end, endErr := strconv.Atoi(fields[1])
	if startErr != nil || endErr != nil || start < 1 || end < start {
		return low, high
	}
	switch {
	case start-1 >= low+1000:
		return low, start - 1
	case start > 11000:
		return 10000, start - 1
	case end+1000 <= 65000:
		return end + 1, 65000
	}
	return low, high
}

// sdwanPortFree reports whether no other process listens on the port.
func sdwanPortFree(port int) bool {
	address := net.JoinHostPort("", strconv.Itoa(port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	_ = listener.Close()
	packet, err := net.ListenPacket("udp", address)
	if err != nil {
		return false
	}
	_ = packet.Close()
	return true
}

func allocateSdwanUplinkPort(inbounds []map[string]interface{}, protocol string) (int, error) {
	low, high := sdwanUplinkPortRange()
	for attempt := 0; attempt < 32; attempt++ {
		ports, _, err := allocateRemoteQuickAdd(inbounds, low+common.RandomInt(high-low+1), 1, sdwanUplinkTags[protocol], protocol)
		if err != nil {
			return 0, err
		}
		if ports[0] >= low && ports[0] <= high && sdwanPortFree(ports[0]) {
			return ports[0], nil
		}
	}
	return 0, common.NewError("no free port for the SD-WAN uplink")
}

func (s *LocalControlService) createSdwanRealityTLS(serverName string, revision uint64, actor, publicHost string) (uint, uint64, error) {
	return s.createRealityTLS(sdwanRealityTLSPrefix+common.Random(8), serverName, revision, "agent:"+actor, publicHost)
}

// defaultProbeRealityServer returns the fastest candidate that completes a TLS
// 1.3 handshake with HTTP/2, which Reality needs to imitate it convincingly.
func defaultProbeRealityServer(ctx context.Context) string {
	type probe struct {
		host    string
		elapsed time.Duration
	}
	results := make(chan probe, len(sdwanRealityCandidates))
	for _, host := range sdwanRealityCandidates {
		go func(host string) {
			started := time.Now()
			dialer := &tls.Dialer{
				NetDialer: &net.Dialer{Timeout: 4 * time.Second},
				Config: &tls.Config{
					ServerName: host, MinVersion: tls.VersionTLS13, NextProtos: []string{"h2", "http/1.1"},
				},
			}
			conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, "443"))
			if err != nil {
				results <- probe{host: host}
				return
			}
			state := conn.(*tls.Conn).ConnectionState()
			_ = conn.Close()
			if state.Version != tls.VersionTLS13 || state.NegotiatedProtocol != "h2" {
				results <- probe{host: host}
				return
			}
			results <- probe{host: host, elapsed: time.Since(started)}
		}(host)
	}
	best := probe{}
	for range sdwanRealityCandidates {
		select {
		case result := <-results:
			if result.elapsed > 0 && (best.elapsed == 0 || result.elapsed < best.elapsed) {
				best = result
			}
		case <-ctx.Done():
			if best.host != "" {
				return best.host
			}
			return defaultSdwanRealitySNI
		}
	}
	if best.host == "" {
		return defaultSdwanRealitySNI
	}
	return best.host
}

func realityPublicKey(privateKey string) string {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(privateKey, "="))
	if err != nil || len(raw) != wgtypes.KeyLen {
		return ""
	}
	var key wgtypes.Key
	copy(key[:], raw)
	publicKey := key.PublicKey()
	return base64.RawURLEncoding.EncodeToString(publicKey[:])
}

// sdwanOutboundFromUplink rebuilds the controller-side sing-box outbound from
// the stored uplink, so repeated provisioning returns the same credentials.
// detail is the Reality handshake target, when there is one.
func sdwanOutboundFromUplink(protocol string, inbound *model.Inbound, client *model.Client, publicHost string) (json.RawMessage, int, string, error) {
	if inbound == nil || client == nil {
		return nil, 0, "", common.NewError("SD-WAN uplink is incomplete")
	}
	options := map[string]interface{}{}
	if err := json.Unmarshal(inbound.Options, &options); err != nil {
		return nil, 0, "", err
	}
	port := intFromInterface(options["listen_port"])
	if port < 1 || port > 65535 {
		return nil, 0, "", common.NewError("SD-WAN uplink has no valid port")
	}
	var clientConfig map[string]map[string]interface{}
	if err := json.Unmarshal(client.Config, &clientConfig); err != nil {
		return nil, 0, "", err
	}
	server := strings.Trim(publicHost, "[]")
	outbound := map[string]interface{}{"server": server, "server_port": port}
	detail := ""
	switch protocol {
	case SdwanProtocolReality:
		uuid, _ := clientConfig["vless"]["uuid"].(string)
		if inbound.Type != "vless" || uuid == "" || inbound.Tls == nil {
			return nil, 0, "", common.NewError("SD-WAN Reality uplink is incomplete")
		}
		var serverTLS map[string]interface{}
		if err := json.Unmarshal(inbound.Tls.Server, &serverTLS); err != nil {
			return nil, 0, "", err
		}
		reality, _ := serverTLS["reality"].(map[string]interface{})
		privateKey, _ := reality["private_key"].(string)
		shortIDs := stringValues(reality["short_id"])
		handshake, _ := reality["handshake"].(map[string]interface{})
		serverName, _ := serverTLS["server_name"].(string)
		if serverName == "" {
			serverName, _ = handshake["server"].(string)
		}
		publicKey := realityPublicKey(privateKey)
		if enabled, _ := reality["enabled"].(bool); !enabled || publicKey == "" || len(shortIDs) == 0 || serverName == "" {
			return nil, 0, "", common.NewError("SD-WAN Reality uplink has no usable key")
		}
		detail = serverName
		outbound["type"] = "vless"
		outbound["uuid"] = uuid
		outbound["flow"] = "xtls-rprx-vision"
		outbound["packet_encoding"] = "xudp"
		outbound["tls"] = map[string]interface{}{
			"enabled":     true,
			"server_name": serverName,
			"utls":        map[string]interface{}{"enabled": true, "fingerprint": sdwanRealityFingerprint},
			"reality":     map[string]interface{}{"enabled": true, "public_key": publicKey, "short_id": shortIDs[0]},
		}
	case SdwanProtocolHysteria2:
		password, _ := clientConfig["hysteria2"]["password"].(string)
		if inbound.Type != "hysteria2" || password == "" || inbound.Tls == nil {
			return nil, 0, "", common.NewError("SD-WAN Hysteria2 uplink is incomplete")
		}
		var serverTLS map[string]interface{}
		if err := json.Unmarshal(inbound.Tls.Server, &serverTLS); err != nil {
			return nil, 0, "", err
		}
		certificate := util.CertPEMFromTLS(serverTLS)
		if certificate == "" {
			return nil, 0, "", common.NewError("SD-WAN Hysteria2 uplink has no certificate")
		}
		serverName, _ := serverTLS["server_name"].(string)
		if serverName == "" {
			serverName = server
		}
		outbound["type"] = "hysteria2"
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
	case SdwanProtocolShadowsocks:
		method, _ := options["method"].(string)
		serverKey, _ := options["password"].(string)
		userKey, _ := clientConfig["shadowsocks16"]["password"].(string)
		if inbound.Type != "shadowsocks" || method != sdwanShadowsocksMethod || serverKey == "" || userKey == "" {
			return nil, 0, "", common.NewError("SD-WAN Shadowsocks uplink is incomplete")
		}
		outbound["type"] = "shadowsocks"
		outbound["method"] = method
		outbound["password"] = serverKey + ":" + userKey
	default:
		return nil, 0, "", common.NewErrorf("unsupported SD-WAN uplink protocol %q", protocol)
	}
	raw, err := json.Marshal(outbound)
	return raw, port, detail, err
}

func stringValues(value interface{}) []string {
	switch values := value.(type) {
	case []interface{}:
		result := make([]string, 0, len(values))
		for _, item := range values {
			if text, ok := item.(string); ok && text != "" {
				result = append(result, text)
			}
		}
		return result
	case []string:
		return values
	case string:
		if values != "" {
			return []string{values}
		}
	}
	return nil
}

type SdwanUplinkStatus struct {
	Protocol string `json:"protocol"`
	Tag      string `json:"tag"`
	Port     int    `json:"port"`
	Running  bool   `json:"running"`
}

type SdwanDiagnoseResponse struct {
	TimeMS      int64               `json:"time_ms"`
	Protocols   []string            `json:"protocols"`
	CoreRunning bool                `json:"core_running"`
	Tuning      SdwanTuningStatus   `json:"tuning"`
	Uplinks     []SdwanUplinkStatus `json:"uplinks"`
}

// DiagnoseSdwanUplink reports uplink health, kernel tuning and the local clock
// so the controller can explain slow or broken paths.
func (s *LocalControlService) DiagnoseSdwanUplink() (*SdwanDiagnoseResponse, error) {
	state, err := loadSdwanUplinks()
	if err != nil {
		return nil, err
	}
	response := &SdwanDiagnoseResponse{
		Protocols: localSdwanProtocols(), Tuning: ReadSdwanTuning(), Uplinks: []SdwanUplinkStatus{},
	}
	if corePtr != nil {
		response.CoreRunning = corePtr.IsRunning()
	}
	for _, protocol := range sdwanProtocolOrder {
		inbound := state.inbounds[protocol]
		if inbound == nil {
			continue
		}
		options := map[string]interface{}{}
		_ = json.Unmarshal(inbound.Options, &options)
		response.Uplinks = append(response.Uplinks, SdwanUplinkStatus{
			Protocol: protocol, Tag: inbound.Tag, Port: intFromInterface(options["listen_port"]),
			Running: corePtr != nil && corePtr.HasInbound(inbound.Tag),
		})
	}
	response.TimeMS = time.Now().UnixMilli()
	return response, nil
}
