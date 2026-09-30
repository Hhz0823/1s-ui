package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/service"
	"github.com/gin-gonic/gin"
)

func TestAgentPublicEndpointsRequireControllerMode(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "agent-handler.db")); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewAgentHandler(engine.Group("/app/agent/v1"))

	disabled := httptest.NewRecorder()
	disabledRequest := httptest.NewRequest(http.MethodPost, "/app/agent/v1/heartbeat", strings.NewReader(`{}`))
	disabledRequest.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(disabled, disabledRequest)
	if disabled.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled controller heartbeat status = %d, want %d", disabled.Code, http.StatusServiceUnavailable)
	}

	if err := database.GetDB().Create(&model.Setting{Key: "controllerMode", Value: "enabled"}).Error; err != nil {
		t.Fatal(err)
	}
	enabled := httptest.NewRecorder()
	enabledRequest := httptest.NewRequest(http.MethodPost, "/app/agent/v1/heartbeat", strings.NewReader(`{}`))
	enabledRequest.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(enabled, enabledRequest)
	if enabled.Code != http.StatusUnauthorized {
		t.Fatalf("enabled controller did not reach Agent authentication: status = %d", enabled.Code)
	}
}

func TestAgentAddressOnlyEnrollmentConsumesWindow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "agent-address-enrollment.db")); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Create(&model.Setting{Key: "controllerMode", Value: "full"}).Error; err != nil {
		t.Fatal(err)
	}
	settings := &service.SettingService{}
	settings.OpenAgentAddressPairing(0)
	t.Cleanup(settings.CloseAgentAddressPairing)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewAgentHandler(engine.Group("/app/agent/v1"))
	requestBody, _ := json.Marshal(map[string]string{"name": "address-only-node"})

	first := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodPost, "/app/agent/v1/enroll", strings.NewReader(string(requestBody)))
	firstRequest.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(first, firstRequest)
	if first.Code != http.StatusOK {
		t.Fatalf("first address enrollment status = %d, body = %s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodPost, "/app/agent/v1/enroll", strings.NewReader(string(requestBody)))
	secondRequest.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(second, secondRequest)
	if second.Code != http.StatusUnauthorized {
		t.Fatalf("reused address enrollment status = %d, want %d", second.Code, http.StatusUnauthorized)
	}

	var count int64
	if err := database.GetDB().Model(&model.AgentNode{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("address enrollment created unexpected nodes: count=%d err=%v", count, err)
	}
}

func TestAgentKeyEnrollmentIsReusableUntilRevoked(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "agent-key-enrollment.db")); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Create(&model.Setting{Key: "controllerMode", Value: "full"}).Error; err != nil {
		t.Fatal(err)
	}
	settings := &service.SettingService{}
	key, err := settings.RotateAgentEnrollmentKey()
	if err != nil {
		t.Fatal(err)
	}
	if !settings.HasAgentEnrollmentKey() {
		t.Fatal("rotated enrollment key is not reported as active")
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewAgentHandler(engine.Group("/app/agent/v1"))
	enroll := func(code string) int {
		body, _ := json.Marshal(map[string]string{"code": code, "name": "fnos-nas"})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/app/agent/v1/enroll", strings.NewReader(string(body)))
		request.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(recorder, request)
		return recorder.Code
	}

	for i := 0; i < 2; i++ {
		if code := enroll(key); code != http.StatusOK {
			t.Fatalf("key enrollment %d status = %d, want %d", i+1, code, http.StatusOK)
		}
	}
	if code := enroll(strings.Repeat("x", 43)); code != http.StatusUnauthorized {
		t.Fatalf("wrong key status = %d, want %d", code, http.StatusUnauthorized)
	}
	if err := settings.RevokeAgentEnrollmentKey(); err != nil {
		t.Fatal(err)
	}
	if settings.HasAgentEnrollmentKey() {
		t.Fatal("revoked enrollment key is still reported as active")
	}
	if code := enroll(key); code != http.StatusUnauthorized {
		t.Fatalf("revoked key status = %d, want %d", code, http.StatusUnauthorized)
	}
}
