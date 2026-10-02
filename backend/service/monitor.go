package service

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/config"
	"github.com/Hhz0823/1s-ui/speedtest"
	"github.com/Hhz0823/1s-ui/util/common"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	psnet "github.com/shirou/gopsutil/v4/net"
)

// monitorKeyHash stores "sha256hex|createdUnix" of the read-only key that the
// mobile monitor app uses. The key itself is shown once and never stored.
const monitorKeyHash = "monitorKeyHash"

// What the monitor key may do besides reading: manage proxy monitors and
// open speed test sessions. Both default to on and are set on the key's card.
const (
	monitorAppProxiesKey   = "monitorAppProxies"
	monitorAppSpeedtestKey = "monitorAppSpeedtest"
	speedtestPortKey       = "speedtestPort"
	// The app may switch nodes and modes of the proxy client on home devices.
	monitorAppClientKey = "monitorAppClient"
)

const (
	localMonitorCacheTTL      = 2 * time.Second
	localMonitorHistoryStep   = 10 * time.Second
	localMonitorHistoryLength = 360
)

// MonitorNode is a read-only server entry for the mobile monitor API. It
// embeds the panel's node view so new monitoring fields reach the app without
// changes here.
type MonitorNode struct {
	AgentNodeView
	Local bool `json:"local"`
}

type MonitorOverview struct {
	PanelVersion string          `json:"panel_version"`
	ServerTime   int64           `json:"server_time"`
	Features     MonitorFeatures `json:"features"`
	Servers      []MonitorNode   `json:"servers"`
}

// MonitorFeatures tells the app what this panel offers to its key.
type MonitorFeatures struct {
	Nodes         bool `json:"nodes"`
	ProxyMonitors bool `json:"proxy_monitors"`
	ManageProxies bool `json:"manage_proxies"`
	Speedtest     bool `json:"speedtest"`
	SpeedtestPort int  `json:"speedtest_port"`
	// Node monitors (share links and server inbounds) and speed tests run
	// from a relay server.
	NodeMonitors   bool `json:"node_monitors"`
	RelaySpeedtest bool `json:"relay_speedtest"`
	// The proxy client on this panel and its servers can be read, and
	// with ManageClients switched (node, mode, tests, subscription update).
	Clients       bool `json:"clients"`
	ManageClients bool `json:"manage_clients"`
}

// MonitorAppSettings are the key's extra permissions and the speed test port.
type MonitorAppSettings struct {
	Proxies       bool `json:"proxies"`
	Speedtest     bool `json:"speedtest"`
	SpeedtestPort int  `json:"speedtest_port"`
	Client        bool `json:"client"`
}

type MonitorKeyStatus struct {
	Enabled   bool  `json:"enabled"`
	CreatedAt int64 `json:"created_at"`
	MonitorAppSettings
}

type MonitorService struct {
	AgentService
	SettingService
}

var localMonitor = struct {
	sync.Mutex
	at       time.Time
	report   agent.Report
	cpuTimes cpu.TimesStat
	cpuReady bool
	netAt    time.Time
	netSent  uint64
	netRecv  uint64
	history  []AgentMetricSample
}{}

func (s *MonitorService) Overview() (*MonitorOverview, error) {
	nodes, err := s.AgentService.List()
	if err != nil {
		return nil, err
	}
	settings := s.SettingService.GetMonitorAppSettings()
	result := &MonitorOverview{
		PanelVersion: config.GetVersion(),
		ServerTime:   time.Now().Unix(),
		Features: MonitorFeatures{
			Nodes: true, ProxyMonitors: true, ManageProxies: settings.Proxies,
			Speedtest: settings.Speedtest, SpeedtestPort: settings.SpeedtestPort,
			NodeMonitors: true, RelaySpeedtest: settings.Speedtest,
			Clients: true, ManageClients: settings.Client,
		},
		Servers: make([]MonitorNode, 0, len(nodes)+1),
	}
	result.Servers = append(result.Servers, localMonitorNode(false))
	for _, node := range nodes {
		node.Commands = nil
		result.Servers = append(result.Servers, MonitorNode{AgentNodeView: node})
	}
	return result, nil
}

// Server returns one server with its history. ID 0 is the panel host itself.
func (s *MonitorService) Server(id uint) (*MonitorNode, error) {
	if id == 0 {
		node := localMonitorNode(true)
		return &node, nil
	}
	view, err := s.AgentService.Get(id)
	if err != nil {
		return nil, err
	}
	view.Commands = nil
	return &MonitorNode{AgentNodeView: *view}, nil
}

// Nodes lists a server's inbounds with their traffic: protocol, port,
// security, transport and user count, never credentials or keys.
func (s *MonitorService) Nodes(id uint) (*PortTrafficResponse, error) {
	if id == 0 {
		return (&PortTrafficService{}).GetPortTraffic()
	}
	response, err := s.AgentService.DispatchRPC(id, agent.RPCMethodPortTraffic, map[string]interface{}{}, "monitor-app")
	if err != nil {
		return nil, err
	}
	var result PortTrafficResponse
	if err := json.Unmarshal(response.Payload, &result); err != nil {
		return nil, err
	}
	if result.Items == nil {
		result.Items = []PortTrafficItem{}
	}
	return &result, nil
}

func localMonitorNode(withHistory bool) MonitorNode {
	report, history := collectLocalMonitorReport()
	name := strings.TrimSpace(report.Hostname)
	if name == "" {
		name = "panel"
	}
	now := time.Now().Unix()
	view := AgentNodeView{
		Name:     name,
		LastSeen: now,
		Version:  config.GetVersion(),
		Online:   true,
		Report:   report,
	}
	if withHistory {
		view.History = history
	}
	return MonitorNode{AgentNodeView: view, Local: true}
}

// collectLocalMonitorReport samples the panel host with cheap calls only (no
// process scan or core exec) and caches the result briefly, so several phones
// polling every few seconds stay light on low-spec controllers.
func collectLocalMonitorReport() (agent.Report, []AgentMetricSample) {
	localMonitor.Lock()
	defer localMonitor.Unlock()
	now := time.Now()
	if !localMonitor.at.IsZero() && now.Sub(localMonitor.at) < localMonitorCacheTTL {
		return localMonitor.report, append([]AgentMetricSample(nil), localMonitor.history...)
	}

	report := agent.Report{OS: runtime.GOOS, Arch: runtime.GOARCH, AgentVersion: config.GetVersion()}
	report.Hostname, _ = os.Hostname()
	report.Uptime, _ = host.Uptime()
	if count, err := cpu.Counts(true); err == nil {
		report.CPUCores = count
	}
	if times, err := cpu.Times(false); err == nil && len(times) > 0 {
		if localMonitor.cpuReady {
			report.CPUPercent = monitorCPUPercent(localMonitor.cpuTimes, times[0])
		} else if percent, err := cpu.Percent(200*time.Millisecond, false); err == nil && len(percent) > 0 {
			report.CPUPercent = percent[0]
			if end, err := cpu.Times(false); err == nil && len(end) > 0 {
				times = end
			}
		}
		localMonitor.cpuTimes, localMonitor.cpuReady = times[0], true
	}
	if value, err := mem.VirtualMemory(); err == nil {
		report.Memory = agent.ResourceUsage{Used: value.Used, Total: value.Total}
	}
	if value, err := mem.SwapMemory(); err == nil {
		report.Swap = agent.ResourceUsage{Used: value.Used, Total: value.Total}
	}
	if value, err := disk.Usage("/"); err == nil {
		report.Disk = agent.ResourceUsage{Used: value.Used, Total: value.Total}
	}
	if values, err := psnet.IOCounters(false); err == nil && len(values) > 0 {
		sent, recv := values[0].BytesSent, values[0].BytesRecv
		report.Network = agent.NetworkUsage{Sent: sent, Recv: recv}
		if !localMonitor.netAt.IsZero() && sent >= localMonitor.netSent && recv >= localMonitor.netRecv {
			if elapsed := now.Sub(localMonitor.netAt).Seconds(); elapsed > 0.2 {
				report.NetRate = agent.NetworkUsage{
					Sent: uint64(float64(sent-localMonitor.netSent) / elapsed),
					Recv: uint64(float64(recv-localMonitor.netRecv) / elapsed),
				}
			}
		}
		localMonitor.netAt, localMonitor.netSent, localMonitor.netRecv = now, sent, recv
	}
	if value, err := load.Avg(); err == nil {
		report.Load = agent.LoadAverage{Load1: value.Load1, Load5: value.Load5, Load15: value.Load15}
	}
	if corePtr != nil {
		report.Cores.SingBoxRunning = corePtr.IsRunning()
	}
	if xrayPtr != nil {
		report.Cores.XrayRunning = xrayPtr.IsRunning()
	}

	sample := AgentMetricSample{
		Time:        now.Unix(),
		CPUPercent:  report.CPUPercent,
		MemPercent:  monitorPercent(report.Memory),
		SwapPercent: monitorPercent(report.Swap),
		DiskPercent: monitorPercent(report.Disk),
		NetSent:     report.NetRate.Sent,
		NetRecv:     report.NetRate.Recv,
	}
	history := localMonitor.history
	if len(history) == 0 || now.Unix()-history[len(history)-1].Time >= int64(localMonitorHistoryStep/time.Second) {
		history = append(history, sample)
		if len(history) > localMonitorHistoryLength {
			history = history[len(history)-localMonitorHistoryLength:]
		}
		localMonitor.history = history
	}
	localMonitor.at = now
	localMonitor.report = report
	return report, append([]AgentMetricSample(nil), history...)
}

func monitorCPUPercent(previous, current cpu.TimesStat) float64 {
	busy := func(v cpu.TimesStat) (float64, float64) {
		total := v.User + v.System + v.Idle + v.Nice + v.Iowait + v.Irq + v.Softirq + v.Steal
		return total, total - v.Idle - v.Iowait
	}
	prevTotal, prevBusy := busy(previous)
	curTotal, curBusy := busy(current)
	totalDelta, busyDelta := curTotal-prevTotal, curBusy-prevBusy
	if totalDelta <= 0 || busyDelta <= 0 {
		return 0
	}
	if value := busyDelta / totalDelta * 100; value < 100 {
		return value
	}
	return 100
}

func monitorPercent(value agent.ResourceUsage) float64 {
	if value.Total == 0 {
		return 0
	}
	return float64(value.Used) / float64(value.Total) * 100
}

// RotateMonitorKey issues a new read-only monitor key, revoking the old one.
func (s *SettingService) RotateMonitorKey() (string, int64, error) {
	token, hash, err := newAgentToken()
	if err != nil {
		return "", 0, err
	}
	created := time.Now().Unix()
	if err := s.setString(monitorKeyHash, hash+"|"+strconv.FormatInt(created, 10)); err != nil {
		return "", 0, err
	}
	return token, created, nil
}

func (s *SettingService) DisableMonitorKey() error {
	return s.setString(monitorKeyHash, "")
}

func (s *SettingService) GetMonitorKeyStatus() MonitorKeyStatus {
	hash, created := s.monitorKey()
	return MonitorKeyStatus{Enabled: hash != "", CreatedAt: created, MonitorAppSettings: s.GetMonitorAppSettings()}
}

func (s *SettingService) GetMonitorAppSettings() MonitorAppSettings {
	settings := MonitorAppSettings{Proxies: true, Speedtest: true, SpeedtestPort: speedtest.DefaultPort, Client: true}
	if value, err := s.getString(monitorAppProxiesKey); err == nil {
		settings.Proxies = value != "false"
	}
	if value, err := s.getString(monitorAppClientKey); err == nil {
		settings.Client = value != "false"
	}
	if value, err := s.getString(monitorAppSpeedtestKey); err == nil {
		settings.Speedtest = value != "false"
	}
	if port, err := s.getInt(speedtestPortKey); err == nil && port > 0 && port <= 65535 {
		settings.SpeedtestPort = port
	}
	return settings
}

func (s *SettingService) SetMonitorAppSettings(value MonitorAppSettings) error {
	if value.SpeedtestPort < 1 || value.SpeedtestPort > 65535 {
		return common.NewError("speed test port must be between 1 and 65535")
	}
	if err := s.setString(monitorAppProxiesKey, strconv.FormatBool(value.Proxies)); err != nil {
		return err
	}
	if err := s.setString(monitorAppSpeedtestKey, strconv.FormatBool(value.Speedtest)); err != nil {
		return err
	}
	if err := s.setString(monitorAppClientKey, strconv.FormatBool(value.Client)); err != nil {
		return err
	}
	return s.setInt(speedtestPortKey, value.SpeedtestPort)
}

func (s *SettingService) ValidateMonitorKey(key string) error {
	key = strings.TrimSpace(key)
	expected, _ := s.monitorKey()
	if expected == "" || !validAgentCredential(key) {
		return common.NewError("invalid monitor key")
	}
	if subtle.ConstantTimeCompare([]byte(hashAgentToken(key)), []byte(expected)) != 1 {
		return common.NewError("invalid monitor key")
	}
	return nil
}

func (s *SettingService) monitorKey() (string, int64) {
	value, err := s.getString(monitorKeyHash)
	if err != nil {
		return "", 0
	}
	hash, createdText, _ := strings.Cut(value, "|")
	if len(hash) != sha256.Size*2 {
		return "", 0
	}
	created, _ := strconv.ParseInt(createdText, 10, 64)
	return hash, created
}
