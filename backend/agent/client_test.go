package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/shirou/gopsutil/v4/cpu"
)

func TestRunReconnectsWebSocketAfterDisconnect(t *testing.T) {
	previousWait := webSocketReconnectWait
	webSocketReconnectWait = 20 * time.Millisecond
	t.Cleanup(func() { webSocketReconnectWait = previousWait })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	connectedAgain := make(chan struct{}, 1)
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/agent/v1/ws":
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			if attempts.Add(1) == 1 {
				_ = conn.Close(websocket.StatusGoingAway, "panel restart")
				return
			}
			connectedAgain <- struct{}{}
			<-ctx.Done()
			_ = conn.Close(websocket.StatusNormalClosure, "done")
		case "/app/agent/v1/heartbeat":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"obj":{"interval_seconds":15}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, ClientConfig{
			PanelURL: server.URL + "/app/",
			Token:    "test-token",
			Interval: 5 * time.Second,
			PreferWS: true,
		})
	}()

	select {
	case <-connectedAgain:
		cancel()
	case <-time.After(3 * time.Second):
		t.Fatal("agent did not reconnect to WebSocket")
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned an error after cancellation: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("agent did not stop after cancellation")
	}
}

func TestPanelCoreStatusOverridesProcessScan(t *testing.T) {
	report := Report{
		Cores: CoreStatus{SingBoxRunning: false, XrayRunning: true, XrayVersion: "26.7.11"},
		Panel: PanelStatus{Cores: &CoreStatus{SingBoxRunning: true, XrayRunning: false}},
	}
	applyPanelCoreStatus(&report)
	if !report.Cores.SingBoxRunning || report.Cores.XrayRunning {
		t.Fatalf("panel core state was not applied: %#v", report.Cores)
	}
	if report.Cores.XrayVersion != "26.7.11" {
		t.Fatalf("Xray version was lost: %#v", report.Cores)
	}
}

func TestCPUPercentBetweenUsesWholeSampleWindow(t *testing.T) {
	previous := cpu.TimesStat{User: 10, System: 10, Idle: 80}
	current := cpu.TimesStat{User: 25, System: 15, Idle: 160}

	if got := cpuPercentBetween(previous, current); got != 20 {
		t.Fatalf("cpuPercentBetween() = %v, want 20", got)
	}
}

func TestCPUPercentBetweenIgnoresIdleAndIOWait(t *testing.T) {
	previous := cpu.TimesStat{User: 10, System: 10, Idle: 70, Iowait: 10}
	current := cpu.TimesStat{User: 10, System: 10, Idle: 150, Iowait: 30}

	if got := cpuPercentBetween(previous, current); got != 0 {
		t.Fatalf("cpuPercentBetween() = %v, want 0", got)
	}
}
