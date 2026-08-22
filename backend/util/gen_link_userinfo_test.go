package util

import (
	"net/url"
	"testing"
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
			name:         "socks",
			link:         socksLink(map[string]interface{}{"username": username, "password": password}, addrs)[0],
			wantUsername: username,
			wantPassword: password,
		},
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
