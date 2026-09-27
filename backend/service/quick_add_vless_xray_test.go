//go:build !openwrt_lite

package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
)

// These tests run the real Xray-core binary named by XRAY_TEST_BINARY on
// both ends: the server from the configuration the panel generates, the
// client from nothing but the share link, the way v2rayN and v2rayNG use it
// and the way PassWall / PassWall 2 import it (passwallXrayOutbound).

func requireXrayBinary(t *testing.T) string {
	t.Helper()
	binary := os.Getenv("XRAY_TEST_BINARY")
	if binary == "" {
		t.Skip("XRAY_TEST_BINARY is not set")
	}
	return binary
}

// xrayClientOutbound turns a vless:// share link into the Xray outbound a
// client builds from it. fingerprint overrides the link's uTLS fingerprint.
func xrayClientOutbound(t *testing.T, link, fingerprint string) map[string]interface{} {
	t.Helper()
	parsed, err := url.Parse(link)
	if err != nil || parsed.Scheme != "vless" {
		t.Fatalf("not a vless link: %q (%v)", link, err)
	}
	query := parsed.Query()
	port, _ := strconv.Atoi(parsed.Port())
	user := map[string]interface{}{"id": parsed.User.Username(), "encryption": query.Get("encryption")}
	if flow := query.Get("flow"); flow != "" {
		user["flow"] = flow
	}
	stream := map[string]interface{}{"network": query.Get("type")}
	fp := query.Get("fp")
	if fingerprint != "" {
		fp = fingerprint
	}
	switch query.Get("security") {
	case "reality":
		stream["security"] = "reality"
		stream["realitySettings"] = map[string]interface{}{
			"serverName": query.Get("sni"), "fingerprint": fp, "publicKey": query.Get("pbk"),
			"shortId": query.Get("sid"), "spiderX": query.Get("spx"),
		}
	case "tls":
		tlsSettings := map[string]interface{}{"serverName": query.Get("sni")}
		if fp != "" {
			tlsSettings["fingerprint"] = fp
		}
		if pin := query.Get("pcs"); pin != "" {
			tlsSettings["pinnedPeerCertSha256"] = pin
		} else if query.Get("allowInsecure") == "1" {
			tlsSettings["allowInsecure"] = true
		}
		if alpn := query.Get("alpn"); alpn != "" {
			tlsSettings["alpn"] = strings.Split(alpn, ",")
		}
		stream["security"] = "tls"
		stream["tlsSettings"] = tlsSettings
	default:
		stream["security"] = "none"
	}
	if query.Get("type") == "xhttp" {
		xhttp := map[string]interface{}{"path": query.Get("path"), "mode": query.Get("mode")}
		if host := query.Get("host"); host != "" {
			xhttp["host"] = host
		}
		// v2rayN and v2rayNG hand "extra" to Xray-core as it is.
		if extra := query.Get("extra"); extra != "" {
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(extra), &parsed); err != nil {
				t.Fatalf("extra is not JSON: %q", extra)
			}
			xhttp["extra"] = parsed
		}
		stream["xhttpSettings"] = xhttp
	}
	return map[string]interface{}{
		"protocol": "vless", "tag": "proxy",
		"settings": map[string]interface{}{"vnext": []interface{}{map[string]interface{}{
			"address": parsed.Hostname(), "port": port, "users": []interface{}{user},
		}}},
		"streamSettings": stream,
	}
}

// startXrayClient runs a separate Xray process with a SOCKS entry in front of
// the outbound built from the link and returns the SOCKS port.
func startXrayClient(t *testing.T, binary, link, fingerprint string) int {
	t.Helper()
	return startXrayClientOutbound(t, binary, xrayClientOutbound(t, link, fingerprint))
}

func startXrayClientOutbound(t *testing.T, binary string, outbound map[string]interface{}) int {
	t.Helper()
	socksPort := freeLocalPort(t)
	config := map[string]interface{}{
		"log": map[string]interface{}{"loglevel": "warning"},
		"inbounds": []interface{}{map[string]interface{}{
			"tag": "socks", "listen": "127.0.0.1", "port": socksPort, "protocol": "socks",
			"settings": map[string]interface{}{"udp": true},
		}},
		"outbounds": []interface{}{outbound},
	}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "client.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	cmd := exec.Command(binary, "run", "-config", path)
	cmd.Stdout, cmd.Stderr = &output, &output
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if t.Failed() {
			t.Logf("xray client config: %s\noutput:\n%s", raw, output.String())
		}
	})
	waitTCP(t, fmt.Sprintf("127.0.0.1:%d", socksPort))
	return socksPort
}

// passwallClientOutbound is passwallXrayOutbound for the test binary. PassWall
// writes streamSettings.method, which Xray-core reads since v26.7.11 (and
// PassWall installs a current Xray-core); older binaries get it as network.
func passwallClientOutbound(t *testing.T, binary string, node passwallNode) map[string]interface{} {
	t.Helper()
	outbound := passwallXrayOutbound(node)
	output, err := exec.Command(binary, "version").Output()
	if err != nil {
		t.Fatalf("xray version: %v", err)
	}
	var major, minor, patch int
	if _, err = fmt.Sscanf(string(output), "Xray %d.%d.%d", &major, &minor, &patch); err != nil {
		t.Fatalf("unexpected xray version output %q", output)
	}
	if major < 26 || (major == 26 && (minor < 7 || (minor == 7 && patch < 11))) {
		stream := outbound["streamSettings"].(map[string]interface{})
		stream["network"] = stream["method"]
		delete(stream, "method")
	}
	return outbound
}

func waitTCP(t *testing.T, address string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if conn, err := net.DialTimeout("tcp", address, 300*time.Millisecond); err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s never started listening", address)
}

func fetchThroughSocks(socksPort int, target string) error {
	proxy, _ := url.Parse(fmt.Sprintf("socks5://127.0.0.1:%d", socksPort))
	client := http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy)}, Timeout: 10 * time.Second}
	defer client.CloseIdleConnections()
	response, err := client.Get(target)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status %d", response.StatusCode)
	}
	return nil
}

func quickAddProbe(t *testing.T) string {
	t.Helper()
	probe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(probe.Close)
	return probe.URL
}

// startQuickAddXrayServer runs the panel's Xray runtime with the generated
// configuration and waits for the node's port.
func startQuickAddXrayServer(t *testing.T, control *LocalControlService, port int) {
	t.Helper()
	raw, err := control.ConfigService.GetXrayConfig()
	if err != nil {
		t.Fatal(err)
	}
	patched := allowLoopbackTargets(t, *raw)
	runtime := core.NewXrayRuntime()
	if err = runtime.Start(patched); err != nil {
		t.Fatalf("xray server: %v\n%s", err, patched)
	}
	t.Cleanup(func() { _ = runtime.Stop() })
	waitTCP(t, fmt.Sprintf("127.0.0.1:%d", port))
}

// allowLoopbackTargets lets the test's copy of the configuration reach the
// probe on 127.0.0.1: since v26.9.8 Xray-core's freedom outbound blackholes
// private and loopback targets of proxy inbounds by default, which the panel
// keeps for real servers.
func allowLoopbackTargets(t *testing.T, raw []byte) []byte {
	t.Helper()
	var config map[string]interface{}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	outbounds, _ := config["outbounds"].([]interface{})
	for _, item := range outbounds {
		outbound, _ := item.(map[string]interface{})
		if outbound["protocol"] != "freedom" {
			continue
		}
		settings, _ := outbound["settings"].(map[string]interface{})
		if settings == nil {
			settings = map[string]interface{}{}
		}
		settings["finalRules"] = []interface{}{map[string]interface{}{"action": "allow", "ip": []string{"127.0.0.0/8"}}}
		outbound["settings"] = settings
	}
	patched, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	return patched
}

// stubXrayBinary is enough for quick add, which only asks for the version.
func stubXrayBinary(t *testing.T) string {
	t.Helper()
	stub := filepath.Join(t.TempDir(), "xray")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho 'Xray 26.3.27 (Xray, Penetrates Everything.)'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	return stub
}

func setupXrayQuickAddTest(t *testing.T, binary string) *LocalControlService {
	t.Helper()
	t.Setenv("SUI_XRAY_PATH", binary)
	t.Setenv("SUI_XRAY_CONFIG", filepath.Join(t.TempDir(), "xray.json"))
	control := setupQuickAddTest(t)
	stubRealityProbe(t, "reality.test")
	return control
}

func TestQuickAddVlessVariantsCarryTrafficWithRealXray(t *testing.T) {
	binary := requireXrayBinary(t)
	for _, test := range []struct {
		variant string
		want    []string
	}{
		{VlessVariantRealityVision, []string{"security=reality", "flow=xtls-rprx-vision", "type=tcp", "pbk=", "sid=", "fp=chrome"}},
		{VlessVariantRealityXHTTP, []string{"security=reality", "type=xhttp", "mode=auto", "pbk="}},
		{VlessVariantRealityXHTTPVision, []string{
			"encryption=mlkem768x25519plus.native.0rtt.", "security=reality", "type=xhttp", "mode=auto",
			"flow=xtls-rprx-vision", "pbk=", "sid=", "fp=chrome",
		}},
		{VlessVariantEncVision, []string{"encryption=mlkem768x25519plus.native.0rtt.", "security=none", "flow=xtls-rprx-vision", "type=tcp"}},
		{VlessVariantEncXHTTP, []string{"encryption=mlkem768x25519plus.native.0rtt.", "security=none", "type=xhttp"}},
		{VlessVariantTLS, []string{"security=tls", "type=xhttp", "pcs="}},
	} {
		t.Run(test.variant, func(t *testing.T) {
			control := setupXrayQuickAddTest(t, binary)
			realityPort := localRealityTarget(t)
			port := freeLocalPort(t)
			response, err := control.QuickAddLocalInbounds(RemoteQuickAddRequest{
				CoreType: model.CoreTypeXray, Protocol: "vless", VlessVariant: test.variant,
				Count: 1, Port: port, PublicHost: "127.0.0.1",
			}, "admin")
			if err != nil {
				t.Fatal(err)
			}
			pointRealityAt(t, realityPort)
			link := quickAddClientLink(t, response.Created[0].ClientID)
			t.Log(link)
			passwall := checkVlessLinkForPassWall(t, link)
			for _, fragment := range test.want {
				if !strings.Contains(link, fragment) {
					t.Fatalf("%s link lacks %q: %s", test.variant, fragment, link)
				}
			}
			startQuickAddXrayServer(t, control, port)
			probe := quickAddProbe(t)
			socks := startXrayClient(t, binary, link, "")
			if err = fetchThroughSocks(socks, probe); err != nil {
				t.Fatalf("no traffic through the %s node: %v\nlink: %s", test.variant, err, link)
			}
			for name, node := range passwall {
				socks = startXrayClientOutbound(t, binary, passwallClientOutbound(t, binary, node))
				if err = fetchThroughSocks(socks, probe); err != nil {
					t.Fatalf("PassWall (%s import) has no traffic through the %s node: %v\nlink: %s", name, test.variant, err, link)
				}
			}
		})
	}
}

// TestQuickAddRealityOnSingBoxAcceptsClientsWithoutMLKEM covers the default
// VLESS node: sing-box's REALITY server must accept a current Xray client as
// well as a ClientHello without an X25519MLKEM768 key share (as sent by
// Shadowrocket and by Anywhere before iOS 26), which Xray-core 26.9.8+
// REALITY servers reject.
func TestQuickAddRealityOnSingBoxAcceptsClientsWithoutMLKEM(t *testing.T) {
	binary := requireXrayBinary(t)
	if !sdwanRealitySupported {
		t.Skip("sing-box REALITY needs the with_utls build tag")
	}
	control := setupXrayQuickAddTest(t, binary)
	realityPort := localRealityTarget(t)
	port := freeLocalPort(t)
	response, err := control.QuickAddLocalInbounds(RemoteQuickAddRequest{
		CoreType: model.CoreTypeSingBox, Protocol: "vless", VlessVariant: VlessVariantRealityVision,
		Count: 1, Port: port, PublicHost: "127.0.0.1",
	}, "admin")
	if err != nil {
		t.Fatal(err)
	}
	pointRealityAt(t, realityPort)
	config, err := control.ConfigService.GetConfigWithDB("", database.GetDB())
	if err != nil {
		t.Fatal(err)
	}
	instance := core.NewCore()
	if err = instance.Start(*config); err != nil {
		t.Fatalf("sing-box: %v\n%s", err, *config)
	}
	defer instance.Stop()
	waitTCP(t, fmt.Sprintf("127.0.0.1:%d", port))

	link := quickAddClientLink(t, response.Created[0].ClientID)
	passwall := checkVlessLinkForPassWall(t, link)
	probe := quickAddProbe(t)
	for _, fingerprint := range []string{"", "hellochrome_120"} {
		socks := startXrayClient(t, binary, link, fingerprint)
		if err = fetchThroughSocks(socks, probe); err != nil {
			t.Fatalf("fingerprint %q could not use the sing-box REALITY node: %v\nlink: %s", fingerprint, err, link)
		}
	}
	for name, node := range passwall {
		socks := startXrayClientOutbound(t, binary, passwallClientOutbound(t, binary, node))
		if err = fetchThroughSocks(socks, probe); err != nil {
			t.Fatalf("PassWall (%s import) could not use the sing-box REALITY node: %v\nlink: %s", name, err, link)
		}
	}
}

// TestQuickAddXHTTPVisionKeepsOtherNodesUnchanged checks both sides of the
// new Vision rule on the server: the REALITY + XHTTP + Vision node keeps its
// users' flow, while an XHTTP node with VLESS Encryption alone still drops it
// for users added later with the default Vision flow, as before.
func TestQuickAddXHTTPVisionKeepsOtherNodesUnchanged(t *testing.T) {
	control := setupXrayQuickAddTest(t, stubXrayBinary(t))
	flows := map[string]string{}
	for index, variant := range []string{VlessVariantRealityXHTTPVision, VlessVariantEncXHTTP} {
		response, err := control.QuickAddLocalInbounds(RemoteQuickAddRequest{
			CoreType: model.CoreTypeXray, Protocol: "vless", VlessVariant: variant,
			Count: 1, Port: 42300 + index, PublicHost: "198.51.100.60",
		}, "admin")
		if err != nil {
			t.Fatalf("%s: %v", variant, err)
		}
		var inbound model.Inbound
		if err = database.GetDB().Preload("Tls").Where("tag = ?", response.Created[0].Tag).First(&inbound).Error; err != nil {
			t.Fatal(err)
		}
		late := model.Client{
			Enable: true, Name: "late-" + variant,
			Config:   json.RawMessage(`{"vless":{"uuid":"22222222-2222-4222-8222-22222222222` + strconv.Itoa(index) + `","flow":"xtls-rprx-vision"}}`),
			Inbounds: json.RawMessage(`[` + strconv.Itoa(int(inbound.Id)) + `]`),
		}
		if err = database.GetDB().Create(&late).Error; err != nil {
			t.Fatal(err)
		}
		built, err := (&InboundService{}).buildXrayVlessInbound(database.GetDB(), &inbound)
		if err != nil {
			t.Fatal(err)
		}
		for _, client := range built["settings"].(map[string]interface{})["clients"].([]map[string]interface{}) {
			flow, _ := client["flow"].(string)
			flows[variant+"/"+client["email"].(string)[:5]] = flow
		}
	}
	for key, flow := range flows {
		wantVision := strings.HasPrefix(key, VlessVariantRealityXHTTPVision+"/")
		if (flow == vlessVisionFlow) != wantVision {
			t.Fatalf("%s flow = %q, want Vision %v (all: %v)", key, flow, wantVision, flows)
		}
	}
}
