// Package speedtest is the server side of the monitor app's speed test. The
// panel opens one port for TCP and UDP only while a test session is active,
// and the port answers only clients that present the session's token, so it
// cannot be used to send traffic to anyone who did not ask for it.
//
// TCP: the client sends a 24-byte header (magic "1SST", version, mode,
// duration in ms, token) and gets one status byte back. Download mode then
// streams data until the duration ends; upload mode counts what the client
// sends and answers with the byte count and the time it took; echo mode
// returns every byte for round-trip timing.
//
// UDP: every packet starts with a 34-byte header (magic "1SSU", version,
// type, token, sequence number, sender timestamp in microseconds). Echo
// packets come back unchanged; upload packets are counted and reported on
// request; a download sends paced packets to the requesting address for as
// long as that address keeps sending keepalives. Uploads and downloads are
// told apart by the client's address, so each test uses a new local port.
package speedtest

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/netip"
	"sync"
	"time"
)

const (
	DefaultPort = 5201

	// MaxDuration caps one test.
	MaxDuration = 15 * time.Second
	// MaxUDPRateKbps caps the UDP download rate (1 Gbit/s).
	MaxUDPRateKbps = 1_000_000

	maxSessions        = 4
	maxConnsPerSession = 16
	maxUDPSenders      = 2
	maxEchoBytes       = 1 << 20
)

const (
	TCPMagic = "1SST"
	UDPMagic = "1SSU"
	Version  = 1

	TCPHeaderLen = 24
	UDPHeaderLen = 34

	// MaxUDPPacket keeps packets under the IPv6 minimum MTU.
	MaxUDPPacket = 1200
	minUDPPacket = 64
)

// TCP modes and status bytes.
const (
	ModeDownload byte = 1
	ModeUpload   byte = 2
	ModeEcho     byte = 3

	StatusOK         byte = 0
	StatusBadToken   byte = 1
	StatusBusy       byte = 2
	StatusBadRequest byte = 3
)

// UDP packet types.
const (
	UDPEchoRequest   byte = 1
	UDPEchoReply     byte = 2
	UDPUploadData    byte = 3
	UDPUploadReport  byte = 4
	UDPUploadResult  byte = 5
	UDPDownloadStart byte = 6
	UDPDownloadData  byte = 7
	UDPKeepalive     byte = 8
	UDPDownloadDone  byte = 9
)

// UDPUploadResultLen is the size of an upload result; report requests must
// be at least this long so a reply is never larger than its request.
const UDPUploadResultLen = UDPHeaderLen + 32

var (
	// SessionTTL is how long a token can start new tests.
	SessionTTL = 2 * time.Minute
	// keepaliveTimeout stops a UDP download when its client goes quiet.
	keepaliveTimeout = time.Second
	// janitorInterval is how often an idle port is checked for closing.
	janitorInterval = 5 * time.Second
)

// Session is what a client needs to run tests.
type Session struct {
	Token      string `json:"token"`
	Port       int    `json:"port"`
	ExpiresAt  int64  `json:"expires_at"`
	MaxSeconds int    `json:"max_seconds"`
	MaxUDPMbps int    `json:"max_udp_mbps"`
}

type session struct {
	token   [16]byte
	expires time.Time
	conns   int
	uploads map[netip.AddrPort]*udpUpload
	sending map[netip.AddrPort]*udpDownload
}

type udpUpload struct {
	packets, maxSeq, outOfOrder, lastSeq uint32
	bytes                                uint64
	first, last                          time.Time
	jitter, prevTransit                  float64
	haveTransit                          bool
}

type udpDownload struct {
	lastHeard time.Time
}

type server struct {
	mu       sync.Mutex
	port     int
	tcp      net.Listener
	udp      *net.UDPConn
	sessions map[[16]byte]*session
	active   int // TCP connections and UDP senders still running
	senders  int // UDP downloads in progress
	janitor  *time.Timer
	started  time.Time
}

var global = &server{sessions: map[[16]byte]*session{}}

// Start opens the speed test port if needed and issues a new session.
func Start(port int) (*Session, error) {
	return global.start(port, time.Now())
}

// Running reports the port the server listens on, or 0.
func Running() int {
	global.mu.Lock()
	defer global.mu.Unlock()
	if global.tcp == nil {
		return 0
	}
	return global.port
}

// Stop closes the listeners and drops every session.
func Stop() {
	global.mu.Lock()
	defer global.mu.Unlock()
	global.closeLocked()
	global.sessions = map[[16]byte]*session{}
}

func (s *server) start(port int, now time.Time) (*Session, error) {
	if port == 0 {
		port = DefaultPort
	}
	if port < 1 || port > 65535 {
		return nil, errors.New("speed test port must be between 1 and 65535")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(now)
	if s.tcp != nil && s.port != port {
		if len(s.sessions) > 0 || s.active > 0 {
			return nil, fmt.Errorf("a speed test is running on port %d; try again shortly", s.port)
		}
		s.closeLocked()
	}
	if len(s.sessions) >= maxSessions {
		return nil, errors.New("too many speed tests at once; try again shortly")
	}
	if s.tcp == nil {
		if err := s.listenLocked(port); err != nil {
			return nil, err
		}
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return nil, err
	}
	expires := now.Add(SessionTTL)
	s.sessions[token] = &session{
		token: token, expires: expires,
		uploads: map[netip.AddrPort]*udpUpload{}, sending: map[netip.AddrPort]*udpDownload{},
	}
	s.scheduleJanitorLocked()
	return &Session{
		Token: hex.EncodeToString(token[:]), Port: port, ExpiresAt: expires.Unix(),
		MaxSeconds: int(MaxDuration / time.Second), MaxUDPMbps: MaxUDPRateKbps / 1000,
	}, nil
}

func (s *server) listenLocked(port int) error {
	tcp, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("speed test port %d is not available: %w", port, err)
	}
	udp, err := net.ListenUDP("udp", &net.UDPAddr{Port: port})
	if err != nil {
		_ = tcp.Close()
		return fmt.Errorf("speed test port %d is not available for UDP: %w", port, err)
	}
	_ = udp.SetReadBuffer(4 << 20)
	_ = udp.SetWriteBuffer(4 << 20)
	s.tcp, s.udp, s.port, s.started = tcp, udp, port, time.Now()
	go s.acceptTCP(tcp)
	go s.readUDP(udp)
	return nil
}

func (s *server) closeLocked() {
	if s.tcp != nil {
		_ = s.tcp.Close()
		s.tcp = nil
	}
	if s.udp != nil {
		_ = s.udp.Close()
		s.udp = nil
	}
	if s.janitor != nil {
		s.janitor.Stop()
		s.janitor = nil
	}
}

func (s *server) pruneLocked(now time.Time) {
	for token, sess := range s.sessions {
		if now.After(sess.expires) && sess.conns == 0 && len(sess.sending) == 0 {
			delete(s.sessions, token)
		}
	}
}

// scheduleJanitorLocked closes the port once every session has expired and
// the last test has finished.
func (s *server) scheduleJanitorLocked() {
	if s.janitor != nil {
		return
	}
	s.janitor = time.AfterFunc(janitorInterval, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.janitor = nil
		s.pruneLocked(time.Now())
		if len(s.sessions) == 0 && s.active == 0 {
			s.closeLocked()
			return
		}
		s.scheduleJanitorLocked()
	})
}

// lookupLocked finds a session that may still start tests.
func (s *server) lookupLocked(token []byte, now time.Time) *session {
	for key, sess := range s.sessions {
		if subtle.ConstantTimeCompare(key[:], token) == 1 {
			if now.After(sess.expires) {
				return nil
			}
			return sess
		}
	}
	return nil
}

func (s *server) acceptTCP(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go s.serveTCP(conn)
	}
}

func (s *server) serveTCP(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	header := make([]byte, TCPHeaderLen)
	if _, err := io.ReadFull(conn, header); err != nil {
		return
	}
	if string(header[:4]) != TCPMagic || header[4] != Version {
		_, _ = conn.Write([]byte{StatusBadRequest})
		return
	}
	mode := header[5]
	duration := time.Duration(binary.BigEndian.Uint16(header[6:8])) * time.Millisecond
	if duration <= 0 || duration > MaxDuration {
		duration = MaxDuration
	}
	s.mu.Lock()
	sess := s.lookupLocked(header[8:24], time.Now())
	status := StatusOK
	switch {
	case sess == nil:
		status = StatusBadToken
	case sess.conns >= maxConnsPerSession:
		status = StatusBusy
	case mode != ModeDownload && mode != ModeUpload && mode != ModeEcho:
		status = StatusBadRequest
	default:
		sess.conns++
		s.active++
	}
	s.mu.Unlock()
	if _, err := conn.Write([]byte{status}); err != nil || status != StatusOK {
		if status == StatusOK {
			s.release(sess)
		}
		return
	}
	defer s.release(sess)

	switch mode {
	case ModeDownload:
		serveDownload(conn, duration)
	case ModeUpload:
		serveUpload(conn, duration)
	case ModeEcho:
		_ = conn.SetDeadline(time.Now().Add(duration + 2*time.Second))
		_, _ = io.CopyN(conn, conn, maxEchoBytes)
	}
}

func (s *server) release(sess *session) {
	s.mu.Lock()
	sess.conns--
	s.active--
	s.mu.Unlock()
}

// payload is sent by downloads; random bytes keep compression on the path
// from flattering the result.
var payload = func() []byte {
	buf := make([]byte, 128<<10)
	_, _ = rand.Read(buf)
	return buf
}()

func serveDownload(conn net.Conn, duration time.Duration) {
	end := time.Now().Add(duration)
	_ = conn.SetDeadline(end.Add(5 * time.Second))
	for time.Now().Before(end) {
		if _, err := conn.Write(payload); err != nil {
			return
		}
	}
}

// serveUpload counts bytes until the client closes its side, then reports
// the count and the time between the first and the last byte.
func serveUpload(conn net.Conn, duration time.Duration) {
	_ = conn.SetDeadline(time.Now().Add(duration + 5*time.Second))
	buf := make([]byte, 64<<10)
	var total uint64
	var first, last time.Time
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			now := time.Now()
			if first.IsZero() {
				first = now
			}
			last = now
			total += uint64(n)
		}
		if err != nil {
			break
		}
	}
	result := make([]byte, 16)
	binary.BigEndian.PutUint64(result[:8], total)
	if !first.IsZero() {
		binary.BigEndian.PutUint64(result[8:], uint64(last.Sub(first).Microseconds()))
	}
	_, _ = conn.Write(result)
}

func (s *server) readUDP(conn *net.UDPConn) {
	buf := make([]byte, 2048)
	for {
		n, from, err := conn.ReadFromUDPAddrPort(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		if n < UDPHeaderLen || string(buf[:4]) != UDPMagic || buf[4] != Version {
			continue
		}
		s.handleUDP(conn, buf[:n], netip.AddrPortFrom(from.Addr().Unmap(), from.Port()))
	}
}

func (s *server) handleUDP(conn *net.UDPConn, packet []byte, from netip.AddrPort) {
	now := time.Now()
	kind := packet[5]
	s.mu.Lock()
	sess := s.lookupLocked(packet[6:22], now)
	if sess == nil {
		// A running download keeps listening to keepalives after expiry.
		if kind == UDPKeepalive {
			for _, candidate := range s.sessions {
				if subtle.ConstantTimeCompare(candidate.token[:], packet[6:22]) == 1 {
					if download := candidate.sending[from]; download != nil {
						download.lastHeard = now
					}
				}
			}
		}
		s.mu.Unlock()
		return
	}
	switch kind {
	case UDPEchoRequest:
		s.mu.Unlock()
		reply := append([]byte(nil), packet...)
		reply[5] = UDPEchoReply
		_, _ = conn.WriteToUDPAddrPort(reply, from)
	case UDPUploadData:
		upload := sess.uploads[from]
		if upload == nil {
			if len(sess.uploads) >= maxConnsPerSession {
				s.mu.Unlock()
				return
			}
			upload = &udpUpload{}
			sess.uploads[from] = upload
		}
		upload.record(packet, now, s.started)
		s.mu.Unlock()
	case UDPUploadReport:
		upload := sess.uploads[from]
		var stats udpUpload
		if upload != nil {
			stats = *upload
		}
		s.mu.Unlock()
		if len(packet) < UDPUploadResultLen {
			return
		}
		reply := make([]byte, UDPUploadResultLen)
		copy(reply, packet[:UDPHeaderLen])
		reply[5] = UDPUploadResult
		body := reply[UDPHeaderLen:]
		binary.BigEndian.PutUint32(body[0:4], stats.packets)
		binary.BigEndian.PutUint64(body[4:12], stats.bytes)
		binary.BigEndian.PutUint32(body[12:16], stats.maxSeq)
		binary.BigEndian.PutUint32(body[16:20], stats.outOfOrder)
		binary.BigEndian.PutUint32(body[20:24], uint32(math.Min(stats.jitter, math.MaxUint32)))
		if !stats.first.IsZero() {
			binary.BigEndian.PutUint64(body[24:32], uint64(stats.last.Sub(stats.first).Microseconds()))
		}
		_, _ = conn.WriteToUDPAddrPort(reply, from)
	case UDPDownloadStart:
		if len(packet) < UDPHeaderLen+8 || sess.sending[from] != nil || s.senders >= maxUDPSenders {
			s.mu.Unlock()
			return
		}
		params := packet[UDPHeaderLen:]
		rateKbps := int(binary.BigEndian.Uint32(params[0:4]))
		size := int(binary.BigEndian.Uint16(params[4:6]))
		duration := time.Duration(binary.BigEndian.Uint16(params[6:8])) * time.Millisecond
		if rateKbps <= 0 || rateKbps > MaxUDPRateKbps {
			rateKbps = MaxUDPRateKbps
		}
		if size < minUDPPacket || size > MaxUDPPacket {
			size = MaxUDPPacket
		}
		if duration <= 0 || duration > MaxDuration {
			duration = MaxDuration
		}
		download := &udpDownload{lastHeard: now}
		sess.sending[from] = download
		s.active++
		s.senders++
		s.mu.Unlock()
		go s.sendUDP(conn, sess, from, download, packet[:UDPHeaderLen], rateKbps, size, duration)
	case UDPKeepalive:
		if download := sess.sending[from]; download != nil {
			download.lastHeard = now
		}
		s.mu.Unlock()
	default:
		s.mu.Unlock()
	}
}

func (u *udpUpload) record(packet []byte, now, epoch time.Time) {
	seq := binary.BigEndian.Uint32(packet[22:26])
	sent := float64(binary.BigEndian.Uint64(packet[26:34]))
	if u.packets == 0 {
		u.first = now
	} else if seq < u.lastSeq {
		u.outOfOrder++
	}
	u.packets++
	u.bytes += uint64(len(packet))
	u.last = now
	u.lastSeq = seq
	if seq > u.maxSeq {
		u.maxSeq = seq
	}
	// RFC 3550 interarrival jitter; the clocks differ but only changes in
	// transit time matter.
	transit := float64(now.Sub(epoch).Microseconds()) - sent
	if u.haveTransit {
		u.jitter += (math.Abs(transit-u.prevTransit) - u.jitter) / 16
	}
	u.prevTransit, u.haveTransit = transit, true
}

// sendUDP paces a download and stops early when the client stops sending
// keepalives, so a forged start request cannot aim the stream elsewhere.
func (s *server) sendUDP(conn *net.UDPConn, sess *session, to netip.AddrPort, download *udpDownload, header []byte, rateKbps, size int, duration time.Duration) {
	defer func() {
		s.mu.Lock()
		delete(sess.sending, to)
		s.active--
		s.senders--
		s.mu.Unlock()
	}()
	packet := make([]byte, size)
	copy(packet, header)
	copy(packet[UDPHeaderLen:], payload)
	packet[5] = UDPDownloadData
	perSecond := float64(rateKbps) * 1000 / 8 / float64(size)
	start := time.Now()
	var sent uint32
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for now := range ticker.C {
		elapsed := now.Sub(start)
		if elapsed >= duration {
			break
		}
		s.mu.Lock()
		quiet := now.Sub(download.lastHeard) > keepaliveTimeout
		s.mu.Unlock()
		if quiet {
			break
		}
		due := uint32(elapsed.Seconds()*perSecond) - sent
		if due > 2048 {
			due = 2048
		}
		for i := uint32(0); i < due; i++ {
			binary.BigEndian.PutUint32(packet[22:26], sent)
			binary.BigEndian.PutUint64(packet[26:34], uint64(time.Since(start).Microseconds()))
			if _, err := conn.WriteToUDPAddrPort(packet, to); err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
			}
			sent++
		}
	}
	done := make([]byte, UDPHeaderLen+4)
	copy(done, header)
	done[5] = UDPDownloadDone
	binary.BigEndian.PutUint32(done[UDPHeaderLen:], sent)
	for i := 0; i < 3; i++ {
		_, _ = conn.WriteToUDPAddrPort(done, to)
		time.Sleep(20 * time.Millisecond)
	}
}
