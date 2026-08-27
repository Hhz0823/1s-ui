package core

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/logger"
	panelutil "github.com/Hhz0823/1s-ui/util"
	"github.com/op/go-logging"
)

func TestNaiveInboundStartsWithGeneratedTLS(t *testing.T) {
	initNaiveTestLogger()
	now := time.Now()
	privateKey, certificate, err := panelutil.GenerateSelfSignedTLS("127.0.0.1", now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	config, err := json.Marshal(map[string]interface{}{
		"inbounds": []interface{}{map[string]interface{}{
			"type": "naive", "tag": "naive-test", "listen": "127.0.0.1", "listen_port": 0,
			"users": []interface{}{map[string]interface{}{"username": "browser", "password": "secret"}},
			"tls": map[string]interface{}{
				"enabled":     true,
				"key":         strings.Split(strings.TrimSpace(string(privateKey)), "\n"),
				"certificate": strings.Split(strings.TrimSpace(string(certificate)), "\n"),
			},
		}},
		"outbounds": []interface{}{map[string]interface{}{"type": "direct", "tag": "direct"}},
		"route":     map[string]interface{}{"final": "direct"},
	})
	if err != nil {
		t.Fatal(err)
	}
	instance := NewCore()
	if err = instance.Start(config); err != nil {
		t.Fatalf("start Naive inbound: %v", err)
	}
	if err = instance.Stop(); err != nil {
		t.Fatalf("stop Naive inbound: %v", err)
	}
}

func initNaiveTestLogger() {
	logger.InitLogger(logging.CRITICAL)
}
