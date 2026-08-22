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

func TestHysteria2LinkEscapesPasswordForV2rayN(t *testing.T) {
	const password = "testAuth/with+symbols="
	inbound := map[string]interface{}{
		"out_json":      json.RawMessage(`{}`),
		"tcp_fast_open": false,
		"obfs": map[string]interface{}{
			"type":     "salamander",
			"password": "testObfs/with+symbols==",
		},
	}
	addrs := []map[string]interface{}{
		{
			"server":      "198.51.100.10",
			"server_port": float64(30827),
			"remark":      "hysteria2-30827",
			"tls": map[string]interface{}{
				"enabled":   true,
				"pinSHA256": "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
			},
		},
	}

	links := hysteria2Link(map[string]interface{}{"password": password}, inbound, addrs)
	if len(links) != 1 {
		t.Fatalf("expected one link, got %d", len(links))
	}
	const expected = "hysteria2://testAuth%2Fwith%2Bsymbols%3D@198.51.100.10:30827?security=tls&pinSHA256=000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f&obfs=salamander&obfs-password=testObfs%2Fwith%2Bsymbols%3D%3D&fastopen=0#hysteria2-30827"
	if links[0] != expected {
		t.Fatalf("unexpected v2rayN link:\nwant %q\ngot  %q", expected, links[0])
	}
	if strings.Contains(strings.SplitN(links[0], "@", 2)[0], "/with") {
		t.Fatalf("password contains an unescaped path separator: %q", links[0])
	}

	parsed, err := url.Parse(links[0])
	if err != nil {
		t.Fatalf("parse generated link: %v", err)
	}
	if parsed.User == nil {
		t.Fatal("generated link has no userinfo password")
	}
	if got := parsed.User.Username(); got != password {
		t.Fatalf("password did not round-trip, got %q", got)
	}
	if got := parsed.Hostname(); got != "198.51.100.10" {
		t.Fatalf("unexpected host %q", got)
	}
	if got := parsed.Port(); got != "30827" {
		t.Fatalf("unexpected port %q", got)
	}
	if got := parsed.Query().Get("obfs-password"); got != "testObfs/with+symbols==" {
		t.Fatalf("obfs password did not round-trip, got %q", got)
	}
	if got := parsed.Fragment; got != "hysteria2-30827" {
		t.Fatalf("unexpected remark %q", got)
	}

	outbound, _, err := GetOutbound(links[0], 0)
	if err != nil {
		t.Fatalf("import generated link: %v", err)
	}
	if got, _ := (*outbound)["password"].(string); got != password {
		t.Fatalf("imported password did not round-trip, got %q", got)
	}
}

func TestHysteria2LinkFormatsIPv6Host(t *testing.T) {
	inbound := map[string]interface{}{
		"out_json": json.RawMessage(`{}`),
	}
	addrs := []map[string]interface{}{
		{
			"server":      "2001:db8::10",
			"server_port": float64(443),
			"remark":      "hy2-ipv6",
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
	if got := parsed.Hostname(); got != "2001:db8::10" {
		t.Fatalf("unexpected IPv6 host %q in %q", got, links[0])
	}
	if got := parsed.Port(); got != "443" {
		t.Fatalf("unexpected IPv6 port %q", got)
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
	if got := query.Get("pcs"); got != hexPin {
		t.Fatalf("pcs should be hex for v2rayN/Xray, got %q", got)
	}
	if got := query.Get("pinSHA256"); got != "" {
		t.Fatalf("generic Xray links should use v2rayN's pcs param, got %q", got)
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

func TestGeneratedTLSLinksMatchV2rayNPinFormat(t *testing.T) {
	raw := testSha256Bytes()
	base64Pin := base64.StdEncoding.EncodeToString(raw)
	hexPin := hex.EncodeToString(raw)
	const uuid = "00000000-0000-4000-8000-000000000000"
	addrs := []map[string]interface{}{
		{
			"server":      "198.51.100.10",
			"server_port": float64(443),
			"remark":      "v2rayn-pin-test",
			"tls": map[string]interface{}{
				"enabled": true,
				"pinned_peer_certificate_sha256": []interface{}{
					base64Pin,
				},
			},
		},
	}
	rawTransport := map[string]interface{}{"transport": map[string]interface{}{"type": "tcp"}}

	tests := []struct {
		name         string
		link         string
		wantInsecure bool
		wantALPN     string
	}{
		{
			name: "vless",
			link: vlessLink(map[string]interface{}{"uuid": uuid}, rawTransport, addrs)[0],
		},
		{
			name: "trojan",
			link: trojanLink(map[string]interface{}{"password": "test-password"}, rawTransport, addrs)[0],
		},
		{
			name:         "anytls",
			link:         anytlsLink(map[string]interface{}{"password": "test-password"}, addrs)[0],
			wantInsecure: true,
		},
		{
			name:         "tuic",
			link:         tuicLink(map[string]interface{}{"uuid": uuid, "password": "test-password"}, map[string]interface{}{}, addrs)[0],
			wantInsecure: true,
			wantALPN:     "h3",
		},
		{
			name: "naive",
			link: naiveLink(map[string]interface{}{"username": "test-user", "password": "test-password"}, map[string]interface{}{}, addrs)[0],
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := url.Parse(test.link)
			if err != nil {
				t.Fatalf("parse generated link: %v", err)
			}
			query := parsed.Query()
			if got := query.Get("pcs"); got != hexPin {
				t.Fatalf("pcs should be normalized hex, got %q", got)
			}
			if got := query.Get("insecure"); (got == "1") != test.wantInsecure {
				t.Fatalf("unexpected insecure compatibility flag %q", got)
			}
			if got := query.Get("alpn"); got != test.wantALPN {
				t.Fatalf("unexpected ALPN %q", got)
			}
		})
	}
}
