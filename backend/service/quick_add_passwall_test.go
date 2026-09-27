package service

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// PassWall and PassWall 2 (OpenWrt) read vless:// links in two places:
//   - subscriptions: root/usr/share/passwall{,2}/subscribe.lua
//   - pasting a link on the node page: luasrc/view/passwall{,2}/
//     node_config/link_share_man.htm (JavaScript)
//
// Both keep the fields below the same way in PassWall and PassWall 2, and
// luasrc/passwall{,2}/util_xray.lua builds the Xray outbound from them. XHTTP
// links always go to Xray-core, which PassWall installs by default.
type passwallNode struct {
	Address, Port, UUID                 string
	Transport                           string
	XHTTPPath, XHTTPHost, XHTTPMode     string
	Encryption, Flow                    string
	TLS, Reality                        bool
	ServerName, Fingerprint, PinSHA256  string
	RealityKey, RealityShortID, SpiderX string
}

// passwallSplit mirrors api.split: empty pieces are kept except a trailing one.
func passwallSplit(value, separator string) []string {
	parts := strings.Split(value, separator)
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// parsePassWallSubscriptionVless follows the vless branch of subscribe.lua.
func parsePassWallSubscriptionVless(t *testing.T, link string) passwallNode {
	t.Helper()
	content := strings.TrimPrefix(link, "vless://")
	if index := strings.Index(content, "#"); index >= 0 {
		content = content[:index]
	}
	info := passwallSplit(content, "@")
	if len(info) < 2 {
		t.Fatalf("PassWall needs uuid@host: %s", link)
	}
	uuid, _ := url.QueryUnescape(info[0])
	query := passwallSplit(strings.ReplaceAll(info[1], "/?", "?"), "?")
	params := map[string]string{}
	if len(query) > 1 {
		for _, pair := range passwallSplit(query[1], "&") {
			if index := strings.Index(pair, "="); index > 0 {
				value, err := url.QueryUnescape(pair[index+1:])
				if err != nil {
					t.Fatalf("PassWall cannot decode %q: %s", pair, link)
				}
				params[pair[:index]] = value
			}
		}
	}
	node := passwallNode{UUID: uuid, Port: "443"}
	hostPort := query[0]
	if index := strings.LastIndex(hostPort, ":"); index >= 0 {
		node.Port = hostPort[index+1:]
		node.Address = strings.TrimSuffix(strings.TrimPrefix(hostPort[:index], "["), "]")
	} else {
		node.Address = hostPort
	}

	transport := strings.ToLower(params["type"])
	if transport == "" || transport == "tcp" {
		transport = "raw"
	}
	node.Transport = transport
	if transport == "xhttp" || transport == "splithttp" {
		node.Transport = "xhttp"
		node.XHTTPHost, node.XHTTPPath, node.XHTTPMode = params["host"], params["path"], params["mode"]
		if node.XHTTPMode == "" {
			node.XHTTPMode = "auto"
		}
	}
	node.Encryption = params["encryption"]
	if node.Encryption == "" {
		node.Encryption = "none"
	}
	node.Flow = params["flow"]

	security := params["security"]
	if security == "" && node.Flow != "" {
		security = "tls"
	}
	if security == "tls" || security == "reality" {
		node.TLS = true
		node.ServerName = params["sni"]
		if node.ServerName == "" {
			node.ServerName = params["host"]
		}
		node.Fingerprint = params["fp"]
		node.PinSHA256 = params["pcs"]
		if security == "reality" {
			node.Reality = true
			node.RealityKey, node.RealityShortID, node.SpiderX = params["pbk"], params["sid"], params["spx"]
		}
	}
	return node
}

// parsePassWallWebVless follows the vless branch of link_share_man.htm. It
// differs from the subscription parser in two ways that matter here: a value
// ends at its second "=", and flow is only read for TLS or REALITY links.
func parsePassWallWebVless(t *testing.T, link string) passwallNode {
	t.Helper()
	parsed, err := url.Parse("http" + strings.TrimPrefix(link, "vless"))
	if err != nil {
		t.Fatalf("PassWall's URL() rejects %s: %v", link, err)
	}
	params := map[string]string{}
	search := strings.SplitN(strings.Replace("?"+parsed.RawQuery, "/?", "?", 1), "?", 3)
	if len(search) > 1 {
		for _, pair := range strings.Split(search[1], "&") {
			pieces := strings.Split(pair, "=")
			key, _ := url.PathUnescape(pieces[0])
			value := ""
			if len(pieces) > 1 {
				value, _ = url.PathUnescape(pieces[1])
			}
			params[key] = value
		}
	}
	if _, ok := params["type"]; !ok {
		t.Fatalf("PassWall's link page throws on a link without type: %s", link)
	}
	node := passwallNode{
		UUID: parsed.User.Username(), Address: parsed.Hostname(), Port: parsed.Port(),
		Encryption: params["encryption"],
	}
	if node.Port == "" {
		node.Port = "443"
	}
	if node.Encryption == "" {
		node.Encryption = "none"
	}
	security := params["security"]
	if security == "" && params["flow"] != "" {
		security = "tls"
	}
	if security == "tls" || security == "reality" {
		node.TLS, node.Reality = true, security == "reality"
		node.Flow = params["flow"]
		node.ServerName, node.Fingerprint = params["sni"], params["fp"]
		if security == "tls" {
			node.PinSHA256 = params["pcs"]
		} else {
			node.RealityKey, node.RealityShortID, node.SpiderX = params["pbk"], params["sid"], params["spx"]
		}
	}
	node.Transport = strings.ToLower(params["type"])
	if node.Transport == "tcp" {
		node.Transport = "raw"
	}
	if node.Transport == "xhttp" {
		node.XHTTPHost, node.XHTTPPath, node.XHTTPMode = params["host"], params["path"], params["mode"]
		if node.XHTTPMode == "" {
			node.XHTTPMode = "auto"
		}
	}
	return node
}

// checkVlessLinkForPassWall imports a quick-add link with both PassWall
// parsers and returns the node of each one that keeps everything the node
// needs. The link page ignores flow on security=none links, so VLESS
// Encryption + Vision nodes without TLS or REALITY import by subscription
// only; every other node must import both ways.
func checkVlessLinkForPassWall(t *testing.T, link string) map[string]passwallNode {
	t.Helper()
	query := checkVlessLinkForClients(t, link)
	nodes := map[string]passwallNode{
		"subscription": parsePassWallSubscriptionVless(t, link),
		"link page":    parsePassWallWebVless(t, link),
	}
	for name, node := range nodes {
		if node.Encryption != query.Get("encryption") {
			t.Fatalf("PassWall %s encryption = %q: %s", name, node.Encryption, link)
		}
		if node.Transport != map[string]string{"tcp": "raw", "xhttp": "xhttp"}[query.Get("type")] {
			t.Fatalf("PassWall %s transport = %q: %s", name, node.Transport, link)
		}
		if node.Transport == "xhttp" && (node.XHTTPPath != query.Get("path") || node.XHTTPMode != query.Get("mode")) {
			t.Fatalf("PassWall %s XHTTP = %q %q: %s", name, node.XHTTPPath, node.XHTTPMode, link)
		}
		if node.Reality && (node.RealityKey != query.Get("pbk") || node.RealityShortID != query.Get("sid") || node.ServerName != query.Get("sni")) {
			t.Fatalf("PassWall %s REALITY settings differ: %+v %s", name, node, link)
		}
		if query.Get("security") == "tls" && node.PinSHA256 != query.Get("pcs") {
			t.Fatalf("PassWall %s pin = %q: %s", name, node.PinSHA256, link)
		}
		if node.Flow != query.Get("flow") {
			if name == "link page" && query.Get("security") == "none" {
				delete(nodes, name)
				continue
			}
			t.Fatalf("PassWall %s flow = %q, link has %q: %s", name, node.Flow, query.Get("flow"), link)
		}
	}
	return nodes
}

// passwallXrayOutbound is the Xray outbound util_xray.lua writes for a node:
// the flat VLESS settings, "method" for the network, a pinned certificate
// instead of allowInsecure and chrome as the default REALITY fingerprint.
func passwallXrayOutbound(node passwallNode) map[string]interface{} {
	orDefault := func(value, fallback string) string {
		if value == "" {
			return fallback
		}
		return value
	}
	port, _ := strconv.Atoi(node.Port)
	settings := map[string]interface{}{
		"address": node.Address, "port": port, "id": node.UUID,
		"encryption": orDefault(node.Encryption, "none"),
	}
	if (node.TLS || orDefault(node.Encryption, "none") != "none") && node.Flow != "" {
		settings["flow"] = node.Flow
	}
	stream := map[string]interface{}{"method": node.Transport, "security": "none"}
	switch {
	case node.Reality:
		stream["security"] = "reality"
		stream["realitySettings"] = map[string]interface{}{
			"serverName": node.ServerName, "publicKey": node.RealityKey, "shortId": node.RealityShortID,
			"spiderX": orDefault(node.SpiderX, "/"), "fingerprint": orDefault(node.Fingerprint, "chrome"),
		}
	case node.TLS:
		tlsSettings := map[string]interface{}{
			"serverName": node.ServerName, "pinnedPeerCertSha256": node.PinSHA256, "verifyPeerCertByName": "",
		}
		if node.Fingerprint != "" {
			tlsSettings["fingerprint"] = node.Fingerprint
		}
		stream["security"] = "tls"
		stream["tlsSettings"] = tlsSettings
	}
	if node.Transport == "xhttp" {
		xhttp := map[string]interface{}{"mode": orDefault(node.XHTTPMode, "auto"), "path": orDefault(node.XHTTPPath, "/")}
		if node.XHTTPHost != "" {
			xhttp["host"] = node.XHTTPHost
		}
		stream["xhttpSettings"] = xhttp
	}
	return map[string]interface{}{"protocol": "vless", "tag": "proxy", "settings": settings, "streamSettings": stream}
}
