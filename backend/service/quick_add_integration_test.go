package service

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/util"
	"github.com/op/go-logging"
	"github.com/sagernet/sing-box/option"
)

func setupQuickAddTest(t *testing.T) *LocalControlService {
	t.Helper()
	logger.InitLogger(logging.CRITICAL)
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	t.Setenv("SUI_SKIP_CORE", "true")
	if err := database.InitDB(filepath.Join(dir, "quick-add.db")); err != nil {
		t.Fatal(err)
	}
	oldCore, oldXray := corePtr, xrayPtr
	corePtr = core.NewCore()
	xrayPtr = core.NewXrayRuntime()
	t.Cleanup(func() { corePtr, xrayPtr = oldCore, oldXray })
	return &LocalControlService{}
}

func TestQuickAddNodesAreImportableAndLoadable(t *testing.T) {
	control := setupQuickAddTest(t)
	list, err := control.ListInbounds()
	if err != nil {
		t.Fatal(err)
	}
	revision := list.Revision
	port := 42000
	protocols := []string{"mixed", "socks", "http", "shadowsocks", "vmess", "trojan", "vless", "hysteria2", "tuic", "anytls"}
	for _, protocol := range protocols {
		response, err := control.QuickAddInbounds(RemoteQuickAddRequest{
			CoreType: model.CoreTypeSingBox, Protocol: protocol, Count: 1, Port: port,
			ExpectedRevision: revision, Actor: "test", PublicHost: "198.51.100.50",
		})
		if err != nil {
			t.Fatalf("quick add %s: %v", protocol, err)
		}
		revision = response.Revision
		port += 10
		if len(response.Created) != 1 || response.Created[0].ClientID == 0 {
			t.Fatalf("quick add %s must create a user so a share link exists: %#v", protocol, response.Created)
		}
	}

	var clients []model.Client
	if err = database.GetDB().Find(&clients).Error; err != nil {
		t.Fatal(err)
	}
	if len(clients) != len(protocols) {
		t.Fatalf("clients = %d, want %d", len(clients), len(protocols))
	}
	for _, client := range clients {
		var links []map[string]string
		if err = json.Unmarshal(client.Links, &links); err != nil || len(links) == 0 {
			t.Fatalf("client %s has no links: %s", client.Name, client.Links)
		}
		for _, link := range links {
			if strings.HasPrefix(link["uri"], "socks5://") {
				t.Fatalf("v2rayN cannot import socks5:// links: %q", link["uri"])
			}
			if _, _, err = util.GetOutbound(link["uri"], 0); err != nil {
				t.Fatalf("generated link %q is not importable: %v", link["uri"], err)
			}
		}
	}

	var vmess model.Inbound
	if err = database.GetDB().Where("type = ?", "vmess").First(&vmess).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(vmess.Options), `"ws"`) {
		t.Fatalf("sing-box VMess quick add must use WebSocket: %s", vmess.Options)
	}

	inboundConfigs, err := (&InboundService{}).GetAllConfig(database.GetDB())
	if err != nil {
		t.Fatal(err)
	}
	ctx := corePtr.GetCtx()
	startable := []json.RawMessage{}
	for _, raw := range inboundConfigs {
		var inbound option.Inbound
		if err = inbound.UnmarshalJSONContext(ctx, raw); err != nil {
			t.Fatalf("sing-box rejected quick-add inbound:\n%s\n%v", raw, err)
		}
		var fields map[string]interface{}
		_ = json.Unmarshal(raw, &fields)
		if users, _ := fields["users"].([]interface{}); len(users) == 0 {
			t.Fatalf("quick-add inbound has no users (open proxy): %s", raw)
		}
		if inbound.Type != "hysteria2" && inbound.Type != "tuic" {
			startable = append(startable, raw) // QUIC needs the with_quic build tag
		}
	}
	config, err := json.Marshal(map[string]interface{}{
		"inbounds":  startable,
		"outbounds": []interface{}{map[string]interface{}{"type": "direct", "tag": "direct"}},
		"route":     map[string]interface{}{"final": "direct"},
	})
	if err != nil {
		t.Fatal(err)
	}
	instance := core.NewCore()
	if err = instance.Start(config); err != nil {
		t.Fatalf("sing-box could not start quick-add inbounds: %v\n%s", err, config)
	}
	if err = instance.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestQuickAddRejectsShadowTLS(t *testing.T) {
	request := RemoteQuickAddRequest{CoreType: model.CoreTypeSingBox, Protocol: "shadowtls", Count: 1, Port: 443}
	if err := validateRemoteQuickAddRequest(&request); err == nil {
		t.Fatal("ShadowTLS quick add cannot produce a working, importable node and must be rejected")
	}
}

func TestVLESSInboundDropsVisionOnWebSocket(t *testing.T) {
	withTLS := map[string]interface{}{"tls": map[string]interface{}{"enabled": true}}
	if !vlessInboundSupportsVision(withTLS) {
		t.Fatal("raw TCP + TLS must keep Vision")
	}
	withTLS["transport"] = map[string]interface{}{"type": "ws"}
	if vlessInboundSupportsVision(withTLS) {
		t.Fatal("WebSocket must not use Vision")
	}
	if vlessInboundSupportsVision(map[string]interface{}{}) {
		t.Fatal("plain VLESS must not use Vision")
	}
}
