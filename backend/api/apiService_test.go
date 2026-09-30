package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
)

func TestSysctlListHas(t *testing.T) {
	tests := []struct {
		name  string
		list  string
		value string
		want  bool
	}{
		{name: "exact token", list: "reno cubic bbr", value: "bbr", want: true},
		{name: "missing token", list: "reno cubic bbr", value: "bbr3", want: false},
		{name: "substring is not token", list: "reno cubic bbr3", value: "bbr", want: false},
		{name: "extra whitespace", list: "  reno\tcubic\nbbr2plus  ", value: "bbr2plus", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sysctlListHas(tt.list, tt.value); got != tt.want {
				t.Fatalf("sysctlListHas(%q, %q) = %v, want %v", tt.list, tt.value, got, tt.want)
			}
		})
	}
}

func TestManagedPanelInstallCommandUsesUnifiedInstaller(t *testing.T) {
	connectURL := "https://panel.example.com/app/"
	command := managedPanelInstallCommand(connectURL)
	if command != "bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh)" {
		t.Fatalf("managed install command is not the single public installer: %q", command)
	}
}

func TestChinaCommandUsesMirror(t *testing.T) {
	command := "bash <(curl -fsSL https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install-agent.sh) --connect 'https://p/agent/v1/pair#x'"
	want := "bash <(curl -fsSL https://ghfast.top/https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install-agent.sh) --mirror cn --connect 'https://p/agent/v1/pair#x'"
	if got := chinaCommand(command); got != want {
		t.Fatalf("chinaCommand = %q", got)
	}
	managed := chinaCommand(managedPanelInstallCommand(""))
	if managed != "bash <(curl -Ls https://ghfast.top/https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh) --mirror cn" {
		t.Fatalf("managed China command = %q", managed)
	}
}

func TestFirstRunSetupAPI(t *testing.T) {
	logger.InitLogger(logging.ERROR)
	if err := database.InitDB(filepath.Join(t.TempDir(), "setup-api.db")); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("s-ui", cookie.NewStore([]byte("0123456789abcdef0123456789abcdef"))))
	policy, err := NewOriginPolicy("")
	if err != nil {
		t.Fatal(err)
	}
	NewAPIHandler(engine.Group("/app/api"), nil, policy)

	statusRecorder := httptest.NewRecorder()
	engine.ServeHTTP(statusRecorder, httptest.NewRequest(http.MethodGet, "/app/api/setup-status", nil))
	var status Msg
	if err := json.Unmarshal(statusRecorder.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	statusObj, ok := status.Obj.(map[string]interface{})
	if !status.Success || !ok || statusObj["required"] != true || statusObj["authenticated"] != false {
		t.Fatalf("fresh setup status = %#v", status)
	}

	crossSite := url.Values{"username": {"panel-admin"}, "password": {"secure-password"}, "confirmPassword": {"secure-password"}}
	crossRecorder := httptest.NewRecorder()
	crossRequest := httptest.NewRequest(http.MethodPost, "/app/api/setup", strings.NewReader(crossSite.Encode()))
	crossRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	crossRequest.Header.Set("Origin", "https://evil.example")
	crossRequest.Host = "panel.example"
	engine.ServeHTTP(crossRecorder, crossRequest)
	var crossResult Msg
	if err := json.Unmarshal(crossRecorder.Body.Bytes(), &crossResult); err != nil {
		t.Fatal(err)
	}
	if crossResult.Success {
		t.Fatal("cross-site setup request was accepted")
	}

	setupRecorder := httptest.NewRecorder()
	setupRequest := httptest.NewRequest(http.MethodPost, "/app/api/setup", strings.NewReader(crossSite.Encode()))
	setupRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	setupRequest.Header.Set("Origin", "http://panel.example")
	setupRequest.Header.Set("X-Requested-With", "XMLHttpRequest")
	setupRequest.Host = "panel.example"
	engine.ServeHTTP(setupRecorder, setupRequest)
	var setupResult Msg
	if err := json.Unmarshal(setupRecorder.Body.Bytes(), &setupResult); err != nil {
		t.Fatal(err)
	}
	if !setupResult.Success {
		t.Fatalf("first setup failed: %#v", setupResult)
	}
	if len(setupRecorder.Result().Cookies()) == 0 {
		t.Fatal("successful setup did not create a login session")
	}
	authStatusRecorder := httptest.NewRecorder()
	authStatusRequest := httptest.NewRequest(http.MethodGet, "/app/api/setup-status", nil)
	for _, cookie := range setupRecorder.Result().Cookies() {
		authStatusRequest.AddCookie(cookie)
	}
	engine.ServeHTTP(authStatusRecorder, authStatusRequest)
	var authStatus Msg
	if err := json.Unmarshal(authStatusRecorder.Body.Bytes(), &authStatus); err != nil {
		t.Fatal(err)
	}
	authStatusObj, ok := authStatus.Obj.(map[string]interface{})
	if !authStatus.Success || !ok || authStatusObj["required"] != false || authStatusObj["authenticated"] != true {
		t.Fatalf("authenticated setup status = %#v", authStatus)
	}

	secondRecorder := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodPost, "/app/api/setup", strings.NewReader(crossSite.Encode()))
	secondRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	secondRequest.Host = "panel.example"
	engine.ServeHTTP(secondRecorder, secondRequest)
	var secondResult Msg
	if err := json.Unmarshal(secondRecorder.Body.Bytes(), &secondResult); err != nil {
		t.Fatal(err)
	}
	if secondResult.Success {
		t.Fatal("second setup request was accepted")
	}
}

func TestManagedLoginAcceptsOnlyOneTimeGrant(t *testing.T) {
	logger.InitLogger(logging.ERROR)
	if err := database.InitDB(filepath.Join(t.TempDir(), "managed-login.db")); err != nil {
		t.Fatal(err)
	}
	if _, err := (&service.UserService{}).InitializeAdmin("child-admin", "secure-password"); err != nil {
		t.Fatal(err)
	}
	grant, err := (&service.ManagedAccessService{}).Issue(service.ManagedPanelAccessRequest{Actor: "controller-admin"})
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("s-ui", cookie.NewStore([]byte("0123456789abcdef0123456789abcdef"))))
	policy, err := NewOriginPolicy("")
	if err != nil {
		t.Fatal(err)
	}
	NewAPIHandler(engine.Group("/app/api"), nil, policy)

	values := url.Values{"token": {grant.Token}}
	request := httptest.NewRequest(http.MethodPost, "/app/api/managed-login", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://controller.example")
	request.Host = "child.example"
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/app/" {
		t.Fatalf("managed login did not redirect to the child panel: status=%d location=%q", recorder.Code, recorder.Header().Get("Location"))
	}
	if len(recorder.Result().Cookies()) == 0 {
		t.Fatal("managed login did not create a child-panel session")
	}

	replay := httptest.NewRequest(http.MethodPost, "/app/api/managed-login", strings.NewReader(values.Encode()))
	replay.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	replay.Host = "child.example"
	replayRecorder := httptest.NewRecorder()
	engine.ServeHTTP(replayRecorder, replay)
	if replayRecorder.Code != http.StatusSeeOther || !strings.Contains(replayRecorder.Header().Get("Location"), "managed=failed") {
		t.Fatalf("replayed managed login token was not rejected: status=%d location=%q", replayRecorder.Code, replayRecorder.Header().Get("Location"))
	}
}
