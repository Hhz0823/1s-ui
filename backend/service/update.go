package service

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Hhz0823/1s-ui/config"
)

const (
	panelReleaseAPI = "https://api.github.com/repos/Hhz0823/1s-ui/releases/latest"
	maxReleaseSize  = 256 << 20
	maxMemberSize   = 160 << 20
)

var (
	panelReleaseAPIURL = panelReleaseAPI
	panelHTTPClient    = &http.Client{Timeout: 20 * time.Second}
	panelUpdateState   = struct {
		sync.Mutex
		value UpdateStatus
	}{value: UpdateStatus{State: "idle"}}
)

// UpdateService exposes the fixed, release-based panel update flow. It does
// not execute install.sh from an HTTP request: the release asset is downloaded
// and validated, then the installed binary is atomically replaced.
type UpdateService struct{}

type UpdateStatus struct {
	State   string `json:"state"`
	Version string `json:"version"`
	Message string `json:"message"`
	Started int64  `json:"started"`
}

type VersionInfo struct {
	Current         string       `json:"current"`
	Latest          string       `json:"latest"`
	UpdateAvailable bool         `json:"update_available"`
	AssetAvailable  bool         `json:"asset_available"`
	Asset           string       `json:"asset"`
	ReleaseURL      string       `json:"release_url"`
	PublishedAt     string       `json:"published_at"`
	CanUpdate       bool         `json:"can_update"`
	Capability      string       `json:"capability"`
	Update          UpdateStatus `json:"update"`
}

type githubRelease struct {
	TagName     string         `json:"tag_name"`
	HTMLURL     string         `json:"html_url"`
	PublishedAt string         `json:"published_at"`
	Draft       bool           `json:"draft"`
	Prerelease  bool           `json:"prerelease"`
	Assets      []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest"`
}

func (s *UpdateService) CheckVersion() (VersionInfo, error) {
	release, err := fetchLatestPanelRelease()
	if err != nil {
		return VersionInfo{Current: config.GetVersion(), Update: snapshotUpdateStatus()}, err
	}
	return buildVersionInfo(release), nil
}

func (s *UpdateService) StartUpdate() (UpdateStatus, error) {
	release, err := fetchLatestPanelRelease()
	if err != nil {
		return snapshotUpdateStatus(), err
	}
	info := buildVersionInfo(release)
	if !info.UpdateAvailable {
		return snapshotUpdateStatus(), fmt.Errorf("panel is already up to date")
	}
	if !info.AssetAvailable {
		return snapshotUpdateStatus(), fmt.Errorf("no Linux release asset for %s", runtime.GOARCH)
	}
	if !info.CanUpdate {
		return snapshotUpdateStatus(), fmt.Errorf("online update unavailable: %s", info.Capability)
	}

	panelUpdateState.Lock()
	defer panelUpdateState.Unlock()
	if isUpdateRunning(panelUpdateState.value.State) {
		return panelUpdateState.value, fmt.Errorf("panel update is already running")
	}
	panelUpdateState.value = UpdateStatus{
		State:   "downloading",
		Version: normalizeVersion(release.TagName),
		Message: "downloading release asset",
		Started: time.Now().Unix(),
	}
	status := panelUpdateState.value
	go runPanelUpdate(release, linuxReleaseAsset(release))
	return status, nil
}

func buildVersionInfo(release githubRelease) VersionInfo {
	asset := linuxReleaseAsset(release)
	current := normalizeVersion(config.GetVersion())
	latest := normalizeVersion(release.TagName)
	canUpdate, capability := updateCapability()
	assetAvailable := asset.Name != "" && asset.BrowserDownloadURL != "" && asset.Size <= maxReleaseSize
	if !assetAvailable && capability == "ready" {
		capability = "no compatible Linux Release asset was found"
	}
	status := snapshotUpdateStatus()
	return VersionInfo{
		Current:         current,
		Latest:          latest,
		UpdateAvailable: compareVersions(current, latest) < 0,
		AssetAvailable:  assetAvailable,
		Asset:           asset.Name,
		ReleaseURL:      release.HTMLURL,
		PublishedAt:     release.PublishedAt,
		CanUpdate:       canUpdate && assetAvailable,
		Capability:      capability,
		Update:          status,
	}
}

func fetchLatestPanelRelease() (githubRelease, error) {
	req, err := http.NewRequest(http.MethodGet, panelReleaseAPIURL, nil)
	if err != nil {
		return githubRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "1s-ui-version-check")
	resp, err := panelHTTPClient.Do(req)
	if err != nil {
		return githubRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return githubRelease{}, fmt.Errorf("GitHub release API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var release githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&release); err != nil {
		return githubRelease{}, err
	}
	if release.Draft || release.Prerelease || !validVersion(release.TagName) {
		return githubRelease{}, fmt.Errorf("latest GitHub release is not a stable semantic version")
	}
	return release, nil
}

func linuxReleaseAsset(release githubRelease) releaseAsset {
	want := linuxReleaseAssetName()
	for _, asset := range release.Assets {
		if asset.Name == want {
			return asset
		}
	}
	return releaseAsset{}
}

func linuxReleaseAssetName() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	switch runtime.GOARCH {
	case "amd64":
		return "s-ui-linux-amd64.tar.gz"
	case "386":
		return "s-ui-linux-386.tar.gz"
	case "arm64":
		return "s-ui-linux-arm64.tar.gz"
	case "arm":
		switch strings.TrimSpace(os.Getenv("GOARM")) {
		case "5":
			return "s-ui-linux-armv5.tar.gz"
		case "6":
			return "s-ui-linux-armv6.tar.gz"
		default:
			return "s-ui-linux-armv7.tar.gz"
		}
	case "s390x":
		return "s-ui-linux-s390x.tar.gz"
	default:
		return ""
	}
}

func updateCapability() (bool, string) {
	if runtime.GOOS != "linux" {
		return false, "online update is currently supported on Linux only"
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false, "systemd/systemctl is not available"
	}
	if !runningAsRoot() {
		return false, "panel service must run as root"
	}
	executable, err := installedPanelExecutable()
	if err != nil {
		return false, err.Error()
	}
	if _, err := os.Stat(executable); err != nil {
		return false, "installed panel binary was not found"
	}
	return true, "ready"
}

func runningAsRoot() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	out, err := exec.Command("id", "-u").Output()
	return err == nil && strings.TrimSpace(string(out)) == "0"
}

func installedPanelExecutable() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return "", err
	}
	executable = filepath.Clean(executable)
	if !filepath.IsAbs(executable) || !strings.HasPrefix(executable, "/usr/local/s-ui/") {
		return "", fmt.Errorf("panel is not running from /usr/local/s-ui")
	}
	return executable, nil
}

func runPanelUpdate(release githubRelease, asset releaseAsset) {
	archivePath, err := downloadReleaseAsset(asset)
	if err != nil {
		finishPanelUpdate("failed", err.Error())
		return
	}
	defer os.Remove(archivePath)

	setPanelUpdateState("installing", "extracting and validating the release")
	extractDir, err := os.MkdirTemp("", "s-ui-update-")
	if err != nil {
		finishPanelUpdate("failed", err.Error())
		return
	}
	defer os.RemoveAll(extractDir)
	if err := extractReleaseArchive(archivePath, extractDir); err != nil {
		finishPanelUpdate("failed", err.Error())
		return
	}
	if err := installPanelFiles(extractDir); err != nil {
		finishPanelUpdate("failed", err.Error())
		return
	}

	setPanelUpdateState("restarting", "release installed; restarting panel")
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		finishPanelUpdate("failed", fmt.Sprintf("systemd reload failed: %v", err))
		return
	}
	// The current process is managed by s-ui.service. Restart asynchronously so
	// the HTTP response can return before systemd stops this process.
	time.Sleep(750 * time.Millisecond)
	if err := exec.Command("systemctl", "restart", "s-ui").Run(); err != nil {
		finishPanelUpdate("failed", fmt.Sprintf("panel restart failed: %v", err))
		return
	}
	finishPanelUpdate("success", fmt.Sprintf("updated to %s", normalizeVersion(release.TagName)))
}

func downloadReleaseAsset(asset releaseAsset) (string, error) {
	if asset.BrowserDownloadURL == "" || asset.Size > maxReleaseSize {
		return "", fmt.Errorf("release asset is missing or too large")
	}
	req, err := http.NewRequest(http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "1s-ui-updater")
	resp, err := panelHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("release download returned %s", resp.Status)
	}
	if resp.ContentLength > maxReleaseSize {
		return "", fmt.Errorf("release asset exceeds %d MiB", maxReleaseSize>>20)
	}
	file, err := os.CreateTemp("", "s-ui-release-*.tar.gz")
	if err != nil {
		return "", err
	}
	name := file.Name()
	defer func() {
		_ = file.Close()
		if err != nil {
			_ = os.Remove(name)
		}
	}()
	written, err := io.Copy(file, io.LimitReader(resp.Body, maxReleaseSize+1))
	if err != nil {
		return "", err
	}
	if written > maxReleaseSize {
		return "", fmt.Errorf("release asset exceeds %d MiB", maxReleaseSize>>20)
	}
	if err = file.Close(); err != nil {
		return "", err
	}
	return name, nil
}

func extractReleaseArchive(archivePath, destination string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	reader := tar.NewReader(gzipReader)
	foundPanel := false
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := path.Clean(strings.TrimPrefix(header.Name, "./"))
		targetName := ""
		switch name {
		case "s-ui/sui", "sui":
			targetName = "sui"
			foundPanel = true
		case "s-ui/s-ui.service", "s-ui.service":
			targetName = "s-ui.service"
		case "s-ui/s-ui.sh", "s-ui.sh":
			targetName = "s-ui.sh"
		case "s-ui/sui-agent", "sui-agent":
			targetName = "sui-agent"
		case "s-ui/s-ui-agent.service", "s-ui-agent.service":
			targetName = "s-ui-agent.service"
		case "s-ui/install-agent.sh", "install-agent.sh":
			targetName = "install-agent.sh"
		default:
			continue
		}
		if header.Typeflag != tar.TypeReg || header.Size < 0 || header.Size > maxMemberSize {
			return fmt.Errorf("invalid release member %s", name)
		}
		target := filepath.Join(destination, targetName)
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, io.LimitReader(reader, maxMemberSize+1))
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if header.Size > maxMemberSize {
			return fmt.Errorf("release member %s is too large", name)
		}
		if targetName == "sui" {
			if err := os.Chmod(target, 0755); err != nil {
				return err
			}
		}
	}
	if !foundPanel {
		return fmt.Errorf("release archive does not contain the panel binary")
	}
	return nil
}

func installPanelFiles(sourceDir string) error {
	targetExecutable, err := installedPanelExecutable()
	if err != nil {
		return err
	}
	newExecutable := filepath.Join(sourceDir, "sui")
	if err := validatePanelBinary(newExecutable); err != nil {
		return err
	}
	if err := replaceFileAtomic(newExecutable, targetExecutable, 0755); err != nil {
		return err
	}

	copyIfPresent := func(source, target string, mode os.FileMode) error {
		if _, err := os.Stat(source); err != nil {
			return nil
		}
		return replaceFileAtomic(source, target, mode)
	}
	if err := copyIfPresent(filepath.Join(sourceDir, "s-ui.sh"), "/usr/local/s-ui/s-ui.sh", 0755); err != nil {
		return err
	}
	if err := copyIfPresent(filepath.Join(sourceDir, "s-ui.sh"), "/usr/bin/s-ui", 0755); err != nil {
		return err
	}
	if err := copyIfPresent(filepath.Join(sourceDir, "s-ui.service"), "/etc/systemd/system/s-ui.service", 0644); err != nil {
		return err
	}
	if _, err := os.Stat("/usr/local/s-ui/sui-agent"); err == nil {
		if err := copyIfPresent(filepath.Join(sourceDir, "sui-agent"), "/usr/local/s-ui/sui-agent", 0755); err != nil {
			return err
		}
	}
	if _, err := os.Stat("/etc/systemd/system/s-ui-agent.service"); err == nil {
		if err := copyIfPresent(filepath.Join(sourceDir, "s-ui-agent.service"), "/etc/systemd/system/s-ui-agent.service", 0644); err != nil {
			return err
		}
	}
	if _, err := os.Stat("/usr/local/s-ui/install-agent.sh"); err == nil {
		if err := copyIfPresent(filepath.Join(sourceDir, "install-agent.sh"), "/usr/local/s-ui/install-agent.sh", 0755); err != nil {
			return err
		}
	}
	return nil
}

func validatePanelBinary(binaryPath string) error {
	info, err := os.Stat(binaryPath)
	if err != nil {
		return err
	}
	if info.Size() < 1<<20 {
		return fmt.Errorf("panel binary is unexpectedly small")
	}
	if err := os.Chmod(binaryPath, 0755); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, "-v")
	cmd.Env = append(os.Environ(), "SUI_SKIP_CORE=true", "SUI_DISABLE_XRAY=true")
	if output, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("panel binary validation failed: %v: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func replaceFileAtomic(source, destination string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".s-ui-update-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := copyFileContents(source, tmp); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, destination)
}

func copyFileContents(source string, destination io.Writer) (int64, error) {
	input, err := os.Open(source)
	if err != nil {
		return 0, err
	}
	defer input.Close()
	return io.Copy(destination, input)
}

func setPanelUpdateState(state, message string) {
	panelUpdateState.Lock()
	defer panelUpdateState.Unlock()
	panelUpdateState.value.State = state
	panelUpdateState.value.Message = message
}

func finishPanelUpdate(state, message string) {
	setPanelUpdateState(state, message)
}

func snapshotUpdateStatus() UpdateStatus {
	panelUpdateState.Lock()
	defer panelUpdateState.Unlock()
	return panelUpdateState.value
}

func isUpdateRunning(state string) bool {
	switch state {
	case "downloading", "installing", "restarting":
		return true
	default:
		return false
	}
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	if index := strings.IndexAny(value, "+-"); index >= 0 {
		value = value[:index]
	}
	return value
}

func validVersion(value string) bool {
	parts := strings.Split(normalizeVersion(value), ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return true
}

func compareVersions(left, right string) int {
	leftParts := versionParts(left)
	rightParts := versionParts(right)
	for i := 0; i < 3; i++ {
		if leftParts[i] < rightParts[i] {
			return -1
		}
		if leftParts[i] > rightParts[i] {
			return 1
		}
	}
	return 0
}

func versionParts(value string) [3]int {
	var result [3]int
	parts := strings.Split(normalizeVersion(value), ".")
	for i := 0; i < len(parts) && i < len(result); i++ {
		result[i], _ = strconv.Atoi(parts[i])
	}
	return result
}
