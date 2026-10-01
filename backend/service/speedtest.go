package service

import (
	"encoding/json"
	"net"
	"net/netip"
	"sort"
	"strings"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/speedtest"
	"github.com/Hhz0823/1s-ui/util/common"
)

// SpeedtestTarget is a speed test session plus the addresses the phone can
// try to reach the server at, best first.
type SpeedtestTarget struct {
	speedtest.Session
	Hosts []string `json:"hosts"`
}

type SpeedtestStartRequest struct {
	Port int `json:"port"`
}

// StartLocalSpeedtest opens a session on this server.
func StartLocalSpeedtest(port int) (*SpeedtestTarget, error) {
	session, err := speedtest.Start(port)
	if err != nil {
		return nil, err
	}
	return &SpeedtestTarget{Session: *session, Hosts: localAddresses()}, nil
}

// StartSpeedtest opens a session on the panel host (0) or a managed server.
func (s *MonitorService) StartSpeedtest(serverID uint) (*SpeedtestTarget, error) {
	settings := s.SettingService.GetMonitorAppSettings()
	if serverID == 0 {
		return StartLocalSpeedtest(settings.SpeedtestPort)
	}
	view, err := s.AgentService.Get(serverID)
	if err != nil {
		return nil, err
	}
	response, err := s.AgentService.DispatchRPC(serverID, agent.RPCMethodSpeedtestStart, SpeedtestStartRequest{Port: settings.SpeedtestPort}, "monitor-app")
	if err != nil {
		if strings.Contains(err.Error(), "unsupported") {
			return nil, common.NewError("the panel on this server is too old for speed tests; update it")
		}
		return nil, err
	}
	var target SpeedtestTarget
	if err := json.Unmarshal(response.Payload, &target); err != nil || target.Token == "" {
		return nil, common.NewError("the server returned an invalid speed test session")
	}
	candidates := []string{ManagedNodePublicHost(view), view.PublicHost, view.RemoteIP}
	candidates = append(candidates, target.Hosts...)
	candidates = append(candidates, view.Report.IPv4...)
	candidates = append(candidates, view.Report.IPv6...)
	target.Hosts = speedtestHosts(candidates)
	return &target, nil
}

// speedtestHosts keeps public addresses and host names, without repeats.
func speedtestHosts(candidates []string) []string {
	hosts := []string{}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		candidate = strings.Trim(strings.TrimSpace(candidate), "[]")
		if candidate == "" || seen[candidate] {
			continue
		}
		if ip, err := netip.ParseAddr(candidate); err == nil && !publicAddress(ip) {
			continue
		}
		seen[candidate] = true
		hosts = append(hosts, candidate)
	}
	return hosts
}

func publicAddress(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !(ip.Is4() && ip.As4()[0] == 100 && ip.As4()[1]&0xc0 == 64)
}

// localAddresses lists this host's public interface addresses, IPv4 first.
func localAddresses() []string {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return []string{}
	}
	var v4, v6 []string
	for _, address := range addresses {
		prefix, err := netip.ParsePrefix(address.String())
		if err != nil || !publicAddress(prefix.Addr()) {
			continue
		}
		if prefix.Addr().Is4() {
			v4 = append(v4, prefix.Addr().String())
		} else {
			v6 = append(v6, prefix.Addr().String())
		}
	}
	sort.Strings(v4)
	sort.Strings(v6)
	return append(append([]string{}, v4...), v6...)
}
