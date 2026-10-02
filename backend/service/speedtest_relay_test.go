package service

import (
	"strings"
	"testing"

	"github.com/Hhz0823/1s-ui/speedtest"
)

func TestRunLocalSpeedtestAgainstASession(t *testing.T) {
	port := freeTCPPort(t)
	target, err := StartLocalSpeedtest(port)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(speedtest.Stop)
	for _, test := range speedtest.Tests {
		result, err := RunLocalSpeedtest(SpeedtestRunRequest{
			Token: target.Token, Port: target.Port, Hosts: []string{"127.0.0.1"}, Test: test,
			Seconds: 3, Streams: 2, UDPMbps: 20,
		})
		if err != nil || result.Test != test || result.Host != "127.0.0.1" {
			t.Fatalf("%s: %+v %v", test, result, err)
		}
		switch {
		case strings.HasSuffix(test, "_ping"):
			if result.Ping == nil || result.Ping.Received == 0 {
				t.Fatalf("%s: %+v", test, result.Ping)
			}
		case strings.HasPrefix(test, "tcp_"):
			if result.Throughput == nil || result.Throughput.BitsPerSecond <= 0 {
				t.Fatalf("%s: %+v", test, result.Throughput)
			}
		default:
			if result.UDP == nil || result.UDP.ReceivedPackets == 0 {
				t.Fatalf("%s: %+v", test, result.UDP)
			}
		}
	}
	if _, err := RunLocalSpeedtest(SpeedtestRunRequest{Token: target.Token, Port: target.Port, Hosts: []string{"127.0.0.1"}, Test: "icmp"}); err == nil {
		t.Fatal("ran an unknown test")
	}
}

func TestNormalizeRelaySpeedtest(t *testing.T) {
	options := RelaySpeedtestOptions{}
	if err := normalizeRelaySpeedtest(&options); err != nil || len(options.Tests) != 6 || options.Seconds != 10 || options.Streams != 4 || options.UDPMbps != 50 {
		t.Fatalf("defaults: %+v %v", options, err)
	}
	options = RelaySpeedtestOptions{Tests: []string{"udp_ping", "tcp_ping"}}
	if err := normalizeRelaySpeedtest(&options); err != nil || strings.Join(options.Tests, ",") != "tcp_ping,udp_ping" {
		t.Fatalf("order: %+v %v", options, err)
	}
	for _, bad := range []RelaySpeedtestOptions{
		{Tests: []string{"ping"}}, {Seconds: 30}, {Seconds: 1}, {Streams: 9}, {UDPMbps: 5000},
	} {
		if err := normalizeRelaySpeedtest(&bad); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
	service := MonitorService{}
	if _, err := service.StartRelaySpeedtest(0, RelaySpeedtestOptions{}); err == nil {
		t.Fatal("tested the panel against itself")
	}
}
