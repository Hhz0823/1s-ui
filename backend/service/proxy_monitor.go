package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/proxyprobe"
	"github.com/Hhz0823/1s-ui/util/common"
)

const (
	proxyMonitorMax             = 100
	proxyMonitorMinInterval     = 30
	proxyMonitorMaxInterval     = 3600
	proxyMonitorDefaultInterval = 60
	proxyMonitorRetention       = 3 * 24 * time.Hour
	proxyMonitorMaxPoints       = 360
	proxyMonitorRecent          = 30
	// maxProxyProbes bounds one probe.proxies request.
	maxProxyProbes = 32
	// StageServer marks a check that could not run because the server
	// chosen to run it was unreachable; it says nothing about the proxy.
	ProxyStageServer = "server"
)

// ProxyProbeRequest is the payload of the probe.proxies RPC.
type ProxyProbeRequest struct {
	Probes []proxyprobe.Spec `json:"probes"`
}

type ProxyProbeResponse struct {
	Results []proxyprobe.Result `json:"results"`
}

// RunProxyProbes checks proxies from this server, a few at a time.
func RunProxyProbes(request ProxyProbeRequest) (*ProxyProbeResponse, error) {
	if len(request.Probes) == 0 || len(request.Probes) > maxProxyProbes {
		return nil, common.NewErrorf("a probe request carries 1-%d proxies", maxProxyProbes)
	}
	results := make([]proxyprobe.Result, len(request.Probes))
	slots := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, spec := range request.Probes {
		wg.Add(1)
		slots <- struct{}{}
		go func(i int, spec proxyprobe.Spec) {
			defer func() { <-slots; wg.Done() }()
			results[i] = proxyprobe.Run(context.Background(), spec)
		}(i, spec)
	}
	wg.Wait()
	return &ProxyProbeResponse{Results: results}, nil
}

// ProxyMonitorInput is what the web UI and the monitor app send to save a
// monitor. A nil Password keeps the stored one; Link fills the proxy fields
// from a pasted share link or host:port:user:pass line.
type ProxyMonitorInput struct {
	Id       uint    `json:"id"`
	Name     string  `json:"name"`
	Link     string  `json:"link"`
	Type     string  `json:"type"`
	Host     string  `json:"host"`
	Port     int     `json:"port"`
	Username string  `json:"username"`
	Password *string `json:"password"`
	Target   string  `json:"target"`
	ServerId uint    `json:"server_id"`
	Interval int     `json:"interval"`
	Enabled  *bool   `json:"enabled"`
}

// ProxyMonitorPoint is one check in a monitor's recent strip.
type ProxyMonitorPoint struct {
	Time      int64 `json:"time"`
	OK        bool  `json:"ok"`
	LatencyMs int64 `json:"latency_ms"`
}

// ProxyMonitorView never carries the password.
type ProxyMonitorView struct {
	model.ProxyMonitor
	HasPassword bool                `json:"has_password"`
	ServerName  string              `json:"server_name"`
	Status      string              `json:"status"`
	Last        *proxyprobe.Result  `json:"last,omitempty"`
	Uptime      float64             `json:"uptime"`
	AvgLatency  float64             `json:"avg_latency"`
	Checks      int                 `json:"checks"`
	Recent      []ProxyMonitorPoint `json:"recent"`
}

// ProxyMonitorBucket averages the checks of one slice of a history range.
type ProxyMonitorBucket struct {
	Time      int64   `json:"time"`
	Checks    int     `json:"checks"`
	Failures  int     `json:"failures"`
	LatencyMs float64 `json:"latency_ms"`
	MaxMs     int64   `json:"max_ms"`
}

type ProxyExitIP struct {
	IP        string `json:"ip"`
	Country   string `json:"country"`
	FirstSeen int64  `json:"first_seen"`
	LastSeen  int64  `json:"last_seen"`
	Checks    int    `json:"checks"`
}

type ProxyMonitorDetail struct {
	ProxyMonitorView
	Range    int64                `json:"range"`
	Points   []ProxyMonitorBucket `json:"points"`
	Failures []proxyprobe.Result  `json:"failures"`
	ExitIPs  []ProxyExitIP        `json:"exit_ips"`
}

type ProxyMonitorService struct {
	AgentService
}

var proxyMonitorState = struct {
	sync.Mutex
	lastRun map[uint]time.Time
	busy    map[uint]bool
}{busy: map[uint]bool{}}

func (s *ProxyMonitorService) List() ([]ProxyMonitorView, error) {
	db := database.GetDB()
	var monitors []model.ProxyMonitor
	if err := db.Order("sort_order ASC, id ASC").Find(&monitors).Error; err != nil {
		return nil, err
	}
	names, err := managedServerNames()
	if err != nil {
		return nil, err
	}
	type summary struct {
		MonitorId uint
		Checks    int
		Good      int
		Latency   float64
	}
	var summaries []summary
	since := time.Now().Add(-24 * time.Hour).Unix()
	if err := db.Raw(`SELECT monitor_id, COUNT(*) AS checks, SUM(CASE WHEN ok THEN 1 ELSE 0 END) AS good,
		COALESCE(AVG(CASE WHEN ok THEN latency_ms END), 0) AS latency
		FROM proxy_monitor_results WHERE time >= ? AND stage <> ? GROUP BY monitor_id`, since, ProxyStageServer).Scan(&summaries).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]summary, len(summaries))
	for _, item := range summaries {
		byID[item.MonitorId] = item
	}
	views := make([]ProxyMonitorView, 0, len(monitors))
	for _, monitor := range monitors {
		view := newProxyMonitorView(monitor, names)
		if item, ok := byID[monitor.Id]; ok && item.Checks > 0 {
			view.Checks = item.Checks
			view.Uptime = float64(item.Good) * 100 / float64(item.Checks)
			view.AvgLatency = item.Latency
		}
		var rows []model.ProxyMonitorResult
		if err := db.Where("monitor_id = ?", monitor.Id).Order("time DESC, id DESC").Limit(proxyMonitorRecent).Find(&rows).Error; err != nil {
			return nil, err
		}
		for i := len(rows) - 1; i >= 0; i-- {
			view.Recent = append(view.Recent, ProxyMonitorPoint{Time: rows[i].Time, OK: rows[i].OK, LatencyMs: rows[i].LatencyMs})
		}
		if len(rows) > 0 {
			last := resultFromRow(rows[0])
			view.Last = &last
		}
		view.Status = proxyMonitorStatus(monitor, view.Last)
		views = append(views, view)
	}
	return views, nil
}

func newProxyMonitorView(monitor model.ProxyMonitor, names map[uint]string) ProxyMonitorView {
	view := ProxyMonitorView{ProxyMonitor: monitor, HasPassword: monitor.Password != "", Recent: []ProxyMonitorPoint{}}
	view.Password = ""
	if monitor.ServerId != 0 {
		view.ServerName = names[monitor.ServerId]
	}
	return view
}

func proxyMonitorStatus(monitor model.ProxyMonitor, last *proxyprobe.Result) string {
	switch {
	case !monitor.Enabled:
		return "paused"
	case last == nil:
		return "pending"
	case last.Stage == ProxyStageServer:
		return "unknown"
	case last.OK:
		return "up"
	default:
		return "down"
	}
}

func managedServerNames() (map[uint]string, error) {
	var nodes []model.AgentNode
	if err := database.GetDB().Select("id", "name").Find(&nodes).Error; err != nil {
		return nil, err
	}
	names := make(map[uint]string, len(nodes))
	for _, node := range nodes {
		names[node.Id] = node.Name
	}
	return names, nil
}

func resultFromRow(row model.ProxyMonitorResult) proxyprobe.Result {
	return proxyprobe.Result{
		OK: row.OK, Time: row.Time, ConnectMs: row.ConnectMs, HandshakeMs: row.HandshakeMs, TLSMs: row.TLSMs,
		TTFBMs: row.TTFBMs, LatencyMs: row.LatencyMs, Status: row.Status, ExitIP: row.ExitIP,
		Country: row.Country, Stage: row.Stage, Error: row.Error,
	}
}

// Detail returns one monitor with its history over the last rangeSeconds.
func (s *ProxyMonitorService) Detail(id uint, rangeSeconds int64) (*ProxyMonitorDetail, error) {
	if rangeSeconds <= 0 {
		rangeSeconds = 24 * 3600
	}
	if max := int64(proxyMonitorRetention / time.Second); rangeSeconds > max {
		rangeSeconds = max
	}
	views, err := s.List()
	if err != nil {
		return nil, err
	}
	var detail *ProxyMonitorDetail
	for _, view := range views {
		if view.Id == id {
			detail = &ProxyMonitorDetail{ProxyMonitorView: view, Range: rangeSeconds}
			break
		}
	}
	if detail == nil {
		return nil, common.NewError("proxy monitor not found")
	}
	from := time.Now().Unix() - rangeSeconds
	var rows []model.ProxyMonitorResult
	if err := database.GetDB().Where("monitor_id = ? AND time >= ?", id, from).Order("time ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	step := rangeSeconds / proxyMonitorMaxPoints
	if interval := int64(detail.Interval); step < interval {
		step = interval
	}
	if step < 1 {
		step = 1
	}
	detail.Points = bucketProxyResults(rows, step)
	detail.Failures = []proxyprobe.Result{}
	exits := map[string]*ProxyExitIP{}
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		if !row.OK && len(detail.Failures) < 20 {
			detail.Failures = append(detail.Failures, resultFromRow(row))
		}
		if row.ExitIP == "" {
			continue
		}
		exit := exits[row.ExitIP]
		if exit == nil {
			exit = &ProxyExitIP{IP: row.ExitIP, Country: row.Country, LastSeen: row.Time}
			exits[row.ExitIP] = exit
		}
		exit.FirstSeen = row.Time
		exit.Checks++
	}
	detail.ExitIPs = make([]ProxyExitIP, 0, len(exits))
	for _, exit := range exits {
		detail.ExitIPs = append(detail.ExitIPs, *exit)
	}
	sort.Slice(detail.ExitIPs, func(i, j int) bool { return detail.ExitIPs[i].LastSeen > detail.ExitIPs[j].LastSeen })
	return detail, nil
}

func bucketProxyResults(rows []model.ProxyMonitorResult, step int64) []ProxyMonitorBucket {
	points := []ProxyMonitorBucket{}
	var good int
	var sum float64
	for _, row := range rows {
		if row.Stage == ProxyStageServer {
			continue
		}
		key := row.Time / step * step
		if len(points) == 0 || points[len(points)-1].Time != key {
			if len(points) > 0 && good > 0 {
				points[len(points)-1].LatencyMs = sum / float64(good)
			}
			points = append(points, ProxyMonitorBucket{Time: key})
			good, sum = 0, 0
		}
		point := &points[len(points)-1]
		point.Checks++
		if row.OK {
			good++
			sum += float64(row.LatencyMs)
			if row.LatencyMs > point.MaxMs {
				point.MaxMs = row.LatencyMs
			}
		} else {
			point.Failures++
		}
	}
	if len(points) > 0 && good > 0 {
		points[len(points)-1].LatencyMs = sum / float64(good)
	}
	return points
}

// normalizeProxyMonitor turns an input into a monitor, keeping the stored
// password unless a new one is given.
func (s *ProxyMonitorService) normalizeProxyMonitor(input ProxyMonitorInput, existing *model.ProxyMonitor) (model.ProxyMonitor, error) {
	monitor := model.ProxyMonitor{}
	if existing != nil {
		monitor = *existing
	}
	spec := proxyprobe.Spec{Type: input.Type, Host: input.Host, Port: input.Port, Username: input.Username, Target: input.Target}
	name := input.Name
	if link := strings.TrimSpace(input.Link); link != "" {
		parsed, linkName, err := proxyprobe.ParseLink(link)
		if err != nil {
			return monitor, err
		}
		parsed.Target = input.Target
		spec = parsed
		if strings.TrimSpace(name) == "" {
			name = linkName
		}
		input.Password = &parsed.Password
	}
	switch {
	case input.Password != nil:
		spec.Password = *input.Password
	case existing != nil:
		spec.Password = existing.Password
	}
	if err := proxyprobe.Normalize(&spec); err != nil {
		return monitor, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = spec.Host + ":" + fmt.Sprint(spec.Port)
	}
	if len([]rune(name)) > 80 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return monitor, common.NewError("monitor name must be at most 80 characters without control characters")
	}
	interval := input.Interval
	if interval == 0 {
		interval = proxyMonitorDefaultInterval
	}
	if interval < proxyMonitorMinInterval || interval > proxyMonitorMaxInterval {
		return monitor, common.NewErrorf("check interval must be %d-%d seconds", proxyMonitorMinInterval, proxyMonitorMaxInterval)
	}
	if input.ServerId != 0 {
		var count int64
		if err := database.GetDB().Model(&model.AgentNode{}).Where("id = ?", input.ServerId).Count(&count).Error; err != nil {
			return monitor, err
		}
		if count == 0 {
			return monitor, common.NewError("the server chosen to run the check does not exist")
		}
	}
	monitor.Name, monitor.Type, monitor.Host, monitor.Port = name, spec.Type, spec.Host, spec.Port
	monitor.Username, monitor.Password, monitor.ServerId, monitor.Interval = spec.Username, spec.Password, input.ServerId, interval
	monitor.Target = ""
	if spec.Target != proxyprobe.DefaultTarget {
		monitor.Target = spec.Target
	}
	switch {
	case input.Enabled != nil:
		monitor.Enabled = *input.Enabled
	case existing == nil:
		monitor.Enabled = true
	}
	return monitor, nil
}

func (s *ProxyMonitorService) Save(input ProxyMonitorInput) (*ProxyMonitorView, error) {
	db := database.GetDB()
	var existing *model.ProxyMonitor
	if input.Id != 0 {
		var stored model.ProxyMonitor
		if err := db.Where("id = ?", input.Id).First(&stored).Error; err != nil {
			return nil, common.NewError("proxy monitor not found")
		}
		existing = &stored
	} else {
		var count int64
		if err := db.Model(&model.ProxyMonitor{}).Count(&count).Error; err != nil {
			return nil, err
		}
		if count >= proxyMonitorMax {
			return nil, common.NewErrorf("at most %d proxy monitors", proxyMonitorMax)
		}
	}
	monitor, err := s.normalizeProxyMonitor(input, existing)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		monitor.CreatedAt = time.Now().Unix()
		if err := db.Create(&monitor).Error; err != nil {
			return nil, err
		}
	} else {
		if err := db.Save(&monitor).Error; err != nil {
			return nil, err
		}
		changed := existing.Host != monitor.Host || existing.Port != monitor.Port || existing.Type != monitor.Type ||
			existing.ServerId != monitor.ServerId || existing.Username != monitor.Username || existing.Password != monitor.Password ||
			existing.Target != monitor.Target
		if changed {
			// Old results describe another proxy.
			if err := db.Where("monitor_id = ?", monitor.Id).Delete(&model.ProxyMonitorResult{}).Error; err != nil {
				return nil, err
			}
		}
	}
	// Check a new or changed monitor on the next tick.
	proxyMonitorState.Lock()
	if proxyMonitorState.lastRun != nil {
		delete(proxyMonitorState.lastRun, monitor.Id)
	}
	proxyMonitorState.Unlock()
	names, _ := managedServerNames()
	view := newProxyMonitorView(monitor, names)
	view.Status = proxyMonitorStatus(monitor, nil)
	return &view, nil
}

func (s *ProxyMonitorService) Delete(id uint) error {
	db := database.GetDB()
	if err := db.Where("id = ?", id).Delete(&model.ProxyMonitor{}).Error; err != nil {
		return err
	}
	return db.Where("monitor_id = ?", id).Delete(&model.ProxyMonitorResult{}).Error
}

// Test checks a proxy once without saving it. With an Id and no password it
// uses the stored password, so editing a monitor can be tested as typed.
func (s *ProxyMonitorService) Test(input ProxyMonitorInput) (*proxyprobe.Result, error) {
	var existing *model.ProxyMonitor
	if input.Id != 0 {
		var stored model.ProxyMonitor
		if err := database.GetDB().Where("id = ?", input.Id).First(&stored).Error; err == nil {
			existing = &stored
		}
	}
	monitor, err := s.normalizeProxyMonitor(input, existing)
	if err != nil {
		return nil, err
	}
	results := s.probe(monitor.ServerId, []model.ProxyMonitor{monitor})
	return &results[0], nil
}

// CheckNow checks a saved monitor immediately and records the result.
func (s *ProxyMonitorService) CheckNow(id uint) (*proxyprobe.Result, error) {
	var monitor model.ProxyMonitor
	if err := database.GetDB().Where("id = ?", id).First(&monitor).Error; err != nil {
		return nil, common.NewError("proxy monitor not found")
	}
	results := s.probe(monitor.ServerId, []model.ProxyMonitor{monitor})
	recordProxyResults([]model.ProxyMonitor{monitor}, results)
	return &results[0], nil
}

func monitorSpec(monitor model.ProxyMonitor) proxyprobe.Spec {
	return proxyprobe.Spec{
		Type: monitor.Type, Host: monitor.Host, Port: monitor.Port,
		Username: monitor.Username, Password: monitor.Password, Target: monitor.Target,
	}
}

// probe runs the checks on the panel host or on a managed server. When the
// server cannot be asked, every result says so with ProxyStageServer.
func (s *ProxyMonitorService) probe(serverID uint, monitors []model.ProxyMonitor) []proxyprobe.Result {
	request := ProxyProbeRequest{Probes: make([]proxyprobe.Spec, len(monitors))}
	for i, monitor := range monitors {
		request.Probes[i] = monitorSpec(monitor)
	}
	var response *ProxyProbeResponse
	var err error
	if serverID == 0 {
		response, err = RunProxyProbes(request)
	} else {
		var rpc *agent.RPCResponse
		rpc, err = s.AgentService.DispatchRPC(serverID, agent.RPCMethodProxyProbe, request, "proxy-monitor")
		if err == nil {
			response = &ProxyProbeResponse{}
			if decodeErr := json.Unmarshal(rpc.Payload, response); decodeErr != nil || len(response.Results) != len(monitors) {
				err = common.NewError("the server returned an invalid probe result")
			}
		}
	}
	if err != nil {
		message := err.Error()
		if strings.Contains(message, "unsupported") {
			message = "the panel on this server is too old to check proxies; update it"
		}
		results := make([]proxyprobe.Result, len(monitors))
		for i := range results {
			results[i] = proxyprobe.Result{Time: time.Now().Unix(), Stage: ProxyStageServer, Error: truncate(message, 200)}
		}
		return results
	}
	return response.Results
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func recordProxyResults(monitors []model.ProxyMonitor, results []proxyprobe.Result) {
	rows := make([]model.ProxyMonitorResult, 0, len(results))
	for i, result := range results {
		if i >= len(monitors) {
			break
		}
		if result.Time == 0 {
			result.Time = time.Now().Unix()
		}
		rows = append(rows, model.ProxyMonitorResult{
			MonitorId: monitors[i].Id, Time: result.Time, OK: result.OK, LatencyMs: result.LatencyMs,
			ConnectMs: result.ConnectMs, HandshakeMs: result.HandshakeMs, TLSMs: result.TLSMs, TTFBMs: result.TTFBMs,
			Status: result.Status, ExitIP: truncate(result.ExitIP, 64), Country: truncate(result.Country, 8),
			Stage: truncate(result.Stage, 16), Error: truncate(result.Error, 255),
		})
	}
	if len(rows) == 0 {
		return
	}
	if err := database.GetDB().CreateInBatches(rows, 50).Error; err != nil {
		logger.Warning("store proxy checks: ", err)
	}
}

// RunDue starts the checks whose interval has passed, one request per
// server at a time. The cron job calls it every few seconds.
func (s *ProxyMonitorService) RunDue(now time.Time) {
	var monitors []model.ProxyMonitor
	if err := database.GetDB().Where("enabled = ?", true).Order("id ASC").Find(&monitors).Error; err != nil {
		logger.Warning("load proxy monitors: ", err)
		return
	}
	proxyMonitorState.Lock()
	if proxyMonitorState.lastRun == nil {
		// After a restart, keep each monitor on its schedule.
		proxyMonitorState.lastRun = map[uint]time.Time{}
		var last []struct {
			MonitorId uint
			Time      int64
		}
		database.GetDB().Raw("SELECT monitor_id, MAX(time) AS time FROM proxy_monitor_results GROUP BY monitor_id").Scan(&last)
		for _, item := range last {
			proxyMonitorState.lastRun[item.MonitorId] = time.Unix(item.Time, 0)
		}
	}
	groups := map[uint][]model.ProxyMonitor{}
	for _, monitor := range monitors {
		if proxyMonitorState.busy[monitor.ServerId] || len(groups[monitor.ServerId]) >= maxProxyProbes {
			continue
		}
		interval := time.Duration(monitor.Interval) * time.Second
		if last, ok := proxyMonitorState.lastRun[monitor.Id]; ok && now.Sub(last) < interval {
			continue
		}
		groups[monitor.ServerId] = append(groups[monitor.ServerId], monitor)
	}
	for serverID, list := range groups {
		proxyMonitorState.busy[serverID] = true
		for _, monitor := range list {
			proxyMonitorState.lastRun[monitor.Id] = now
		}
		go func(serverID uint, list []model.ProxyMonitor) {
			defer func() {
				proxyMonitorState.Lock()
				delete(proxyMonitorState.busy, serverID)
				proxyMonitorState.Unlock()
			}()
			recordProxyResults(list, s.probe(serverID, list))
		}(serverID, list)
	}
	proxyMonitorState.Unlock()
}

// CleanupProxyResults drops checks older than the retention window.
func (s *ProxyMonitorService) CleanupProxyResults() error {
	cutoff := time.Now().Add(-proxyMonitorRetention).Unix()
	return database.GetDB().Where("time < ?", cutoff).Delete(&model.ProxyMonitorResult{}).Error
}
