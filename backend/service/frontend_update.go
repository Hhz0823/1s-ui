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
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Hhz0823/1s-ui/config"
	"github.com/Hhz0823/1s-ui/logger"
)

// The web UI is a release asset of its own (s-ui-frontend.tar.gz) that
// install.sh unpacks into /usr/local/s-ui/frontend, where nginx serves it.
// The panel updater installs it together with the new binary, and a panel
// whose UI does not match its own version (for example after an update by a
// version whose updater left the UI alone) installs the matching UI itself.

const (
	frontendAssetName  = "s-ui-frontend.tar.gz"
	maxFrontendArchive = 64 << 20
	maxFrontendBytes   = 160 << 20
	maxFrontendFiles   = 4000
)

// frontendInstallDir is where install.sh installs the UI; tests move it.
var frontendInstallDir = "/usr/local/s-ui/frontend"

var frontendRepair sync.Mutex

// InstalledFrontendVersion is the version recorded in the installed UI's
// version.json, or "" for a UI built before the file existed.
func InstalledFrontendVersion() string {
	raw, err := os.ReadFile(filepath.Join(frontendInstallDir, "version.json"))
	if err != nil {
		return ""
	}
	var info struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(raw, &info) != nil {
		return ""
	}
	return releaseVersion(info.Version)
}

func frontendInstalled() bool {
	info, err := os.Stat(filepath.Join(frontendInstallDir, "index.html"))
	return err == nil && info.Mode().IsRegular()
}

// releaseFrontendAsset is the UI asset of a release: from the API with its
// SHA-256, otherwise its github.com address.
func releaseFrontendAsset(release githubRelease) releaseAsset {
	for _, asset := range release.Assets {
		if asset.Name == frontendAssetName {
			return asset
		}
	}
	return releaseAsset{Name: frontendAssetName, BrowserDownloadURL: githubAssetURL(panelRepo, release.TagName, frontendAssetName)}
}

func downloadFrontendAsset(asset releaseAsset, status func(source string)) (string, error) {
	if asset.Size > maxFrontendArchive || !isGitHubAssetURL(asset.BrowserDownloadURL, panelRepo) {
		return "", fmt.Errorf("web UI asset is missing or too large")
	}
	return downloadGitHubAsset(panelHTTPClient, asset.BrowserDownloadURL, asset.Digest, maxFrontendArchive, "s-ui-frontend-*.tar.gz", status)
}

// stageFrontend unpacks a UI archive next to the installed UI. Only regular
// files and directories inside the archive root are accepted.
func stageFrontend(archivePath string) (string, error) {
	stage := frontendInstallDir + ".new"
	if err := os.RemoveAll(stage); err != nil {
		return "", err
	}
	if err := os.MkdirAll(stage, 0755); err != nil {
		return "", err
	}
	if err := unpackFrontend(archivePath, stage); err != nil {
		_ = os.RemoveAll(stage)
		return "", err
	}
	if info, err := os.Stat(filepath.Join(stage, "index.html")); err != nil || !info.Mode().IsRegular() {
		_ = os.RemoveAll(stage)
		return "", fmt.Errorf("web UI archive has no index.html")
	}
	return stage, nil
}

func unpackFrontend(archivePath, destination string) error {
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
	var files int
	var total int64
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		for _, segment := range strings.Split(header.Name, "/") {
			if segment == ".." {
				return fmt.Errorf("unsafe web UI path %q", header.Name)
			}
		}
		name := path.Clean("/" + header.Name)
		if name == "/" {
			continue
		}
		target := filepath.Join(destination, filepath.FromSlash(name))
		switch header.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			files++
			total += header.Size
			if files > maxFrontendFiles || header.Size < 0 || total > maxFrontendBytes {
				return fmt.Errorf("web UI archive is too large")
			}
			if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			written, copyErr := io.Copy(out, io.LimitReader(reader, header.Size))
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			if written != header.Size {
				return fmt.Errorf("web UI archive is truncated")
			}
		default:
			return fmt.Errorf("unsupported web UI entry %q", header.Name)
		}
	}
}

// swapFrontend puts a staged UI in place of the installed one.
func swapFrontend(stage string) error {
	previous := frontendInstallDir + ".previous"
	if err := os.RemoveAll(previous); err != nil {
		return err
	}
	if err := os.Rename(frontendInstallDir, previous); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(stage, frontendInstallDir); err != nil {
		_ = os.Rename(previous, frontendInstallDir)
		return err
	}
	_ = os.RemoveAll(previous)
	return nil
}

// fetchPanelReleaseByTag reads one release from the API for its SHA-256
// digests, or names only its github.com assets when the API is unreachable.
func fetchPanelReleaseByTag(tag string) githubRelease {
	release := githubRelease{TagName: tag}
	ctx, cancel := context.WithTimeout(context.Background(), githubLookupTimeout)
	defer cancel()
	apiURL := strings.TrimSuffix(panelReleaseAPIURL, "/latest") + "/tags/" + tag
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return release
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "1s-ui-version-check")
	response, err := panelHTTPClient.Do(request)
	if err != nil {
		return release
	}
	defer response.Body.Close()
	var found githubRelease
	if response.StatusCode == http.StatusOK && json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&found) == nil && found.TagName == tag {
		return found
	}
	return release
}

// RepairFrontend installs the UI of this panel's own release when the
// installed UI records another version (or none). It only acts on panels
// installed by install.sh, whose online update is available.
func (s *UpdateService) RepairFrontend() (bool, error) {
	if ok, _ := updateCapability(); !ok {
		return false, nil
	}
	return installFrontendRelease(releaseVersion(config.GetVersion()))
}

// installFrontendRelease installs the UI of release version want unless the
// installed UI already is that version.
func installFrontendRelease(want string) (bool, error) {
	if !frontendRepair.TryLock() {
		return false, nil
	}
	defer frontendRepair.Unlock()
	if !frontendInstalled() || !validVersion(want) || InstalledFrontendVersion() == want {
		return false, nil
	}
	if isUpdateRunning(snapshotUpdateStatus().State) {
		return false, nil
	}
	release := fetchPanelReleaseByTag("v" + want)
	archive, err := downloadFrontendAsset(releaseFrontendAsset(release), nil)
	if err != nil {
		return false, err
	}
	defer os.Remove(archive)
	stage, err := stageFrontend(archive)
	if err != nil {
		return false, err
	}
	if err = swapFrontend(stage); err != nil {
		_ = os.RemoveAll(stage)
		return false, err
	}
	logger.Infof("installed the web UI of v%s", want)
	return true, nil
}
