//go:build !openwrt_lite

package service

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
)

func TestXrayInboundRejectsBandwidthLimit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "xray-limit.db")); err != nil {
		t.Fatal(err)
	}
	oldCore := corePtr
	corePtr = core.NewCore()
	t.Cleanup(func() { corePtr = oldCore })
	err := (&InboundService{}).Save(database.GetDB(), "new", json.RawMessage(
		`{"type":"dokodemo-door","tag":"xray","core_type":"xray","port":1234,"upload_limit":1}`,
	), "", "")
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("Xray limit was not explicitly rejected: %v", err)
	}
}
