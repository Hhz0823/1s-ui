package service

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util/common"
)

const (
	agentMetricMinute    = int64(60)
	agentMetricCoarse    = int64(900)
	agentMetricFineKeep  = 24 * time.Hour
	agentMetricRetention = 31 * 24 * time.Hour
	agentMetricMaxPoints = 360
	agentTrafficCacheTTL = 30 * time.Second
	agentMetricMaxRange  = int64(31 * 24 * 3600)
)

// agentMinuteAcc sums the reports and pings a node sent during the current
// minute; FlushAgentMetrics turns it into one stored row.
type agentMinuteAcc struct {
	samples            int
	cpu, mem, swap     float64
	disk, load1, procs float64
	tcp, udp           float64
	sentRate, recvRate float64
	sent, recv         uint64
	pingSum            float64
	pingOK, pingFail   int
}

// AgentMetricPoint is one point of a stored history range.
type AgentMetricPoint struct {
	Time        int64   `json:"time"`
	CPU         float64 `json:"cpu"`
	Mem         float64 `json:"mem"`
	Swap        float64 `json:"swap"`
	Disk        float64 `json:"disk"`
	Load1       float64 `json:"load1"`
	Processes   float64 `json:"processes"`
	TCP         float64 `json:"tcp"`
	UDP         float64 `json:"udp"`
	NetSentRate float64 `json:"net_sent_rate"`
	NetRecvRate float64 `json:"net_recv_rate"`
	NetSent     uint64  `json:"net_sent"`
	NetRecv     uint64  `json:"net_recv"`
	PingMs      float64 `json:"ping_ms"`
	PingLoss    float64 `json:"ping_loss"`
}

// AgentTrafficView is traffic counted since the node's last monthly reset.
type AgentTrafficView struct {
	Sent        uint64 `json:"sent"`
	Recv        uint64 `json:"recv"`
	PeriodStart int64  `json:"period_start"`
	NextReset   int64  `json:"next_reset"`
}

var (
	agentAccMu        sync.Mutex
	agentAcc          = map[uint]*agentMinuteAcc{}
	agentTrafficMu    sync.Mutex
	agentTrafficCache = map[uint]agentTrafficCacheEntry{}
)

type agentTrafficCacheEntry struct {
	start      int64
	sent, recv uint64
	at         time.Time
}

func accFor(id uint) *agentMinuteAcc {
	acc := agentAcc[id]
	if acc == nil {
		acc = &agentMinuteAcc{}
		agentAcc[id] = acc
	}
	return acc
}

// counterDelta returns traffic since the previous report. Counters restart
// from zero when the server reboots, so a smaller value is all new traffic.
func counterDelta(previous, current uint64) uint64 {
	if current >= previous {
		return current - previous
	}
	return current
}

func recordAgentMetric(id uint, report agent.Report, sentDelta, recvDelta uint64) {
	agentAccMu.Lock()
	defer agentAccMu.Unlock()
	acc := accFor(id)
	acc.samples++
	acc.cpu += report.CPUPercent
	acc.mem += usagePercent(report.Memory)
	acc.swap += usagePercent(report.Swap)
	acc.disk += usagePercent(report.Disk)
	acc.load1 += report.Load.Load1
	acc.procs += float64(report.ProcessCount)
	acc.tcp += float64(report.TCPConns)
	acc.udp += float64(report.UDPConns)
	acc.sentRate += float64(report.NetRate.Sent)
	acc.recvRate += float64(report.NetRate.Recv)
	acc.sent += sentDelta
	acc.recv += recvDelta
}

func recordAgentPing(id uint, ms int64, ok bool) {
	agentAccMu.Lock()
	defer agentAccMu.Unlock()
	acc := accFor(id)
	if ok {
		acc.pingOK++
		acc.pingSum += float64(ms)
	} else {
		acc.pingFail++
	}
}

func usagePercent(value agent.ResourceUsage) float64 {
	if value.Total == 0 {
		return 0
	}
	return float64(value.Used) * 100 / float64(value.Total)
}

// FlushAgentMetrics stores last minute's samples. It runs once a minute.
func (s *AgentService) FlushAgentMetrics() error {
	agentAccMu.Lock()
	pending := agentAcc
	agentAcc = map[uint]*agentMinuteAcc{}
	agentAccMu.Unlock()
	if len(pending) == 0 {
		return nil
	}
	bucket := time.Now().Unix()/agentMetricMinute*agentMetricMinute - agentMetricMinute
	rows := make([]model.AgentMetric, 0, len(pending))
	for id, acc := range pending {
		row := model.AgentMetric{NodeId: id, Time: bucket, Resolution: agentMetricMinute, Samples: acc.samples, NetSent: acc.sent, NetRecv: acc.recv}
		if n := float64(acc.samples); n > 0 {
			row.CPU, row.Mem, row.Swap, row.Disk = acc.cpu/n, acc.mem/n, acc.swap/n, acc.disk/n
			row.Load1, row.Processes, row.TCP, row.UDP = acc.load1/n, acc.procs/n, acc.tcp/n, acc.udp/n
			row.NetSentRate, row.NetRecvRate = acc.sentRate/n, acc.recvRate/n
		}
		if acc.pingOK > 0 {
			row.PingMs = acc.pingSum / float64(acc.pingOK)
		}
		if total := acc.pingOK + acc.pingFail; total > 0 {
			row.PingLoss = float64(acc.pingFail) * 100 / float64(total)
		}
		rows = append(rows, row)
	}
	err := database.GetDB().CreateInBatches(rows, 100).Error
	agentTrafficMu.Lock()
	agentTrafficCache = map[uint]agentTrafficCacheEntry{}
	agentTrafficMu.Unlock()
	return err
}

// CompactAgentMetrics merges minute rows older than a day into 15-minute rows
// and drops rows past the retention window, keeping the table small.
func (s *AgentService) CompactAgentMetrics() error {
	now := time.Now()
	cutoff := now.Add(-agentMetricFineKeep).Unix() / agentMetricCoarse * agentMetricCoarse
	db := database.GetDB()
	err := db.Exec(`INSERT INTO agent_metrics (node_id, time, resolution, samples, cpu, mem, swap, disk, load1, processes, tcp, udp, net_sent_rate, net_recv_rate, net_sent, net_recv, ping_ms, ping_loss)
		SELECT node_id, (time / ?) * ?, ?, SUM(samples), AVG(cpu), AVG(mem), AVG(swap), AVG(disk), AVG(load1), AVG(processes), AVG(tcp), AVG(udp),
			AVG(net_sent_rate), AVG(net_recv_rate), SUM(net_sent), SUM(net_recv),
			COALESCE(AVG(CASE WHEN ping_ms > 0 THEN ping_ms END), 0), AVG(ping_loss)
		FROM agent_metrics WHERE resolution = ? AND time < ? GROUP BY node_id, time / ?`,
		agentMetricCoarse, agentMetricCoarse, agentMetricCoarse, agentMetricMinute, cutoff, agentMetricCoarse).Error
	if err != nil {
		return err
	}
	if err := db.Where("resolution = ? AND time < ?", agentMetricMinute, cutoff).Delete(&model.AgentMetric{}).Error; err != nil {
		return err
	}
	return db.Where("time < ?", now.Add(-agentMetricRetention).Unix()).Delete(&model.AgentMetric{}).Error
}

// Metrics returns up to agentMetricMaxPoints averaged points covering the
// last rangeSeconds.
func (s *AgentService) Metrics(id uint, rangeSeconds int64) ([]AgentMetricPoint, error) {
	if rangeSeconds <= 0 || rangeSeconds > agentMetricMaxRange {
		return nil, common.NewError("invalid metric range")
	}
	from := time.Now().Unix() - rangeSeconds
	var rows []model.AgentMetric
	if err := database.GetDB().Where("node_id = ? AND time >= ?", id, from).Order("time ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	step := int64(math.Ceil(float64(rangeSeconds)/agentMetricMaxPoints/float64(agentMetricMinute))) * agentMetricMinute
	if step < agentMetricMinute {
		step = agentMetricMinute
	}
	return bucketAgentMetrics(rows, step), nil
}

func bucketAgentMetrics(rows []model.AgentMetric, step int64) []AgentMetricPoint {
	type bucket struct {
		point      AgentMetricPoint
		n, pingN   float64
		resolution int64
	}
	byTime := map[int64]*bucket{}
	for _, row := range rows {
		resolution := row.Resolution
		if resolution < agentMetricMinute {
			resolution = agentMetricMinute
		}
		size := step
		if resolution > size {
			size = resolution
		}
		key := row.Time / size * size
		b := byTime[key]
		if b == nil {
			b = &bucket{point: AgentMetricPoint{Time: key}, resolution: size}
			byTime[key] = b
		}
		p := &b.point
		p.CPU += row.CPU
		p.Mem += row.Mem
		p.Swap += row.Swap
		p.Disk += row.Disk
		p.Load1 += row.Load1
		p.Processes += row.Processes
		p.TCP += row.TCP
		p.UDP += row.UDP
		p.NetSentRate += row.NetSentRate
		p.NetRecvRate += row.NetRecvRate
		p.NetSent += row.NetSent
		p.NetRecv += row.NetRecv
		p.PingLoss += row.PingLoss
		if row.PingMs > 0 {
			p.PingMs += row.PingMs
			b.pingN++
		}
		b.n++
	}
	keys := make([]int64, 0, len(byTime))
	for key := range byTime {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	points := make([]AgentMetricPoint, 0, len(keys))
	for _, key := range keys {
		b := byTime[key]
		p := b.point
		p.CPU /= b.n
		p.Mem /= b.n
		p.Swap /= b.n
		p.Disk /= b.n
		p.Load1 /= b.n
		p.Processes /= b.n
		p.TCP /= b.n
		p.UDP /= b.n
		p.NetSentRate /= b.n
		p.NetRecvRate /= b.n
		p.PingLoss /= b.n
		if b.pingN > 0 {
			p.PingMs /= b.pingN
		}
		points = append(points, p)
	}
	return points
}

// trafficPeriod returns the start of the current monthly period and the next
// reset, for a reset on resetDay (1-28) at local midnight.
func trafficPeriod(now time.Time, resetDay int) (time.Time, time.Time) {
	if resetDay < 1 || resetDay > 28 {
		resetDay = 1
	}
	start := time.Date(now.Year(), now.Month(), resetDay, 0, 0, 0, 0, now.Location())
	if now.Before(start) {
		start = start.AddDate(0, -1, 0)
	}
	return start, start.AddDate(0, 1, 0)
}

// periodTraffic sums stored and not yet flushed traffic since the node's
// last reset. Results are cached briefly because the list refreshes often.
func periodTraffic(node model.AgentNode, now time.Time) AgentTrafficView {
	start, next := trafficPeriod(now, node.TrafficResetDay)
	view := AgentTrafficView{PeriodStart: start.Unix(), NextReset: next.Unix()}
	agentTrafficMu.Lock()
	cached, ok := agentTrafficCache[node.Id]
	agentTrafficMu.Unlock()
	if ok && cached.start == view.PeriodStart && now.Sub(cached.at) < agentTrafficCacheTTL {
		view.Sent, view.Recv = cached.sent, cached.recv
	} else {
		var sums struct{ Sent, Recv uint64 }
		database.GetDB().Model(&model.AgentMetric{}).
			Select("COALESCE(SUM(net_sent), 0) AS sent, COALESCE(SUM(net_recv), 0) AS recv").
			Where("node_id = ? AND time >= ?", node.Id, view.PeriodStart).Scan(&sums)
		view.Sent, view.Recv = sums.Sent, sums.Recv
		agentTrafficMu.Lock()
		agentTrafficCache[node.Id] = agentTrafficCacheEntry{start: view.PeriodStart, sent: sums.Sent, recv: sums.Recv, at: now}
		agentTrafficMu.Unlock()
	}
	agentAccMu.Lock()
	if acc := agentAcc[node.Id]; acc != nil {
		view.Sent += acc.sent
		view.Recv += acc.recv
	}
	agentAccMu.Unlock()
	return view
}

func forgetAgentMetrics(id uint) {
	agentAccMu.Lock()
	delete(agentAcc, id)
	agentAccMu.Unlock()
	agentTrafficMu.Lock()
	delete(agentTrafficCache, id)
	agentTrafficMu.Unlock()
	database.GetDB().Where("node_id = ?", id).Delete(&model.AgentMetric{})
}
