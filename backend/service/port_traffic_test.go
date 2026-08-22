package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
)

func TestPortTrafficTotalsDoNotDoublePersistedStats(t *testing.T) {
	upload, download := portTrafficTotals(
		trafficTotalsByDirection{upload: 100, download: 200},
		core.InboundTrafficSnapshot{
			UploadPending: 20, DownloadPending: 30,
			UploadSession: 120, DownloadSession: 230,
		},
		true,
	)
	if upload != 120 || download != 230 {
		t.Fatalf("persisted traffic was double counted: upload=%d download=%d", upload, download)
	}
}

func TestPersistedInboundTrafficCacheAndPortFilter(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "port-traffic.db")); err != nil {
		t.Fatal(err)
	}
	invalidatePersistedInboundTraffic()
	stats := model.Stats{Resource: "inbound", Tag: "port", DateTime: 1, Direction: true, Traffic: 100}
	if err := database.GetDB().Create(&stats).Error; err != nil {
		t.Fatal(err)
	}
	first, err := persistedInboundTraffic()
	if err != nil {
		t.Fatal(err)
	}
	if first["port"].upload != 100 {
		t.Fatalf("unexpected first aggregate: %#v", first)
	}
	stats = model.Stats{Resource: "inbound", Tag: "port", DateTime: 2, Direction: true, Traffic: 50}
	if err := database.GetDB().Create(&stats).Error; err != nil {
		t.Fatal(err)
	}
	cached, err := persistedInboundTraffic()
	if err != nil {
		t.Fatal(err)
	}
	if cached["port"].upload != 100 {
		t.Fatalf("aggregate cache was bypassed: %#v", cached)
	}
	invalidatePersistedInboundTraffic()
	refreshed, err := persistedInboundTraffic()
	if err != nil {
		t.Fatal(err)
	}
	if refreshed["port"].upload != 150 {
		t.Fatalf("invalidated aggregate was not refreshed: %#v", refreshed)
	}

	for _, inbound := range []model.Inbound{
		{Type: "tun", Tag: "tun", Options: json.RawMessage(`{"interface_name":"tun0"}`)},
		{Type: "mixed", Tag: "port", Options: json.RawMessage(`{"listen":"::","listen_port":2095}`)},
	} {
		if err := database.GetDB().Create(&inbound).Error; err != nil {
			t.Fatal(err)
		}
	}
	oldCore := corePtr
	corePtr = nil
	t.Cleanup(func() { corePtr = oldCore })
	response, err := (&PortTrafficService{}).GetPortTraffic()
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 1 || response.Items[0].Tag != "port" || response.Items[0].Port != 2095 {
		t.Fatalf("non-port inbound was not filtered: %#v", response.Items)
	}
}

func TestOnlineResourcesReturnsSnapshot(t *testing.T) {
	setOnlineResources(onlines{Inbound: []string{"one"}})
	first, err := (&StatsService{}).GetOnlines()
	if err != nil {
		t.Fatal(err)
	}
	first.Inbound[0] = "mutated"
	second, _ := (&StatsService{}).GetOnlines()
	if len(second.Inbound) != 1 || second.Inbound[0] != "one" {
		t.Fatalf("GetOnlines returned shared state: %#v", second)
	}
}
