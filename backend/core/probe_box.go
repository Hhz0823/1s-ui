package core

import (
	"context"
	"encoding/json"
	"errors"
	"net"

	sb "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

// ProbeBox is a throwaway sing-box instance that holds only outbounds, so
// nodes can be checked from this server without touching the running core.
// It has its own context, registries and DNS (the system resolver), and it
// logs nothing. Close it when the checks are done.
type ProbeBox struct {
	instance *sb.Box
}

// StartProbeBox builds the outbounds (sing-box outbound objects, each with a
// unique tag) and starts them.
func StartProbeBox(outbounds []map[string]interface{}) (*ProbeBox, error) {
	if len(outbounds) == 0 {
		return nil, errors.New("no outbounds to check")
	}
	ctx := sb.Context(context.Background(), InboundRegistry(), OutboundRegistry(), EndpointRegistry(), DNSTransportRegistry(), ServiceRegistry())
	raw, err := json.Marshal(map[string]interface{}{
		"log":       map[string]interface{}{"disabled": true},
		"dns":       map[string]interface{}{"servers": []interface{}{map[string]interface{}{"type": "local", "tag": "local"}}},
		"outbounds": outbounds,
		"route":     map[string]interface{}{"default_domain_resolver": "local"},
	})
	if err != nil {
		return nil, err
	}
	var options option.Options
	if err := options.UnmarshalJSONContext(ctx, raw); err != nil {
		return nil, err
	}
	instance, err := sb.New(sb.Options{Context: ctx, Options: options})
	if err != nil {
		return nil, err
	}
	if err := instance.Start(); err != nil {
		_ = instance.Close()
		return nil, err
	}
	return &ProbeBox{instance: instance}, nil
}

// ValidateOutbound reports whether sing-box accepts one outbound's options,
// so a bad node does not stop the others from being checked.
func ValidateOutbound(outbound map[string]interface{}) error {
	ctx := sb.Context(context.Background(), InboundRegistry(), OutboundRegistry(), EndpointRegistry(), DNSTransportRegistry(), ServiceRegistry())
	raw, err := json.Marshal(outbound)
	if err != nil {
		return err
	}
	var parsed option.Outbound
	return parsed.UnmarshalJSONContext(ctx, raw)
}

// Dial opens a TCP connection to host:port through the outbound with tag.
func (b *ProbeBox) Dial(ctx context.Context, tag, host, port string) (net.Conn, error) {
	outbound, ok := b.instance.Outbound().Outbound(tag)
	if !ok {
		return nil, errors.New("outbound " + tag + " not found")
	}
	return outbound.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddrHostPortStr(host, port))
}

func (b *ProbeBox) Close() error {
	return b.instance.Close()
}
