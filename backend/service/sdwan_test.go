package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
)

func TestInjectSdwanRouteRuleKeepsLeadingActions(t *testing.T) {
	route := json.RawMessage(`{"rules":[{"action":"sniff"},{"protocol":["dns"],"action":"hijack-dns"},{"ip_is_private":true,"outbound":"direct"}],"final":"direct"}`)
	updated, err := injectSdwanRouteRule(route, []string{"entry"}, false)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Rules []map[string]interface{} `json:"rules"`
		Final string                   `json:"final"`
	}
	if err = json.Unmarshal(updated, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Rules) != 4 || parsed.Rules[2]["outbound"] != SdwanGroupTag || parsed.Final != "direct" {
		t.Fatalf("SD-WAN rule must follow sniff/hijack-dns and precede custom rules: %s", updated)
	}

	updated, err = injectSdwanRouteRule(route, []string{"entry"}, true)
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(updated, &parsed)
	if parsed.Rules[len(parsed.Rules)-1]["outbound"] != SdwanGroupTag {
		t.Fatalf("rules_first must append the SD-WAN rule: %s", updated)
	}

	updated, err = injectSdwanRouteRule(nil, []string{"entry"}, false)
	if err != nil || string(updated) != `{"rules":[{"action":"route","inbound":["entry"],"outbound":"sdwan-auto"}]}` {
		t.Fatalf("empty route = %s (%v)", updated, err)
	}
}

func TestNormalizeSdwanSettingsValidatesInput(t *testing.T) {
	settings := SdwanSettings{Protocol: "hy2", EntryInbounds: []string{" a ", "a", "", "b"}}
	if err := normalizeSdwanSettings(&settings); err != nil {
		t.Fatal(err)
	}
	if settings.Protocol != SdwanProtocolHysteria2 || len(settings.EntryInbounds) != 2 ||
		settings.TestURL != defaultSdwanTestURL || settings.Interval != defaultSdwanInterval || settings.Tolerance != defaultSdwanTolerance {
		t.Fatalf("normalized = %#v", settings)
	}
	for _, invalid := range []SdwanSettings{
		{Protocol: "vmess"},
		{TestURL: "ftp://example.com"},
		{Interval: 5},
		{Tolerance: 9000},
		{EntryInbounds: []string{"bad\ntag"}},
	} {
		if err := normalizeSdwanSettings(&invalid); err == nil {
			t.Fatalf("accepted invalid settings %#v", invalid)
		}
	}
}

func TestSdwanUplinkProvisioningIsIdempotentAndRemovable(t *testing.T) {
	control := setupQuickAddTest(t)
	for _, protocol := range []string{SdwanProtocolShadowsocks, SdwanProtocolHysteria2} {
		first, err := control.ProvisionSdwanUplink(SdwanProvisionRequest{Protocol: protocol, Actor: "controller", PublicHost: "198.51.100.60"})
		if err != nil {
			t.Fatalf("provision %s: %v", protocol, err)
		}
		if err = core.ValidateOutboundJSON(first.Outbound); err != nil {
			t.Fatalf("%s uplink outbound rejected by sing-box: %v\n%s", protocol, err, first.Outbound)
		}
		var outbound map[string]interface{}
		_ = json.Unmarshal(first.Outbound, &outbound)
		if outbound["type"] != protocol || outbound["server"] != "198.51.100.60" || int(outbound["server_port"].(float64)) != first.Port {
			t.Fatalf("unexpected uplink outbound: %s", first.Outbound)
		}
		if protocol == SdwanProtocolHysteria2 {
			tlsConfig := outbound["tls"].(map[string]interface{})
			if tlsConfig["insecure"] != nil || tlsConfig["certificate"] == nil {
				t.Fatalf("Hysteria2 uplink must pin the generated certificate instead of skipping verification: %#v", tlsConfig)
			}
		}
		again, err := control.ProvisionSdwanUplink(SdwanProvisionRequest{Protocol: protocol, Actor: "controller", PublicHost: "198.51.100.60"})
		if err != nil {
			t.Fatal(err)
		}
		if string(again.Outbound) != string(first.Outbound) {
			t.Fatalf("re-provisioning changed the credentials:\n%s\n%s", first.Outbound, again.Outbound)
		}
		var inbounds, clients int64
		database.GetDB().Model(&model.Inbound{}).Where("tag = ?", sdwanUplinkTag).Count(&inbounds)
		database.GetDB().Model(&model.Client{}).Where("name = ?", sdwanUplinkClient).Count(&clients)
		if inbounds != 1 || clients != 1 {
			t.Fatalf("expected exactly one uplink inbound/client, got %d/%d", inbounds, clients)
		}
	}

	removed, err := control.RemoveSdwanUplink(SdwanRemoveRequest{Actor: "controller", PublicHost: "198.51.100.60"})
	if err != nil || !removed.Removed {
		t.Fatalf("remove uplink = %#v, %v", removed, err)
	}
	var inbounds, clients, tlsConfigs int64
	database.GetDB().Model(&model.Inbound{}).Where("tag = ?", sdwanUplinkTag).Count(&inbounds)
	database.GetDB().Model(&model.Client{}).Where("name = ?", sdwanUplinkClient).Count(&clients)
	database.GetDB().Model(&model.Tls{}).Count(&tlsConfigs)
	if inbounds != 0 || clients != 0 || tlsConfigs != 0 {
		t.Fatalf("uplink leftovers: inbounds=%d clients=%d tls=%d", inbounds, clients, tlsConfigs)
	}
}

// TestSdwanGroupRoutesThroughUplink runs a managed server's uplink and the
// controller's SD-WAN group in one sing-box instance and proves traffic flows
// through the uplink: the group's URL test must succeed via the member.
func TestSdwanGroupRoutesThroughUplink(t *testing.T) {
	control := setupQuickAddTest(t)
	probe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer probe.Close()

	uplink, err := control.ProvisionSdwanUplink(SdwanProvisionRequest{
		Protocol: SdwanProtocolShadowsocks, Actor: "controller", PublicHost: "127.0.0.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	outbound, err := sdwanMemberOutbound(7, uplink.Outbound)
	if err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	if err = db.Create(&model.SdwanMember{NodeId: 7, Protocol: uplink.Protocol, Server: uplink.Server, Port: uplink.Port, Outbound: outbound}).Error; err != nil {
		t.Fatal(err)
	}
	var entry model.Inbound
	if err = db.Where("tag = ?", sdwanUplinkTag).First(&entry).Error; err != nil {
		t.Fatal(err)
	}
	settings := defaultSdwanSettings()
	settings.Enabled = true
	settings.TestURL = probe.URL
	settings.Interval = 10
	settings.EntryInbounds = []string{"missing-entry"}
	if err = storeSdwanSettings(db, settings); err != nil {
		t.Fatal(err)
	}

	config, err := control.ConfigService.GetConfigWithDB("", db)
	if err != nil {
		t.Fatal(err)
	}
	var built struct {
		Outbounds []map[string]interface{} `json:"outbounds"`
		Route     struct {
			Rules []map[string]interface{} `json:"rules"`
		} `json:"route"`
	}
	if err = json.Unmarshal(*config, &built); err != nil {
		t.Fatal(err)
	}
	foundGroup := false
	for _, item := range built.Outbounds {
		if item["tag"] == SdwanGroupTag {
			foundGroup = true
			members, _ := item["outbounds"].([]interface{})
			if len(members) != 1 || members[0] != sdwanMemberTag(7) {
				t.Fatalf("group members = %#v", item["outbounds"])
			}
		}
	}
	if !foundGroup {
		t.Fatalf("SD-WAN group missing from config: %s", *config)
	}
	for _, rule := range built.Route.Rules {
		if rule["outbound"] == SdwanGroupTag {
			t.Fatalf("rules for inbounds that do not exist must be skipped: %#v", rule)
		}
	}

	instance := core.NewCore()
	if err = instance.Start(*config); err != nil {
		t.Fatalf("start controller+uplink config: %v\n%s", err, *config)
	}
	defer instance.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	results, err := instance.GroupURLTest(ctx, SdwanGroupTag)
	if err != nil {
		t.Fatalf("URL test through the uplink failed: %v", err)
	}
	if _, ok := results[sdwanMemberTag(7)]; !ok {
		t.Fatalf("member was not reachable through the uplink: %#v", results)
	}
	status, err := instance.GroupStatus(SdwanGroupTag)
	if err != nil || status.Now != sdwanMemberTag(7) {
		t.Fatalf("group status = %#v, %v", status, err)
	}
	if _, ok := status.Delays[sdwanMemberTag(7)]; !ok {
		t.Fatalf("shared URL-test history did not record the member delay: %#v", status)
	}
}

func TestApplySdwanConfigSkipsBrokenMembersAndCollisions(t *testing.T) {
	setupQuickAddTest(t)
	db := database.GetDB()
	valid := json.RawMessage(`{"type":"shadowsocks","server":"198.51.100.61","server_port":8388,"method":"2022-blake3-aes-128-gcm","password":"AAAAAAAAAAAAAAAAAAAAAA==:AAAAAAAAAAAAAAAAAAAAAA=="}`)
	broken := json.RawMessage(`{"type":"shadowsocks","server":"198.51.100.62","server_port":8388,"method":"2022-blake3-aes-128-gcm","password":"x","unknown":true}`)
	db.Create(&model.SdwanMember{NodeId: 1, Protocol: SdwanProtocolShadowsocks, Outbound: valid})
	db.Create(&model.SdwanMember{NodeId: 2, Protocol: SdwanProtocolShadowsocks, Outbound: broken})
	config := SingBoxConfig{Outbounds: []json.RawMessage{json.RawMessage(`{"type":"direct","tag":"direct"}`)}}
	applySdwanConfig(db, &config)
	tags := configTags(config.Outbounds)
	if tags[sdwanMemberTag(1)] != "shadowsocks" || tags[SdwanGroupTag] != "urltest" {
		t.Fatalf("valid member/group missing: %#v", tags)
	}
	if _, exists := tags[sdwanMemberTag(2)]; exists {
		t.Fatalf("broken member must be skipped: %#v", tags)
	}

	collision := SingBoxConfig{Outbounds: []json.RawMessage{json.RawMessage(`{"type":"direct","tag":"sdwan-auto"}`)}}
	applySdwanConfig(db, &collision)
	if len(collision.Outbounds) != 1 {
		t.Fatalf("SD-WAN must not touch a config whose tags collide: %d outbounds", len(collision.Outbounds))
	}
}

func freeLocalPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// TestSdwanFailsOverWhenExitBreaks proves traffic leaves a broken exit within
// one heal cycle instead of waiting for the next periodic URL test.
func TestSdwanFailsOverWhenExitBreaks(t *testing.T) {
	control := setupQuickAddTest(t)
	probe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer probe.Close()

	primary, err := control.ProvisionSdwanUplink(SdwanProvisionRequest{
		Protocol: SdwanProtocolShadowsocks, Actor: "controller", PublicHost: "127.0.0.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	primaryOutbound, err := sdwanMemberOutbound(7, primary.Outbound)
	if err != nil {
		t.Fatal(err)
	}
	backupPort := freeLocalPort(t)
	backupKey := relayShadowsocksKey(sdwanShadowsocksMethod)
	backupOutbound, err := sdwanMemberOutbound(8, json.RawMessage(fmt.Sprintf(
		`{"type":"shadowsocks","server":"127.0.0.1","server_port":%d,"method":%q,"password":%q}`,
		backupPort, sdwanShadowsocksMethod, backupKey)))
	if err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	db.Create(&model.SdwanMember{NodeId: 7, Protocol: SdwanProtocolShadowsocks, Outbound: primaryOutbound})
	db.Create(&model.SdwanMember{NodeId: 8, Protocol: SdwanProtocolShadowsocks, Outbound: backupOutbound})
	settings := defaultSdwanSettings()
	settings.TestURL = probe.URL
	if err = storeSdwanSettings(db, settings); err != nil {
		t.Fatal(err)
	}
	config, err := control.ConfigService.GetConfigWithDB("", db)
	if err != nil {
		t.Fatal(err)
	}
	instance := core.NewCore()
	if err = instance.Start(*config); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer instance.Stop()

	if err = instance.GroupCheckNow(SdwanGroupTag); err != nil {
		t.Fatal(err)
	}
	status, err := instance.GroupStatus(SdwanGroupTag)
	if err != nil || status.Now != sdwanMemberTag(7) {
		t.Fatalf("only the primary exit is up, got %#v (%v)", status, err)
	}

	// The backup server comes online and the primary one breaks.
	backupInbound := fmt.Sprintf(`{"type":"shadowsocks","tag":"backup-uplink","listen":"127.0.0.1","listen_port":%d,"method":%q,"password":%q}`,
		backupPort, sdwanShadowsocksMethod, backupKey)
	if err = instance.AddInbound([]byte(backupInbound)); err != nil {
		t.Fatal(err)
	}
	if err = instance.RemoveInbound(sdwanUplinkTag); err != nil {
		t.Fatal(err)
	}
	// A user connection through the group fails and sing-box drops the history
	// of the broken exit; the heal job must then switch without waiting.
	if result := core.CheckOutbound(context.Background(), SdwanGroupTag, probe.URL); result.OK {
		t.Fatal("connection through the broken exit unexpectedly succeeded")
	}
	switched, err := instance.GroupHeal(SdwanGroupTag)
	if err != nil || !switched {
		t.Fatalf("heal did not run: switched=%v err=%v", switched, err)
	}
	status, err = instance.GroupStatus(SdwanGroupTag)
	if err != nil || status.Now != sdwanMemberTag(8) {
		t.Fatalf("SD-WAN did not fail over to the backup exit: %#v (%v)", status, err)
	}
	if result := core.CheckOutbound(context.Background(), SdwanGroupTag, probe.URL); !result.OK {
		t.Fatalf("traffic still fails after failover: %s", result.Error)
	}
	if switched, _ = instance.GroupHeal(SdwanGroupTag); switched {
		t.Fatal("a healthy exit must not trigger another re-test")
	}
}
