package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/util/common"
)

// SD-WAN detection and one-click tuning run as background jobs: measuring
// every path takes longer than an HTTP request may, so the page polls the job
// for its progress, log and report.
const (
	SdwanJobDiagnose = "diagnose"
	SdwanJobOptimize = "optimize"

	SdwanJobStatusRunning = "running"
	SdwanJobStatusDone    = "done"
	SdwanJobStatusFailed  = "failed"

	sdwanSeverityError   = "error"
	sdwanSeverityWarning = "warning"
	sdwanSeverityInfo    = "info"

	sdwanLatencySamples   = 5
	sdwanSampleTimeout    = 6 * time.Second
	sdwanSampleGap        = 200 * time.Millisecond
	sdwanMeasureParallel  = 4
	sdwanSpeedTestTimeout = 15 * time.Second
	sdwanSpeedTestBytes   = 32 << 20
	sdwanJobTimeout       = 20 * time.Minute
	sdwanMaxJobLogs       = 400
	sdwanLossyPercent     = 20
	// Shadowsocks 2022 rejects requests whose timestamp is more than 30
	// seconds off, so clock skew is reported before it breaks that path.
	sdwanClockSkewWarnMS  = 10_000
	sdwanClockSkewErrorMS = 30_000
)

// Path measurements go through the running sing-box; replaceable in tests.
var (
	sdwanCheckPath = func(ctx context.Context, tag, link string) (uint16, error) {
		result := core.CheckOutbound(ctx, tag, link)
		if !result.OK {
			return 0, errors.New(result.Error)
		}
		return result.Delay, nil
	}
	sdwanSpeedPath = func(ctx context.Context, tag, link string) (float64, error) {
		result, err := core.SpeedTest(ctx, tag, link, sdwanSpeedTestBytes)
		if err != nil {
			return 0, err
		}
		return result.Mbps(), nil
	}
	sdwanPathLoaded = func(tag string) bool {
		return corePtr != nil && corePtr.HasOutbound(tag)
	}
)

type SdwanJobLog struct {
	Time   int64             `json:"time"`
	Level  string            `json:"level"`
	Code   string            `json:"code"`
	Params map[string]string `json:"params,omitempty"`
}

type SdwanPathReport struct {
	Protocol   string  `json:"protocol"`
	Tag        string  `json:"tag"`
	Port       int     `json:"port"`
	Disabled   bool    `json:"disabled"`
	Measured   bool    `json:"measured"`
	Reachable  bool    `json:"reachable"`
	Samples    int     `json:"samples"`
	Failures   int     `json:"failures"`
	Latency    int     `json:"latency"`
	Min        int     `json:"min"`
	Jitter     int     `json:"jitter"`
	Loss       int     `json:"loss"`
	Mbps       float64 `json:"mbps,omitempty"`
	SpeedError string  `json:"speed_error,omitempty"`
	Error      string  `json:"error,omitempty"`
	Best       bool    `json:"best"`
	Selected   bool    `json:"selected"`
}

type SdwanNodeReport struct {
	NodeID      uint                `json:"node_id"`
	Name        string              `json:"name"`
	Online      bool                `json:"online"`
	Supported   bool                `json:"supported"`
	Diagnosed   bool                `json:"diagnosed"`
	CoreRunning bool                `json:"core_running"`
	ClockSkewMS *int64              `json:"clock_skew_ms,omitempty"`
	Tuning      *SdwanTuningStatus  `json:"tuning,omitempty"`
	Uplinks     []SdwanUplinkStatus `json:"uplinks"`
	Desired     []string            `json:"desired"`
	Paths       []SdwanPathReport   `json:"paths"`
	Error       string              `json:"error,omitempty"`
}

// SdwanAdvice is one finding of a detection run; fixable findings are what
// one-click tuning repairs.
type SdwanAdvice struct {
	Code     string            `json:"code"`
	Severity string            `json:"severity"`
	Fixable  bool              `json:"fixable"`
	NodeID   uint              `json:"node_id,omitempty"`
	Protocol string            `json:"protocol,omitempty"`
	Params   map[string]string `json:"params,omitempty"`
}

type SdwanReport struct {
	Time                 int64             `json:"time"`
	CoreRunning          bool              `json:"core_running"`
	Selected             string            `json:"selected,omitempty"`
	Controller           SdwanTuningStatus `json:"controller"`
	Nodes                []SdwanNodeReport `json:"nodes"`
	Advice               []SdwanAdvice     `json:"advice"`
	Score                int               `json:"score"`
	BestTag              string            `json:"best_tag,omitempty"`
	BestLatency          int               `json:"best_latency,omitempty"`
	Tolerance            int               `json:"tolerance"`
	RecommendedTolerance int               `json:"recommended_tolerance,omitempty"`
	Bandwidth            bool              `json:"bandwidth"`
}

// SdwanJob is the state of the latest detection or tuning run. Reports are
// never modified after they are attached, so snapshots may share them.
type SdwanJob struct {
	ID        string        `json:"id"`
	Kind      string        `json:"kind"`
	Status    string        `json:"status"`
	Bandwidth bool          `json:"bandwidth"`
	Stage     string        `json:"stage"`
	Progress  int           `json:"progress"`
	StartedAt int64         `json:"started_at"`
	EndedAt   int64         `json:"ended_at,omitempty"`
	Logs      []SdwanJobLog `json:"logs"`
	Changes   []SdwanJobLog `json:"changes"`
	Before    *SdwanReport  `json:"before,omitempty"`
	Report    *SdwanReport  `json:"report,omitempty"`
	Error     string        `json:"error,omitempty"`
}

var (
	sdwanJobMu sync.Mutex
	sdwanJob   *SdwanJob
)

var errSdwanJobRunning = common.NewError("an SD-WAN detection or tuning job is running, wait for it to finish")

func sdwanJobRunning() bool {
	sdwanJobMu.Lock()
	defer sdwanJobMu.Unlock()
	return sdwanJob != nil && sdwanJob.Status == SdwanJobStatusRunning
}

func currentSdwanJob() *SdwanJob {
	sdwanJobMu.Lock()
	defer sdwanJobMu.Unlock()
	if sdwanJob == nil {
		return nil
	}
	snapshot := *sdwanJob
	snapshot.Logs = append([]SdwanJobLog{}, sdwanJob.Logs...)
	snapshot.Changes = append([]SdwanJobLog{}, sdwanJob.Changes...)
	return &snapshot
}

// SdwanJob returns the running or latest finished job, if any.
func (s *SdwanService) SdwanJob() *SdwanJob {
	return currentSdwanJob()
}

func sdwanParams(pairs ...string) map[string]string {
	params := make(map[string]string, len(pairs)/2)
	for index := 0; index+1 < len(pairs); index += 2 {
		params[pairs[index]] = pairs[index+1]
	}
	return params
}

type sdwanJobRun struct {
	job   *SdwanJob
	actor string
}

func (r *sdwanJobRun) update(apply func(job *SdwanJob)) {
	sdwanJobMu.Lock()
	defer sdwanJobMu.Unlock()
	apply(r.job)
}

func (r *sdwanJobRun) log(level, code string, params map[string]string) {
	entry := SdwanJobLog{Time: time.Now().Unix(), Level: level, Code: code, Params: params}
	r.update(func(job *SdwanJob) {
		job.Logs = append(job.Logs, entry)
		if len(job.Logs) > sdwanMaxJobLogs {
			job.Logs = append([]SdwanJobLog{}, job.Logs[len(job.Logs)-sdwanMaxJobLogs:]...)
		}
	})
}

// change logs and records something the tuning job modified.
func (r *sdwanJobRun) change(code string, params map[string]string) {
	r.log("success", code, params)
	entry := SdwanJobLog{Time: time.Now().Unix(), Level: "success", Code: code, Params: params}
	r.update(func(job *SdwanJob) { job.Changes = append(job.Changes, entry) })
}

func (r *sdwanJobRun) stage(stage string, progress int) {
	r.update(func(job *SdwanJob) {
		job.Stage = stage
		job.Progress = max(job.Progress, progress)
	})
}

func (r *sdwanJobRun) progress(from, to, done, total int) {
	if total <= 0 {
		return
	}
	r.update(func(job *SdwanJob) { job.Progress = max(job.Progress, from+(to-from)*done/total) })
}

// StartSdwanJob starts a detection ("diagnose") or one-click tuning
// ("optimize") run in the background; bandwidth adds a download test through
// every reachable path.
func (s *SdwanService) StartSdwanJob(kind string, bandwidth bool, actor string) (*SdwanJob, error) {
	if kind != SdwanJobDiagnose && kind != SdwanJobOptimize {
		return nil, common.NewError("unknown SD-WAN job")
	}
	if err := (&SettingService{}).RequireControllerControl(); err != nil {
		return nil, err
	}
	sdwanJobMu.Lock()
	if sdwanJob != nil && sdwanJob.Status == SdwanJobStatusRunning {
		sdwanJobMu.Unlock()
		return nil, errSdwanJobRunning
	}
	job := &SdwanJob{
		ID: common.Random(12), Kind: kind, Status: SdwanJobStatusRunning, Bandwidth: bandwidth,
		Stage: "start", StartedAt: time.Now().Unix(), Logs: []SdwanJobLog{}, Changes: []SdwanJobLog{},
	}
	sdwanJob = job
	sdwanJobMu.Unlock()
	go s.runSdwanJob(&sdwanJobRun{job: job, actor: actor})
	return currentSdwanJob(), nil
}

func (s *SdwanService) runSdwanJob(run *sdwanJobRun) {
	ctx, cancel := context.WithTimeout(context.Background(), sdwanJobTimeout)
	defer cancel()
	var err error
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("SD-WAN job panicked: ", recovered)
				err = fmt.Errorf("internal error: %v", recovered)
			}
		}()
		// Kind and Bandwidth never change after the job is created.
		if run.job.Kind == SdwanJobOptimize {
			err = s.runSdwanOptimize(ctx, run)
		} else {
			err = s.runSdwanDiagnose(ctx, run)
		}
	}()
	if err != nil {
		logger.Warning("SD-WAN ", run.job.Kind, " failed: ", err)
		run.log("error", "job_failed", sdwanParams("error", err.Error()))
	}
	run.update(func(job *SdwanJob) {
		job.EndedAt = time.Now().Unix()
		job.Progress = 100
		job.Stage = "done"
		job.Status = SdwanJobStatusDone
		if err != nil {
			job.Status = SdwanJobStatusFailed
			job.Error = err.Error()
		}
	})
}

func (s *SdwanService) runSdwanDiagnose(ctx context.Context, run *sdwanJobRun) error {
	report, err := s.collectSdwanReport(ctx, run, run.job.Bandwidth, 0, 100)
	if err != nil {
		return err
	}
	run.update(func(job *SdwanJob) { job.Report = report })
	run.log("success", "diagnose_done", sdwanParams(
		"score", strconv.Itoa(report.Score), "advice", strconv.Itoa(len(report.Advice)),
		"fixable", strconv.Itoa(countFixableSdwanAdvice(report)),
	))
	return nil
}

// runSdwanOptimize measures, tunes the kernels of the controller and every
// member, repairs what detection found, applies the result and measures again
// so the page can show before and after.
func (s *SdwanService) runSdwanOptimize(ctx context.Context, run *sdwanJobRun) error {
	before, err := s.collectSdwanReport(ctx, run, run.job.Bandwidth, 0, 40)
	if err != nil {
		return err
	}
	run.update(func(job *SdwanJob) { job.Before = before })

	run.stage("tuning", 40)
	s.tuneSdwanKernels(run, before)

	run.stage("repair", 55)
	sdwanMu.Lock()
	changed := s.repairSdwanLocked(run, before)
	if changed || !before.CoreRunning {
		run.stage("apply", 65)
		if applyErr := s.apply(); applyErr != nil {
			run.log("error", "core_restart_failed", sdwanParams("error", applyErr.Error()))
		} else {
			run.log("success", "core_restarted", nil)
		}
	}
	sdwanMu.Unlock()

	run.stage("remeasure", 70)
	if corePtr != nil && corePtr.IsRunning() {
		// Let the group pick the best path right away instead of waiting
		// for its next periodic test.
		waitSdwanGroupReady(ctx)
		_ = corePtr.GroupCheckNow(SdwanGroupTag)
	}
	after, err := s.collectSdwanReport(ctx, run, run.job.Bandwidth, 70, 100)
	if err != nil {
		return err
	}
	run.update(func(job *SdwanJob) { job.Report = after })
	if len(currentSdwanJob().Changes) == 0 {
		run.log("info", "optimize_nothing", nil)
	}
	run.log("success", "optimize_done", sdwanParams("before", strconv.Itoa(before.Score), "after", strconv.Itoa(after.Score)))
	return nil
}

func (s *SdwanService) tuneSdwanKernels(run *sdwanJobRun, report *SdwanReport) {
	logSdwanTuning(run, "", ApplySdwanTuning())
	for _, node := range report.Nodes {
		if !node.Online || !node.Supported {
			continue
		}
		response, err := s.dispatch(node.NodeID, agent.RPCMethodSdwanTune, struct{}{}, run.actor)
		var result SdwanTuningResult
		if err == nil {
			err = json.Unmarshal(response.Payload, &result)
		}
		if err != nil {
			run.log("warning", "tuning_node_error", sdwanParams("node", node.Name, "error", err.Error()))
			continue
		}
		logSdwanTuning(run, node.Name, result)
	}
}

// logSdwanTuning records a kernel tuning result; node is empty for the
// controller itself.
func logSdwanTuning(run *sdwanJobRun, node string, result SdwanTuningResult) {
	code := "tuning_controller"
	if node != "" {
		code = "tuning_node"
	}
	if !result.Before.Supported {
		run.log("info", code+"_skipped", sdwanParams("node", node, "reason", result.Before.Reason))
		return
	}
	if len(result.Applied) > 0 {
		run.change(code, sdwanParams(
			"node", node, "count", strconv.Itoa(len(result.Applied)), "keys", strings.Join(result.Applied, ", "),
			"cc", result.After.CongestionControl, "qdisc", result.After.Qdisc,
		))
	} else if len(result.Failed) == 0 {
		run.log("info", code+"_ok", sdwanParams("node", node, "cc", result.After.CongestionControl))
	}
	for _, failure := range result.Failed {
		run.log("warning", "tuning_key_failed", sdwanParams("node", node, "key", failure.Key, "error", failure.Error))
	}
}

func hasSdwanAdvice(report *SdwanReport, code string, nodeID uint) bool {
	return slices.ContainsFunc(report.Advice, func(advice SdwanAdvice) bool {
		return advice.Code == code && advice.NodeID == nodeID
	})
}

func countFixableSdwanAdvice(report *SdwanReport) int {
	count := 0
	for _, advice := range report.Advice {
		if advice.Fixable {
			count++
		}
	}
	return count
}

// repairSdwanLocked applies the fixable findings of a report: members that
// need other protocols or have a broken uplink are re-provisioned, a path that
// is unreachable while another path of the same server works leaves the group,
// recovered paths rejoin it and the tolerance follows the measured jitter.
func (s *SdwanService) repairSdwanLocked(run *sdwanJobRun, report *SdwanReport) bool {
	db := database.GetDB()
	settings, _ := loadSdwanSettings(db)
	changed := false
	reprovisioned := map[uint]bool{}
	for _, node := range report.Nodes {
		upgrade := hasSdwanAdvice(report, "protocol_upgrade", node.NodeID)
		rotate := hasSdwanAdvice(report, "uplink_down", node.NodeID)
		if (!upgrade && !rotate) || !node.Online || !node.Supported {
			continue
		}
		if err := s.provisionLocked(node.NodeID, settings, run.actor, rotate); err != nil {
			run.log("error", "reprovision_failed", sdwanParams("node", node.Name, "error", err.Error()))
			continue
		}
		reprovisioned[node.NodeID] = true
		changed = true
		code := "protocol_upgraded"
		if rotate {
			code = "uplink_rebuilt"
		}
		run.change(code, sdwanParams("node", node.Name, "protocols", strings.Join(node.Desired, "+")))
	}
	for _, advice := range report.Advice {
		if advice.NodeID == 0 || reprovisioned[advice.NodeID] {
			continue
		}
		var disable bool
		switch advice.Code {
		case "path_unreachable":
			disable = true
		case "path_recovered":
			disable = false
		default:
			continue
		}
		reason := ""
		if disable {
			reason = "unreachable"
		}
		if err := updateSdwanPathState(advice.NodeID, advice.Protocol, disable, reason); err != nil {
			run.log("error", "path_update_failed", sdwanParams("node", advice.Params["node"], "protocol", advice.Protocol, "error", err.Error()))
			continue
		}
		changed = true
		code := "path_enabled"
		if disable {
			code = "path_disabled"
		}
		run.change(code, sdwanParams("node", advice.Params["node"], "protocol", advice.Protocol))
	}
	if report.RecommendedTolerance > 0 && hasSdwanAdvice(report, "tolerance", 0) && settings.Tolerance != report.RecommendedTolerance {
		previous := settings.Tolerance
		settings.Tolerance = report.RecommendedTolerance
		if err := storeSdwanSettings(db, settings); err != nil {
			run.log("error", "settings_failed", sdwanParams("error", err.Error()))
		} else {
			changed = true
			run.change("tolerance_changed", sdwanParams("from", strconv.Itoa(previous), "to", strconv.Itoa(settings.Tolerance)))
		}
	}
	return changed
}

type sdwanPathRef struct {
	node int
	path int
}

// collectSdwanReport checks the controller and every member, measures each
// path through the running sing-box and analyses the result. Progress moves
// from `from` to `to` percent.
func (s *SdwanService) collectSdwanReport(ctx context.Context, run *sdwanJobRun, bandwidth bool, from, to int) (*SdwanReport, error) {
	db := database.GetDB()
	settings, _ := loadSdwanSettings(db)
	report := &SdwanReport{
		Time: time.Now().Unix(), Nodes: []SdwanNodeReport{}, Advice: []SdwanAdvice{},
		Tolerance: settings.Tolerance, Bandwidth: bandwidth,
	}
	span := to - from
	run.stage("controller", from)
	run.log("info", "check_controller", nil)
	report.Controller = ReadSdwanTuning()
	report.CoreRunning = corePtr != nil && corePtr.IsRunning()
	if !report.CoreRunning {
		run.log("warning", "core_stopped", nil)
	}

	var members []model.SdwanMember
	if err := db.Order("node_id").Find(&members).Error; err != nil {
		return nil, err
	}
	nodes, err := s.nodes()
	if err != nil {
		return nil, err
	}
	nodeByID := make(map[uint]AgentNodeView, len(nodes))
	for _, node := range nodes {
		nodeByID[node.Id] = node
	}
	report.Nodes = make([]SdwanNodeReport, len(members))
	var diagnosable []int
	var paths []sdwanPathRef
	for index, member := range members {
		node, known := nodeByID[member.NodeId]
		nodeReport := &report.Nodes[index]
		*nodeReport = SdwanNodeReport{
			NodeID: member.NodeId, Name: node.Name, Online: known && node.Managed,
			Supported: known && sdwanNodeSupported(&node),
			Desired:   sdwanProtocolsFor(settings.Mode, node.Report.Panel.Capabilities),
			Uplinks:   []SdwanUplinkStatus{}, Paths: []SdwanPathReport{},
		}
		if nodeReport.Name == "" {
			nodeReport.Name = fmt.Sprintf("#%d", member.NodeId)
		}
		for _, path := range memberPaths(member) {
			nodeReport.Paths = append(nodeReport.Paths, SdwanPathReport{
				Protocol: path.Protocol, Tag: sdwanPathTag(member.NodeId, path.Protocol), Port: path.Port, Disabled: path.Disabled,
			})
			paths = append(paths, sdwanPathRef{node: index, path: len(nodeReport.Paths) - 1})
		}
		switch {
		case !nodeReport.Online:
			run.log("warning", "node_offline", sdwanParams("node", nodeReport.Name))
		case !nodeReport.Supported:
			run.log("warning", "node_outdated", sdwanParams("node", nodeReport.Name))
		default:
			diagnosable = append(diagnosable, index)
		}
	}

	run.stage("nodes", from+span/10)
	runSdwanParallel(len(diagnosable), func(index int) {
		nodeReport := &report.Nodes[diagnosable[index]]
		s.diagnoseSdwanNode(nodeReport, run.actor)
		if nodeReport.Error != "" {
			run.log("warning", "node_diagnose_failed", sdwanParams("node", nodeReport.Name, "error", nodeReport.Error))
			return
		}
		skew := ""
		if nodeReport.ClockSkewMS != nil {
			skew = strconv.FormatFloat(float64(*nodeReport.ClockSkewMS)/1000, 'f', 1, 64)
		}
		run.log("info", "node_checked", sdwanParams("node", nodeReport.Name, "skew", skew))
	}, func(done int) {
		run.progress(from+span/10, from+span*3/10, done, len(diagnosable))
	})

	latencyEnd := from + span*9/10
	if bandwidth {
		latencyEnd = from + span*6/10
	}
	run.stage("latency", from+span*3/10)
	runSdwanParallel(len(paths), func(index int) {
		nodeReport := &report.Nodes[paths[index].node]
		path := &nodeReport.Paths[paths[index].path]
		if !report.CoreRunning {
			path.Error = "sing-box is not running"
			return
		}
		if !sdwanPathLoaded(path.Tag) {
			path.Error = "this path is not loaded in sing-box"
			run.log("warning", "path_not_loaded", sdwanParams("node", nodeReport.Name, "protocol", path.Protocol))
			return
		}
		measureSdwanPath(ctx, path, settings.TestURL)
		if path.Reachable {
			run.log("info", "path_measured", sdwanParams(
				"node", nodeReport.Name, "protocol", path.Protocol, "latency", strconv.Itoa(path.Latency),
				"jitter", strconv.Itoa(path.Jitter), "loss", strconv.Itoa(path.Loss),
			))
		} else {
			run.log("warning", "path_failed", sdwanParams("node", nodeReport.Name, "protocol", path.Protocol, "error", path.Error))
		}
	}, func(done int) {
		run.progress(from+span*3/10, latencyEnd, done, len(paths))
	})

	if bandwidth && report.CoreRunning {
		run.stage("bandwidth", latencyEnd)
		var reachable []sdwanPathRef
		for _, ref := range paths {
			if report.Nodes[ref.node].Paths[ref.path].Reachable {
				reachable = append(reachable, ref)
			}
		}
		// One at a time: parallel downloads would compete for bandwidth.
		for index, ref := range reachable {
			if ctx.Err() != nil {
				break
			}
			nodeReport := &report.Nodes[ref.node]
			path := &nodeReport.Paths[ref.path]
			speedCtx, cancel := context.WithTimeout(ctx, sdwanSpeedTestTimeout)
			mbps, speedErr := sdwanSpeedPath(speedCtx, path.Tag, settings.SpeedTestURL)
			cancel()
			if speedErr != nil {
				path.SpeedError = speedErr.Error()
				run.log("warning", "speed_failed", sdwanParams("node", nodeReport.Name, "protocol", path.Protocol, "error", path.SpeedError))
			} else {
				path.Mbps = math.Round(mbps*10) / 10
				run.log("info", "speed_measured", sdwanParams(
					"node", nodeReport.Name, "protocol", path.Protocol, "mbps", strconv.FormatFloat(path.Mbps, 'f', 1, 64),
				))
			}
			run.progress(latencyEnd, from+span*9/10, index+1, len(reachable))
		}
	}

	run.stage("analysis", from+span*9/10)
	if report.CoreRunning {
		if status, err := corePtr.GroupStatus(SdwanGroupTag); err == nil {
			report.Selected = status.Now
		}
	}
	analyzeSdwanReport(report, settings)
	run.progress(from, to, 1, 1)
	return report, nil
}

// waitSdwanGroupReady waits for the URL test sing-box starts with the group:
// a forced re-test is skipped while another one is still running.
func waitSdwanGroupReady(ctx context.Context) {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if corePtr == nil || !corePtr.IsRunning() {
			return
		}
		if status, err := corePtr.GroupStatus(SdwanGroupTag); err != nil || status.Now != "" {
			return
		}
		if !sleepContext(ctx, 100*time.Millisecond) {
			return
		}
	}
}

// runSdwanParallel runs work for every index with bounded concurrency and
// reports how many items are done after each one.
func runSdwanParallel(count int, work func(index int), progress func(done int)) {
	if count == 0 {
		return
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	semaphore := make(chan struct{}, sdwanMeasureParallel)
	for index := 0; index < count; index++ {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(index int) {
			defer wg.Done()
			defer func() { <-semaphore }()
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("SD-WAN measurement panicked: ", recovered)
				}
			}()
			work(index)
			mu.Lock()
			done++
			current := done
			mu.Unlock()
			progress(current)
		}(index)
	}
	wg.Wait()
}

func (s *SdwanService) diagnoseSdwanNode(report *SdwanNodeReport, actor string) {
	sent := time.Now()
	response, err := s.dispatch(report.NodeID, agent.RPCMethodSdwanDiagnose, struct{}{}, actor)
	received := time.Now()
	var diagnosis SdwanDiagnoseResponse
	if err == nil {
		err = json.Unmarshal(response.Payload, &diagnosis)
	}
	if err != nil {
		report.Error = err.Error()
		return
	}
	report.Diagnosed = true
	report.CoreRunning = diagnosis.CoreRunning
	report.Tuning = &diagnosis.Tuning
	if diagnosis.Uplinks != nil {
		report.Uplinks = diagnosis.Uplinks
	}
	if diagnosis.TimeMS > 0 {
		// The remote clock was read roughly halfway through the round trip.
		skew := diagnosis.TimeMS - (sent.UnixMilli() + received.Sub(sent).Milliseconds()/2)
		report.ClockSkewMS = &skew
	}
}

// measureSdwanPath takes several URL-test samples through one path.
func measureSdwanPath(ctx context.Context, path *SdwanPathReport, testURL string) {
	path.Measured = true
	path.Samples, path.Failures, path.Error = 0, 0, ""
	delays := make([]int, 0, sdwanLatencySamples)
	for sample := 0; sample < sdwanLatencySamples; sample++ {
		if sample > 0 && !sleepContext(ctx, sdwanSampleGap) {
			break
		}
		sampleCtx, cancel := context.WithTimeout(ctx, sdwanSampleTimeout)
		delay, err := sdwanCheckPath(sampleCtx, path.Tag, testURL)
		cancel()
		path.Samples++
		if err != nil {
			path.Failures++
			path.Error = err.Error()
			continue
		}
		delays = append(delays, int(delay))
	}
	summarizeSdwanSamples(path, delays)
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// summarizeSdwanSamples turns URL-test samples into the median latency, the
// jitter (mean difference between consecutive samples, leaving out the first
// one when possible because it includes connection warm-up) and the loss.
func summarizeSdwanSamples(path *SdwanPathReport, delays []int) {
	path.Reachable = len(delays) > 0
	path.Latency, path.Min, path.Jitter, path.Loss = 0, 0, 0, 0
	if path.Samples > 0 {
		path.Loss = path.Failures * 100 / path.Samples
	}
	if len(delays) == 0 {
		return
	}
	sorted := slices.Sorted(slices.Values(delays))
	path.Min = sorted[0]
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		path.Latency = sorted[middle]
	} else {
		path.Latency = (sorted[middle-1] + sorted[middle] + 1) / 2
	}
	series := delays
	if len(series) >= 3 {
		series = series[1:]
	}
	if len(series) >= 2 {
		total := 0
		for index := 1; index < len(series); index++ {
			total += absInt(series[index] - series[index-1])
		}
		path.Jitter = int(math.Round(float64(total) / float64(len(series)-1)))
	}
}

func sdwanPathTransport(protocol string) string {
	if protocol == SdwanProtocolHysteria2 {
		return "UDP"
	}
	return "TCP"
}

func sameProtocolSet(current, desired []string) bool {
	if len(current) != len(desired) {
		return false
	}
	for _, protocol := range current {
		if !slices.Contains(desired, protocol) {
			return false
		}
	}
	return true
}

// recommendSdwanTolerance sizes the urltest tolerance to the measured jitter:
// large enough that noise does not flip the exit between equivalent paths,
// small enough that a clearly faster path still wins.
func recommendSdwanTolerance(jitters []int) int {
	sorted := slices.Sorted(slices.Values(jitters))
	value := 2*sorted[len(sorted)/2] + 20
	value = (value + 9) / 10 * 10
	return min(max(value, 30), 200)
}

// analyzeSdwanReport turns measurements into findings, marks the best path
// and computes a 0-100 health score.
func analyzeSdwanReport(report *SdwanReport, settings SdwanSettings) {
	advice := []SdwanAdvice{}
	add := func(item SdwanAdvice) { advice = append(advice, item) }
	if !report.CoreRunning {
		add(SdwanAdvice{Code: "core_not_running", Severity: sdwanSeverityError, Fixable: true})
	}
	if len(report.Nodes) == 0 {
		add(SdwanAdvice{Code: "no_members", Severity: sdwanSeverityInfo})
	} else if !settings.Enabled || len(settings.EntryInbounds) == 0 {
		add(SdwanAdvice{Code: "routing_disabled", Severity: sdwanSeverityInfo})
	}
	if !report.Controller.Supported {
		add(SdwanAdvice{Code: "controller_tuning_unsupported", Severity: sdwanSeverityInfo, Params: sdwanParams("reason", report.Controller.Reason)})
	} else if len(report.Controller.Pending) > 0 {
		add(SdwanAdvice{Code: "controller_tuning", Severity: sdwanSeverityWarning, Fixable: true, Params: sdwanParams(
			"count", strconv.Itoa(len(report.Controller.Pending)), "keys", strings.Join(report.Controller.Pending, ", "),
		)})
	}

	var jitters []int
	var best *SdwanPathReport
	for nodeIndex := range report.Nodes {
		node := &report.Nodes[nodeIndex]
		nodeAdvice := func(code, severity string, fixable bool, protocol string, params ...string) {
			add(SdwanAdvice{
				Code: code, Severity: severity, Fixable: fixable, NodeID: node.NodeID, Protocol: protocol,
				Params: sdwanParams(append([]string{"node", node.Name}, params...)...),
			})
		}
		switch {
		case !node.Online:
			nodeAdvice("node_offline", sdwanSeverityError, false, "")
		case !node.Supported:
			nodeAdvice("node_outdated", sdwanSeverityError, false, "")
		case node.Error != "":
			nodeAdvice("node_diagnose_failed", sdwanSeverityWarning, false, "", "error", node.Error)
		}
		current := make([]string, 0, len(node.Paths))
		for _, path := range node.Paths {
			current = append(current, path.Protocol)
		}
		if node.Online && node.Supported && !sameProtocolSet(current, node.Desired) {
			nodeAdvice("protocol_upgrade", sdwanSeverityWarning, true, "",
				"current", strings.Join(current, "+"), "desired", strings.Join(node.Desired, "+"))
		}
		if node.Diagnosed {
			if !node.CoreRunning {
				nodeAdvice("node_core_down", sdwanSeverityError, false, "")
			} else {
				for _, path := range node.Paths {
					index := slices.IndexFunc(node.Uplinks, func(uplink SdwanUplinkStatus) bool { return uplink.Protocol == path.Protocol })
					if path.Disabled || (index >= 0 && node.Uplinks[index].Running) {
						continue
					}
					nodeAdvice("uplink_down", sdwanSeverityError, true, path.Protocol,
						"protocol", path.Protocol, "port", strconv.Itoa(path.Port))
				}
			}
			if node.Tuning != nil {
				if !node.Tuning.Supported {
					nodeAdvice("node_tuning_unsupported", sdwanSeverityInfo, false, "", "reason", node.Tuning.Reason)
				} else if len(node.Tuning.Pending) > 0 {
					nodeAdvice("node_tuning", sdwanSeverityWarning, true, "",
						"count", strconv.Itoa(len(node.Tuning.Pending)), "keys", strings.Join(node.Tuning.Pending, ", "))
				}
			}
			if node.ClockSkewMS != nil {
				skew := *node.ClockSkewMS
				seconds := strconv.FormatInt(skew/1000, 10)
				switch magnitude := max(skew, -skew); {
				case magnitude >= sdwanClockSkewErrorMS:
					severity := sdwanSeverityWarning
					if slices.ContainsFunc(node.Paths, func(path SdwanPathReport) bool {
						return path.Protocol == SdwanProtocolShadowsocks && !path.Disabled
					}) {
						severity = sdwanSeverityError
					}
					nodeAdvice("clock_skew", severity, false, "", "seconds", seconds)
				case magnitude >= sdwanClockSkewWarnMS:
					nodeAdvice("clock_skew", sdwanSeverityInfo, false, "", "seconds", seconds)
				}
			}
		}

		measured, reachable := 0, 0
		for _, path := range node.Paths {
			if path.Measured && !path.Disabled {
				measured++
				if path.Reachable {
					reachable++
				}
			}
		}
		var blocked []string
		for pathIndex := range node.Paths {
			path := &node.Paths[pathIndex]
			path.Selected = report.Selected != "" && path.Tag == report.Selected
			if !path.Measured {
				continue
			}
			if path.Disabled {
				if path.Reachable && path.Loss < sdwanLossyPercent {
					nodeAdvice("path_recovered", sdwanSeverityInfo, true, path.Protocol, "protocol", path.Protocol)
				}
				continue
			}
			if !path.Reachable {
				transport := sdwanPathTransport(path.Protocol)
				blocked = append(blocked, transport+" "+strconv.Itoa(path.Port))
				if reachable > 0 {
					nodeAdvice("path_unreachable", sdwanSeverityWarning, true, path.Protocol,
						"protocol", path.Protocol, "transport", transport, "port", strconv.Itoa(path.Port))
				}
				continue
			}
			if path.Loss >= sdwanLossyPercent {
				nodeAdvice("path_lossy", sdwanSeverityWarning, false, path.Protocol,
					"protocol", path.Protocol, "loss", strconv.Itoa(path.Loss))
			}
			if path.Jitter >= max(30, path.Latency/2) {
				nodeAdvice("path_jitter", sdwanSeverityInfo, false, path.Protocol,
					"protocol", path.Protocol, "jitter", strconv.Itoa(path.Jitter))
			}
			jitters = append(jitters, path.Jitter)
			if path.Loss < 50 && (best == nil || path.Latency < best.Latency) {
				best = path
			}
		}
		if measured > 0 && reachable == 0 {
			nodeAdvice("node_unreachable", sdwanSeverityError, false, "", "ports", strings.Join(blocked, ", "))
		}
	}
	if best != nil {
		best.Best = true
		report.BestTag = best.Tag
		report.BestLatency = best.Latency
	}
	if len(jitters) >= 2 {
		report.RecommendedTolerance = recommendSdwanTolerance(jitters)
		if absInt(report.RecommendedTolerance-report.Tolerance) >= 20 {
			add(SdwanAdvice{Code: "tolerance", Severity: sdwanSeverityInfo, Fixable: true, Params: sdwanParams(
				"current", strconv.Itoa(report.Tolerance), "recommended", strconv.Itoa(report.RecommendedTolerance),
			)})
		}
	}
	report.Advice = advice
	report.Score = sdwanHealthScore(report)
}

func sdwanHealthScore(report *SdwanReport) int {
	if len(report.Nodes) == 0 {
		return 0
	}
	score := 100
	for _, advice := range report.Advice {
		switch advice.Severity {
		case sdwanSeverityError:
			score -= 25
		case sdwanSeverityWarning:
			score -= 10
		}
	}
	return max(score, 0)
}
