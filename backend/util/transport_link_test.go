package util

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestHTTPTransportLinksUseV2rayNHeaderShape(t *testing.T) {
	const uuid = "00000000-0000-4000-8000-000000000000"
	inbound := map[string]interface{}{
		"transport": map[string]interface{}{
			"type": "http",
			"path": "/http-test",
			"host": []interface{}{"example.com"},
		},
	}
	addrs := []map[string]interface{}{
		{
			"server":      "198.51.100.10",
			"server_port": float64(443),
			"remark":      "http-transport-test",
		},
	}

	for name, link := range map[string]string{
		"vless": vlessLink(map[string]interface{}{"uuid": uuid}, inbound, addrs)[0],
		"trojan": trojanLink(
			map[string]interface{}{"password": "test-password"},
			inbound,
			addrs,
		)[0],
	} {
		t.Run(name, func(t *testing.T) {
			parsed, err := url.Parse(link)
			if err != nil {
				t.Fatalf("parse generated link: %v", err)
			}
			query := parsed.Query()
			if got := query.Get("type"); got != "tcp" {
				t.Fatalf("unexpected transport type %q", got)
			}
			if got := query.Get("headerType"); got != "http" {
				t.Fatalf("unexpected header type %q", got)
			}
			if got := query.Get("host"); got != "example.com" {
				t.Fatalf("unexpected host %q", got)
			}
			if got := query.Get("path"); got != "/http-test" {
				t.Fatalf("unexpected path %q", got)
			}
		})
	}

	vmess := vmessLink(map[string]interface{}{"uuid": uuid}, inbound, addrs)[0]
	payload := strings.TrimPrefix(vmess, "vmess://")
	rawJSON, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode VMess payload: %v", err)
	}
	var vmessConfig map[string]interface{}
	if err := json.Unmarshal(rawJSON, &vmessConfig); err != nil {
		t.Fatalf("unmarshal VMess payload: %v", err)
	}
	if got := vmessConfig["net"]; got != "tcp" {
		t.Fatalf("unexpected VMess network %#v", got)
	}
	if got := vmessConfig["type"]; got != "http" {
		t.Fatalf("unexpected VMess header type %#v", got)
	}
}
