package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/logger"
)

// On OpenWrt, LAN devices ask the router's dnsmasq for names, and the TUN
// does not see queries addressed to the router itself. While the client runs
// with DNS takeover on, dnsmasq forwards to the client's DNS port instead,
// through a file in its runtime config directory: nothing persistent
// changes, and a reboot or a stopped client puts dnsmasq back as it was.

const proxyClientDnsmasqFile = "1s-ui-client.conf"

// Test hooks.
var (
	proxyClientDnsmasqDirs = func() []string {
		dirs, _ := filepath.Glob("/tmp/dnsmasq.cfg*.d")
		if info, err := os.Stat("/tmp/dnsmasq.d"); err == nil && info.IsDir() {
			dirs = append(dirs, "/tmp/dnsmasq.d")
		}
		return dirs
	}
	proxyClientRestartDnsmasq = func() error {
		output, err := exec.Command("/etc/init.d/dnsmasq", "restart").CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, output)
		}
		return nil
	}
)

func proxyClientDnsmasqContent(port int) string {
	return fmt.Sprintf("# Written by 1S-UI while its proxy client answers DNS; removed when it stops.\nno-resolv\nserver=127.0.0.1#%d\n", port)
}

// proxyClientDNSHijacked reports whether dnsmasq is pointed at the client.
func proxyClientDNSHijacked() bool {
	for _, dir := range proxyClientDnsmasqDirs() {
		if _, err := os.Stat(filepath.Join(dir, proxyClientDnsmasqFile)); err == nil {
			return true
		}
	}
	return false
}

// syncProxyClientDNS points dnsmasq at the client's DNS port while the
// client is serving it, and back otherwise.
func syncProxyClientDNS(settings ProxyClientSettings) {
	platform := proxyClientPlatform()
	want := settings.Enabled && settings.Tun && settings.DNSHijack && platform.OpenWrt && platform.Dnsmasq &&
		corePtr != nil && corePtr.HasInbound(proxyClientDNSInTag)
	writeProxyClientDnsmasq(want, settings.DNSPort)
}

// writeProxyClientDnsmasq adds or removes the dnsmasq file and restarts
// dnsmasq when that changed anything.
func writeProxyClientDnsmasq(want bool, port int) {
	content := proxyClientDnsmasqContent(port)
	changed := false
	for _, dir := range proxyClientDnsmasqDirs() {
		path := filepath.Join(dir, proxyClientDnsmasqFile)
		current, err := os.ReadFile(path)
		exists := err == nil
		switch {
		case want && string(current) != content:
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				logger.Warning("point dnsmasq at the proxy client: ", err)
				continue
			}
			changed = true
		case !want && exists:
			if err := os.Remove(path); err != nil {
				logger.Warning("restore dnsmasq: ", err)
				continue
			}
			changed = true
		}
	}
	if changed {
		if err := proxyClientRestartDnsmasq(); err != nil {
			logger.Warning("restart dnsmasq: ", err)
		}
	}
}

// SyncDNS re-checks the dnsmasq hand-off; the cron job calls it so DNS goes
// back to normal soon after sing-box stops.
func (s *ProxyClientService) SyncDNS() {
	syncProxyClientDNS(loadProxyClientSettings(database.GetDB()))
}
