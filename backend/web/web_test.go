package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hhz0823/1s-ui/api"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
)

func initWebTestDB(t *testing.T) {
	t.Helper()
	logger.InitLogger(logging.CRITICAL)
	if err := database.InitDB(filepath.Join(t.TempDir(), "web.db")); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
}

func TestRouterProvidesCanonicalAndLegacyAPIRoutes(t *testing.T) {
	initWebTestDB(t)
	server := NewServer()
	engine, err := server.initRouter()
	if err != nil {
		t.Fatal(err)
	}

	for _, route := range []string{"/api/setup-status", "/app/api/setup-status"} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, route, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, body = %q", route, recorder.Code, recorder.Body.String())
		}
		var result api.Msg
		if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil || !result.Success {
			t.Fatalf("GET %s returned %#v, error %v", route, result, err)
		}
	}

	for _, route := range []string{"/agent/v1/ws", "/app/agent/v1/ws"} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, route, nil))
		if recorder.Code == http.StatusNotFound {
			t.Fatalf("legacy/canonical Agent route %s was not registered", route)
		}
	}
	for _, route := range []string{"/apiv2/status", "/app/apiv2/status"} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, route, nil))
		if recorder.Code == http.StatusNotFound {
			t.Fatalf("legacy/canonical API v2 route %s was not registered", route)
		}
	}
}

func TestNoRouteIsJSON404(t *testing.T) {
	initWebTestDB(t)
	server := NewServer()
	engine, err := server.initRouter()
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/app/login", nil))
	if recorder.Code != http.StatusNotFound || recorder.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("NoRoute = %d %q, body %q", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.String())
	}
}

func TestServerStartsWithoutFrontendFiles(t *testing.T) {
	initWebTestDB(t)
	t.Chdir(t.TempDir())
	t.Setenv("SUI_API_LISTEN", "127.0.0.1")
	t.Setenv("SUI_API_PORT", "0")
	t.Setenv("SUI_CONTROL_SOCKET", filepath.Join(t.TempDir(), "control.sock"))

	server := NewServer()
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Stop() })
	response, err := http.Get("http://" + server.listener.Addr().String() + "/api/setup-status")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("API status = %d", response.StatusCode)
	}
}

func TestRoutePathsDeduplicatesRootLegacyAlias(t *testing.T) {
	paths := routePaths("/", "api")
	if len(paths) != 1 || paths[0] != "/api" {
		t.Fatalf("routePaths = %#v", paths)
	}
}

// With SUI_FRONTEND_DIR (OpenWrt), the panel serves the web UI itself.
func TestServesFrontendFromDirectory(t *testing.T) {
	initWebTestDB(t)
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "assets"), 0o755)
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>ui</html>"), 0o644)
	os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log(1)"), 0o644)
	t.Setenv("SUI_FRONTEND_DIR", dir)
	server := NewServer()
	engine, err := server.initRouter()
	if err != nil {
		t.Fatal(err)
	}
	get := func(target string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		return recorder
	}
	for target, want := range map[string]string{"/app/": "<html>ui</html>", "/app/proxy-client": "<html>ui</html>", "/app/assets/app.js": "console.log(1)"} {
		if response := get(target); response.Code != http.StatusOK || response.Body.String() != want {
			t.Fatalf("%s: %d %q", target, response.Code, response.Body.String())
		}
	}
	if response := get("/app/assets/app.js"); !strings.Contains(response.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("assets are not cached: %q", response.Header().Get("Cache-Control"))
	}
	if response := get("/.well-known/1s-ui/config.js"); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"basePath":"/app/"`) {
		t.Fatalf("config.js: %d %q", response.Code, response.Body.String())
	}
	if response := get("/"); response.Code != http.StatusFound || response.Header().Get("Location") != "/app/" {
		t.Fatalf("root: %d %q", response.Code, response.Header().Get("Location"))
	}
	if response := get("/app"); response.Code != http.StatusPermanentRedirect || response.Header().Get("Location") != "/app/" {
		t.Fatalf("bare path: %d %q", response.Code, response.Header().Get("Location"))
	}
	// API paths never get the UI page, and nothing outside the UI directory
	// is served.
	for _, target := range []string{"/app/api/a/b/c/d", "/app/apiv2/x/y/z/w", "/app/agent/v1/a/b/c", "/other"} {
		response := get(target)
		if response.Code < 400 || strings.Contains(response.Body.String(), "<html>") || response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
			t.Fatalf("%s: %d %q", target, response.Code, response.Body.String())
		}
	}
	if response := get("/app/../../etc/passwd"); strings.Contains(response.Body.String(), "root:") {
		t.Fatal("served a file outside the UI directory")
	}
}
