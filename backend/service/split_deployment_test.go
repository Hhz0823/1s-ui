package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/Hhz0823/1s-ui/database"
)

func TestFrontendEntrySettingsRequireExplicitApply(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "split.db")); err != nil {
		t.Fatal(err)
	}
	if _, err := (&SettingService{}).GetAllSetting(); err != nil {
		t.Fatal(err)
	}
	service := ConfigService{}
	data := json.RawMessage(`{"webPort":"3095","webPath":"/panel/"}`)
	if _, err := service.Save("settings", "edit", data, "", "tester", "localhost"); err != nil {
		t.Fatal(err)
	}
	if !FrontendEntryApplyRequired() {
		t.Fatal("frontend entry change was not marked for apply")
	}
	status := (&SettingService{}).GetDeploymentStatus()
	if status["deploymentMode"] != "split" || status["frontendApplyRequired"] != "true" {
		t.Fatalf("deployment status = %#v", status)
	}
	if err := ClearFrontendEntryApplyRequired(); err != nil {
		t.Fatal(err)
	}
	if FrontendEntryApplyRequired() {
		t.Fatal("frontend apply marker was not cleared")
	}
}
