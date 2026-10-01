package api

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/service"
	"github.com/Hhz0823/1s-ui/speedtest"
	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
)

func TestMonitorRoutesUseReadOnlyMonitorKey(t *testing.T) {
	logger.InitLogger(logging.CRITICAL)
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "monitor.db")); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Create(&model.AgentNode{Name: "hk-1", TokenHash: "x"}).Error; err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewAPIv2Handler(engine.Group("/apiv2"))

	get := func(path, key string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if key != "" {
			request.Header.Set("X-Monitor-Key", key)
		}
		engine.ServeHTTP(recorder, request)
		return recorder
	}

	if code := get("/apiv2/monitor/servers", "").Code; code != http.StatusUnauthorized {
		t.Fatalf("no key: status = %d, want 401", code)
	}
	settings := &service.SettingService{}
	key, _, err := settings.RotateMonitorKey()
	if err != nil {
		t.Fatal(err)
	}
	if code := get("/apiv2/monitor/servers", key+"x").Code; code != http.StatusUnauthorized {
		t.Fatalf("wrong key: status = %d, want 401", code)
	}
	// The monitor key must not unlock the admin API.
	var admin Msg
	if err := json.Unmarshal(get("/apiv2/settings", key).Body.Bytes(), &admin); err != nil || admin.Success {
		t.Fatalf("monitor key reached admin API: %+v %v", admin, err)
	}

	response := get("/apiv2/monitor/servers", key)
	var overview struct {
		Success bool
		Obj     struct {
			Servers []struct {
				Id    uint   `json:"id"`
				Name  string `json:"name"`
				Local bool   `json:"local"`
			} `json:"servers"`
		}
	}
	if err := json.Unmarshal(response.Body.Bytes(), &overview); err != nil || !overview.Success {
		t.Fatalf("overview: %s %v", response.Body.String(), err)
	}
	if len(overview.Obj.Servers) != 2 || !overview.Obj.Servers[0].Local || overview.Obj.Servers[1].Name != "hk-1" {
		t.Fatalf("unexpected servers: %+v", overview.Obj.Servers)
	}
	for _, path := range []string{"/apiv2/monitor/servers/0", "/apiv2/monitor/servers/1"} {
		var detail Msg
		if err := json.Unmarshal(get(path, key).Body.Bytes(), &detail); err != nil || !detail.Success {
			t.Fatalf("%s: %+v %v", path, detail, err)
		}
	}

	if err := settings.DisableMonitorKey(); err != nil {
		t.Fatal(err)
	}
	if code := get("/apiv2/monitor/servers", key).Code; code != http.StatusUnauthorized {
		t.Fatalf("disabled key: status = %d, want 401", code)
	}
	all, err := settings.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if _, exposed := (*all)["monitorKeyHash"]; exposed {
		t.Fatal("monitor key hash is exposed in settings")
	}
}

func TestMonitorAppNodesProxiesAndSpeedtest(t *testing.T) {
	logger.InitLogger(logging.CRITICAL)
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "monitor-app.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	// A REALITY + XHTTP node with VLESS Encryption and one enabled user, and
	// a plain SOCKS node with a disabled user.
	tls := model.Tls{Name: "reality", Server: json.RawMessage(`{"enabled":true,"reality":{"enabled":true,"private_key":"SECRET-KEY"}}`), Client: json.RawMessage(`{}`)}
	db.Create(&tls)
	vless := model.Inbound{Type: "vless", Tag: "vless-xhttp", TlsId: tls.Id, CoreType: model.CoreTypeXray,
		Options: json.RawMessage(`{"listen_port":443,"transport":{"type":"xhttp","path":"/x"},"decryption":"mlkem768x25519plus.native.600s.SECRET-DECRYPTION"}`)}
	socks := model.Inbound{Type: "socks", Tag: "socks-1080", Options: json.RawMessage(`{"listen_port":1080}`)}
	db.Create(&vless)
	db.Create(&socks)
	db.Create(&model.Client{Name: "alice", Enable: true, Config: json.RawMessage(`{"vless":{"uuid":"SECRET-UUID"}}`), Inbounds: json.RawMessage(fmt.Sprintf("[%d]", vless.Id))})
	db.Create(&model.Client{Name: "bob", Enable: false, Inbounds: json.RawMessage(fmt.Sprintf("[%d]", socks.Id))})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewAPIv2Handler(engine.Group("/apiv2"))
	settings := &service.SettingService{}
	key, _, err := settings.RotateMonitorKey()
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string) (int, string) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("X-Monitor-Key", key)
		request.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(recorder, request)
		return recorder.Code, recorder.Body.String()
	}

	_, body := call(http.MethodGet, "/apiv2/monitor/servers", "")
	if !strings.Contains(body, `"features":{"nodes":true,"proxy_monitors":true,"manage_proxies":true,"speedtest":true,"speedtest_port":5201}`) {
		t.Fatalf("features: %s", body)
	}

	_, body = call(http.MethodGet, "/apiv2/monitor/servers/0/nodes", "")
	var nodes struct {
		Success bool
		Obj     service.PortTrafficResponse
	}
	if err := json.Unmarshal([]byte(body), &nodes); err != nil || !nodes.Success || len(nodes.Obj.Items) != 2 {
		t.Fatalf("nodes: %s %v", body, err)
	}
	first, second := nodes.Obj.Items[0], nodes.Obj.Items[1]
	if first.Security != "reality" || first.Transport != "xhttp" || !first.Encryption || first.Users != 1 || first.Port != 443 {
		t.Fatalf("vless node: %+v", first)
	}
	if second.Security != "" || second.Users != 0 || second.Type != "socks" {
		t.Fatalf("socks node: %+v", second)
	}
	for _, secret := range []string{"SECRET", "alice", "uuid", "private_key"} {
		if strings.Contains(body, secret) {
			t.Fatalf("node summary leaks %q: %s", secret, body)
		}
	}

	code, body := call(http.MethodPost, "/apiv2/monitor/proxies", `{"link":"socks5://u:pw@203.0.113.10:1080#hk","interval":120}`)
	if code != http.StatusOK || !strings.Contains(body, `"success":true`) || !strings.Contains(body, `"name":"hk"`) || strings.Contains(body, `"pw"`) {
		t.Fatalf("create proxy: %d %s", code, body)
	}
	_, body = call(http.MethodGet, "/apiv2/monitor/proxies", "")
	if !strings.Contains(body, `"has_password":true`) || !strings.Contains(body, `"status":"pending"`) || strings.Contains(body, "pw") {
		t.Fatalf("list proxies: %s", body)
	}
	_, body = call(http.MethodPost, "/apiv2/monitor/proxies/test", `{"type":"socks5","host":"127.0.0.1","port":1080}`)
	if !strings.Contains(body, `"stage":"connect"`) || !strings.Contains(body, "not allowed") {
		t.Fatalf("loopback test: %s", body)
	}

	// The admin can take management and speed tests away from the key.
	if err := settings.SetMonitorAppSettings(service.MonitorAppSettings{Proxies: false, Speedtest: false, SpeedtestPort: 5201}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/apiv2/monitor/proxies", "/apiv2/monitor/proxies/test", "/apiv2/monitor/proxies/1/delete", "/apiv2/monitor/proxies/1/check", "/apiv2/monitor/servers/0/speedtest"} {
		if code, body := call(http.MethodPost, path, `{}`); code != http.StatusForbidden {
			t.Fatalf("%s while turned off: %d %s", path, code, body)
		}
	}
	if code, _ := call(http.MethodGet, "/apiv2/monitor/proxies", ""); code != http.StatusOK {
		t.Fatalf("reading proxies needs no permission: %d", code)
	}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	if err := settings.SetMonitorAppSettings(service.MonitorAppSettings{Proxies: true, Speedtest: true, SpeedtestPort: port}); err != nil {
		t.Fatal(err)
	}
	defer speedtest.Stop()
	_, body = call(http.MethodPost, "/apiv2/monitor/servers/0/speedtest", "")
	var started struct {
		Success bool
		Obj     service.SpeedtestTarget
	}
	if err := json.Unmarshal([]byte(body), &started); err != nil || !started.Success || started.Obj.Port != port || len(started.Obj.Token) != 32 {
		t.Fatalf("speedtest: %s %v", body, err)
	}
	token, _ := hex.DecodeString(started.Obj.Token)
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	header := append([]byte(speedtest.TCPMagic), speedtest.Version, speedtest.ModeDownload, 0x01, 0x2c)
	conn.Write(append(header, token...))
	received, _ := io.Copy(io.Discard, conn)
	conn.Close()
	if received < 1<<16 {
		t.Fatalf("speed test downloaded %d bytes", received)
	}
}
