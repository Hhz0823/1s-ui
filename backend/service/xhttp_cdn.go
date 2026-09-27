package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/Hhz0823/1s-ui/util"
	"github.com/Hhz0823/1s-ui/util/common"
)

// XHTTP downlink separation through a CDN ("下行分离"). A node keeps its
// upload on its own entry (REALITY, TLS or VLESS Encryption on the server's
// address) while clients download through a CDN domain whose origin is this
// server. XHTTP pairs the two directions of a session only inside one XHTTP
// inbound, so the Xray configuration of such a node has:
//
//   - the XHTTP inbound itself without a security layer, on an abstract Unix
//     socket, or on the node's port when the node has no security layer
//     (VLESS Encryption);
//   - an entry on the node's port with the node's REALITY or TLS layer, whose
//     VLESS fallback hands every connection to the XHTTP inbound;
//   - a TLS entry on the CDN port, where the CDN connects to the origin,
//     handing its connections to the XHTTP inbound the same way.
//
// Share links put the download side into XHTTP's "extra" (downloadSettings),
// which v2rayN, v2rayNG, PassWall / PassWall 2 and Anywhere apply. A client
// that ignores it keeps both directions on the node's entry, which still
// works because that entry reaches the same XHTTP inbound.

// cloudflareHTTPSPorts are the HTTPS ports Cloudflare proxies to the origin.
var cloudflareHTTPSPorts = []int{443, 2053, 2083, 2087, 2096, 8443}

const cdnProbePath = "/.well-known/1s-ui/cdn-check/"

type xhttpCDN struct {
	Domain      string
	Port        int
	Certificate []string
	Key         []string
}

// parseXHTTPCDN reads an inbound's "cdn" option; nil when there is none.
func parseXHTTPCDN(value interface{}) (*xhttpCDN, error) {
	options, ok := value.(map[string]interface{})
	if !ok || len(options) == 0 {
		return nil, nil
	}
	domain, _ := options["domain"].(string)
	domain, err := normalizeCDNDomain(domain)
	if err != nil {
		return nil, err
	}
	cdn := &xhttpCDN{
		Domain:      domain,
		Port:        intFromInterface(options["port"]),
		Certificate: cdnStringList(options["certificate"]),
		Key:         cdnStringList(options["key"]),
	}
	if cdn.Port < 1 || cdn.Port > 65535 {
		return nil, common.NewError("the CDN port must be 1-65535")
	}
	if _, err = tls.X509KeyPair([]byte(strings.Join(cdn.Certificate, "\n")), []byte(strings.Join(cdn.Key, "\n"))); err != nil {
		return nil, common.NewError("the CDN origin certificate is invalid: ", err)
	}
	return cdn, nil
}

func cdnStringList(value interface{}) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []interface{}:
		lines := make([]string, 0, len(typed))
		for _, item := range typed {
			if line, ok := item.(string); ok {
				lines = append(lines, line)
			}
		}
		return lines
	}
	return nil
}

// normalizeCDNDomain accepts a bare domain name such as cdn.example.com.
func normalizeCDNDomain(value string) (string, error) {
	domain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
	invalid := common.NewError("the CDN domain must be a domain name such as cdn.example.com")
	if domain == "" || len(domain) > 253 || !strings.Contains(domain, ".") || net.ParseIP(domain) != nil {
		return "", invalid
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", invalid
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return "", invalid
			}
		}
	}
	return domain, nil
}

// validateXHTTPCDN checks the "cdn" option of an inbound being saved.
func validateXHTTPCDN(inbound map[string]interface{}) error {
	cdn, err := parseXHTTPCDN(inbound["cdn"])
	if err != nil || cdn == nil {
		return err
	}
	if coreType, _ := inbound["core_type"].(string); inbound["type"] != "vless" || coreType != "xray" {
		return common.NewError("downlink through a CDN needs a VLESS inbound on Xray-core")
	}
	transport, _ := inbound["transport"].(map[string]interface{})
	if transportType, _ := transport["type"].(string); transportType != "xhttp" {
		return common.NewError("downlink through a CDN needs the XHTTP transport")
	}
	if cdn.Port == intFromInterface(inbound["listen_port"]) {
		return common.NewError("the CDN port must differ from the node's port")
	}
	if runtime.GOOS != "linux" {
		return common.NewError("downlink through a CDN is only supported on Linux")
	}
	return nil
}

// newCDNOriginCertificate is the certificate the CDN sees at the origin.
// Cloudflare's "Full" SSL mode accepts any certificate there.
func newCDNOriginCertificate(domain string) (certificate, key []string, err error) {
	now := time.Now()
	keyPEM, certPEM, err := util.GenerateSelfSignedTLS(domain, now, now.AddDate(10, 0, 0))
	if err != nil {
		return nil, nil, err
	}
	return pemLines(certPEM), pemLines(keyPEM), nil
}

// pickCDNPorts chooses a CDN port for each node: the requested one for a
// single node, otherwise free Cloudflare HTTPS ports.
func pickCDNPorts(requested, count int, used map[int]bool) ([]int, error) {
	if requested > 0 {
		if count != 1 {
			return nil, common.NewError("a CDN port can only be chosen for a single node")
		}
		if used[requested] || !localPortFree(requested) {
			return nil, common.NewErrorf("CDN port %d is already in use", requested)
		}
		return []int{requested}, nil
	}
	ports := make([]int, 0, count)
	for _, port := range cloudflareHTTPSPorts {
		if len(ports) == count {
			break
		}
		if !used[port] && localPortFree(port) {
			ports = append(ports, port)
			used[port] = true
		}
	}
	if len(ports) < count {
		return nil, common.NewErrorf("only %d of Cloudflare's HTTPS ports (443, 2053, 2083, 2087, 2096, 8443) are free for CDN downlinks", len(ports))
	}
	return ports, nil
}

func localPortFree(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

// cdnProbeResult is what a probe through the CDN found out.
type cdnProbeResult struct {
	// ServerIP is this server's address as the CDN saw it.
	ServerIP string
}

// Test hooks: how the probe connects to a domain, and the roots it trusts
// (nil: the system roots).
var (
	cdnProbeDial  = dialPreferIPv4
	cdnProbeRoots = func() *tls.Config { return nil }
)

// dialPreferIPv4 reaches the CDN over IPv4 when it can, so the address the
// CDN reports for this server is the IPv4 one most clients use.
func dialPreferIPv4(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	if conn, err := dialer.DialContext(ctx, "tcp4", address); err == nil {
		return conn, nil
	}
	return dialer.DialContext(ctx, network, address)
}

const cdnProbeTimeout = 20 * time.Second

// probeCDNDomain checks that https://domain:port reaches this server through
// the CDN: it serves a random token on the port for a moment and fetches it
// through the domain.
func probeCDNDomain(domain string, port int, certificate, key []string) (cdnProbeResult, error) {
	pair, err := tls.X509KeyPair([]byte(strings.Join(certificate, "\n")), []byte(strings.Join(key, "\n")))
	if err != nil {
		return cdnProbeResult{}, err
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return cdnProbeResult{}, common.NewErrorf("CDN port %d is already in use", port)
	}
	token := common.Random(24)
	server := &http.Server{
		Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != cdnProbePath+token {
				http.NotFound(writer, request)
				return
			}
			ip := request.Header.Get("CF-Connecting-IP")
			if ip == "" {
				ip, _, _ = strings.Cut(request.Header.Get("X-Forwarded-For"), ",")
			}
			writer.Header().Set("X-1S-UI-Client", strings.TrimSpace(ip))
			_, _ = io.WriteString(writer, token)
		}),
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{pair}, NextProtos: []string{"h2", "http/1.1"}},
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = server.ServeTLS(listener, "", "") }()
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), cdnProbeTimeout)
	defer cancel()
	target := fmt.Sprintf("https://%s:%d%s%s", domain, port, cdnProbePath, token)
	response, body, err := cdnProbeGet(ctx, target, domain, false)
	if err != nil {
		var verifyErr *tls.CertificateVerificationError
		if errors.As(err, &verifyErr) {
			// The domain may point straight at this server without the CDN.
			if _, direct, directErr := cdnProbeGet(ctx, target, domain, true); directErr == nil && direct == token {
				return cdnProbeResult{}, common.NewErrorf("%s reaches this server directly, not through a CDN: turn on the CDN proxy for the domain (Cloudflare: orange cloud)", domain)
			}
		}
		return cdnProbeResult{}, common.NewErrorf("%s:%d is unreachable from this server: %v", domain, port, shortNetError(err))
	}
	if body != token {
		return cdnProbeResult{}, cdnProbeStatusError(domain, port, response.StatusCode)
	}
	return cdnProbeResult{ServerIP: response.Header.Get("X-1S-UI-Client")}, nil
}

func cdnProbeGet(ctx context.Context, target, domain string, insecure bool) (*http.Response, string, error) {
	tlsConfig := cdnProbeRoots()
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	}
	tlsConfig = tlsConfig.Clone()
	tlsConfig.ServerName = domain
	tlsConfig.InsecureSkipVerify = insecure
	client := &http.Client{
		Transport: &http.Transport{
			DialContext:       cdnProbeDial,
			TLSClientConfig:   tlsConfig,
			ForceAttemptHTTP2: true,
			// The probe must reach the CDN directly, not through a proxy.
			Proxy: nil,
		},
		// Only the tested address counts.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, "", err
	}
	request.Header.Set("User-Agent", "1s-ui-cdn-check")
	response, err := client.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return response, string(body), nil
}

// cdnProbeStatusError explains what a CDN answered instead of the token.
func cdnProbeStatusError(domain string, port, status int) error {
	switch status {
	case 403:
		return common.NewErrorf("the CDN blocked the request to %s (HTTP 403); turn off bot protection (Cloudflare: Bot Fight Mode) and WAF rules that challenge non-browser clients for this domain", domain)
	case 521, 522, 523:
		return common.NewErrorf("the CDN cannot connect to port %d of this server (HTTP %d): make sure %s points to this server's IP and that the firewall or security group allows TCP %d", port, status, domain, port)
	case 525:
		return common.NewErrorf("the CDN's TLS handshake with port %d failed (HTTP 525); set Cloudflare SSL/TLS to \"Full\"", port)
	case 526:
		return common.NewErrorf("the CDN rejects the origin certificate (HTTP 526); set Cloudflare SSL/TLS to \"Full\" instead of \"Full (strict)\"")
	}
	return common.NewErrorf("%s:%d did not reach this server through the CDN (HTTP %d); check that the domain is proxied to this server's IP and that Cloudflare SSL/TLS is \"Full\"", domain, port, status)
}

// Cloudflare's published address ranges (https://www.cloudflare.com/ips/).
var cloudflareNetworks = func() []*net.IPNet {
	var networks []*net.IPNet
	for _, cidr := range []string{
		"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
		"141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20",
		"197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
		"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
		"2400:cb00::/32", "2606:4700::/32", "2803:f800::/32", "2405:b500::/32",
		"2405:8100::/32", "2a06:98c0::/29", "2c0f:f248::/32",
	} {
		_, network, _ := net.ParseCIDR(cidr)
		networks = append(networks, network)
	}
	return networks
}()

// hostBehindCloudflare reports whether a host name or address only leads to
// Cloudflare, so connecting to it cannot reach this server's own ports.
func hostBehindCloudflare(host string) bool {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return false
		}
		for _, addr := range addrs {
			ips = append(ips, addr.IP)
		}
	}
	if len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		inside := false
		for _, network := range cloudflareNetworks {
			if network.Contains(ip) {
				inside = true
				break
			}
		}
		if !inside {
			return false
		}
	}
	return true
}

// lookupPublicIP asks public services for this server's address.
var lookupPublicIP = func() string {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	client := &http.Client{Transport: &http.Transport{DialContext: dialPreferIPv4, Proxy: nil}}
	defer client.CloseIdleConnections()
	results := make(chan string, 3)
	for _, source := range []string{"https://www.cloudflare.com/cdn-cgi/trace", "https://api.ipify.org", "https://ipv4.icanhazip.com"} {
		go func(source string) {
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
			if err != nil {
				results <- ""
				return
			}
			response, err := client.Do(request)
			if err != nil {
				results <- ""
				return
			}
			defer response.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
			text := strings.TrimSpace(string(body))
			for _, line := range strings.Split(text, "\n") {
				if value, found := strings.CutPrefix(line, "ip="); found {
					text = value
				}
			}
			if net.ParseIP(text) == nil {
				text = ""
			}
			results <- text
		}(source)
	}
	for range 3 {
		if ip := <-results; ip != "" {
			return ip
		}
	}
	return ""
}

// quickAddCDN is what a batch of CDN nodes shares.
type quickAddCDN struct {
	ports       []int
	certificate []string
	key         []string
	// nodeHost is where uploads go: the server itself, never a CDN name.
	nodeHost string
}

// prepareQuickAddCDN picks the CDN ports of a batch and proves, before
// anything is created, that the domain reaches this server through the CDN
// on each of them.
func prepareQuickAddCDN(request RemoteQuickAddRequest, inbounds []map[string]interface{}, nodePorts []int, publicHost string) (*quickAddCDN, error) {
	used := quickAddUsedPorts(inbounds)
	for _, port := range nodePorts {
		used[port] = true
	}
	ports, err := pickCDNPorts(request.CDNPort, request.Count, used)
	if err != nil {
		return nil, err
	}
	certificate, key, err := newCDNOriginCertificate(request.CDNDomain)
	if err != nil {
		return nil, err
	}
	serverIP := ""
	for _, port := range ports {
		result, err := probeCDNDomain(request.CDNDomain, port, certificate, key)
		if err != nil {
			return nil, err
		}
		if serverIP == "" {
			serverIP = result.ServerIP
		}
	}
	nodeHost := strings.Trim(publicHost, "[]")
	if strings.EqualFold(nodeHost, request.CDNDomain) || hostBehindCloudflare(nodeHost) {
		if net.ParseIP(serverIP) == nil {
			serverIP = lookupPublicIP()
		}
		if serverIP == "" {
			return nil, common.NewErrorf("%s goes through a CDN and this server's own IP is unknown; open the panel by the server's IP and try again", nodeHost)
		}
		nodeHost = serverIP
	}
	return &quickAddCDN{ports: ports, certificate: certificate, key: key, nodeHost: nodeHost}, nil
}

// quickAddUsedPorts are the ports existing inbounds listen on, CDN ports
// included.
func quickAddUsedPorts(inbounds []map[string]interface{}) map[int]bool {
	used := make(map[int]bool, len(inbounds))
	for _, inbound := range inbounds {
		if port := intFromInterface(inbound["listen_port"]); port > 0 {
			used[port] = true
		}
		if cdn, ok := inbound["cdn"].(map[string]interface{}); ok {
			if port := intFromInterface(cdn["port"]); port > 0 {
				used[port] = true
			}
		}
	}
	return used
}
