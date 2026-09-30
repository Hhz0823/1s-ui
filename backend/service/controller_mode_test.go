package service

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
)

func TestControllerModeDefaultsToClientAndPreservesLegacyControllers(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "controller-mode.db")); err != nil {
		t.Fatal(err)
	}
	settings := SettingService{}
	status, err := settings.GetControllerModeStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Enabled || status.Mode != "client" || status.Configured != controllerModeAuto {
		t.Fatalf("fresh panel did not default to client mode: %#v", status)
	}

	node := model.AgentNode{
		Name: "existing-agent", TokenHash: strings.Repeat("a", 64), PairCodeHash: strings.Repeat("b", 64),
		PairExpiresAt: time.Now().Add(time.Hour).Unix(), CreatedAt: time.Now().Unix(), Report: []byte(`{}`),
	}
	if err := database.GetDB().Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	status, err = settings.GetControllerModeStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled || status.Mode != "controller" || !status.Inherited || status.AgentCount != 1 {
		t.Fatalf("legacy controller was not inherited: %#v", status)
	}
}

func TestMonitorProfileRejectsControlAtServiceLayer(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "controller-monitor.db")); err != nil {
		t.Fatal(err)
	}
	settings := &SettingService{}
	if err := settings.setString(controllerModeKey, ControllerProfileMonitor); err != nil {
		t.Fatal(err)
	}
	status, err := settings.GetControllerModeStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled || !status.MonitorOnly || status.CanControl || status.Profile != ControllerProfileMonitor {
		t.Fatalf("unexpected monitor status: %#v", status)
	}
	agents := &AgentService{}
	node := model.AgentNode{Name: "unchanged", TokenHash: strings.Repeat("a", 64), CreatedAt: time.Now().Unix(), Report: []byte(`{}`)}
	if err := database.GetDB().Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := agents.Update(node.Id, "changed", ""); err == nil || !strings.Contains(err.Error(), "does not allow") {
		t.Fatalf("monitor profile allowed local node update: %v", err)
	}
	var saved model.AgentNode
	if err := database.GetDB().First(&saved, node.Id).Error; err != nil || saved.Name != "unchanged" {
		t.Fatalf("monitor update changed the node: node=%#v err=%v", saved, err)
	}
	if _, err := agents.DispatchCommand(1, agent.CmdPing, nil, "test"); err == nil || !strings.Contains(err.Error(), "does not allow") {
		t.Fatalf("monitor profile allowed command dispatch: %v", err)
	}
	if _, err := agents.DispatchBatch([]uint{1}, agent.CmdPing, nil, "test"); err == nil || !strings.Contains(err.Error(), "does not allow") {
		t.Fatalf("monitor profile allowed batch dispatch: %v", err)
	}
	if _, err := agents.AttachBrowserTerminal(1, nil, 80, 24); err == nil || !strings.Contains(err.Error(), "does not allow") {
		t.Fatalf("monitor profile allowed terminal attach: %v", err)
	}
	for _, method := range []string{
		agent.RPCMethodInboundList, agent.RPCMethodInboundEdit, agent.RPCMethodInboundSave,
		agent.RPCMethodInboundQuickAdd, agent.RPCMethodRelayGet, agent.RPCMethodRelayCreate,
		agent.RPCMethodRelayDelete, agent.RPCMethodRelayExport, agent.RPCMethodPanelAccess,
	} {
		if _, err := agents.DispatchRPC(1, method, map[string]interface{}{}, "test"); err == nil || !strings.Contains(err.Error(), "only allows read-only metrics") {
			t.Fatalf("monitor profile allowed RPC %s: %v", method, err)
		}
	}
	for _, method := range []string{agent.RPCMethodCapabilities, agent.RPCMethodPortTraffic} {
		if _, err := agents.DispatchRPC(1, method, map[string]interface{}{}, "test"); err == nil || strings.Contains(err.Error(), "only allows") {
			t.Fatalf("monitor profile rejected read-only RPC %s before session lookup: %v", method, err)
		}
	}
}

func TestDisablingControllerRevokesEnrollmentWithoutDeletingNodes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "controller-disable.db")); err != nil {
		t.Fatal(err)
	}
	settings := SettingService{}
	key, err := settings.RotateAgentEnrollmentKey()
	if err != nil {
		t.Fatal(err)
	}
	node := model.AgentNode{
		Name: "preserved-agent", TokenHash: strings.Repeat("c", 64), PairCodeHash: strings.Repeat("d", 64),
		PairExpiresAt: time.Now().Add(time.Hour).Unix(), CreatedAt: time.Now().Unix(), Report: []byte(`{}`),
	}
	if err := database.GetDB().Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	settings.OpenAgentAddressPairing(node.Id)
	status, err := settings.SetControllerMode(false)
	if err != nil {
		t.Fatal(err)
	}
	if status.Enabled || status.AgentCount != 1 || status.Configured != controllerModeDisabled {
		t.Fatalf("controller mode was not disabled safely: %#v", status)
	}
	if err := settings.ValidateAgentEnrollmentKey(key); err == nil {
		t.Fatal("reusable enrollment key remained valid after disabling controller mode")
	}
	if _, err := settings.ConsumeAgentAddressPairing(); err == nil {
		t.Fatal("address pairing window remained open after changing controller profile")
	}
	var saved model.AgentNode
	if err := database.GetDB().First(&saved, node.Id).Error; err != nil {
		t.Fatal(err)
	}
	if saved.PairCodeHash != "" || saved.PairExpiresAt != 0 {
		t.Fatalf("one-time pairing credentials were not revoked: %#v", saved)
	}
	all, err := settings.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if _, exposed := (*all)[controllerModeKey]; exposed {
		t.Fatal("controller mode internals leaked through generic settings")
	}
}

func TestLowSpecControllerRunsInLiteMode(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	t.Setenv("SUI_DISABLE_XRAY", "true")
	if err := database.InitDB(filepath.Join(dir, "controller-lite.db")); err != nil {
		t.Fatal(err)
	}
	previous := controllerCapacity
	controllerCapacity = func() (int, uint64) { return 1, 480 * 1024 * 1024 }
	t.Cleanup(func() { controllerCapacity = previous })

	settings := &SettingService{}
	status, err := settings.SetControllerProfile(ControllerProfileFull)
	if err != nil {
		t.Fatalf("1c512m host could not become a lite controller: %v", err)
	}
	if !status.Enabled || !status.CanControl || !status.Lite || !status.SingboxOnly {
		t.Fatalf("unexpected lite controller status: %#v", status)
	}
	if !liteControllerActive() {
		t.Fatal("lite controller was not reported as active")
	}

	controllerCapacity = func() (int, uint64) { return 1, 256 * 1024 * 1024 }
	if _, err := settings.SetControllerProfile(ControllerProfileMonitor); err == nil {
		t.Fatal("host below the lite floor became a controller")
	}

	controllerCapacity = func() (int, uint64) { return 2, 2 * 1024 * 1024 * 1024 }
	status, err = settings.GetControllerModeStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Lite {
		t.Fatalf("2c2G controller reported lite mode: %#v", status)
	}
}
