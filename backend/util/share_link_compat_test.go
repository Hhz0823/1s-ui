package util

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/database/model"
)

func mustOutbound(t *testing.T, link string) map[string]interface{} {
	t.Helper()
	outbound, tag, err := GetOutbound(link, 0)
	if err != nil {
		t.Fatalf("import %q: %v", link, err)
	}
	if tag == "" || (*outbound)["tag"] != tag {
		t.Fatalf("import %q returned tag %q / %#v", link, tag, (*outbound)["tag"])
	}
	return *outbound
}

func selfSignedTLSModel(t *testing.T, serverName string) *model.Tls {
	t.Helper()
	_, certificate, err := GenerateSelfSignedTLS(serverName, time.Now(), time.Now().AddDate(1, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(certificate)), "\n")
	server, _ := json.Marshal(map[string]interface{}{"enabled": true, "server_name": serverName, "certificate": lines})
	client, _ := json.Marshal(map[string]interface{}{
		"certificate":                    lines,
		"pinned_peer_certificate_sha256": []string{CertSha256Base64(string(certificate))},
	})
	return &model.Tls{Id: 1, Name: "auto-test", Server: server, Client: client}
}

func TestShadowsocksLinkUsesURLSafeSIP002(t *testing.T) {
	// Chosen so that standard base64 of "method:server:user" contains "/" and "+".
	const serverPSK = "/////////////////////w=="
	const userPSK = "+++++++++++++++++++++w=="
	links := shadowsocksLink(
		map[string]map[string]interface{}{"shadowsocks16": {"password": userPSK}},
		map[string]interface{}{"method": "2022-blake3-aes-128-gcm", "password": serverPSK},
		[]map[string]interface{}{{"server": "2001:db8::30", "server_port": float64(8388), "remark": "香港 SS 01"}},
	)
	if len(links) != 1 {
		t.Fatalf("links = %#v", links)
	}
	userInfo := strings.TrimPrefix(strings.SplitN(links[0], "@", 2)[0], "ss://")
	if strings.ContainsAny(userInfo, "/+=") {
		t.Fatalf("SIP002 userinfo must be URL-safe base64: %q", links[0])
	}
	if !strings.Contains(links[0], "@[2001:db8::30]:8388#") {
		t.Fatalf("IPv6 host is not bracketed: %q", links[0])
	}
	if _, err := url.Parse(links[0]); err != nil {
		t.Fatalf("generated link is not a valid URL: %v", err)
	}
	outbound := mustOutbound(t, links[0])
	if outbound["method"] != "2022-blake3-aes-128-gcm" || outbound["password"] != serverPSK+":"+userPSK {
		t.Fatalf("round trip lost credentials: %#v", outbound)
	}
	if outbound["server"] != "2001:db8::30" || outbound["server_port"] != 8388 || outbound["tag"] != "香港 SS 01" {
		t.Fatalf("round trip lost endpoint: %#v", outbound)
	}
}

func TestShadowsocksImportAcceptsCommonForms(t *testing.T) {
	plain := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pa/ss+word"))
	legacy := base64.StdEncoding.EncodeToString([]byte("chacha20-ietf-poly1305:secret@198.51.100.7:8443"))
	tests := map[string]struct{ method, password, server string }{
		"ss://" + plain + "@198.51.100.7:8443#std":                                {"aes-256-gcm", "pa/ss+word", "198.51.100.7"},
		"ss://2022-blake3-aes-256-gcm:a%2Bb%3D%3Ac%2Fd@198.51.100.7:8443/?#plain": {"2022-blake3-aes-256-gcm", "a+b=:c/d", "198.51.100.7"},
		"ss://" + legacy + "#legacy":                                              {"chacha20-ietf-poly1305", "secret", "198.51.100.7"},
	}
	for link, want := range tests {
		outbound := mustOutbound(t, link)
		if outbound["method"] != want.method || outbound["password"] != want.password || outbound["server"] != want.server {
			t.Fatalf("import %q = %#v", link, outbound)
		}
	}
}

func TestSocksLinkUsesV2rayNFormat(t *testing.T) {
	links := socksLink(
		map[string]interface{}{"username": "user-1", "password": "p@ss:word/+"},
		[]map[string]interface{}{{"server": "198.51.100.8", "server_port": float64(1080), "remark": "socks node"}},
	)
	if len(links) != 1 || !strings.HasPrefix(links[0], "socks://") {
		t.Fatalf("links = %#v", links)
	}
	parsed, err := url.Parse(links[0])
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parsed.User.Username())
	if err != nil || string(decoded) != "user-1:p@ss:word/+" {
		t.Fatalf("v2rayN userinfo = %q (%v)", decoded, err)
	}
	if parsed.Fragment != "socks node" {
		t.Fatalf("remark = %q", parsed.Fragment)
	}
	outbound := mustOutbound(t, links[0])
	if outbound["username"] != "user-1" || outbound["password"] != "p@ss:word/+" || outbound["version"] != "5" {
		t.Fatalf("round trip = %#v", outbound)
	}
}

func TestSocksImportAcceptsCommonForms(t *testing.T) {
	legacy := base64.StdEncoding.EncodeToString([]byte("alice:secret@198.51.100.9:7890"))
	tests := map[string][3]string{
		"socks5://alice:secret@198.51.100.9:7890#s5":        {"alice", "secret", "5"},
		"socks5h://alice:secret@198.51.100.9:7890":          {"alice", "secret", "5"},
		"socks://" + legacy + "?remarks=legacy":             {"alice", "secret", "5"},
		"socks4://198.51.100.9:7890":                        {"", "", "4"},
		"socks://YWxpY2U6c2VjcmV0@198.51.100.9:7890#v2rayn": {"alice", "secret", "5"},
	}
	for link, want := range tests {
		outbound := mustOutbound(t, link)
		username, _ := outbound["username"].(string)
		password, _ := outbound["password"].(string)
		if username != want[0] || password != want[1] || outbound["version"] != want[2] ||
			outbound["server"] != "198.51.100.9" || outbound["server_port"] != 7890 {
			t.Fatalf("import %q = %#v", link, outbound)
		}
	}
}

func TestHTTPProxyImport(t *testing.T) {
	outbound := mustOutbound(t, "https://bob:pw@proxy.example.com:8443#https")
	tlsConfig, _ := outbound["tls"].(map[string]interface{})
	if outbound["type"] != "http" || outbound["username"] != "bob" || outbound["password"] != "pw" ||
		outbound["server_port"] != 8443 || tlsConfig["enabled"] != true || tlsConfig["server_name"] != "proxy.example.com" {
		t.Fatalf("https proxy import = %#v", outbound)
	}
}

func TestHysteria2ImportPortHoppingAndPins(t *testing.T) {
	link := "hysteria2://user:pa%2Fss@example.com:443,20000-30000/?sni=cdn.example.com&obfs=salamander&obfs-password=ob&pinSHA256=000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f&fastopen=1&mport=40000-40010#hy2"
	outbound := mustOutbound(t, link)
	if outbound["server"] != "example.com" || outbound["server_port"] != 443 || outbound["password"] != "user:pa/ss" {
		t.Fatalf("hy2 endpoint/auth = %#v", outbound)
	}
	ports, _ := outbound["server_ports"].([]string)
	if strings.Join(ports, ",") != "443:443,20000:30000,40000:40010" {
		t.Fatalf("server_ports = %#v", outbound["server_ports"])
	}
	if _, exists := outbound["fastopen"]; exists {
		t.Fatalf("fastopen is not a sing-box hysteria2 option: %#v", outbound)
	}
	tlsConfig := outbound["tls"].(map[string]interface{})
	if _, exists := tlsConfig["pinned_peer_certificate_sha256"]; exists {
		t.Fatalf("sing-box rejects pinned_peer_certificate_sha256: %#v", tlsConfig)
	}
	if tlsConfig["insecure"] != true || tlsConfig["server_name"] != "cdn.example.com" {
		t.Fatalf("pinned self-signed hy2 must stay usable: %#v", tlsConfig)
	}
	obfs := outbound["obfs"].(map[string]interface{})
	if obfs["type"] != "salamander" || obfs["password"] != "ob" {
		t.Fatalf("obfs = %#v", obfs)
	}
}

func TestVmessImportHandlesStringPortsAndSlashBase64(t *testing.T) {
	for attempt := 0; attempt < 64; attempt++ {
		payload, _ := json.Marshal(map[string]interface{}{
			"v": "2", "ps": strings.Repeat("?", attempt) + "vm", "add": "198.51.100.11", "port": "443",
			"id": "00000000-0000-4000-8000-000000000000", "aid": "0", "net": "ws", "type": "none",
			"host": "cdn.example.com", "path": "/ws?ed=2048", "tls": "tls", "sni": "cdn.example.com",
			"allowInsecure": false,
		})
		encoded := base64.StdEncoding.EncodeToString(payload)
		if !strings.Contains(encoded, "/") {
			continue
		}
		outbound := mustOutbound(t, "vmess://"+encoded)
		transport := outbound["transport"].(map[string]interface{})
		tlsConfig := outbound["tls"].(map[string]interface{})
		if outbound["server_port"] != 443 || transport["type"] != "ws" || transport["path"] != "/ws" ||
			transport["max_early_data"] != 2048 || tlsConfig["insecure"] != nil || tlsConfig["enabled"] != true {
			t.Fatalf("vmess import = %#v", outbound)
		}
		return
	}
	t.Fatal("could not build a vmess payload whose base64 contains '/'")
}

func TestTrojanImportDefaultsToTLSAndPeer(t *testing.T) {
	outbound := mustOutbound(t, "trojan://secret@198.51.100.12:443?peer=trojan.example.com&allowInsecure=1#t")
	tlsConfig := outbound["tls"].(map[string]interface{})
	if tlsConfig["enabled"] != true || tlsConfig["server_name"] != "trojan.example.com" || tlsConfig["insecure"] != true {
		t.Fatalf("trojan tls = %#v", tlsConfig)
	}
}

func TestImportRejectsXrayOnlyTransports(t *testing.T) {
	if _, _, err := GetOutbound("vless://00000000-0000-4000-8000-000000000000@198.51.100.13:443?security=tls&type=xhttp#x", 0); err == nil {
		t.Fatal("xhttp must be reported as unsupported instead of silently becoming TCP")
	}
}

func TestVLESSFlowOnlyOnRawTCP(t *testing.T) {
	tlsConfig := map[string]interface{}{"enabled": true, "server_name": "example.com"}
	addrs := []map[string]interface{}{{"server": "2001:db8::31", "server_port": float64(443), "remark": "v", "tls": tlsConfig}}
	user := map[string]interface{}{"uuid": "00000000-0000-4000-8000-000000000000", "flow": "xtls-rprx-vision"}

	wsLink := vlessLink(user, map[string]interface{}{"transport": map[string]interface{}{"type": "ws", "path": "/"}}, addrs)[0]
	tcpLink := vlessLink(user, map[string]interface{}{}, addrs)[0]
	wsURL, err := url.Parse(wsLink)
	if err != nil {
		t.Fatalf("IPv6 VLESS link must be a valid URL: %v", err)
	}
	tcpURL, _ := url.Parse(tcpLink)
	if wsURL.Query().Get("flow") != "" {
		t.Fatalf("Vision must not be exported for WebSocket: %q", wsLink)
	}
	if tcpURL.Query().Get("flow") != "xtls-rprx-vision" {
		t.Fatalf("Vision missing on raw TCP: %q", tcpLink)
	}
	if wsURL.Hostname() != "2001:db8::31" {
		t.Fatalf("unexpected host in %q", wsLink)
	}
}

func TestGeneratedSelfSignedNodesCarryClientCompatibilityFlags(t *testing.T) {
	tlsModel := selfSignedTLSModel(t, "198.51.100.14")
	client := json.RawMessage(`{
		"vless":{"uuid":"00000000-0000-4000-8000-000000000000","flow":"xtls-rprx-vision"},
		"trojan":{"password":"tp"},"hysteria2":{"password":"hp"},
		"tuic":{"uuid":"00000000-0000-4000-8000-000000000000","password":"tp"},
		"anytls":{"password":"ap"},"vmess":{"uuid":"00000000-0000-4000-8000-000000000000"}}`)
	tests := map[string]struct {
		options string
		flag    string
	}{
		"vless":     {`{"listen_port":443}`, "allowInsecure"},
		"trojan":    {`{"listen_port":443}`, "allowInsecure"},
		"hysteria2": {`{"listen_port":443,"obfs":{"type":"salamander","password":"o"}}`, "insecure"},
		"tuic":      {`{"listen_port":443,"congestion_control":"cubic"}`, "insecure"},
		"anytls":    {`{"listen_port":443}`, "insecure"},
	}
	for inboundType, test := range tests {
		inbound := &model.Inbound{
			Type: inboundType, Tag: inboundType + "-node", CoreType: model.CoreTypeSingBox, TlsId: 1, Tls: tlsModel,
			Addrs: json.RawMessage(`[]`), OutJson: json.RawMessage(`{}`), Options: json.RawMessage(test.options),
		}
		links := LinkGenerator(client, inbound, "198.51.100.14", "")
		if len(links) != 1 {
			t.Fatalf("%s links = %#v", inboundType, links)
		}
		parsed, err := url.Parse(links[0])
		if err != nil {
			t.Fatalf("%s link invalid: %v", inboundType, err)
		}
		if parsed.Query().Get(test.flag) != "1" {
			t.Fatalf("%s link lacks %s=1 for a self-signed certificate: %q", inboundType, test.flag, links[0])
		}
		if parsed.Query().Get("pcs") == "" && parsed.Query().Get("pinSHA256") == "" {
			t.Fatalf("%s link dropped the certificate pin: %q", inboundType, links[0])
		}
		outbound := mustOutbound(t, links[0])
		if _, exists := outbound["tls"].(map[string]interface{})["pinned_peer_certificate_sha256"]; exists {
			t.Fatalf("%s import kept an option sing-box rejects: %#v", inboundType, outbound)
		}
	}

	vmessInbound := &model.Inbound{
		Type: "vmess", Tag: "vmess-node", CoreType: model.CoreTypeSingBox, TlsId: 1, Tls: tlsModel,
		Addrs: json.RawMessage(`[]`), OutJson: json.RawMessage(`{}`),
		Options: json.RawMessage(`{"listen_port":443,"transport":{"type":"ws","path":"/"}}`),
	}
	links := LinkGenerator(client, vmessInbound, "198.51.100.14", "")
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(links[0], "vmess://"))
	if err != nil {
		t.Fatal(err)
	}
	var vmess map[string]interface{}
	if err = json.Unmarshal(raw, &vmess); err != nil {
		t.Fatal(err)
	}
	if vmess["allowInsecure"] != true || vmess["net"] != "ws" || vmess["tls"] != "tls" || vmess["port"] != "443" {
		t.Fatalf("vmess share = %#v", vmess)
	}
}

func TestTrustedCertificateLinksStayStrict(t *testing.T) {
	tlsModel := &model.Tls{
		Id:     1,
		Server: json.RawMessage(`{"enabled":true,"server_name":"node.example.com","certificate_path":"/nonexistent/fullchain.pem"}`),
		Client: json.RawMessage(`{}`),
	}
	inbound := &model.Inbound{
		Type: "trojan", Tag: "trojan-acme", CoreType: model.CoreTypeSingBox, TlsId: 1, Tls: tlsModel,
		Addrs: json.RawMessage(`[]`), OutJson: json.RawMessage(`{}`), Options: json.RawMessage(`{"listen_port":443}`),
	}
	links := LinkGenerator(json.RawMessage(`{"trojan":{"password":"tp"}}`), inbound, "node.example.com", "")
	if len(links) != 1 || strings.Contains(links[0], "allowInsecure") || strings.Contains(links[0], "insecure=1") {
		t.Fatalf("CA-issued certificates must not disable verification: %#v", links)
	}
}

func TestHTTPTransportUsesH2UnderTLS(t *testing.T) {
	inbound := map[string]interface{}{"transport": map[string]interface{}{"type": "http", "path": "/h2", "host": []interface{}{"h2.example.com"}}}
	addrs := []map[string]interface{}{{
		"server": "198.51.100.15", "server_port": float64(443), "remark": "h2",
		"tls": map[string]interface{}{"enabled": true, "server_name": "h2.example.com"},
	}}
	parsed, _ := url.Parse(vlessLink(map[string]interface{}{"uuid": "00000000-0000-4000-8000-000000000000"}, inbound, addrs)[0])
	if parsed.Query().Get("type") != "http" || parsed.Query().Get("headerType") != "" {
		t.Fatalf("TLS HTTP transport must be exported as h2: %q", parsed.String())
	}
	raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(vmessLink(map[string]interface{}{"uuid": "x"}, inbound, addrs)[0], "vmess://"))
	var vmess map[string]interface{}
	_ = json.Unmarshal(raw, &vmess)
	if vmess["net"] != "h2" {
		t.Fatalf("vmess h2 net = %#v", vmess)
	}
}

func TestHysteria2LinkConvertsPortHoppingForClients(t *testing.T) {
	links := hysteria2Link(
		map[string]interface{}{"password": "p"},
		map[string]interface{}{"out_json": json.RawMessage(`{"server_ports":["20000:30000"]}`)},
		[]map[string]interface{}{{"server": "example.com", "server_port": float64(443), "remark": "hop"}},
	)
	parsed, _ := url.Parse(links[0])
	if parsed.Query().Get("mport") != "20000-30000" {
		t.Fatalf("mport = %q", parsed.Query().Get("mport"))
	}
	outbound := mustOutbound(t, links[0])
	if ports, _ := outbound["server_ports"].([]string); strings.Join(ports, ",") != "20000:30000" {
		t.Fatalf("round trip server_ports = %#v", outbound["server_ports"])
	}
}

func TestNormalizeSingBoxOutboundKeepsEmbeddedCertificate(t *testing.T) {
	outbound := map[string]interface{}{
		"type": "trojan",
		"tls": map[string]interface{}{
			"enabled": true, "certificate": []interface{}{"-----BEGIN CERTIFICATE-----", "x", "-----END CERTIFICATE-----"},
			"pinned_peer_certificate_sha256": []interface{}{"abc"},
		},
		"transport": map[string]interface{}{},
	}
	NormalizeSingBoxOutbound(outbound)
	tlsConfig := outbound["tls"].(map[string]interface{})
	if _, exists := tlsConfig["pinned_peer_certificate_sha256"]; exists || tlsConfig["insecure"] != nil {
		t.Fatalf("embedded certificate should be trusted without insecure: %#v", tlsConfig)
	}
	if _, exists := outbound["transport"]; exists {
		t.Fatalf("empty transport should be removed: %#v", outbound)
	}
}

// A 2022 inbound whose users carry no key of its size is single-user: the
// link must hold the server key alone, never "method:server:" with an empty
// user key, which no client can use.
func TestShadowsocks2022SingleUserLinkHasNoEmptyUserKey(t *testing.T) {
	const serverPSK = "EVFx4BzsbMO3FGb2xGB6lA=="
	links := shadowsocksLink(
		map[string]map[string]interface{}{"shadowsocks": {"password": serverPSK}},
		map[string]interface{}{"method": "2022-blake3-aes-128-gcm", "password": serverPSK},
		[]map[string]interface{}{{"server": "192.0.2.2", "server_port": float64(24288), "remark": "ss"}},
	)
	userInfo := strings.TrimPrefix(strings.SplitN(links[0], "@", 2)[0], "ss://")
	decoded, err := base64.RawURLEncoding.DecodeString(userInfo)
	if err != nil || string(decoded) != "2022-blake3-aes-128-gcm:"+serverPSK {
		t.Fatalf("userinfo %q (%v)", decoded, err)
	}
	if outbound := mustOutbound(t, links[0]); outbound["password"] != serverPSK {
		t.Fatalf("password %q", outbound["password"])
	}
}
