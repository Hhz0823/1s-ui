//go:build !openwrt_lite

package service

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
)

func TestBuildXrayStreamSettingsCurrentTransports(t *testing.T) {
	tests := []struct {
		name        string
		network     string
		transport   map[string]interface{}
		settingsKey string
		wantNetwork string
	}{
		{name: "raw", network: "raw", transport: map[string]interface{}{"accept_proxy_protocol": true}, settingsKey: "rawSettings", wantNetwork: "raw"},
		{name: "websocket", network: "ws", transport: map[string]interface{}{"host": "cdn.example.com", "path": "/ws"}, settingsKey: "wsSettings", wantNetwork: "ws"},
		{name: "grpc", network: "grpc", transport: map[string]interface{}{"service_name": "edge"}, settingsKey: "grpcSettings", wantNetwork: "grpc"},
		{name: "httpupgrade", network: "httpupgrade", transport: map[string]interface{}{"host": "cdn.example.com", "path": "/up"}, settingsKey: "httpupgradeSettings", wantNetwork: "httpupgrade"},
		{name: "xhttp", network: "xhttp", transport: map[string]interface{}{"host": "cdn.example.com", "path": "/x", "mode": "stream-one"}, settingsKey: "xhttpSettings", wantNetwork: "xhttp"},
		{name: "mkcp", network: "kcp", transport: map[string]interface{}{"mtu": float64(1400)}, settingsKey: "kcpSettings", wantNetwork: "kcp"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stream, err := buildXrayStreamSettings(&model.Inbound{}, test.transport, test.network)
			if err != nil {
				t.Fatal(err)
			}
			if stream["network"] != test.wantNetwork || stream[test.settingsKey] == nil {
				t.Fatalf("unexpected stream settings: %#v", stream)
			}
			if test.network == "ws" {
				settings := stream["wsSettings"].(map[string]interface{})
				if settings["host"] != "cdn.example.com" || settings["headers"] != nil {
					t.Fatalf("websocket host uses obsolete shape: %#v", settings)
				}
			}
			if test.network == "xhttp" || test.network == "ws" || test.network == "grpc" || test.network == "httpupgrade" {
				sockopt := stream["sockopt"].(map[string]interface{})
				headers := sockopt["trustedXForwardedFor"].([]string)
				if len(headers) != 1 || headers[0] != "X-1S-UI-Trusted-XFF" {
					t.Fatalf("trusted XFF guard was not generated: %#v", sockopt)
				}
			}
		})
	}
}

func TestBuildXrayStreamSettingsKeepsCustomTrustedXFFHeaders(t *testing.T) {
	stream, err := buildXrayStreamSettings(
		&model.Inbound{},
		map[string]interface{}{
			"trusted_x_forwarded_for": []interface{}{"X-Real-IP", "CF-Connecting-IP"},
		},
		"xhttp",
	)
	if err != nil {
		t.Fatal(err)
	}
	sockopt := stream["sockopt"].(map[string]interface{})
	headers := sockopt["trustedXForwardedFor"].([]string)
	if !reflect.DeepEqual(headers, []string{"X-Real-IP", "CF-Connecting-IP"}) {
		t.Fatalf("custom trusted XFF headers were lost: %#v", sockopt)
	}
}

func TestBuildXrayStreamSettingsRejectsRealityOnKCP(t *testing.T) {
	tlsConfig := &model.Tls{Server: json.RawMessage(`{"enabled":true,"reality":{"enabled":true}}`)}
	_, err := buildXrayStreamSettings(&model.Inbound{Tls: tlsConfig}, map[string]interface{}{}, "kcp")
	if err == nil {
		t.Fatal("Reality with mKCP was accepted")
	}
}

func TestBuildXrayDokodemoInbound(t *testing.T) {
	inbound := &model.Inbound{Tag: "transparent", UploadLimit: 1024, DownloadLimit: 2048, Options: json.RawMessage(`{
		"listen":"0.0.0.0","listen_port":12345,"network":"tcp,udp",
		"address":"127.0.0.1","port":8080,"follow_redirect":true
	}`)}
	config, err := (&InboundService{}).buildXrayDokodemoInbound(inbound)
	if err != nil {
		t.Fatal(err)
	}
	settings := config["settings"].(map[string]interface{})
	if config["protocol"] != "dokodemo-door" || settings["address"] != "127.0.0.1" || settings["port"] != 8080 {
		t.Fatalf("unexpected dokodemo config: %#v", config)
	}
	raw, err := json.Marshal(config)
	if err != nil || strings.Contains(string(raw), "upload_limit") || strings.Contains(string(raw), "download_limit") {
		t.Fatalf("bandwidth limits leaked into Xray config: %s err=%v", raw, err)
	}
}

func TestXrayHysteriaMasqueradeMapping(t *testing.T) {
	tests := []struct {
		value interface{}
		want  map[string]interface{}
	}{
		{value: "https://example.com/", want: map[string]interface{}{"type": "proxy", "url": "https://example.com/"}},
		{value: "file:///srv/www", want: map[string]interface{}{"type": "file", "dir": "/srv/www"}},
		{value: "hello", want: map[string]interface{}{"type": "string", "content": "hello"}},
	}
	for _, test := range tests {
		got, ok := xrayHysteriaMasquerade(test.value)
		if !ok {
			t.Fatalf("masquerade was rejected: %#v", test.value)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Fatalf("unexpected masquerade mapping: got %#v, want %#v", got, test.want)
		}
	}
}

func TestBuildXrayHysteria2InboundUsesUsers(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "xray-hysteria2.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	client := model.Client{
		Enable:   true,
		Name:     "hysteria-user",
		Config:   json.RawMessage(`{"hysteria2":{"password":"test-password"}}`),
		Inbounds: json.RawMessage(`[43]`),
	}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}
	inbound := &model.Inbound{
		Id:      43,
		Tag:     "hysteria-in",
		Type:    "hysteria2",
		Options: json.RawMessage(`{"listen":"127.0.0.1","listen_port":32104,"transport":{"type":"hysteria"}}`),
		Tls:     &model.Tls{Server: json.RawMessage(`{"enabled":true}`)},
	}
	config, err := (&InboundService{}).buildXrayHysteria2Inbound(db, inbound)
	if err != nil {
		t.Fatal(err)
	}
	settings := config["settings"].(map[string]interface{})
	users, ok := settings["users"].([]map[string]interface{})
	if !ok || len(users) != 1 || users[0]["auth"] != "test-password" {
		t.Fatalf("unexpected Hysteria2 users: %#v", settings)
	}
	if _, exists := settings["clients"]; exists {
		t.Fatalf("deprecated Hysteria2 clients field was generated: %#v", settings)
	}
}

func TestFetchXrayVlessClientsKeepsVisionFlowOnRaw(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "xray-flow.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	client := model.Client{
		Enable:   true,
		Name:     "vision-user",
		Config:   json.RawMessage(`{"vless":{"uuid":"11111111-1111-4111-8111-111111111111","flow":"xtls-rprx-vision"}}`),
		Inbounds: json.RawMessage(`[42]`),
	}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}
	clients, err := (&InboundService{}).fetchXrayVlessClients(db, 42, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 1 || clients[0]["flow"] != "xtls-rprx-vision" {
		t.Fatalf("Vision flow was lost for RAW transport: %#v", clients)
	}
}

func TestXrayCapabilitiesDoNotAdvertiseRemovedTransports(t *testing.T) {
	for _, capability := range xrayTransportCapabilities() {
		if (capability.ID == "http" || capability.ID == "quic") && capability.Supported {
			t.Fatalf("removed transport advertised as supported: %#v", capability)
		}
	}
}

func TestXraySupportedProtocolsIncludeWireGuardAndDokodemo(t *testing.T) {
	found := map[string]bool{}
	for _, capability := range xrayProtocolCapabilities() {
		if capability.Supported {
			found[capability.ID] = true
		}
	}
	for _, id := range []string{"vless", "vmess", "trojan", "shadowsocks", "socks", "http", "mixed", "hysteria2", "dokodemo-door", "wireguard"} {
		if !found[id] {
			t.Fatalf("expected supported protocol %s", id)
		}
	}
}

func TestBuildXrayWireGuardInbound(t *testing.T) {
	inbound := &model.Inbound{Tag: "wg-in", Options: json.RawMessage(`{
		"listen":"0.0.0.0","listen_port":51820,
		"secret_key":"cOQHjI0u4m4r8n0Yh8rH0o2p3q4s5t6u7v8w9x0y1z2=",
		"peers":[{"public_key":"abc","allowed_ips":["0.0.0.0/0"]}]
	}`)}
	config, err := (&InboundService{}).buildXrayWireGuardInbound(inbound)
	if err != nil {
		t.Fatal(err)
	}
	if config["protocol"] != "wireguard" {
		t.Fatalf("unexpected protocol: %#v", config)
	}
	settings := config["settings"].(map[string]interface{})
	if settings["secretKey"] == "" {
		t.Fatalf("missing secretKey: %#v", settings)
	}
}
