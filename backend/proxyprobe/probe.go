// Package proxyprobe checks a proxy the way a client uses it: connect,
// authenticate, open a tunnel and fetch a small page through it. SOCKS5 and
// HTTP proxies are spoken to directly; nodes from share links (VLESS, VMess,
// Trojan, Shadowsocks, Hysteria2, TUIC …) go through a sing-box outbound the
// caller dials with. The panel runs it on whichever server should reach the
// proxy, so a node can be watched from a relay in the network that uses it.
package proxyprobe

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	TypeSOCKS5 = "socks5"
	TypeHTTP   = "http"
	// TypeNode is a share link checked through a sing-box outbound.
	TypeNode = "node"

	// MaxLinkLength bounds a node's share link.
	MaxLinkLength = 8 << 10

	// DefaultTarget answers with the exit IP and its country, so one request
	// shows both that the proxy works and where its traffic leaves.
	DefaultTarget = "https://www.cloudflare.com/cdn-cgi/trace"

	// Timeout bounds one whole check.
	Timeout = 15 * time.Second

	maxBody = 8 << 10
)

// handshakeTimeout bounds the proxy's answer to the login and tunnel
// request; a proxy of the other type usually just waits for more bytes.
var handshakeTimeout = 6 * time.Second

// Test hooks: tests reach proxies on loopback and trust their own target.
var (
	dialControl = allowedAddress
	rootCAs     *x509.CertPool
)

// Stages name the step a check failed at, so clients can explain it.
const (
	StageConfig  = "config"
	StageConnect = "connect"
	// StageHandshake: the node's server answered on TCP (or, for QUIC
	// protocols, was tried) but the proxy handshake failed: TLS, REALITY,
	// QUIC or the login was rejected or interfered with.
	StageHandshake = "handshake"
	StageAuth      = "auth"
	StageTunnel    = "tunnel"
	StageTLS       = "tls"
	StageHTTP      = "http"
)

// Spec is one proxy and the page fetched through it. A node (TypeNode) is
// described by its share link; Host and Port are then informational.
type Spec struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Target   string `json:"target,omitempty"`
	Link     string `json:"link,omitempty"`
}

// Result is one check. Times are milliseconds; LatencyMs runs from the first
// connection attempt to the target's response headers.
type Result struct {
	OK          bool   `json:"ok"`
	Time        int64  `json:"time"`
	ConnectMs   int64  `json:"connect_ms"`
	HandshakeMs int64  `json:"handshake_ms"`
	TLSMs       int64  `json:"tls_ms"`
	TTFBMs      int64  `json:"ttfb_ms"`
	LatencyMs   int64  `json:"latency_ms"`
	Status      int    `json:"status,omitempty"`
	ExitIP      string `json:"exit_ip,omitempty"`
	Country     string `json:"country,omitempty"`
	Stage       string `json:"stage,omitempty"`
	Error       string `json:"error,omitempty"`
}

// Normalize validates a spec in place and fills in the default target.
func Normalize(spec *Spec) error {
	spec.Type = strings.ToLower(strings.TrimSpace(spec.Type))
	switch spec.Type {
	case "socks", "socks5h":
		spec.Type = TypeSOCKS5
	}
	switch spec.Type {
	case TypeNode:
		spec.Link = strings.TrimSpace(spec.Link)
		if spec.Link == "" || !strings.Contains(spec.Link, "://") {
			return errors.New("a node check needs the node's share link")
		}
		if len(spec.Link) > MaxLinkLength {
			return errors.New("share link is too long")
		}
	case TypeSOCKS5, TypeHTTP:
		spec.Host = strings.Trim(strings.TrimSpace(spec.Host), "[]")
		if spec.Host == "" || len(spec.Host) > 253 || strings.ContainsAny(spec.Host, " /?#@\t\r\n") {
			return errors.New("proxy host must be a hostname or IP address")
		}
		if spec.Port < 1 || spec.Port > 65535 {
			return errors.New("proxy port must be between 1 and 65535")
		}
		if len(spec.Username) > 255 || len(spec.Password) > 255 {
			return errors.New("proxy username and password must be at most 255 bytes")
		}
	default:
		return errors.New("proxy type must be socks5, http or node")
	}
	target, err := normalizeTarget(spec.Target)
	if err != nil {
		return err
	}
	spec.Target = target
	return nil
}

// normalizeTarget checks a test URL, defaulting to the trace page.
func normalizeTarget(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = DefaultTarget
	}
	target, err := url.Parse(value)
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Hostname() == "" || target.User != nil {
		return "", errors.New("test URL must be an http:// or https:// address")
	}
	if len(value) > 2048 {
		return "", errors.New("test URL is too long")
	}
	return value, nil
}

// Run checks a SOCKS5 or HTTP proxy once. It never returns an error:
// failures are part of the result, with the stage they happened at.
func Run(ctx context.Context, spec Spec) Result {
	result := Result{Time: time.Now().Unix()}
	if err := Normalize(&spec); err != nil {
		return failed(result, StageConfig, err)
	}
	if spec.Type == TypeNode {
		return failed(result, StageConfig, errors.New("node checks run through a sing-box outbound"))
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	target, _ := url.Parse(spec.Target)

	started := time.Now()
	dialer := &net.Dialer{Timeout: 6 * time.Second, ControlContext: dialControl}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(spec.Host, strconv.Itoa(spec.Port)))
	if err != nil {
		return failed(result, StageConnect, err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	connected := time.Now()
	result.ConnectMs = since(started, connected)

	reader := bufio.NewReader(conn)
	targetHost, targetPort := targetAddress(target)

	// A plain-HTTP page through an HTTP proxy is fetched the way browsers do,
	// with an absolute URL; everything else goes through a tunnel.
	options := fetchOptions{forward: spec.Type == TypeHTTP && target.Scheme == "http"}
	if !options.forward {
		handshakeDeadline := connected.Add(handshakeTimeout)
		if deadline, ok := ctx.Deadline(); ok && deadline.Before(handshakeDeadline) {
			handshakeDeadline = deadline
		}
		_ = conn.SetDeadline(handshakeDeadline)
		var stage string
		if spec.Type == TypeSOCKS5 {
			stage, err = socks5Connect(conn, reader, spec.Username, spec.Password, targetHost, targetPort)
		} else {
			stage, err = httpConnect(conn, reader, spec.Username, spec.Password, net.JoinHostPort(targetHost, targetPort))
		}
		if err != nil {
			return failed(result, stage, err)
		}
		result.HandshakeMs = since(connected, time.Now())
		if deadline, ok := ctx.Deadline(); ok {
			_ = conn.SetDeadline(deadline)
		}
	} else if spec.Username != "" || spec.Password != "" {
		options.proxyAuth = basicAuth(spec.Username, spec.Password)
	}
	return fetch(ctx, &bufferedConn{Conn: conn, reader: reader}, target, options, result, started)
}

// DialFunc opens a connection to host:port through the node under test.
type DialFunc func(ctx context.Context, host, port string) (net.Conn, error)

// RunDialer checks a node once through dial, which reaches the target
// through the node (a sing-box outbound built from its share link). When
// server is set (host:port of a TCP node), a plain TCP connection to it comes
// first, so an address that is blocked or down is told apart from a proxy
// protocol that fails. ConnectMs is that TCP connection; HandshakeMs is the
// node's own connection setup; LatencyMs runs from the node connection to
// the target's response headers, like a client's real-delay test.
func RunDialer(ctx context.Context, target string, server string, dial DialFunc) Result {
	result := Result{Time: time.Now().Unix()}
	normalized, err := normalizeTarget(target)
	if err != nil {
		return failed(result, StageConfig, err)
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	targetURL, _ := url.Parse(normalized)

	if server != "" {
		started := time.Now()
		dialer := &net.Dialer{Timeout: 6 * time.Second, ControlContext: dialControl}
		conn, err := dialer.DialContext(ctx, "tcp", server)
		if err != nil {
			return failed(result, StageConnect, err)
		}
		result.ConnectMs = since(started, time.Now())
		_ = conn.Close()
	}

	started := time.Now()
	host, port := targetAddress(targetURL)
	conn, err := dial(ctx, host, port)
	if err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return failed(result, StageHandshake, err)
	}
	defer conn.Close()
	// Not every proxied connection honors deadlines; closing it does.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	result.HandshakeMs = since(started, time.Now())
	return fetch(ctx, conn, targetURL, fetchOptions{node: true}, result, started)
}

func targetAddress(target *url.URL) (string, string) {
	port := target.Port()
	if port == "" {
		port = "443"
		if target.Scheme == "http" {
			port = "80"
		}
	}
	return target.Hostname(), port
}

type fetchOptions struct {
	// forward sends an absolute-URL request to an HTTP proxy.
	forward   bool
	proxyAuth string
	// node marks a connection through a node: when it breaks before the
	// target answers, the node accepted the connection but did not relay.
	node bool
}

// errNodeDidNotRelay explains a node connection that broke before the target
// answered: usually a wrong UUID or password (most protocols only check it
// once data flows), interference, or a node that cannot reach the test URL.
const errNodeDidNotRelay = "the node did not relay the connection (wrong UUID/password, interference, or the node cannot reach the test URL)"

// fetch requests the target over conn and reads the answer into result.
func fetch(ctx context.Context, conn net.Conn, target *url.URL, options fetchOptions, result Result, started time.Time) Result {
	var stream io.ReadWriter = conn
	if target.Scheme == "https" {
		tlsStarted := time.Now()
		tlsConn := tls.Client(conn, &tls.Config{
			ServerName: target.Hostname(),
			RootCAs:    rootCAs,
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"http/1.1"},
		})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			if options.node && !certificateError(err) {
				return failed(result, StageTunnel, fmt.Errorf("%s: %s", errNodeDidNotRelay, describe(err)))
			}
			return failed(result, StageTLS, err)
		}
		result.TLSMs = since(tlsStarted, time.Now())
		stream = tlsConn
	}

	requestURI := target.RequestURI()
	if options.forward {
		requestURI = target.String()
	}
	request := "GET " + requestURI + " HTTP/1.1\r\nHost: " + target.Host +
		"\r\nUser-Agent: 1s-ui-probe/1\r\nAccept: */*\r\nConnection: close\r\n"
	if options.proxyAuth != "" {
		request += "Proxy-Authorization: Basic " + options.proxyAuth + "\r\n"
	}
	request += "\r\n"
	requestSent := time.Now()
	// With an absolute URL the proxy itself answers first, so a dead
	// connection here means it is not an HTTP proxy.
	readStage := StageHTTP
	if options.forward || options.node && target.Scheme == "http" {
		readStage = StageTunnel
	}
	if _, err := io.WriteString(stream, request); err != nil {
		return failed(result, readStage, err)
	}
	responseReader := bufio.NewReader(stream)
	response, err := http.ReadResponse(responseReader, &http.Request{Method: http.MethodGet})
	if err != nil {
		switch {
		case options.forward:
			err = fmt.Errorf("no HTTP proxy reply (is it a SOCKS5 proxy?): %s", describe(err))
		case readStage == StageTunnel:
			err = fmt.Errorf("%s: %s", errNodeDidNotRelay, describe(err))
		}
		return failed(result, readStage, err)
	}
	defer response.Body.Close()
	now := time.Now()
	result.TTFBMs = since(requestSent, now)
	result.LatencyMs = since(started, now)
	result.Status = response.StatusCode
	if options.forward && response.StatusCode == http.StatusProxyAuthRequired {
		return failed(result, StageAuth, errors.New("proxy rejected the username or password"))
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, maxBody))
	if response.StatusCode >= 400 {
		return failed(result, StageHTTP, fmt.Errorf("test URL answered HTTP %d", response.StatusCode))
	}
	if target.String() == DefaultTarget || strings.HasSuffix(target.Path, "/cdn-cgi/trace") {
		result.ExitIP, result.Country = parseTrace(string(body))
	}
	result.OK = true
	return result
}

// certificateError reports a TLS failure caused by the target's
// certificate rather than by the connection carrying it.
func certificateError(err error) bool {
	var verification *tls.CertificateVerificationError
	var unknown x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	return errors.As(err, &verification) || errors.As(err, &unknown) || errors.As(err, &hostname) || errors.As(err, &invalid)
}

func failed(result Result, stage string, err error) Result {
	result.OK = false
	result.Stage = stage
	result.Error = describe(err)
	return result
}

func since(from, to time.Time) int64 {
	ms := to.Sub(from).Milliseconds()
	if ms < 1 && to.After(from) {
		return 1
	}
	return ms
}

// describe keeps error text short and free of the proxy credentials.
func describe(err error) string {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timed out"
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "connection refused"
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return "connection closed by the proxy"
	}
	text := err.Error()
	if index := strings.LastIndex(text, ": "); index >= 0 && strings.HasPrefix(text, "dial tcp") {
		text = text[index+2:]
	}
	if len(text) > 200 {
		text = text[:200]
	}
	return text
}

// allowedAddress refuses addresses a proxy check has no reason to reach:
// loopback (the server's own services), link-local (cloud metadata) and
// non-unicast addresses. Private networks stay allowed for VPC relays.
func allowedAddress(_ context.Context, _, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return err
	}
	return allowedIP(ip)
}

func allowedIP(ip netip.Addr) error {
	ip = ip.Unmap()
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() ||
		ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() || ip == netip.AddrFrom4([4]byte{255, 255, 255, 255}) ||
		(ip.Is4() && ip.As4()[0] == 0) {
		return fmt.Errorf("address %s is not allowed for proxy checks", ip)
	}
	return nil
}

// CheckNodeHost resolves a node's server name and refuses the same
// addresses a direct proxy check may not reach. A node's own connection is
// made by sing-box, so the check happens before it is built.
func CheckNodeHost(ctx context.Context, host string) error {
	if dialControl == nil {
		return nil
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" {
		return errors.New("the share link has no server address")
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return allowedIP(ip)
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("cannot resolve %s: %s", host, describe(err))
	}
	for _, ip := range addresses {
		if err := allowedIP(ip); err != nil {
			return err
		}
	}
	return nil
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.reader.Read(p) }

func basicAuth(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

func httpConnect(conn net.Conn, reader *bufio.Reader, username, password, target string) (string, error) {
	request := "CONNECT " + target + " HTTP/1.1\r\nHost: " + target + "\r\nUser-Agent: 1s-ui-probe/1\r\n"
	if username != "" || password != "" {
		request += "Proxy-Authorization: Basic " + basicAuth(username, password) + "\r\n"
	}
	request += "\r\n"
	if _, err := io.WriteString(conn, request); err != nil {
		return StageTunnel, err
	}
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		return StageTunnel, fmt.Errorf("no HTTP proxy reply (is it a SOCKS5 proxy?): %s", describe(err))
	}
	_ = response.Body.Close()
	switch {
	case response.StatusCode == http.StatusProxyAuthRequired:
		if username == "" && password == "" {
			return StageAuth, errors.New("proxy requires a username and password")
		}
		return StageAuth, errors.New("proxy rejected the username or password")
	case response.StatusCode < 200 || response.StatusCode > 299:
		return StageTunnel, fmt.Errorf("proxy refused the tunnel: %s", response.Status)
	}
	return "", nil
}

var socks5Replies = map[byte]string{
	1: "general SOCKS server failure",
	2: "connection not allowed by ruleset",
	3: "network unreachable",
	4: "host unreachable",
	5: "connection refused by the destination",
	6: "TTL expired",
	7: "command not supported",
	8: "address type not supported",
}

func socks5Connect(conn net.Conn, reader *bufio.Reader, username, password, host, port string) (string, error) {
	methods := []byte{0x00}
	if username != "" || password != "" {
		methods = []byte{0x02, 0x00}
	}
	if _, err := conn.Write(append([]byte{0x05, byte(len(methods))}, methods...)); err != nil {
		return StageTunnel, err
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(reader, reply); err != nil {
		return StageTunnel, fmt.Errorf("no SOCKS5 reply (is it an HTTP proxy?): %s", describe(err))
	}
	if reply[0] != 0x05 {
		return StageTunnel, errors.New("not a SOCKS5 proxy (is it an HTTP proxy?)")
	}
	switch reply[1] {
	case 0x00:
	case 0x02:
		if username == "" && password == "" {
			return StageAuth, errors.New("proxy requires a username and password")
		}
		auth := []byte{0x01, byte(len(username))}
		auth = append(auth, username...)
		auth = append(auth, byte(len(password)))
		auth = append(auth, password...)
		if _, err := conn.Write(auth); err != nil {
			return StageAuth, err
		}
		if _, err := io.ReadFull(reader, reply); err != nil {
			return StageAuth, err
		}
		if reply[1] != 0x00 {
			return StageAuth, errors.New("proxy rejected the username or password")
		}
	case 0xff:
		if username == "" && password == "" {
			return StageAuth, errors.New("proxy requires a username and password")
		}
		return StageAuth, errors.New("proxy accepts none of the offered login methods")
	default:
		return StageAuth, fmt.Errorf("proxy chose unsupported login method %d", reply[1])
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return StageConfig, errors.New("invalid test URL port")
	}
	request := []byte{0x05, 0x01, 0x00}
	if ip, err := netip.ParseAddr(host); err == nil {
		if ip.Is4() {
			v4 := ip.As4()
			request = append(append(request, 0x01), v4[:]...)
		} else {
			v6 := ip.As16()
			request = append(append(request, 0x04), v6[:]...)
		}
	} else {
		if len(host) > 255 {
			return StageConfig, errors.New("test URL host is too long")
		}
		request = append(append(request, 0x03, byte(len(host))), host...)
	}
	request = binary.BigEndian.AppendUint16(request, uint16(portNumber))
	if _, err := conn.Write(request); err != nil {
		return StageTunnel, err
	}
	head := make([]byte, 4)
	if _, err := io.ReadFull(reader, head); err != nil {
		return StageTunnel, err
	}
	if head[1] != 0x00 {
		if text, ok := socks5Replies[head[1]]; ok {
			return StageTunnel, errors.New(text)
		}
		return StageTunnel, fmt.Errorf("SOCKS5 error %d", head[1])
	}
	var skip int
	switch head[3] {
	case 0x01:
		skip = 4
	case 0x04:
		skip = 16
	case 0x03:
		length, err := reader.ReadByte()
		if err != nil {
			return StageTunnel, err
		}
		skip = int(length)
	default:
		return StageTunnel, errors.New("invalid SOCKS5 reply")
	}
	if _, err := io.CopyN(io.Discard, reader, int64(skip+2)); err != nil {
		return StageTunnel, err
	}
	return "", nil
}

// parseTrace reads the exit IP and country from a Cloudflare trace page.
func parseTrace(body string) (string, string) {
	var ip, country string
	for _, line := range strings.Split(body, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "ip":
			if _, err := netip.ParseAddr(value); err == nil {
				ip = value
			}
		case "loc":
			if len(value) == 2 {
				country = strings.ToUpper(value)
			}
		}
	}
	return ip, country
}
