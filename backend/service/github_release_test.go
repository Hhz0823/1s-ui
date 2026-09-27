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

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
)

// fakeGitHub stands in for github.com and for a mirror at /mirror/ that takes
// the github.com URL after its prefix, as ghfast.top does. With githubDown
// set, github.com itself answers 503 like an unreachable GitHub.
type fakeGitHub struct {
	server     *httptest.Server
	githubDown bool
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
		mirrored := strings.HasPrefix(path, "/mirror/")
		if mirrored {
			path = strings.TrimPrefix(strings.TrimPrefix(path, "/mirror/"), fake.server.URL)
		} else if fake.githubDown {
			writer.WriteHeader(http.StatusServiceUnavailable)
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
	fake.latest = "v1.7.0"
	for _, source := range []githubSource{{trusted: true}, {prefix: fake.server.URL + "/mirror/"}} {
		tag, err := githubLatestTag(newGitHubHTTPClient(), source, panelRepo)
		if err != nil || tag != "v1.7.0" {
			t.Fatalf("%q: tag = %q, err = %v", source.prefix, tag, err)
		}
	}
}

func TestPanelVersionCheckWorksWithoutGitHubAPI(t *testing.T) {
	fake := newFakeGitHub(t)
	fake.latest = "v1.7.0"
	oldAPI := panelReleaseAPIURL
	panelReleaseAPIURL = closedURL(t)
	t.Cleanup(func() { panelReleaseAPIURL = oldAPI })

	for _, githubDown := range []bool{false, true} {
		fake.githubDown = githubDown
		info, err := (&UpdateService{}).CheckVersion()
		if err != nil {
			t.Fatalf("github down %v: %v", githubDown, err)
		}
		if info.Latest != "1.7.0" || !info.UpdateAvailable {
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
	path := "/" + panelRepo + "/releases/download/v1.7.0/s-ui-linux-amd64.tar.gz"
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
