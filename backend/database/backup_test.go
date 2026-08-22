package database

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetDbIncludesServicesAndTokens(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "source.db")); err != nil {
		t.Fatal(err)
	}

	hashedPassword, err := util.HashPassword("backup-password")
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{Username: "backup-admin", Password: hashedPassword}
	if err = GetDB().Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	service := model.Service{Type: "derp", Tag: "backup-service", Options: json.RawMessage(`{}`)}
	token := model.Tokens{Desc: "backup-token", Token: "secret-token", UserId: user.Id}
	relayPool := model.RelayPool{Name: "backup-relay", Mode: "upstream", ListenHost: "127.0.0.1", PortStart: 30000, Count: 1, Items: json.RawMessage(`[]`)}
	if err := GetDB().Create(&service).Error; err != nil {
		t.Fatal(err)
	}
	if err := GetDB().Create(&token).Error; err != nil {
		t.Fatal(err)
	}
	if err := GetDB().Create(&relayPool).Error; err != nil {
		t.Fatal(err)
	}

	data, err := GetDb("")
	if err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(t.TempDir(), "backup.db")
	if err = os.WriteFile(backupPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	backupDB, err := gorm.Open(sqlite.Open(backupPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	var serviceCount int64
	if err = backupDB.Model(&model.Service{}).Where("tag = ?", service.Tag).Count(&serviceCount).Error; err != nil {
		t.Fatal(err)
	}
	if serviceCount != 1 {
		t.Fatalf("service count = %d, want 1", serviceCount)
	}
	var tokenCount int64
	if err = backupDB.Model(&model.Tokens{}).Where("token = ?", token.Token).Count(&tokenCount).Error; err != nil {
		t.Fatal(err)
	}
	if tokenCount != 1 {
		t.Fatalf("token count = %d, want 1", tokenCount)
	}
	var relayCount int64
	if err = backupDB.Model(&model.RelayPool{}).Where("name = ?", relayPool.Name).Count(&relayCount).Error; err != nil {
		t.Fatal(err)
	}
	if relayCount != 1 {
		t.Fatalf("relay pool count = %d, want 1", relayCount)
	}
}
