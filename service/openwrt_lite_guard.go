//go:build !openwrt_lite

package service

import "github.com/Hhz0823/1s-ui/database/model"

func validateInboundRuntimeCore(inbound *model.Inbound) error {
	return nil
}

func validateOutboundLiteFeature(outbound *model.Outbound) error {
	return nil
}

func validateEndpointLiteFeature(endpoint *model.Endpoint) error {
	return nil
}
