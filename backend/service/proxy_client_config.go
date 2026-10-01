package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"

	"gorm.io/gorm"
)

// privateRanges never go through the client: the LAN, CGNAT, link-local and
// multicast. The TUN routes skip them, so local traffic never enters sing-box.
var privateRanges = []string{
	"10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12",
	"192.168.0.0/16", "224.0.0.0/4", "240.0.0.0/4",
	"fc00::/7", "fe80::/10", "ff00::/8",
}

// applyProxyClientConfig adds the proxy client to the panel's sing-box
// config. Every rule it adds names the client's inbounds, so the panel's own
// inbounds route exactly as before.
func applyProxyClientConfig(db *gorm.DB, config *SingBoxConfig) {
	proxyClientRunning.Store("")
	settings := loadProxyClientSettings(db)
	if !settings.Enabled {
		return
	}
	if err := normalizeProxyClientSettings(&settings); err != nil {
		logger.Warning("proxy client disabled: ", err)
		return
	}
	nodes := proxyClientSelectedNodes(db, settings)
	if len(nodes) == 0 {
		logger.Warning("proxy client is enabled but no node is selected")
		return
	}
	platform := proxyClientPlatform()
	tun := settings.Tun && proxyClientProblem(db, settings, platform) == ""

	taken := configTags(config.Inbounds)
	for tag, kind := range configTags(config.Outbounds) {
		taken[tag] = kind
	}
	for tag, kind := range configTags(config.Endpoints) {
		taken[tag] = kind
	}
	for _, tag := range []string{proxyClientTunTag, proxyClientMixedTag, proxyClientDNSInTag, proxyClientProxyTag, proxyClientDirectTag} {
		if _, used := taken[tag]; used {
			logger.Warning("proxy client disabled: tag ", tag, " is already used")
			return
		}
	}

	// Outbounds: the nodes, the proxy (selector or urltest) and direct.
	var outbounds []json.RawMessage
	var memberTags []string
	for _, node := range nodes {
		outbound, err := clientNodeOutbound(node)
		if err != nil {
			logger.Warning("proxy client skips node ", node.Name, ": ", err)
			continue
		}
		raw, err := json.Marshal(outbound)
		if err != nil {
			continue
		}
		outbounds = append(outbounds, raw)
		memberTags = append(memberTags, outbound["tag"].(string))
	}
	if len(memberTags) == 0 {
		logger.Warning("proxy client has no usable node")
		return
	}
	var group map[string]interface{}
	if settings.Auto {
		group = map[string]interface{}{
			"type": "urltest", "tag": proxyClientProxyTag, "outbounds": memberTags, "url": settings.TestURL,
			"interval": fmt.Sprintf("%ds", settings.AutoInterval), "tolerance": settings.AutoTolerance,
			"idle_timeout": fmt.Sprintf("%ds", max(settings.AutoInterval*6, 1800)),
		}
	} else {
		group = map[string]interface{}{"type": "selector", "tag": proxyClientProxyTag, "outbounds": memberTags, "default": memberTags[0]}
	}
	groupRaw, _ := json.Marshal(group)
	directRaw, _ := json.Marshal(map[string]interface{}{"type": "direct", "tag": proxyClientDirectTag})
	outbounds = append(outbounds, groupRaw, directRaw)

	// Inbounds.
	var inbounds []json.RawMessage
	var clientInbounds []string
	if tun {
		addresses := []string{"172.19.0.1/30"}
		if settings.IPv6 {
			addresses = append(addresses, "fdfe:dcba:9876::1/126")
		}
		tunInbound := map[string]interface{}{
			"type": "tun", "tag": proxyClientTunTag, "interface_name": "sui-tun", "address": addresses,
			"mtu": 9000, "auto_route": true, "auto_redirect": platform.Nftables, "strict_route": true,
			"stack": "system", "route_exclude_address": privateRanges,
		}
		if settings.Mode == ProxyClientModeRule && platform.Nftables && proxyClientRuleSetReady(ruleSetGeoIPCN) {
			// Mainland China addresses skip sing-box entirely, like
			// PassWall's chnroute set: fast, and no memory spent on them.
			tunInbound["route_exclude_address_set"] = []string{ruleSetGeoIPCN}
		}
		raw, _ := json.Marshal(tunInbound)
		inbounds = append(inbounds, raw)
		clientInbounds = append(clientInbounds, proxyClientTunTag)
	}
	if settings.Mixed {
		listen := "0.0.0.0"
		if settings.MixedListen == ProxyClientListenLocal {
			listen = "127.0.0.1"
		}
		mixed := map[string]interface{}{"type": "mixed", "tag": proxyClientMixedTag, "listen": listen, "listen_port": settings.MixedPort}
		if settings.MixedUsername != "" {
			mixed["users"] = []map[string]string{{"username": settings.MixedUsername, "password": settings.MixedPassword}}
		}
		raw, _ := json.Marshal(mixed)
		inbounds = append(inbounds, raw)
		clientInbounds = append(clientInbounds, proxyClientMixedTag)
	}
	dnsIn, _ := json.Marshal(map[string]interface{}{"type": "direct", "tag": proxyClientDNSInTag, "listen": "127.0.0.1", "listen_port": settings.DNSPort})
	inbounds = append(inbounds, dnsIn)
	clientInbounds = append(clientInbounds, proxyClientDNSInTag)

	// Rule sets the mode needs, from the files on disk.
	var ruleSets []interface{}
	ready := map[string]bool{}
	for _, tag := range proxyClientRuleSetsFor(settings.Mode) {
		if !proxyClientRuleSetReady(tag) {
			logger.Warning("proxy client rule file ", tag, " is missing; it routes as if empty")
			continue
		}
		ready[tag] = true
		ruleSets = append(ruleSets, map[string]interface{}{"type": "local", "tag": tag, "format": "binary", "path": proxyClientRuleSetPath(tag)})
	}

	routeRules, dnsRules := proxyClientRules(settings, clientInbounds, ready)
	dnsServers, err := proxyClientDNSServers(settings)
	if err != nil {
		logger.Warning("proxy client disabled: ", err)
		return
	}

	route, err := mergeProxyClientRoute(config.Route, routeRules, ruleSets, tun)
	if err != nil {
		logger.Warning("proxy client disabled: route: ", err)
		return
	}
	dns, err := mergeProxyClientDNS(config.Dns, dnsServers, dnsRules)
	if err != nil {
		logger.Warning("proxy client disabled: dns: ", err)
		return
	}
	config.Route, config.Dns = route, dns
	config.Inbounds = append(config.Inbounds, inbounds...)
	config.Outbounds = append(config.Outbounds, outbounds...)
	proxyClientRunning.Store(proxyClientNodesFingerprint(nodes))
}

// proxyClientRules builds the route and DNS rules for the client inbounds.
func proxyClientRules(settings ProxyClientSettings, inbounds []string, ready map[string]bool) ([]interface{}, []interface{}) {
	scoped := func(rule map[string]interface{}) map[string]interface{} {
		rule["inbound"] = inbounds
		return rule
	}
	direct := func(rule map[string]interface{}) map[string]interface{} {
		rule["outbound"] = proxyClientDirectTag
		return scoped(rule)
	}
	proxy := func(rule map[string]interface{}) map[string]interface{} {
		rule["outbound"] = proxyClientProxyTag
		return scoped(rule)
	}
	rules := []interface{}{
		scoped(map[string]interface{}{"action": "sniff"}),
		map[string]interface{}{"inbound": []string{proxyClientDNSInTag}, "action": "hijack-dns"},
		scoped(map[string]interface{}{"protocol": []string{"dns"}, "action": "hijack-dns"}),
	}
	if settings.Mixed && settings.MixedListen == ProxyClientListenLAN {
		// The proxy port answers private (LAN) clients only.
		rules = append(rules, map[string]interface{}{
			"type": "logical", "mode": "and", "action": "reject",
			"rules": []interface{}{
				map[string]interface{}{"inbound": []string{proxyClientMixedTag}},
				map[string]interface{}{"source_ip_is_private": true, "invert": true},
			},
		})
	}
	rules = append(rules, direct(map[string]interface{}{"ip_is_private": true}))
	var controllerDomains, controllerIPs []string
	if host := localControllerHost(); host != "" {
		// The agent's connection to the controller never depends on the proxy.
		if strings.Contains(host, ":") || strings.Count(host, ".") == 3 && strings.Trim(host, "0123456789.") == "" {
			controllerIPs = append(controllerIPs, host)
		} else {
			controllerDomains = append(controllerDomains, host)
		}
	}
	if len(controllerDomains) > 0 {
		rules = append(rules, direct(map[string]interface{}{"domain": controllerDomains}))
	}
	if len(controllerIPs) > 0 {
		rules = append(rules, direct(map[string]interface{}{"ip_cidr": controllerIPs}))
	}
	if len(settings.BypassIPs) > 0 {
		rules = append(rules, direct(map[string]interface{}{"source_ip_cidr": settings.BypassIPs}))
	}
	if len(settings.ProxyIPs) > 0 {
		rules = append(rules, map[string]interface{}{
			"type": "logical", "mode": "and", "outbound": proxyClientDirectTag,
			"rules": []interface{}{
				map[string]interface{}{"inbound": inbounds},
				map[string]interface{}{"source_ip_cidr": settings.ProxyIPs, "invert": true},
			},
		})
	}
	if len(settings.DirectDomains) > 0 {
		rules = append(rules, direct(map[string]interface{}{"domain_suffix": settings.DirectDomains}))
	}
	if len(settings.ProxyDomains) > 0 {
		rules = append(rules, proxy(map[string]interface{}{"domain_suffix": settings.ProxyDomains}))
	}
	switch settings.Mode {
	case ProxyClientModeRule:
		var sets []string
		for _, tag := range []string{ruleSetGeositeCN, ruleSetGeoIPCN} {
			if ready[tag] {
				sets = append(sets, tag)
			}
		}
		if len(sets) > 0 {
			rules = append(rules, direct(map[string]interface{}{"rule_set": sets}))
		}
	case ProxyClientModeGFW:
		if ready[ruleSetGeositeGFW] {
			if settings.BlockQUIC {
				rules = append(rules, scoped(map[string]interface{}{"rule_set": []string{ruleSetGeositeGFW}, "network": []string{"udp"}, "port": []int{443}, "action": "reject"}))
			}
			rules = append(rules, proxy(map[string]interface{}{"rule_set": []string{ruleSetGeositeGFW}}))
		}
		rules = append(rules, direct(map[string]interface{}{}))
	}
	if settings.Mode != ProxyClientModeGFW {
		if settings.BlockQUIC {
			// Browsers fall back to TCP, which most proxies carry better.
			rules = append(rules, scoped(map[string]interface{}{"network": []string{"udp"}, "port": []int{443}, "action": "reject"}))
		}
		rules = append(rules, proxy(map[string]interface{}{}))
	}

	// DNS: names that go direct are resolved directly, the rest through the
	// proxy, so foreign names get answers that suit the proxy's exit.
	route := func(rule map[string]interface{}, server string) map[string]interface{} {
		rule["inbound"] = inbounds
		rule["server"] = server
		if !settings.IPv6 {
			rule["strategy"] = "ipv4_only"
		}
		return rule
	}
	var dnsRules []interface{}
	if len(controllerDomains) > 0 {
		dnsRules = append(dnsRules, route(map[string]interface{}{"domain": controllerDomains}, proxyClientDNSDirectTag))
	}
	if len(settings.DirectDomains) > 0 {
		dnsRules = append(dnsRules, route(map[string]interface{}{"domain_suffix": settings.DirectDomains}, proxyClientDNSDirectTag))
	}
	if len(settings.ProxyDomains) > 0 {
		dnsRules = append(dnsRules, route(map[string]interface{}{"domain_suffix": settings.ProxyDomains}, proxyClientDNSRemoteTag))
	}
	switch settings.Mode {
	case ProxyClientModeRule:
		if ready[ruleSetGeositeCN] {
			dnsRules = append(dnsRules, route(map[string]interface{}{"rule_set": []string{ruleSetGeositeCN}}, proxyClientDNSDirectTag))
		}
		dnsRules = append(dnsRules, route(map[string]interface{}{}, proxyClientDNSRemoteTag))
	case ProxyClientModeGFW:
		if ready[ruleSetGeositeGFW] {
			dnsRules = append(dnsRules, route(map[string]interface{}{"rule_set": []string{ruleSetGeositeGFW}}, proxyClientDNSRemoteTag))
		}
		dnsRules = append(dnsRules, route(map[string]interface{}{}, proxyClientDNSDirectTag))
	default:
		dnsRules = append(dnsRules, route(map[string]interface{}{}, proxyClientDNSRemoteTag))
	}
	return rules, dnsRules
}

func proxyClientDNSServers(settings ProxyClientSettings) ([]interface{}, error) {
	direct, err := proxyClientDNSServer(settings.DirectDNS, proxyClientDNSDirectTag, "")
	if err != nil {
		return nil, err
	}
	remote, err := proxyClientDNSServer(settings.RemoteDNS, proxyClientDNSRemoteTag, proxyClientProxyTag)
	if err != nil {
		return nil, err
	}
	if remote["type"] == "local" {
		// The system resolver cannot be sent through the proxy.
		remote, _ = proxyClientDNSServer(proxyClientDefaultRemote, proxyClientDNSRemoteTag, proxyClientProxyTag)
	}
	return []interface{}{direct, remote}, nil
}

// mergeProxyClientRoute puts the client's rules before the panel's own and
// adds its rule sets. With TUN, sing-box must bind its own connections to
// the real interface, or they would loop back into the TUN.
func mergeProxyClientRoute(raw json.RawMessage, rules, ruleSets []interface{}, tun bool) (json.RawMessage, error) {
	route := map[string]interface{}{}
	if trimmed := strings.TrimSpace(string(raw)); trimmed != "" && trimmed != "null" {
		if err := json.Unmarshal(raw, &route); err != nil {
			return nil, err
		}
	}
	existing, _ := route["rules"].([]interface{})
	route["rules"] = append(append([]interface{}{}, rules...), existing...)
	if len(ruleSets) > 0 {
		current, _ := route["rule_set"].([]interface{})
		route["rule_set"] = append(current, ruleSets...)
	}
	if tun {
		route["auto_detect_interface"] = true
	}
	return json.Marshal(route)
}

func mergeProxyClientDNS(raw json.RawMessage, servers, rules []interface{}) (json.RawMessage, error) {
	dns := map[string]interface{}{}
	if trimmed := strings.TrimSpace(string(raw)); trimmed != "" && trimmed != "null" {
		if err := json.Unmarshal(raw, &dns); err != nil {
			return nil, err
		}
	}
	existingServers, _ := dns["servers"].([]interface{})
	for _, server := range existingServers {
		if item, ok := server.(map[string]interface{}); ok {
			if tag, _ := item["tag"].(string); tag == proxyClientDNSDirectTag || tag == proxyClientDNSRemoteTag {
				return nil, fmt.Errorf("DNS server tag %s is already used", tag)
			}
		}
	}
	if len(existingServers) == 0 {
		// Without servers of its own the panel used the system resolver;
		// keep that as the default rather than the client's servers.
		existingServers = []interface{}{map[string]interface{}{"type": "local", "tag": "local"}}
		if _, set := dns["final"]; !set {
			dns["final"] = "local"
		}
	}
	dns["servers"] = append(existingServers, servers...)
	existingRules, _ := dns["rules"].([]interface{})
	dns["rules"] = append(append([]interface{}{}, rules...), existingRules...)
	return json.Marshal(dns)
}

// proxyClientConfigNodes is for tests: the node outbounds of a config.
func proxyClientConfigNodes(config *SingBoxConfig) []model.ProxyClientNode {
	var nodes []model.ProxyClientNode
	for _, raw := range config.Outbounds {
		var outbound struct {
			Tag string `json:"tag"`
		}
		if json.Unmarshal(raw, &outbound) == nil && strings.HasPrefix(outbound.Tag, proxyClientNodePrefix) {
			nodes = append(nodes, model.ProxyClientNode{Name: outbound.Tag})
		}
	}
	return nodes
}
