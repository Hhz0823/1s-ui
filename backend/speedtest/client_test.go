package speedtest

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestClientAgainstServer runs every client test against the real server,
// the way a home relay measures its line to a VPS.
func TestClientAgainstServer(t *testing.T) {
	port := freePort(t)
	session, _ := startSession(t, port)
	ctx := context.Background()
	client, err := Connect(ctx, session.Token, session.Port, []string{"", "192.0.2.1", "[127.0.0.1]"})
	if err == nil && client.Host != "127.0.0.1" {
		t.Fatalf("connected to %s", client.Host)
	}
	if err != nil {
		// 192.0.2.1 is unroutable (TEST-NET-1); it may hang until the
		// 3-second timeout or fail at once, either way loopback answers.
		t.Fatal(err)
	}

	tcpPing, err := client.TCPPing(ctx, 5)
	if err != nil || tcpPing.Received != 5 || tcpPing.AvgMs <= 0 || tcpPing.LossPct != 0 {
		t.Fatalf("tcp ping: %+v %v", tcpPing, err)
	}
	udpPing, err := client.UDPPing(ctx, 10)
	if err != nil || udpPing.Received != 10 || udpPing.MaxMs < udpPing.MinMs {
		t.Fatalf("udp ping: %+v %v", udpPing, err)
	}
	download, err := client.TCPDownload(ctx, 2*time.Second, 2)
	if err != nil || download.Bytes == 0 || download.BitsPerSecond < 10e6 || download.Streams != 2 {
		t.Fatalf("tcp download: %+v %v", download, err)
	}
	upload, err := client.TCPUpload(ctx, 2*time.Second, 2)
	if err != nil || upload.Bytes == 0 || upload.BitsPerSecond < 10e6 {
		t.Fatalf("tcp upload: %+v %v", upload, err)
	}
	udpDown, err := client.UDPDownload(ctx, 2*time.Second, 20)
	if err != nil || udpDown.ReceivedPackets == 0 || udpDown.SentPackets < udpDown.ReceivedPackets || udpDown.LossPct > 5 {
		t.Fatalf("udp download: %+v %v", udpDown, err)
	}
	// 20 Mbit/s for 2 s is about 4 MB, give or take the pacing.
	if mbps := udpDown.BitsPerSecond / 1e6; mbps < 12 || mbps > 30 {
		t.Fatalf("udp download ran at %.1f Mbit/s", mbps)
	}
	udpUp, err := client.UDPUpload(ctx, 2*time.Second, 20)
	if err != nil || udpUp.ReceivedPackets == 0 || udpUp.SentPackets < udpUp.ReceivedPackets || udpUp.LossPct > 5 {
		t.Fatalf("udp upload: %+v %v", udpUp, err)
	}

	// A cancelled test stops promptly.
	cancelled, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := client.TCPDownload(cancelled, 10*time.Second, 1); err == nil {
		t.Fatal("cancelled download returned no error")
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("cancel took %s", elapsed)
	}
}

func TestClientReportsBadSessions(t *testing.T) {
	port := freePort(t)
	session, _ := startSession(t, port)
	ctx := context.Background()
	if _, err := Connect(ctx, "zz", session.Port, []string{"127.0.0.1"}); err == nil {
		t.Fatal("accepted a malformed token")
	}
	stranger, err := Connect(ctx, strings.Repeat("ab", 16), session.Port, []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stranger.TCPPing(ctx, 3); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("unknown token: %v", err)
	}
	closed := freePort(t)
	if _, err := Connect(ctx, session.Token, closed, []string{"127.0.0.1"}); err == nil || !strings.Contains(err.Error(), "cannot reach") {
		t.Fatalf("closed port: %v", err)
	}
	if _, err := Connect(ctx, session.Token, session.Port, nil); err == nil {
		t.Fatal("connected without an address")
	}
}
