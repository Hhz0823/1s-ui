package service

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"

	"github.com/op/go-logging"
	sb "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

// recordingProxy is a SOCKS5 "node" that stands in for the internet: it
// remembers every destination and answers each request itself.
type recordingProxy struct {
	listener net.Listener
	mu       sync.Mutex
	targets  []string
}

func startRecordingProxy(t *testing.T, host string) *recordingProxy {
	t.Helper()
	listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		t.Fatal(err)
	}
	proxy := &recordingProxy{listener: listener}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go proxy.serve(conn)
		}
	}()
	return proxy
}

func (p *recordingProxy) port() int { return p.listener.Addr().(*net.TCPAddr).Port }

func (p *recordingProxy) seen() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.targets...)
}

func (p *recordingProxy) serve(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(conn)
	head := make([]byte, 2)
	if _, err := io.ReadFull(reader, head); err != nil || head[0] != 5 {
		return
	}
	if _, err := io.CopyN(io.Discard, reader, int64(head[1])); err != nil {
		return
	}
	_, _ = conn.Write([]byte{5, 0})
	request := make([]byte, 4)
	if _, err := io.ReadFull(reader, request); err != nil || request[1] != 1 {
		return
	}
	var host string
	switch request[3] {
	case 1:
		ip := make([]byte, 4)
		io.ReadFull(reader, ip)
		host = net.IP(ip).String()
	case 3:
		size, _ := reader.ReadByte()
		name := make([]byte, size)
		io.ReadFull(reader, name)
		host = string(name)
	case 4:
		ip := make([]byte, 16)
		io.ReadFull(reader, ip)
		host = net.IP(ip).String()
	}
	portBytes := make([]byte, 2)
	io.ReadFull(reader, portBytes)
	p.mu.Lock()
	p.targets = append(p.targets, net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(portBytes)))))
	p.mu.Unlock()
	_, _ = conn.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
	if _, err := http.ReadRequest(reader); err != nil {
		return
	}
	body := "via-node"
	fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
}

// writeRuleSet writes a sing-box binary rule set for the tests.
func writeRuleSet(t *testing.T, tag string, rule map[string]interface{}) {
	t.Helper()
	raw, _ := json.Marshal(map[string]interface{}{"version": C.RuleSetVersion3, "rules": []interface{}{rule}})
	var compat option.PlainRuleSetCompat
	if err := compat.UnmarshalJSON(raw); err != nil {
		t.Fatal(err)
	}
	plain, err := compat.Upgrade()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(proxyClientRuleSetDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(proxyClientRuleSetPath(tag))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := srs.Write(file, plain, C.RuleSetVersion3); err != nil {
		t.Fatal(err)
	}
}

func initProxyClientDB(t *testing.T) {
	t.Helper()
	logger.InitLogger(logging.CRITICAL)
	dir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dir)
	t.Setenv("SUI_AGENT_ENV_FILE", filepath.Join(dir, "agent.env"))
	if err := database.InitDB(filepath.Join(dir, "client.db")); err != nil {
		t.Fatal(err)
	}
	oldDir := proxyClientRuleSetDir
	proxyClientRuleSetDir = func() string { return filepath.Join(dir, "rulesets") }
	t.Cleanup(func() { proxyClientRuleSetDir = oldDir })
}

// fetchThrough requests target through the SOCKS5 proxy on port.
func fetchThrough(t *testing.T, port int, target string) (string, error) {
	t.Helper()
	proxyURL, _ := url.Parse(fmt.Sprintf("socks5h://127.0.0.1:%d", port))
	client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	defer client.CloseIdleConnections()
	response, err := client.Get(target)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	return string(body), err
}

func buildClientConfig(t *testing.T) []byte {
	t.Helper()
	raw, err := (&ConfigService{}).GetConfigWithDB("", database.GetDB())
	if err != nil {
		t.Fatal(err)
	}
	return *raw
}

func TestProxyClientRoutesThroughTheSelectedNode(t *testing.T) {
	initProxyClientDB(t)
	host := hostAddress(t)
	node := startRecordingProxy(t, host)
	_, _, trace := startTestProxies(t, host)
	traceURL, _ := url.Parse(trace)

	client := &ProxyClientService{}
	state, err := client.AddNodes(fmt.Sprintf("socks5://%s:%d#Home%%20node\nnot a link\nvless://broken", host, node.port()))
	if err != nil || len(state.Nodes) != 1 || state.Nodes[0].Name != "Home node" || state.Nodes[0].Protocol != "socks" {
		t.Fatalf("add nodes: %+v %v", state, err)
	}
	nodeID := state.Nodes[0].Id
	mixedPort := freeTCPPort(t)
	settings := defaultProxyClientSettings()
	settings.Enabled, settings.Mode, settings.NodeId = true, ProxyClientModeGlobal, nodeID
	settings.MixedListen, settings.MixedPort, settings.DNSPort = ProxyClientListenLocal, mixedPort, freeTCPPort(t)
	if err := normalizeProxyClientSettings(&settings); err != nil {
		t.Fatal(err)
	}
	if err := saveProxyClientSettings(database.GetDB(), settings); err != nil {
		t.Fatal(err)
	}

	start := func() func() {
		box := core.NewCore()
		if err := box.Start(buildClientConfig(t)); err != nil {
			t.Fatal(err)
		}
		return func() { _ = box.Stop() }
	}

	// Global: everything public goes through the node.
	stop := start()
	body, err := fetchThrough(t, mixedPort, "http://example.test/page")
	if err != nil || body != "via-node" {
		t.Fatalf("global domain: %q %v", body, err)
	}
	body, err = fetchThrough(t, mixedPort, trace)
	if err != nil || body != "via-node" {
		t.Fatalf("global IP: %q %v", body, err)
	}
	if seen := node.seen(); len(seen) != 2 || seen[0] != "example.test:80" || seen[1] != traceURL.Host {
		t.Fatalf("node saw %v", seen)
	}
	stop()

	// Rule mode: addresses in the China set go direct, the rest through
	// the node; custom domains follow their lists.
	writeRuleSet(t, ruleSetGeoIPCN, map[string]interface{}{"ip_cidr": []string{traceURL.Hostname() + "/32"}})
	writeRuleSet(t, ruleSetGeositeCN, map[string]interface{}{"domain_suffix": []string{"cn.test"}})
	settings.Mode = ProxyClientModeRule
	settings.ProxyDomains = []string{"cn-but-proxied.cn.test"}
	saveProxyClientSettings(database.GetDB(), settings)
	stop = start()
	defer stop()
	body, err = fetchThrough(t, mixedPort, trace)
	if err != nil || !strings.Contains(body, "ip=198.51.100.77") {
		t.Fatalf("rule mode China address: %q %v", body, err)
	}
	body, err = fetchThrough(t, mixedPort, "http://abroad.test/")
	if err != nil || body != "via-node" {
		t.Fatalf("rule mode abroad: %q %v", body, err)
	}
	body, err = fetchThrough(t, mixedPort, "http://cn-but-proxied.cn.test/")
	if err != nil || body != "via-node" {
		t.Fatalf("custom proxy domain: %q %v", body, err)
	}
	seen := node.seen()
	if len(seen) != 4 || seen[2] != "abroad.test:80" || seen[3] != "cn-but-proxied.cn.test:80" {
		t.Fatalf("node saw %v", seen)
	}

	// A latency test measures the node through its own outbound.
	state, err = client.TestNodes(nil)
	if err != nil || state.Nodes[0].TcpMs <= 0 {
		t.Fatalf("test: %+v %v", state.Nodes, err)
	}
}

func TestProxyClientConfigIsValidSingBox(t *testing.T) {
	initProxyClientDB(t)
	oldPlatform := proxyClientPlatform
	proxyClientPlatform = func() ProxyClientPlatform {
		return ProxyClientPlatform{OS: "linux", Root: true, TunDevice: true, Nftables: true, OpenWrt: true, Dnsmasq: true}
	}
	t.Cleanup(func() { proxyClientPlatform = oldPlatform })
	writeRuleSet(t, ruleSetGeoIPCN, map[string]interface{}{"ip_cidr": []string{"1.0.1.0/24"}})
	writeRuleSet(t, ruleSetGeositeCN, map[string]interface{}{"domain_suffix": []string{"cn"}})
	writeRuleSet(t, ruleSetGeositeGFW, map[string]interface{}{"domain_suffix": []string{"google.com"}})
	os.WriteFile(os.Getenv("SUI_AGENT_ENV_FILE"), []byte("SUI_AGENT_PANEL=https://panel.example.com:2095/app/\n"), 0o600)

	client := &ProxyClientService{}
	links := strings.Join([]string{
		"vless://" + testNodeUUID + "@a.example.com:443?type=ws&security=tls&sni=a.example.com&path=%2Fws&flow=#Tokyo",
		"hy2://secret@b.example.com:8443?sni=b.example.com#Osaka",
		"ss://" + "YWVzLTI1Ni1nY206cGFzcw" + "@c.example.com:8388#HK",
	}, "\n")
	if _, err := client.AddNodes(links); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{ProxyClientModeRule, ProxyClientModeGFW, ProxyClientModeGlobal} {
		settings := defaultProxyClientSettings()
		settings.Enabled, settings.Mode, settings.Auto, settings.Tun, settings.BlockQUIC = true, mode, true, true, true
		settings.BypassIPs, settings.ProxyIPs = []string{"192.168.1.10"}, []string{"192.168.1.0/24"}
		settings.DirectDomains, settings.ProxyDomains = []string{"example.cn"}, []string{"github.com"}
		settings.RemoteDNS, settings.MixedUsername, settings.MixedPassword = "tls://dns.google", "u", "p"
		if err := normalizeProxyClientSettings(&settings); err != nil {
			t.Fatal(err)
		}
		saveProxyClientSettings(database.GetDB(), settings)
		raw := buildClientConfig(t)
		var config map[string]interface{}
		json.Unmarshal(raw, &config)
		inbounds, _ := json.Marshal(config["inbounds"])
		if !strings.Contains(string(inbounds), `"auto_redirect":true`) || !strings.Contains(string(inbounds), proxyClientMixedTag) {
			t.Fatalf("%s inbounds: %s", mode, inbounds)
		}
		if mode == ProxyClientModeRule && !strings.Contains(string(inbounds), `"route_exclude_address_set":["client-geoip-cn"]`) {
			t.Fatalf("rule mode keeps China addresses out of the TUN: %s", inbounds)
		}
		route, _ := json.Marshal(config["route"])
		if !strings.Contains(string(route), `"panel.example.com"`) || !strings.Contains(string(route), `"auto_detect_interface":true`) {
			t.Fatalf("%s route: %s", mode, route)
		}
		// sing-box accepts it as a whole; building the box (without starting
		// it, which would take over this machine's routes) checks every
		// option, rule and reference.
		ctx := sb.Context(context.Background(), core.InboundRegistry(), core.OutboundRegistry(), core.EndpointRegistry(), core.DNSTransportRegistry(), core.ServiceRegistry())
		var options option.Options
		if err := options.UnmarshalJSONContext(ctx, raw); err != nil {
			t.Fatalf("%s: %v\n%s", mode, err, raw)
		}
		instance, err := sb.New(sb.Options{Context: ctx, Options: options})
		if err != nil {
			t.Fatalf("%s: %v\n%s", mode, err, raw)
		}
		_ = instance.Close()
	}

	// No node, no client: the panel's config stays as it was.
	database.GetDB().Where("1 = 1").Delete(&model.ProxyClientNode{})
	raw := buildClientConfig(t)
	if strings.Contains(string(raw), proxyClientProxyTag) {
		t.Fatal("client added without nodes")
	}
}

func TestProxyClientSubscriptions(t *testing.T) {
	initProxyClientDB(t)
	links := "vless://" + testNodeUUID + "@a.example.com:443?type=tcp&security=tls&sni=a.example.com#A\n" +
		"trojan://pw@b.example.com:443?sni=b.example.com#B\n" +
		"vmess://" + "eyJ2IjoiMiIsInBzIjoi5Ymp5L2Z5rWB6YePOjEwMEdCIiwiYWRkIjoiMS4xLjEuMSIsInBvcnQiOiI0NDMiLCJpZCI6IjlmMWMzYjVlLTZhMmQtNGM4ZS05YjdhLTFkMmUzZjRhNWI2YyJ9"
	var served string
	var agent string
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent = r.UserAgent()
		w.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=1000; expire=1893456000")
		fmt.Fprint(w, served)
	})}
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	subscriptionURL := "http://" + listener.Addr().String() + "/sub?token=secret"

	served = base64Std(links)
	client := &ProxyClientService{}
	state, err := client.SaveSubscription(ProxyClientSubscriptionInput{Name: "Airport", URL: subscriptionURL, AutoUpdate: 12})
	if err != nil || len(state.Subscriptions) != 1 || state.Subscriptions[0].NodeCount != 2 || state.Subscriptions[0].Total != 1000 {
		t.Fatalf("subscription: %+v %v", state, err)
	}
	if agent != proxyClientDefaultUserAgent {
		t.Fatalf("user agent %q", agent)
	}
	// The traffic notice "剩余流量" is not a node.
	if len(state.Nodes) != 2 || state.Nodes[0].Name != "A" || state.Nodes[1].Name != "B" {
		t.Fatalf("nodes: %+v", state.Nodes)
	}
	keep := state.Nodes[0].Id
	if _, err := client.Select(keep); err != nil {
		t.Fatal(err)
	}

	// A refresh keeps the node ids that did not change.
	served = "vless://" + testNodeUUID + "@a.example.com:443?type=tcp&security=tls&sni=a.example.com#A\nhy2://pw@c.example.com:443#C"
	state, err = client.UpdateSubscriptions(0)
	if err != nil || len(state.Nodes) != 2 || state.Nodes[0].Id != keep || state.Nodes[1].Name != "C" || state.Settings.NodeId != keep {
		t.Fatalf("refresh: %+v %v", state, err)
	}

	// sing-box JSON subscriptions work too; Clash ones are explained.
	served = `{"outbounds":[{"type":"selector","tag":"proxy","outbounds":["J"]},{"type":"trojan","tag":"J","server":"j.example.com","server_port":443,"password":"x","tls":{"enabled":true}},{"type":"direct","tag":"direct"}]}`
	state, err = client.UpdateSubscriptions(state.Subscriptions[0].Id)
	if err != nil || len(state.Nodes) != 1 || state.Nodes[0].Name != "J" || state.Nodes[0].Protocol != "trojan" {
		t.Fatalf("sing-box subscription: %+v %v", state, err)
	}
	served = "proxies:\n  - name: x\n"
	if _, err := client.UpdateSubscriptions(state.Subscriptions[0].Id); err == nil || !strings.Contains(err.Error(), "Clash") {
		t.Fatalf("clash: %v", err)
	}
	state, _ = client.State()
	if state.Subscriptions[0].LastError == "" || len(state.Nodes) != 1 {
		t.Fatalf("a failed refresh keeps the nodes: %+v", state)
	}

	// The app never sees the subscription token or the proxy password.
	settings := loadProxyClientSettings(database.GetDB())
	settings.MixedPassword = "hunter2"
	saveProxyClientSettings(database.GetDB(), settings)
	raw, err := client.CallProxyClient(0, ProxyClientCall{Action: "state"})
	if err != nil {
		t.Fatal(err)
	}
	redacted, err := RedactProxyClientResult(raw)
	if err != nil || strings.Contains(string(redacted), "secret") || strings.Contains(string(redacted), "hunter2") || !strings.Contains(string(raw), "token=secret") {
		t.Fatalf("redaction: %s %v", redacted, err)
	}

	if state, err := client.DeleteSubscription(state.Subscriptions[0].Id); err != nil || len(state.Nodes) != 0 || len(state.Subscriptions) != 0 {
		t.Fatalf("delete: %+v %v", state, err)
	}
}

func TestProxyClientSettingsValidation(t *testing.T) {
	for _, bad := range []ProxyClientSettings{
		{Mode: "chnroute"},
		{Mixed: true, MixedListen: ProxyClientListenAll},
		{MixedPort: 70000},
		{Mixed: true, MixedPort: 5335, DNSPort: 5335},
		{DirectDNS: "dns.alidns.com"},
		{RemoteDNS: "ftp://1.1.1.1"},
		{BypassIPs: []string{"router"}},
		{ProxyDomains: []string{"not a domain"}},
		{TestURL: "ftp://x"},
	} {
		if err := normalizeProxyClientSettings(&bad); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
	settings := ProxyClientSettings{BypassIPs: []string{" 192.168.1.5 ", "10.0.0.0/8", "fd00::1"}, DirectDomains: []string{"*.Example.CN", ".qq.com"}}
	if err := normalizeProxyClientSettings(&settings); err != nil {
		t.Fatal(err)
	}
	if strings.Join(settings.BypassIPs, ",") != "192.168.1.5/32,10.0.0.0/8,fd00::1/128" || strings.Join(settings.DirectDomains, ",") != "example.cn,qq.com" {
		t.Fatalf("normalized: %+v", settings)
	}
	server, err := proxyClientDNSServer("https://dns.google/resolve", proxyClientDNSRemoteTag, proxyClientProxyTag)
	if err != nil || server["type"] != "https" || server["server"] != "dns.google" || server["path"] != "/resolve" || server["domain_resolver"] != proxyClientDNSDirectTag {
		t.Fatalf("doh: %v %v", server, err)
	}
	server, err = proxyClientDNSServer("[2400:3200::1]:53", proxyClientDNSDirectTag, "")
	if err != nil || server["type"] != "udp" || server["server"] != "2400:3200::1" || server["server_port"] != 53 {
		t.Fatalf("udp v6: %v %v", server, err)
	}
	if info := parseSubscriptionUserinfo("upload=1; download=2; total=3; expire=4; bogus=x"); len(info) != 4 || info["total"] != int64(3) {
		t.Fatalf("userinfo: %v", info)
	}
}

func base64Std(value string) string {
	return base64.StdEncoding.EncodeToString([]byte(value))
}

func TestProxyClientDnsmasqHandOff(t *testing.T) {
	dirs := []string{t.TempDir(), t.TempDir()}
	restarts := 0
	oldDirs, oldRestart := proxyClientDnsmasqDirs, proxyClientRestartDnsmasq
	proxyClientDnsmasqDirs = func() []string { return dirs }
	proxyClientRestartDnsmasq = func() error { restarts++; return nil }
	t.Cleanup(func() { proxyClientDnsmasqDirs, proxyClientRestartDnsmasq = oldDirs, oldRestart })

	writeProxyClientDnsmasq(true, 5335)
	for _, dir := range dirs {
		content, err := os.ReadFile(filepath.Join(dir, proxyClientDnsmasqFile))
		if err != nil || !strings.Contains(string(content), "server=127.0.0.1#5335") || !strings.Contains(string(content), "no-resolv") {
			t.Fatalf("%s: %q %v", dir, content, err)
		}
	}
	if !proxyClientDNSHijacked() || restarts != 1 {
		t.Fatalf("hijacked=%v restarts=%d", proxyClientDNSHijacked(), restarts)
	}
	// Nothing changed: dnsmasq is left alone.
	writeProxyClientDnsmasq(true, 5335)
	if restarts != 1 {
		t.Fatalf("restarted dnsmasq without a change")
	}
	writeProxyClientDnsmasq(true, 5353)
	if restarts != 2 {
		t.Fatal("a new port did not reach dnsmasq")
	}
	// When sing-box no longer serves DNS, dnsmasq goes back to normal.
	writeProxyClientDnsmasq(false, 5353)
	if proxyClientDNSHijacked() || restarts != 3 {
		t.Fatalf("hijacked=%v restarts=%d", proxyClientDNSHijacked(), restarts)
	}
}
