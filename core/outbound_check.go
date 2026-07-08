package core

import (
	"context"
	"time"

	urltest "github.com/sagernet/sing-box/common/urltest"
)

const checkTimeout = 15 * time.Second

type CheckOutboundResult struct {
	OK    bool
	Delay uint16
	Error string
}

func CheckOutbound(ctx context.Context, tag string, link string) (result CheckOutboundResult) {
	if outbound_manager == nil {
		result.Error = "core not running"
		return result
	}
	ob, ok := outbound_manager.Outbound(tag)
	if !ok {
		if endpoint_manager == nil {
			result.Error = "outbound or endpoint not found"
			return result
		}
		ep, epOk := endpoint_manager.Get(tag)
		if !epOk {
			result.Error = "outbound or endpoint not found"
			return result
		}
		ob = ep
	}

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	delay, err := urltest.URLTest(ctx, link, ob)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.OK = true
	result.Delay = delay
	return result
}
