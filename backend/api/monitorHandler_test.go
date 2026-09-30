package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/service"
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
