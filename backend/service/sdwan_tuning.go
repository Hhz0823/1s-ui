package service

import (
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/shirou/gopsutil/v4/mem"
)

// The SD-WAN network profile: BBR with the fq scheduler, TCP Fast Open, MTU
// probing, no slow-start restart after idle and large socket buffers so long
// fat links (and QUIC, which wants >= 7 MiB UDP buffers) reach full speed.
const sdwanSysctlFile = "/etc/sysctl.d/99-1s-ui-sdwan.conf"

type sysctlSetting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type SdwanTuningStatus struct {
	Supported         bool              `json:"supported"`
	Reason            string            `json:"reason,omitempty"`
	CongestionControl string            `json:"congestion_control"`
	Qdisc             string            `json:"qdisc"`
	BBRAvailable      bool              `json:"bbr_available"`
	Values            map[string]string `json:"values"`
	Pending           []string          `json:"pending"`
	// Blocked lists pending settings this kernel rejected when tuning last
	// ran, with the reason; tuning again cannot fix them.
	Blocked   map[string]string `json:"blocked,omitempty"`
	Optimized bool              `json:"optimized"`
}

type SdwanTuningFailure struct {
	Key   string `json:"key"`
	Error string `json:"error"`
}

type SdwanTuningResult struct {
	Before    SdwanTuningStatus    `json:"before"`
	After     SdwanTuningStatus    `json:"after"`
	Applied   []string             `json:"applied"`
	Failed    []SdwanTuningFailure `json:"failed"`
	Persisted bool                 `json:"persisted"`
}

// sysctlBackend isolates kernel access so the tuning logic can be tested.
type sysctlBackend interface {
	Available() (bool, string)
	Read(key string) (string, error)
	Write(key, value string) error
	LoadModule(name string)
	Persist(settings []sysctlSetting) error
	MemTotal() uint64
}

type procSysctl struct{}

func (procSysctl) Available() (bool, string) {
	if runtime.GOOS != "linux" {
		return false, "not_linux"
	}
	if os.Geteuid() != 0 {
		return false, "not_root"
	}
	if _, err := os.Stat("/proc/sys/net/ipv4"); err != nil {
		return false, "no_procfs"
	}
	return true, ""
}

func sysctlPath(key string) string {
	return filepath.Join("/proc/sys", strings.ReplaceAll(key, ".", "/"))
}

func (procSysctl) Read(key string) (string, error) {
	data, err := os.ReadFile(sysctlPath(key))
	if err != nil {
		return "", err
	}
	return strings.Join(strings.Fields(string(data)), " "), nil
}

func (procSysctl) Write(key, value string) error {
	return os.WriteFile(sysctlPath(key), []byte(value), 0644)
}

func (procSysctl) LoadModule(name string) {
	_ = exec.Command("modprobe", name).Run()
}

func (procSysctl) Persist(settings []sysctlSetting) error {
	var content strings.Builder
	content.WriteString("# Managed by 1S-UI SD-WAN one-click tuning; applied at boot by systemd-sysctl.\n")
	for _, setting := range settings {
		content.WriteString(setting.Key + " = " + setting.Value + "\n")
	}
	if err := os.MkdirAll(filepath.Dir(sdwanSysctlFile), 0755); err != nil {
		return err
	}
	return os.WriteFile(sdwanSysctlFile, []byte(content.String()), 0644)
}

func (procSysctl) MemTotal() uint64 {
	if memory, err := mem.VirtualMemory(); err == nil {
		return memory.Total
	}
	return 0
}

var (
	sdwanSysctl   sysctlBackend = procSysctl{}
	sdwanTuningMu sync.Mutex
	// What this kernel rejected, remembered so it is reported as unavailable
	// instead of as fixable on every run.
	sdwanTuningMemoryMu sync.Mutex
	sdwanTuningBlocked  = map[string]string{}
	sdwanFqUnavailable  bool
)

func sdwanTuningMemory() (map[string]string, bool) {
	sdwanTuningMemoryMu.Lock()
	defer sdwanTuningMemoryMu.Unlock()
	return maps.Clone(sdwanTuningBlocked), sdwanFqUnavailable
}

func rememberSdwanTuning(blocked map[string]string, fqUnavailable bool) {
	sdwanTuningMemoryMu.Lock()
	defer sdwanTuningMemoryMu.Unlock()
	sdwanTuningBlocked = maps.Clone(blocked)
	sdwanFqUnavailable = fqUnavailable
}

const (
	sdwanQdiscKey      = "net.core.default_qdisc"
	sdwanCongestionKey = "net.ipv4.tcp_congestion_control"
	sdwanQdiscFallback = "fq_codel"
	// Reasons the page translates (sdwan.reasons.*).
	sdwanNoBBRError     = "no_bbr"
	sdwanNoFairQueueing = "no_fq"
)

var sdwanBBRVariants = []string{"bbr", "bbr2", "bbr3", "bbrplus", "bbr2plus"}

func sdwanBufferSize(memTotal uint64) string {
	if memTotal > 0 && memTotal < 1<<30 {
		return "8388608"
	}
	return "16777216"
}

func sdwanTuningProfile(memTotal uint64, bbr bool) []sysctlSetting {
	buffer := sdwanBufferSize(memTotal)
	settings := []sysctlSetting{{"net.core.default_qdisc", "fq"}}
	if bbr {
		settings = append(settings, sysctlSetting{"net.ipv4.tcp_congestion_control", "bbr"})
	}
	return append(settings,
		sysctlSetting{"net.ipv4.tcp_fastopen", "3"},
		sysctlSetting{"net.ipv4.tcp_mtu_probing", "1"},
		sysctlSetting{"net.ipv4.tcp_slow_start_after_idle", "0"},
		sysctlSetting{"net.ipv4.tcp_notsent_lowat", "16384"},
		sysctlSetting{"net.core.rmem_max", buffer},
		sysctlSetting{"net.core.wmem_max", buffer},
		sysctlSetting{"net.ipv4.tcp_rmem", "4096 131072 " + buffer},
		sysctlSetting{"net.ipv4.tcp_wmem", "4096 65536 " + buffer},
	)
}

func lastField(value string) int64 {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return -1
	}
	number, err := strconv.ParseInt(fields[len(fields)-1], 10, 64)
	if err != nil {
		return -1
	}
	return number
}

// sysctlSatisfied accepts values that are at least as good as the profile, so
// tuning never lowers buffers an administrator raised or replaces bbr3 etc.
func sysctlSatisfied(key, current, target string) bool {
	current = strings.TrimSpace(current)
	switch key {
	case sdwanCongestionKey:
		return slices.Contains(sdwanBBRVariants, current)
	case sdwanQdiscKey:
		return current == "fq" || current == "cake" || current == "fq_pie"
	case "net.ipv4.tcp_fastopen":
		value, err := strconv.ParseInt(current, 10, 64)
		return err == nil && value&3 == 3
	case "net.ipv4.tcp_mtu_probing":
		value, err := strconv.ParseInt(current, 10, 64)
		return err == nil && value >= 1
	case "net.ipv4.tcp_notsent_lowat":
		value, err := strconv.ParseInt(current, 10, 64)
		return err == nil && value > 0 && value <= 131072
	case "net.core.rmem_max", "net.core.wmem_max", "net.ipv4.tcp_rmem", "net.ipv4.tcp_wmem":
		return lastField(current) >= lastField(target)
	default:
		return current == target
	}
}

// sdwanSettingSatisfied also accepts fq_codel on kernels built without fq:
// BBR paces by itself there and fq_codel still gives fair queueing.
func sdwanSettingSatisfied(key, current, target string, fqUnavailable bool) bool {
	if key == sdwanQdiscKey && fqUnavailable && strings.TrimSpace(current) == sdwanQdiscFallback {
		return true
	}
	return sysctlSatisfied(key, current, target)
}

func sdwanBBRAvailable(backend sysctlBackend) bool {
	available, err := backend.Read("net.ipv4.tcp_available_congestion_control")
	if err != nil {
		return false
	}
	for _, name := range strings.Fields(available) {
		if slices.Contains(sdwanBBRVariants, name) {
			return true
		}
	}
	return false
}

func readSdwanTuning(backend sysctlBackend) SdwanTuningStatus {
	status := SdwanTuningStatus{Values: map[string]string{}, Pending: []string{}}
	if ok, reason := backend.Available(); !ok {
		status.Reason = reason
		return status
	}
	status.Supported = true
	status.BBRAvailable = sdwanBBRAvailable(backend)
	blocked, fqUnavailable := sdwanTuningMemory()
	for _, setting := range sdwanTuningProfile(backend.MemTotal(), true) {
		current, err := backend.Read(setting.Key)
		if err != nil {
			if setting.Key == "net.ipv4.tcp_notsent_lowat" && errors.Is(err, os.ErrNotExist) {
				continue // older kernels without the knob
			}
			current = ""
		}
		status.Values[setting.Key] = current
		if !sdwanSettingSatisfied(setting.Key, current, setting.Value, fqUnavailable) {
			status.Pending = append(status.Pending, setting.Key)
			if reason, ok := blocked[setting.Key]; ok {
				if status.Blocked == nil {
					status.Blocked = map[string]string{}
				}
				status.Blocked[setting.Key] = reason
			}
		}
	}
	status.CongestionControl = status.Values["net.ipv4.tcp_congestion_control"]
	status.Qdisc = status.Values["net.core.default_qdisc"]
	status.Optimized = len(status.Pending) == 0
	return status
}

// ReadSdwanTuning reports how close this host is to the SD-WAN profile.
func ReadSdwanTuning() SdwanTuningStatus {
	return readSdwanTuning(sdwanSysctl)
}

// ApplySdwanTuning applies the profile key by key, keeping values that are
// already better, and persists what was applied for the next boot.
func ApplySdwanTuning() SdwanTuningResult {
	sdwanTuningMu.Lock()
	defer sdwanTuningMu.Unlock()
	return applySdwanTuning(sdwanSysctl)
}

func applySdwanTuning(backend sysctlBackend) SdwanTuningResult {
	result := SdwanTuningResult{Applied: []string{}, Failed: []SdwanTuningFailure{}}
	result.Before = readSdwanTuning(backend)
	if !result.Before.Supported {
		result.After = result.Before
		return result
	}
	bbr := sdwanBBRAvailable(backend)
	if !bbr {
		backend.LoadModule("tcp_bbr")
		bbr = sdwanBBRAvailable(backend)
	}
	backend.LoadModule("sch_fq")
	blocked, fqUnavailable := sdwanTuningMemory()
	var persisted []sysctlSetting
	for _, setting := range sdwanTuningProfile(backend.MemTotal(), bbr) {
		current, readErr := backend.Read(setting.Key)
		if readErr != nil && errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if sdwanSettingSatisfied(setting.Key, current, setting.Value, fqUnavailable) {
			delete(blocked, setting.Key)
			if current != "" {
				persisted = append(persisted, sysctlSetting{setting.Key, current})
			}
			continue
		}
		err := backend.Write(setting.Key, setting.Value)
		if err != nil && setting.Key == sdwanQdiscKey {
			// Kernels built without fq reject it; fall back to fq_codel.
			if fallbackErr := backend.Write(setting.Key, sdwanQdiscFallback); fallbackErr == nil {
				fqUnavailable = true
				delete(blocked, setting.Key)
				if strings.TrimSpace(current) != sdwanQdiscFallback {
					result.Applied = append(result.Applied, setting.Key)
				}
				persisted = append(persisted, sysctlSetting{setting.Key, sdwanQdiscFallback})
				continue
			}
			err = errors.New(sdwanNoFairQueueing)
		}
		if err != nil {
			blocked[setting.Key] = err.Error()
			result.Failed = append(result.Failed, SdwanTuningFailure{Key: setting.Key, Error: err.Error()})
			continue
		}
		delete(blocked, setting.Key)
		result.Applied = append(result.Applied, setting.Key)
		persisted = append(persisted, setting)
	}
	if !bbr {
		blocked[sdwanCongestionKey] = sdwanNoBBRError
		result.Failed = append(result.Failed, SdwanTuningFailure{Key: sdwanCongestionKey, Error: sdwanNoBBRError})
	}
	rememberSdwanTuning(blocked, fqUnavailable)
	if len(persisted) > 0 {
		result.Persisted = backend.Persist(persisted) == nil
	}
	result.After = readSdwanTuning(backend)
	return result
}
