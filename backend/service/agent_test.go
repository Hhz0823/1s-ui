package service

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
)

func TestAgentPairingCodeIsSingleUseAndRotatesToken(t *testing.T) {
	dir := t.TempDir()
	if err := database.InitDB(filepath.Join(dir, "agent-pairing.db")); err != nil {
		t.Fatal(err)
	}
	service := AgentService{
		capacityProvider: func() (int, uint64) {
			return MinClusterCPUCores, MinClusterMemBytes
		},
	}
	enrollment, err := service.Create("pairing-node")
	if err != nil {
		t.Fatal(err)
	}
	if enrollment.PairCode == "" || enrollment.PairExpiresAt <= time.Now().Unix() {
		t.Fatalf("missing pairing details: %#v", enrollment)
	}
	if _, err := service.AuthenticateToken(enrollment.Token); err != nil {
		t.Fatalf("temporary token should be valid before pairing: %v", err)
	}

	paired, err := service.ConsumePairingCode(enrollment.PairCode)
	if err != nil {
		t.Fatal(err)
	}
	if paired.Token == "" || paired.Token == enrollment.Token || paired.Node.Id != enrollment.Node.Id {
		t.Fatalf("pairing did not rotate credentials: %#v", paired)
	}
	if _, err := service.AuthenticateToken(enrollment.Token); err == nil {
		t.Fatal("temporary token remained valid after pairing")
	}
	if _, err := service.AuthenticateToken(paired.Token); err != nil {
		t.Fatalf("paired token is invalid: %v", err)
	}
	if _, err := service.ConsumePairingCode(enrollment.PairCode); err == nil {
		t.Fatal("single-use pairing code was accepted twice")
	}
}

func TestAgentAddressReconnectRotatesExistingNode(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "agent-address-reconnect.db")); err != nil {
		t.Fatal(err)
	}
	service := AgentService{capacityProvider: func() (int, uint64) {
		return MinClusterCPUCores, MinClusterMemBytes
	}}
	first, err := service.CreateConnected("address-node")
	if err != nil {
		t.Fatal(err)
	}
	reconnected, err := service.Reconnect(first.Node.Id)
	if err != nil {
		t.Fatal(err)
	}
	if reconnected.Node.Id != first.Node.Id || reconnected.Token == first.Token || reconnected.PairCode != "" {
		t.Fatalf("unexpected reconnect result: %#v", reconnected)
	}
	if _, err := service.AuthenticateToken(first.Token); err == nil {
		t.Fatal("old Agent token remained valid after reconnect")
	}
	if _, err := service.AuthenticateToken(reconnected.Token); err != nil {
		t.Fatalf("new Agent token is invalid: %v", err)
	}
	var count int64
	if err := database.GetDB().Model(&model.AgentNode{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("reconnect created duplicate nodes: count=%d err=%v", count, err)
	}
}

func TestAgentPairingCodeExpires(t *testing.T) {
	dir := t.TempDir()
	if err := database.InitDB(filepath.Join(dir, "agent-pairing-expiry.db")); err != nil {
		t.Fatal(err)
	}
	service := AgentService{
		capacityProvider: func() (int, uint64) {
			return MinClusterCPUCores, MinClusterMemBytes
		},
	}
	enrollment, err := service.Create("expired-pairing")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Model(&model.AgentNode{}).
		Where("id = ?", enrollment.Node.Id).
		Update("pair_expires_at", time.Now().Add(-time.Minute).Unix()).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConsumePairingCode(enrollment.PairCode); err == nil {
		t.Fatal("expired pairing code was accepted")
	}
}

func TestAgentCreateConnectedHasNoPairingCode(t *testing.T) {
	dir := t.TempDir()
	if err := database.InitDB(filepath.Join(dir, "agent-connected.db")); err != nil {
		t.Fatal(err)
	}
	service := AgentService{
		capacityProvider: func() (int, uint64) {
			return MinClusterCPUCores, MinClusterMemBytes
		},
	}
	enrollment, err := service.CreateConnected("connected-node")
	if err != nil {
		t.Fatal(err)
	}
	if enrollment.PairCode != "" || enrollment.PairExpiresAt != 0 {
		t.Fatalf("connected enrollment left pairing credentials: %#v", enrollment)
	}
	if _, err := service.AuthenticateToken(enrollment.Token); err != nil {
		t.Fatalf("connected enrollment token is invalid: %v", err)
	}
}

func TestAgentEnrollmentKeyRotationAndValidation(t *testing.T) {
	dir := t.TempDir()
	if err := database.InitDB(filepath.Join(dir, "agent-enrollment-key.db")); err != nil {
		t.Fatal(err)
	}
	settings := SettingService{}
	first, err := settings.RotateAgentEnrollmentKey()
	if err != nil {
		t.Fatal(err)
	}
	if err := settings.ValidateAgentEnrollmentKey(first); err != nil {
		t.Fatalf("fresh enrollment key was rejected: %v", err)
	}
	if err := settings.ValidateAgentEnrollmentKey("wrong-credential-value-that-is-long-enough-0000"); err == nil {
		t.Fatal("invalid enrollment key was accepted")
	}

	all, err := settings.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if _, exposed := (*all)[agentEnrollmentKey]; exposed {
		t.Fatal("enrollment key hash was exposed through settings")
	}

	second, err := settings.RotateAgentEnrollmentKey()
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatal("enrollment key rotation returned the same credential")
	}
	if err := settings.ValidateAgentEnrollmentKey(first); err == nil {
		t.Fatal("rotated enrollment key remained valid")
	}
	if err := settings.ValidateAgentEnrollmentKey(second); err != nil {
		t.Fatalf("rotated enrollment key was rejected: %v", err)
	}
}

func TestAgentEnrollmentHeartbeatAndRotation(t *testing.T) {
	dir := t.TempDir()
	if err := database.InitDB(filepath.Join(dir, "agents.db")); err != nil {
		t.Fatal(err)
	}
	service := AgentService{
		capacityProvider: func() (int, uint64) {
			return MinClusterCPUCores, MinClusterMemBytes
		},
	}
	enrollment, err := service.Create("edge-1")
	if err != nil {
		t.Fatal(err)
	}
	if enrollment.Token == "" || enrollment.Node.Name != "edge-1" {
		t.Fatalf("unexpected enrollment: %#v", enrollment)
	}
	report := agent.Report{Hostname: "vps-1", OS: "linux", Arch: "amd64", AgentVersion: "test", CPUPercent: 12.5, ConnMode: "http"}
	resp, err := service.Heartbeat(enrollment.Token, "203.0.113.10", report)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil || resp.ServerTime == 0 {
		t.Fatalf("expected heartbeat response, got %#v", resp)
	}
	nodes, err := service.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || !nodes[0].Online || nodes[0].Report.Hostname != "vps-1" {
		t.Fatalf("unexpected node status: %#v", nodes)
	}
	if nodes[0].ConnMode != "http" {
		t.Fatalf("expected conn_mode http, got %#v", nodes[0])
	}

	detail, err := service.Get(nodes[0].Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.History) == 0 {
		t.Fatal("expected metric history after heartbeat")
	}
	updated, err := service.Update(enrollment.Node.Id, AgentUpdate{Name: "edge-renamed", PublicHost: "node.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "edge-renamed" || updated.PublicHost != "node.example.com" {
		t.Fatalf("unexpected updated node: %#v", updated)
	}
	if _, err := service.AuthenticateToken(enrollment.Token); err != nil {
		t.Fatalf("renaming disconnected or rotated the agent token: %v", err)
	}

	rotated, err := service.Rotate(enrollment.Node.Id)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.Token == enrollment.Token {
		t.Fatal("rotated token did not change")
	}
	if _, err := service.Heartbeat(enrollment.Token, "203.0.113.10", report); err == nil {
		t.Fatal("old agent token remained valid after rotation")
	}
	if _, err := service.Heartbeat(rotated.Token, "203.0.113.10", report); err != nil {
		t.Fatalf("rotated token failed: %v", err)
	}

	if time.Since(time.Unix(detail.LastSeen, 0)) > agentOnlineWindow {
		t.Fatal("last seen unexpectedly old")
	}
}

func TestAgentHeartbeatReplacesLiveMetricsAndKeepsHistory(t *testing.T) {
	dir := t.TempDir()
	if err := database.InitDB(filepath.Join(dir, "agent-live-metrics.db")); err != nil {
		t.Fatal(err)
	}
	service := AgentService{
		capacityProvider: func() (int, uint64) {
			return MinClusterCPUCores, MinClusterMemBytes
		},
	}
	enrollment, err := service.Create("live-metrics")
	if err != nil {
		t.Fatal(err)
	}

	first := agent.Report{
		Hostname: "metrics-vps", CPUPercent: 10,
		Memory: agent.ResourceUsage{Used: 256, Total: 1024},
	}
	if _, err := service.Heartbeat(enrollment.Token, "203.0.113.40", first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.CPUPercent = 67.5
	second.Memory.Used = 768
	second.Swap = agent.ResourceUsage{Used: 128, Total: 512}
	second.Disk = agent.ResourceUsage{Used: 300, Total: 1200}
	second.ProcessCount = 42
	second.NetRate = agent.NetworkUsage{Sent: 1234, Recv: 5678}
	if _, err := service.Heartbeat(enrollment.Token, "203.0.113.40", second); err != nil {
		t.Fatal(err)
	}

	detail, err := service.Get(enrollment.Node.Id)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Report.CPUPercent != second.CPUPercent || detail.Report.Memory.Used != second.Memory.Used {
		t.Fatalf("detail returned stale metrics: %#v", detail.Report)
	}
	if len(detail.History) < 2 {
		t.Fatalf("expected both metric samples, got %#v", detail.History)
	}
	last := detail.History[len(detail.History)-1]
	if last.CPUPercent != second.CPUPercent || last.MemPercent != 75 || last.SwapPercent != 25 || last.DiskPercent != 25 || last.ProcessCount != 42 || last.NetSent != 1234 || last.NetRecv != 5678 {
		t.Fatalf("latest history sample is stale: %#v", last)
	}
}

func TestAgentListAcceptsLegacyTextReport(t *testing.T) {
	dir := t.TempDir()
	if err := database.InitDB(filepath.Join(dir, "agents-text-report.db")); err != nil {
		t.Fatal(err)
	}
	result := database.GetDB().Exec(`INSERT INTO agent_nodes
		(name, token_hash, created_at, last_seen, remote_ip, version, report)
		VALUES (?, ?, ?, ?, ?, ?, CAST(? AS TEXT))`,
		"legacy-agent", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		time.Now().Unix(), time.Now().Unix(), "203.0.113.20", "legacy",
		`{"hostname":"legacy-vps","conn_mode":"http"}`)
	if result.Error != nil {
		t.Fatal(result.Error)
	}

	nodes, err := (&AgentService{}).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Report.Hostname != "legacy-vps" || nodes[0].ConnMode != "http" {
		t.Fatalf("unexpected legacy node: %#v", nodes)
	}
}

func TestAgentNameValidation(t *testing.T) {
	for _, name := range []string{"", "\n", string(make([]byte, 81))} {
		if _, err := normalizeAgentName(name); err == nil {
			t.Fatalf("accepted invalid agent name %q", name)
		}
	}
}

func TestAgentPublicHostValidation(t *testing.T) {
	for _, value := range []string{"https://example.com", "example.com:443", "example.com/path", "bad host"} {
		if _, err := normalizeAgentPublicHost(value); err == nil {
			t.Fatalf("accepted invalid public host %q", value)
		}
	}
	for _, value := range []string{"example.com", "203.0.113.10", "[2001:db8::10]", ""} {
		if _, err := normalizeAgentPublicHost(value); err != nil {
			t.Fatalf("rejected valid public host %q: %v", value, err)
		}
	}
}

func TestAgentLatencyAggregation(t *testing.T) {
	const nodeID = uint(987654)
	agentLatencyMu.Lock()
	delete(agentLatencies, nodeID)
	agentLatencyMu.Unlock()
	t.Cleanup(func() {
		agentLatencyMu.Lock()
		delete(agentLatencies, nodeID)
		agentLatencyMu.Unlock()
	})
	appendAgentLatency(nodeID, 20, true)
	appendAgentLatency(nodeID, 40, true)
	appendAgentLatency(nodeID, 0, false)
	view := getAgentLatency(nodeID)
	if view.LastMS == nil || *view.LastMS != 40 || view.AverageMS != 30 || view.LossPct < 33 || view.LossPct > 34 {
		t.Fatalf("unexpected latency aggregate: %#v", view)
	}
}

func TestClusterRequirementBoundary(t *testing.T) {
	tests := []struct {
		name string
		cpu  int
		mem  uint64
		ok   bool
	}{
		{name: "minimum", cpu: 2, mem: 2 * 1024 * 1024 * 1024, ok: true},
		{name: "one cpu", cpu: 1, mem: 4 * 1024 * 1024 * 1024, ok: false},
		{name: "under two gib", cpu: 4, mem: 2*1024*1024*1024 - 1, ok: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := meetsClusterRequirements(test.cpu, test.mem); got != test.ok {
				t.Fatalf("meetsClusterRequirements(%d, %d) = %v, want %v", test.cpu, test.mem, got, test.ok)
			}
		})
	}
}

func TestBuildHostRequirementsKeepsMinimalPanelAvailable(t *testing.T) {
	requirements := buildHostRequirements(1, 512*1024*1024, 0)
	if requirements["mode"] != "panel" || requirements["applies"] != false || requirements["ok"] != true {
		t.Fatalf("minimal panel should remain available on 1c512m: %#v", requirements)
	}
	if requirements["can_enable_agents"] != false {
		t.Fatalf("1c512m must not enable the Agent control plane: %#v", requirements)
	}
}

func TestAgentCreateRejectsUnderSpecControlPlane(t *testing.T) {
	service := AgentService{
		capacityProvider: func() (int, uint64) {
			return 1, 4 * 1024 * 1024 * 1024
		},
	}
	if _, err := service.Create("edge-1"); err == nil {
		t.Fatal("under-spec control plane created a server Agent")
	}
}
