package service

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"

	"github.com/op/go-logging"
)

// hostAddress is this machine's first non-loopback IPv4 address. Proxy checks
// refuse loopback, so the test proxies are reached the way a relay reaches a
// proxy in its own network.
func hostAddress(t *testing.T) string {
	t.Helper()
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	for _, address := range addresses {
		if prefix, ok := address.(*net.IPNet); ok && !prefix.IP.IsLoopback() && prefix.IP.To4() != nil {
			return prefix.IP.String()
		}
	}
	t.Skip("no non-loopback IPv4 address")
	return ""
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// startTestProxies runs the panel's sing-box with a SOCKS5 and an HTTP proxy
// that require u / p@ss, plus a plain-HTTP trace page to fetch through them.
func startTestProxies(t *testing.T, host string) (socksPort, httpPort int, target string) {
	t.Helper()
	socksPort, httpPort = freeTCPPort(t), freeTCPPort(t)
	users := []map[string]string{{"username": "u", "password": "p@ss"}}
	config, _ := json.Marshal(map[string]interface{}{
		"log": map[string]interface{}{"level": "error"},
		"inbounds": []map[string]interface{}{
			{"type": "socks", "tag": "socks", "listen": "0.0.0.0", "listen_port": socksPort, "users": users},
			{"type": "http", "tag": "http", "listen": "0.0.0.0", "listen_port": httpPort, "users": users},
		},
		"outbounds": []map[string]interface{}{{"type": "direct", "tag": "direct"}},
	})
	box := core.NewCore()
	if err := box.Start(config); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = box.Stop() })
	listener, err := net.Listen("tcp", host+":0")
	if err != nil {
		t.Fatal(err)
	}
	page := &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ip=198.51.100.77\nloc=SG\n")
	})}}
	page.Start()
	t.Cleanup(page.Close)
	return socksPort, httpPort, page.URL + "/cdn-cgi/trace"
}

func waitForResults(t *testing.T, want int64) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		var count int64
		database.GetDB().Model(&model.ProxyMonitorResult{}).Count(&count)
		if count >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d of %d checks were stored", count, want)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestProxyMonitorsCheckRealProxiesOnSchedule(t *testing.T) {
	logger.InitLogger(logging.CRITICAL)
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "proxy-monitors.db")); err != nil {
		t.Fatal(err)
	}
	proxyMonitorState.Lock()
	proxyMonitorState.lastRun, proxyMonitorState.busy = nil, map[uint]bool{}
	proxyMonitorState.Unlock()
	host := hostAddress(t)
	socksPort, httpPort, target := startTestProxies(t, host)
	offline := model.AgentNode{Name: "relay-1", TokenHash: strings.Repeat("e", 64), Report: []byte(`{}`)}
	if err := database.GetDB().Create(&offline).Error; err != nil {
		t.Fatal(err)
	}
	service := ProxyMonitorService{}
	password := "p@ss"
	wrong := "wrong"

	socks, err := service.Save(ProxyMonitorInput{Link: fmt.Sprintf("socks5://u:p%%40ss@%s:%d#Tokyo%%20A", host, socksPort), Target: target})
	if err != nil || socks.Name != "Tokyo A" || socks.Type != "socks5" || !socks.HasPassword || !socks.Enabled || socks.Interval != 60 {
		t.Fatalf("socks monitor: %+v %v", socks, err)
	}
	httpMonitor, err := service.Save(ProxyMonitorInput{Type: "http", Host: host, Port: httpPort, Username: "u", Password: &password, Target: target, Interval: 30})
	if err != nil || httpMonitor.Name != fmt.Sprintf("%s:%d", host, httpPort) {
		t.Fatalf("http monitor: %+v %v", httpMonitor, err)
	}
	bad, err := service.Save(ProxyMonitorInput{Name: "bad login", Type: "socks5", Host: host, Port: socksPort, Username: "u", Password: &wrong, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	relayed, err := service.Save(ProxyMonitorInput{Name: "via relay", Type: "socks5", Host: host, Port: socksPort, ServerId: offline.Id, Target: target})
	if err != nil || relayed.ServerName != "relay-1" {
		t.Fatalf("relayed monitor: %+v %v", relayed, err)
	}
	for _, input := range []ProxyMonitorInput{
		{Type: "socks5", Host: host, Port: socksPort, ServerId: 999},
		{Type: "socks5", Host: host, Port: socksPort, Interval: 5},
		{Type: "vmess", Host: host, Port: socksPort},
		{Link: "https://u:p@" + host + ":1"},
	} {
		if _, err := service.Save(input); err == nil {
			t.Fatalf("saved %+v", input)
		}
	}

	now := time.Now()
	service.RunDue(now)
	waitForResults(t, 4)
	views, err := service.List()
	if err != nil || len(views) != 4 {
		t.Fatalf("list: %d %v", len(views), err)
	}
	status := map[string]ProxyMonitorView{}
	for _, view := range views {
		status[view.Name] = view
	}
	for _, name := range []string{"Tokyo A", httpMonitor.Name} {
		view := status[name]
		if view.Status != "up" || view.Last == nil || view.Last.ExitIP != "198.51.100.77" || view.Last.Country != "SG" || view.Uptime != 100 || len(view.Recent) != 1 {
			t.Fatalf("%s: %+v %+v", name, view, view.Last)
		}
	}
	if view := status["bad login"]; view.Status != "down" || view.Last.Stage != "auth" || view.Uptime != 0 {
		t.Fatalf("bad login: %+v %+v", view, view.Last)
	}
	// An offline relay says nothing about the proxy: unknown, not down, and
	// it does not count against uptime.
	if view := status["via relay"]; view.Status != "unknown" || view.Last.Stage != ProxyStageServer || view.Checks != 0 {
		t.Fatalf("via relay: %+v %+v", view, view.Last)
	}
	raw, _ := json.Marshal(views)
	if strings.Contains(string(raw), "p@ss") || strings.Contains(string(raw), "wrong") || strings.Contains(string(raw), `"password"`) {
		t.Fatalf("monitor list exposes a password: %s", raw)
	}

	// Nothing is due again until the interval passes.
	service.RunDue(now.Add(10 * time.Second))
	time.Sleep(300 * time.Millisecond)
	var count int64
	database.GetDB().Model(&model.ProxyMonitorResult{}).Count(&count)
	if count != 4 {
		t.Fatalf("%d checks after an early tick", count)
	}
	service.RunDue(now.Add(31 * time.Second))
	waitForResults(t, 5)
	service.RunDue(now.Add(61 * time.Second))
	waitForResults(t, 9)

	detail, err := service.Detail(socks.Id, 3600)
	if err != nil || len(detail.Points) == 0 || detail.Points[0].Checks == 0 || len(detail.ExitIPs) != 1 || detail.ExitIPs[0].Checks != 2 {
		t.Fatalf("detail: %+v %v", detail, err)
	}
	badDetail, err := service.Detail(bad.Id, 3600)
	if err != nil || len(badDetail.Failures) != 2 || badDetail.Points[0].Failures == 0 {
		t.Fatalf("bad detail: %+v %v", badDetail, err)
	}

	// Testing an edited monitor reuses the stored password.
	result, err := service.Test(ProxyMonitorInput{Id: socks.Id, Type: "socks5", Host: host, Port: socksPort, Username: "u", Target: target})
	if err != nil || !result.OK {
		t.Fatalf("test with stored password: %+v %v", result, err)
	}
	result, err = service.CheckNow(bad.Id)
	if err != nil || result.OK || result.Stage != "auth" {
		t.Fatalf("check now: %+v %v", result, err)
	}

	// Pointing a monitor at another proxy drops the old history.
	edited, err := service.Save(ProxyMonitorInput{Id: bad.Id, Name: "bad login", Type: "socks5", Host: host, Port: socksPort, Username: "u", Password: &password, Target: target})
	if err != nil || edited.Status != "pending" {
		t.Fatalf("edit: %+v %v", edited, err)
	}
	database.GetDB().Model(&model.ProxyMonitorResult{}).Where("monitor_id = ?", bad.Id).Count(&count)
	if count != 0 {
		t.Fatalf("%d old checks kept after the proxy changed", count)
	}
	paused := false
	if view, err := service.Save(ProxyMonitorInput{Id: httpMonitor.Id, Name: "renamed", Type: "http", Host: host, Port: httpPort, Username: "u", Target: target, Interval: 30, Enabled: &paused}); err != nil || view.Status != "paused" || !view.HasPassword {
		t.Fatalf("pause: %+v %v", view, err)
	}

	old := model.ProxyMonitorResult{MonitorId: socks.Id, Time: time.Now().Add(-4 * 24 * time.Hour).Unix(), OK: true}
	database.GetDB().Create(&old)
	if err := service.CleanupProxyResults(); err != nil {
		t.Fatal(err)
	}
	database.GetDB().Model(&model.ProxyMonitorResult{}).Where("id = ?", old.Id).Count(&count)
	if count != 0 {
		t.Fatal("cleanup kept a check older than the retention window")
	}
	if err := service.Delete(socks.Id); err != nil {
		t.Fatal(err)
	}
	database.GetDB().Model(&model.ProxyMonitorResult{}).Where("monitor_id = ?", socks.Id).Count(&count)
	if count != 0 {
		t.Fatal("deleting a monitor kept its checks")
	}
}
