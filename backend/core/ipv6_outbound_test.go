package core

import (
	"testing"

	"github.com/Hhz0823/1s-ui/logger"
	"github.com/op/go-logging"
)

func TestDirectOutboundAcceptsIPv6OnlyStrategy(t *testing.T) {
	logger.InitLogger(logging.CRITICAL)
	instance := NewCore()
	config := []byte(`{
  "log": {"disabled": true},
  "outbounds": [{
    "type": "direct",
    "tag": "relay-ipv6",
    "inet6_bind_address": "2001:db8::10",
    "domain_strategy": "ipv6_only"
  }],
  "route": {
    "rules": [{"ip_version": 4, "action": "reject"}]
  }
}`)
	if err := instance.Start(config); err != nil {
		t.Fatal(err)
	}
	if err := instance.Stop(); err != nil {
		t.Fatal(err)
	}
}
