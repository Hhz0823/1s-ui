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

func TestUserInfoLinksEscapeCredentials(t *testing.T) {
	const (
		username = "test/user+name="
		password = "test/password+value="
		uuid     = "00000000-0000-4000-8000-000000000000"
	)
	addrs := []map[string]interface{}{
		{
			"server":      "198.51.100.10",
			"server_port": float64(443),
			"remark":      "userinfo-test",
		},
	}

	tests := []struct {
		name         string
		link         string
		wantUsername string
		wantPassword string
	}{
		{
			name:         "http",
			link:         httpLink(map[string]interface{}{"username": username, "password": password}, addrs)[0],
			wantUsername: username,
			wantPassword: password,
		},
		{
			name:         "naive",
			link:         naiveLink(map[string]interface{}{"username": username, "password": password}, map[string]interface{}{}, addrs)[0],
			wantUsername: username,
			wantPassword: password,
		},
		{
			name:         "anytls",
			link:         anytlsLink(map[string]interface{}{"password": password}, addrs)[0],
			wantUsername: password,
		},
		{
			name:         "tuic",
			link:         tuicLink(map[string]interface{}{"uuid": uuid, "password": password}, map[string]interface{}{}, addrs)[0],
			wantUsername: uuid,
			wantPassword: password,
		},
		{
			name:         "sing-box trojan",
			link:         trojanLink(map[string]interface{}{"password": password}, map[string]interface{}{}, addrs)[0],
			wantUsername: password,
		},
		{
			name:         "xray trojan",
			link:         xrayTrojanLink(map[string]interface{}{"password": password}, map[string]interface{}{}, addrs)[0],
			wantUsername: password,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := url.Parse(test.link)
			if err != nil {
				t.Fatalf("parse generated link: %v", err)
			}
			if parsed.User == nil {
				t.Fatalf("generated link has no userinfo: %q", test.link)
			}
			if got := parsed.User.Username(); got != test.wantUsername {
				t.Fatalf("unexpected username %q in %q", got, test.link)
			}
			if test.wantPassword != "" {
				got, ok := parsed.User.Password()
				if !ok || got != test.wantPassword {
					t.Fatalf("unexpected password %q in %q", got, test.link)
				}
			}
			if got := parsed.Hostname(); got != "198.51.100.10" {
				t.Fatalf("unexpected host %q in %q", got, test.link)
			}
			if got := parsed.Port(); got != "443" {
				t.Fatalf("unexpected port %q in %q", got, test.link)
			}
		})
	}
}

func TestNaiveLinkUsesCurrentClientSchemeAndSNI(t *testing.T) {
	const sni = "naive.example.com"
	addrs := []map[string]interface{}{
		{
			"server":      "198.51.100.10",
			"server_port": float64(443),
			"remark":      "naive-test",
			"tls": map[string]interface{}{
				"server_name": sni,
				"insecure":    true,
				"pinned_peer_certificate_sha256": []interface{}{
					"uIKKJEaSu1IkowxZx3ud+BmdfFmk6kKy7tNm1YBxFjQ=",
				},
			},
		},
	}

	link := naiveLink(
		map[string]interface{}{"username": "test/user", "password": "test/password+"},
		map[string]interface{}{},
		addrs,
	)[0]
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parse generated link: %v", err)
	}
	if parsed.Scheme != "naive+https" {
		t.Fatalf("unexpected scheme %q in %q", parsed.Scheme, link)
	}
	if got := parsed.Query().Get("sni"); got != sni {
		t.Fatalf("unexpected sni %q in %q", got, link)
	}
	if got := parsed.Query().Get("peer"); got != sni {
		t.Fatalf("unexpected compatibility peer %q in %q", got, link)
	}
	if got := parsed.Query().Get("insecure"); got != "" {
		t.Fatalf("Naive must not export unsupported insecure flag %q in %q", got, link)
	}
	if got := parsed.Query().Get("security"); got != "tls" {
		t.Fatalf("unexpected security %q in %q", got, link)
	}
	if got := parsed.Query().Get("pcs"); got == "" {
		t.Fatalf("missing pinned certificate SHA-256 in %q", link)
	}
	outbound, _, err := parseNaiveLink(parsed, 0)
	if err != nil {
		t.Fatal(err)
	}
	if tlsConfig := (*outbound)["tls"].(map[string]interface{}); tlsConfig["pinned_peer_certificate_sha256"] != nil {
		t.Fatalf("Naive importer must ignore unsupported certificate pin: %#v", tlsConfig)
	}
}

func TestNaiveLinkUsesQUICAndExtraHeaders(t *testing.T) {
	links := naiveLink(
		map[string]interface{}{"username": "browser", "password": "secret"},
		map[string]interface{}{
			"network":  "udp",
			"out_json": json.RawMessage(`{"quic":true,"extra_headers":{"X-Edge":"stable","X-Mode":"browser","Padding":"forbidden"}}`),
		},
		[]map[string]interface{}{{
			"server": "node.example.com", "server_port": float64(443), "remark": "naive-quic",
		}},
	)
	if len(links) != 1 {
		t.Fatalf("links = %#v", links)
	}
	parsed, err := url.Parse(links[0])
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "naive+quic" {
		t.Fatalf("scheme = %q", parsed.Scheme)
	}
	extraHeaders := parsed.Query().Get("extra-headers")
	if !strings.Contains(extraHeaders, "X-Edge: stable") || !strings.Contains(extraHeaders, "X-Mode: browser") {
		t.Fatalf("extra-headers = %q", extraHeaders)
	}
	if strings.Contains(extraHeaders, "Padding") {
		t.Fatalf("reserved header leaked into link: %q", extraHeaders)
	}
	outbound, _, err := parseNaiveLink(parsed, 0)
	if err != nil {
		t.Fatal(err)
	}
	if outboundHeaders := (*outbound)["extra_headers"].(map[string]string); outboundHeaders["X-Edge"] != "stable" {
		t.Fatalf("parsed headers = %#v", outboundHeaders)
	}
}

func TestNaiveV2rayNLinkPreservesSecureAdvancedOptions(t *testing.T) {
	_, certificate, err := GenerateSelfSignedTLS("node.example.com", time.Now(), time.Now().AddDate(1, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	serverTLS, err := json.Marshal(map[string]interface{}{
		"enabled": true, "server_name": "node.example.com",
		"certificate": strings.Split(strings.TrimSpace(string(certificate)), "\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	inbound := &model.Inbound{
		Type: "naive", Tag: "naive-secure", CoreType: model.CoreTypeSingBox, TlsId: 1,
		Tls:     &model.Tls{Id: 1, Server: serverTLS, Client: json.RawMessage(`{}`)},
		Addrs:   json.RawMessage(`[{"server":"198.51.100.20","server_port":443,"remark":""}]`),
		OutJson: json.RawMessage(`{"quic":true,"quic_congestion_control":"bbr2","udp_over_tcp":{"enabled":true},"insecure_concurrency":0}`),
		Options: json.RawMessage(`{"listen_port":443,"network":"udp","quic_congestion_control":"bbr2"}`),
	}
	links := LinkGenerator(
		json.RawMessage(`{"naive":{"username":"browser","password":"strong-secret"}}`),
		inbound,
		"198.51.100.20",
		"",
	)
	if len(links) != 1 {
		t.Fatalf("links = %#v", links)
	}
	parsed, err := url.Parse(links[0])
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "v2rayn" || parsed.Host != "naive" {
		t.Fatalf("unexpected v2rayN URI: %q", links[0])
	}
	encoded := strings.TrimPrefix(parsed.Path, "/")
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var profile struct {
		ConfigType     int
		CoreType       int
		ConfigVersion  int
		Address        string
		Port           int
		Username       string
		Password       string
		Remarks        string
		StreamSecurity string
		Sni            string
		Cert           string
		AllowInsecure  string
		ProtoExtraObj  struct {
			Uot                 bool
			NaiveQuic           bool
			CongestionControl   string
			InsecureConcurrency int
		}
	}
	if err = json.Unmarshal(decoded, &profile); err != nil {
		t.Fatal(err)
	}
	if profile.ConfigType != 12 || profile.CoreType != 24 || profile.ConfigVersion != 4 {
		t.Fatalf("unexpected v2rayN profile identity: %#v", profile)
	}
	if profile.Address != "198.51.100.20" || profile.Port != 443 || profile.Username != "browser" || profile.Password != "strong-secret" {
		t.Fatalf("unexpected v2rayN endpoint or credentials: %#v", profile)
	}
	if profile.StreamSecurity != "tls" || profile.Sni != "node.example.com" || profile.AllowInsecure != "" {
		t.Fatalf("TLS verification was weakened: %#v", profile)
	}
	if !strings.Contains(profile.Cert, "BEGIN CERTIFICATE") || strings.Contains(profile.Cert, "PRIVATE KEY") {
		t.Fatalf("public certificate was not embedded safely")
	}
	if !profile.ProtoExtraObj.Uot || !profile.ProtoExtraObj.NaiveQuic || profile.ProtoExtraObj.CongestionControl != "bbr2" {
		t.Fatalf("advanced Naive settings were lost: %#v", profile.ProtoExtraObj)
	}
	if profile.ProtoExtraObj.InsecureConcurrency != 0 {
		t.Fatalf("secure default changed: %#v", profile.ProtoExtraObj)
	}
}

func TestNaiveV2rayNLinkRetainsPortableURLForTrustedTLS(t *testing.T) {
	inbound := &model.Inbound{
		Type: "naive", Tag: "naive-public", CoreType: model.CoreTypeSingBox, TlsId: 1,
		Tls: &model.Tls{
			Id:     1,
			Server: json.RawMessage(`{"enabled":true,"server_name":"node.example.com"}`),
			Client: json.RawMessage(`{}`),
		},
		Addrs:   json.RawMessage(`[{"server":"node.example.com","server_port":443,"remark":""}]`),
		OutJson: json.RawMessage(`{"quic":true,"udp_over_tcp":{"enabled":true}}`),
		Options: json.RawMessage(`{"listen_port":443,"network":"udp"}`),
	}
	links := LinkGenerator(
		json.RawMessage(`{"naive":{"username":"browser","password":"strong-secret"}}`),
		inbound,
		"node.example.com",
		"",
	)
	if len(links) != 2 || !strings.HasPrefix(links[0], "v2rayn://naive/") || !strings.HasPrefix(links[1], "naive+quic://") {
		t.Fatalf("trusted TLS links = %#v", links)
	}
}

func TestUserInfoLinksFormatIPv6Host(t *testing.T) {
	addrs := []map[string]interface{}{
		{
			"server":      "2001:db8::20",
			"server_port": float64(8443),
			"remark":      "ipv6-test",
		},
	}
	link := anytlsLink(map[string]interface{}{"password": "test/password="}, addrs)[0]
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parse generated link: %v", err)
	}
	if got := parsed.Hostname(); got != "2001:db8::20" {
		t.Fatalf("unexpected host %q in %q", got, link)
	}
	if got := parsed.Port(); got != "8443" {
		t.Fatalf("unexpected port %q in %q", got, link)
	}
}
