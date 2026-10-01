package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Hhz0823/1s-ui/config"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/logger"

	"github.com/sagernet/sing-box/common/srs"
)

// Rule sets of the proxy client, kept as local files so sing-box never
// needs the network to start. They are small (tens of KB) and load into a
// few MB, which suits routers.
const (
	ruleSetGeositeCN  = "client-geosite-cn"
	ruleSetGeoIPCN    = "client-geoip-cn"
	ruleSetGeositeGFW = "client-geosite-gfw"

	proxyClientRuleSetMaxAge = 7 * 24 * time.Hour
	proxyClientRuleSetMax    = 8 << 20
)

// proxyClientRuleSetSources lists where each file comes from; mirrors serve
// mainland China, where raw.githubusercontent.com is often unreachable.
var proxyClientRuleSetSources = map[string][]string{
	ruleSetGeositeCN: {
		"https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs",
		"https://testingcf.jsdelivr.net/gh/SagerNet/sing-geosite@rule-set/geosite-cn.srs",
		"https://ghfast.top/https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs",
		"https://fastly.jsdelivr.net/gh/SagerNet/sing-geosite@rule-set/geosite-cn.srs",
	},
	ruleSetGeoIPCN: {
		"https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs",
		"https://testingcf.jsdelivr.net/gh/SagerNet/sing-geoip@rule-set/geoip-cn.srs",
		"https://ghfast.top/https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs",
		"https://fastly.jsdelivr.net/gh/SagerNet/sing-geoip@rule-set/geoip-cn.srs",
	},
	ruleSetGeositeGFW: {
		"https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/gfw.srs",
		"https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geosite/gfw.srs",
		"https://ghfast.top/https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/gfw.srs",
		"https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geosite/gfw.srs",
	},
}

// proxyClientRuleSetFiles maps a tag to its file name.
var proxyClientRuleSetFiles = map[string]string{
	ruleSetGeositeCN:  "geosite-cn.srs",
	ruleSetGeoIPCN:    "geoip-cn.srs",
	ruleSetGeositeGFW: "geosite-gfw.srs",
}

// Test hooks.
var (
	proxyClientRuleSetDir    = func() string { return filepath.Join(config.GetDBFolderPath(), "rulesets") }
	proxyClientRuleSetClient = &http.Client{
		Timeout:   40 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, TLSHandshakeTimeout: 10 * time.Second},
	}
)

var proxyClientRuleSetMu sync.Mutex

func proxyClientRuleSetsFor(mode string) []string {
	switch mode {
	case ProxyClientModeRule:
		return []string{ruleSetGeositeCN, ruleSetGeoIPCN}
	case ProxyClientModeGFW:
		return []string{ruleSetGeositeGFW}
	}
	return nil
}

func proxyClientRuleSetPath(tag string) string {
	return filepath.Join(proxyClientRuleSetDir(), proxyClientRuleSetFiles[tag])
}

func proxyClientRuleSetReady(tag string) bool {
	info, err := os.Stat(proxyClientRuleSetPath(tag))
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

// ProxyClientRuleSetStatus is one rule file on disk.
type ProxyClientRuleSetStatus struct {
	Tag       string `json:"tag"`
	Ready     bool   `json:"ready"`
	Size      int64  `json:"size"`
	UpdatedAt int64  `json:"updated_at"`
}

func proxyClientRuleSetStatus() []ProxyClientRuleSetStatus {
	var result []ProxyClientRuleSetStatus
	for _, tag := range []string{ruleSetGeositeCN, ruleSetGeoIPCN, ruleSetGeositeGFW} {
		status := ProxyClientRuleSetStatus{Tag: tag}
		if info, err := os.Stat(proxyClientRuleSetPath(tag)); err == nil && info.Size() > 0 {
			status.Ready, status.Size, status.UpdatedAt = true, info.Size(), info.ModTime().Unix()
		}
		result = append(result, status)
	}
	return result
}

// ensureProxyClientRuleSets downloads the files a mode needs that are
// missing, or every one of them when force is set.
func ensureProxyClientRuleSets(mode string, force bool) error {
	var failures []string
	for _, tag := range proxyClientRuleSetsFor(mode) {
		if !force && proxyClientRuleSetReady(tag) {
			continue
		}
		if err := downloadProxyClientRuleSet(tag); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

// UpdateRuleSets downloads the current mode's rule files again and restarts
// sing-box to load them.
func (s *ProxyClientService) UpdateRuleSets() (*ProxyClientState, error) {
	settings := loadProxyClientSettings(database.GetDB())
	mode := settings.Mode
	if mode == ProxyClientModeGlobal {
		mode = ProxyClientModeRule
	}
	if err := ensureProxyClientRuleSets(mode, true); err != nil {
		return nil, err
	}
	if settings.Enabled && corePtr != nil && corePtr.IsRunning() {
		if err := s.ConfigService.RestartCore(); err != nil {
			return nil, err
		}
	}
	return s.State()
}

// RefreshOldRuleSets updates rule files older than a week; the cron job
// calls it.
func (s *ProxyClientService) RefreshOldRuleSets(now time.Time) {
	settings := loadProxyClientSettings(database.GetDB())
	if !settings.Enabled {
		return
	}
	changed := false
	for _, tag := range proxyClientRuleSetsFor(settings.Mode) {
		info, err := os.Stat(proxyClientRuleSetPath(tag))
		if err == nil && now.Sub(info.ModTime()) < proxyClientRuleSetMaxAge {
			continue
		}
		if err := downloadProxyClientRuleSet(tag); err != nil {
			logger.Warning("update rule file ", tag, ": ", err)
			continue
		}
		changed = true
	}
	if changed && corePtr != nil && corePtr.IsRunning() {
		if err := s.ConfigService.RestartCore(); err != nil {
			logger.Warning("reload rule files: ", err)
		}
	}
}

func downloadProxyClientRuleSet(tag string) error {
	sources := proxyClientRuleSetSources[tag]
	if len(sources) == 0 {
		return fmt.Errorf("unknown rule file %s", tag)
	}
	proxyClientRuleSetMu.Lock()
	defer proxyClientRuleSetMu.Unlock()
	var lastErr error
	for _, source := range sources {
		data, err := fetchRuleSet(source)
		if err == nil {
			// Only a file sing-box can read replaces the old one.
			_, err = srs.Read(bytes.NewReader(data), false)
			if err != nil {
				err = fmt.Errorf("%s is not a sing-box rule set: %v", source, err)
			}
		}
		if err != nil {
			lastErr = err
			continue
		}
		dir := proxyClientRuleSetDir()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		temp, err := os.CreateTemp(dir, ".download-*")
		if err != nil {
			return err
		}
		_, writeErr := temp.Write(data)
		closeErr := temp.Close()
		if writeErr != nil || closeErr != nil {
			_ = os.Remove(temp.Name())
			return errors.Join(writeErr, closeErr)
		}
		if err := os.Rename(temp.Name(), proxyClientRuleSetPath(tag)); err != nil {
			_ = os.Remove(temp.Name())
			return err
		}
		return nil
	}
	return fmt.Errorf("could not download %s: %v", proxyClientRuleSetFiles[tag], lastErr)
}

func fetchRuleSet(source string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "1s-ui")
	response, err := proxyClientRuleSetClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered HTTP %d", source, response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, proxyClientRuleSetMax+1))
	if err != nil {
		return nil, err
	}
	if len(data) > proxyClientRuleSetMax {
		return nil, fmt.Errorf("%s is too large", source)
	}
	return data, nil
}
