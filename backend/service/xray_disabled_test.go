//go:build !openwrt_lite

package service

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
)

func TestLowResourceProfileRejectsXrayInbound(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dbDir)
	t.Setenv("SUI_DISABLE_XRAY", "true")
	if err := database.InitDB(filepath.Join(dbDir, "disabled-xray.db")); err != nil {
		t.Fatal(err)
	}

	db := database.GetDB()
	if err := db.Create(&model.Inbound{Type: "vless", Tag: "disabled-xray", CoreType: model.CoreTypeXray}).Error; err != nil {
		t.Fatal(err)
	}
	err := (&ConfigService{}).validateXrayConfig(db)
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("Xray inbound validation did not report the low-resource guard: %v", err)
	}
}
