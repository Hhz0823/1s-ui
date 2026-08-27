//go:build with_naive_outbound

package core

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	panelutil "github.com/Hhz0823/1s-ui/util"
)

func TestNaiveOutboundStartsWithGeneratedFeatures(t *testing.T) {
	initNaiveTestLogger()
	now := time.Now()
	_, certificate, err := panelutil.GenerateSelfSignedTLS("127.0.0.1", now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	config, err := json.Marshal(map[string]interface{}{
		"outbounds": []interface{}{
			map[string]interface{}{
				"type": "naive", "tag": "naive-https", "server": "127.0.0.1", "server_port": 443,
				"username": "browser", "password": "secret", "insecure_concurrency": 2,
				"extra_headers": map[string]string{"X-Edge": "stable"},
				"udp_over_tcp":  map[string]interface{}{"enabled": true},
				"tls": map[string]interface{}{
					"enabled": true, "server_name": "127.0.0.1",
					"certificate": strings.Split(strings.TrimSpace(string(certificate)), "\n"),
				},
			},
			map[string]interface{}{
				"type": "naive", "tag": "naive-quic", "server": "127.0.0.1", "server_port": 443,
				"username": "browser", "password": "secret",
				"extra_headers": map[string]string{"X-Edge": "stable"},
				"udp_over_tcp":  map[string]interface{}{"enabled": true},
				"quic":          true, "quic_congestion_control": "bbr2",
				"tls": map[string]interface{}{
					"enabled": true, "server_name": "127.0.0.1",
					"certificate": strings.Split(strings.TrimSpace(string(certificate)), "\n"),
				},
			},
			map[string]interface{}{"type": "direct", "tag": "direct"},
		},
		"route": map[string]interface{}{"final": "direct"},
	})
	if err != nil {
		t.Fatal(err)
	}
	instance := NewCore()
	if err = instance.Start(config); err != nil {
		t.Fatalf("start Naive outbound: %v", err)
	}
	if err = instance.Stop(); err != nil {
		t.Fatalf("stop Naive outbound: %v", err)
	}
}
