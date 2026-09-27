package service

import (
	"errors"
	"maps"
	"os"
	"slices"
	"sync"
	"testing"
)

type fakeSysctl struct {
	mu        sync.Mutex
	available bool
	reason    string
	values    map[string]string
	failing   map[string]bool
	rejected  map[string]bool // "key=value" writes the kernel does not know
	bbrModule bool
	modules   []string
	persisted []sysctlSetting
	memTotal  uint64
}

func (f *fakeSysctl) Available() (bool, string) { return f.available, f.reason }

func (f *fakeSysctl) Read(key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value, ok := f.values[key]
	if !ok {
		return "", os.ErrNotExist
	}
	return value, nil
}

func (f *fakeSysctl) Write(key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failing[key] {
		return errors.New("permission denied")
	}
	if f.rejected[key+"="+value] {
		return os.ErrNotExist
	}
	f.values[key] = value
	return nil
}

func (f *fakeSysctl) LoadModule(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.modules = append(f.modules, name)
	if name == "tcp_bbr" && f.bbrModule {
		f.values["net.ipv4.tcp_available_congestion_control"] = "reno cubic bbr"
	}
}

func (f *fakeSysctl) Persist(settings []sysctlSetting) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.persisted = slices.Clone(settings)
	return nil
}

func (f *fakeSysctl) MemTotal() uint64 { return f.memTotal }

// untunedSysctl looks like a stock distribution kernel: CUBIC, no fq and
// small socket buffers, with BBR available as a module.
func untunedSysctl() *fakeSysctl {
	return &fakeSysctl{
		available: true, bbrModule: true, memTotal: 4 << 30, failing: map[string]bool{}, rejected: map[string]bool{},
		values: map[string]string{
			"net.ipv4.tcp_available_congestion_control": "reno cubic",
			"net.ipv4.tcp_congestion_control":           "cubic",
			"net.core.default_qdisc":                    "pfifo_fast",
			"net.ipv4.tcp_fastopen":                     "1",
			"net.ipv4.tcp_mtu_probing":                  "0",
			"net.ipv4.tcp_slow_start_after_idle":        "1",
			"net.ipv4.tcp_notsent_lowat":                "4294967295",
			"net.core.rmem_max":                         "212992",
			"net.core.wmem_max":                         "212992",
			"net.ipv4.tcp_rmem":                         "4096 131072 6291456",
			"net.ipv4.tcp_wmem":                         "4096 16384 4194304",
		},
	}
}

func useFakeSysctl(t *testing.T, fake *fakeSysctl) *fakeSysctl {
	t.Helper()
	previous := sdwanSysctl
	blocked, fqUnavailable := sdwanTuningMemory()
	sdwanSysctl = fake
	rememberSdwanTuning(map[string]string{}, false)
	t.Cleanup(func() {
		sdwanSysctl = previous
		rememberSdwanTuning(blocked, fqUnavailable)
	})
	return fake
}

func TestSdwanTuningAppliesTheNetworkProfile(t *testing.T) {
	fake := useFakeSysctl(t, untunedSysctl())
	before := ReadSdwanTuning()
	if !before.Supported || before.Optimized || before.CongestionControl != "cubic" || len(before.Pending) != 10 {
		t.Fatalf("stock kernel status = %#v", before)
	}
	result := ApplySdwanTuning()
	if len(result.Failed) != 0 || len(result.Applied) != 10 || !result.Persisted || !result.After.Optimized {
		t.Fatalf("tuning result = %#v", result)
	}
	want := map[string]string{
		"net.ipv4.tcp_congestion_control":    "bbr",
		"net.core.default_qdisc":             "fq",
		"net.ipv4.tcp_fastopen":              "3",
		"net.ipv4.tcp_mtu_probing":           "1",
		"net.ipv4.tcp_slow_start_after_idle": "0",
		"net.ipv4.tcp_notsent_lowat":         "16384",
		"net.core.rmem_max":                  "16777216",
		"net.ipv4.tcp_wmem":                  "4096 65536 16777216",
	}
	for key, value := range want {
		if fake.values[key] != value {
			t.Fatalf("%s = %q, want %q", key, fake.values[key], value)
		}
	}
	if !slices.Contains(fake.modules, "tcp_bbr") {
		t.Fatalf("BBR module was not loaded: %v", fake.modules)
	}
	persisted := map[string]string{}
	for _, setting := range fake.persisted {
		persisted[setting.Key] = setting.Value
	}
	for key, value := range want {
		if persisted[key] != value {
			t.Fatalf("persisted %s = %q, want %q (reboot would lose the tuning)", key, persisted[key], value)
		}
	}

	// Tuning is idempotent.
	again := ApplySdwanTuning()
	if len(again.Applied) != 0 || !again.After.Optimized {
		t.Fatalf("second run changed settings: %#v", again)
	}
}

func TestSdwanTuningKeepsBetterValues(t *testing.T) {
	fake := untunedSysctl()
	fake.values["net.ipv4.tcp_available_congestion_control"] = "reno cubic bbr3"
	fake.values["net.ipv4.tcp_congestion_control"] = "bbr3"
	fake.values["net.core.default_qdisc"] = "cake"
	fake.values["net.core.rmem_max"] = "67108864"
	fake.values["net.ipv4.tcp_rmem"] = "4096 262144 67108864"
	useFakeSysctl(t, fake)
	original := maps.Clone(fake.values)
	result := ApplySdwanTuning()
	for _, key := range []string{"net.ipv4.tcp_congestion_control", "net.core.default_qdisc", "net.core.rmem_max", "net.ipv4.tcp_rmem"} {
		if slices.Contains(result.Applied, key) || fake.values[key] != original[key] {
			t.Fatalf("%s was changed from a better value %q to %q", key, original[key], fake.values[key])
		}
	}
	if !result.After.Optimized {
		t.Fatalf("host with better values must count as optimized: %#v", result.After)
	}
}

func TestSdwanTuningReportsWhatCannotBeApplied(t *testing.T) {
	fake := untunedSysctl()
	fake.bbrModule = false
	fake.failing["net.core.rmem_max"] = true
	delete(fake.values, "net.ipv4.tcp_notsent_lowat") // older kernel
	useFakeSysctl(t, fake)
	result := ApplySdwanTuning()
	failed := map[string]string{}
	for _, failure := range result.Failed {
		failed[failure.Key] = failure.Error
	}
	if failed["net.ipv4.tcp_congestion_control"] != sdwanNoBBRError || failed["net.core.rmem_max"] == "" {
		t.Fatalf("failures = %#v", result.Failed)
	}
	if fake.values["net.ipv4.tcp_congestion_control"] != "cubic" || fake.values["net.core.default_qdisc"] != "fq" {
		t.Fatalf("the rest of the profile must still be applied: %#v", fake.values)
	}
	if _, exists := fake.values["net.ipv4.tcp_notsent_lowat"]; exists {
		t.Fatal("missing kernel knobs must be skipped, not created")
	}
	if result.After.Optimized || !slices.Contains(result.After.Pending, "net.ipv4.tcp_congestion_control") {
		t.Fatalf("after = %#v", result.After)
	}
}

func TestSdwanTuningSkipsUnsupportedHosts(t *testing.T) {
	fake := untunedSysctl()
	fake.available, fake.reason = false, "not_root"
	useFakeSysctl(t, fake)
	original := maps.Clone(fake.values)
	result := ApplySdwanTuning()
	if result.Before.Supported || result.Before.Reason != "not_root" || len(result.Applied) != 0 || fake.persisted != nil {
		t.Fatalf("result = %#v", result)
	}
	if !maps.Equal(original, fake.values) {
		t.Fatal("an unsupported host must not be modified")
	}
}

func TestSdwanTuningUsesSmallerBuffersOnSmallHosts(t *testing.T) {
	fake := untunedSysctl()
	fake.memTotal = 512 << 20
	useFakeSysctl(t, fake)
	ApplySdwanTuning()
	if fake.values["net.core.rmem_max"] != "8388608" || fake.values["net.ipv4.tcp_rmem"] != "4096 131072 8388608" {
		t.Fatalf("512 MiB host buffers = %q / %q", fake.values["net.core.rmem_max"], fake.values["net.ipv4.tcp_rmem"])
	}
}

func TestSdwanTuningFallsBackToFqCodel(t *testing.T) {
	fake := untunedSysctl()
	fake.rejected["net.core.default_qdisc=fq"] = true
	useFakeSysctl(t, fake)
	result := ApplySdwanTuning()
	if fake.values["net.core.default_qdisc"] != "fq_codel" || len(result.Failed) != 0 || !result.After.Optimized {
		t.Fatalf("kernel without fq: qdisc=%q result=%#v", fake.values["net.core.default_qdisc"], result)
	}
	if status := ReadSdwanTuning(); !status.Optimized {
		t.Fatalf("fq_codel must count as tuned when fq is missing: %#v", status)
	}
	persisted := map[string]string{}
	for _, setting := range fake.persisted {
		persisted[setting.Key] = setting.Value
	}
	if persisted["net.core.default_qdisc"] != "fq_codel" {
		t.Fatalf("persisted qdisc = %q", persisted["net.core.default_qdisc"])
	}
}

func TestSdwanTuningRemembersWhatTheKernelRejects(t *testing.T) {
	fake := untunedSysctl()
	fake.rejected["net.core.default_qdisc=fq"] = true
	fake.rejected["net.core.default_qdisc=fq_codel"] = true
	fake.bbrModule = false
	useFakeSysctl(t, fake)
	before := ReadSdwanTuning()
	if len(before.Blocked) != 0 {
		t.Fatalf("nothing is known to be blocked before tuning ran: %#v", before.Blocked)
	}
	result := ApplySdwanTuning()
	blocked := result.After.Blocked
	if blocked["net.core.default_qdisc"] != sdwanNoFairQueueing || blocked["net.ipv4.tcp_congestion_control"] != sdwanNoBBRError {
		t.Fatalf("blocked = %#v", blocked)
	}
	fixable, rejected, reason := splitSdwanPending(ReadSdwanTuning())
	if len(fixable) != 0 || len(rejected) != 2 || reason == "" {
		t.Fatalf("after tuning: fixable=%v blocked=%v reason=%q", fixable, rejected, reason)
	}

	report := &SdwanReport{CoreRunning: true, Controller: ReadSdwanTuning(), Nodes: []SdwanNodeReport{}}
	analyzeSdwanReport(report, SdwanSettings{})
	if findSdwanAdvice(report, "controller_tuning", 0) != nil {
		t.Fatal("settings the kernel rejects must not be offered as fixable")
	}
	if advice := findSdwanAdvice(report, "controller_tuning_blocked", 0); advice == nil || advice.Fixable || advice.Severity != sdwanSeverityInfo {
		t.Fatalf("blocked settings advice = %#v", report.Advice)
	}

	// Once the kernel accepts the setting (e.g. after a kernel upgrade) it
	// is fixable again and the memory is cleared.
	delete(fake.rejected, "net.core.default_qdisc=fq")
	ApplySdwanTuning()
	if fake.values["net.core.default_qdisc"] != "fq" {
		t.Fatalf("qdisc = %q", fake.values["net.core.default_qdisc"])
	}
	if status := ReadSdwanTuning(); status.Blocked["net.core.default_qdisc"] != "" {
		t.Fatalf("stale blocked entry: %#v", status.Blocked)
	}
}
