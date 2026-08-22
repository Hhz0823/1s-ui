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
	"net/url"
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
)

const (
	xrayReleaseAPI     = "https://api.github.com/repos/XTLS/Xray-core/releases/latest"
	maxXrayArchiveSize = 128 << 20
	maxXrayMemberSize  = 96 << 20
)

var (
	xrayReleaseAPIURL = xrayReleaseAPI
	xrayHTTPClient    = &http.Client{Timeout: 45 * time.Second}
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
	release, asset, err := fetchLatestXrayRelease()
	if err != nil {
		finishXrayInstall("failed", "", err.Error())
		return
	}
	setXrayInstallProgress("downloading", release.TagName, "downloading the official Xray-core archive")
	archivePath, err := downloadXrayArchive(asset)
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

func fetchLatestXrayRelease() (xrayRelease, releaseAsset, error) {
	req, err := http.NewRequest(http.MethodGet, xrayReleaseAPIURL, nil)
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

func downloadXrayArchive(asset releaseAsset) (string, error) {
	parsed, err := url.Parse(asset.BrowserDownloadURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "github.com" || !strings.HasPrefix(parsed.Path, "/XTLS/Xray-core/releases/download/") {
		return "", fmt.Errorf("Xray-core release has an untrusted download URL")
	}
	req, err := http.NewRequest(http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "1s-ui-xray-installer")
	response, err := xrayHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Xray-core download returned %s", response.Status)
	}
	if response.ContentLength > maxXrayArchiveSize {
		return "", fmt.Errorf("Xray-core archive exceeds %d MiB", maxXrayArchiveSize>>20)
	}
	file, err := os.CreateTemp("", "1s-ui-xray-*.zip")
	if err != nil {
		return "", err
	}
	name := file.Name()
	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = os.Remove(name)
		}
	}()
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(response.Body, maxXrayArchiveSize+1))
	if err != nil {
		return "", err
	}
	if written > maxXrayArchiveSize {
		return "", fmt.Errorf("Xray-core archive exceeds %d MiB", maxXrayArchiveSize>>20)
	}
	if err := verifyReleaseDigest(asset.Digest, hash.Sum(nil)); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	success = true
	return name, nil
}

func verifyReleaseDigest(digest string, actual []byte) error {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return nil
	}
	algorithm, expected, found := strings.Cut(digest, ":")
	if !found || !strings.EqualFold(algorithm, "sha256") {
		return fmt.Errorf("Xray-core release uses an unsupported digest")
	}
	decoded, err := hex.DecodeString(strings.TrimSpace(expected))
	if err != nil || len(decoded) != sha256.Size {
		return fmt.Errorf("Xray-core release digest is invalid")
	}
	if !strings.EqualFold(hex.EncodeToString(decoded), hex.EncodeToString(actual)) {
		return fmt.Errorf("Xray-core archive checksum mismatch")
	}
	return nil
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
