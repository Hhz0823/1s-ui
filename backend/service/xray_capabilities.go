//go:build !openwrt_lite

package service

import (
	"encoding/json"
	"fmt"

	"github.com/Hhz0823/1s-ui/config"
	"github.com/Hhz0823/1s-ui/core"
)

type XrayCapability struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Supported   bool   `json:"supported"`
	Shareable   bool   `json:"shareable,omitempty"`
	Recommended bool   `json:"recommended,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

type XraySelfCheck struct {
	Healthy         bool             `json:"healthy"`
	Disabled        bool             `json:"disabled"`
	OnDemand        bool             `json:"on_demand"`
	BinaryAvailable bool             `json:"binary_available"`
	Version         string           `json:"version,omitempty"`
	Path            string           `json:"path,omitempty"`
	ConfigValid     bool             `json:"config_valid"`
	HasInbounds     bool             `json:"has_inbounds"`
	Running         bool             `json:"running"`
	InboundCount    int              `json:"inbound_count"`
	Error           string           `json:"error,omitempty"`
	Protocols       []XrayCapability `json:"protocols"`
	Transports      []XrayCapability `json:"transports"`
	Checks          []string         `json:"checks,omitempty"`
}

func xrayProtocolCapabilities() []XrayCapability {
	return []XrayCapability{
		{ID: "vless", Name: "VLESS", Supported: true, Shareable: true, Recommended: true},
		{ID: "vmess", Name: "VMess", Supported: true, Shareable: true},
		{ID: "trojan", Name: "Trojan", Supported: true, Shareable: true},
		{ID: "shadowsocks", Name: "Shadowsocks", Supported: true, Shareable: true},
		{ID: "socks", Name: "SOCKS", Supported: true, Shareable: true},
		{ID: "http", Name: "HTTP", Supported: true, Shareable: true},
		{ID: "mixed", Name: "Mixed (SOCKS+HTTP)", Supported: true, Shareable: true},
		{ID: "hysteria2", Name: "Hysteria2", Supported: true, Shareable: true, Recommended: true},
		{ID: "dokodemo-door", Name: "Dokodemo-door", Supported: true},
		{ID: "wireguard", Name: "WireGuard", Supported: true},
		// Protocols that belong to sing-box only — surface clearly in self-check.
		{ID: "tuic", Name: "TUIC", Supported: false, Reason: "Xray-core has no TUIC inbound; use sing-box"},
		{ID: "naive", Name: "NaiveProxy", Supported: false, Reason: "Xray-core has no Naive inbound; use sing-box"},
		{ID: "anytls", Name: "AnyTLS", Supported: false, Reason: "Xray-core has no AnyTLS inbound; use sing-box"},
		{ID: "shadowtls", Name: "ShadowTLS", Supported: false, Reason: "Xray-core has no ShadowTLS inbound; use sing-box"},
		{ID: "hysteria", Name: "Hysteria v1", Supported: false, Reason: "use Hysteria2 on Xray-core"},
		{ID: "tun", Name: "TUN inbound", Supported: false, Reason: "requires privileged interface lifecycle; use sing-box or system TUN"},
	}
}

func xrayTransportCapabilities() []XrayCapability {
	return []XrayCapability{
		{ID: "xhttp", Name: "XHTTP", Supported: true, Recommended: true},
		{ID: "raw", Name: "RAW/TCP", Supported: true, Recommended: true},
		{ID: "kcp", Name: "mKCP", Supported: true},
		{ID: "grpc", Name: "gRPC", Supported: true},
		{ID: "ws", Name: "WebSocket", Supported: true},
		{ID: "httpupgrade", Name: "HTTPUpgrade", Supported: true},
		{ID: "hysteria", Name: "Hysteria2 transport", Supported: true},
		{ID: "http", Name: "HTTP/2", Supported: false, Reason: "removed by Xray-core; use XHTTP stream-one"},
		{ID: "quic", Name: "QUIC", Supported: false, Reason: "removed by Xray-core; use XHTTP stream-one H3"},
	}
}

func (s *ConfigService) CheckXray() XraySelfCheck {
	result := XraySelfCheck{
		Protocols:  xrayProtocolCapabilities(),
		Transports: xrayTransportCapabilities(),
		Checks:     []string{},
		OnDemand:   config.IsXrayOnDemand(),
	}
	if config.IsXrayDisabled() {
		result.Disabled = true
		result.Error = "Xray-core is disabled on this low-resource server; use sing-box"
		result.Checks = append(result.Checks, "runtime: disabled by low-resource profile")
		result.HasInbounds, _ = s.HasXrayInbounds()
		return result
	}
	if xrayPtr == nil {
		xrayPtr = core.NewXrayRuntime()
	}
	if result.OnDemand {
		result.Checks = append(result.Checks, "runtime: on-demand (sing-box remains default)")
	}
	status := xrayPtr.Status()
	result.Path, _ = status["path"].(string)
	result.Running, _ = status["running"].(bool)
	result.HasInbounds, _ = s.HasXrayInbounds()

	version, err := xrayPtr.Version()
	if err != nil {
		result.Error = err.Error()
		result.Checks = append(result.Checks, "binary: unavailable")
		return result
	}
	result.BinaryAvailable = true
	result.Version = version
	result.Checks = append(result.Checks, "binary: ok ("+version+")")

	rawConfig, err := s.GetXrayConfig()
	if err != nil {
		result.Error = err.Error()
		result.Checks = append(result.Checks, "config build: failed")
		return result
	}

	var parsed struct {
		Inbounds []interface{} `json:"inbounds"`
	}
	_ = json.Unmarshal(*rawConfig, &parsed)
	result.InboundCount = len(parsed.Inbounds)
	result.Checks = append(result.Checks, fmt.Sprintf("inbounds: %d", result.InboundCount))

	if result.InboundCount == 0 {
		// Empty inbound list is valid when no Xray nodes are configured.
		result.ConfigValid = true
		result.Healthy = true
		result.Checks = append(result.Checks, "config: empty (no xray inbounds)")
		return result
	}

	if err = xrayPtr.Validate(*rawConfig); err != nil {
		result.Error = err.Error()
		result.Checks = append(result.Checks, "config validate: failed")
		return result
	}
	result.ConfigValid = true
	result.Healthy = true
	result.Checks = append(result.Checks, "config validate: ok")
	if result.Running {
		result.Checks = append(result.Checks, "runtime: running")
	} else {
		result.Checks = append(result.Checks, "runtime: stopped")
	}
	return result
}
