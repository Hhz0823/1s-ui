package service

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
)

func TestTrafficPeriodStart(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)
	cases := []struct {
		now   time.Time
		day   int
		start time.Time
		next  time.Time
	}{
		{time.Date(2026, 9, 30, 12, 0, 0, 0, loc), 1, time.Date(2026, 9, 1, 0, 0, 0, 0, loc), time.Date(2026, 10, 1, 0, 0, 0, 0, loc)},
		{time.Date(2026, 9, 10, 12, 0, 0, 0, loc), 15, time.Date(2026, 8, 15, 0, 0, 0, 0, loc), time.Date(2026, 9, 15, 0, 0, 0, 0, loc)},
		{time.Date(2026, 9, 15, 0, 0, 0, 0, loc), 15, time.Date(2026, 9, 15, 0, 0, 0, 0, loc), time.Date(2026, 10, 15, 0, 0, 0, 0, loc)},
		// Short months reset on their last day.
		{time.Date(2027, 2, 28, 1, 0, 0, 0, loc), 31, time.Date(2027, 2, 28, 0, 0, 0, 0, loc), time.Date(2027, 3, 31, 0, 0, 0, 0, loc)},
		{time.Date(2027, 2, 27, 1, 0, 0, 0, loc), 31, time.Date(2027, 1, 31, 0, 0, 0, 0, loc), time.Date(2027, 2, 28, 0, 0, 0, 0, loc)},
		{time.Date(2027, 1, 5, 1, 0, 0, 0, loc), 10, time.Date(2026, 12, 10, 0, 0, 0, 0, loc), time.Date(2027, 1, 10, 0, 0, 0, 0, loc)},
	}
	for _, tc := range cases {
		start := trafficPeriodStart(tc.now, tc.day)
		if !start.Equal(tc.start) {
			t.Errorf("start(%s, %d) = %s, want %s", tc.now, tc.day, start, tc.start)
		}
		if next := nextTrafficReset(start, tc.day); !next.Equal(tc.next) {
			t.Errorf("next(%s, %d) = %s, want %s", start, tc.day, next, tc.next)
		}
	}
}

func setupLimitDB(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "limits.db")); err != nil {
		t.Fatal(err)
	}
	oldCore := corePtr
	corePtr = core.NewCore()
	t.Cleanup(func() { corePtr = oldCore })
}

func TestInboundTrafficLimitSaveKeepsUsage(t *testing.T) {
	setupLimitDB(t)
	service := &InboundService{}
	db := database.GetDB()
	err := service.Save(db, "new", json.RawMessage(
		`{"type":"direct","tag":"capped","listen":"::","listen_port":2345,"traffic_limit":1000,"traffic_reset_day":5,"ip_limit":2,"traffic_used":999}`,
	), "", "")
	if err != nil {
		t.Fatal(err)
	}
	var saved model.Inbound
	if err = db.Where("tag = ?", "capped").First(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if saved.TrafficLimit != 1000 || saved.TrafficResetDay != 5 || saved.IPLimit != 2 {
		t.Fatalf("limits not stored: %+v", saved)
	}
	if saved.TrafficUsed != 0 || saved.TrafficPeriodStart == 0 {
		t.Fatalf("new inbound usage = %d, period start = %d", saved.TrafficUsed, saved.TrafficPeriodStart)
	}
	var options map[string]interface{}
	_ = json.Unmarshal(saved.Options, &options)
	for _, key := range []string{"traffic_limit", "traffic_reset_day", "ip_limit", "traffic_used"} {
		if _, ok := options[key]; ok {
			t.Fatalf("%s leaked into the core options", key)
		}
	}

	if err = db.Model(&model.Inbound{}).Where("id = ?", saved.Id).Update("traffic_used", 400).Error; err != nil {
		t.Fatal(err)
	}
	edit, _ := json.Marshal(map[string]interface{}{
		"id": saved.Id, "type": "direct", "tag": "capped", "listen": "::", "listen_port": 2345,
		"traffic_limit": 2000, "traffic_reset_day": 5, "ip_limit": 3, "traffic_used": 0,
	})
	if err = service.Save(db, "edit", edit, "", ""); err != nil {
		t.Fatal(err)
	}
	if err = db.First(&saved, saved.Id).Error; err != nil {
		t.Fatal(err)
	}
	if saved.TrafficUsed != 400 || saved.TrafficLimit != 2000 || saved.IPLimit != 3 {
		t.Fatalf("edit lost usage or limits: %+v", saved)
	}

	limits, err := service.BandwidthLimits(db)
	if err != nil {
		t.Fatal(err)
	}
	if got := limits["capped"]; !got.QuotaEnabled || got.QuotaRemaining != 1600 || got.MaxIPs != 3 {
		t.Fatalf("core limit = %+v", got)
	}

	if err = service.ResetTrafficUsage(saved.Id); err != nil {
		t.Fatal(err)
	}
	if err = db.First(&saved, saved.Id).Error; err != nil {
		t.Fatal(err)
	}
	if saved.TrafficUsed != 0 {
		t.Fatalf("reset left usage at %d", saved.TrafficUsed)
	}
}

func TestRollTrafficPeriods(t *testing.T) {
	setupLimitDB(t)
	db := database.GetDB()
	loc := time.UTC
	oldStart := time.Date(2026, 8, 1, 0, 0, 0, 0, loc).Unix()
	rows := []model.Inbound{
		{Tag: "due", Type: "direct", TrafficLimit: 10, TrafficResetDay: 1, TrafficUsed: 10, TrafficPeriodStart: oldStart},
		{Tag: "later", Type: "direct", TrafficLimit: 10, TrafficResetDay: 31, TrafficUsed: 7, TrafficPeriodStart: time.Date(2026, 8, 31, 0, 0, 0, 0, loc).Unix()},
	}
	for i := range rows {
		rows[i].Options = json.RawMessage(`{}`)
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := (&InboundService{}).RollTrafficPeriods(db, time.Date(2026, 9, 29, 12, 0, 0, 0, loc)); err != nil {
		t.Fatal(err)
	}
	var due, later model.Inbound
	db.Where("tag = ?", "due").First(&due)
	db.Where("tag = ?", "later").First(&later)
	if due.TrafficUsed != 0 || due.TrafficPeriodStart != time.Date(2026, 9, 1, 0, 0, 0, 0, loc).Unix() {
		t.Fatalf("due inbound not rolled over: %+v", due)
	}
	if later.TrafficUsed != 7 {
		t.Fatalf("inbound reset before its reset day: %+v", later)
	}
}

func TestXrayInboundRejectsTrafficAndIPLimit(t *testing.T) {
	setupLimitDB(t)
	for _, field := range []string{"traffic_limit", "ip_limit"} {
		err := (&InboundService{}).Save(database.GetDB(), "new", json.RawMessage(
			`{"type":"dokodemo-door","tag":"xray-`+field+`","core_type":"xray","port":1234,"`+field+`":1}`,
		), "", "")
		if err == nil {
			t.Fatalf("Xray %s was accepted", field)
		}
	}
}
