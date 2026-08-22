package core

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
)

func TestStatsTrackerTCPDirectionsAndExtendedConn(t *testing.T) {
	tracker := NewStatsTracker()
	local, peer := net.Pipe()
	defer peer.Close()
	wrapped := tracker.wrapConnection(context.Background(), local, "tcp-in", "", "")
	defer wrapped.Close()
	extended, ok := wrapped.(network.ExtendedConn)
	if !ok {
		t.Fatalf("limited connection lost network.ExtendedConn: %T", wrapped)
	}

	readDone := make(chan error, 1)
	go func() {
		payload := make([]byte, 4)
		_, err := io.ReadFull(peer, payload)
		readDone <- err
	}()
	if err := extended.WriteBuffer(buf.As([]byte("down"))); err != nil {
		t.Fatal(err)
	}
	if err := <-readDone; err != nil {
		t.Fatal(err)
	}

	writeDone := make(chan error, 1)
	go func() {
		_, err := peer.Write([]byte("up"))
		writeDone <- err
	}()
	readBuffer := buf.NewSize(2)
	defer readBuffer.Release()
	if err := extended.ReadBuffer(readBuffer); err != nil {
		t.Fatal(err)
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}

	tracker.snapshotAccess.Lock()
	sampleAt := tracker.lastSample.Add(time.Second)
	tracker.snapshotAccess.Unlock()
	_, snapshot := tracker.inboundTrafficSnapshotAt(sampleAt)
	traffic := snapshot["tcp-in"]
	if traffic.UploadSession != 2 || traffic.DownloadSession != 4 || traffic.UploadBPS != 2 || traffic.DownloadBPS != 4 {
		t.Fatalf("unexpected TCP traffic directions: %#v", traffic)
	}
	_ = tracker.GetStats()
	_, snapshot = tracker.inboundTrafficSnapshotAt(sampleAt.Add(500 * time.Millisecond))
	traffic = snapshot["tcp-in"]
	if traffic.UploadPending != 0 || traffic.DownloadPending != 0 || traffic.UploadSession != 2 || traffic.DownloadSession != 4 {
		t.Fatalf("pending/session counters were mixed: %#v", traffic)
	}
}

func TestStatsTrackerUDPDirections(t *testing.T) {
	local, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	peer, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	tracker := NewStatsTracker()
	wrapped := tracker.wrapPacketConnection(context.Background(), bufio.NewPacketConn(local), "udp-in", "", "")
	defer wrapped.Close()
	if err := wrapped.WritePacket(buf.As([]byte("down")), M.SocksaddrFromNet(peer.LocalAddr()).Unwrap()); err != nil {
		t.Fatal(err)
	}
	received := make([]byte, 8)
	if n, _, err := peer.ReadFrom(received); err != nil || n != 4 {
		t.Fatalf("read UDP download: n=%d err=%v", n, err)
	}
	if _, err := peer.WriteTo([]byte("up"), local.LocalAddr()); err != nil {
		t.Fatal(err)
	}
	readBuffer := buf.NewPacket()
	defer readBuffer.Release()
	if _, err := wrapped.ReadPacket(readBuffer); err != nil {
		t.Fatal(err)
	}
	_, snapshot := tracker.InboundTrafficSnapshot()
	traffic := snapshot["udp-in"]
	if traffic.UploadSession != 2 || traffic.DownloadSession != 4 {
		t.Fatalf("unexpected UDP traffic directions: %#v", traffic)
	}
}

func TestStatsTrackerSharesUpdatedLimiterAcrossConnections(t *testing.T) {
	tracker := NewStatsTracker()
	tracker.SetInboundLimit("shared", 1000, 2000)
	left1, right1 := net.Pipe()
	defer left1.Close()
	defer right1.Close()
	left2, right2 := net.Pipe()
	defer left2.Close()
	defer right2.Close()
	first := tracker.wrapConnection(context.Background(), left1, "shared", "", "").(*limitedConn)
	second := tracker.wrapConnection(context.Background(), left2, "shared", "", "").(*limitedConn)
	if first.limiter != second.limiter {
		t.Fatal("connections on one inbound received separate limiters")
	}
	tracker.SetInboundLimit("shared", 1234, 5678)
	if first.limiter.uploadLimit.Load() != 1234 || second.limiter.downloadLimit.Load() != 5678 {
		t.Fatal("existing connections did not observe the updated shared limit")
	}
	tracker.RemoveInboundLimit("shared")
	if first.limiter.upload.Load() != nil || second.limiter.download.Load() != nil {
		t.Fatal("existing connections retained a removed limit")
	}
}

func TestStatsTrackerKeepsUnlimitedConnectionsOnCounterFastPath(t *testing.T) {
	tracker := NewStatsTracker()
	local, peer := net.Pipe()
	defer peer.Close()
	wrapper := tracker.wrapConnection(context.Background(), local, "unlimited", "", "")
	defer wrapper.Close()
	if _, limited := wrapper.(*limitedConn); limited {
		t.Fatal("unlimited inbound received a limiter wrapper")
	}
}

func TestStatsTrackerAppliesSharedTCPDownloadLimit(t *testing.T) {
	tracker := NewStatsTracker()
	tracker.SetInboundLimit("limited", 0, 64*1024)
	local, peer := net.Pipe()
	defer peer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wrapped := tracker.wrapConnection(ctx, local, "limited", "", "")
	defer wrapped.Close()
	readDone := make(chan error, 1)
	go func() {
		_, err := io.CopyN(io.Discard, peer, 128*1024)
		readDone <- err
	}()
	started := time.Now()
	if _, err := wrapped.Write(bytes.Repeat([]byte{'x'}, 128*1024)); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(started)
	if err := <-readDone; err != nil {
		t.Fatal(err)
	}
	if elapsed < 800*time.Millisecond || elapsed > 4*time.Second {
		t.Fatalf("64 KiB/s limiter took %s for 128 KiB", elapsed)
	}
}
