package service

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/op/go-logging"
)

func TestLocalControlInboundLifecycleAndRevisionConflict(t *testing.T) {
	logger.InitLogger(logging.CRITICAL)
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	t.Setenv("SUI_SKIP_CORE", "true")
	if err := database.InitDB(filepath.Join(dir, "local-control.db")); err != nil {
		t.Fatal(err)
	}
	oldCore, oldXray := corePtr, xrayPtr
	corePtr = core.NewCore()
	xrayPtr = core.NewXrayRuntime()
	t.Cleanup(func() { corePtr, xrayPtr = oldCore, oldXray })

	control := &LocalControlService{}
	before, err := control.ListInbounds()
	if err != nil {
		t.Fatal(err)
	}
	payload := json.RawMessage(`{"type":"direct","tag":"remote-direct","listen":"::","listen_port":31000}`)
	created, err := control.SaveInbound(RemoteInboundSaveRequest{
		Action: "new", Data: payload, ExpectedRevision: before.Revision,
		Actor: "admin", PublicHost: "node.example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Revision <= before.Revision {
		t.Fatalf("revision did not advance: before=%d after=%d", before.Revision, created.Revision)
	}
	after, err := control.ListInbounds()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Inbounds) != 1 || after.Inbounds[0]["tag"] != "remote-direct" {
		t.Fatalf("unexpected remote inbound list: %#v", after.Inbounds)
	}

	_, err = control.SaveInbound(RemoteInboundSaveRequest{
		Action: "new", Data: payload, ExpectedRevision: before.Revision,
		Actor: "admin", PublicHost: "node.example.com",
	})
	var conflict *ConfigRevisionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("stale save did not return revision conflict: %v", err)
	}
}

func TestLocalControlRequiresPublicHostForInboundWrites(t *testing.T) {
	control := &LocalControlService{}
	_, err := control.SaveInbound(RemoteInboundSaveRequest{
		Action: "new", Data: json.RawMessage(`{"type":"direct","tag":"x","listen_port":31001}`),
	})
	if err == nil {
		t.Fatal("remote inbound save accepted an empty public host")
	}
}
