package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/proxyprobe"
	"github.com/Hhz0823/1s-ui/util"

	"github.com/op/go-logging"
)

const testNodeUUID = "9f1c3b5e-6a2d-4c8e-9b7a-1d2e3f4a5b6c"

func freeUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenPacket("udp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

type testNodes struct {
	vless, shadowsocks, hysteria2, trojan, closed int
}

// startTestNodes runs real VLESS, Shadowsocks, Hysteria2 and Trojan servers
// the way a user's VPS would, listening on every address.
func startTestNodes(t *testing.T) testNodes {
	t.Helper()
	logger.InitLogger(logging.CRITICAL)
	nodes := testNodes{vless: freeTCPPort(t), shadowsocks: freeTCPPort(t), hysteria2: freeUDPPort(t), trojan: freeTCPPort(t), closed: freeTCPPort(t)}
	dir := t.TempDir()
	key, cert, err := util.GenerateSelfSignedTLS("probe.test", time.Now(), time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	keyPath, certPath := filepath.Join(dir, "key.pem"), filepath.Join(dir, "cert.pem")
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certPath, cert, 0o600); err != nil {
		t.Fatal(err)
	}
	serverTLS := map[string]interface{}{"enabled": true, "server_name": "probe.test", "certificate_path": certPath, "key_path": keyPath}
	config, _ := json.Marshal(map[string]interface{}{
		"log": map[string]interface{}{"level": "error"},
		"inbounds": []map[string]interface{}{
			{"type": "vless", "tag": "vless", "listen": "0.0.0.0", "listen_port": nodes.vless, "users": []map[string]string{{"uuid": testNodeUUID}}},
			{"type": "shadowsocks", "tag": "ss", "listen": "0.0.0.0", "listen_port": nodes.shadowsocks, "method": "aes-128-gcm", "password": "ss-secret"},
			{"type": "hysteria2", "tag": "hy2", "listen": "0.0.0.0", "listen_port": nodes.hysteria2, "users": []map[string]string{{"password": "hy2-secret"}}, "tls": serverTLS},
			{"type": "trojan", "tag": "trojan", "listen": "0.0.0.0", "listen_port": nodes.trojan, "users": []map[string]string{{"password": "trojan-secret"}}, "tls": serverTLS},
		},
		"outbounds": []map[string]interface{}{{"type": "direct", "tag": "direct"}},
	})
	box := core.NewCore()
	if err := box.Start(config); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = box.Stop() })
	return nodes
}

func TestNodeProbesThroughRealNodes(t *testing.T) {
	host := hostAddress(t)
	nodes := startTestNodes(t)
	_, _, target := startTestProxies(t, host)
	ssUser := base64.RawURLEncoding.EncodeToString([]byte("aes-128-gcm:ss-secret"))
	links := []string{
		fmt.Sprintf("vless://%s@%s:%d?type=tcp&security=none&encryption=none#vless-node", testNodeUUID, host, nodes.vless),
		fmt.Sprintf("ss://%s@%s:%d#ss-node", ssUser, host, nodes.shadowsocks),
		fmt.Sprintf("hy2://hy2-secret@%s:%d?insecure=1&sni=probe.test#hy2-node", host, nodes.hysteria2),
		fmt.Sprintf("trojan://trojan-secret@%s:%d?security=tls&sni=probe.test&allowInsecure=1&type=tcp#trojan-node", host, nodes.trojan),
		fmt.Sprintf("vless://00000000-0000-4000-8000-000000000000@%s:%d?type=tcp&security=none&encryption=none#wrong-uuid", host, nodes.vless),
		fmt.Sprintf("hy2://wrong@%s:%d?insecure=1&sni=probe.test#wrong-password", host, nodes.hysteria2),
		fmt.Sprintf("vless://%s@%s:%d?type=tcp&security=none#closed", testNodeUUID, host, nodes.closed),
		"vless://not-a-node",
		fmt.Sprintf("vless://%s@127.0.0.1:%d?type=tcp&security=none#loopback", testNodeUUID, nodes.vless),
	}
	request := ProxyProbeRequest{}
	for _, link := range links {
		request.Probes = append(request.Probes, proxyprobe.Spec{Type: proxyprobe.TypeNode, Link: link, Target: target})
	}
	// A SOCKS5 proxy in the same request still goes the direct way.
	request.Probes = append(request.Probes, proxyprobe.Spec{Type: "socks5", Host: host, Port: nodes.closed, Target: target})
	started := time.Now()
	response, err := RunProxyProbes(request)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 30*time.Second {
		t.Fatalf("checks took %s", elapsed)
	}
	results := response.Results
	for i, name := range []string{"vless", "shadowsocks", "hysteria2", "trojan"} {
		result := results[i]
		if !result.OK || result.ExitIP != "198.51.100.77" || result.Country != "SG" || result.LatencyMs <= 0 || result.HandshakeMs <= 0 {
			t.Fatalf("%s: %+v", name, result)
		}
		tcp := name != "hysteria2"
		if tcp != (result.ConnectMs > 0) {
			t.Fatalf("%s: TCP pre-check %d ms", name, result.ConnectMs)
		}
	}
	expect := []struct {
		name  string
		index int
		stage string
	}{
		{"wrong uuid", 4, proxyprobe.StageTunnel},
		{"wrong hysteria2 password", 5, proxyprobe.StageHandshake},
		{"closed port", 6, proxyprobe.StageConnect},
		{"broken link", 7, proxyprobe.StageConfig},
		{"loopback node", 8, proxyprobe.StageConfig},
		{"socks5 on a closed port", 9, proxyprobe.StageConnect},
	}
	for _, item := range expect {
		if result := results[item.index]; result.OK || result.Stage != item.stage || result.Error == "" {
			t.Fatalf("%s: %+v", item.name, result)
		}
	}
}

func TestParseNodeLink(t *testing.T) {
	_, info, err := ParseNodeLink("vless://" + testNodeUUID + "@node.example.com:8443?type=tcp&security=reality&pbk=abc&sni=www.example.com#Tokyo%201")
	if err != nil || info.Protocol != "vless" || info.Host != "node.example.com" || info.Port != 8443 || info.Name != "Tokyo 1" {
		t.Fatalf("vless: %+v %v", info, err)
	}
	if _, _, err := ParseNodeLink("socks5://u:p@1.2.3.4:1080"); err == nil {
		t.Fatal("socks5 links are proxies, not nodes")
	}
	outbound, info, err := ParseNodeLink("hy2://pw@203.0.113.5:443?sni=a.example#hy")
	if err != nil || info.Protocol != "hysteria2" || nodeUsesTCP(outbound) {
		t.Fatalf("hysteria2: %+v %v", info, err)
	}
	if !IsNodeLink("trojan://x@h:1") || IsNodeLink("http://h:1") || IsNodeLink("h:1:u:p") {
		t.Fatal("IsNodeLink")
	}
}

func TestNodeMonitorsFromLinksAndInbounds(t *testing.T) {
	logger.InitLogger(logging.CRITICAL)
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "node-monitors.db")); err != nil {
		t.Fatal(err)
	}
	proxyMonitorState.Lock()
	proxyMonitorState.lastRun, proxyMonitorState.busy = nil, map[uint]bool{}
	proxyMonitorState.Unlock()
	host := hostAddress(t)
	nodes := startTestNodes(t)
	_, _, target := startTestProxies(t, host)
	vlessLink := fmt.Sprintf("vless://%s@%s:%d?type=tcp&security=none&encryption=none#Home%%20VLESS", testNodeUUID, host, nodes.vless)
	hy2Link := fmt.Sprintf("hy2://hy2-secret@%s:%d?insecure=1&sni=probe.test", host, nodes.hysteria2)

	// One of this panel's own inbounds, with a user whose link reaches it.
	inbound := model.Inbound{Type: "vless", Tag: "vless-home"}
	if err := database.GetDB().Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	links, _ := json.Marshal([]map[string]string{{"remark": "vless-home", "type": "local", "uri": vlessLink}})
	disabledLinks, _ := json.Marshal([]map[string]string{{"remark": "vless-home", "type": "local", "uri": strings.Replace(vlessLink, testNodeUUID, "00000000-0000-4000-8000-000000000000", 1)}})
	inboundIDs, _ := json.Marshal([]uint{inbound.Id})
	for _, client := range []model.Client{
		{Name: "off", Enable: false, Inbounds: inboundIDs, Links: disabledLinks, Config: json.RawMessage(`{}`)},
		{Name: "on", Enable: true, Inbounds: inboundIDs, Links: links, Config: json.RawMessage(`{}`)},
	} {
		if err := database.GetDB().Create(&client).Error; err != nil {
			t.Fatal(err)
		}
	}

	service := ProxyMonitorService{}
	pasted, err := service.Save(ProxyMonitorInput{Link: hy2Link, Target: target})
	if err != nil || pasted.Type != proxyprobe.TypeNode || pasted.Protocol != "hysteria2" || pasted.Host != host || pasted.Port != nodes.hysteria2 ||
		pasted.Name != fmt.Sprintf("hysteria2-%s-%d", host, nodes.hysteria2) {
		t.Fatalf("pasted node: %+v %v", pasted, err)
	}
	picked, err := service.Save(ProxyMonitorInput{NodeInboundId: inbound.Id, Target: target})
	if err != nil || picked.Type != proxyprobe.TypeNode || picked.Protocol != "vless" || picked.Name != "vless-home" || picked.NodeInboundId != inbound.Id {
		t.Fatalf("picked node: %+v %v", picked, err)
	}
	if _, err := service.Save(ProxyMonitorInput{NodeInboundId: inbound.Id + 100}); err == nil {
		t.Fatal("saved a node from a missing inbound")
	}
	var stored model.ProxyMonitor
	database.GetDB().Where("id = ?", picked.Id).First(&stored)
	if stored.Link != vlessLink || stored.LinkUpdatedAt == 0 {
		t.Fatalf("stored link %q", stored.Link)
	}

	service.RunDue(time.Now())
	waitForResults(t, 2)
	views, err := service.List()
	if err != nil || len(views) != 2 {
		t.Fatalf("list: %d %v", len(views), err)
	}
	for _, view := range views {
		if view.Status != "up" || view.Last == nil || view.Last.ExitIP != "198.51.100.77" {
			t.Fatalf("%s: %+v %+v", view.Name, view, view.Last)
		}
	}
	raw, _ := json.Marshal(views)
	if strings.Contains(string(raw), testNodeUUID) || strings.Contains(string(raw), "hy2-secret") || strings.Contains(string(raw), "vless://") {
		t.Fatalf("monitor list exposes a node link: %s", raw)
	}

	// Renaming keeps the node and its history; pointing it at another
	// node starts over.
	renamed, err := service.Save(ProxyMonitorInput{Id: picked.Id, Name: "home", NodeInboundId: inbound.Id, Target: target, Interval: 120})
	if err != nil || renamed.Name != "home" || renamed.Protocol != "vless" {
		t.Fatalf("rename: %+v %v", renamed, err)
	}
	var count int64
	database.GetDB().Model(&model.ProxyMonitorResult{}).Where("monitor_id = ?", picked.Id).Count(&count)
	if count != 1 {
		t.Fatalf("rename dropped the history: %d", count)
	}
	edited, err := service.Save(ProxyMonitorInput{Id: pasted.Id, Name: "hy2", Target: target})
	if err != nil || edited.Type != proxyprobe.TypeNode || edited.Protocol != "hysteria2" {
		t.Fatalf("edit without a link: %+v %v", edited, err)
	}
	database.GetDB().Model(&model.ProxyMonitorResult{}).Where("monitor_id = ?", pasted.Id).Count(&count)
	if count != 1 {
		t.Fatalf("editing the name dropped the history: %d", count)
	}
	result, err := service.Test(ProxyMonitorInput{Id: pasted.Id, Target: target})
	if err != nil || !result.OK {
		t.Fatalf("test stored node: %+v %v", result, err)
	}

	// A changed inbound link is picked up by the refresh.
	moved := strings.Replace(vlessLink, "Home%20VLESS", "Moved", 1)
	links, _ = json.Marshal([]map[string]string{{"remark": "vless-home", "type": "local", "uri": moved}})
	database.GetDB().Model(&model.Client{}).Where("name = ?", "on").Update("links", links)
	service.RefreshNodeLinks(time.Now().Add(11 * time.Minute))
	database.GetDB().Where("id = ?", picked.Id).First(&stored)
	if stored.Link != moved {
		t.Fatalf("refresh kept %q", stored.Link)
	}
}
