package service

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
)

func resetAgentMetricState() {
	agentAccMu.Lock()
	agentAcc = map[uint]*agentMinuteAcc{}
	agentAccMu.Unlock()
	agentTrafficMu.Lock()
	agentTrafficCache = map[uint]agentTrafficCacheEntry{}
	agentTrafficMu.Unlock()
}

func TestAgentMetricsStoreTrafficAcrossReboots(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "agent-metrics.db")); err != nil {
		t.Fatal(err)
	}
	resetAgentMetricState()
	service := AgentService{capacityProvider: func() (int, uint64) { return MinClusterCPUCores, MinClusterMemBytes }}
	enrollment, err := service.Create("metrics-node")
	if err != nil {
		t.Fatal(err)
	}
	reports := []agent.Report{
		{CPUPercent: 20, Network: agent.NetworkUsage{Sent: 1000, Recv: 5000}, TCPConns: 10},
		{CPUPercent: 40, Network: agent.NetworkUsage{Sent: 1500, Recv: 8000}, TCPConns: 30},
		// The counters restarted after a reboot.
		{CPUPercent: 60, Network: agent.NetworkUsage{Sent: 200, Recv: 100}, TCPConns: 20},
	}
	for _, report := range reports {
		if _, err := service.Heartbeat(enrollment.Token, "203.0.113.9", report); err != nil {
			t.Fatal(err)
		}
	}
	appendAgentLatency(enrollment.Node.Id, 40, true)
	appendAgentLatency(enrollment.Node.Id, 0, false)

	nodes, err := service.List()
	if err != nil {
		t.Fatal(err)
	}
	// The first report has no baseline, so only later deltas count.
	if nodes[0].Traffic.Sent != 700 || nodes[0].Traffic.Recv != 3100 {
		t.Fatalf("unflushed period traffic = %d/%d, want 700/3100", nodes[0].Traffic.Sent, nodes[0].Traffic.Recv)
	}

	if err := service.FlushAgentMetrics(); err != nil {
		t.Fatal(err)
	}
	var rows []model.AgentMetric
	database.GetDB().Find(&rows)
	if len(rows) != 1 {
		t.Fatalf("stored %d rows, want 1", len(rows))
	}
	row := rows[0]
	if row.Samples != 3 || row.CPU != 40 || row.TCP != 20 || row.NetSent != 700 || row.NetRecv != 3100 || row.PingMs != 40 || row.PingLoss != 50 {
		t.Fatalf("unexpected stored row: %#v", row)
	}
	nodes, _ = service.List()
	if nodes[0].Traffic.Sent != 700 || nodes[0].Traffic.Recv != 3100 {
		t.Fatalf("flushed period traffic = %d/%d", nodes[0].Traffic.Sent, nodes[0].Traffic.Recv)
	}
	points, err := service.Metrics(enrollment.Node.Id, 3600)
	if err != nil || len(points) != 1 || points[0].CPU != 40 {
		t.Fatalf("metrics = %#v, %v", points, err)
	}
	if _, err := service.Metrics(enrollment.Node.Id, 0); err == nil {
		t.Fatal("zero range accepted")
	}
}

func TestCompactAgentMetricsMergesOldMinutes(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "agent-compact.db")); err != nil {
		t.Fatal(err)
	}
	old := (time.Now().Add(-48*time.Hour).Unix() / agentMetricCoarse) * agentMetricCoarse
	rows := []model.AgentMetric{
		{NodeId: 1, Time: old, Resolution: 60, Samples: 4, CPU: 10, NetSent: 100, PingMs: 20},
		{NodeId: 1, Time: old + 60, Resolution: 60, Samples: 4, CPU: 30, NetSent: 50},
		{NodeId: 1, Time: time.Now().Add(-40 * 24 * time.Hour).Unix(), Resolution: 900, CPU: 99},
		{NodeId: 1, Time: time.Now().Unix() - 120, Resolution: 60, CPU: 50},
	}
	if err := database.GetDB().Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if err := (&AgentService{}).CompactAgentMetrics(); err != nil {
		t.Fatal(err)
	}
	var stored []model.AgentMetric
	database.GetDB().Order("time ASC").Find(&stored)
	if len(stored) != 2 {
		t.Fatalf("stored rows = %#v", stored)
	}
	merged := stored[0]
	if merged.Resolution != 900 || merged.Time != old || merged.CPU != 20 || merged.NetSent != 150 || merged.Samples != 8 || merged.PingMs != 20 {
		t.Fatalf("merged row = %#v", merged)
	}
	if stored[1].CPU != 50 || stored[1].Resolution != 60 {
		t.Fatalf("recent row changed: %#v", stored[1])
	}
}

func TestTrafficPeriod(t *testing.T) {
	loc := time.UTC
	start, next := trafficPeriod(time.Date(2026, 3, 2, 10, 0, 0, 0, loc), 5)
	if !start.Equal(time.Date(2026, 2, 5, 0, 0, 0, 0, loc)) || !next.Equal(time.Date(2026, 3, 5, 0, 0, 0, 0, loc)) {
		t.Fatalf("period = %v - %v", start, next)
	}
	start, _ = trafficPeriod(time.Date(2026, 3, 5, 0, 0, 0, 0, loc), 5)
	if !start.Equal(time.Date(2026, 3, 5, 0, 0, 0, 0, loc)) {
		t.Fatalf("reset day should start a new period, got %v", start)
	}
}

func TestNormalizeAgentMeta(t *testing.T) {
	meta, err := normalizeAgentMeta(AgentNodeMeta{Region: "jp", Tags: " CN2<red>;IPv6 "})
	if err != nil {
		t.Fatal(err)
	}
	if meta.Region != "JP" || meta.Tags != "CN2<red>;IPv6" || meta.TrafficLimitType != "sum" || meta.TrafficResetDay != 1 || meta.BillingCycle != 30 {
		t.Fatalf("normalized meta = %#v", meta)
	}
	for _, bad := range []AgentNodeMeta{
		{Region: "JPN"},
		{TrafficLimitType: "both"},
		{TrafficResetDay: 31},
		{Price: -5},
	} {
		if _, err := normalizeAgentMeta(bad); err == nil {
			t.Fatalf("accepted invalid meta %#v", bad)
		}
	}
}
