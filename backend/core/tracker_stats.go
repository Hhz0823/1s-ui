package core

import (
	"context"
	"net"
	"sync"
	stdAtomic "sync/atomic"
	"time"

	"github.com/Hhz0823/1s-ui/database/model"

	"github.com/sagernet/sing-box/adapter"
	SAtomic "github.com/sagernet/sing/common/atomic"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
	"golang.org/x/time/rate"
)

const limiterBurst = 64 * 1024

type Counter struct {
	read       *SAtomic.Int64
	write      *SAtomic.Int64
	totalRead  *SAtomic.Int64
	totalWrite *SAtomic.Int64
}

type InboundBandwidthLimit struct {
	Upload   int64
	Download int64
}

type InboundTrafficSnapshot struct {
	UploadBPS       int64
	DownloadBPS     int64
	UploadPending   int64
	DownloadPending int64
	UploadSession   int64
	DownloadSession int64
	UploadLimit     int64
	DownloadLimit   int64
}

type trafficTotals struct {
	upload   int64
	download int64
}

type inboundLimiter struct {
	upload        stdAtomic.Pointer[rate.Limiter]
	download      stdAtomic.Pointer[rate.Limiter]
	uploadLimit   stdAtomic.Int64
	downloadLimit stdAtomic.Int64
}

func (l *inboundLimiter) set(upload, download int64) {
	l.uploadLimit.Store(upload)
	l.downloadLimit.Store(download)
	l.upload.Store(newLimiter(upload))
	l.download.Store(newLimiter(download))
}

func newLimiter(bytesPerSecond int64) *rate.Limiter {
	if bytesPerSecond <= 0 {
		return nil
	}
	burst := int64(limiterBurst)
	if bytesPerSecond < burst {
		burst = bytesPerSecond
	}
	return rate.NewLimiter(rate.Limit(bytesPerSecond), int(burst))
}

func waitForLimit(ctx context.Context, limiter *stdAtomic.Pointer[rate.Limiter], size int) error {
	for size > 0 {
		current := limiter.Load()
		if current == nil {
			return nil
		}
		chunk := size
		if chunk > current.Burst() {
			chunk = current.Burst()
		}
		if err := current.WaitN(ctx, chunk); err != nil {
			return err
		}
		size -= chunk
	}
	return nil
}

type StatsTracker struct {
	access    sync.RWMutex
	inbounds  map[string]Counter
	outbounds map[string]Counter
	users     map[string]Counter
	limiters  map[string]*inboundLimiter

	snapshotAccess sync.Mutex
	lastSample     time.Time
	lastTotals     map[string]trafficTotals
	cachedBPS      map[string]trafficTotals
}

func NewStatsTracker() *StatsTracker {
	now := time.Now()
	return &StatsTracker{
		inbounds:   make(map[string]Counter),
		outbounds:  make(map[string]Counter),
		users:      make(map[string]Counter),
		limiters:   make(map[string]*inboundLimiter),
		lastSample: now,
		lastTotals: make(map[string]trafficTotals),
		cachedBPS:  make(map[string]trafficTotals),
	}
}

func (c *StatsTracker) Reset() {
	c.access.Lock()
	c.inbounds = make(map[string]Counter)
	c.outbounds = make(map[string]Counter)
	c.users = make(map[string]Counter)
	c.limiters = make(map[string]*inboundLimiter)
	c.access.Unlock()

	now := time.Now()
	c.snapshotAccess.Lock()
	c.lastSample = now
	c.lastTotals = make(map[string]trafficTotals)
	c.cachedBPS = make(map[string]trafficTotals)
	c.snapshotAccess.Unlock()
}

func (c *StatsTracker) SetInboundLimit(tag string, upload, download int64) {
	if tag == "" {
		return
	}
	if upload <= 0 && download <= 0 {
		c.RemoveInboundLimit(tag)
		return
	}
	c.access.Lock()
	limiter := c.loadOrCreateLimiter(tag)
	c.access.Unlock()
	limiter.set(upload, download)
}

func (c *StatsTracker) RemoveInboundLimit(tag string) {
	c.access.Lock()
	limiter := c.limiters[tag]
	delete(c.limiters, tag)
	c.access.Unlock()
	if limiter != nil {
		limiter.set(0, 0)
	}
}

func (c *StatsTracker) SyncInboundLimits(limits map[string]InboundBandwidthLimit) {
	c.access.Lock()
	states := make(map[string]*inboundLimiter, len(c.limiters)+len(limits))
	for tag, limiter := range c.limiters {
		states[tag] = limiter
		limit, exists := limits[tag]
		if !exists || (limit.Upload <= 0 && limit.Download <= 0) {
			delete(c.limiters, tag)
		}
	}
	for tag, limit := range limits {
		if limit.Upload <= 0 && limit.Download <= 0 {
			continue
		}
		states[tag] = c.loadOrCreateLimiter(tag)
	}
	c.access.Unlock()
	for tag, limiter := range states {
		limit := limits[tag]
		limiter.set(limit.Upload, limit.Download)
	}
}

func (c *StatsTracker) loadOrCreateLimiter(tag string) *inboundLimiter {
	limiter := c.limiters[tag]
	if limiter == nil {
		limiter = &inboundLimiter{}
		c.limiters[tag] = limiter
	}
	return limiter
}

func (c *StatsTracker) tracking(inbound, outbound, user string) ([]*SAtomic.Int64, []*SAtomic.Int64, *inboundLimiter) {
	var readCounter []*SAtomic.Int64
	var writeCounter []*SAtomic.Int64
	var limiter *inboundLimiter
	c.access.Lock()
	defer c.access.Unlock()

	if inbound != "" {
		counter := c.loadOrCreateCounter(&c.inbounds, inbound, true)
		readCounter = append(readCounter, counter.read, counter.totalRead)
		writeCounter = append(writeCounter, counter.write, counter.totalWrite)
		limiter = c.limiters[inbound]
	}
	if outbound != "" {
		counter := c.loadOrCreateCounter(&c.outbounds, outbound, false)
		readCounter = append(readCounter, counter.read)
		writeCounter = append(writeCounter, counter.write)
	}
	if user != "" {
		counter := c.loadOrCreateCounter(&c.users, user, false)
		readCounter = append(readCounter, counter.read)
		writeCounter = append(writeCounter, counter.write)
	}
	return readCounter, writeCounter, limiter
}

func (c *StatsTracker) loadOrCreateCounter(obj *map[string]Counter, name string, cumulative bool) Counter {
	counter, loaded := (*obj)[name]
	if loaded {
		return counter
	}
	counter = Counter{read: &SAtomic.Int64{}, write: &SAtomic.Int64{}}
	if cumulative {
		counter.totalRead = &SAtomic.Int64{}
		counter.totalWrite = &SAtomic.Int64{}
	}
	(*obj)[name] = counter
	return counter
}

func (c *StatsTracker) RoutedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) net.Conn {
	return c.wrapConnection(ctx, conn, metadata.Inbound, matchOutbound.Tag(), metadata.User)
}

func (c *StatsTracker) wrapConnection(ctx context.Context, conn net.Conn, inbound, outbound, user string) net.Conn {
	readCounter, writeCounter, limiter := c.tracking(inbound, outbound, user)
	counted := bufio.NewInt64CounterConn(conn, readCounter, writeCounter)
	if limiter == nil {
		return counted
	}
	limitCtx, cancel := context.WithCancel(ctx)
	return &limitedConn{ExtendedConn: counted, limiter: limiter, ctx: limitCtx, cancel: cancel}
}

func (c *StatsTracker) RoutedPacketConnection(ctx context.Context, conn network.PacketConn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) network.PacketConn {
	return c.wrapPacketConnection(ctx, conn, metadata.Inbound, matchOutbound.Tag(), metadata.User)
}

func (c *StatsTracker) wrapPacketConnection(ctx context.Context, conn network.PacketConn, inbound, outbound, user string) network.PacketConn {
	readCounter, writeCounter, limiter := c.tracking(inbound, outbound, user)
	counted := bufio.NewInt64CounterPacketConn(conn, readCounter, nil, writeCounter, nil)
	if limiter == nil {
		return counted
	}
	limitCtx, cancel := context.WithCancel(ctx)
	return &limitedPacketConn{PacketConn: counted, limiter: limiter, ctx: limitCtx, cancel: cancel}
}

type limitedConn struct {
	network.ExtendedConn
	limiter *inboundLimiter
	ctx     context.Context
	cancel  context.CancelFunc
}

func (c *limitedConn) Read(buffer []byte) (int, error) {
	n, err := c.ExtendedConn.Read(buffer)
	if n > 0 {
		if limitErr := waitForLimit(c.ctx, &c.limiter.upload, n); limitErr != nil && err == nil {
			err = limitErr
		}
	}
	return n, err
}

func (c *limitedConn) Write(buffer []byte) (int, error) {
	if err := waitForLimit(c.ctx, &c.limiter.download, len(buffer)); err != nil {
		return 0, err
	}
	return c.ExtendedConn.Write(buffer)
}

func (c *limitedConn) ReadBuffer(buffer *buf.Buffer) error {
	if err := c.ExtendedConn.ReadBuffer(buffer); err != nil {
		return err
	}
	return waitForLimit(c.ctx, &c.limiter.upload, buffer.Len())
}

func (c *limitedConn) WriteBuffer(buffer *buf.Buffer) error {
	if err := waitForLimit(c.ctx, &c.limiter.download, buffer.Len()); err != nil {
		return err
	}
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *limitedConn) Close() error {
	c.cancel()
	return c.ExtendedConn.Close()
}

func (c *limitedConn) Upstream() any {
	return c.ExtendedConn
}

var _ network.ExtendedConn = (*limitedConn)(nil)

type limitedPacketConn struct {
	network.PacketConn
	limiter *inboundLimiter
	ctx     context.Context
	cancel  context.CancelFunc
}

func (c *limitedPacketConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	destination, err := c.PacketConn.ReadPacket(buffer)
	if err == nil && buffer.Len() > 0 {
		err = waitForLimit(c.ctx, &c.limiter.upload, buffer.Len())
	}
	return destination, err
}

func (c *limitedPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	if err := waitForLimit(c.ctx, &c.limiter.download, buffer.Len()); err != nil {
		return err
	}
	return c.PacketConn.WritePacket(buffer, destination)
}

func (c *limitedPacketConn) Close() error {
	c.cancel()
	return c.PacketConn.Close()
}

func (c *limitedPacketConn) Upstream() any {
	return c.PacketConn
}

func (c *StatsTracker) InboundTrafficSnapshot() (int64, map[string]InboundTrafficSnapshot) {
	return c.inboundTrafficSnapshotAt(time.Now())
}

func (c *StatsTracker) inboundTrafficSnapshotAt(now time.Time) (int64, map[string]InboundTrafficSnapshot) {
	c.access.RLock()
	counters := make(map[string]Counter, len(c.inbounds))
	limiters := make(map[string]*inboundLimiter, len(c.limiters))
	for tag, counter := range c.inbounds {
		counters[tag] = counter
		limiters[tag] = c.limiters[tag]
	}
	c.access.RUnlock()

	c.snapshotAccess.Lock()
	defer c.snapshotAccess.Unlock()
	elapsed := now.Sub(c.lastSample)
	recompute := elapsed >= time.Second
	result := make(map[string]InboundTrafficSnapshot, len(counters))
	for tag, counter := range counters {
		totals := trafficTotals{upload: counter.totalRead.Load(), download: counter.totalWrite.Load()}
		if recompute {
			previous := c.lastTotals[tag]
			c.cachedBPS[tag] = trafficTotals{
				upload:   bytesPerSecond(totals.upload-previous.upload, elapsed),
				download: bytesPerSecond(totals.download-previous.download, elapsed),
			}
			c.lastTotals[tag] = totals
		}
		bps := c.cachedBPS[tag]
		item := InboundTrafficSnapshot{
			UploadBPS: bps.upload, DownloadBPS: bps.download,
			UploadPending: counter.read.Load(), DownloadPending: counter.write.Load(),
			UploadSession: totals.upload, DownloadSession: totals.download,
		}
		if limiter := limiters[tag]; limiter != nil {
			item.UploadLimit = limiter.uploadLimit.Load()
			item.DownloadLimit = limiter.downloadLimit.Load()
		}
		result[tag] = item
	}
	if recompute {
		c.lastSample = now
	}
	return c.lastSample.Unix(), result
}

func bytesPerSecond(bytes int64, elapsed time.Duration) int64 {
	if bytes <= 0 || elapsed <= 0 {
		return 0
	}
	return int64(float64(bytes) / elapsed.Seconds())
}

func (c *StatsTracker) GetStats() *[]model.Stats {
	c.access.Lock()
	defer c.access.Unlock()

	dt := time.Now().Unix()
	s := []model.Stats{}
	for inbound, counter := range c.inbounds {
		down := counter.write.Swap(0)
		up := counter.read.Swap(0)
		if down > 0 || up > 0 {
			s = append(s,
				model.Stats{DateTime: dt, Resource: "inbound", Tag: inbound, Direction: false, Traffic: down},
				model.Stats{DateTime: dt, Resource: "inbound", Tag: inbound, Direction: true, Traffic: up},
			)
		}
	}
	for outbound, counter := range c.outbounds {
		down := counter.write.Swap(0)
		up := counter.read.Swap(0)
		if down > 0 || up > 0 {
			s = append(s,
				model.Stats{DateTime: dt, Resource: "outbound", Tag: outbound, Direction: false, Traffic: down},
				model.Stats{DateTime: dt, Resource: "outbound", Tag: outbound, Direction: true, Traffic: up},
			)
		}
	}
	for user, counter := range c.users {
		down := counter.write.Swap(0)
		up := counter.read.Swap(0)
		if down > 0 || up > 0 {
			s = append(s,
				model.Stats{DateTime: dt, Resource: "user", Tag: user, Direction: false, Traffic: down},
				model.Stats{DateTime: dt, Resource: "user", Tag: user, Direction: true, Traffic: up},
			)
		}
	}
	return &s
}
