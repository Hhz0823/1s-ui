package database

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/Hhz0823/1s-ui/database/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMaxOpenConnectionsForLowResourceHosts(t *testing.T) {
	if got := maxOpenConnections(1); got != 4 {
		t.Fatalf("single-core pool = %d, want 4", got)
	}
	if got := maxOpenConnections(8); got != 8 {
		t.Fatalf("multi-core pool = %d, want 8", got)
	}
}

func TestInitDBLeavesFreshPanelUninitialized(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "fresh.db")); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := GetDB().Model(&model.User{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("fresh database contains %d default users, want 0", count)
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

func TestInitDBMigratesSingleUplinkSdwanMembers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-sdwan.db")
	legacy, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE sdwan_members (id integer PRIMARY KEY AUTOINCREMENT, node_id integer NOT NULL, protocol text, server text, port integer, outbound text, created_at integer, updated_at integer)`,
		`CREATE UNIQUE INDEX idx_sdwan_members_node_id ON sdwan_members(node_id)`,
		`INSERT INTO sdwan_members (node_id, protocol, server, port, outbound) VALUES (3, 'shadowsocks', '198.51.100.3', 8388, '{"type":"shadowsocks","tag":"sdwan-node-3","server":"198.51.100.3","server_port":8388}')`,
		`INSERT INTO sdwan_members (node_id, protocol, server, port, outbound) VALUES (4, 'shadowsocks', '198.51.100.4', 8388, '')`,
	}
	for _, statement := range statements {
		if err = legacy.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	if sqlDB, dbErr := legacy.DB(); dbErr == nil {
		_ = sqlDB.Close()
	}

	if err = InitDB(path); err != nil {
		t.Fatal(err)
	}
	var members []model.SdwanMember
	if err = GetDB().Order("node_id").Find(&members).Error; err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 {
		t.Fatalf("members = %d", len(members))
	}
	var paths []struct {
		Protocol string          `json:"protocol"`
		Port     int             `json:"port"`
		Outbound json.RawMessage `json:"outbound"`
	}
	if err = json.Unmarshal(members[0].Paths, &paths); err != nil || len(paths) != 1 {
		t.Fatalf("legacy uplink was not converted into a path: %s (%v)", members[0].Paths, err)
	}
	if paths[0].Protocol != "shadowsocks" || paths[0].Port != 8388 || !json.Valid(paths[0].Outbound) {
		t.Fatalf("converted path = %#v", paths[0])
	}
	if len(members[1].Paths) != 0 && string(members[1].Paths) != "null" {
		t.Fatalf("a member without an outbound must stay empty: %s", members[1].Paths)
	}

	// Running the migration again must not touch converted members.
	before := string(members[0].Paths)
	if err = migrateLegacySdwanMembers(); err != nil {
		t.Fatal(err)
	}
	var again model.SdwanMember
	GetDB().Where("node_id = ?", 3).First(&again)
	if string(again.Paths) != before {
		t.Fatalf("second migration changed paths: %s", again.Paths)
	}
}
