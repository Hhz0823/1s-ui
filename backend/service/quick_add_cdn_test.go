//go:build !openwrt_lite

package service

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util"
)

// fakeCDN stands in for Cloudflare in front of this server: TLS and HTTP/2
// for cdn.test at the edge, HTTP/1.1 over TLS to the origin port without
// checking its certificate ("Full" SSL mode), and CF-Connecting-IP.
type fakeCDN struct {
	edge      *httptest.Server
	requests  atomic.Int64
	downloads atomic.Int64
	status    atomic.Int64 // answer this status instead of proxying
	leafPin   string
}

const fakeCDNDomain = "cdn.test"

func newFakeCDN(t *testing.T, originPort int) *fakeCDN {
	t.Helper()
	cdn := &fakeCDN{}
	origin, _ := url.Parse(fmt.Sprintf("https://127.0.0.1:%d", originPort))
	proxy := httputil.NewSingleHostReverseProxy(origin)
	proxy.FlushInterval = -1
	proxy.Transport = &http.Transport{TLSClientConfig: &tls.Config{ServerName: fakeCDNDomain, InsecureSkipVerify: true}}
	cdn.edge = httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		cdn.requests.Add(1)
		if status := cdn.status.Load(); status != 0 {
			writer.WriteHeader(int(status))
			return
		}
		if request.Method == http.MethodGet && !strings.HasPrefix(request.URL.Path, cdnProbePath) {
			cdn.downloads.Add(1)
		}
		request.Header.Set("CF-Connecting-IP", "198.51.100.7")
		proxy.ServeHTTP(writer, request)
	}))
	now := time.Now()
	key, certificate, err := util.GenerateSelfSignedTLS(fakeCDNDomain, now, now.AddDate(0, 3, 0))
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.X509KeyPair(certificate, key)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(certificate)
	leaf, _ := x509.ParseCertificate(block.Bytes)
	sum := sha256.Sum256(leaf.Raw)
	cdn.leafPin = hex.EncodeToString(sum[:])
	cdn.edge.EnableHTTP2 = true
	cdn.edge.TLS = &tls.Config{Certificates: []tls.Certificate{pair}}
	cdn.edge.StartTLS()
	t.Cleanup(cdn.edge.Close)

	// The panel's probe reaches cdn.test at the edge and trusts its certificate.
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	edgeAddress := cdn.edge.Listener.Addr().String()
	oldDial, oldRoots := cdnProbeDial, cdnProbeRoots
	cdnProbeDial = func(ctx context.Context, network, address string) (net.Conn, error) {
		if host, _, _ := net.SplitHostPort(address); host == fakeCDNDomain {
			address = edgeAddress
		}
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}
	cdnProbeRoots = func() *tls.Config { return &tls.Config{RootCAs: roots} }
	t.Cleanup(func() { cdnProbeDial, cdnProbeRoots = oldDial, oldRoots })
	return cdn
}

// throughEdge points a link's download side at the fake edge by address, as
// clients do with a preferred CDN IP, and trusts the edge's test certificate.
func (cdn *fakeCDN) throughEdge(t *testing.T, outbound map[string]interface{}) {
	t.Helper()
	stream := outbound["streamSettings"].(map[string]interface{})
	xhttp := stream["xhttpSettings"].(map[string]interface{})
	extra, _ := xhttp["extra"].(map[string]interface{})
	download, _ := extra["downloadSettings"].(map[string]interface{})
	if download == nil {
		t.Fatalf("no downloadSettings in %v", xhttp)
	}
	host, port, _ := net.SplitHostPort(cdn.edge.Listener.Addr().String())
	download["address"] = host
	var edgePort int
	fmt.Sscanf(port, "%d", &edgePort)
	download["port"] = edgePort
	tlsSettings := download["tlsSettings"].(map[string]interface{})
	tlsSettings["pinnedPeerCertSha256"] = cdn.leafPin
}

func TestQuickAddXHTTPDownloadsThroughCDN(t *testing.T) {
	binary := requireXrayBinary(t)
	for _, variant := range []string{VlessVariantRealityXHTTP, VlessVariantRealityXHTTPVision, VlessVariantEncXHTTP} {
		t.Run(variant, func(t *testing.T) {
			control := setupXrayQuickAddTest(t, binary)
			realityPort := localRealityTarget(t)
			port, cdnPort := freeLocalPort(t), freeLocalPort(t)
			cdn := newFakeCDN(t, cdnPort)
			response, err := control.QuickAddLocalInbounds(RemoteQuickAddRequest{
				CoreType: model.CoreTypeXray, Protocol: "vless", VlessVariant: variant,
				Count: 1, Port: port, PublicHost: "127.0.0.1", CDNDomain: fakeCDNDomain, CDNPort: cdnPort,
			}, "admin")
			if err != nil {
				t.Fatal(err)
			}
			if cdn.requests.Load() == 0 {
				t.Fatal("quick add did not check the domain through the CDN")
			}
			pointRealityAt(t, realityPort)
			link := quickAddClientLink(t, response.Created[0].ClientID)
			parsed, _ := url.Parse(link)
			var extra struct {
				DownloadSettings struct {
					Address       string `json:"address"`
					Port          int    `json:"port"`
					Security      string `json:"security"`
					XHTTPSettings struct {
						Path string `json:"path"`
						Host string `json:"host"`
					} `json:"xhttpSettings"`
				} `json:"downloadSettings"`
			}
			if err = json.Unmarshal([]byte(parsed.Query().Get("extra")), &extra); err != nil {
				t.Fatalf("link has no valid extra: %s", link)
			}
			download := extra.DownloadSettings
			if download.Address != fakeCDNDomain || download.Port != cdnPort || download.Security != "tls" ||
				download.XHTTPSettings.Path != parsed.Query().Get("path") || download.XHTTPSettings.Host != fakeCDNDomain {
				t.Fatalf("unexpected downloadSettings %+v in %s", download, link)
			}
			if parsed.Hostname() != "127.0.0.1" {
				t.Fatalf("uploads must go to the server itself: %s", link)
			}

			startQuickAddXrayServer(t, control, port)
			waitTCP(t, fmt.Sprintf("127.0.0.1:%d", cdnPort))
			probe := quickAddProbe(t)
			outbound := xrayClientOutbound(t, link, "")
			cdn.throughEdge(t, outbound)
			socks := startXrayClientOutbound(t, binary, outbound)
			for attempt := 0; attempt < 3; attempt++ {
				if err = fetchThroughSocks(socks, probe); err != nil {
					t.Fatalf("no traffic through the %s node with a CDN downlink: %v\nlink: %s", variant, err, link)
				}
			}
			if cdn.downloads.Load() == 0 {
				t.Fatal("downloads did not go through the CDN")
			}

			// A client that ignores extra keeps both directions on the node's
			// own entry.
			direct := xrayClientOutbound(t, link, "")
			delete(direct["streamSettings"].(map[string]interface{})["xhttpSettings"].(map[string]interface{}), "extra")
			before := cdn.downloads.Load()
			socks = startXrayClientOutbound(t, binary, direct)
			if err = fetchThroughSocks(socks, probe); err != nil {
				t.Fatalf("a client without downloadSettings has no traffic: %v", err)
			}
			if cdn.downloads.Load() != before {
				t.Fatal("a client without downloadSettings went through the CDN")
			}
		})
	}
}

func TestQuickAddCDNRefusesDomainsThatMissThisServer(t *testing.T) {
	control := setupXrayQuickAddTest(t, stubXrayBinary(t))
	port, cdnPort := freeLocalPort(t), freeLocalPort(t)
	cdn := newFakeCDN(t, cdnPort)
	cdn.status.Store(522)
	_, err := control.QuickAddLocalInbounds(RemoteQuickAddRequest{
		CoreType: model.CoreTypeXray, Protocol: "vless", VlessVariant: VlessVariantEncXHTTP,
		Count: 1, Port: port, PublicHost: "127.0.0.1", CDNDomain: fakeCDNDomain, CDNPort: cdnPort,
	}, "admin")
	if err == nil || !strings.Contains(err.Error(), "cannot connect") {
		t.Fatalf("a CDN that cannot reach the origin must be reported, got %v", err)
	}
	list, listErr := control.ListInbounds()
	if listErr != nil || len(list.Inbounds) != 0 {
		t.Fatalf("a failed CDN check must not create nodes: %v %v", list, listErr)
	}
}

func TestQuickAddCDNValidation(t *testing.T) {
	for _, test := range []struct {
		variant, domain string
		port            int
		ok              bool
	}{
		{VlessVariantRealityXHTTP, "cdn.example.com", 0, true},
		{VlessVariantEncXHTTP, " CDN.Example.com. ", 8443, true},
		{VlessVariantRealityVision, "cdn.example.com", 0, false},
		{VlessVariantTLS, "cdn.example.com", 0, false},
		{VlessVariantRealityXHTTP, "1.2.3.4", 0, false},
		{VlessVariantRealityXHTTP, "https://cdn.example.com/", 0, false},
		{VlessVariantRealityXHTTP, "cdn.example.com", 443, false},
	} {
		request := RemoteQuickAddRequest{
			CoreType: model.CoreTypeXray, Protocol: "vless", Count: 1, Port: 443,
			VlessVariant: test.variant, CDNDomain: test.domain, CDNPort: test.port,
		}
		err := validateRemoteQuickAddRequest(&request)
		if (err == nil) != test.ok {
			t.Fatalf("%s %q %d: err = %v", test.variant, test.domain, test.port, err)
		}
		if err == nil && request.CDNDomain != "cdn.example.com" {
			t.Fatalf("domain not normalized: %q", request.CDNDomain)
		}
	}
	request := RemoteQuickAddRequest{CoreType: model.CoreTypeSingBox, Protocol: "trojan", Count: 1, Port: 443, CDNDomain: "cdn.example.com"}
	if err := validateRemoteQuickAddRequest(&request); err == nil {
		t.Fatal("a CDN downlink was accepted for Trojan")
	}
}

func TestHostBehindCloudflare(t *testing.T) {
	for host, want := range map[string]bool{
		"104.16.1.1": true, "[2606:4700::1111]": true, "172.67.1.1": true,
		"198.51.100.7": false, "127.0.0.1": false, "8.8.8.8": false,
	} {
		if got := hostBehindCloudflare(host); got != want {
			t.Fatalf("%s: %v", host, got)
		}
	}
}

// TestXHTTPCDNNodeConfiguration checks the Xray configuration of CDN nodes
// without an Xray binary: the entries hand everything to one XHTTP inbound.
func TestXHTTPCDNNodeConfiguration(t *testing.T) {
	for _, variant := range []string{VlessVariantRealityXHTTP, VlessVariantEncXHTTP} {
		t.Run(variant, func(t *testing.T) {
			control := setupXrayQuickAddTest(t, stubXrayBinary(t))
			port, cdnPort := freeLocalPort(t), freeLocalPort(t)
			newFakeCDN(t, cdnPort)
			response, err := control.QuickAddLocalInbounds(RemoteQuickAddRequest{
				CoreType: model.CoreTypeXray, Protocol: "vless", VlessVariant: variant,
				Count: 1, Port: port, PublicHost: "127.0.0.1", CDNDomain: fakeCDNDomain, CDNPort: cdnPort,
			}, "admin")
			if err != nil {
				t.Fatal(err)
			}
			raw, err := control.ConfigService.GetXrayConfig()
			if err != nil {
				t.Fatal(err)
			}
			var config struct {
				Inbounds []map[string]interface{} `json:"inbounds"`
			}
			if err = json.Unmarshal(*raw, &config); err != nil {
				t.Fatal(err)
			}
			byTag := map[string]map[string]interface{}{}
			for _, inbound := range config.Inbounds {
				byTag[inbound["tag"].(string)] = inbound
			}
			tag := response.Created[0].Tag
			node, cdn := byTag[tag], byTag[tag+"-cdn"]
			if node == nil || cdn == nil {
				t.Fatalf("missing inbounds in %v", byTag)
			}
			nodeStream := node["streamSettings"].(map[string]interface{})
			if nodeStream["security"] != nil || nodeStream["network"] != "xhttp" {
				t.Fatalf("the XHTTP inbound must not carry a security layer: %v", nodeStream)
			}
			if host := nodeStream["xhttpSettings"].(map[string]interface{})["host"]; host != nil {
				t.Fatalf("the XHTTP inbound must accept the CDN's Host header, has %v", host)
			}
			if clients := node["settings"].(map[string]interface{})["clients"].([]interface{}); len(clients) != 1 {
				t.Fatalf("the XHTTP inbound carries the users: %v", clients)
			}
			cdnDest := cdn["settings"].(map[string]interface{})["fallbacks"].([]interface{})[0].(map[string]interface{})["dest"]
			if toInt(cdn["port"]) != cdnPort || cdn["streamSettings"].(map[string]interface{})["security"] != "tls" {
				t.Fatalf("unexpected CDN entry %v", cdn)
			}
			if variant == VlessVariantEncXHTTP {
				if toInt(node["port"]) != port || toInt(cdnDest) != port || byTag[tag+"-entry"] != nil {
					t.Fatalf("VLESS Encryption keeps XHTTP on the node port: node %v, CDN dest %v", node, cdnDest)
				}
				return
			}
			entry := byTag[tag+"-entry"]
			socket := xhttpCDNSocket(response.Created[0].ID)
			entryDest := entry["settings"].(map[string]interface{})["fallbacks"].([]interface{})[0].(map[string]interface{})["dest"]
			if node["listen"] != socket || entryDest != socket || cdnDest != socket || toInt(entry["port"]) != port {
				t.Fatalf("REALITY entry and CDN entry must fall back to %s: node %v, entry %v, CDN dest %v", socket, node, entry, cdnDest)
			}
			entryStream := entry["streamSettings"].(map[string]interface{})
			if entryStream["security"] != "reality" || entryStream["network"] != "raw" || entryStream["realitySettings"] == nil {
				t.Fatalf("unexpected REALITY entry %v", entryStream)
			}
		})
	}
}

func TestPickCDNPorts(t *testing.T) {
	if _, err := pickCDNPorts(2053, 2, map[int]bool{}); err == nil {
		t.Fatal("a chosen CDN port was accepted for two nodes")
	}
	if _, err := pickCDNPorts(2053, 1, map[int]bool{2053: true}); err == nil {
		t.Fatal("a CDN port used by another inbound was accepted")
	}
	used := quickAddUsedPorts([]map[string]interface{}{{"listen_port": 443, "cdn": map[string]interface{}{"domain": "a.example.com", "port": 2053}}})
	if !used[443] || !used[2053] {
		t.Fatalf("existing CDN ports must count as used: %v", used)
	}
}
