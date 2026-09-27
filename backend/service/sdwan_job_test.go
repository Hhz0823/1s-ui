package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
)

func TestSummarizeSdwanSamples(t *testing.T) {
	path := &SdwanPathReport{Samples: 5}
	summarizeSdwanSamples(path, []int{120, 50, 52, 48, 54})
	if !path.Reachable || path.Latency != 52 || path.Min != 48 || path.Jitter != 4 || path.Loss != 0 {
		t.Fatalf("stable path = %#v (the warm-up sample must not count as jitter)", path)
	}
	path = &SdwanPathReport{Samples: 4, Failures: 2}
	summarizeSdwanSamples(path, []int{30, 40})
	if !path.Reachable || path.Latency != 35 || path.Jitter != 10 || path.Loss != 50 {
		t.Fatalf("lossy path = %#v", path)
	}
	path = &SdwanPathReport{Samples: 5, Failures: 5}
	summarizeSdwanSamples(path, nil)
	if path.Reachable || path.Latency != 0 || path.Loss != 100 {
		t.Fatalf("dead path = %#v", path)
	}
}

func TestRecommendSdwanTolerance(t *testing.T) {
	for _, test := range []struct {
		jitters []int
		want    int
	}{
		{[]int{0, 1}, 30},
		{[]int{12, 3, 40}, 50},
		{[]int{100, 200, 150}, 200},
	} {
		if got := recommendSdwanTolerance(test.jitters); got != test.want {
			t.Fatalf("tolerance for %v = %d, want %d", test.jitters, got, test.want)
		}
	}
}

func findSdwanAdvice(report *SdwanReport, code string, nodeID uint) *SdwanAdvice {
	for index := range report.Advice {
		if report.Advice[index].Code == code && report.Advice[index].NodeID == nodeID {
			return &report.Advice[index]
		}
	}
	return nil
}

func TestAnalyzeSdwanReportFindsProblems(t *testing.T) {
	skew := int64(45_000)
	report := &SdwanReport{
		CoreRunning: true, Selected: "sdwan-node-1-hy2", Tolerance: 50,
		Controller: SdwanTuningStatus{Supported: true, Pending: []string{"net.ipv4.tcp_congestion_control"}},
		Nodes: []SdwanNodeReport{
			{
				NodeID: 1, Name: "hk", Online: true, Supported: true, Diagnosed: true, CoreRunning: true,
				Desired: []string{SdwanProtocolReality, SdwanProtocolHysteria2}, Tuning: &SdwanTuningStatus{Supported: true},
				Uplinks: []SdwanUplinkStatus{{Protocol: SdwanProtocolReality, Running: true}, {Protocol: SdwanProtocolHysteria2, Running: true}},
				Paths: []SdwanPathReport{
					{Protocol: SdwanProtocolReality, Tag: "sdwan-node-1-reality", Port: 443, Measured: true, Reachable: true, Samples: 5, Latency: 80, Jitter: 4},
					{Protocol: SdwanProtocolHysteria2, Tag: "sdwan-node-1-hy2", Port: 8443, Measured: true, Samples: 5, Failures: 5, Loss: 100},
				},
			},
			{
				NodeID: 2, Name: "jp", Online: true, Supported: true, Diagnosed: true, CoreRunning: true, ClockSkewMS: &skew,
				Desired: []string{SdwanProtocolReality, SdwanProtocolHysteria2},
				Tuning:  &SdwanTuningStatus{Supported: true, Pending: []string{"net.core.rmem_max"}},
				Uplinks: []SdwanUplinkStatus{{Protocol: SdwanProtocolShadowsocks, Port: 8388, Running: false}},
				Paths: []SdwanPathReport{
					{Protocol: SdwanProtocolShadowsocks, Tag: "sdwan-node-2-ss", Port: 8388, Measured: true, Reachable: true, Samples: 5, Failures: 2, Loss: 40, Latency: 60, Jitter: 30},
				},
			},
			{
				NodeID: 3, Name: "us", Online: false, Supported: true, Desired: []string{SdwanProtocolReality},
				Paths: []SdwanPathReport{{Protocol: SdwanProtocolReality, Tag: "sdwan-node-3-reality", Port: 443, Measured: true, Samples: 5, Failures: 5, Loss: 100}},
			},
			{
				NodeID: 4, Name: "sg", Online: true, Supported: false, Desired: []string{SdwanProtocolShadowsocks},
				Paths: []SdwanPathReport{{Protocol: SdwanProtocolShadowsocks, Tag: "sdwan-node-4-ss", Port: 9000, Disabled: true, Measured: true, Reachable: true, Samples: 5, Latency: 90}},
			},
		},
	}
	analyzeSdwanReport(report, SdwanSettings{Enabled: true, EntryInbounds: []string{"in"}})

	expect := func(code string, nodeID uint, severity string, fixable bool) *SdwanAdvice {
		t.Helper()
		advice := findSdwanAdvice(report, code, nodeID)
		if advice == nil {
			t.Fatalf("missing %s for node %d in %#v", code, nodeID, report.Advice)
		}
		if advice.Severity != severity || advice.Fixable != fixable {
			t.Fatalf("%s for node %d = %#v", code, nodeID, advice)
		}
		return advice
	}
	expect("controller_tuning", 0, sdwanSeverityWarning, true)
	unreachable := expect("path_unreachable", 1, sdwanSeverityWarning, true)
	if unreachable.Protocol != SdwanProtocolHysteria2 || unreachable.Params["transport"] != "UDP" || unreachable.Params["port"] != "8443" {
		t.Fatalf("a dead Hysteria2 path must point at its UDP port: %#v", unreachable)
	}
	expect("protocol_upgrade", 2, sdwanSeverityWarning, true)
	expect("uplink_down", 2, sdwanSeverityError, true)
	expect("node_tuning", 2, sdwanSeverityWarning, true)
	if skewAdvice := expect("clock_skew", 2, sdwanSeverityError, false); skewAdvice.Params["seconds"] != "45" {
		t.Fatalf("clock skew = %#v", skewAdvice)
	}
	expect("path_lossy", 2, sdwanSeverityWarning, false)
	expect("node_offline", 3, sdwanSeverityError, false)
	if blocked := expect("node_unreachable", 3, sdwanSeverityError, false); blocked.Params["ports"] != "TCP 443" {
		t.Fatalf("node_unreachable must list the ports to open: %#v", blocked)
	}
	expect("node_outdated", 4, sdwanSeverityError, false)
	// Re-enabling a path needs no RPC, so it works even for outdated servers.
	expect("path_recovered", 4, sdwanSeverityInfo, true)
	for _, unexpected := range []struct {
		code string
		node uint
	}{{"node_unreachable", 1}, {"protocol_upgrade", 1}, {"protocol_upgrade", 4}, {"routing_disabled", 0}} {
		if findSdwanAdvice(report, unexpected.code, unexpected.node) != nil {
			t.Fatalf("unexpected %s for node %d", unexpected.code, unexpected.node)
		}
	}
	if tolerance := expect("tolerance", 0, sdwanSeverityInfo, true); tolerance.Params["recommended"] != "80" || report.RecommendedTolerance != 80 {
		t.Fatalf("tolerance advice = %#v", tolerance)
	}
	if report.BestTag != "sdwan-node-2-ss" || report.BestLatency != 60 || !report.Nodes[1].Paths[0].Best {
		t.Fatalf("best path = %s (%d ms)", report.BestTag, report.BestLatency)
	}
	if !report.Nodes[0].Paths[1].Selected || report.Nodes[0].Paths[0].Selected {
		t.Fatal("the path the group currently uses must be marked")
	}
	if report.Score != 0 {
		t.Fatalf("score = %d", report.Score)
	}

	// A path whose uplink is missing on the server is a rebuild, not a
	// firewall problem.
	missing := &SdwanReport{
		CoreRunning: true, Tolerance: 50, Controller: SdwanTuningStatus{Supported: true},
		Nodes: []SdwanNodeReport{{
			NodeID: 5, Name: "de", Online: true, Supported: true, Diagnosed: true, CoreRunning: true,
			Desired: []string{SdwanProtocolShadowsocks}, Tuning: &SdwanTuningStatus{Supported: true}, Uplinks: []SdwanUplinkStatus{},
			Paths: []SdwanPathReport{{Protocol: SdwanProtocolShadowsocks, Tag: "sdwan-node-5-ss", Port: 39741, Measured: true, Samples: 5, Failures: 5, Loss: 100}},
		}},
	}
	analyzeSdwanReport(missing, SdwanSettings{Enabled: true, EntryInbounds: []string{"in"}})
	if findSdwanAdvice(missing, "uplink_down", 5) == nil || findSdwanAdvice(missing, "node_unreachable", 5) != nil || findSdwanAdvice(missing, "path_unreachable", 5) != nil {
		t.Fatalf("missing uplink findings = %#v", missing.Advice)
	}

	healthy := &SdwanReport{
		CoreRunning: true, Tolerance: 50, Controller: SdwanTuningStatus{Supported: true},
		Nodes: []SdwanNodeReport{{
			NodeID: 1, Name: "hk", Online: true, Supported: true, Diagnosed: true, CoreRunning: true,
			Desired: []string{SdwanProtocolShadowsocks}, Tuning: &SdwanTuningStatus{Supported: true},
			Uplinks: []SdwanUplinkStatus{{Protocol: SdwanProtocolShadowsocks, Running: true}},
			Paths:   []SdwanPathReport{{Protocol: SdwanProtocolShadowsocks, Tag: "sdwan-node-1-ss", Measured: true, Reachable: true, Samples: 5, Latency: 40, Jitter: 2}},
		}},
	}
	analyzeSdwanReport(healthy, SdwanSettings{Enabled: true, EntryInbounds: []string{"in"}})
	if healthy.Score != 100 || len(healthy.Advice) != 0 {
		t.Fatalf("healthy network = %d %#v", healthy.Score, healthy.Advice)
	}
}

func setSdwanJobForTest(t *testing.T, job *SdwanJob) {
	t.Helper()
	sdwanJobMu.Lock()
	previous := sdwanJob
	sdwanJob = job
	sdwanJobMu.Unlock()
	t.Cleanup(func() {
		sdwanJobMu.Lock()
		sdwanJob = previous
		sdwanJobMu.Unlock()
	})
}

func TestSdwanChangesWaitForRunningJob(t *testing.T) {
	setupQuickAddTest(t)
	if _, err := (&SettingService{}).SetControllerMode(true); err != nil {
		t.Fatal(err)
	}
	setSdwanJobForTest(t, &SdwanJob{Status: SdwanJobStatusRunning, Logs: []SdwanJobLog{}, Changes: []SdwanJobLog{}})
	service := &SdwanService{restartCore: func() error { return nil }}
	if _, err := service.SaveSdwanSettings(defaultSdwanSettings(), "admin"); err != errSdwanJobRunning {
		t.Fatalf("settings changed during a job: %v", err)
	}
	if _, err := service.ResyncSdwan("admin"); err != errSdwanJobRunning {
		t.Fatalf("resync during a job: %v", err)
	}
	if _, err := service.StartSdwanJob(SdwanJobDiagnose, false, "admin"); err != errSdwanJobRunning {
		t.Fatalf("two jobs at once: %v", err)
	}
	if _, err := service.StartSdwanJob("reboot", false, "admin"); err == nil {
		t.Fatal("unknown job kind accepted")
	}
}

func TestSdwanRepairTogglesPathsAndTolerance(t *testing.T) {
	setupQuickAddTest(t)
	db := database.GetDB()
	valid := json.RawMessage(`{"type":"shadowsocks","server":"198.51.100.61","server_port":8388,"method":"2022-blake3-aes-128-gcm","password":"AAAAAAAAAAAAAAAAAAAAAA==:AAAAAAAAAAAAAAAAAAAAAA=="}`)
	db.Create(&model.SdwanMember{NodeId: 1, Paths: sdwanTestPaths(t, SdwanPath{Protocol: SdwanProtocolShadowsocks, Outbound: valid})})
	db.Create(&model.SdwanMember{NodeId: 2, Paths: sdwanTestPaths(t, SdwanPath{Protocol: SdwanProtocolShadowsocks, Outbound: valid, Disabled: true, Reason: "unreachable"})})
	if err := storeSdwanSettings(db, defaultSdwanSettings()); err != nil {
		t.Fatal(err)
	}
	report := &SdwanReport{
		RecommendedTolerance: 120,
		Nodes:                []SdwanNodeReport{{NodeID: 1, Name: "hk"}, {NodeID: 2, Name: "jp"}},
		Advice: []SdwanAdvice{
			{Code: "path_unreachable", NodeID: 1, Protocol: SdwanProtocolShadowsocks, Fixable: true, Params: sdwanParams("node", "hk")},
			{Code: "path_recovered", NodeID: 2, Protocol: SdwanProtocolShadowsocks, Fixable: true, Params: sdwanParams("node", "jp")},
			{Code: "tolerance", Fixable: true},
		},
	}
	run := &sdwanJobRun{job: &SdwanJob{Logs: []SdwanJobLog{}, Changes: []SdwanJobLog{}}}
	service := &SdwanService{}
	if !service.repairSdwanLocked(run, report) {
		t.Fatal("repair reported no change")
	}
	var codes []string
	for _, change := range run.job.Changes {
		codes = append(codes, change.Code)
	}
	if !slices.Equal(codes, []string{"path_disabled", "path_enabled", "tolerance_changed"}) {
		t.Fatalf("changes = %v", codes)
	}
	settings, _ := loadSdwanSettings(db)
	if settings.Tolerance != 120 {
		t.Fatalf("tolerance = %d", settings.Tolerance)
	}
	config := SingBoxConfig{}
	applySdwanConfig(db, &config)
	for _, raw := range config.Outbounds {
		var group struct {
			Tag       string   `json:"tag"`
			Outbounds []string `json:"outbounds"`
			Tolerance int      `json:"tolerance"`
		}
		if json.Unmarshal(raw, &group) == nil && group.Tag == SdwanGroupTag {
			if !slices.Equal(group.Outbounds, []string{sdwanPathTag(2, SdwanProtocolShadowsocks)}) || group.Tolerance != 120 {
				t.Fatalf("group after repair = %#v", group)
			}
			return
		}
	}
	t.Fatal("SD-WAN group missing after repair")
}

// sdwanHarness runs the controller and one managed server in a single
// process: the server's RPCs are served by LocalControlService on the same
// database, and applying the configuration restarts one sing-box instance
// that carries both the uplink inbounds and the controller's paths.
type sdwanHarness struct {
	t           *testing.T
	control     *LocalControlService
	service     *SdwanService
	instance    *core.Core
	realityPort int
	restarts    int
}

func newSdwanHarness(t *testing.T, capabilities []string) *sdwanHarness {
	t.Helper()
	control := setupQuickAddTest(t)
	if _, err := (&SettingService{}).SetControllerMode(true); err != nil {
		t.Fatal(err)
	}
	stubRealityProbe(t, "reality.test")
	setSdwanJobForTest(t, nil)
	harness := &sdwanHarness{t: t, control: control, realityPort: localRealityTarget(t)}
	node := AgentNodeView{Id: 7, Name: "tokyo", Online: true, Managed: true, PublicHost: "127.0.0.1"}
	node.Report.Panel.Capabilities = capabilities
	harness.service = &SdwanService{
		restartCore: harness.restart,
		rpc:         harness.rpc,
		listNodes:   func() ([]AgentNodeView, error) { return []AgentNodeView{node}, nil },
		getNode: func(id uint) (*AgentNodeView, error) {
			if id != node.Id {
				return nil, errors.New("not found")
			}
			view := node
			return &view, nil
		},
	}
	t.Cleanup(func() {
		if harness.instance != nil {
			_ = harness.instance.Stop()
		}
	})
	return harness
}

func (h *sdwanHarness) rpc(nodeID uint, method string, payload interface{}, actor string) (*agent.RPCResponse, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var result interface{}
	switch method {
	case agent.RPCMethodSdwanProvision, agent.RPCMethodSdwanRemove:
		// The managed server's saves only touch the database here; the
		// harness rebuilds the shared instance when the controller applies.
		running := corePtr
		corePtr = core.NewCore()
		defer func() { corePtr = running }()
		if method == agent.RPCMethodSdwanProvision {
			var request SdwanProvisionRequest
			if err = json.Unmarshal(raw, &request); err == nil {
				result, err = h.control.ProvisionSdwanUplink(request)
			}
		} else {
			var request SdwanRemoveRequest
			if err = json.Unmarshal(raw, &request); err == nil {
				result, err = h.control.RemoveSdwanUplink(request)
			}
		}
	case agent.RPCMethodSdwanDiagnose:
		result, err = h.control.DiagnoseSdwanUplink()
	case agent.RPCMethodSdwanTune:
		result = ApplySdwanTuning()
	default:
		err = fmt.Errorf("unexpected RPC %s", method)
	}
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(result)
	return &agent.RPCResponse{OK: true, Payload: encoded}, err
}

func (h *sdwanHarness) restart() error {
	h.restarts++
	if h.instance != nil {
		_ = h.instance.Stop()
		h.instance = nil
	}
	pointRealityAt(h.t, h.realityPort)
	config, err := h.control.ConfigService.GetConfigWithDB("", database.GetDB())
	if err != nil {
		return err
	}
	instance := core.NewCore()
	if err = instance.Start(*config); err != nil {
		return err
	}
	h.instance = instance
	corePtr = instance
	return nil
}

func waitSdwanJob(t *testing.T) *SdwanJob {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if job := currentSdwanJob(); job != nil && job.Status != SdwanJobStatusRunning {
			return job
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("SD-WAN job did not finish: %#v", currentSdwanJob())
	return nil
}

func sdwanTestProbe(t *testing.T) *httptest.Server {
	t.Helper()
	payload := strings.Repeat("x", 2<<20)
	probe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/speed" {
			_, _ = w.Write([]byte(payload))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(probe.Close)
	return probe
}

// TestSdwanOptimizeUpgradesTunesAndRemeasures runs one-click tuning against a
// first-generation member (a single Shadowsocks uplink) on a stock kernel.
func TestSdwanOptimizeUpgradesTunesAndRemeasures(t *testing.T) {
	capabilities := []string{agent.CapabilitySdwanV2, agent.CapabilitySdwanReality, agent.CapabilitySdwanHysteria2}
	harness := newSdwanHarness(t, capabilities)
	fake := useFakeSysctl(t, untunedSysctl())
	probe := sdwanTestProbe(t)
	db := database.GetDB()
	settings := defaultSdwanSettings()
	settings.TestURL = probe.URL + "/generate_204"
	settings.SpeedTestURL = probe.URL + "/speed"
	if err := storeSdwanSettings(db, settings); err != nil {
		t.Fatal(err)
	}
	legacy, err := harness.control.ProvisionSdwanUplink(SdwanProvisionRequest{
		Protocols: []string{SdwanProtocolShadowsocks}, Actor: "controller", PublicHost: "127.0.0.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = saveSdwanMemberUplinks(7, *legacy); err != nil {
		t.Fatal(err)
	}
	if err = harness.restart(); err != nil {
		t.Fatal(err)
	}

	if _, err = harness.service.StartSdwanJob(SdwanJobOptimize, true, "admin"); err != nil {
		t.Fatal(err)
	}
	job := waitSdwanJob(t)
	if job.Status != SdwanJobStatusDone || job.Before == nil || job.Report == nil || job.Progress != 100 {
		t.Fatalf("job = %s %q, logs: %#v", job.Status, job.Error, job.Logs)
	}
	before, after := job.Before, job.Report
	for _, code := range []string{"controller_tuning", "node_tuning"} {
		nodeID := uint(0)
		if code == "node_tuning" {
			nodeID = 7
		}
		if findSdwanAdvice(before, code, nodeID) == nil || findSdwanAdvice(after, code, nodeID) != nil {
			t.Fatalf("%s must be found before and fixed after tuning", code)
		}
	}
	if fake.values["net.ipv4.tcp_congestion_control"] != "bbr" || fake.values["net.core.default_qdisc"] != "fq" {
		t.Fatalf("kernel was not tuned: %#v", fake.values)
	}

	desired := sdwanProtocolsFor(SdwanModeAuto, capabilities)
	upgrade := !slices.Equal(desired, []string{SdwanProtocolShadowsocks})
	if (findSdwanAdvice(before, "protocol_upgrade", 7) != nil) != upgrade {
		t.Fatalf("protocol_upgrade advice = %v, want %v", findSdwanAdvice(before, "protocol_upgrade", 7), upgrade)
	}
	var changes []string
	for _, change := range job.Changes {
		changes = append(changes, change.Code)
	}
	if !slices.Contains(changes, "tuning_controller") || slices.Contains(changes, "protocol_upgraded") != upgrade {
		t.Fatalf("changes = %v", changes)
	}
	if upgrade && harness.restarts < 2 {
		t.Fatal("upgraded paths were not applied")
	}

	node := after.Nodes[0]
	var protocols []string
	for _, path := range node.Paths {
		protocols = append(protocols, path.Protocol)
		if path.Disabled || !path.Reachable || path.Loss != 0 {
			t.Fatalf("path %s after tuning = %#v", path.Tag, path)
		}
		if path.Mbps <= 0 {
			t.Fatalf("bandwidth through %s was not measured: %#v", path.Tag, path)
		}
	}
	if !slices.Equal(protocols, desired) || findSdwanAdvice(after, "protocol_upgrade", 7) != nil {
		t.Fatalf("paths after tuning = %v, want %v", protocols, desired)
	}
	if remaining := countFixableSdwanAdvice(after); remaining != 0 {
		t.Fatalf("one-click tuning left %d fixable findings: %#v", remaining, after.Advice)
	}
	if after.Score <= before.Score || after.BestTag == "" || !strings.HasPrefix(after.Selected, "sdwan-node-7-") {
		t.Fatalf("score %d -> %d, best %q, selected %q, advice %#v", before.Score, after.Score, after.BestTag, after.Selected, after.Advice)
	}
	if !slices.ContainsFunc(job.Logs, func(entry SdwanJobLog) bool { return entry.Code == "optimize_done" }) {
		t.Fatal("job log has no summary")
	}

	// Detection alone changes nothing.
	fake.values["net.ipv4.tcp_congestion_control"] = "cubic"
	if _, err = harness.service.StartSdwanJob(SdwanJobDiagnose, false, "admin"); err != nil {
		t.Fatal(err)
	}
	job = waitSdwanJob(t)
	if job.Status != SdwanJobStatusDone || len(job.Changes) != 0 || fake.values["net.ipv4.tcp_congestion_control"] != "cubic" {
		t.Fatalf("detection must be read-only: %#v", job.Changes)
	}
	if findSdwanAdvice(job.Report, "controller_tuning", 0) == nil {
		t.Fatal("detection did not notice the kernel lost its tuning")
	}
	state, err := harness.service.SdwanState()
	if err != nil || state.Job == nil || state.Job.ID != job.ID {
		t.Fatalf("state does not expose the latest job: %#v (%v)", state, err)
	}
}
