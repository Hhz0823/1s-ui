package proxyprobe

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/logger"

	"github.com/op/go-logging"
)

// traceServer stands in for Cloudflare's trace page.
func traceServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/forbidden" {
			http.Error(w, "no", http.StatusForbidden)
			return
		}
		fmt.Fprint(w, "fl=1\nh=www.cloudflare.com\nip=203.0.113.9\nts=1\nloc=JP\n")
	}))
	t.Cleanup(server.Close)
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	oldRoots, oldControl := rootCAs, dialControl
	rootCAs, dialControl = pool, nil
	t.Cleanup(func() { rootCAs, dialControl = oldRoots, oldControl })
	return server
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// startSingBox runs the panel's own sing-box with SOCKS5 and HTTP proxies
// that require a login, as a user's proxy node would.
func startSingBox(t *testing.T) (socksPort, httpPort, openPort int) {
	t.Helper()
	logger.InitLogger(logging.CRITICAL)
	socksPort, httpPort, openPort = freePort(t), freePort(t), freePort(t)
	users := []map[string]string{{"username": "u", "password": "p@ss"}}
	config := map[string]interface{}{
		"log": map[string]interface{}{"level": "error"},
		"inbounds": []map[string]interface{}{
			{"type": "socks", "tag": "socks", "listen": "127.0.0.1", "listen_port": socksPort, "users": users},
			{"type": "http", "tag": "http", "listen": "127.0.0.1", "listen_port": httpPort, "users": users},
			{"type": "mixed", "tag": "open", "listen": "127.0.0.1", "listen_port": openPort},
		},
		"outbounds": []map[string]interface{}{{"type": "direct", "tag": "direct"}},
	}
	raw, _ := json.Marshal(config)
	box := core.NewCore()
	if err := box.Start(raw); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = box.Stop() })
	return
}

func TestProbeThroughRealSOCKS5AndHTTPProxies(t *testing.T) {
	target := traceServer(t)
	oldHandshake := handshakeTimeout
	handshakeTimeout = time.Second
	defer func() { handshakeTimeout = oldHandshake }()
	socksPort, httpPort, openPort := startSingBox(t)
	traceURL := target.URL + "/cdn-cgi/trace"

	for _, spec := range []Spec{
		{Type: "socks5", Host: "127.0.0.1", Port: socksPort, Username: "u", Password: "p@ss", Target: traceURL},
		{Type: "http", Host: "127.0.0.1", Port: httpPort, Username: "u", Password: "p@ss", Target: traceURL},
		{Type: "socks5", Host: "127.0.0.1", Port: openPort, Target: traceURL},
		{Type: "http", Host: "127.0.0.1", Port: openPort, Target: traceURL},
	} {
		result := Run(context.Background(), spec)
		if !result.OK {
			t.Fatalf("%s :%d: %+v", spec.Type, spec.Port, result)
		}
		if result.ExitIP != "203.0.113.9" || result.Country != "JP" || result.Status != 200 {
			t.Fatalf("%s: exit %q country %q status %d", spec.Type, result.ExitIP, result.Country, result.Status)
		}
		if result.LatencyMs < result.TTFBMs || result.LatencyMs <= 0 {
			t.Fatalf("%s: timings %+v", spec.Type, result)
		}
	}

	// Plain-HTTP pages go through an HTTP proxy with an absolute URL.
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			// What nginx and most web servers answer to a tunnel request.
			http.Error(w, "not allowed", http.StatusMethodNotAllowed)
			return
		}
		fmt.Fprint(w, "ok")
	}))
	defer plain.Close()
	for _, port := range []int{httpPort, socksPort} {
		kind := "http"
		if port == socksPort {
			kind = "socks5"
		}
		result := Run(context.Background(), Spec{Type: kind, Host: "127.0.0.1", Port: port, Username: "u", Password: "p@ss", Target: plain.URL + "/x"})
		if !result.OK || result.ExitIP != "" {
			t.Fatalf("%s plain page: %+v", kind, result)
		}
	}

	for name, check := range map[string]struct {
		spec  Spec
		stage string
		text  string
	}{
		"socks5 wrong password": {Spec{Type: "socks5", Host: "127.0.0.1", Port: socksPort, Username: "u", Password: "bad", Target: traceURL}, StageAuth, "rejected"},
		"socks5 no login":       {Spec{Type: "socks5", Host: "127.0.0.1", Port: socksPort, Target: traceURL}, StageAuth, "requires"},
		"http wrong password":   {Spec{Type: "http", Host: "127.0.0.1", Port: httpPort, Username: "u", Password: "bad", Target: traceURL}, StageAuth, "rejected"},
		"http no login":         {Spec{Type: "http", Host: "127.0.0.1", Port: httpPort, Target: traceURL}, StageAuth, "requires"},
		"nothing listening":     {Spec{Type: "socks5", Host: "127.0.0.1", Port: freePort(t), Target: traceURL}, StageConnect, "refused"},
		"target refuses":        {Spec{Type: "socks5", Host: "127.0.0.1", Port: openPort, Target: fmt.Sprintf("https://127.0.0.1:%d/", freePort(t))}, StageTunnel, ""},
		"target error page":     {Spec{Type: "socks5", Host: "127.0.0.1", Port: openPort, Target: target.URL + "/forbidden"}, StageHTTP, "403"},
		"http proxy as socks5":  {Spec{Type: "socks5", Host: "127.0.0.1", Port: httpPort, Target: traceURL}, StageTunnel, "is it an HTTP proxy"},
		"socks5 proxy as http":  {Spec{Type: "http", Host: "127.0.0.1", Port: socksPort, Target: plain.URL + "/x"}, StageTunnel, "is it a SOCKS5 proxy"},
		"web server as proxy":   {Spec{Type: "http", Host: "127.0.0.1", Port: portOf(t, plain.URL), Target: traceURL}, StageTunnel, "405"},
	} {
		started := time.Now()
		result := Run(context.Background(), check.spec)
		t.Logf("%s: %v %s %s", name, time.Since(started).Round(time.Millisecond), result.Stage, result.Error)
		if result.OK || result.Stage != check.stage || !strings.Contains(result.Error, check.text) {
			t.Fatalf("%s: %+v", name, result)
		}
		if strings.Contains(result.Error, "p@ss") || strings.Contains(result.Error, "bad") && name != "target error page" {
			t.Fatalf("%s: the error repeats the password: %q", name, result.Error)
		}
	}

	// An untrusted certificate on the test page is a TLS failure.
	rootCAs = nil
	result := Run(context.Background(), Spec{Type: "socks5", Host: "127.0.0.1", Port: openPort, Target: traceURL})
	if result.OK || result.Stage != StageTLS {
		t.Fatalf("untrusted target: %+v", result)
	}
}

func portOf(t *testing.T, rawURL string) int {
	t.Helper()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(rawURL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	var value int
	fmt.Sscan(port, &value)
	return value
}

func TestProbeTimesOutOnSilentProxy(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() { _, _ = io.Copy(io.Discard, conn) }()
		}
	}()
	old := dialControl
	dialControl = nil
	defer func() { dialControl = old }()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	started := time.Now()
	result := Run(ctx, Spec{Type: "socks5", Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port})
	if result.OK || !strings.Contains(result.Error, "timed out") || time.Since(started) > 3*time.Second {
		t.Fatalf("silent proxy: %+v after %v", result, time.Since(started))
	}
}

func TestProbeRefusesLocalAndMetadataAddresses(t *testing.T) {
	for _, address := range []string{"127.0.0.1:1080", "[::1]:1080", "169.254.169.254:80", "[fe80::1]:80", "0.0.0.0:80", "224.0.0.1:80", "[::ffff:127.0.0.1]:80"} {
		if err := allowedAddress(context.Background(), "tcp", address, nil); err == nil {
			t.Fatalf("%s was allowed", address)
		}
	}
	for _, address := range []string{"203.0.113.5:1080", "10.0.0.8:1080", "[2001:db8::1]:443"} {
		if err := allowedAddress(context.Background(), "tcp", address, nil); err != nil {
			t.Fatalf("%s: %v", address, err)
		}
	}
	result := Run(context.Background(), Spec{Type: "socks5", Host: "127.0.0.1", Port: 1080})
	if result.OK || result.Stage != StageConnect || !strings.Contains(result.Error, "not allowed") {
		t.Fatalf("loopback proxy: %+v", result)
	}
}

func TestNormalizeSpec(t *testing.T) {
	spec := Spec{Type: " SOCKS ", Host: " [2001:db8::1] ", Port: 1080}
	if err := Normalize(&spec); err != nil || spec.Type != TypeSOCKS5 || spec.Host != "2001:db8::1" || spec.Target != DefaultTarget {
		t.Fatalf("%+v %v", spec, err)
	}
	for _, bad := range []Spec{
		{Type: "vmess", Host: "a", Port: 1},
		{Type: "http", Host: "", Port: 1},
		{Type: "http", Host: "a b", Port: 1},
		{Type: "http", Host: "a", Port: 70000},
		{Type: "http", Host: "a", Port: 80, Target: "ftp://x/"},
		{Type: "http", Host: "a", Port: 80, Target: "https://user:pw@x/"},
		{Type: "http", Host: "a", Port: 80, Username: strings.Repeat("u", 256)},
	} {
		if err := Normalize(&bad); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
	if ip, country := parseTrace("ip=2001:db8::5\nloc=us\nwarp=off"); ip != "2001:db8::5" || country != "US" {
		t.Fatalf("trace: %q %q", ip, country)
	}
}

func TestParseLink(t *testing.T) {
	for text, want := range map[string]struct {
		spec Spec
		name string
	}{
		"socks5://u:p%40ss@1.2.3.4:1080#Tokyo%20A":          {Spec{Type: TypeSOCKS5, Host: "1.2.3.4", Port: 1080, Username: "u", Password: "p@ss"}, "Tokyo A"},
		"socks://dXNlcjpwYXNz@proxy.example.com:443#v2rayN": {Spec{Type: TypeSOCKS5, Host: "proxy.example.com", Port: 443, Username: "user", Password: "pass"}, "v2rayN"},
		"http://u:p@[2001:db8::2]:8080":                     {Spec{Type: TypeHTTP, Host: "2001:db8::2", Port: 8080, Username: "u", Password: "p"}, ""},
		"socks5h://h.example:1080":                          {Spec{Type: TypeSOCKS5, Host: "h.example", Port: 1080}, ""},
		"5.6.7.8:3128:alice:s3cret":                         {Spec{Type: TypeSOCKS5, Host: "5.6.7.8", Port: 3128, Username: "alice", Password: "s3cret"}, ""},
		"alice:s3cret@5.6.7.8:3128":                         {Spec{Type: TypeSOCKS5, Host: "5.6.7.8", Port: 3128, Username: "alice", Password: "s3cret"}, ""},
		" 5.6.7.8:1080 ":                                    {Spec{Type: TypeSOCKS5, Host: "5.6.7.8", Port: 1080}, ""},
	} {
		spec, name, err := ParseLink(text)
		want.spec.Target = DefaultTarget
		if err != nil || spec != want.spec || name != want.name {
			t.Fatalf("%q: %+v %q %v", text, spec, name, err)
		}
	}
	for _, bad := range []string{"", "vmess://abc", "https://u:p@h:443", "socks5://h", "1.2.3.4", "a:b:c", "h:99999"} {
		if _, _, err := ParseLink(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
