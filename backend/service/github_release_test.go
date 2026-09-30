package service

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
)

// fakeGitHub stands in for github.com and for a mirror at /mirror/ that takes
// the github.com URL after its prefix, as ghfast.top does. With githubDown
// set, github.com itself answers 503 like an unreachable GitHub.
type fakeGitHub struct {
	server     *httptest.Server
	githubDown bool
	githubSlow bool
	latest     string
	files      map[string][]byte
	mirrored   map[string][]byte
	requests   []string
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	t.Helper()
	fake := &fakeGitHub{files: map[string][]byte{}, mirrored: map[string][]byte{}}
	fake.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path := request.URL.Path
		fake.requests = append(fake.requests, path)
		mirrored := strings.HasPrefix(path, "/mirror/") || strings.HasPrefix(path, "/mirror2/")
		if mirrored {
			path = strings.TrimPrefix(path[strings.Index(path[1:], "/")+2:], fake.server.URL)
		} else if fake.githubDown {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		} else if fake.githubSlow {
			// A trickle, like github.com from some networks in China.
			for request.Context().Err() == nil {
				_, _ = writer.Write([]byte("x"))
				writer.(http.Flusher).Flush()
				time.Sleep(20 * time.Millisecond)
			}
			return
		}
		if strings.HasSuffix(path, "/releases/latest") && fake.latest != "" {
			target := strings.TrimSuffix(path, "latest") + "tag/" + fake.latest
			if mirrored {
				// Mirrors that follow redirects hand back the release page.
				_, _ = writer.Write([]byte(`<meta property="og:url" content="https://github.com` + target + `">`))
				return
			}
			http.Redirect(writer, request, target, http.StatusFound)
			return
		}
		content, ok := fake.files[path]
		if mirrored {
			if tampered, found := fake.mirrored[path]; found {
				content, ok = tampered, true
			}
		}
		if !ok {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write(content)
	}))
	t.Cleanup(fake.server.Close)
	oldBase, oldMirrors := githubWebBase, builtinGitHubMirrors
	githubWebBase, builtinGitHubMirrors = fake.server.URL, []string{fake.server.URL + "/mirror/"}
	t.Cleanup(func() { githubWebBase, builtinGitHubMirrors = oldBase, oldMirrors })
	return fake
}

// closedURL is an address nothing listens on, like an unreachable API.
func closedURL(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	return "http://" + address + "/releases/latest"
}

func sha256Digest(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func TestGitHubLatestTagFromRedirectAndMirrorPage(t *testing.T) {
	fake := newFakeGitHub(t)
	fake.latest = "v99.0.0"
	for _, source := range []githubSource{{trusted: true}, {prefix: fake.server.URL + "/mirror/"}} {
		tag, err := githubLatestTag(newGitHubHTTPClient(), source, panelRepo)
		if err != nil || tag != "v99.0.0" {
			t.Fatalf("%q: tag = %q, err = %v", source.prefix, tag, err)
		}
	}
}

func TestPanelVersionCheckWorksWithoutGitHubAPI(t *testing.T) {
	fake := newFakeGitHub(t)
	fake.latest = "v99.0.0"
	oldAPI := panelReleaseAPIURL
	panelReleaseAPIURL = closedURL(t)
	t.Cleanup(func() { panelReleaseAPIURL = oldAPI })

	for _, githubDown := range []bool{false, true} {
		fake.githubDown = githubDown
		info, err := (&UpdateService{}).CheckVersion()
		if err != nil {
			t.Fatalf("github down %v: %v", githubDown, err)
		}
		if info.Latest != "99.0.0" || !info.UpdateAvailable {
			t.Fatalf("github down %v: %#v", githubDown, info)
		}
	}
	fake.latest = ""
	if _, err := (&UpdateService{}).CheckVersion(); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("an unreachable GitHub must say why, got %v", err)
	}
}

func TestGitHubDownloadsFromMirrorsOnlyWhenVerifiable(t *testing.T) {
	fake := newFakeGitHub(t)
	fake.githubDown = true
	path := "/" + panelRepo + "/releases/download/v99.0.0/s-ui-linux-amd64.tar.gz"
	archive := []byte("release archive")
	fake.files[path] = archive
	assetURL := githubWebBase + path
	download := func(digest string) error {
		name, err := downloadGitHubAsset(newGitHubHTTPClient(), assetURL, digest, 1<<20, "github-test-*", nil)
		if err == nil {
			content, _ := os.ReadFile(name)
			_ = os.Remove(name)
			if string(content) != string(archive) {
				t.Fatalf("downloaded %q", content)
			}
		}
		return err
	}

	if err := download(sha256Digest(archive)); err != nil {
		t.Fatalf("a mirror must serve an archive with a known SHA-256: %v", err)
	}
	if err := download(""); err == nil {
		t.Fatal("a built-in mirror served an archive nothing can verify")
	}
	fake.mirrored[path] = []byte("tampered archive")
	if err := download(sha256Digest(archive)); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("a tampered mirror copy was accepted: %v", err)
	}

	// The administrator's own mirror is trusted like GitHub.
	setupQuickAddTest(t)
	delete(fake.mirrored, path)
	mirror := fake.server.URL + "/mirror"
	if err := database.GetDB().Create(&model.Setting{Key: "githubMirror", Value: mirror}).Error; err != nil {
		t.Fatal(err)
	}
	builtinGitHubMirrors = nil
	if err := download(""); err != nil {
		t.Fatalf("the configured mirror was not used: %v", err)
	}
}

func TestNormalizeGitHubMirror(t *testing.T) {
	for input, want := range map[string]string{
		"":                       "",
		" https://ghfast.top ":   "https://ghfast.top/",
		"https://gh.example/gh/": "https://gh.example/gh/",
		"http://10.0.0.2:8080":   "http://10.0.0.2:8080/",
	} {
		if got, err := normalizeGitHubMirror(input); err != nil || got != want {
			t.Fatalf("%q: got %q, %v", input, got, err)
		}
	}
	for _, input := range []string{"ghfast.top", "ftp://mirror.example/", "https://user:pw@mirror.example/", "https://mirror.example/?u="} {
		if _, err := normalizeGitHubMirror(input); err == nil {
			t.Fatalf("%q was accepted", input)
		}
	}
}

func setDownloadLine(t *testing.T, line string) {
	t.Helper()
	if err := database.GetDB().Where(&model.Setting{Key: "downloadLine"}).Assign(model.Setting{Value: line}).FirstOrCreate(&model.Setting{}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestDownloadLineOrdersSources(t *testing.T) {
	setupQuickAddTest(t)
	old := builtinGitHubMirrors
	builtinGitHubMirrors = []string{"https://m1/", "https://m2/"}
	t.Cleanup(func() { builtinGitHubMirrors = old })
	describe := func() string {
		var parts []string
		for _, source := range githubSources() {
			name := source.prefix
			if name == "" {
				name = "github"
			}
			if source.trusted {
				name += "+"
			}
			parts = append(parts, name)
		}
		return strings.Join(parts, " ")
	}
	for line, want := range map[string]string{
		"auto":   "github+ https://m1/ https://m2/",
		"github": "github+",
		"cn":     "https://m1/+ https://m2/+ github+",
		"bogus":  "github+ https://m1/ https://m2/",
	} {
		setDownloadLine(t, line)
		if got := describe(); got != want {
			t.Fatalf("%s: sources %q, want %q", line, got, want)
		}
	}
	if _, err := normalizeDownloadLine("bogus"); err == nil {
		t.Fatal("an unknown download line was accepted")
	}
}

func TestMirrorsServeReleaseCheckedBySHA256SUMS(t *testing.T) {
	// The default line, whatever an earlier test left in the shared database:
	// the "cn" line trusts a single built-in mirror.
	setupQuickAddTest(t)
	setDownloadLine(t, "auto")
	fake := newFakeGitHub(t)
	fake.githubDown = true
	builtinGitHubMirrors = []string{fake.server.URL + "/mirror/", fake.server.URL + "/mirror2/"}
	dir := "/" + panelRepo + "/releases/download/v99.0.0/"
	archive := []byte("release archive")
	fake.files[dir+"s-ui-linux-amd64.tar.gz"] = archive
	sum := strings.TrimPrefix(sha256Digest(archive), "sha256:")
	fake.files[dir+"SHA256SUMS"] = []byte(sum + "  s-ui-linux-amd64.tar.gz\n")
	download := func() error {
		name, err := downloadGitHubAsset(newGitHubHTTPClient(), githubWebBase+dir+"s-ui-linux-amd64.tar.gz", "", 1<<20, "github-test-*", nil)
		if err == nil {
			_ = os.Remove(name)
		}
		return err
	}
	if err := download(); err != nil {
		t.Fatalf("two mirrors agreeing on SHA256SUMS must be enough: %v", err)
	}
	// One mirror alone cannot vouch for what it serves.
	builtinGitHubMirrors = builtinGitHubMirrors[:1]
	if err := download(); err == nil {
		t.Fatal("a checksum from a single mirror was trusted")
	}
}

func TestSlowDownloadMovesToNextSource(t *testing.T) {
	fake := newFakeGitHub(t)
	fake.githubSlow = true
	oldGrace := slowDownloadGrace
	slowDownloadGrace = 200 * time.Millisecond
	t.Cleanup(func() { slowDownloadGrace = oldGrace })
	path := "/" + panelRepo + "/releases/download/v99.0.0/s-ui-linux-amd64.tar.gz"
	archive := []byte("release archive")
	fake.files[path] = archive
	started := time.Now()
	name, err := downloadGitHubAsset(newGitHubHTTPClient(), githubWebBase+path, sha256Digest(archive), 1<<20, "github-test-*", nil)
	if err != nil {
		t.Fatalf("the mirror did not take over from a crawling GitHub: %v", err)
	}
	_ = os.Remove(name)
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("switching took %v", elapsed)
	}
}

func TestChecksumFor(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	sums := sum + "  s-ui-frontend.tar.gz\n" + strings.Repeat("cd", 32) + " *s-ui-linux-arm64.tar.gz\n"
	if got := checksumFor(sums, "s-ui-frontend.tar.gz"); got != "sha256:"+sum {
		t.Fatalf("got %q", got)
	}
	if got := checksumFor(sums, "s-ui-linux-arm64.tar.gz"); got != "sha256:"+strings.Repeat("cd", 32) {
		t.Fatalf("binary-mode line: got %q", got)
	}
	if got := checksumFor(sums, "missing.tar.gz"); got != "" {
		t.Fatalf("missing asset: got %q", got)
	}
}
