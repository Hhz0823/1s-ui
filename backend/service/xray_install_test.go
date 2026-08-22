//go:build !openwrt_lite

package service

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
)

func TestLowResourceHostCanChooseXrayInstall(t *testing.T) {
	status := baseXrayInstallStatus(true, "ready", 1, 512<<20, XrayInstallProgress{State: "idle"})
	if !status.LowResource {
		t.Fatal("512 MiB host was not identified as low resource")
	}
	if !status.CanInstall {
		t.Fatal("low-resource host must still be allowed to choose Xray-core installation")
	}
	if status.OnDemand || status.Running {
		t.Fatal("installation choice must not imply that Xray starts automatically")
	}
}

func TestLowResourceCoreModeIsExclusive(t *testing.T) {
	if !isLowResourceCoreMemory(512 << 20) {
		t.Fatal("512 MiB host must use exclusive core mode")
	}
	if isLowResourceCoreMemory(2 << 30) {
		t.Fatal("2 GiB host must not be forced into exclusive core mode")
	}
	if !shouldSwitchToXrayExclusively(true, true) {
		t.Fatal("running sing-box was not selected for shutdown before Xray")
	}
	if shouldSwitchToXrayExclusively(false, true) || shouldSwitchToXrayExclusively(true, false) {
		t.Fatal("exclusive switch was selected outside the low-resource dual-running case")
	}
	if !xrayOwnsExclusiveRuntime(true, true) || xrayOwnsExclusiveRuntime(false, true) {
		t.Fatal("exclusive Xray runtime ownership was computed incorrectly")
	}
	if !shouldRestoreSingBoxAfterXrayStop(true, true, false, false) {
		t.Fatal("stopping Xray did not select sing-box restoration")
	}
	if shouldRestoreSingBoxAfterXrayStop(true, true, false, true) ||
		shouldRestoreSingBoxAfterXrayStop(true, false, false, false) ||
		shouldRestoreSingBoxAfterXrayStop(false, true, false, false) {
		t.Fatal("sing-box restoration was selected outside a live low-resource switch")
	}
	if !shouldReloadRunningXray(true, true) || shouldReloadRunningXray(true, false) || shouldReloadRunningXray(false, true) {
		t.Fatal("running Xray reload decision is incorrect")
	}
}

func TestEffectiveMemoryTotalHonorsCgroup(t *testing.T) {
	if got := effectiveMemoryTotal(8<<30, 512<<20); got != 512<<20 {
		t.Fatalf("cgroup memory was ignored: got %d", got)
	}
	if got := effectiveMemoryTotal(2<<30, 1<<63); got != 2<<30 {
		t.Fatalf("unlimited cgroup value changed host memory: got %d", got)
	}
}

func TestUninstallXrayRejectsConfiguredInbounds(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "xray-uninstall.db")); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Create(&model.Inbound{Type: "vless", Tag: "keep-me", CoreType: model.CoreTypeXray}).Error; err != nil {
		t.Fatal(err)
	}
	service := XrayInstallService{}
	if _, err := service.Uninstall(); err == nil || !strings.Contains(err.Error(), "configured inbounds") {
		t.Fatalf("uninstall did not protect Xray inbounds: %v", err)
	}
}

func TestXrayLifecycleRejectsChangesDuringInstall(t *testing.T) {
	xrayInstallState.Lock()
	previous := xrayInstallState.value
	xrayInstallState.value = XrayInstallProgress{State: "installing"}
	xrayInstallState.Unlock()
	t.Cleanup(func() {
		xrayInstallState.Lock()
		xrayInstallState.value = previous
		xrayInstallState.Unlock()
	})

	service := XrayInstallService{}
	if _, err := service.SetEnabled(false); err == nil || !strings.Contains(err.Error(), "in progress") {
		t.Fatalf("runtime state changed during installation: %v", err)
	}
	if _, err := service.Uninstall(); err == nil || !strings.Contains(err.Error(), "in progress") {
		t.Fatalf("uninstall was accepted during installation: %v", err)
	}
}

func TestXrayReleaseAssetNames(t *testing.T) {
	tests := map[string]string{
		"amd64/":  "Xray-linux-64.zip",
		"386/":    "Xray-linux-32.zip",
		"arm64/":  "Xray-linux-arm64-v8a.zip",
		"arm/5":   "Xray-linux-arm32-v5.zip",
		"arm/6":   "Xray-linux-arm32-v6.zip",
		"arm/7":   "Xray-linux-arm32-v7a.zip",
		"s390x/":  "Xray-linux-s390x.zip",
		"mips64/": "",
	}
	for input, expected := range tests {
		var arch, arm string
		for index := range input {
			if input[index] == '/' {
				arch, arm = input[:index], input[index+1:]
				break
			}
		}
		if actual := xrayReleaseAssetName(arch, arm); actual != expected {
			t.Fatalf("asset for %s: got %q, want %q", input, actual, expected)
		}
	}
}

func TestExtractXrayArchive(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "xray.zip")
	writeTestXrayZip(t, archive, map[string]string{
		"xray":        "#!/bin/sh\necho 'Xray 26.7.11 test'\n",
		"geoip.dat":   "geoip",
		"geosite.dat": "geosite",
		"README.md":   "ignored",
	})
	destination := t.TempDir()
	if err := extractXrayArchive(archive, destination); err != nil {
		t.Fatal(err)
	}
	if version, err := xrayBinaryVersion(filepath.Join(destination, "xray")); err != nil || version != "Xray 26.7.11 test" {
		t.Fatalf("unexpected extracted binary: version=%q err=%v", version, err)
	}
	if _, err := os.Stat(filepath.Join(destination, "README.md")); !os.IsNotExist(err) {
		t.Fatal("unneeded archive files must not be extracted")
	}
}

func TestExtractXrayArchiveRejectsTraversal(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "unsafe.zip")
	writeTestXrayZip(t, archive, map[string]string{"../xray": "unsafe"})
	if err := extractXrayArchive(archive, t.TempDir()); err == nil {
		t.Fatal("path traversal archive was accepted")
	}
}

func TestVerifyReleaseDigest(t *testing.T) {
	actual := sha256.Sum256([]byte("xray"))
	if err := verifyReleaseDigest("sha256:"+fmt.Sprintf("%x", actual[:]), actual[:]); err != nil {
		t.Fatal(err)
	}
	if err := verifyReleaseDigest("sha256:"+fmt.Sprintf("%064x", 1), actual[:]); err == nil {
		t.Fatal("checksum mismatch was accepted")
	}
}

func writeTestXrayZip(t *testing.T, target string, files map[string]string) {
	t.Helper()
	file, err := os.Create(target)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, content := range files {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0755)
		member, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := member.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
