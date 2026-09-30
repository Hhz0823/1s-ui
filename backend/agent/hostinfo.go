package agent

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
)

type staticHostInfo struct {
	cpuModel       string
	platform       string
	kernel         string
	virtualization string
}

var (
	hostInfoOnce  sync.Once
	hostInfoCache staticHostInfo
)

// hostFacts reads facts that do not change while the agent runs.
func hostFacts() staticHostInfo {
	hostInfoOnce.Do(func() {
		if values, err := cpu.Info(); err == nil && len(values) > 0 {
			hostInfoCache.cpuModel = strings.TrimSpace(values[0].ModelName)
		}
		if info, err := host.Info(); err == nil {
			hostInfoCache.platform = strings.TrimSpace(strings.Join([]string{info.Platform, info.PlatformVersion}, " "))
			hostInfoCache.kernel = strings.TrimSpace(info.KernelVersion)
			if info.VirtualizationRole == "guest" {
				hostInfoCache.virtualization = info.VirtualizationSystem
			}
		}
		if hostInfoCache.virtualization == "" {
			hostInfoCache.virtualization = detectContainer()
		}
	})
	return hostInfoCache
}

func detectContainer() string {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker"
	}
	if data, err := os.ReadFile("/proc/1/environ"); err == nil && strings.Contains(string(data), "container=lxc") {
		return "lxc"
	}
	return ""
}

func applyHostFacts(report *Report) {
	facts := hostFacts()
	report.CPUModel = facts.cpuModel
	report.Platform = facts.platform
	report.Kernel = facts.kernel
	report.Virtualization = facts.virtualization
	report.TCPConns, report.UDPConns = socketCounts()
}

// socketCounts sums in-use TCP and UDP sockets. sockstat is a few lines even
// on busy proxy hosts, unlike /proc/net/tcp.
func socketCounts() (int, int) {
	tcp, udp := 0, 0
	for _, path := range []string{"/proc/net/sockstat", "/proc/net/sockstat6"} {
		t, u := parseSockstat(path)
		tcp += t
		udp += u
	}
	return tcp, udp
}

func parseSockstat(path string) (int, int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0
	}
	return parseSockstatText(string(data))
}

func parseSockstatText(text string) (int, int) {
	tcp, udp := 0, 0
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != "inuse" {
			continue
		}
		value, _ := strconv.Atoi(fields[2])
		switch fields[0] {
		case "TCP:", "TCP6:":
			tcp += value
		case "UDP:", "UDP6:":
			udp += value
		}
	}
	return tcp, udp
}
