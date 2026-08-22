package service

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
)

func TestUserTrafficRankingAggregatesTimeWindow(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "user-traffic.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	if !db.Migrator().HasIndex(&model.Stats{}, "idx_stats_resource_time") {
		t.Fatal("user traffic time index was not created")
	}
	clients := []model.Client{
		{Name: "alice", Group: "paid", Desc: "primary", Enable: true},
		{Name: "bob", Group: "trial", Enable: true},
	}
	if err := db.Create(&clients).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	start := now - 600
	stats := []model.Stats{
		{Resource: "user", Tag: "alice", DateTime: now - 120, Direction: true, Traffic: 1000},
		{Resource: "user", Tag: "alice", DateTime: now - 120, Direction: false, Traffic: 2000},
		{Resource: "user", Tag: "alice", DateTime: now - 60, Direction: true, Traffic: 2000},
		{Resource: "user", Tag: "alice", DateTime: now - 60, Direction: false, Traffic: 1000},
		{Resource: "user", Tag: "bob", DateTime: now - 120, Direction: true, Traffic: 500},
		{Resource: "user", Tag: "bob", DateTime: now - 120, Direction: false, Traffic: 500},
		{Resource: "inbound", Tag: "ignored", DateTime: now - 60, Direction: true, Traffic: 999999},
	}
	if err := db.Create(&stats).Error; err != nil {
		t.Fatal(err)
	}
	setOnlineResources(onlines{User: []string{"alice"}})
	t.Cleanup(func() { setOnlineResources(onlines{}) })

	result, err := (&StatsService{}).GetUserTrafficRanking(start, now, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Enabled || result.Summary.ActiveUsers != 2 || result.Summary.TotalBytes != 7000 {
		t.Fatalf("unexpected traffic summary: %#v", result.Summary)
	}
	if result.Summary.PeakUploadBytesPerSec != 33 || result.Summary.PeakDownloadBytesPerSec != 41 {
		t.Fatalf("unexpected summary peak bandwidth: %#v", result.Summary)
	}
	if len(result.Items) != 2 || result.Items[0].Name != "alice" || result.Items[0].Rank != 1 {
		t.Fatalf("unexpected traffic ranking: %#v", result.Items)
	}
	alice := result.Items[0]
	if alice.UploadBytes != 3000 || alice.DownloadBytes != 3000 || alice.TotalBytes != 6000 {
		t.Fatalf("unexpected alice totals: %#v", alice)
	}
	if !alice.Online || !alice.Exists || !alice.Enabled || alice.Group != "paid" || alice.Description != "primary" {
		t.Fatalf("alice metadata was not joined: %#v", alice)
	}
	if alice.PeakUploadBytesPerSec != 33 || alice.PeakDownloadBytesPerSec != 33 {
		t.Fatalf("unexpected bucket peak bandwidth: %#v", alice)
	}
	if alice.AverageUploadBytesPerSec != 5 || alice.AverageDownloadBytesPerSec != 5 {
		t.Fatalf("unexpected average bandwidth: %#v", alice)
	}
}

func TestUserTrafficRankingKeepsDisabledAndDeletedUsers(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "user-traffic-states.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	if err := db.Create(&model.Client{Name: "disabled", Enable: false}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	stats := []model.Stats{
		{Resource: "user", Tag: "disabled", DateTime: now - 60, Direction: true, Traffic: 200},
		{Resource: "user", Tag: "deleted", DateTime: now - 60, Direction: false, Traffic: 100},
	}
	if err := db.Create(&stats).Error; err != nil {
		t.Fatal(err)
	}

	result, err := (&StatsService{}).GetUserTrafficRanking(now-120, now, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("unexpected traffic ranking: %#v", result.Items)
	}
	if !result.Items[0].Exists || result.Items[0].Enabled || result.Items[0].Name != "disabled" {
		t.Fatalf("disabled user state was not preserved: %#v", result.Items[0])
	}
	if result.Items[1].Exists || result.Items[1].Enabled || result.Items[1].Name != "deleted" {
		t.Fatalf("deleted user history was not preserved: %#v", result.Items[1])
	}
}

func TestGetStatsAggregatesAllUsersForTrend(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "user-traffic-trend.db")); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	start := now - 600
	stats := []model.Stats{
		{Resource: "user", Tag: "alice", DateTime: start + 60, Direction: true, Traffic: 100},
		{Resource: "user", Tag: "bob", DateTime: start + 60, Direction: true, Traffic: 200},
		{Resource: "user", Tag: "alice", DateTime: start + 60, Direction: false, Traffic: 400},
		{Resource: "user", Tag: "bob", DateTime: start + 120, Direction: false, Traffic: 500},
		{Resource: "inbound", Tag: "ignored", DateTime: start + 60, Direction: true, Traffic: 9999},
	}
	if err := database.GetDB().Create(&stats).Error; err != nil {
		t.Fatal(err)
	}

	data, err := (&StatsService{}).GetStats("user", "", 0, start, now)
	if err != nil {
		t.Fatal(err)
	}
	result := data.(map[string]any)
	buckets := result["stats"].(map[int64][]int64)
	if got := buckets[1]; len(got) != 2 || got[0] != 300 || got[1] != 400 {
		t.Fatalf("unexpected first aggregate bucket: %#v", got)
	}
	if got := buckets[2]; len(got) != 2 || got[0] != 0 || got[1] != 500 {
		t.Fatalf("unexpected second aggregate bucket: %#v", got)
	}

	data, err = (&StatsService{}).GetStats("user", "alice", 0, start, now)
	if err != nil {
		t.Fatal(err)
	}
	buckets = data.(map[string]any)["stats"].(map[int64][]int64)
	if got := buckets[1]; len(got) != 2 || got[0] != 100 || got[1] != 400 {
		t.Fatalf("unexpected selected-user bucket: %#v", got)
	}
	if _, exists := buckets[2]; exists {
		t.Fatalf("selected-user trend included another user's bucket: %#v", buckets[2])
	}
}

func TestUserTrafficRankingHonorsRetentionAndRangeLimit(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "user-traffic-disabled.db")); err != nil {
		t.Fatal(err)
	}
	settings := &SettingService{}
	if err := settings.setString("trafficAge", "0"); err != nil {
		t.Fatal(err)
	}
	result, err := (&StatsService{}).GetUserTrafficRanking(0, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.Enabled || len(result.Items) != 0 {
		t.Fatalf("disabled traffic history returned ranking data: %#v", result)
	}
	now := time.Now().Unix()
	if _, err := (&StatsService{}).GetUserTrafficRanking(now-int64(91*24*time.Hour/time.Second), now, 100); err == nil {
		t.Fatal("traffic ranking accepted a range longer than 90 days")
	}
}
