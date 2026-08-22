package web

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/op/go-logging"
)

func TestLocalControlSocketCapabilities(t *testing.T) {
	logger.InitLogger(logging.CRITICAL)
	dir := t.TempDir()
	path := filepath.Join(os.TempDir(), fmt.Sprintf("sui-control-%d-%d.sock", os.Getpid(), time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(path) })
	t.Setenv("SUI_CONTROL_SOCKET", path)
	if err := database.InitDB(filepath.Join(dir, "control.db")); err != nil {
		t.Fatal(err)
	}
	server := NewServer()
	if err := server.startControlSocket(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.stopControlSocket() })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	response := agent.CallLocalRPC(ctx, path, agent.RPCRequest{ID: "test", Method: agent.RPCMethodCapabilities})
	if !response.OK {
		t.Fatalf("local RPC failed: %#v", response)
	}
	var status agent.PanelStatus
	if err := json.Unmarshal(response.Payload, &status); err != nil {
		t.Fatal(err)
	}
	if !status.ControlAvailable || status.ProtocolVersion != agent.ProtocolVersion {
		t.Fatalf("unexpected capabilities: %#v", status)
	}
	if status.Cores == nil {
		t.Fatalf("capabilities omitted core runtime state: %#v", status)
	}
	hasPortTraffic := false
	for _, capability := range status.Capabilities {
		if capability == agent.CapabilityPortTrafficV1 {
			hasPortTraffic = true
		}
	}
	if !hasPortTraffic {
		t.Fatalf("capabilities omitted port traffic: %#v", status.Capabilities)
	}
	response = agent.CallLocalRPC(ctx, path, agent.RPCRequest{ID: "traffic", Method: agent.RPCMethodPortTraffic})
	if !response.OK {
		t.Fatalf("port traffic RPC failed: %#v", response)
	}
	var traffic struct {
		SampledAt int64         `json:"sampled_at"`
		Items     []interface{} `json:"items"`
	}
	if err := json.Unmarshal(response.Payload, &traffic); err != nil || traffic.SampledAt == 0 || traffic.Items == nil {
		t.Fatalf("unexpected port traffic RPC response: %s err=%v", response.Payload, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("control socket mode = %o, want 600", info.Mode().Perm())
	}
}
