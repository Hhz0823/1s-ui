package service

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/speedtest"
	"github.com/Hhz0823/1s-ui/util/common"

	"github.com/gofrs/uuid/v5"
)

// A relay speed test measures the line from one server (a home NAS or
// router running the panel, or any managed server) to another: the target
// opens a speed test session, and the relay runs the tests against it one by
// one, so the page can show each result as it arrives.

// SpeedtestRunRequest asks a server to run one test against a session.
type SpeedtestRunRequest struct {
	Token   string   `json:"token"`
	Port    int      `json:"port"`
	Hosts   []string `json:"hosts"`
	Test    string   `json:"test"`
	Seconds int      `json:"seconds"`
	Streams int      `json:"streams"`
	UDPMbps int      `json:"udp_mbps"`
	// Controller marks a test to the controller itself: the relay tries the
	// address its agent connects to first.
	Controller bool `json:"controller,omitempty"`
}

// SpeedtestRunResult is one test's result; only the field of its kind is set.
type SpeedtestRunResult struct {
	Test       string                     `json:"test"`
	Host       string                     `json:"host"`
	Ping       *speedtest.PingStats       `json:"ping,omitempty"`
	Throughput *speedtest.ThroughputStats `json:"throughput,omitempty"`
	UDP        *speedtest.UDPStats        `json:"udp,omitempty"`
	Error      string                     `json:"error,omitempty"`
}

// speedtestClientSlot keeps one outgoing test at a time on this server, so
// two tests do not share (and halve) the same line.
var speedtestClientSlot = make(chan struct{}, 1)

// RunLocalSpeedtest runs one test from this server.
func RunLocalSpeedtest(request SpeedtestRunRequest) (*SpeedtestRunResult, error) {
	if !slices.Contains(speedtest.Tests, request.Test) {
		return nil, common.NewError("unknown speed test ", request.Test)
	}
	select {
	case speedtestClientSlot <- struct{}{}:
		defer func() { <-speedtestClientSlot }()
	default:
		return nil, common.NewError("another speed test is running on this server; try again shortly")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	hosts := request.Hosts
	if request.Controller {
		if host := localControllerHost(); host != "" {
			hosts = append([]string{host}, hosts...)
		}
	}
	client, err := speedtest.Connect(ctx, request.Token, request.Port, hosts)
	if err != nil {
		return nil, err
	}
	duration := time.Duration(request.Seconds) * time.Second
	result := &SpeedtestRunResult{Test: request.Test, Host: client.Host}
	switch request.Test {
	case speedtest.TestTCPPing:
		var stats speedtest.PingStats
		stats, err = client.TCPPing(ctx, 10)
		result.Ping = &stats
	case speedtest.TestUDPPing:
		var stats speedtest.PingStats
		stats, err = client.UDPPing(ctx, 20)
		result.Ping = &stats
	case speedtest.TestTCPDownload:
		var stats speedtest.ThroughputStats
		stats, err = client.TCPDownload(ctx, duration, request.Streams)
		result.Throughput = &stats
	case speedtest.TestTCPUpload:
		var stats speedtest.ThroughputStats
		stats, err = client.TCPUpload(ctx, duration, request.Streams)
		result.Throughput = &stats
	case speedtest.TestUDPDownload:
		var stats speedtest.UDPStats
		stats, err = client.UDPDownload(ctx, duration, request.UDPMbps)
		result.UDP = &stats
	case speedtest.TestUDPUpload:
		var stats speedtest.UDPStats
		stats, err = client.UDPUpload(ctx, duration, request.UDPMbps)
		result.UDP = &stats
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RelaySpeedtestOptions picks the relay (0 = this panel) and the tests.
type RelaySpeedtestOptions struct {
	RelayId uint     `json:"relay_id"`
	Tests   []string `json:"tests"`
	Seconds int      `json:"seconds"`
	Streams int      `json:"streams"`
	UDPMbps int      `json:"udp_mbps"`
}

const (
	RelaySpeedtestRunning = "running"
	RelaySpeedtestDone    = "done"
	RelaySpeedtestFailed  = "failed"

	relaySpeedtestKeep    = 15 * time.Minute
	relaySpeedtestMaxJobs = 20
)

// RelaySpeedtestJob is a relay speed test in progress or finished.
type RelaySpeedtestJob struct {
	Id         string               `json:"id"`
	ServerId   uint                 `json:"server_id"`
	ServerName string               `json:"server_name"`
	RelayId    uint                 `json:"relay_id"`
	RelayName  string               `json:"relay_name"`
	Status     string               `json:"status"`
	Tests      []string             `json:"tests"`
	Seconds    int                  `json:"seconds"`
	Streams    int                  `json:"streams"`
	UDPMbps    int                  `json:"udp_mbps"`
	Current    string               `json:"current,omitempty"`
	Host       string               `json:"host,omitempty"`
	Results    []SpeedtestRunResult `json:"results"`
	Error      string               `json:"error,omitempty"`
	StartedAt  int64                `json:"started_at"`
	FinishedAt int64                `json:"finished_at,omitempty"`
}

var relaySpeedtests = struct {
	sync.Mutex
	jobs map[string]*RelaySpeedtestJob
}{jobs: map[string]*RelaySpeedtestJob{}}

func normalizeRelaySpeedtest(options *RelaySpeedtestOptions) error {
	if len(options.Tests) == 0 {
		options.Tests = append([]string(nil), speedtest.Tests...)
	}
	var tests []string
	for _, test := range speedtest.Tests {
		if slices.Contains(options.Tests, test) {
			tests = append(tests, test)
		}
	}
	if len(tests) != len(options.Tests) {
		return common.NewError("unknown speed test; use tcp_ping, udp_ping, tcp_download, tcp_upload, udp_download or udp_upload")
	}
	options.Tests = tests
	if options.Seconds == 0 {
		options.Seconds = 10
	}
	if options.Streams == 0 {
		options.Streams = 4
	}
	if options.UDPMbps == 0 {
		options.UDPMbps = 50
	}
	if options.Seconds < 3 || options.Seconds > int(speedtest.MaxDuration/time.Second) {
		return common.NewErrorf("test duration must be 3-%d seconds", int(speedtest.MaxDuration/time.Second))
	}
	if options.Streams < 1 || options.Streams > 8 {
		return common.NewError("TCP streams must be 1-8")
	}
	if options.UDPMbps < 1 || options.UDPMbps > speedtest.MaxUDPRateKbps/1000 {
		return common.NewErrorf("UDP rate must be 1-%d Mbit/s", speedtest.MaxUDPRateKbps/1000)
	}
	return nil
}

// StartRelaySpeedtest starts testing the line from a relay to a server.
func (s *MonitorService) StartRelaySpeedtest(serverID uint, options RelaySpeedtestOptions) (*RelaySpeedtestJob, error) {
	if err := normalizeRelaySpeedtest(&options); err != nil {
		return nil, err
	}
	if options.RelayId == serverID {
		return nil, common.NewError("pick another server to test from")
	}
	names, err := managedServerNames()
	if err != nil {
		return nil, err
	}
	if serverID != 0 && names[serverID] == "" {
		return nil, common.NewError("server not found")
	}
	if options.RelayId != 0 {
		relay, err := s.AgentService.Get(options.RelayId)
		if err != nil {
			return nil, common.NewError("the server to test from does not exist")
		}
		if !relay.Online {
			return nil, common.NewError("the server to test from is offline")
		}
		if !hasCapability(relay.Report.Panel.Capabilities, agent.CapabilitySpeedtestClientV1) {
			return nil, common.NewError("the panel on the server to test from is too old for speed tests; update it")
		}
	}
	now := time.Now()
	relaySpeedtests.Lock()
	pruneRelaySpeedtestsLocked(now)
	for _, job := range relaySpeedtests.jobs {
		if job.Status == RelaySpeedtestRunning && (job.RelayId == options.RelayId || job.ServerId == serverID) {
			relaySpeedtests.Unlock()
			return nil, common.NewError("a speed test from or to one of these servers is already running")
		}
	}
	job := &RelaySpeedtestJob{
		Id: uuid.Must(uuid.NewV4()).String(), ServerId: serverID, ServerName: names[serverID],
		RelayId: options.RelayId, RelayName: names[options.RelayId], Status: RelaySpeedtestRunning,
		Tests: options.Tests, Seconds: options.Seconds, Streams: options.Streams, UDPMbps: options.UDPMbps,
		Results: []SpeedtestRunResult{}, StartedAt: now.Unix(),
	}
	relaySpeedtests.jobs[job.Id] = job
	snapshot := copyRelaySpeedtest(job)
	relaySpeedtests.Unlock()
	go s.runRelaySpeedtest(job.Id, serverID, options)
	return snapshot, nil
}

func pruneRelaySpeedtestsLocked(now time.Time) {
	var finished []*RelaySpeedtestJob
	for id, job := range relaySpeedtests.jobs {
		if job.Status != RelaySpeedtestRunning {
			if now.Unix()-job.FinishedAt > int64(relaySpeedtestKeep/time.Second) {
				delete(relaySpeedtests.jobs, id)
				continue
			}
			finished = append(finished, job)
		}
	}
	if extra := len(relaySpeedtests.jobs) - relaySpeedtestMaxJobs + 1; extra > 0 {
		sort.Slice(finished, func(i, j int) bool { return finished[i].FinishedAt < finished[j].FinishedAt })
		for i := 0; i < extra && i < len(finished); i++ {
			delete(relaySpeedtests.jobs, finished[i].Id)
		}
	}
}

func copyRelaySpeedtest(job *RelaySpeedtestJob) *RelaySpeedtestJob {
	copied := *job
	copied.Tests = append([]string(nil), job.Tests...)
	copied.Results = append([]SpeedtestRunResult{}, job.Results...)
	return &copied
}

func updateRelaySpeedtest(id string, apply func(job *RelaySpeedtestJob)) {
	relaySpeedtests.Lock()
	defer relaySpeedtests.Unlock()
	if job := relaySpeedtests.jobs[id]; job != nil {
		apply(job)
	}
}

// RelaySpeedtest returns a relay speed test by id.
func (s *MonitorService) RelaySpeedtest(id string) (*RelaySpeedtestJob, error) {
	relaySpeedtests.Lock()
	defer relaySpeedtests.Unlock()
	job := relaySpeedtests.jobs[id]
	if job == nil {
		return nil, common.NewError("speed test not found")
	}
	return copyRelaySpeedtest(job), nil
}

func (s *MonitorService) runRelaySpeedtest(id string, serverID uint, options RelaySpeedtestOptions) {
	fail := func(err error) {
		updateRelaySpeedtest(id, func(job *RelaySpeedtestJob) {
			job.Status, job.Error, job.Current, job.FinishedAt = RelaySpeedtestFailed, err.Error(), "", time.Now().Unix()
		})
	}
	target, err := s.StartSpeedtest(serverID)
	if err != nil {
		fail(err)
		return
	}
	hosts := target.Hosts
	for _, test := range options.Tests {
		updateRelaySpeedtest(id, func(job *RelaySpeedtestJob) { job.Current = test })
		request := SpeedtestRunRequest{
			Token: target.Token, Port: target.Port, Hosts: hosts, Test: test,
			Seconds: options.Seconds, Streams: options.Streams, UDPMbps: options.UDPMbps,
			Controller: serverID == 0,
		}
		result, err := s.runSpeedtestOn(options.RelayId, request)
		if err != nil {
			result = &SpeedtestRunResult{Test: test, Error: truncate(err.Error(), 300)}
		} else if result.Host != "" {
			// Later tests try the address that worked first.
			hosts = append([]string{result.Host}, hosts...)
		}
		updateRelaySpeedtest(id, func(job *RelaySpeedtestJob) {
			job.Results = append(job.Results, *result)
			if result.Host != "" {
				job.Host = result.Host
			}
		})
		if err != nil && strings.Contains(err.Error(), "cannot reach speed test port") {
			// Every other test would fail the same way.
			fail(err)
			return
		}
	}
	updateRelaySpeedtest(id, func(job *RelaySpeedtestJob) {
		job.Status, job.Current, job.FinishedAt = RelaySpeedtestDone, "", time.Now().Unix()
	})
}

func (s *MonitorService) runSpeedtestOn(relayID uint, request SpeedtestRunRequest) (*SpeedtestRunResult, error) {
	if relayID == 0 {
		return RunLocalSpeedtest(request)
	}
	response, err := s.AgentService.DispatchRPC(relayID, agent.RPCMethodSpeedtestRun, request, "speedtest")
	if err != nil {
		return nil, err
	}
	var result SpeedtestRunResult
	if err := json.Unmarshal(response.Payload, &result); err != nil || result.Test != request.Test {
		return nil, common.NewError("the server returned an invalid speed test result")
	}
	return &result, nil
}

// localControllerHost is the host of the controller this server's agent
// connects to, which is how this server reaches the controller.
func localControllerHost() string {
	content, err := os.ReadFile(envOrDefault("SUI_AGENT_ENV_FILE", defaultLocalAgentEnvFile))
	if err != nil {
		return ""
	}
	parsed, err := url.Parse(strings.TrimSpace(parseAgentEnvironment(content)["SUI_AGENT_PANEL"]))
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}
