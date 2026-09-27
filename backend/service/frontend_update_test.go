package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hhz0823/1s-ui/logger"
	"github.com/op/go-logging"
)

type tarEntry struct {
	name, body string
	kind       byte
}

func frontendArchive(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(gz)
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Typeflag: entry.kind, Mode: 0644, Size: int64(len(entry.body))}
		if entry.kind == tar.TypeDir || entry.kind == tar.TypeSymlink {
			header.Size = 0
			header.Mode = 0755
			header.Linkname = entry.body
		}
		if err := archive.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if entry.kind == tar.TypeReg {
			if _, err := archive.Write([]byte(entry.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// setupFrontendRoot installs an old UI, built before version.json existed.
func setupFrontendRoot(t *testing.T) string {
	t.Helper()
	logger.InitLogger(logging.CRITICAL)
	root := filepath.Join(t.TempDir(), "frontend")
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"index.html": "old ui", "assets/old.js": "old"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	old := frontendInstallDir
	frontendInstallDir = root
	t.Cleanup(func() { frontendInstallDir = old })
	return root
}

func TestFrontendInstallsTheUIOfThePanelRelease(t *testing.T) {
	fake := newFakeGitHub(t)
	root := setupFrontendRoot(t)
	archive := frontendArchive(t, []tarEntry{
		{name: "./", kind: tar.TypeDir},
		{name: "./index.html", body: "new ui", kind: tar.TypeReg},
		{name: "./version.json", body: `{"version":"1.7.0"}`, kind: tar.TypeReg},
		{name: "./assets/", kind: tar.TypeDir},
		{name: "./assets/app.js", body: "new", kind: tar.TypeReg},
	})
	assetPath := "/" + panelRepo + "/releases/download/v1.7.0/" + frontendAssetName
	fake.files[assetPath] = archive
	release, _ := json.Marshal(githubRelease{TagName: "v1.7.0", Assets: []releaseAsset{{
		Name: frontendAssetName, BrowserDownloadURL: githubWebBase + assetPath, Digest: sha256Digest(archive),
	}}})
	fake.files["/repos/"+panelRepo+"/releases/tags/v1.7.0"] = release
	oldAPI := panelReleaseAPIURL
	panelReleaseAPIURL = fake.server.URL + "/repos/" + panelRepo + "/releases/latest"
	t.Cleanup(func() { panelReleaseAPIURL = oldAPI })

	if InstalledFrontendVersion() != "" {
		t.Fatal("an old UI has no version")
	}
	installed, err := installFrontendRelease("1.7.0")
	if err != nil || !installed {
		t.Fatalf("installed %v, %v", installed, err)
	}
	if got := InstalledFrontendVersion(); got != "1.7.0" {
		t.Fatalf("installed UI version %q", got)
	}
	if body, _ := os.ReadFile(filepath.Join(root, "index.html")); string(body) != "new ui" {
		t.Fatalf("index.html = %q", body)
	}
	if _, err = os.Stat(filepath.Join(root, "assets", "old.js")); !os.IsNotExist(err) {
		t.Fatal("files of the old UI were kept")
	}
	for _, name := range []string{root, filepath.Join(root, "assets")} {
		if info, _ := os.Stat(name); info.Mode().Perm() != 0755 {
			t.Fatalf("%s mode %v: nginx must be able to read the UI", name, info.Mode())
		}
	}
	if info, _ := os.Stat(filepath.Join(root, "assets", "app.js")); info.Mode().Perm() != 0644 {
		t.Fatalf("file mode %v", info.Mode())
	}
	for _, leftover := range []string{root + ".new", root + ".previous"} {
		if _, err = os.Stat(leftover); !os.IsNotExist(err) {
			t.Fatalf("%s was left behind", leftover)
		}
	}
	if again, err := installFrontendRelease("1.7.0"); again || err != nil {
		t.Fatalf("a matching UI was installed again: %v, %v", again, err)
	}

	// Without the API the UI still comes from github.com itself.
	panelReleaseAPIURL = closedURL(t)
	archive2 := frontendArchive(t, []tarEntry{
		{name: "index.html", body: "newer ui", kind: tar.TypeReg},
		{name: "version.json", body: `{"version":"1.7.1"}`, kind: tar.TypeReg},
	})
	fake.files["/"+panelRepo+"/releases/download/v1.7.1/"+frontendAssetName] = archive2
	if installed, err = installFrontendRelease("1.7.1"); err != nil || !installed || InstalledFrontendVersion() != "1.7.1" {
		t.Fatalf("installed %v, %v, version %q", installed, err, InstalledFrontendVersion())
	}
}

func TestFrontendArchiveMustStayInsideTheUI(t *testing.T) {
	setupFrontendRoot(t)
	for name, entries := range map[string][]tarEntry{
		"parent path": {{name: "../escape.js", body: "x", kind: tar.TypeReg}, {name: "index.html", body: "ui", kind: tar.TypeReg}},
		"symlink":     {{name: "index.html", body: "/etc/passwd", kind: tar.TypeSymlink}},
		"no index":    {{name: "assets/app.js", body: "x", kind: tar.TypeReg}},
	} {
		path := filepath.Join(t.TempDir(), "ui.tar.gz")
		if err := os.WriteFile(path, frontendArchive(t, entries), 0600); err != nil {
			t.Fatal(err)
		}
		if stage, err := stageFrontend(path); err == nil {
			t.Fatalf("%s: staged %s", name, stage)
		}
		if _, err := os.Stat(frontendInstallDir + ".new"); !os.IsNotExist(err) {
			t.Fatalf("%s: a rejected archive left a staging directory", name)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(frontendInstallDir), "escape.js")); !os.IsNotExist(err) {
			t.Fatalf("%s: wrote outside the UI", name)
		}
	}
	if body, _ := os.ReadFile(filepath.Join(frontendInstallDir, "index.html")); !strings.Contains(string(body), "old ui") {
		t.Fatal("a rejected archive touched the installed UI")
	}
}
