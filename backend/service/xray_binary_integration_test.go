//go:build !openwrt_lite

package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database/model"
)

func TestXrayGeneratedTransportConfigsWithRealBinary(t *testing.T) {
	binary := os.Getenv("XRAY_TEST_BINARY")
	if binary == "" {
		t.Skip("XRAY_TEST_BINARY is not set")
	}
	t.Setenv("SUI_XRAY_PATH", binary)
	t.Setenv("SUI_XRAY_CONFIG", filepath.Join(t.TempDir(), "xray.json"))
	runtime := core.NewXrayRuntime()

	tests := []struct {
		name      string
		network   string
		transport map[string]interface{}
	}{
		{name: "raw", network: "raw", transport: map[string]interface{}{}},
		{name: "xhttp", network: "xhttp", transport: map[string]interface{}{"path": "/xhttp", "mode": "auto"}},
		{name: "websocket", network: "ws", transport: map[string]interface{}{"path": "/ws", "host": "example.com"}},
		{name: "grpc", network: "grpc", transport: map[string]interface{}{"service_name": "edge"}},
		{name: "httpupgrade", network: "httpupgrade", transport: map[string]interface{}{"path": "/up", "host": "example.com"}},
		{name: "mkcp", network: "kcp", transport: map[string]interface{}{}},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stream, err := buildXrayStreamSettings(&model.Inbound{}, test.transport, test.network)
			if err != nil {
				t.Fatal(err)
			}
			config := XrayConfig{
				Inbounds: []map[string]interface{}{{
					"tag": "test-" + test.name, "listen": "127.0.0.1", "port": 32000 + index,
					"protocol": "vless", "settings": map[string]interface{}{
						"clients": []map[string]interface{}{{"id": "11111111-1111-4111-8111-111111111111"}}, "decryption": "none",
					}, "streamSettings": stream,
				}},
				Outbounds: []map[string]interface{}{{"protocol": "freedom", "tag": "direct"}},
			}
			raw, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			if err := runtime.Validate(raw); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestXrayAdditionalInboundConfigsWithRealBinary(t *testing.T) {
	binary := os.Getenv("XRAY_TEST_BINARY")
	if binary == "" {
		t.Skip("XRAY_TEST_BINARY is not set")
	}
	t.Setenv("SUI_XRAY_PATH", binary)
	t.Setenv("SUI_XRAY_CONFIG", filepath.Join(t.TempDir(), "xray.json"))
	runtime := core.NewXrayRuntime()
	inbounds := []map[string]interface{}{
		{"tag": "mixed", "listen": "127.0.0.1", "port": 32100, "protocol": "mixed", "settings": map[string]interface{}{"auth": "noauth", "udp": true}},
		{"tag": "transparent", "listen": "127.0.0.1", "port": 32101, "protocol": "dokodemo-door", "settings": map[string]interface{}{"network": "tcp,udp", "address": "127.0.0.1", "port": 80, "followRedirect": true}},
		{"tag": "hysteria2", "listen": "127.0.0.1", "port": 32102, "protocol": "hysteria", "settings": map[string]interface{}{"version": 2, "users": []map[string]interface{}{{"auth": "test-password", "email": "test"}}}, "streamSettings": map[string]interface{}{"network": "hysteria", "hysteriaSettings": map[string]interface{}{"version": 2, "udpIdleTimeout": 60, "masquerade": map[string]interface{}{"type": "proxy", "url": "https://example.com/"}}}},
	}
	for _, inbound := range inbounds {
		t.Run(inbound["tag"].(string), func(t *testing.T) {
			config := XrayConfig{Inbounds: []map[string]interface{}{inbound}, Outbounds: []map[string]interface{}{{"protocol": "freedom", "tag": "direct"}}}
			raw, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			if err := runtime.Validate(raw); err != nil {
				t.Fatal(err)
			}
		})
	}
}
