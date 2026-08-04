package database

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/Hhz0823/1s-ui/database/model"
)

func TestMaxOpenConnectionsForLowResourceHosts(t *testing.T) {
	if got := maxOpenConnections(1); got != 4 {
		t.Fatalf("single-core pool = %d, want 4", got)
	}
	if got := maxOpenConnections(8); got != 8 {
		t.Fatalf("multi-core pool = %d, want 8", got)
	}
}

func TestNormalizeIPv4SharedInboundListeners(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "listen-normalization.db")); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		tag        string
		server     string
		wantListen string
	}{
		{tag: "ipv4", server: "198.51.100.20", wantListen: "0.0.0.0"},
		{tag: "ipv6", server: "2001:db8::20", wantListen: "::"},
		{tag: "domain", server: "node.example.com", wantListen: "::"},
	}
	for _, test := range tests {
		inbound := model.Inbound{
			Type: "hysteria2", Tag: test.tag, CoreType: model.CoreTypeSingBox,
			Options: json.RawMessage(`{"listen":"::","listen_port":33305}`),
			Addrs:   json.RawMessage(`[]`),
			OutJson: json.RawMessage(`{"server":"` + test.server + `","server_port":33305}`),
		}
		if err := GetDB().Create(&inbound).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := normalizeIPv4SharedInboundListeners(); err != nil {
		t.Fatal(err)
	}
	var inbounds []model.Inbound
	if err := GetDB().Order("id").Find(&inbounds).Error; err != nil {
		t.Fatal(err)
	}
	for index, inbound := range inbounds {
		var options map[string]interface{}
		if err := json.Unmarshal(inbound.Options, &options); err != nil {
			t.Fatal(err)
		}
		if got := options["listen"]; got != tests[index].wantListen {
			t.Errorf("%s listen = %v, want %s", inbound.Tag, got, tests[index].wantListen)
		}
	}
}
