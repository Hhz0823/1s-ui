package util

import (
	"encoding/json"
	"testing"

	"github.com/Hhz0823/1s-ui/database/model"
)

func TestFillNaiveOutJsonPreservesClientFeatures(t *testing.T) {
	inbound := &model.Inbound{
		Type: "naive", Tag: "naive-test", TlsId: 1,
		Options: json.RawMessage(`{"listen_port":443,"network":"udp","quic_congestion_control":"bbr2"}`),
		Addrs:   json.RawMessage(`[{"server":"edge.example.com","server_port":8443}]`),
		OutJson: json.RawMessage(`{"extra_headers":{"X-Edge":"stable"},"udp_over_tcp":{"enabled":true},"insecure_concurrency":2}`),
		Tls: &model.Tls{
			Server: json.RawMessage(`{"enabled":true,"server_name":"node.example.com","certificate":["cert"]}`),
			Client: json.RawMessage(`{"insecure":true,"alpn":["h2"],"utls":{"enabled":true},"pinned_peer_certificate_sha256":["pin"]}`),
		},
	}
	if err := FillOutJson(inbound, "node.example.com"); err != nil {
		t.Fatal(err)
	}
	var outbound map[string]interface{}
	if err := json.Unmarshal(inbound.OutJson, &outbound); err != nil {
		t.Fatal(err)
	}
	if outbound["type"] != "naive" || outbound["quic"] != true || outbound["quic_congestion_control"] != "bbr2" {
		t.Fatalf("unexpected Naive outbound: %#v", outbound)
	}
	if outbound["server"] != "edge.example.com" || int(outbound["server_port"].(float64)) != 8443 {
		t.Fatalf("server = %#v:%#v", outbound["server"], outbound["server_port"])
	}
	if int(outbound["insecure_concurrency"].(float64)) != 2 {
		t.Fatalf("insecure_concurrency = %#v", outbound["insecure_concurrency"])
	}
	if headers := outbound["extra_headers"].(map[string]interface{}); headers["X-Edge"] != "stable" {
		t.Fatalf("extra_headers = %#v", headers)
	}
	tlsConfig := outbound["tls"].(map[string]interface{})
	if tlsConfig["server_name"] != "node.example.com" || tlsConfig["certificate"] == nil {
		t.Fatalf("TLS = %#v", tlsConfig)
	}
	for _, unsupported := range []string{"insecure", "alpn", "utls", "pinned_peer_certificate_sha256"} {
		if _, exists := tlsConfig[unsupported]; exists {
			t.Fatalf("unsupported Naive TLS option %q survived: %#v", unsupported, tlsConfig)
		}
	}
}
