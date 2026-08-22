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
	if !strings.Contains(command, " --connect '"+connectURL+"'") {
		t.Fatalf("managed install command does not contain the public panel address: %q", command)
	}
	if strings.Contains(command, "#") || strings.Contains(command, "--managed-client") || strings.Contains(command, " -y ") {
		t.Fatalf("managed install command still depends on the legacy mode selection: %q", command)
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
