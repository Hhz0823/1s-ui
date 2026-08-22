//go:build !openwrt_lite

package service

import (
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util/common"
)

// xraySupportedInboundTypes is the complete set of inbound protocols this panel can
// translate into valid Xray-core configuration.
var xraySupportedInboundTypes = map[string]bool{
	"vless": true, "vmess": true, "trojan": true, "shadowsocks": true,
	"socks": true, "http": true, "mixed": true, "hysteria2": true,
	"dokodemo-door": true, "wireguard": true,
}

func validateInboundRuntimeCore(inbound *model.Inbound) error {
	if inbound == nil {
		return nil
	}
	if inbound.RuntimeCore() != model.CoreTypeXray {
		return nil
	}
	if inbound.Type == "" {
		return common.NewError("xray inbound type is required")
	}
	if !xraySupportedInboundTypes[inbound.Type] {
		return common.NewErrorf("xray inbound type <%s> is not supported; supported: vless, vmess, trojan, shadowsocks, socks, http, mixed, hysteria2, dokodemo-door, wireguard", inbound.Type)
	}
	return nil
}

func validateOutboundLiteFeature(outbound *model.Outbound) error {
	return nil
}

func validateEndpointLiteFeature(endpoint *model.Endpoint) error {
	return nil
}

func validateTlsLiteFeature(tls *model.Tls) error {
	return nil
}
