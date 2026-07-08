package util

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func testSha256Bytes() []byte {
	value := make([]byte, sha256DigestLength)
	for i := range value {
		value[i] = byte(i)
	}
	return value
}

func TestPinnedPeerCertSha256ForLink(t *testing.T) {
	raw := testSha256Bytes()
	base64Pin := base64.StdEncoding.EncodeToString(raw)
	hexPin := hex.EncodeToString(raw)
	v2rayNFailingBase64Pin := "MsMCXLvVYm6xpdOHirUNjBJT1GqVRoZb2zcltprnm9Y="
	v2rayNExpectedHexPin := "32c3025cbbd5626eb1a5d3878ab50d8c1253d46a9546865bdb3725b69ae79bd6"

	if got := pinnedPeerCertSha256ForLink(base64Pin); got != hexPin {
		t.Fatalf("base64 pin should be exported as hex, got %q", got)
	}
	if got := pinnedPeerCertSha256ForLink(v2rayNFailingBase64Pin); got != v2rayNExpectedHexPin {
		t.Fatalf("v2rayN/Xray pin should be exported as hex, got %q", got)
	}
	if got := pinnedPeerCertSha256ForLink(strings.ToUpper(hexPin)); got != hexPin {
		t.Fatalf("hex pin should be normalized to lowercase, got %q", got)
	}
	if got := pinnedPeerCertSha256ForLink("not-a-sha256-pin"); got != "not-a-sha256-pin" {
		t.Fatalf("unknown pin format should pass through, got %q", got)
	}
}

func TestPinnedPeerCertSha256ForConfig(t *testing.T) {
	raw := testSha256Bytes()
	base64Pin := base64.StdEncoding.EncodeToString(raw)
	hexPin := hex.EncodeToString(raw)

	if got := pinnedPeerCertSha256ForConfig(hexPin); got != base64Pin {
		t.Fatalf("hex pin should be stored as base64, got %q", got)
	}
	if got := pinnedPeerCertSha256ForConfig(base64Pin); got != base64Pin {
		t.Fatalf("base64 pin should stay base64, got %q", got)
	}
}

func TestHysteria2LinkExportsHexPinOnly(t *testing.T) {
	raw := testSha256Bytes()
	base64Pin := base64.StdEncoding.EncodeToString(raw)
	hexPin := hex.EncodeToString(raw)
	inbound := map[string]interface{}{
		"out_json": json.RawMessage(`{}`),
	}
	addrs := []map[string]interface{}{
		{
			"server":      "example.com",
			"server_port": float64(443),
			"remark":      "hy2",
			"tls": map[string]interface{}{
				"enabled": true,
				"pinned_peer_certificate_sha256": []interface{}{
					base64Pin,
				},
			},
		},
	}

	links := hysteria2Link(map[string]interface{}{"password": "secret"}, inbound, addrs)
	if len(links) != 1 {
		t.Fatalf("expected one link, got %d", len(links))
	}
	parsed, err := url.Parse(links[0])
	if err != nil {
		t.Fatalf("parse generated link: %v", err)
	}
	query := parsed.Query()
	if got := query.Get("pinSHA256"); got != hexPin {
		t.Fatalf("pinSHA256 should be hex for v2rayN/Xray, got %q", got)
	}
	if got := query.Get("pcs"); got != "" {
		t.Fatalf("hysteria2 link should not include sing-box pcs param, got %q", got)
	}
}

func TestXrayVlessLinkExportsHexPinSHA256(t *testing.T) {
	raw := testSha256Bytes()
	base64Pin := base64.StdEncoding.EncodeToString(raw)
	hexPin := hex.EncodeToString(raw)
	inbound := map[string]interface{}{
		"transport": map[string]interface{}{
			"type": "xhttp",
			"path": "/xhttp",
			"mode": "auto",
		},
	}
	addrs := []map[string]interface{}{
		{
			"server":      "example.com",
			"server_port": float64(443),
			"remark":      "xray-vless",
			"tls": map[string]interface{}{
				"enabled": true,
				"pinned_peer_certificate_sha256": []interface{}{
					base64Pin,
				},
			},
		},
	}

	links := xrayVlessLink(map[string]interface{}{"uuid": "00000000-0000-0000-0000-000000000000"}, inbound, addrs)
	if len(links) != 1 {
		t.Fatalf("expected one link, got %d", len(links))
	}
	parsed, err := url.Parse(links[0])
	if err != nil {
		t.Fatalf("parse generated link: %v", err)
	}
	query := parsed.Query()
	if got := query.Get("pinSHA256"); got != hexPin {
		t.Fatalf("pinSHA256 should be hex for v2rayN/Xray, got %q", got)
	}
	if got := query.Get("pcs"); got != "" {
		t.Fatalf("xray link should not include raw pcs param, got %q", got)
	}
}

func TestXrayVmessLinkExportsHexPinSHA256(t *testing.T) {
	raw := testSha256Bytes()
	base64Pin := base64.StdEncoding.EncodeToString(raw)
	hexPin := hex.EncodeToString(raw)
	inbound := map[string]interface{}{
		"transport": map[string]interface{}{
			"type": "ws",
			"path": "/",
			"host": "example.com",
		},
	}
	addrs := []map[string]interface{}{
		{
			"server":      "example.com",
			"server_port": float64(443),
			"remark":      "xray-vmess",
			"tls": map[string]interface{}{
				"enabled": true,
				"pinned_peer_certificate_sha256": []interface{}{
					base64Pin,
				},
			},
		},
	}

	links := xrayVmessLink(map[string]interface{}{"uuid": "00000000-0000-0000-0000-000000000000"}, inbound, addrs)
	if len(links) != 1 {
		t.Fatalf("expected one link, got %d", len(links))
	}
	payload := strings.TrimPrefix(links[0], "vmess://")
	rawJSON, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode vmess payload: %v", err)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(rawJSON, &obj); err != nil {
		t.Fatalf("unmarshal vmess payload: %v", err)
	}
	if got, _ := obj["pinSHA256"].(string); got != hexPin {
		t.Fatalf("pinSHA256 should be hex for v2rayN/Xray, got %q", got)
	}
	if got, _ := obj["pcs"].(string); got != hexPin {
		t.Fatalf("pcs should be normalized hex for v2rayN/Xray, got %q", got)
	}
}
