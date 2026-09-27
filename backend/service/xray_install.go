//go:build !openwrt_lite

package service

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Hhz0823/1s-ui/config"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/logger"
)

const (
	xrayRepo           = "XTLS/Xray-core"
	xrayReleaseAPI     = "https://api.github.com/repos/" + xrayRepo + "/releases/latest"
	maxXrayArchiveSize = 128 << 20
	maxXrayMemberSize  = 96 << 20

	// xrayFallbackTag is installed when GitHub cannot say which release is
	// the latest: the Xray-core version the panel is tested with.
	xrayFallbackTag = "v26.3.27"
)

// xrayFallbackDigests are the SHA2-256 values XTLS published in the .dgst
// files of xrayFallbackTag, so any mirror can serve those archives.
var xrayFallbackDigests = map[string]string{
	"Xray-linux-64.zip":        "23cd9af937744d97776ee35ecad4972cf4b2109d1e0fe6be9930467608f7c8ae",
	"Xray-linux-32.zip":        "d1eeb0d9a9106eefd286fbb73595c2dfe1c48c56aa91ba1c9aefe04f188d0927",
	"Xray-linux-arm64-v8a.zip": "4d30283ae614e3057f730f67cd088a42be6fdf91f8639d82cb69e48cde80413c",
	"Xray-linux-arm32-v5.zip":  "0da9a632e15e82504831f61bf6b46c21e3081bcc79ad46bdc16e7dc2f0dc9088",
	"Xray-linux-arm32-v6.zip":  "0c6e751e2bba3f3ff09a793dd6a6bc45fd6fd89f49b49dfc0cf6d922dc123bec",
	"Xray-linux-arm32-v7a.zip": "c7265ae13c63ca0241a037df4ef960ad37938c8a67d984cc08834b2cfdf5654b",
	"Xray-linux-s390x.zip":     "a209bcea3df9b0dc1ef5938695679ba1308ad9678a671279d7b1b6c5ceec09c7",
}

var (
	xrayReleaseAPIURL = xrayReleaseAPI
	xrayHTTPClient    = newGitHubHTTPClient()
	xrayInstallState  = struct {
		sync.Mutex
		value XrayInstallProgress
	}{value: XrayInstallProgress{State: "idle"}}
)

// XrayInstallService installs Xray-core without starting it. Low-resource
// hosts keep sing-box as the default and start Xray only after a panel action.
type XrayInstallService struct{}

type XrayInstallProgress struct {
	State   string `json:"state"`
	Version string `json:"version"`
	Message string `json:"message"`
	Started int64  `json:"started"`
}

type XrayInstallStatus struct {
	Supported    bool                `json:"supported"`
	CanInstall   bool                `json:"can_install"`
	Installed    bool                `json:"installed"`
	Disabled     bool                `json:"disabled"`
	OnDemand     bool                `json:"on_demand"`
	Running      bool                `json:"running"`
	HasInbounds  bool                `json:"has_inbounds"`
	LowResource  bool                `json:"low_resource"`
	ExclusiveRun bool                `json:"exclusive_run"`
	Architecture string              `json:"architecture"`
	Version      string              `json:"version"`
	Path         string              `json:"path"`
	Capability   string              `json:"capability"`
	CPUCores     int                 `json:"cpu_cores"`
	MemoryBytes  uint64              `json:"memory_bytes"`
	Install      XrayInstallProgress `json:"install"`
}

type xrayRelease struct {
	TagName    string         `json:"tag_name"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
	Assets     []releaseAsset `json:"assets"`
}

func (s *XrayInstallService) Status() XrayInstallStatus {
	supported, capability := xrayInstallCapability()
	cpuCount, memoryBytes := currentClusterCapacity()
	status := baseXrayInstallStatus(supported, capability, cpuCount, memoryBytes, snapshotXrayInstallProgress())
	status.Disabled = config.IsXrayDisabled()
	status.OnDemand = config.IsXrayOnDemand()
	status.Path = config.GetXrayPath()
	if info, err := os.Stat(status.Path); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
		status.Installed = true
		status.Version, _ = xrayBinaryVersion(status.Path)
	}
	if xrayPtr != nil {
		status.Running = xrayPtr.IsRunning()
	}
	if database.GetDB() != nil {
		status.HasInbounds, _ = (&ConfigService{}).HasXrayInbounds()
	}
	status.ExclusiveRun = status.LowResource
	return status
}

func baseXrayInstallStatus(supported bool, capability string, cpuCount int, memoryBytes uint64, progress XrayInstallProgress) XrayInstallStatus {
	return XrayInstallStatus{
		Supported:    supported,
		CanInstall:   supported && !isXrayInstallRunning(progress.State),
		Architecture: runtime.GOARCH,
		Capability:   capability,
		CPUCores:     cpuCount,
		MemoryBytes:  memoryBytes,
		LowResource:  memoryBytes > 0 && memoryBytes < LowResourceCoreMemBytes,
		ExclusiveRun: memoryBytes > 0 && memoryBytes < LowResourceCoreMemBytes,
		Install:      progress,
	}
}

func (s *XrayInstallService) StartInstall() (XrayInstallStatus, error) {
	status := s.Status()
	if !status.Supported {
		return status, fmt.Errorf("Xray-core installation unavailable: %s", status.Capability)
	}
	if status.Running {
		return status, fmt.Errorf("stop Xray-core before installing or updating it")
	}

	xrayInstallState.Lock()
	if isXrayInstallRunning(xrayInstallState.value.State) {
		xrayInstallState.Unlock()
		return s.Status(), fmt.Errorf("Xray-core installation is already running")
	}
	xrayInstallState.value = XrayInstallProgress{
		State:   "downloading",
		Message: "downloading the official Xray-core release",
		Started: time.Now().Unix(),
	}
	xrayInstallState.Unlock()

	// A fresh install becomes available in on-demand mode. Reinstalling a
	// deliberately disabled component preserves that administrator choice.
	enableAfterInstall := !status.Installed || !status.Disabled
	go runXrayInstall(enableAfterInstall)
	return s.Status(), nil
}

func (s *XrayInstallService) SetEnabled(enabled bool) (XrayInstallStatus, error) {
	status := s.Status()
	if isXrayInstallRunning(status.Install.State) {
		return status, fmt.Errorf("Xray-core installation is in progress")
	}
	if !status.Installed {
		return status, fmt.Errorf("Xray-core is not installed")
	}
	if enabled {
		if err := persistXrayRuntimeState(true); err != nil {
			return s.Status(), err
		}
		return s.Status(), nil
	}
	if err := (&ConfigService{}).StopXrayCore(); err != nil {
		return status, err
	}
	if err := persistXrayRuntimeState(false); err != nil {
		return s.Status(), err
	}
	return s.Status(), nil
}

func (s *XrayInstallService) Uninstall() (XrayInstallStatus, error) {
	status := s.Status()
	if isXrayInstallRunning(status.Install.State) {
		return status, fmt.Errorf("Xray-core installation is in progress")
	}
	hasInbounds, err := (&ConfigService{}).HasXrayInbounds()
	if err != nil {
		return status, err
	}
	if hasInbounds {
		return status, fmt.Errorf("Xray-core still has configured inbounds; delete or migrate them before uninstalling")
	}
	if !status.Supported {
		return status, fmt.Errorf("Xray-core removal unavailable: %s", status.Capability)
	}
	if err := (&ConfigService{}).StopXrayCore(); err != nil {
		return status, err
	}
	if err := persistXrayRuntimeState(false); err != nil {
		return s.Status(), err
	}
	target := filepath.Clean(config.GetXrayPath())
	for _, file := range []string{target, config.GetXrayConfigPath()} {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return s.Status(), err
		}
	}
	return s.Status(), nil
}

func runXrayInstall(enableAfterInstall bool) {
	release, asset, err := resolveXrayRelease()
	if err != nil {
		finishXrayInstall("failed", "", err.Error())
		return
	}
	setXrayInstallProgress("downloading", release.TagName, "downloading the official Xray-core archive")
	archivePath, err := downloadXrayArchive(release.TagName, asset)
	if err != nil {
		finishXrayInstall("failed", release.TagName, err.Error())
		return
	}
	defer os.Remove(archivePath)

	setXrayInstallProgress("installing", release.TagName, "validating and installing Xray-core")
	extractDir, err := os.MkdirTemp("", "1s-ui-xray-*")
	if err != nil {
		finishXrayInstall("failed", release.TagName, err.Error())
		return
	}
	defer os.RemoveAll(extractDir)
	if err := extractXrayArchive(archivePath, extractDir); err != nil {
		finishXrayInstall("failed", release.TagName, err.Error())
		return
	}

	binary := filepath.Join(extractDir, "xray")
	version, err := xrayBinaryVersion(binary)
	if err != nil {
		finishXrayInstall("failed", release.TagName, fmt.Sprintf("Xray-core validation failed: %v", err))
		return
	}
	if err := installXrayFiles(extractDir); err != nil {
		finishXrayInstall("failed", release.TagName, err.Error())
		return
	}
	if err := persistXrayRuntimeState(enableAfterInstall); err != nil {
		finishXrayInstall("failed", release.TagName, fmt.Sprintf("Xray-core was installed but its runtime state could not be persisted: %v", err))
		return
	}
	message := fmt.Sprintf("%s installed; sing-box remains the default", version)
	if !enableAfterInstall {
		message = fmt.Sprintf("%s installed; Xray-core remains disabled", version)
	}
	finishXrayInstall("success", release.TagName, message)
}

func xrayInstallCapability() (bool, string) {
	if runtime.GOOS != "linux" {
		return false, "installation from the panel is supported on Linux only"
	}
	if !runningAsRoot() {
		return false, "the panel service must run as root"
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false, "systemd/systemctl is not available"
	}
	if _, err := installedPanelExecutable(); err != nil {
		return false, err.Error()
	}
	if xrayReleaseAssetName(runtime.GOARCH, strings.TrimSpace(os.Getenv("GOARM"))) == "" {
		return false, "this CPU architecture has no automatic Xray-core package"
	}
	target := filepath.Clean(config.GetXrayPath())
	if !filepath.IsAbs(target) || !strings.HasPrefix(target, "/usr/local/s-ui/bin/") {
		return false, "Xray-core target must be under /usr/local/s-ui/bin"
	}
	return true, "ready"
}

// resolveXrayRelease picks the archive to install: the latest stable release
// from the GitHub API or from github.com, else xrayFallbackTag. Archives from
// the API carry its SHA-256, those found on github.com the one in XTLS's .dgst
// file there.
func resolveXrayRelease() (xrayRelease, releaseAsset, error) {
	wanted := xrayReleaseAssetName(runtime.GOARCH, strings.TrimSpace(os.Getenv("GOARM")))
	if wanted == "" {
		return xrayRelease{}, releaseAsset{}, fmt.Errorf("this CPU architecture has no automatic Xray-core package")
	}
	var attempts githubAttempts
	release, asset, err := fetchLatestXrayRelease()
	if err == nil {
		return release, asset, nil
	}
	attempts.add(urlHost(xrayReleaseAPIURL), err)
	github := githubSource{trusted: true}
	tag, err := githubLatestTag(xrayHTTPClient, github, xrayRepo)
	if err == nil {
		asset = releaseAsset{Name: wanted, BrowserDownloadURL: githubAssetURL(xrayRepo, tag, wanted)}
		// Without the .dgst file the archive is only fetched from GitHub or
		// the administrator's mirror.
		asset.Digest, _ = fetchXrayDigest(asset.BrowserDownloadURL)
		return xrayRelease{TagName: tag}, asset, nil
	}
	attempts.add(github.name(), err)
	if digest := xrayFallbackDigests[wanted]; digest != "" {
		logger.Warning("installing the tested Xray-core ", xrayFallbackTag, ": ", attempts.err("Xray-core release lookup"))
		return xrayRelease{TagName: xrayFallbackTag}, releaseAsset{
			Name:               wanted,
			BrowserDownloadURL: githubAssetURL(xrayRepo, xrayFallbackTag, wanted),
			Digest:             "sha256:" + digest,
		}, nil
	}
	return xrayRelease{}, releaseAsset{}, attempts.err("Xray-core release lookup")
}

// fetchXrayDigest reads the SHA2-256 line of the .dgst file XTLS publishes
// next to every archive.
func fetchXrayDigest(assetURL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), githubLookupTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL+".dgst", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "1s-ui-xray-installer")
	response, err := xrayHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Xray-core digest returned %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4096))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(body), "\n") {
		if value, found := strings.CutPrefix(strings.TrimSpace(line), "SHA2-256="); found {
			value = strings.ToLower(strings.TrimSpace(value))
			if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == sha256.Size {
				return "sha256:" + value, nil
			}
		}
	}
	return "", fmt.Errorf("Xray-core digest file has no SHA2-256")
}

func fetchLatestXrayRelease() (xrayRelease, releaseAsset, error) {
	ctx, cancel := context.WithTimeout(context.Background(), githubLookupTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, xrayReleaseAPIURL, nil)
	if err != nil {
		return xrayRelease{}, releaseAsset{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "1s-ui-xray-installer")
	response, err := xrayHTTPClient.Do(req)
	if err != nil {
		return xrayRelease{}, releaseAsset{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return xrayRelease{}, releaseAsset{}, fmt.Errorf("Xray-core release API returned %s", response.Status)
	}
	var release xrayRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&release); err != nil {
		return xrayRelease{}, releaseAsset{}, err
	}
	if release.Draft || release.Prerelease || !validVersion(release.TagName) {
		return xrayRelease{}, releaseAsset{}, fmt.Errorf("latest Xray-core release is not stable")
	}
	wanted := xrayReleaseAssetName(runtime.GOARCH, strings.TrimSpace(os.Getenv("GOARM")))
	for _, asset := range release.Assets {
		if asset.Name == wanted {
			if asset.Size <= 0 || asset.Size > maxXrayArchiveSize {
				return xrayRelease{}, releaseAsset{}, fmt.Errorf("Xray-core archive size is invalid")
			}
			return release, asset, nil
		}
	}
	return xrayRelease{}, releaseAsset{}, fmt.Errorf("Xray-core release does not contain %s", wanted)
}

func xrayReleaseAssetName(goarch, goarm string) string {
	switch goarch {
	case "amd64":
		return "Xray-linux-64.zip"
	case "386":
		return "Xray-linux-32.zip"
	case "arm64":
		return "Xray-linux-arm64-v8a.zip"
	case "arm":
		switch goarm {
		case "5":
			return "Xray-linux-arm32-v5.zip"
		case "6":
			return "Xray-linux-arm32-v6.zip"
		default:
			return "Xray-linux-arm32-v7a.zip"
		}
	case "s390x":
		return "Xray-linux-s390x.zip"
	default:
		return ""
	}
}

func downloadXrayArchive(tag string, asset releaseAsset) (string, error) {
	if !isGitHubAssetURL(asset.BrowserDownloadURL, xrayRepo) {
		return "", fmt.Errorf("Xray-core release has an untrusted download URL")
	}
	return downloadGitHubAsset(xrayHTTPClient, asset.BrowserDownloadURL, asset.Digest, maxXrayArchiveSize, "1s-ui-xray-*.zip", func(source string) {
		setXrayInstallProgress("downloading", tag, "downloading Xray-core "+tag+" from "+source)
	})
}

func extractXrayArchive(archivePath, destination string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	foundBinary := false
	for _, member := range reader.File {
		clean := path.Clean(strings.TrimPrefix(member.Name, "./"))
		if clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
			return fmt.Errorf("Xray-core archive contains an unsafe path")
		}
		name := path.Base(clean)
		mode := os.FileMode(0644)
		switch name {
		case "xray":
			mode = 0755
			foundBinary = true
		case "geoip.dat", "geosite.dat":
		default:
			continue
		}
		if !member.Mode().IsRegular() || member.UncompressedSize64 > maxXrayMemberSize {
			return fmt.Errorf("Xray-core archive member %s is invalid", name)
		}
		input, err := member.Open()
		if err != nil {
			return err
		}
		target := filepath.Join(destination, name)
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			input.Close()
			return err
		}
		written, copyErr := io.Copy(output, io.LimitReader(input, maxXrayMemberSize+1))
		closeErr := output.Close()
		input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if written > maxXrayMemberSize {
			return fmt.Errorf("Xray-core archive member %s is too large", name)
		}
	}
	if !foundBinary {
		return fmt.Errorf("Xray-core archive does not contain the xray binary")
	}
	return nil
}

func xrayBinaryVersion(binary string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, binary, "version").CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(output)))
	}
	version := strings.TrimSpace(string(output))
	if line, _, found := strings.Cut(version, "\n"); found {
		version = strings.TrimSpace(line)
	}
	if !strings.HasPrefix(strings.ToLower(version), "xray ") {
		return "", fmt.Errorf("unexpected Xray-core version output")
	}
	return version, nil
}

func installXrayFiles(sourceDir string) error {
	target := config.GetXrayPath()
	if err := replaceFileAtomic(filepath.Join(sourceDir, "xray"), target, 0755); err != nil {
		return err
	}
	for _, name := range []string{"geoip.dat", "geosite.dat"} {
		source := filepath.Join(sourceDir, name)
		if _, err := os.Stat(source); err == nil {
			if err := replaceFileAtomic(source, filepath.Join(filepath.Dir(target), name), 0644); err != nil {
				return err
			}
		}
	}
	return nil
}

func persistXrayRuntimeState(enabled bool) error {
	dbFolder := config.GetDBFolderPath()
	if err := os.MkdirAll(dbFolder, 0750); err != nil {
		return err
	}
	disabledMarker := filepath.Join(dbFolder, ".disable_xray")
	enabledMarker := filepath.Join(dbFolder, ".xray_enabled")
	onDemandMarker := filepath.Join(dbFolder, ".xray_on_demand")
	if enabled {
		if err := os.Remove(disabledMarker); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.WriteFile(enabledMarker, nil, 0600); err != nil {
			return err
		}
		if err := os.WriteFile(onDemandMarker, nil, 0600); err != nil {
			return err
		}
	} else {
		if err := os.Remove(enabledMarker); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.WriteFile(disabledMarker, nil, 0600); err != nil {
			return err
		}
		if err := os.Remove(onDemandMarker); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	dropIn := []byte("[Service]\nEnvironment=SUI_DISABLE_XRAY=true\nEnvironment=SUI_XRAY_ON_DEMAND=false\n")
	if enabled {
		dropIn = []byte("[Service]\nEnvironment=SUI_DISABLE_XRAY=false\nEnvironment=SUI_XRAY_ON_DEMAND=true\n")
	}
	// This file sorts after installer-owned optimize.conf, so Web choices stay
	// effective after panel restarts and future upgrades.
	if err := writeXrayAtomicFile("/etc/systemd/system/s-ui.service.d/99-xray-runtime.conf", dropIn, 0644); err != nil {
		return err
	}
	if output, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemd daemon-reload failed: %v: %s", err, strings.TrimSpace(string(output)))
	}
	if enabled {
		if err := os.Setenv("SUI_DISABLE_XRAY", "false"); err != nil {
			return err
		}
		return os.Setenv("SUI_XRAY_ON_DEMAND", "true")
	}
	if err := os.Setenv("SUI_DISABLE_XRAY", "true"); err != nil {
		return err
	}
	return os.Setenv("SUI_XRAY_ON_DEMAND", "false")
}

func writeXrayAtomicFile(target string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".1s-ui-xray-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, target)
}

func snapshotXrayInstallProgress() XrayInstallProgress {
	xrayInstallState.Lock()
	defer xrayInstallState.Unlock()
	return xrayInstallState.value
}

func setXrayInstallProgress(state, version, message string) {
	xrayInstallState.Lock()
	defer xrayInstallState.Unlock()
	xrayInstallState.value.State = state
	xrayInstallState.value.Version = version
	xrayInstallState.value.Message = message
}

func finishXrayInstall(state, version, message string) {
	setXrayInstallProgress(state, version, message)
}

func isXrayInstallRunning(state string) bool {
	return state == "downloading" || state == "installing"
}
