package service

import (
	"encoding/json"
	"net"
	"net/netip"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/util/common"

	"gorm.io/gorm"
)

// The proxy client turns the panel on a home NAS or router into a
// PassWall / v2rayN-style client: subscriptions and nodes, latency tests,
// manual or automatic node selection, routing modes (bypass mainland China,
// GFW list only, global), a transparent proxy for the LAN (TUN with
// auto-redirect, which also works on OpenWrt's fw4 without extra rules) and a
// local SOCKS5/HTTP port. It runs inside the panel's own sing-box, so it adds
// no second proxy process and suits low-memory routers.

const proxyClientSettingsKey = "proxyClient"

// Routing modes.
const (
	ProxyClientModeRule   = "rule"   // mainland China direct, the rest through the proxy
	ProxyClientModeGFW    = "gfw"    // only the GFW list through the proxy
	ProxyClientModeGlobal = "global" // everything but private addresses through the proxy
)

// Local proxy listen scopes.
const (
	ProxyClientListenLAN   = "lan"   // every address, but only private clients
	ProxyClientListenLocal = "local" // this device only
	ProxyClientListenAll   = "all"   // anyone; needs a username and password
)

// Tags the client adds to the sing-box config.
const (
	proxyClientTunTag       = "client-tun"
	proxyClientMixedTag     = "client-mixed"
	proxyClientDNSInTag     = "client-dns-in"
	proxyClientProxyTag     = "client-proxy"
	proxyClientDirectTag    = "client-direct"
	proxyClientDNSDirectTag = "client-dns-direct"
	proxyClientDNSRemoteTag = "client-dns-remote"
	proxyClientNodePrefix   = "client-node-"

	proxyClientMaxNodes      = 2000
	proxyClientMaxAutoNodes  = 50
	proxyClientDefaultTest   = "https://www.gstatic.com/generate_204"
	proxyClientDefaultDirect = "223.5.5.5"
	proxyClientDefaultRemote = "https://1.1.1.1/dns-query"
)

// ProxyClientSettings is the client's configuration.
type ProxyClientSettings struct {
	Enabled bool   `json:"enabled"`
	Mode    string `json:"mode"`
	// NodeId is the node in use; Auto picks the fastest of the auto nodes.
	NodeId           uint   `json:"node_id"`
	Auto             bool   `json:"auto"`
	AutoSubscription uint   `json:"auto_subscription"`
	AutoFilter       string `json:"auto_filter"`
	AutoInterval     int    `json:"auto_interval"`
	AutoTolerance    int    `json:"auto_tolerance"`
	TestURL          string `json:"test_url"`

	// Tun proxies this device and, on a router, the whole LAN.
	Tun bool `json:"tun"`
	// Mixed is a SOCKS5 + HTTP proxy port for devices set up by hand.
	Mixed         bool   `json:"mixed"`
	MixedListen   string `json:"mixed_listen"`
	MixedPort     int    `json:"mixed_port"`
	MixedUsername string `json:"mixed_username"`
	MixedPassword string `json:"mixed_password"`

	DirectDNS string `json:"direct_dns"`
	RemoteDNS string `json:"remote_dns"`
	// DNSPort is a local DNS server (127.0.0.1) answering with the client's
	// DNS rules; on OpenWrt, DNSHijack points dnsmasq at it.
	DNSPort   int  `json:"dns_port"`
	DNSHijack bool `json:"dns_hijack"`
	IPv6      bool `json:"ipv6"`
	BlockQUIC bool `json:"block_quic"`

	// LAN devices that skip the proxy, or, when ProxyIPs is set, the only
	// devices that use it.
	BypassIPs     []string `json:"bypass_ips"`
	ProxyIPs      []string `json:"proxy_ips"`
	DirectDomains []string `json:"direct_domains"`
	ProxyDomains  []string `json:"proxy_domains"`
}

func defaultProxyClientSettings() ProxyClientSettings {
	return ProxyClientSettings{
		Mode: ProxyClientModeRule, AutoInterval: 300, AutoTolerance: 50, TestURL: proxyClientDefaultTest,
		Mixed: true, MixedListen: ProxyClientListenLAN, MixedPort: 7890,
		DirectDNS: proxyClientDefaultDirect, RemoteDNS: proxyClientDefaultRemote, DNSPort: 5335, DNSHijack: true,
		BypassIPs: []string{}, ProxyIPs: []string{}, DirectDomains: []string{}, ProxyDomains: []string{},
	}
}

func loadProxyClientSettings(db *gorm.DB) ProxyClientSettings {
	settings := defaultProxyClientSettings()
	var row model.Setting
	if err := db.Where("key = ?", proxyClientSettingsKey).First(&row).Error; err == nil && row.Value != "" {
		if err := json.Unmarshal([]byte(row.Value), &settings); err != nil {
			logger.Warning("proxy client settings are invalid, using defaults: ", err)
			settings = defaultProxyClientSettings()
		}
	}
	for _, list := range []*[]string{&settings.BypassIPs, &settings.ProxyIPs, &settings.DirectDomains, &settings.ProxyDomains} {
		if *list == nil {
			*list = []string{}
		}
	}
	return settings
}

func saveProxyClientSettings(db *gorm.DB, settings ProxyClientSettings) error {
	raw, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	var row model.Setting
	err = db.Where("key = ?", proxyClientSettingsKey).First(&row).Error
	if database.IsNotFound(err) {
		return db.Create(&model.Setting{Key: proxyClientSettingsKey, Value: string(raw)}).Error
	}
	if err != nil {
		return err
	}
	return db.Model(&model.Setting{}).Where("id = ?", row.Id).Update("value", string(raw)).Error
}

// normalizeProxyClientSettings validates settings in place.
func normalizeProxyClientSettings(settings *ProxyClientSettings) error {
	switch settings.Mode {
	case "":
		settings.Mode = ProxyClientModeRule
	case ProxyClientModeRule, ProxyClientModeGFW, ProxyClientModeGlobal:
	default:
		return common.NewError("mode must be rule, gfw or global")
	}
	if settings.AutoInterval == 0 {
		settings.AutoInterval = 300
	}
	if settings.AutoInterval < 60 || settings.AutoInterval > 3600 {
		return common.NewError("auto test interval must be 60-3600 seconds")
	}
	if settings.AutoTolerance < 0 || settings.AutoTolerance > 1000 {
		return common.NewError("auto tolerance must be 0-1000 ms")
	}
	settings.AutoFilter = strings.TrimSpace(settings.AutoFilter)
	if len(settings.AutoFilter) > 200 {
		return common.NewError("node filter is too long")
	}
	settings.TestURL = strings.TrimSpace(settings.TestURL)
	if settings.TestURL == "" {
		settings.TestURL = proxyClientDefaultTest
	}
	if !strings.HasPrefix(settings.TestURL, "http://") && !strings.HasPrefix(settings.TestURL, "https://") {
		return common.NewError("test URL must start with http:// or https://")
	}
	switch settings.MixedListen {
	case "":
		settings.MixedListen = ProxyClientListenLAN
	case ProxyClientListenLAN, ProxyClientListenLocal:
	case ProxyClientListenAll:
		if settings.Mixed && (settings.MixedUsername == "" || settings.MixedPassword == "") {
			return common.NewError("a proxy port open to every address needs a username and password")
		}
	default:
		return common.NewError("proxy port listen must be lan, local or all")
	}
	if settings.MixedPort == 0 {
		settings.MixedPort = 7890
	}
	if settings.DNSPort == 0 {
		settings.DNSPort = 5335
	}
	for _, port := range []int{settings.MixedPort, settings.DNSPort} {
		if port < 1 || port > 65535 {
			return common.NewError("ports must be between 1 and 65535")
		}
	}
	if settings.Mixed && settings.MixedPort == settings.DNSPort {
		return common.NewError("the proxy port and the DNS port must differ")
	}
	if len(settings.MixedUsername) > 64 || len(settings.MixedPassword) > 128 {
		return common.NewError("proxy port username or password is too long")
	}
	if settings.DirectDNS = strings.TrimSpace(settings.DirectDNS); settings.DirectDNS == "" {
		settings.DirectDNS = proxyClientDefaultDirect
	}
	if settings.RemoteDNS = strings.TrimSpace(settings.RemoteDNS); settings.RemoteDNS == "" {
		settings.RemoteDNS = proxyClientDefaultRemote
	}
	if _, err := proxyClientDNSServer(settings.DirectDNS, proxyClientDNSDirectTag, ""); err != nil {
		return err
	}
	if _, err := proxyClientDNSServer(settings.RemoteDNS, proxyClientDNSRemoteTag, proxyClientProxyTag); err != nil {
		return err
	}
	var err error
	if settings.BypassIPs, err = normalizeClientCIDRs(settings.BypassIPs); err != nil {
		return err
	}
	if settings.ProxyIPs, err = normalizeClientCIDRs(settings.ProxyIPs); err != nil {
		return err
	}
	if settings.DirectDomains, err = normalizeClientDomains(settings.DirectDomains); err != nil {
		return err
	}
	if settings.ProxyDomains, err = normalizeClientDomains(settings.ProxyDomains); err != nil {
		return err
	}
	return nil
}

// normalizeClientCIDRs accepts IP addresses and CIDR prefixes.
func normalizeClientCIDRs(values []string) ([]string, error) {
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(value); err == nil {
			result = append(result, prefix.Masked().String())
			continue
		}
		ip, err := netip.ParseAddr(value)
		if err != nil {
			return nil, common.NewErrorf("%q is not an IP address or CIDR", value)
		}
		result = append(result, netip.PrefixFrom(ip.Unmap(), ip.Unmap().BitLen()).String())
	}
	if len(result) > 256 {
		return nil, common.NewError("at most 256 device addresses")
	}
	return result, nil
}

// normalizeClientDomains accepts domain suffixes such as example.com.
func normalizeClientDomains(values []string) ([]string, error) {
	result := []string{}
	for _, value := range values {
		value = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), ".")
		value = strings.TrimPrefix(value, "*.")
		if value == "" {
			continue
		}
		if len(value) > 253 || strings.ContainsAny(value, " /:@\t") || !strings.Contains(value, ".") && net.ParseIP(value) == nil {
			return nil, common.NewErrorf("%q is not a domain", value)
		}
		result = append(result, value)
	}
	if len(result) > 1000 {
		return nil, common.NewError("at most 1000 custom domains")
	}
	return result, nil
}

// proxyClientDNSServer turns "223.5.5.5", "tcp://8.8.8.8", "tls://dns.google",
// "https://1.1.1.1/dns-query", "quic://…" or "local" into a sing-box DNS
// server. detour sends the queries through an outbound.
func proxyClientDNSServer(address, tag, detour string) (map[string]interface{}, error) {
	address = strings.TrimSpace(address)
	server := map[string]interface{}{"tag": tag}
	if address == "local" {
		server["type"] = "local"
		return server, nil
	}
	scheme, rest, hasScheme := strings.Cut(address, "://")
	if !hasScheme {
		scheme, rest = "udp", address
	}
	scheme = strings.ToLower(scheme)
	host, path := rest, ""
	if scheme == "https" || scheme == "h3" {
		if index := strings.Index(rest, "/"); index >= 0 {
			host, path = rest[:index], rest[index:]
		}
	}
	hostname, portText := host, ""
	if h, p, err := net.SplitHostPort(host); err == nil {
		hostname, portText = h, p
	}
	hostname = strings.Trim(hostname, "[]")
	if hostname == "" || strings.ContainsAny(hostname, " /@") {
		return nil, common.NewErrorf("invalid DNS server %q", address)
	}
	switch scheme {
	case "udp", "tcp", "tls", "https", "quic", "h3":
	default:
		return nil, common.NewErrorf("DNS server %q must be an IP, or start with tcp://, tls://, https:// or quic://", address)
	}
	server["type"] = scheme
	server["server"] = hostname
	if portText != "" {
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return nil, common.NewErrorf("invalid DNS server port in %q", address)
		}
		server["server_port"] = port
	}
	if path != "" && path != "/dns-query" {
		server["path"] = path
	}
	if detour != "" {
		server["detour"] = detour
	}
	if net.ParseIP(hostname) == nil && tag != proxyClientDNSDirectTag {
		// A DNS server given by name is found with the direct DNS.
		server["domain_resolver"] = proxyClientDNSDirectTag
	} else if net.ParseIP(hostname) == nil {
		return nil, common.NewError("the direct DNS server must be an IP address")
	}
	return server, nil
}

// ProxyClientPlatform describes what the device can run.
type ProxyClientPlatform struct {
	OS        string `json:"os"`
	OpenWrt   bool   `json:"openwrt"`
	Root      bool   `json:"root"`
	TunDevice bool   `json:"tun_device"`
	Nftables  bool   `json:"nftables"`
	Dnsmasq   bool   `json:"dnsmasq"`
}

// proxyClientPlatform is replaceable in tests.
var proxyClientPlatform = detectProxyClientPlatform

func detectProxyClientPlatform() ProxyClientPlatform {
	platform := ProxyClientPlatform{OS: runtime.GOOS, Root: os.Geteuid() == 0}
	if _, err := os.Stat("/etc/openwrt_release"); err == nil {
		platform.OpenWrt = true
	}
	if _, err := os.Stat("/dev/net/tun"); err == nil {
		platform.TunDevice = true
	}
	for _, path := range []string{"/usr/sbin/nft", "/sbin/nft", "/usr/bin/nft"} {
		if _, err := os.Stat(path); err == nil {
			platform.Nftables = true
			break
		}
	}
	if _, err := os.Stat("/proc/net/netfilter/nf_tables"); err == nil {
		platform.Nftables = true
	}
	if platform.OpenWrt {
		if _, err := os.Stat("/etc/init.d/dnsmasq"); err == nil {
			platform.Dnsmasq = true
		}
	}
	return platform
}

// ProxyClientNodeView is a node without its link.
type ProxyClientNodeView struct {
	model.ProxyClientNode
	Subscription string `json:"subscription,omitempty"`
}

// ProxyClientState is everything the client page shows.
type ProxyClientState struct {
	Settings      ProxyClientSettings             `json:"settings"`
	Platform      ProxyClientPlatform             `json:"platform"`
	Running       bool                            `json:"running"`
	Applied       bool                            `json:"applied"`
	Problem       string                          `json:"problem,omitempty"`
	Current       string                          `json:"current,omitempty"`
	CurrentId     uint                            `json:"current_id,omitempty"`
	Subscriptions []model.ProxyClientSubscription `json:"subscriptions"`
	Nodes         []ProxyClientNodeView           `json:"nodes"`
	RuleSets      []ProxyClientRuleSetStatus      `json:"rule_sets"`
	DNSHijacked   bool                            `json:"dns_hijacked"`
	Testing       bool                            `json:"testing"`
	UpdatedAt     int64                           `json:"updated_at"`
}

type ProxyClientService struct {
	ConfigService
}

// proxyClientMu serializes changes so a subscription refresh cannot race a
// node selection or a settings save.
var proxyClientMu sync.Mutex

// State reports the client's settings, nodes and runtime status.
func (s *ProxyClientService) State() (*ProxyClientState, error) {
	db := database.GetDB()
	state := &ProxyClientState{
		Settings: loadProxyClientSettings(db), Platform: proxyClientPlatform(),
		RuleSets: proxyClientRuleSetStatus(), UpdatedAt: time.Now().Unix(),
	}
	if err := db.Order("sort_order ASC, id ASC").Find(&state.Subscriptions).Error; err != nil {
		return nil, err
	}
	names := map[uint]string{}
	for _, subscription := range state.Subscriptions {
		names[subscription.Id] = subscription.Name
	}
	var nodes []model.ProxyClientNode
	if err := db.Order("subscription_id ASC, sort_order ASC, id ASC").Find(&nodes).Error; err != nil {
		return nil, err
	}
	state.Nodes = make([]ProxyClientNodeView, len(nodes))
	for i, node := range nodes {
		state.Nodes[i] = ProxyClientNodeView{ProxyClientNode: node, Subscription: names[node.SubscriptionId]}
	}
	state.Running = corePtr != nil && corePtr.IsRunning()
	state.Applied = state.Running && corePtr.HasOutbound(proxyClientProxyTag)
	state.DNSHijacked = proxyClientDNSHijacked()
	state.Testing = proxyClientTesting()
	if state.Settings.Enabled {
		state.Problem = proxyClientProblem(db, state.Settings, state.Platform)
		if state.Applied {
			if status, err := corePtr.GroupStatus(proxyClientProxyTag); err == nil {
				state.Current = status.Now
				if id, err := strconv.ParseUint(strings.TrimPrefix(status.Now, proxyClientNodePrefix), 10, 32); err == nil {
					state.CurrentId = uint(id)
				}
			}
		} else if state.Problem == "" && !state.Running {
			state.Problem = "sing-box is not running"
		}
	}
	return state, nil
}

// proxyClientProblem explains why an enabled client cannot run as set.
func proxyClientProblem(db *gorm.DB, settings ProxyClientSettings, platform ProxyClientPlatform) string {
	if len(proxyClientSelectedNodes(db, settings)) == 0 {
		if settings.Auto {
			return "no node matches the automatic selection"
		}
		return "no node is selected"
	}
	if settings.Tun {
		switch {
		case platform.OS != "linux":
			return "the transparent proxy needs Linux"
		case !platform.Root:
			return "the transparent proxy needs the panel to run as root"
		case !platform.TunDevice:
			return "/dev/net/tun is missing; install the TUN kernel module (OpenWrt: opkg install kmod-tun)"
		}
	}
	return ""
}

// proxyClientSelectedNodes returns the nodes the client routes through: the
// chosen node, or the automatic candidates.
func proxyClientSelectedNodes(db *gorm.DB, settings ProxyClientSettings) []model.ProxyClientNode {
	var nodes []model.ProxyClientNode
	if !settings.Auto {
		if settings.NodeId != 0 {
			db.Where("id = ?", settings.NodeId).Limit(1).Find(&nodes)
		}
		return nodes
	}
	query := db.Order("delay_ms = 0 ASC, delay_ms ASC, id ASC")
	if settings.AutoSubscription != 0 {
		query = query.Where("subscription_id = ?", settings.AutoSubscription)
	}
	var candidates []model.ProxyClientNode
	query.Find(&candidates)
	keywords := strings.FieldsFunc(strings.ToLower(settings.AutoFilter), func(r rune) bool { return r == '|' || r == ',' || r == ' ' })
	for _, node := range candidates {
		if len(keywords) > 0 && !slices.ContainsFunc(keywords, func(keyword string) bool {
			return strings.Contains(strings.ToLower(node.Name), keyword)
		}) {
			continue
		}
		// Nodes whose last real test failed come last but stay eligible.
		nodes = append(nodes, node)
		if len(nodes) >= proxyClientMaxAutoNodes {
			break
		}
	}
	return nodes
}

// SaveSettings stores new settings and applies them.
func (s *ProxyClientService) SaveSettings(settings ProxyClientSettings) (*ProxyClientState, error) {
	if err := normalizeProxyClientSettings(&settings); err != nil {
		return nil, err
	}
	proxyClientMu.Lock()
	db := database.GetDB()
	previous := loadProxyClientSettings(db)
	if err := saveProxyClientSettings(db, settings); err != nil {
		proxyClientMu.Unlock()
		return nil, err
	}
	proxyClientMu.Unlock()
	if settings.Enabled {
		// Fetch the rule files the mode needs before the core restarts.
		if err := ensureProxyClientRuleSets(settings.Mode, false); err != nil {
			logger.Warning("proxy client rule files: ", err)
		}
	}
	if err := s.apply(previous, settings); err != nil {
		return nil, err
	}
	return s.State()
}

// apply restarts sing-box when the client's part of the config changed and
// keeps OpenWrt's dnsmasq pointed at the client DNS while it runs.
func (s *ProxyClientService) apply(previous, current ProxyClientSettings) error {
	if !previous.Enabled && !current.Enabled {
		return nil
	}
	syncProxyClientDNS(current)
	if corePtr == nil {
		return nil
	}
	if !corePtr.IsRunning() {
		if current.Enabled {
			return s.ConfigService.StartCore()
		}
		return nil
	}
	return s.ConfigService.RestartCore()
}

// Select switches to one node, or to automatic selection with nodeID 0.
func (s *ProxyClientService) Select(nodeID uint) (*ProxyClientState, error) {
	db := database.GetDB()
	proxyClientMu.Lock()
	settings := loadProxyClientSettings(db)
	previous := settings
	if nodeID == 0 {
		settings.Auto = true
	} else {
		var count int64
		db.Model(&model.ProxyClientNode{}).Where("id = ?", nodeID).Count(&count)
		if count == 0 {
			proxyClientMu.Unlock()
			return nil, common.NewError("node not found")
		}
		settings.Auto, settings.NodeId = false, nodeID
	}
	if err := saveProxyClientSettings(db, settings); err != nil {
		proxyClientMu.Unlock()
		return nil, err
	}
	proxyClientMu.Unlock()
	if err := s.apply(previous, settings); err != nil {
		return nil, err
	}
	return s.State()
}

// ProxyClientExit is where traffic through the proxy leaves the internet.
type ProxyClientExit struct {
	OK      bool   `json:"ok"`
	DelayMs int    `json:"delay_ms"`
	IP      string `json:"ip,omitempty"`
	Country string `json:"country,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Exit fetches Cloudflare's trace page through the running client proxy.
func (s *ProxyClientService) Exit() *ProxyClientExit {
	if corePtr == nil || !corePtr.HasOutbound(proxyClientProxyTag) {
		return &ProxyClientExit{Error: "the proxy client is not running"}
	}
	result := core.CheckWarp(corePtr.GetCtx(), proxyClientProxyTag, "")
	return &ProxyClientExit{OK: result.OK, DelayMs: int(result.Delay), IP: result.IP, Country: result.Loc, Error: result.Error}
}

// ProxyClientCall is one client action, as sent by the page or, for a
// managed device, by the controller over client.call.
type ProxyClientCall struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data,omitempty"`
}

// ProxyClientReadOnly lists the actions that change nothing.
func ProxyClientReadOnly(action string) bool {
	return action == "state" || action == "exit"
}

// Call runs one client action and returns its result.
func (s *ProxyClientService) Call(call ProxyClientCall) (interface{}, error) {
	decode := func(target interface{}) error {
		if len(call.Data) == 0 {
			return nil
		}
		return json.Unmarshal(call.Data, target)
	}
	switch call.Action {
	case "state":
		return s.State()
	case "exit":
		return s.Exit(), nil
	case "settings":
		var settings ProxyClientSettings
		if err := decode(&settings); err != nil {
			return nil, err
		}
		return s.SaveSettings(settings)
	case "select":
		var payload struct {
			NodeId uint `json:"node_id"`
		}
		if err := decode(&payload); err != nil {
			return nil, err
		}
		return s.Select(payload.NodeId)
	case "test":
		var payload struct {
			Ids []uint `json:"ids"`
		}
		if err := decode(&payload); err != nil {
			return nil, err
		}
		return s.TestNodes(payload.Ids)
	case "subscription":
		var payload ProxyClientSubscriptionInput
		if err := decode(&payload); err != nil {
			return nil, err
		}
		return s.SaveSubscription(payload)
	case "subscription.update":
		var payload struct {
			Id uint `json:"id"`
		}
		if err := decode(&payload); err != nil {
			return nil, err
		}
		return s.UpdateSubscriptions(payload.Id)
	case "subscription.delete":
		var payload struct {
			Id uint `json:"id"`
		}
		if err := decode(&payload); err != nil {
			return nil, err
		}
		return s.DeleteSubscription(payload.Id)
	case "nodes.add":
		var payload struct {
			Links string `json:"links"`
		}
		if err := decode(&payload); err != nil {
			return nil, err
		}
		return s.AddNodes(payload.Links)
	case "nodes.delete":
		var payload struct {
			Ids []uint `json:"ids"`
		}
		if err := decode(&payload); err != nil {
			return nil, err
		}
		return s.DeleteNodes(payload.Ids)
	case "rules.update":
		return s.UpdateRuleSets()
	case "mode":
		var payload struct {
			Enabled *bool  `json:"enabled"`
			Mode    string `json:"mode"`
		}
		if err := decode(&payload); err != nil {
			return nil, err
		}
		return s.SetMode(payload.Enabled, payload.Mode)
	default:
		return nil, common.NewError("unknown proxy client action ", call.Action)
	}
}

// ProxyClientWantsCore reports whether the proxy client is switched on, so
// sing-box must run even on installs that do not start it by default.
func ProxyClientWantsCore() bool {
	db := database.GetDB()
	return db != nil && loadProxyClientSettings(db).Enabled
}
