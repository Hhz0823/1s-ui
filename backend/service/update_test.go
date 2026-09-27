package service

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		left  string
		right string
		want  int
	}{
		{left: "1.5.9", right: "1.5.9", want: 0},
		{left: "v1.5.9", right: "1.5.10", want: -1},
		{left: "1.6.0", right: "1.5.99", want: 1},
		{left: "1.6.0-boost", right: "1.6.0", want: 0},
	}
	for _, test := range tests {
		if got := compareVersions(test.left, test.right); got != test.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}

func TestVersionCheckReadsStableRelease(t *testing.T) {
	server := httptest.NewServer(httpHandlerFunc(func() string {
		return `{"tag_name":"v1.6.3","html_url":"https://github.com/Hhz0823/1s-ui/releases/tag/v1.6.3","published_at":"2026-08-08T00:00:00Z","assets":[{"name":"s-ui-linux-amd64.tar.gz","browser_download_url":"https://example.test/s-ui-linux-amd64.tar.gz","size":123}]}`
	}))
	defer server.Close()
	oldURL, oldClient := panelReleaseAPIURL, panelHTTPClient
	panelReleaseAPIURL, panelHTTPClient = server.URL, server.Client()
	t.Cleanup(func() { panelReleaseAPIURL, panelHTTPClient = oldURL, oldClient })

	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	info, err := (&UpdateService{}).CheckVersion()
	if err != nil {
		t.Fatal(err)
	}
	if info.Current != "1.6.2" || info.Latest != "1.6.3" {
		t.Fatalf("unexpected versions: %#v", info)
	}
	if !info.UpdateAvailable {
		t.Fatalf("release was not detected as updateable: %#v", info)
	}
	if runtime.GOOS == "linux" && !info.AssetAvailable {
		t.Fatalf("Linux release asset was not detected: %#v", info)
	}
}

// httpHandlerFunc keeps this test independent of the production router.
type httpHandlerFunc func() string

func (handler httpHandlerFunc) ServeHTTP(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(handler()))
}
