package core

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/database/model"
	panelutil "github.com/Hhz0823/1s-ui/util"
	"github.com/sagernet/sing-box/option"
)

// panelShareLinks returns links produced by the panel itself for a node that
// uses the generated self-signed certificate, i.e. what users import into the
// outbound page of another 1S-UI server.
func panelShareLinks(t *testing.T) []string {
	t.Helper()
	_, certificate, err := panelutil.GenerateSelfSignedTLS("198.51.100.40", time.Now(), time.Now().AddDate(1, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(certificate)), "\n")
	server, _ := json.Marshal(map[string]interface{}{"enabled": true, "server_name": "198.51.100.40", "certificate": lines})
	client, _ := json.Marshal(map[string]interface{}{
		"certificate":                    lines,
		"pinned_peer_certificate_sha256": []string{panelutil.CertSha256Base64(string(certificate))},
	})
	tlsModel := &model.Tls{Id: 1, Name: "auto-test", Server: server, Client: client}
	user := json.RawMessage(`{
		"vless":{"uuid":"00000000-0000-4000-8000-000000000000","flow":"xtls-rprx-vision"},
		"vmess":{"uuid":"00000000-0000-4000-8000-000000000000"},
		"trojan":{"password":"tp"},"hysteria2":{"password":"hp"},"anytls":{"password":"ap"},
		"tuic":{"uuid":"00000000-0000-4000-8000-000000000000","password":"tp"},
		"socks":{"username":"u","password":"p"},"http":{"username":"u","password":"p"},
		"shadowsocks":{"password":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}}`)
	nodes := []struct {
		inboundType string
		options     string
		tls         bool
	}{
		{"vless", `{"listen_port":443}`, true},
		{"vless", `{"listen_port":443,"transport":{"type":"ws","path":"/"}}`, true},
		{"vmess", `{"listen_port":443,"transport":{"type":"ws","path":"/"}}`, true},
		{"trojan", `{"listen_port":443}`, true},
		{"hysteria2", `{"listen_port":443,"obfs":{"type":"salamander","password":"o"},"tcp_fast_open":true}`, true},
		{"tuic", `{"listen_port":443,"congestion_control":"cubic"}`, true},
		{"anytls", `{"listen_port":443}`, true},
		{"mixed", `{"listen_port":1080}`, false},
		{"shadowsocks", `{"listen_port":8388,"method":"2022-blake3-aes-256-gcm","password":"BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="}`, false},
	}
	var links []string
	for _, node := range nodes {
		inbound := &model.Inbound{
			Type: node.inboundType, Tag: node.inboundType, CoreType: model.CoreTypeSingBox,
			Addrs: json.RawMessage(`[]`), OutJson: json.RawMessage(`{}`), Options: json.RawMessage(node.options),
		}
		if node.tls {
			inbound.TlsId = 1
			inbound.Tls = tlsModel
		}
		generated := panelutil.LinkGenerator(user, inbound, "198.51.100.40", "")
		if len(generated) == 0 {
			t.Fatalf("no links for %s", node.inboundType)
		}
		links = append(links, generated...)
	}
	return links
}

func thirdPartyShareLinks() []string {
	vmess, _ := json.Marshal(map[string]interface{}{
		"v": "2", "ps": "vm", "add": "198.51.100.41", "port": "443", "id": "00000000-0000-4000-8000-000000000000",
		"aid": "0", "scy": "auto", "net": "grpc", "path": "grpc-svc", "tls": "tls", "sni": "vm.example.com",
	})
	legacySocks := base64.StdEncoding.EncodeToString([]byte("alice:secret@198.51.100.41:7890"))
	return []string{
		"socks5://alice:secret@198.51.100.41:7890#socks5",
		"socks://YWxpY2U6c2VjcmV0@198.51.100.41:7890#v2rayN-socks",
		"socks://" + legacySocks + "?remarks=legacy",
		"http://bob:pw@198.51.100.41:3128#http",
		"https://bob:pw@proxy.example.com:8443#https",
		"hysteria2://auth@hy2.example.com:443,20000-30000/?sni=hy2.example.com&obfs=salamander&obfs-password=o&insecure=0&pinSHA256=00:01:02:03:04:05:06:07:08:09:0a:0b:0c:0d:0e:0f:10:11:12:13:14:15:16:17:18:19:1a:1b:1c:1d:1e:1f&fastopen=1#hy2-hop",
		"hy2://user:pass@198.51.100.41:8443?insecure=1&mport=9000-9100#hy2-alias",
		"vmess://" + base64.StdEncoding.EncodeToString(vmess),
		"trojan://secret@198.51.100.41:443?peer=trojan.example.com&type=ws&path=%2Fws%3Fed%3D2048&host=trojan.example.com#trojan-ws",
		"vless://00000000-0000-4000-8000-000000000000@198.51.100.41:443?security=reality&pbk=Z84J2IelR9ch3k8VtlVhhs5ycBUlXA7wHBWcBrjqnAw&sid=6ba85179e30d4fc2&sni=www.microsoft.com&flow=xtls-rprx-vision-udp443&type=tcp#reality",
		"ss://" + base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:pa/ss+word")) + "@198.51.100.41:8388#ss",
		"tuic://00000000-0000-4000-8000-000000000000:pw@198.51.100.41:443?congestion_control=bbr&udp_relay_mode=native&allow_insecure=1#tuic",
		"anytls://pw@198.51.100.41:443?sni=anytls.example.com#anytls",
	}
}

func TestImportedShareLinksDecodeInSingBox(t *testing.T) {
	initNaiveTestLogger()
	ctx := (&Core{}).GetCtx()
	for _, link := range append(panelShareLinks(t), thirdPartyShareLinks()...) {
		outbound, _, err := panelutil.GetOutbound(link, 0)
		if err != nil {
			t.Fatalf("import %q: %v", link, err)
		}
		raw, err := json.Marshal(outbound)
		if err != nil {
			t.Fatal(err)
		}
		var decoded option.Outbound
		if err = decoded.UnmarshalJSONContext(ctx, raw); err != nil {
			t.Fatalf("sing-box rejected imported %q:\n%s\n%v", link, raw, err)
		}
	}
}

func TestImportedTCPShareLinksStartInSingBox(t *testing.T) {
	initNaiveTestLogger()
	outbounds := []interface{}{}
	for index, link := range append(panelShareLinks(t), thirdPartyShareLinks()...) {
		outbound, _, err := panelutil.GetOutbound(link, index+1)
		if err != nil {
			t.Fatalf("import %q: %v", link, err)
		}
		switch (*outbound)["type"] {
		case "hysteria2", "tuic":
			continue // QUIC protocols need the with_quic build tag
		}
		if tlsConfig, ok := (*outbound)["tls"].(map[string]interface{}); ok && (tlsConfig["utls"] != nil || tlsConfig["reality"] != nil) {
			continue // uTLS/Reality need the with_utls build tag
		}
		if transport, ok := (*outbound)["transport"].(map[string]interface{}); ok && transport["type"] == "grpc" {
			continue // gRPC needs the with_grpc build tag
		}
		outbounds = append(outbounds, *outbound)
	}
	outbounds = append(outbounds, map[string]interface{}{"type": "direct", "tag": "direct"})
	config, err := json.Marshal(map[string]interface{}{
		"outbounds": outbounds,
		"route":     map[string]interface{}{"final": "direct"},
	})
	if err != nil {
		t.Fatal(err)
	}
	instance := NewCore()
	if err = instance.Start(config); err != nil {
		t.Fatalf("start imported outbounds: %v\n%s", err, config)
	}
	if err = instance.Stop(); err != nil {
		t.Fatalf("stop imported outbounds: %v", err)
	}
}
