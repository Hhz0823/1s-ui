package service

import (
	"strings"
	"testing"

	"github.com/Hhz0823/1s-ui/database/model"
)

func TestAllocateRemoteQuickAddSkipsUsedPortsAndTags(t *testing.T) {
	ports, tags, err := allocateRemoteQuickAdd([]map[string]interface{}{
		{"listen_port": float64(30000), "tag": "node-1"},
		{"listen_port": float64(30002), "tag": "node-2"},
	}, 30000, 3, "node", "socks")
	if err != nil {
		t.Fatal(err)
	}
	wantPorts := []int{30001, 30003, 30004}
	for index := range wantPorts {
		if ports[index] != wantPorts[index] {
			t.Fatalf("ports = %#v, want %#v", ports, wantPorts)
		}
	}
	if tags[0] != "node-1-copy1" || tags[1] != "node-2-copy1" || tags[2] != "node-3" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
}

func TestBuildRemoteQuickAddXrayVlessUsesXHTTP(t *testing.T) {
	request := RemoteQuickAddRequest{CoreType: model.CoreTypeXray, Protocol: "vless"}
	inbound := buildRemoteQuickAddInbound(request, "vless-1", 443, "secret", 2, "node.example.com")
	if inbound["listen"] != "0.0.0.0" {
		t.Fatalf("listen = %v, want IPv4 wildcard for hostname entry", inbound["listen"])
	}
	transport, ok := inbound["transport"].(map[string]interface{})
	if !ok || transport["type"] != "xhttp" || transport["host"] != "node.example.com" {
		t.Fatalf("unexpected Xray VLESS transport: %#v", inbound["transport"])
	}
}

func TestQuickAddListenAddressFollowsPublicEntryFamily(t *testing.T) {
	tests := map[string]string{
		"198.51.100.20":    "0.0.0.0",
		"node.example.com": "0.0.0.0",
		"2001:db8::20":     "::",
		"[2001:db8::20]":   "::",
	}
	for host, want := range tests {
		if got := quickAddListenAddress(host); got != want {
			t.Errorf("quickAddListenAddress(%q) = %q, want %q", host, got, want)
		}
	}
}

func TestRemoteQuickAddTLSUsesPublicEntryInsteadOfDisplayTag(t *testing.T) {
	request := RemoteQuickAddRequest{Protocol: "hysteria2", Tag: "美国-🇺🇸三网优化9.30"}
	if got := remoteQuickAddTLSServerName(request, "198.51.100.20"); got != "198.51.100.20" {
		t.Fatalf("TLS server name = %q", got)
	}
	request = RemoteQuickAddRequest{Protocol: "naive", NaiveServer: "[2001:db8::20]"}
	if got := remoteQuickAddTLSServerName(request, "198.51.100.20"); got != "2001:db8::20" {
		t.Fatalf("Naive TLS server name = %q", got)
	}
}

func TestValidateRemoteQuickAddForcesXrayShadowsocks256(t *testing.T) {
	request := RemoteQuickAddRequest{
		CoreType: model.CoreTypeXray, Protocol: "shadowsocks", Method: "aes-128-gcm",
		Count: 1, Port: 8388,
	}
	if err := validateRemoteQuickAddRequest(&request); err != nil {
		t.Fatal(err)
	}
	if request.Method != "2022-blake3-aes-256-gcm" {
		t.Fatalf("method = %q", request.Method)
	}
	inbound := buildRemoteQuickAddInbound(request, "ss", 8388, quickAddPassword(request, 0), 0, "example.com")
	if inbound["method"] != "2022-blake3-aes-256-gcm" {
		t.Fatalf("inbound method = %v", inbound["method"])
	}
	if password, _ := inbound["password"].(string); len(password) < 40 {
		t.Fatalf("expected a 256-bit base64 key, got %q", password)
	}
}

func TestValidateRemoteQuickAddRejectsUnsafeBounds(t *testing.T) {
	for _, request := range []RemoteQuickAddRequest{
		{Protocol: "socks", Count: 101, Port: 30000},
		{Protocol: "socks", Count: 1, Port: 0},
		{Protocol: "unknown", Count: 1, Port: 30000},
		{Protocol: "socks", Count: 1, Port: 30000, Tag: "bad\ntag"},
	} {
		if err := validateRemoteQuickAddRequest(&request); err == nil {
			t.Fatalf("accepted invalid request: %#v", request)
		}
	}
}

func TestRemoteQuickAddNaivePreservesIdentityAndFeatures(t *testing.T) {
	request := RemoteQuickAddRequest{
		CoreType:                   model.CoreTypeSingBox,
		Protocol:                   "naive",
		Count:                      1,
		Port:                       443,
		NaiveUsername:              "browser-user",
		NaiveServer:                "node.example.com",
		NaiveMode:                  "quic",
		NaiveExtraHeaders:          map[string]string{"X-Edge": "stable"},
		NaiveUDPOverTCP:            true,
		NaiveQUICCongestionControl: "bbr2",
	}
	if err := validateRemoteQuickAddRequest(&request); err != nil {
		t.Fatal(err)
	}
	inbound := buildRemoteQuickAddInbound(request, "naive-443", 443, "secret", 7, "198.51.100.20")
	if inbound["network"] != "udp" || inbound["quic_congestion_control"] != "bbr2" {
		t.Fatalf("unexpected Naive inbound: %#v", inbound)
	}
	addrs, ok := inbound["addrs"].([]interface{})
	if !ok || len(addrs) != 1 || addrs[0].(map[string]interface{})["server"] != "node.example.com" {
		t.Fatalf("unexpected Naive server addresses: %#v", inbound["addrs"])
	}
	outbound, ok := inbound["out_json"].(map[string]interface{})
	if !ok || outbound["quic"] != true || outbound["insecure_concurrency"] != nil {
		t.Fatalf("unexpected Naive outbound features: %#v", inbound["out_json"])
	}
	if headers := outbound["extra_headers"].(map[string]string); headers["X-Edge"] != "stable" {
		t.Fatalf("unexpected Naive extra headers: %#v", headers)
	}
}

func TestValidateRemoteQuickAddRejectsUnsafeNaiveOptions(t *testing.T) {
	for _, request := range []RemoteQuickAddRequest{
		{CoreType: model.CoreTypeSingBox, Protocol: "naive", Count: 1, Port: 443, NaiveMode: "http3"},
		{CoreType: model.CoreTypeSingBox, Protocol: "naive", Count: 1, Port: 443, NaiveInsecureConcurrency: 5},
		{CoreType: model.CoreTypeSingBox, Protocol: "naive", Count: 1, Port: 443, NaiveMode: "quic", NaiveInsecureConcurrency: 1},
		{CoreType: model.CoreTypeSingBox, Protocol: "naive", Count: 1, Port: 443, NaiveExtraHeaders: map[string]string{"Padding": "override"}},
	} {
		if err := validateRemoteQuickAddRequest(&request); err == nil {
			t.Fatalf("accepted unsafe Naive options: %#v", request)
		}
	}
}

func TestRemoteQuickAddNaiveHTTPSPreservesConcurrency(t *testing.T) {
	request := RemoteQuickAddRequest{
		CoreType: model.CoreTypeSingBox, Protocol: "naive", Count: 1, Port: 443,
		NaiveUsername: "browser-user", NaiveServer: "node.example.com", NaiveMode: "https",
		NaiveInsecureConcurrency: 2,
	}
	if err := validateRemoteQuickAddRequest(&request); err != nil {
		t.Fatal(err)
	}
	inbound := buildRemoteQuickAddInbound(request, "naive-https", 443, "secret", 7, "198.51.100.20")
	outbound := inbound["out_json"].(map[string]interface{})
	if inbound["network"] != "tcp" || outbound["quic"] != false || outbound["insecure_concurrency"] != 2 {
		t.Fatalf("unexpected HTTPS Naive options: inbound=%#v outbound=%#v", inbound, outbound)
	}
}

func TestNormalizeRemoteActor(t *testing.T) {
	if actor, err := normalizeRemoteActor("  admin  "); err != nil || actor != "admin" {
		t.Fatalf("normalize actor = %q, %v", actor, err)
	}
	if _, err := normalizeRemoteActor(strings.Repeat("a", 101)); err == nil {
		t.Fatal("accepted oversized actor")
	}
}
