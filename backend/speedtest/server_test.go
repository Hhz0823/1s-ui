package speedtest

import (
	"encoding/binary"
	"encoding/hex"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The helpers below are a minimal client written from the protocol notes in
// server.go; the Android app implements the same steps.

func freePort(t *testing.T) int {
	t.Helper()
	tcp, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	port := tcp.Addr().(*net.TCPAddr).Port
	_ = tcp.Close()
	return port
}

// setLocked changes a tunable the way the server reads it: under its lock.
func setLocked(change func()) {
	global.mu.Lock()
	defer global.mu.Unlock()
	change()
}

func startSession(t *testing.T, port int) (*Session, []byte) {
	t.Helper()
	session, err := Start(port)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(Stop)
	token, err := hex.DecodeString(session.Token)
	if err != nil || len(token) != 16 {
		t.Fatalf("token %q", session.Token)
	}
	return session, token
}

func tcpHeader(mode byte, duration time.Duration, token []byte) []byte {
	header := []byte(TCPMagic)
	header = append(header, Version, mode)
	header = binary.BigEndian.AppendUint16(header, uint16(duration/time.Millisecond))
	return append(header, token...)
}

func dialTest(t *testing.T, port int, mode byte, duration time.Duration, token []byte) (net.Conn, byte) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	if _, err := conn.Write(tcpHeader(mode, duration, token)); err != nil {
		t.Fatal(err)
	}
	status := make([]byte, 1)
	if _, err := io.ReadFull(conn, status); err != nil {
		t.Fatal(err)
	}
	return conn, status[0]
}

func TestTCPDownloadUploadAndEcho(t *testing.T) {
	port := freePort(t)
	session, token := startSession(t, port)
	if session.Port != port || Running() != port || session.MaxSeconds != 15 {
		t.Fatalf("session %+v, running %d", session, Running())
	}

	conn, status := dialTest(t, port, ModeDownload, 500*time.Millisecond, token)
	if status != StatusOK {
		t.Fatalf("download status %d", status)
	}
	started := time.Now()
	received, _ := io.Copy(io.Discard, conn)
	conn.Close()
	if elapsed := time.Since(started); received < 1<<20 || elapsed < 400*time.Millisecond || elapsed > 5*time.Second {
		t.Fatalf("download got %d bytes in %v", received, elapsed)
	}

	conn, status = dialTest(t, port, ModeUpload, time.Second, token)
	if status != StatusOK {
		t.Fatalf("upload status %d", status)
	}
	chunk := make([]byte, 32<<10)
	var sent uint64
	for end := time.Now().Add(300 * time.Millisecond); time.Now().Before(end); {
		n, err := conn.Write(chunk)
		if err != nil {
			t.Fatal(err)
		}
		sent += uint64(n)
	}
	_ = conn.(*net.TCPConn).CloseWrite()
	result := make([]byte, 16)
	if _, err := io.ReadFull(conn, result); err != nil {
		t.Fatal(err)
	}
	conn.Close()
	if got := binary.BigEndian.Uint64(result[:8]); got != sent {
		t.Fatalf("server counted %d of %d bytes", got, sent)
	}
	if micros := binary.BigEndian.Uint64(result[8:]); micros == 0 || micros > 5_000_000 {
		t.Fatalf("upload span %dµs", micros)
	}

	conn, status = dialTest(t, port, ModeEcho, time.Second, token)
	if status != StatusOK {
		t.Fatalf("echo status %d", status)
	}
	ping := make([]byte, 8)
	for i := 0; i < 5; i++ {
		binary.BigEndian.PutUint64(ping, uint64(i))
		if _, err := conn.Write(ping); err != nil {
			t.Fatal(err)
		}
		pong := make([]byte, 8)
		if _, err := io.ReadFull(conn, pong); err != nil || binary.BigEndian.Uint64(pong) != uint64(i) {
			t.Fatalf("echo %d: %v %x", i, err, pong)
		}
	}
	conn.Close()
}

func TestTCPRejectsWrongTokenAndRequests(t *testing.T) {
	port := freePort(t)
	_, token := startSession(t, port)
	wrong := append([]byte(nil), token...)
	wrong[0] ^= 0xff
	conn, status := dialTest(t, port, ModeDownload, time.Second, wrong)
	if status != StatusBadToken {
		t.Fatalf("wrong token status %d", status)
	}
	if n, _ := io.Copy(io.Discard, conn); n != 0 {
		t.Fatalf("sent %d bytes to a client without a token", n)
	}
	conn.Close()
	conn, status = dialTest(t, port, 9, time.Second, token)
	if status != StatusBadRequest {
		t.Fatalf("unknown mode status %d", status)
	}
	conn.Close()

	raw, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = raw.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n"))
	reply, _ := io.ReadAll(raw)
	raw.Close()
	if len(reply) != 1 || reply[0] != StatusBadRequest {
		t.Fatalf("HTTP request got %q", reply)
	}
}

func udpPacket(kind byte, token []byte, seq uint32, size int) []byte {
	packet := make([]byte, size)
	copy(packet, UDPMagic)
	packet[4] = Version
	packet[5] = kind
	copy(packet[6:22], token)
	binary.BigEndian.PutUint32(packet[22:26], seq)
	binary.BigEndian.PutUint64(packet[26:34], uint64(time.Now().UnixMicro()))
	return packet
}

func dialUDP(t *testing.T, port int) *net.UDPConn {
	t.Helper()
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadBuffer(4 << 20)
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestUDPEchoAndUpload(t *testing.T) {
	port := freePort(t)
	_, token := startSession(t, port)
	conn := dialUDP(t, port)

	_, _ = conn.Write(udpPacket(UDPEchoRequest, token, 7, 64))
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	reply := make([]byte, 2048)
	n, err := conn.Read(reply)
	if err != nil || n != 64 || reply[5] != UDPEchoReply || binary.BigEndian.Uint32(reply[22:26]) != 7 {
		t.Fatalf("echo: %v %d %x", err, n, reply[:8])
	}

	wrong := append([]byte(nil), token...)
	wrong[3] ^= 1
	_, _ = conn.Write(udpPacket(UDPEchoRequest, wrong, 8, 64))
	_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, err := conn.Read(reply); err == nil {
		t.Fatal("answered a packet with a wrong token")
	}

	upload := dialUDP(t, port)
	for seq := uint32(0); seq < 500; seq++ {
		_, _ = upload.Write(udpPacket(UDPUploadData, token, seq, MaxUDPPacket))
		if seq%50 == 0 {
			time.Sleep(time.Millisecond)
		}
	}
	time.Sleep(50 * time.Millisecond)
	// A report request shorter than the result is ignored.
	_, _ = upload.Write(udpPacket(UDPUploadReport, token, 0, UDPHeaderLen))
	_ = upload.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, err := upload.Read(reply); err == nil {
		t.Fatal("a short report request got a larger reply")
	}
	_, _ = upload.Write(udpPacket(UDPUploadReport, token, 0, UDPUploadResultLen))
	_ = upload.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err = upload.Read(reply)
	if err != nil || n != UDPUploadResultLen || reply[5] != UDPUploadResult {
		t.Fatalf("report: %v %d", err, n)
	}
	body := reply[UDPHeaderLen:n]
	packets, bytes, maxSeq := binary.BigEndian.Uint32(body[0:4]), binary.BigEndian.Uint64(body[4:12]), binary.BigEndian.Uint32(body[12:16])
	if packets < 450 || bytes != uint64(packets)*MaxUDPPacket || maxSeq != 499 {
		t.Fatalf("upload stats: %d packets, %d bytes, max seq %d", packets, bytes, maxSeq)
	}
}

func receiveDownload(t *testing.T, conn *net.UDPConn, token []byte, keepalive bool) (received, sent uint32, elapsed time.Duration) {
	t.Helper()
	stop := make(chan struct{})
	defer close(stop)
	if keepalive {
		go func() {
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					_, _ = conn.Write(udpPacket(UDPKeepalive, token, 0, UDPHeaderLen))
				}
			}
		}()
	}
	started := time.Now()
	buf := make([]byte, 2048)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatalf("download read: %v after %d packets", err, received)
		}
		switch buf[5] {
		case UDPDownloadData:
			if n != 1000 {
				t.Fatalf("packet size %d", n)
			}
			received++
		case UDPDownloadDone:
			return received, binary.BigEndian.Uint32(buf[UDPHeaderLen:]), time.Since(started)
		}
	}
}

func startDownload(conn *net.UDPConn, token []byte, rateKbps uint32, duration time.Duration) {
	start := udpPacket(UDPDownloadStart, token, 0, UDPHeaderLen+8)
	params := start[UDPHeaderLen:]
	binary.BigEndian.PutUint32(params[0:4], rateKbps)
	binary.BigEndian.PutUint16(params[4:6], 1000)
	binary.BigEndian.PutUint16(params[6:8], uint16(duration/time.Millisecond))
	_, _ = conn.Write(start)
}

func TestUDPDownloadIsPacedAndNeedsKeepalives(t *testing.T) {
	old := keepaliveTimeout
	setLocked(func() { keepaliveTimeout = 400 * time.Millisecond })
	defer setLocked(func() { keepaliveTimeout = old })
	port := freePort(t)
	_, token := startSession(t, port)

	conn := dialUDP(t, port)
	startDownload(conn, token, 8_000, time.Second) // 8 Mbit/s of 1000-byte packets = 1000 packets/s
	received, sent, elapsed := receiveDownload(t, conn, token, true)
	if sent < 900 || sent > 1100 || received < sent*9/10 || elapsed < 900*time.Millisecond {
		t.Fatalf("paced download: received %d of %d in %v", received, sent, elapsed)
	}

	// Without keepalives the stream stops after keepaliveTimeout.
	quiet := dialUDP(t, port)
	startDownload(quiet, token, 8_000, 10*time.Second)
	received, sent, elapsed = receiveDownload(t, quiet, token, false)
	if elapsed > 2*time.Second || sent > 700 {
		t.Fatalf("download without keepalives ran %v and sent %d packets", elapsed, sent)
	}
}

func TestPortClosesWhenSessionsExpire(t *testing.T) {
	oldTTL, oldJanitor := SessionTTL, janitorInterval
	setLocked(func() { SessionTTL, janitorInterval = 200*time.Millisecond, 50*time.Millisecond })
	defer setLocked(func() { SessionTTL, janitorInterval = oldTTL, oldJanitor })
	port := freePort(t)
	_, token := startSession(t, port)
	if other := freePort(t); other != port {
		if _, err := Start(other); err == nil || !strings.Contains(err.Error(), "running on port") {
			t.Fatalf("second port while busy: %v", err)
		}
	}
	time.Sleep(400 * time.Millisecond)
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), time.Second)
	if err == nil {
		_, _ = conn.Write(tcpHeader(ModeDownload, time.Second, token))
		reply, _ := io.ReadAll(conn)
		conn.Close()
		if len(reply) > 1 {
			t.Fatalf("expired token still downloads %d bytes", len(reply))
		}
	}
	if Running() != 0 {
		t.Fatal("the port stayed open after the session expired")
	}
	if _, err := Start(-1); err == nil {
		t.Fatal("accepted a negative port")
	}
}
