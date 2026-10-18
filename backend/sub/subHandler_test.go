package sub

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
)

func TestIsSingBoxApp(t *testing.T) {
	agents := map[string]bool{
		"SFW (sing-box 1.15.0-alpha.12; language zh_CN)":            true,
		"SFL (sing-box 1.15.0; language en_US)":                     true,
		"SFA/1.12.4 (612; sing-box 1.12.4; language zh_CN)":         true,
		"SFI/1.13.0 (Apple iOS 26.0; sing-box 1.13.0)":              true,
		"SFM/1.13.0 (sing-box 1.13.0)":                              true,
		"HiddifyNext/2.5.7 (android) like ClashMeta v2ray sing-box": false,
		"NekoBox/Android 1.3.9":                                     false,
		"v2rayN/7.13.8":                                             false,
		"clash.meta":                                                false,
		"Shadowrocket/2.2.92 CFNetwork/3826 Darwin/24.4.0":          false,
		"SFTP client": false,
		"curl/8.5.0":  false,
		"":            false,
	}
	for agent, want := range agents {
		if got := isSingBoxApp(agent); got != want {
			t.Errorf("isSingBoxApp(%q) = %v, want %v", agent, got, want)
		}
	}
}

func TestSubscriptionServesJSONToSingBoxApps(t *testing.T) {
	logger.InitLogger(logging.CRITICAL)
	if err := database.InitDB(filepath.Join(t.TempDir(), "sub.db")); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	db := database.GetDB()
	inbound := model.Inbound{
		Type: "shadowsocks", Tag: "ss-test",
		Options: json.RawMessage(`{"method":"aes-256-gcm","listen_port":8388}`),
		OutJson: json.RawMessage(`{"type":"shadowsocks","tag":"ss-test","server":"198.51.100.10","server_port":8388,"method":"aes-256-gcm"}`),
		Addrs:   json.RawMessage(`[]`),
	}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	client := model.Client{
		Enable: true, Name: "alice",
		Config:   json.RawMessage(`{"shadowsocks":{"name":"alice","password":"pw"}}`),
		Inbounds: json.RawMessage(`[` + jsonNumber(inbound.Id) + `]`),
		Links:    json.RawMessage(`[{"type":"local","remark":"ss-test","uri":"ss://YWVzLTI1Ni1nY206cHc@198.51.100.10:8388#ss-test"}]`),
	}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	NewSubHandler(engine.Group("/sub"))
	get := func(path, agent string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("User-Agent", agent)
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s (%s) = %d %q", path, agent, recorder.Code, recorder.Body.String())
		}
		return recorder
	}
	assertJSON := func(recorder *httptest.ResponseRecorder, what string) {
		t.Helper()
		if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
			t.Fatalf("%s: Content-Type = %q", what, contentType)
		}
		var config struct {
			Outbounds []map[string]interface{} `json:"outbounds"`
			Inbounds  []map[string]interface{} `json:"inbounds"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &config); err != nil {
			t.Fatalf("%s: not a sing-box config: %v\n%s", what, err, recorder.Body.String())
		}
		found := false
		for _, outbound := range config.Outbounds {
			if outbound["tag"] == "ss-test" && outbound["password"] == "pw" {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: shadowsocks outbound missing: %s", what, recorder.Body.String())
		}
		for _, in := range config.Inbounds {
			if _, ok := in["stack"]; ok {
				t.Fatalf("%s: TUN still sets the stack deprecated in sing-box 1.15: %v", what, in)
			}
		}
	}

	// The sing-box apps get JSON from the plain link.
	assertJSON(get("/sub/alice", "SFW (sing-box 1.15.0-alpha.12; language zh_CN)"), "SFW")
	assertJSON(get("/sub/alice", "SFA/1.12.4 (612; sing-box 1.12.4; language zh_CN)"), "SFA")
	assertJSON(get("/sub/alice?format=json", "curl/8.5.0"), "format=json")

	// Everyone else keeps the base64 link list.
	for _, agent := range []string{"v2rayN/7.13.8", "HiddifyNext/2.5.7 (android) like ClashMeta v2ray sing-box", ""} {
		body := get("/sub/alice", agent).Body.String()
		decoded, err := base64.StdEncoding.DecodeString(body)
		if err != nil || !strings.HasPrefix(string(decoded), "ss://") {
			t.Fatalf("%q: want the base64 link list, got %q", agent, body)
		}
	}
	// An explicit format wins over the user agent.
	if body := get("/sub/alice?format=clash", "SFW (sing-box 1.15.0; language en_US)").Body.String(); !strings.Contains(body, "proxies:") {
		t.Fatalf("format=clash from a sing-box app: %q", body)
	}
}

func jsonNumber(id uint) string {
	data, _ := json.Marshal(id)
	return string(data)
}
