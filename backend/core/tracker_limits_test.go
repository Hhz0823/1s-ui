package core

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
)

func TestInboundTrafficQuotaStopsConnections(t *testing.T) {
	tracker := NewStatsTracker()
	tracker.SetInboundLimit("capped", InboundBandwidthLimit{QuotaEnabled: true, QuotaRemaining: 10})

	local, peer := net.Pipe()
	defer peer.Close()
	wrapped := tracker.wrapConnection(context.Background(), local, "capped", "", "", netip.Addr{})
	go func() {
		buffer := make([]byte, 64)
		for {
			if _, err := peer.Read(buffer); err != nil {
				return
			}
		}
	}()
	if _, err := wrapped.Write(make([]byte, 8)); err != nil {
		t.Fatalf("write within quota failed: %v", err)
	}
	if _, err := wrapped.Write(make([]byte, 8)); !errors.Is(err, ErrInboundTrafficExhausted) {
		t.Fatalf("write past quota returned %v", err)
	}
	wrapped.Close()

	next, nextPeer := net.Pipe()
	defer nextPeer.Close()
	rejected := tracker.wrapConnection(context.Background(), next, "capped", "", "", netip.Addr{})
	if _, err := rejected.Write([]byte("x")); !errors.Is(err, ErrInboundTrafficExhausted) {
		t.Fatalf("new connection on exhausted inbound returned %v", err)
	}

	// A new period restores the quota.
	tracker.SetInboundLimit("capped", InboundBandwidthLimit{QuotaEnabled: true, QuotaRemaining: 100})
	fresh, freshPeer := net.Pipe()
	defer freshPeer.Close()
	if _, ok := tracker.wrapConnection(context.Background(), fresh, "capped", "", "", netip.Addr{}).(*limitedConn); !ok {
		t.Fatal("connection after reset was rejected")
	}
}

func TestInboundIPLimit(t *testing.T) {
	tracker := NewStatsTracker()
	tracker.SetInboundLimit("ip", InboundBandwidthLimit{MaxIPs: 1})
	first := netip.MustParseAddr("192.0.2.1")
	second := netip.MustParseAddr("192.0.2.2")

	open := func(addr netip.Addr) net.Conn {
		local, peer := net.Pipe()
		t.Cleanup(func() { peer.Close() })
		return tracker.wrapConnection(context.Background(), local, "ip", "", "", addr)
	}
	a1 := open(first)
	a2 := open(first)
	if _, ok := a2.(*limitedConn); !ok {
		t.Fatal("second connection from the same IP was rejected")
	}
	if _, ok := open(second).(*rejectedConn); !ok {
		t.Fatal("connection from a second IP was admitted over the limit")
	}
	if got := tracker.InboundActiveIPs()["ip"]; got != 1 {
		t.Fatalf("active IPs = %d, want 1", got)
	}
	a1.Close()
	a1.Close()
	if _, ok := open(second).(*rejectedConn); !ok {
		t.Fatal("IP was released while it still had an open connection")
	}
	a2.Close()
	if _, ok := open(netip.MustParseAddr("::ffff:192.0.2.2")).(*limitedConn); !ok {
		t.Fatal("new IP rejected after the previous IP disconnected")
	}
}

func TestInboundLimiterKeepsRateOnRepeatedSync(t *testing.T) {
	tracker := NewStatsTracker()
	tracker.SetInboundLimit("rate", InboundBandwidthLimit{Upload: 1000})
	before := tracker.limiters["rate"].upload.Load()
	tracker.SyncInboundLimits(map[string]InboundBandwidthLimit{"rate": {Upload: 1000, QuotaEnabled: true, QuotaRemaining: 5}})
	if tracker.limiters["rate"].upload.Load() != before {
		t.Fatal("unchanged rate limit rebuilt its token bucket")
	}
	tracker.SyncInboundLimits(map[string]InboundBandwidthLimit{})
	if _, ok := tracker.limiters["rate"]; ok {
		t.Fatal("limiter kept after its limits were removed")
	}
}
