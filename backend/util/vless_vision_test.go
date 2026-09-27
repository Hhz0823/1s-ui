package util

import (
	"net/url"
	"testing"
)

// TestXrayVlessLinkExportsVisionWhereXrayRunsIt covers every place a VLESS
// user's Vision flow can end up: it is exported where Xray-core runs Vision,
// including XHTTP under VLESS Encryption and REALITY, and nowhere else.
func TestXrayVlessLinkExportsVisionWhereXrayRunsIt(t *testing.T) {
	reality := map[string]interface{}{
		"enabled": true, "server_name": "www.example.com",
		"reality": map[string]interface{}{"enabled": true, "public_key": "pk", "short_id": "0123456789abcdef"},
	}
	for _, test := range []struct {
		name       string
		transport  string
		encryption bool
		tls        map[string]interface{}
		want       bool
	}{
		{"raw REALITY", "tcp", false, reality, true},
		{"raw VLESS Encryption", "tcp", true, nil, true},
		{"raw without security", "tcp", false, nil, false},
		{"XHTTP REALITY", "xhttp", false, reality, false},
		{"XHTTP VLESS Encryption", "xhttp", true, nil, false},
		{"XHTTP VLESS Encryption + REALITY", "xhttp", true, reality, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			inbound := map[string]interface{}{"transport": map[string]interface{}{"type": test.transport}}
			if test.transport == "xhttp" {
				inbound["transport"] = map[string]interface{}{"type": "xhttp", "path": "/p", "mode": "auto"}
			}
			if test.encryption {
				inbound["decryption"] = "mlkem768x25519plus.native.600s.server"
				inbound["encryption"] = "mlkem768x25519plus.native.0rtt.client"
			}
			addr := map[string]interface{}{"server": "example.com", "server_port": float64(443), "remark": "node"}
			if test.tls != nil {
				addr["tls"] = test.tls
			}
			links := xrayVlessLink(map[string]interface{}{
				"uuid": "00000000-0000-0000-0000-000000000000", "flow": "xtls-rprx-vision",
			}, inbound, []map[string]interface{}{addr})
			parsed, err := url.Parse(links[0])
			if err != nil {
				t.Fatal(err)
			}
			if got := parsed.Query().Get("flow") != ""; got != test.want {
				t.Fatalf("flow exported = %v, want %v: %s", got, test.want, links[0])
			}
			if got := XrayVlessVisionAllowed(test.transport, test.encryption, test.tls != nil); got != test.want {
				t.Fatalf("XrayVlessVisionAllowed = %v, want %v", got, test.want)
			}
		})
	}
}
