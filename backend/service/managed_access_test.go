package service

import (
	"crypto/sha256"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/database"
)

func TestManagedPanelAccessIsShortLivedAndSingleUse(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "managed-access.db")); err != nil {
		t.Fatal(err)
	}
	if _, err := (&UserService{}).InitializeAdmin("panel-admin", "secure-password"); err != nil {
		t.Fatal(err)
	}
	managedAccessStore.Lock()
	managedAccessStore.grants = make(map[[sha256.Size]byte]managedAccessEntry)
	managedAccessStore.Unlock()

	service := &ManagedAccessService{}
	grant, err := service.Issue(ManagedPanelAccessRequest{Actor: "controller-admin"})
	if err != nil {
		t.Fatal(err)
	}
	if grant.Token == "" || grant.ExpiresAt <= time.Now().Unix() {
		t.Fatalf("invalid managed access grant: %#v", grant)
	}
	username, err := service.Consume(grant.Token)
	if err != nil || username != "panel-admin" {
		t.Fatalf("could not consume managed access grant: user=%q err=%v", username, err)
	}
	if _, err := service.Consume(grant.Token); err == nil {
		t.Fatal("managed access grant was accepted twice")
	}
}
