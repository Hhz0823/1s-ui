package speedtest

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// The client side runs the same tests as the monitor app, so a home relay
// (a NAS or router running the panel) can measure its line to a server.

// Test names a client test.
const (
	TestTCPPing     = "tcp_ping"
	TestUDPPing     = "udp_ping"
	TestTCPDownload = "tcp_download"
	TestTCPUpload   = "tcp_upload"
	TestUDPDownload = "udp_download"
	TestUDPUpload   = "udp_upload"
)

// Tests lists every test in the order a full run takes them.
var Tests = []string{TestTCPPing, TestUDPPing, TestTCPDownload, TestTCPUpload, TestUDPDownload, TestUDPUpload}

type PingStats struct {
	Sent     int     `json:"sent"`
	Received int     `json:"received"`
	MinMs    float64 `json:"min_ms"`
	AvgMs    float64 `json:"avg_ms"`
	MaxMs    float64 `json:"max_ms"`
	JitterMs float64 `json:"jitter_ms"`
	LossPct  float64 `json:"loss_pct"`
}

type ThroughputStats struct {
	Bytes         int64   `json:"bytes"`
	Seconds       float64 `json:"seconds"`
	BitsPerSecond float64 `json:"bits_per_second"`
	Streams       int     `json:"streams"`
}

type UDPStats struct {
	TargetMbps      int     `json:"target_mbps"`
	SentPackets     int64   `json:"sent_packets"`
	ReceivedPackets int64   `json:"received_packets"`
	Bytes           int64   `json:"bytes"`
	Seconds         float64 `json:"seconds"`
	BitsPerSecond   float64 `json:"bits_per_second"`
	JitterMs        float64 `json:"jitter_ms"`
	OutOfOrder      int64   `json:"out_of_order"`
	LossPct         float64 `json:"loss_pct"`
}

const (
	clientConnectTimeout = 5 * time.Second
	clientPacketSize     = MaxUDPPacket
)

// clientPayload is random so compression on the path cannot flatter results.
var clientPayload = func() []byte {
	buf := make([]byte, 64<<10)
	_, _ = rand.Read(buf)
	return buf
}()

// Client runs tests against one session on one server address.
type Client struct {
	Host  string
	Port  int
	token [16]byte
}

// Connect returns a client for the first host that accepts a TCP connection
// on the session's port. The port only answers holders of the token, so
// trying several addresses is harmless.
func Connect(ctx context.Context, token string, port int, hosts []string) (*Client, error) {
	raw, err := hex.DecodeString(token)
	if err != nil || len(raw) != 16 {
		return nil, errors.New("invalid speed test token")
	}
	if port < 1 || port > 65535 {
		return nil, errors.New("invalid speed test port")
	}
	var tried []string
	for _, host := range hosts {
		host = strings.Trim(strings.TrimSpace(host), "[]")
		if host == "" {
			continue
		}
		tried = append(tried, host)
		dialer := net.Dialer{Timeout: 3 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		if err != nil {
			continue
		}
		_ = conn.Close()
		client := &Client{Host: host, Port: port}
		copy(client.token[:], raw)
		return client, nil
	}
	if len(tried) == 0 {
		return nil, errors.New("the server has no known address; set its public address in the panel")
	}
	return nil, fmt.Errorf("cannot reach speed test port %d (tried %s); allow TCP and UDP %d in the firewall and security group", port, strings.Join(tried, ", "), port)
}

func (c *Client) address() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// openTCP connects and sends the test header; the server answers with a
// status byte.
func (c *Client) openTCP(ctx context.Context, mode byte, duration time.Duration) (net.Conn, error) {
	dialer := net.Dialer{Timeout: clientConnectTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.address())
	if err != nil {
		return nil, err
	}
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
	}
	header := make([]byte, TCPHeaderLen)
	copy(header, TCPMagic)
	header[4], header[5] = Version, mode
	binary.BigEndian.PutUint16(header[6:8], uint16(duration/time.Millisecond))
	copy(header[8:], c.token[:])
	_ = conn.SetDeadline(time.Now().Add(clientConnectTimeout))
	status := make([]byte, 1)
	if _, err := conn.Write(header); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if _, err := io.ReadFull(conn, status); err != nil {
		_ = conn.Close()
		return nil, errors.New("the speed test port did not answer; it may not be a speed test service")
	}
	switch status[0] {
	case StatusOK:
		_ = conn.SetDeadline(time.Now().Add(duration + 10*time.Second))
		return conn, nil
	case StatusBadToken:
		err = errors.New("the speed test session expired; start again")
	case StatusBusy:
		err = errors.New("the server is running too many speed tests; try again shortly")
	default:
		err = fmt.Errorf("the speed test service refused the request (%d)", status[0])
	}
	_ = conn.Close()
	return nil, err
}

// closeOnDone closes conn when ctx ends, which unblocks reads and writes.
func closeOnDone(ctx context.Context, conn io.Closer) func() bool {
	return context.AfterFunc(ctx, func() { _ = conn.Close() })
}

// TCPPing measures round trips over one TCP connection.
func (c *Client) TCPPing(ctx context.Context, count int) (PingStats, error) {
	if count < 1 || count > 100 {
		count = 10
	}
	conn, err := c.openTCP(ctx, ModeEcho, time.Duration(count)*300*time.Millisecond+2*time.Second)
	if err != nil {
		return PingStats{}, err
	}
	defer conn.Close()
	defer closeOnDone(ctx, conn)()
	var rtts []float64
	message, reply := make([]byte, 8), make([]byte, 8)
	for seq := 0; seq < count; seq++ {
		binary.BigEndian.PutUint64(message, uint64(seq))
		started := time.Now()
		if _, err := conn.Write(message); err != nil {
			return PingStats{}, err
		}
		if _, err := io.ReadFull(conn, reply); err != nil {
			return PingStats{}, err
		}
		if binary.BigEndian.Uint64(reply) == uint64(seq) {
			rtts = append(rtts, float64(time.Since(started).Microseconds())/1000)
		}
		if !sleepCtx(ctx, 100*time.Millisecond) {
			return PingStats{}, ctx.Err()
		}
	}
	return pingStats(count, rtts), nil
}

func (c *Client) openUDP(ctx context.Context) (*net.UDPConn, error) {
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", c.Host)
	if err != nil || len(addresses) == 0 {
		return nil, fmt.Errorf("cannot resolve %s", c.Host)
	}
	return net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(netipAddrPort(addresses[0], c.Port)))
}

func netipAddrPort(addr netip.Addr, port int) netip.AddrPort {
	return netip.AddrPortFrom(addr.Unmap(), uint16(port))
}

func (c *Client) udpPacket(kind byte, seq uint32, size int) []byte {
	packet := make([]byte, size)
	copy(packet, UDPMagic)
	packet[4], packet[5] = Version, kind
	copy(packet[6:22], c.token[:])
	binary.BigEndian.PutUint32(packet[22:26], seq)
	binary.BigEndian.PutUint64(packet[26:34], uint64(time.Now().UnixMicro()))
	if size > UDPHeaderLen {
		copy(packet[UDPHeaderLen:], clientPayload)
	}
	return packet
}

func (c *Client) sameToken(packet []byte) bool {
	return len(packet) >= UDPHeaderLen && string(packet[6:22]) == string(c.token[:])
}

// udpRead waits up to timeout for one datagram. It returns false on a
// timeout and an error when the port is closed.
func (c *Client) udpRead(conn *net.UDPConn, buffer []byte, timeout time.Duration) (int, bool, error) {
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	n, err := conn.Read(buffer)
	if err == nil {
		return n, true, nil
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return 0, false, nil
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return 0, false, fmt.Errorf("UDP port %d is closed; allow UDP %d in the firewall and security group", c.Port, c.Port)
	}
	return 0, false, err
}

// UDPPing sends echo requests 50 ms apart and waits for the replies.
func (c *Client) UDPPing(ctx context.Context, count int) (PingStats, error) {
	if count < 1 || count > 200 {
		count = 20
	}
	conn, err := c.openUDP(ctx)
	if err != nil {
		return PingStats{}, err
	}
	defer conn.Close()
	defer closeOnDone(ctx, conn)()
	sentAt := make([]atomic.Int64, count)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for seq := 0; seq < count; seq++ {
			sentAt[seq].Store(time.Now().UnixNano())
			if _, err := conn.Write(c.udpPacket(UDPEchoRequest, uint32(seq), 64)); err != nil {
				return
			}
			if !sleepCtx(ctx, 50*time.Millisecond) {
				return
			}
		}
	}()
	rtts := make([]float64, count)
	got := make([]bool, count)
	received := 0
	buffer := make([]byte, 2048)
	var sendingDone time.Time
	for received < count {
		select {
		case <-done:
			if sendingDone.IsZero() {
				sendingDone = time.Now()
			}
		default:
		}
		if !sendingDone.IsZero() && time.Since(sendingDone) > time.Second {
			break
		}
		n, ok, err := c.udpRead(conn, buffer, 100*time.Millisecond)
		if err != nil {
			<-done
			return PingStats{}, err
		}
		if ctx.Err() != nil {
			<-done
			return PingStats{}, ctx.Err()
		}
		if !ok || n < UDPHeaderLen || buffer[5] != UDPEchoReply || !c.sameToken(buffer[:n]) {
			continue
		}
		seq := int(binary.BigEndian.Uint32(buffer[22:26]))
		if seq < count && !got[seq] && sentAt[seq].Load() != 0 {
			rtts[seq] = float64(time.Now().UnixNano()-sentAt[seq].Load()) / 1e6
			got[seq] = true
			received++
		}
	}
	<-done
	var values []float64
	for i, ok := range got {
		if ok {
			values = append(values, rtts[i])
		}
	}
	return pingStats(count, values), nil
}

func clampDuration(duration time.Duration) time.Duration {
	if duration < time.Second {
		return time.Second
	}
	if duration > MaxDuration {
		return MaxDuration
	}
	return duration
}

func clampStreams(streams int) int {
	if streams < 1 {
		return 1
	}
	if streams > 8 {
		return 8
	}
	return streams
}

// warmup leaves TCP slow start out of a long enough result.
func warmup(duration time.Duration) time.Duration {
	if quarter := duration / 4; quarter < time.Second {
		return quarter
	}
	return time.Second
}

// TCPDownload receives over streams parallel connections for duration.
func (c *Client) TCPDownload(ctx context.Context, duration time.Duration, streams int) (ThroughputStats, error) {
	duration, streams = clampDuration(duration), clampStreams(streams)
	conns, err := c.openStreams(ctx, ModeDownload, duration, streams)
	if err != nil {
		return ThroughputStats{}, err
	}
	var total, lastByte atomic.Int64
	started := time.Now()
	var warm struct {
		sync.Mutex
		at    time.Time
		bytes int64
	}
	timer := time.AfterFunc(warmup(duration), func() {
		warm.Lock()
		warm.at, warm.bytes = time.Now(), total.Load()
		warm.Unlock()
	})
	defer timer.Stop()
	var wg sync.WaitGroup
	for _, conn := range conns {
		wg.Add(1)
		go func(conn net.Conn) {
			defer wg.Done()
			defer conn.Close()
			defer closeOnDone(ctx, conn)()
			buffer := make([]byte, 64<<10)
			for {
				n, err := conn.Read(buffer)
				if n > 0 {
					total.Add(int64(n))
					lastByte.Store(time.Now().UnixNano())
				}
				if err != nil {
					return
				}
			}
		}(conn)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return ThroughputStats{}, ctx.Err()
	}
	end := time.Now()
	if last := lastByte.Load(); last > 0 {
		end = time.Unix(0, last)
	}
	result := ThroughputStats{Bytes: total.Load(), Seconds: math.Max(end.Sub(started).Seconds(), 0.001), Streams: len(conns)}
	result.BitsPerSecond = float64(result.Bytes) * 8 / result.Seconds
	warm.Lock()
	if !warm.at.IsZero() && end.Sub(warm.at) > time.Second && result.Bytes > warm.bytes {
		result.BitsPerSecond = float64(result.Bytes-warm.bytes) * 8 / end.Sub(warm.at).Seconds()
	}
	warm.Unlock()
	if result.Bytes == 0 {
		return result, errors.New("no data arrived from the server")
	}
	return result, nil
}

func (c *Client) openStreams(ctx context.Context, mode byte, duration time.Duration, streams int) ([]net.Conn, error) {
	conns := make([]net.Conn, 0, streams)
	for i := 0; i < streams; i++ {
		conn, err := c.openTCP(ctx, mode, duration)
		if err != nil {
			for _, open := range conns {
				_ = open.Close()
			}
			return nil, err
		}
		conns = append(conns, conn)
	}
	return conns, nil
}

// TCPUpload sends over streams parallel connections for duration; the
// server reports what arrived and over how long.
func (c *Client) TCPUpload(ctx context.Context, duration time.Duration, streams int) (ThroughputStats, error) {
	duration, streams = clampDuration(duration), clampStreams(streams)
	conns, err := c.openStreams(ctx, ModeUpload, duration, streams)
	if err != nil {
		return ThroughputStats{}, err
	}
	deadline := time.Now().Add(duration)
	var received, span atomic.Int64
	var failures atomic.Int32
	var wg sync.WaitGroup
	for _, conn := range conns {
		wg.Add(1)
		go func(conn net.Conn) {
			defer wg.Done()
			defer conn.Close()
			defer closeOnDone(ctx, conn)()
			for time.Now().Before(deadline) {
				if _, err := conn.Write(clientPayload); err != nil {
					failures.Add(1)
					return
				}
			}
			if tcp, ok := conn.(*net.TCPConn); ok {
				_ = tcp.CloseWrite()
			}
			_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
			result := make([]byte, 16)
			if _, err := io.ReadFull(conn, result); err != nil {
				failures.Add(1)
				return
			}
			received.Add(int64(binary.BigEndian.Uint64(result[:8])))
			micros := int64(binary.BigEndian.Uint64(result[8:]))
			for {
				current := span.Load()
				if micros <= current || span.CompareAndSwap(current, micros) {
					break
				}
			}
		}(conn)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return ThroughputStats{}, ctx.Err()
	}
	if int(failures.Load()) == len(conns) {
		return ThroughputStats{}, errors.New("the server did not report the upload")
	}
	seconds := duration.Seconds()
	if micros := span.Load(); micros > 0 {
		seconds = float64(micros) / 1e6
	}
	result := ThroughputStats{Bytes: received.Load(), Seconds: seconds, Streams: len(conns)}
	result.BitsPerSecond = float64(result.Bytes) * 8 / seconds
	return result, nil
}

func clampRate(mbps int) int {
	if mbps < 1 {
		return 1
	}
	if mbps > MaxUDPRateKbps/1000 {
		return MaxUDPRateKbps / 1000
	}
	return mbps
}

// UDPDownload asks the server to send at rateMbps for duration and counts
// what arrives, keeping the stream alive with keepalives.
func (c *Client) UDPDownload(ctx context.Context, duration time.Duration, rateMbps int) (UDPStats, error) {
	duration, rateMbps = clampDuration(duration), clampRate(rateMbps)
	conn, err := c.openUDP(ctx)
	if err != nil {
		return UDPStats{}, err
	}
	defer conn.Close()
	defer closeOnDone(ctx, conn)()
	_ = conn.SetReadBuffer(4 << 20)
	start := c.udpPacket(UDPDownloadStart, 0, UDPHeaderLen+8)
	binary.BigEndian.PutUint32(start[UDPHeaderLen:], uint32(rateMbps*1000))
	binary.BigEndian.PutUint16(start[UDPHeaderLen+4:], clientPacketSize)
	binary.BigEndian.PutUint16(start[UDPHeaderLen+6:], uint16(duration/time.Millisecond))
	if _, err := conn.Write(start); err != nil {
		return UDPStats{}, err
	}
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		keepalive := c.udpPacket(UDPKeepalive, 0, UDPHeaderLen)
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				_, _ = conn.Write(keepalive)
			}
		}
	}()
	result := UDPStats{TargetMbps: rateMbps, SentPackets: -1}
	buffer := make([]byte, 2048)
	started := time.Now()
	var first, last, doneAt time.Time
	maxSeq := int64(-1)
	var jitter, prevTransit float64
	haveTransit := false
	timeout := 3 * time.Second
	for {
		n, ok, err := c.udpRead(conn, buffer, timeout)
		if err != nil {
			return UDPStats{}, err
		}
		if ctx.Err() != nil {
			return UDPStats{}, ctx.Err()
		}
		now := time.Now()
		if !ok {
			if result.ReceivedPackets == 0 {
				return UDPStats{}, fmt.Errorf("no UDP data arrived: UDP %d may be blocked by a firewall or the ISP", c.Port)
			}
			break
		}
		if n >= UDPHeaderLen && c.sameToken(buffer[:n]) {
			switch buffer[5] {
			case UDPDownloadData:
				seq := int64(binary.BigEndian.Uint32(buffer[22:26]))
				sentMicros := float64(binary.BigEndian.Uint64(buffer[26:34]))
				if first.IsZero() {
					first = now
				}
				last = now
				result.ReceivedPackets++
				result.Bytes += int64(n)
				if seq < maxSeq {
					result.OutOfOrder++
				} else {
					maxSeq = seq
				}
				transit := float64(now.Sub(started).Microseconds()) - sentMicros
				if haveTransit {
					jitter += (math.Abs(transit-prevTransit) - jitter) / 16
				}
				prevTransit, haveTransit = transit, true
			case UDPDownloadDone:
				if n >= UDPHeaderLen+4 {
					result.SentPackets = int64(binary.BigEndian.Uint32(buffer[UDPHeaderLen:]))
				}
				if doneAt.IsZero() {
					doneAt = now
				}
				timeout = 200 * time.Millisecond
			}
		}
		if !doneAt.IsZero() && now.Sub(doneAt) > 200*time.Millisecond {
			break
		}
	}
	if result.SentPackets < 0 {
		result.SentPackets = maxSeq + 1
	}
	result.Seconds = duration.Seconds()
	if last.After(first) {
		result.Seconds = last.Sub(first).Seconds()
	}
	result.BitsPerSecond = float64(result.Bytes) * 8 / result.Seconds
	result.JitterMs = jitter / 1000
	result.LossPct = lossPct(result.SentPackets, result.ReceivedPackets)
	return result, nil
}

// UDPUpload sends paced packets at rateMbps for duration and asks the
// server what arrived.
func (c *Client) UDPUpload(ctx context.Context, duration time.Duration, rateMbps int) (UDPStats, error) {
	duration, rateMbps = clampDuration(duration), clampRate(rateMbps)
	conn, err := c.openUDP(ctx)
	if err != nil {
		return UDPStats{}, err
	}
	defer conn.Close()
	defer closeOnDone(ctx, conn)()
	_ = conn.SetWriteBuffer(4 << 20)
	perSecond := float64(rateMbps) * 1e6 / 8 / clientPacketSize
	packet := c.udpPacket(UDPUploadData, 0, clientPacketSize)
	started := time.Now()
	end := started.Add(duration)
	var sent int64
	for {
		now := time.Now()
		if !now.Before(end) || ctx.Err() != nil {
			break
		}
		due := int64(now.Sub(started).Seconds()*perSecond) - sent
		burst := due
		if burst > 256 {
			burst = 256
		}
		for ; burst > 0; burst-- {
			binary.BigEndian.PutUint32(packet[22:26], uint32(sent))
			binary.BigEndian.PutUint64(packet[26:34], uint64(time.Since(started).Microseconds()))
			if _, err := conn.Write(packet); err != nil {
				if errors.Is(err, syscall.ECONNREFUSED) {
					return UDPStats{}, fmt.Errorf("UDP port %d is closed; allow UDP %d in the firewall and security group", c.Port, c.Port)
				}
				continue
			}
			sent++
		}
		if due < 64 {
			time.Sleep(time.Millisecond)
		}
	}
	if ctx.Err() != nil {
		return UDPStats{}, ctx.Err()
	}
	report := c.udpPacket(UDPUploadReport, 0, UDPUploadResultLen)
	buffer := make([]byte, 2048)
	for attempt := 0; attempt < 5; attempt++ {
		if _, err := conn.Write(report); err != nil {
			continue
		}
		n, ok, err := c.udpRead(conn, buffer, 700*time.Millisecond)
		if err != nil {
			return UDPStats{}, err
		}
		if !ok || n < UDPUploadResultLen || buffer[5] != UDPUploadResult || !c.sameToken(buffer[:n]) {
			continue
		}
		body := buffer[UDPHeaderLen:]
		result := UDPStats{
			TargetMbps: rateMbps, SentPackets: sent,
			ReceivedPackets: int64(binary.BigEndian.Uint32(body[0:4])),
			Bytes:           int64(binary.BigEndian.Uint64(body[4:12])),
			OutOfOrder:      int64(binary.BigEndian.Uint32(body[16:20])),
			JitterMs:        float64(binary.BigEndian.Uint32(body[20:24])) / 1000,
			Seconds:         duration.Seconds(),
		}
		if micros := binary.BigEndian.Uint64(body[24:32]); micros > 0 {
			result.Seconds = float64(micros) / 1e6
		}
		result.BitsPerSecond = float64(result.Bytes) * 8 / result.Seconds
		result.LossPct = lossPct(result.SentPackets, result.ReceivedPackets)
		return result, nil
	}
	return UDPStats{}, fmt.Errorf("the server did not report the UDP upload: UDP %d may be blocked by a firewall or the ISP", c.Port)
}

func lossPct(sent, received int64) float64 {
	if sent <= 0 {
		return 0
	}
	return math.Max(0, float64(sent-received)*100/float64(sent))
}

func pingStats(sent int, rtts []float64) PingStats {
	stats := PingStats{Sent: sent, Received: len(rtts)}
	if sent > 0 {
		stats.LossPct = float64(sent-len(rtts)) * 100 / float64(sent)
	}
	if len(rtts) == 0 {
		return stats
	}
	stats.MinMs, stats.MaxMs = rtts[0], rtts[0]
	var sum, jitter float64
	for i, rtt := range rtts {
		sum += rtt
		stats.MinMs = math.Min(stats.MinMs, rtt)
		stats.MaxMs = math.Max(stats.MaxMs, rtt)
		if i > 0 {
			jitter += math.Abs(rtt - rtts[i-1])
		}
	}
	stats.AvgMs = sum / float64(len(rtts))
	if len(rtts) > 1 {
		stats.JitterMs = jitter / float64(len(rtts)-1)
	}
	return stats
}

func sleepCtx(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
