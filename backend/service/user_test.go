package service

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util"
)

func TestInitializeAdminIsOneTimeAndHashed(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "first-run.db")); err != nil {
		t.Fatal(err)
	}
	users := UserService{}
	required, err := users.NeedsSetup()
	if err != nil {
		t.Fatal(err)
	}
	if !required {
		t.Fatal("fresh database should require Web setup")
	}

	if _, err = users.InitializeAdmin("ab", "12345678"); err == nil {
		t.Fatal("short username was accepted")
	}
	if _, err = users.InitializeAdmin("panel-admin", "short"); err == nil {
		t.Fatal("short password was accepted")
	}
	username, err := users.InitializeAdmin("  panel-admin  ", "secure-password")
	if err != nil {
		t.Fatal(err)
	}
	if username != "panel-admin" {
		t.Fatalf("normalized username = %q", username)
	}

	var stored model.User
	if err = database.GetDB().First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if !util.IsHashedPassword(stored.Password) || stored.Password == "secure-password" {
		t.Fatalf("administrator password was not hashed: %q", stored.Password)
	}
	if _, err = users.Login("panel-admin", "secure-password", "127.0.0.1"); err != nil {
		t.Fatalf("initialized administrator cannot log in: %v", err)
	}

	required, err = users.NeedsSetup()
	if err != nil || required {
		t.Fatalf("setup state after initialization = %v, %v", required, err)
	}
	if _, err = users.InitializeAdmin("second-admin", "another-password"); !errors.Is(err, ErrSetupAlreadyComplete) {
		t.Fatalf("second initialization error = %v, want ErrSetupAlreadyComplete", err)
	}
	var count int64
	if err = database.GetDB().Model(&model.User{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("administrator count = %d, want 1", count)
	}
}
