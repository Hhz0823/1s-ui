package core

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	urltest "github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
)

const checkTimeout = 15 * time.Second

type CheckOutboundResult struct {
	OK    bool
	Delay uint16
	Error string
}

type CheckWarpResult struct {
	OK    bool
	Delay uint16
	IP    string
	Colo  string
	Loc   string
	Warp  string
	Error string
}

func CheckOutbound(ctx context.Context, tag string, link string) (result CheckOutboundResult) {
	ob, err := checkDialer(tag)
	if err != nil {
		result.Error = err.Error()
		return
	}

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	delay, err := urltest.URLTest(ctx, link, ob)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.OK = true
	result.Delay = delay
	return result
}

func CheckWarp(ctx context.Context, tag string, link string) (result CheckWarpResult) {
	ob, err := checkDialer(tag)
	if err != nil {
		result.Error = err.Error()
		return
	}
	if link == "" {
		link = "https://www.cloudflare.com/cdn-cgi/trace"
	}

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	trace, delay, err := fetchWarpTrace(ctx, link, ob)
	if err != nil {
		result.Error = err.Error()
		return
	}
	result.OK = true
	result.Delay = delay
	result.IP = trace["ip"]
	result.Colo = trace["colo"]
	result.Loc = trace["loc"]
	result.Warp = trace["warp"]
	return
}

func checkDialer(tag string) (N.Dialer, error) {
	if outbound_manager == nil {
		return nil, errors.New("core not running")
	}
	ob, ok := outbound_manager.Outbound(tag)
	if ok {
		return ob, nil
	}
	if endpoint_manager == nil {
		return nil, errors.New("outbound or endpoint not found")
	}
	ep, epOk := endpoint_manager.Get(tag)
	if !epOk {
		return nil, errors.New("outbound or endpoint not found")
	}
	return ep, nil
}

func fetchWarpTrace(ctx context.Context, link string, detour N.Dialer) (map[string]string, uint16, error) {
	linkURL, err := url.Parse(link)
	if err != nil {
		return nil, 0, err
	}
	hostname := linkURL.Hostname()
	port := linkURL.Port()
	if port == "" {
		switch linkURL.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return nil, 0, fmt.Errorf("unsupported trace scheme: %s", linkURL.Scheme)
		}
	}
	if hostname == "" {
		return nil, 0, errors.New("missing trace host")
	}

	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
				return detour.DialContext(ctx, network, M.ParseSocksaddrHostPortStr(hostname, port))
			},
			TLSClientConfig: &tls.Config{
				Time:    ntp.TimeFuncFromContext(ctx),
				RootCAs: adapter.RootPoolFromContext(ctx),
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: C.TCPTimeout,
	}
	defer client.CloseIdleConnections()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, 0, err
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	delay := uint16(time.Since(start) / time.Millisecond)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, delay, fmt.Errorf("warp trace request returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return nil, delay, err
	}
	trace := make(map[string]string)
	for _, line := range strings.Split(string(body), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && key != "" {
			trace[key] = value
		}
	}
	if len(trace) == 0 {
		return nil, delay, errors.New("empty warp trace response")
	}
	return trace, delay, nil
}
