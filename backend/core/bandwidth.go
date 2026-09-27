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
	"time"

	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/ntp"
)

// SpeedTestResult is the download throughput measured through one outbound.
type SpeedTestResult struct {
	Bytes   int64
	Elapsed time.Duration
}

func (r SpeedTestResult) Mbps() float64 {
	if r.Elapsed <= 0 {
		return 0
	}
	return float64(r.Bytes) * 8 / r.Elapsed.Seconds() / 1e6
}

// minimumSpeedSample avoids reporting a rate from a handful of packets.
const minimumSpeedSample = 256 * 1024

// SpeedTest downloads up to maxBytes from link through the outbound or
// endpoint named tag. Only the transfer phase is timed, so connection setup
// latency (measured separately) does not distort the throughput. Hitting the
// context deadline after enough data still yields a valid result.
func SpeedTest(ctx context.Context, tag string, link string, maxBytes int64) (SpeedTestResult, error) {
	detour, err := checkDialer(tag)
	if err != nil {
		return SpeedTestResult{}, err
	}
	linkURL, err := url.Parse(link)
	if err != nil {
		return SpeedTestResult{}, err
	}
	if linkURL.Scheme != "http" && linkURL.Scheme != "https" {
		return SpeedTestResult{}, fmt.Errorf("unsupported speed test scheme: %s", linkURL.Scheme)
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
			return detour.DialContext(ctx, network, M.ParseSocksaddr(addr))
		},
		TLSClientConfig: &tls.Config{
			Time:    ntp.TimeFuncFromContext(ctx),
			RootCAs: adapter.RootPoolFromContext(ctx),
		},
		DisableCompression: true,
	}
	client := http.Client{Transport: transport}
	defer client.CloseIdleConnections()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return SpeedTestResult{}, err
	}
	request.Header.Set("Cache-Control", "no-cache")
	response, err := client.Do(request)
	if err != nil {
		return SpeedTestResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return SpeedTestResult{}, fmt.Errorf("speed test URL returned status %d", response.StatusCode)
	}
	started := time.Now()
	written, copyErr := io.Copy(io.Discard, io.LimitReader(response.Body, maxBytes))
	result := SpeedTestResult{Bytes: written, Elapsed: time.Since(started)}
	if copyErr != nil && !(errors.Is(copyErr, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded)) {
		return result, copyErr
	}
	if written < minimumSpeedSample {
		if copyErr != nil {
			return result, copyErr
		}
		return result, fmt.Errorf("speed test received only %d bytes", written)
	}
	return result, nil
}
