package service

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/op/go-logging"
)

func TestGetChangesUsesExactFilters(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "changes.db")); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	changes := []model.Changes{
		{DateTime: now, Actor: "alice", Key: "inbounds", Action: "new", Obj: json.RawMessage(`{}`)},
		{DateTime: now, Actor: "bob", Key: "tls", Action: "edit", Obj: json.RawMessage(`{}`)},
	}
	if err := database.GetDB().Create(&changes).Error; err != nil {
		t.Fatal(err)
	}

	service := ConfigService{}
	if got := service.GetChanges("alice", "", "10"); len(got) != 1 || got[0].Actor != "alice" {
		t.Fatalf("exact actor filter returned %#v", got)
	}
	if got := service.GetChanges("alice' OR 1=1 --", "", "10"); len(got) != 0 {
		t.Fatalf("injection-like actor returned %d rows", len(got))
	}
}

func TestCheckChangesRejectsInvalidTimestamp(t *testing.T) {
	LastUpdate.Store(0)
	service := ConfigService{}
	if _, err := service.CheckChanges("0 OR 1=1"); err == nil {
		t.Fatal("CheckChanges() accepted a non-numeric timestamp")
	}
}

func TestCheckChangesAcceptsMillisecondTimestamp(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "millisecond-changes.db")); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := database.GetDB().Create(&model.Changes{
		DateTime: now.Unix(), Actor: "test", Key: "inbounds", Action: "new", Obj: json.RawMessage(`{}`),
	}).Error; err != nil {
		t.Fatal(err)
	}
	LastUpdate.Store(0)
	service := ConfigService{}
	updated, err := service.CheckChanges(strconv.FormatInt(now.Add(-time.Second).UnixMilli(), 10))
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("CheckChanges() missed a persisted change with a millisecond cursor")
	}
}

func TestSaveKeepsCoreStoppedInSafeMode(t *testing.T) {
	logger.InitLogger(logging.CRITICAL)
	dbDir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dbDir)
	t.Setenv("SUI_SKIP_CORE", "true")
	if err := database.InitDB(filepath.Join(dbDir, "safe-save.db")); err != nil {
		t.Fatal(err)
	}
	if _, err := (&SettingService{}).GetAllSetting(); err != nil {
		t.Fatal(err)
	}

	oldCore, oldXray := corePtr, xrayPtr
	corePtr = core.NewCore()
	xrayPtr = core.NewXrayRuntime()
	t.Cleanup(func() {
		corePtr = oldCore
		xrayPtr = oldXray
	})

	service := ConfigService{}
	if _, err := service.Save("config", "edit", json.RawMessage(defaultConfig), "", "tester", "localhost"); err != nil {
		t.Fatal(err)
	}
	if corePtr.IsRunning() {
		t.Fatal("safe-mode config save started sing-box")
	}
}

func TestLowResourceProfileDisablesXrayRuntime(t *testing.T) {
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	t.Setenv("SUI_DISABLE_XRAY", "true")

	oldXray := xrayPtr
	xrayPtr = core.NewXrayRuntime()
	t.Cleanup(func() { xrayPtr = oldXray })

	service := ConfigService{}
	if err := service.ensureXrayCore(false, false); err != nil {
		t.Fatalf("automatic Xray check should be a no-op when disabled: %v", err)
	}
	if err := service.ensureXrayCore(false, true); err == nil {
		t.Fatal("manual Xray start was accepted while disabled")
	}
	if xrayPtr.IsRunning() {
		t.Fatal("disabled Xray runtime is running")
	}
}

func TestOnDemandXraySkipsAutomaticStart(t *testing.T) {
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	t.Setenv("SUI_XRAY_ON_DEMAND", "true")

	oldXray := xrayPtr
	xrayPtr = core.NewXrayRuntime()
	t.Cleanup(func() { xrayPtr = oldXray })

	service := ConfigService{}
	if err := service.ensureXrayCore(false, false); err != nil {
		t.Fatalf("automatic Xray check should be deferred in on-demand mode: %v", err)
	}
	if xrayPtr.IsRunning() {
		t.Fatal("on-demand Xray runtime was started automatically")
	}
}

func TestInboundChangeAffectsOnlySelectedCore(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "core-impact.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	singInbound := model.Inbound{Type: "mixed", Tag: "sing", CoreType: model.CoreTypeSingBox}
	xrayInbound := model.Inbound{Type: "vless", Tag: "xray", CoreType: model.CoreTypeXray}
	if err := db.Create(&singInbound).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xrayInbound).Error; err != nil {
		t.Fatal(err)
	}

	singEdit := json.RawMessage(`{"id":` + strconv.FormatUint(uint64(singInbound.Id), 10) + `,"type":"mixed","tag":"sing","core_type":"sing-box"}`)
	affectsXray, err := inboundChangeAffectsXray(db, "edit", singEdit)
	if err != nil {
		t.Fatal(err)
	}
	if affectsXray {
		t.Fatal("sing-box-only edit should not require Xray validation")
	}

	xrayDelete := json.RawMessage(`"xray"`)
	affectsXray, err = inboundChangeAffectsXray(db, "del", xrayDelete)
	if err != nil {
		t.Fatal(err)
	}
	if !affectsXray {
		t.Fatal("deleting an Xray inbound must require Xray restart")
	}
}

func TestInboundIDsContainXray(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "client-impact.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	inbounds := []model.Inbound{
		{Type: "mixed", Tag: "sing", CoreType: model.CoreTypeSingBox},
		{Type: "vless", Tag: "xray", CoreType: model.CoreTypeXray},
	}
	if err := db.Create(&inbounds).Error; err != nil {
		t.Fatal(err)
	}

	affectsXray, err := inboundIDsContainXray(db, []uint{inbounds[0].Id})
	if err != nil {
		t.Fatal(err)
	}
	if affectsXray {
		t.Fatal("sing-box client change should not require Xray validation")
	}
	affectsXray, err = inboundIDsContainXray(db, []uint{inbounds[0].Id, inbounds[1].Id})
	if err != nil {
		t.Fatal(err)
	}
	if !affectsXray {
		t.Fatal("Xray client change must require Xray validation")
	}
}
