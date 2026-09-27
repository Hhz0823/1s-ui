package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/agent"
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
	settings := SdwanSettings{EntryInbounds: []string{" a ", "a", "", "b"}, RealityServer: " WWW.Apple.com "}
	if err := normalizeSdwanSettings(&settings); err != nil {
		t.Fatal(err)
	}
	if settings.Mode != SdwanModeAuto || len(settings.EntryInbounds) != 2 || settings.RealityServer != "www.apple.com" ||
		settings.TestURL != defaultSdwanTestURL || settings.SpeedTestURL != defaultSdwanSpeedURL ||
		settings.Interval != defaultSdwanInterval || settings.Tolerance != defaultSdwanTolerance {
		t.Fatalf("normalized = %#v", settings)
	}
	settings = SdwanSettings{Mode: "hy2"}
	if err := normalizeSdwanSettings(&settings); err != nil || settings.Mode != SdwanProtocolHysteria2 {
		t.Fatalf("hy2 mode = %q (%v)", settings.Mode, err)
	}
	for _, invalid := range []SdwanSettings{
		{Mode: "vmess"},
		{TestURL: "ftp://example.com"},
		{SpeedTestURL: "file:///etc/passwd"},
		{Interval: 5},
		{Tolerance: 9000},
		{RealityServer: "198.51.100.1"},
		{RealityServer: "example.com:443"},
		{EntryInbounds: []string{"bad\ntag"}},
	} {
		if err := normalizeSdwanSettings(&invalid); err == nil {
			t.Fatalf("accepted invalid settings %#v", invalid)
		}
	}
}

func TestSdwanProtocolsFollowModeAndCapabilities(t *testing.T) {
	all := []string{agent.CapabilitySdwanV2, agent.CapabilitySdwanReality, agent.CapabilitySdwanHysteria2}
	var want []string
	if sdwanRealitySupported {
		want = append(want, SdwanProtocolReality)
	}
	if sdwanHysteria2Supported {
		want = append(want, SdwanProtocolHysteria2)
	}
	if len(want) == 0 {
		want = []string{SdwanProtocolShadowsocks}
	}
	if got := sdwanProtocolsFor(SdwanModeAuto, all); !slices.Equal(got, want) {
		t.Fatalf("auto = %v, want %v", got, want)
	}
	ss := []string{SdwanProtocolShadowsocks}
	if got := sdwanProtocolsFor(SdwanModeAuto, []string{agent.CapabilitySdwanV2}); !slices.Equal(got, ss) {
		t.Fatalf("a server without Reality/QUIC support must fall back to Shadowsocks 2022, got %v", got)
	}
	if got := sdwanProtocolsFor(SdwanProtocolHysteria2, []string{agent.CapabilitySdwanV2}); !slices.Equal(got, ss) {
		t.Fatalf("an explicit mode the server cannot run must fall back, got %v", got)
	}
	if got := sdwanProtocolsFor(SdwanProtocolShadowsocks, all); !slices.Equal(got, ss) {
		t.Fatalf("explicit Shadowsocks = %v", got)
	}
}

func stubRealityProbe(t *testing.T, host string) {
	t.Helper()
	previous := probeRealityServer
	probeRealityServer = func(context.Context) string { return host }
	t.Cleanup(func() { probeRealityServer = previous })
}

// localRealityTarget starts a TLS 1.3 + h2 server standing in for the public
// website a Reality uplink imitates, because tests have no internet access.
func localRealityTarget(t *testing.T) int {
	t.Helper()
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.EnableHTTP2 = true
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13}
	server.StartTLS()
	t.Cleanup(server.Close)
	return server.Listener.Addr().(*net.TCPAddr).Port
}

// pointRealityAt makes every generated REALITY configuration hand
// unauthenticated handshakes to the local target instead of the public site.
func pointRealityAt(t *testing.T, port int) {
	t.Helper()
	db := database.GetDB()
	var records []model.Tls
	if err := db.Find(&records).Error; err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		var server map[string]interface{}
		if err := json.Unmarshal(record.Server, &server); err != nil {
			t.Fatal(err)
		}
		reality, _ := server["reality"].(map[string]interface{})
		if enabled, _ := reality["enabled"].(bool); !enabled {
			continue
		}
		reality["handshake"] = map[string]interface{}{"server": "127.0.0.1", "server_port": port}
		raw, _ := json.Marshal(server)
		if err := db.Model(&model.Tls{}).Where("id = ?", record.Id).Update("server", json.RawMessage(raw)).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func countSdwanUplinks(t *testing.T) (inbounds, clients, tlsConfigs int64) {
	t.Helper()
	db := database.GetDB()
	tags := []string{sdwanLegacyUplinkTag}
	for _, tag := range sdwanUplinkTags {
		tags = append(tags, tag)
	}
	db.Model(&model.Inbound{}).Where("tag IN ?", tags).Count(&inbounds)
	db.Model(&model.Client{}).Where("name = ?", sdwanUplinkClient).Count(&clients)
	db.Model(&model.Tls{}).Count(&tlsConfigs)
	return
}

func uplinkByProtocol(response *SdwanProvisionResponse, protocol string) *SdwanUplink {
	for index := range response.Uplinks {
		if response.Uplinks[index].Protocol == protocol {
			return &response.Uplinks[index]
		}
	}
	return nil
}

func TestSdwanUplinkProvisioningIsIdempotentAndRemovable(t *testing.T) {
	control := setupQuickAddTest(t)
	stubRealityProbe(t, "www.example.com")
	protocols := localSdwanProtocols()
	request := SdwanProvisionRequest{Protocols: protocols, Actor: "controller", PublicHost: "198.51.100.60"}
	first, err := control.ProvisionSdwanUplink(request)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Uplinks) != len(protocols) || first.Server != "198.51.100.60" {
		t.Fatalf("provisioned %#v for %v", first, protocols)
	}
	for _, uplink := range first.Uplinks {
		if err = core.ValidateOutboundJSON(uplink.Outbound); err != nil {
			t.Fatalf("%s uplink outbound rejected by sing-box: %v\n%s", uplink.Protocol, err, uplink.Outbound)
		}
		var outbound map[string]interface{}
		_ = json.Unmarshal(uplink.Outbound, &outbound)
		if outbound["type"] != sdwanInboundTypes[uplink.Protocol] || outbound["server"] != "198.51.100.60" ||
			int(outbound["server_port"].(float64)) != uplink.Port || uplink.Tag != sdwanUplinkTags[uplink.Protocol] {
			t.Fatalf("unexpected %s uplink: %#v %s", uplink.Protocol, uplink, uplink.Outbound)
		}
		tlsConfig, _ := outbound["tls"].(map[string]interface{})
		switch uplink.Protocol {
		case SdwanProtocolReality:
			reality, _ := tlsConfig["reality"].(map[string]interface{})
			utls, _ := tlsConfig["utls"].(map[string]interface{})
			if outbound["flow"] != "xtls-rprx-vision" || reality["public_key"] == "" || reality["short_id"] == "" ||
				utls["fingerprint"] != sdwanRealityFingerprint || tlsConfig["server_name"] != "www.example.com" || uplink.Detail != "www.example.com" {
				t.Fatalf("Reality uplink must use Vision, uTLS and the probed target: %s", uplink.Outbound)
			}
		case SdwanProtocolHysteria2:
			if tlsConfig["insecure"] != nil || tlsConfig["certificate"] == nil {
				t.Fatalf("Hysteria2 uplink must pin the generated certificate instead of skipping verification: %#v", tlsConfig)
			}
			if obfs, _ := outbound["obfs"].(map[string]interface{}); obfs["type"] != "salamander" {
				t.Fatalf("Hysteria2 uplink must be obfuscated: %s", uplink.Outbound)
			}
		case SdwanProtocolShadowsocks:
			if outbound["method"] != sdwanShadowsocksMethod {
				t.Fatalf("Shadowsocks uplink must use SS2022: %s", uplink.Outbound)
			}
		}
	}

	again, err := control.ProvisionSdwanUplink(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, uplink := range first.Uplinks {
		if repeated := uplinkByProtocol(again, uplink.Protocol); repeated == nil || string(repeated.Outbound) != string(uplink.Outbound) {
			t.Fatalf("re-provisioning changed the %s credentials", uplink.Protocol)
		}
	}
	if inbounds, clients, _ := countSdwanUplinks(t); inbounds != int64(len(protocols)) || clients != 1 {
		t.Fatalf("expected %d uplink inbounds and one client, got %d/%d", len(protocols), inbounds, clients)
	}

	// Narrowing to Shadowsocks removes the other uplinks and their TLS records
	// but keeps the Shadowsocks credentials.
	only, err := control.ProvisionSdwanUplink(SdwanProvisionRequest{
		Protocols: []string{SdwanProtocolShadowsocks}, Actor: "controller", PublicHost: "198.51.100.60",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(only.Uplinks) != 1 || string(only.Uplinks[0].Outbound) != string(uplinkByProtocol(first, SdwanProtocolShadowsocks).Outbound) {
		t.Fatalf("narrowed uplinks = %#v", only.Uplinks)
	}
	if inbounds, _, tlsConfigs := countSdwanUplinks(t); inbounds != 1 || tlsConfigs != 0 {
		t.Fatalf("leftovers after narrowing: inbounds=%d tls=%d", inbounds, tlsConfigs)
	}

	// Rotation issues new credentials.
	rotated, err := control.ProvisionSdwanUplink(SdwanProvisionRequest{
		Protocols: []string{SdwanProtocolShadowsocks}, Rotate: true, Actor: "controller", PublicHost: "198.51.100.60",
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(rotated.Uplinks[0].Outbound) == string(only.Uplinks[0].Outbound) {
		t.Fatal("rotation must replace the uplink credentials")
	}

	// A first-generation uplink is replaced by the current one.
	database.GetDB().Model(&model.Inbound{}).Where("tag = ?", sdwanUplinkTags[SdwanProtocolShadowsocks]).Update("tag", sdwanLegacyUplinkTag)
	if _, err = control.ProvisionSdwanUplink(SdwanProvisionRequest{
		Protocols: []string{SdwanProtocolShadowsocks}, Actor: "controller", PublicHost: "198.51.100.60",
	}); err != nil {
		t.Fatal(err)
	}
	var legacy int64
	database.GetDB().Model(&model.Inbound{}).Where("tag = ?", sdwanLegacyUplinkTag).Count(&legacy)
	if inbounds, _, _ := countSdwanUplinks(t); legacy != 0 || inbounds != 1 {
		t.Fatalf("legacy uplink not replaced: legacy=%d inbounds=%d", legacy, inbounds)
	}

	removed, err := control.RemoveSdwanUplink(SdwanRemoveRequest{Actor: "controller", PublicHost: "198.51.100.60"})
	if err != nil || !removed.Removed {
		t.Fatalf("remove uplink = %#v, %v", removed, err)
	}
	if inbounds, clients, tlsConfigs := countSdwanUplinks(t); inbounds != 0 || clients != 0 || tlsConfigs != 0 {
		t.Fatalf("uplink leftovers: inbounds=%d clients=%d tls=%d", inbounds, clients, tlsConfigs)
	}
}

func TestSdwanProvisioningRejectsUnsupportedRequests(t *testing.T) {
	control := setupQuickAddTest(t)
	for _, request := range []SdwanProvisionRequest{
		{Actor: "controller", PublicHost: "198.51.100.60"},
		{Protocols: []string{"vmess"}, Actor: "controller", PublicHost: "198.51.100.60"},
		{Protocols: []string{SdwanProtocolShadowsocks}, Actor: "controller"},
		{Protocols: []string{SdwanProtocolShadowsocks}, RealityServer: "10.0.0.1", Actor: "controller", PublicHost: "198.51.100.60"},
	} {
		if _, err := control.ProvisionSdwanUplink(request); err == nil {
			t.Fatalf("accepted %#v", request)
		}
	}
	for _, protocol := range []string{SdwanProtocolReality, SdwanProtocolHysteria2} {
		if sdwanProtocolAvailable(protocol) {
			continue
		}
		if _, err := control.ProvisionSdwanUplink(SdwanProvisionRequest{
			Protocols: []string{protocol}, Actor: "controller", PublicHost: "198.51.100.60",
		}); err == nil {
			t.Fatalf("a build without %s support accepted that uplink", protocol)
		}
	}
}

func sdwanPathTags(nodeID uint, protocols []string) []string {
	tags := make([]string, 0, len(protocols))
	for _, protocol := range protocols {
		tags = append(tags, sdwanPathTag(nodeID, protocol))
	}
	return tags
}

// TestSdwanPathsCarryTraffic runs a managed server's uplinks and the
// controller's SD-WAN group in one sing-box instance and proves traffic flows
// through every uplink protocol this build supports.
func TestSdwanPathsCarryTraffic(t *testing.T) {
	control := setupQuickAddTest(t)
	stubRealityProbe(t, "reality.test")
	probe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer probe.Close()

	protocols := localSdwanProtocols()
	uplinks, err := control.ProvisionSdwanUplink(SdwanProvisionRequest{Protocols: protocols, Actor: "controller", PublicHost: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	pointRealityAt(t, localRealityTarget(t))
	if err = saveSdwanMemberUplinks(7, *uplinks); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	settings := defaultSdwanSettings()
	settings.Enabled = true
	settings.TestURL = probe.URL
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
	want := sdwanPathTags(7, protocols)
	foundGroup := false
	for _, item := range built.Outbounds {
		if item["tag"] == SdwanGroupTag {
			foundGroup = true
			var members []string
			for _, member := range item["outbounds"].([]interface{}) {
				members = append(members, member.(string))
			}
			if !slices.Equal(members, want) {
				t.Fatalf("group members = %v, want %v", members, want)
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
	waitGroupReady(t, instance)
	for _, tag := range want {
		if result := core.CheckOutbound(context.Background(), tag, probe.URL); !result.OK {
			t.Fatalf("no traffic through %s: %s", tag, result.Error)
		}
	}
	if err = instance.GroupCheckNow(SdwanGroupTag); err != nil {
		t.Fatal(err)
	}
	status, err := instance.GroupStatus(SdwanGroupTag)
	if err != nil || !slices.Contains(want, status.Now) {
		t.Fatalf("group status = %#v, %v", status, err)
	}
	for _, tag := range want {
		if _, ok := status.Delays[tag]; !ok {
			t.Fatalf("shared URL-test history did not record %s: %#v", tag, status.Delays)
		}
	}
}

func sdwanTestPaths(t *testing.T, paths ...SdwanPath) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(paths)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestApplySdwanConfigSkipsBrokenPathsAndCollisions(t *testing.T) {
	setupQuickAddTest(t)
	db := database.GetDB()
	valid := json.RawMessage(`{"type":"shadowsocks","server":"198.51.100.61","server_port":8388,"method":"2022-blake3-aes-128-gcm","password":"AAAAAAAAAAAAAAAAAAAAAA==:AAAAAAAAAAAAAAAAAAAAAA=="}`)
	broken := json.RawMessage(`{"type":"shadowsocks","server":"198.51.100.62","server_port":8388,"method":"2022-blake3-aes-128-gcm","password":"x","unknown":true}`)
	db.Create(&model.SdwanMember{NodeId: 1, Paths: sdwanTestPaths(t, SdwanPath{Protocol: SdwanProtocolShadowsocks, Outbound: valid})})
	db.Create(&model.SdwanMember{NodeId: 2, Paths: sdwanTestPaths(t, SdwanPath{Protocol: SdwanProtocolShadowsocks, Outbound: broken})})
	db.Create(&model.SdwanMember{NodeId: 3, Paths: sdwanTestPaths(t, SdwanPath{Protocol: SdwanProtocolShadowsocks, Outbound: valid, Disabled: true})})
	config := SingBoxConfig{Outbounds: []json.RawMessage{json.RawMessage(`{"type":"direct","tag":"direct"}`)}}
	applySdwanConfig(db, &config)
	tags := configTags(config.Outbounds)
	if tags[sdwanPathTag(1, SdwanProtocolShadowsocks)] != "shadowsocks" || tags[SdwanGroupTag] != "urltest" {
		t.Fatalf("valid path/group missing: %#v", tags)
	}
	if _, exists := tags[sdwanPathTag(2, SdwanProtocolShadowsocks)]; exists {
		t.Fatalf("broken path must be skipped: %#v", tags)
	}
	if tags[sdwanPathTag(3, SdwanProtocolShadowsocks)] != "shadowsocks" {
		t.Fatalf("a disabled path must stay dialable so detection can re-test it: %#v", tags)
	}
	for _, raw := range config.Outbounds {
		var group struct {
			Tag       string   `json:"tag"`
			Outbounds []string `json:"outbounds"`
		}
		if json.Unmarshal(raw, &group) == nil && group.Tag == SdwanGroupTag &&
			!slices.Equal(group.Outbounds, []string{sdwanPathTag(1, SdwanProtocolShadowsocks)}) {
			t.Fatalf("only enabled paths may join the group: %v", group.Outbounds)
		}
	}

	collision := SingBoxConfig{Outbounds: []json.RawMessage{json.RawMessage(`{"type":"direct","tag":"sdwan-auto"}`)}}
	applySdwanConfig(db, &collision)
	if len(collision.Outbounds) != 1 {
		t.Fatalf("SD-WAN must not touch a config whose tags collide: %d outbounds", len(collision.Outbounds))
	}

	db.Where("1 = 1").Delete(&model.SdwanMember{})
	db.Create(&model.SdwanMember{NodeId: 3, Paths: sdwanTestPaths(t, SdwanPath{Protocol: SdwanProtocolShadowsocks, Outbound: valid, Disabled: true})})
	allDisabled := SingBoxConfig{}
	applySdwanConfig(db, &allDisabled)
	tags = configTags(allDisabled.Outbounds)
	if _, exists := tags[SdwanGroupTag]; exists || len(tags) != 1 {
		t.Fatalf("with every path disabled there is no group, only dialable paths: %#v", tags)
	}
}

// waitGroupReady waits for the URL test sing-box runs when the group starts;
// until it finishes a forced re-test is skipped.
func waitGroupReady(t *testing.T, instance *core.Core) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if status, err := instance.GroupStatus(SdwanGroupTag); err == nil && status.Now != "" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the SD-WAN group never finished its first URL test")
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
		Protocols: []string{SdwanProtocolShadowsocks}, Actor: "controller", PublicHost: "127.0.0.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = saveSdwanMemberUplinks(7, *primary); err != nil {
		t.Fatal(err)
	}
	backupPort := freeLocalPort(t)
	backupKey := relayShadowsocksKey(sdwanShadowsocksMethod)
	backup := SdwanProvisionResponse{Server: "127.0.0.1", Uplinks: []SdwanUplink{{
		Protocol: SdwanProtocolShadowsocks, Port: backupPort,
		Outbound: json.RawMessage(fmt.Sprintf(`{"type":"shadowsocks","server":"127.0.0.1","server_port":%d,"method":%q,"password":%q}`,
			backupPort, sdwanShadowsocksMethod, backupKey)),
	}}}
	if err = saveSdwanMemberUplinks(8, backup); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
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

	primaryTag, backupTag := sdwanPathTag(7, SdwanProtocolShadowsocks), sdwanPathTag(8, SdwanProtocolShadowsocks)
	waitGroupReady(t, instance)
	if err = instance.GroupCheckNow(SdwanGroupTag); err != nil {
		t.Fatal(err)
	}
	status, err := instance.GroupStatus(SdwanGroupTag)
	if err != nil || status.Now != primaryTag {
		t.Fatalf("only the primary exit is up, got %#v (%v)", status, err)
	}

	// The backup server comes online and the primary one breaks.
	backupInbound := fmt.Sprintf(`{"type":"shadowsocks","tag":"backup-uplink","listen":"127.0.0.1","listen_port":%d,"method":%q,"password":%q}`,
		backupPort, sdwanShadowsocksMethod, backupKey)
	if err = instance.AddInbound([]byte(backupInbound)); err != nil {
		t.Fatal(err)
	}
	if err = instance.RemoveInbound(sdwanUplinkTags[SdwanProtocolShadowsocks]); err != nil {
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
	if err != nil || status.Now != backupTag {
		t.Fatalf("SD-WAN did not fail over to the backup exit: %#v (%v)", status, err)
	}
	if result := core.CheckOutbound(context.Background(), SdwanGroupTag, probe.URL); !result.OK {
		t.Fatalf("traffic still fails after failover: %s", result.Error)
	}
	if switched, _ = instance.GroupHeal(SdwanGroupTag); switched {
		t.Fatal("a healthy exit must not trigger another re-test")
	}
}

func TestSaveSdwanMemberUplinksKeepsDisabledPaths(t *testing.T) {
	setupQuickAddTest(t)
	outbound := json.RawMessage(`{"type":"shadowsocks","server":"198.51.100.61","server_port":8388,"method":"2022-blake3-aes-128-gcm","password":"AAAAAAAAAAAAAAAAAAAAAA==:AAAAAAAAAAAAAAAAAAAAAA=="}`)
	response := SdwanProvisionResponse{Server: "198.51.100.61", Uplinks: []SdwanUplink{{Protocol: SdwanProtocolShadowsocks, Port: 8388, Outbound: outbound}}}
	if err := saveSdwanMemberUplinks(5, response); err != nil {
		t.Fatal(err)
	}
	if err := updateSdwanPathState(5, SdwanProtocolShadowsocks, true, "unreachable"); err != nil {
		t.Fatal(err)
	}
	load := func() SdwanPath {
		t.Helper()
		var member model.SdwanMember
		if err := database.GetDB().Where("node_id = ?", 5).First(&member).Error; err != nil {
			t.Fatal(err)
		}
		paths := memberPaths(member)
		if len(paths) != 1 {
			t.Fatalf("paths = %#v", paths)
		}
		return paths[0]
	}
	// Re-syncing an unchanged uplink must not put a disabled path back.
	if err := saveSdwanMemberUplinks(5, response); err != nil {
		t.Fatal(err)
	}
	if path := load(); !path.Disabled || path.Reason != "unreachable" {
		t.Fatalf("unchanged uplink lost its disabled flag: %#v", path)
	}
	// A rebuilt uplink (new port) gets a fresh chance.
	response.Uplinks[0].Port = 9388
	response.Uplinks[0].Outbound = json.RawMessage(`{"type":"shadowsocks","server":"198.51.100.61","server_port":9388,"method":"2022-blake3-aes-128-gcm","password":"AAAAAAAAAAAAAAAAAAAAAA==:AAAAAAAAAAAAAAAAAAAAAA=="}`)
	if err := saveSdwanMemberUplinks(5, response); err != nil {
		t.Fatal(err)
	}
	if path := load(); path.Disabled || path.Port != 9388 {
		t.Fatalf("rebuilt uplink must be enabled again: %#v", path)
	}
}
