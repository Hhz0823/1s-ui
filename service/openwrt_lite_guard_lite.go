//go:build openwrt_lite

package service

import (
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util/common"
)

func validateInboundRuntimeCore(inbound *model.Inbound) error {
	if inbound.RuntimeCore() == model.CoreTypeXray {
		return common.NewError("Xray-core is disabled in OpenWrt Lite build")
	}
	return nil
}

func validateOutboundLiteFeature(outbound *model.Outbound) error {
	if outbound.Type == "naive" {
		return common.NewError("naive outbound is disabled in OpenWrt Lite build")
	}
	return nil
}

func validateEndpointLiteFeature(endpoint *model.Endpoint) error {
	if endpoint.Type == "tailscale" {
		return common.NewError("Tailscale endpoint is disabled in OpenWrt Lite build")
	}
	return nil
}
