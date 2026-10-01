package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Hhz0823/1s-ui/database"
)

// Release lookups and downloads from GitHub. Servers that cannot reach
// api.github.com, common in mainland China, still update the panel and install
// Xray-core:
//
//  1. api.github.com gives the latest stable tag, the assets and their SHA-256.
//  2. github.com tells the tag through the /releases/latest redirect and serves
//     the assets under /releases/download/<tag>/, TLS-authenticated like the
//     API.
//  3. GitHub mirrors take a github.com URL after their own prefix, as in
//     https://ghfast.top/https://github.com/...: first the one set under
//     Settings, then the built-in ones. A mirror sees and could alter what it
//     forwards, so the built-in mirrors only serve downloads whose SHA-256 is
//     known: from the API, from the SHA256SUMS file of the release (on
//     GitHub, or the same on two mirrors) or built into the panel. The
//     administrator's own mirror is trusted like GitHub, and so are the
//     built-in ones once the administrator picks the China line.
//
// The download line under Settings orders the sources: GitHub first (auto),
// GitHub and the administrator's mirror only (github), or the mirrors first
// (cn). A download that crawls is dropped for the next source.
var builtinGitHubMirrors = []string{
	"https://ghfast.top/",
	"https://gh-proxy.com/",
	"https://ghproxy.net/",
}

// githubWebBase is github.com; tests point it at a local server.
var githubWebBase = "https://github.com"

const (
	githubLookupTimeout   = 15 * time.Second
	githubDownloadTimeout = 15 * time.Minute
)

// newGitHubHTTPClient bounds connecting and waiting for a response, not the
// transfer: release archives take minutes on slow links.
func newGitHubHTTPClient() *http.Client {
	return &http.Client{Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
	}}
}

// githubSource is GitHub itself (empty prefix) or a mirror.
type githubSource struct {
	prefix  string
	trusted bool
}

func (s githubSource) url(githubURL string) string { return s.prefix + githubURL }

func (s githubSource) name() string {
	if s.prefix == "" {
		if parsed, err := url.Parse(githubWebBase); err == nil {
			return parsed.Host
		}
		return "github.com"
	}
	if parsed, err := url.Parse(s.prefix); err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return s.prefix
}

// githubSources lists GitHub, the mirror set under Settings and the built-in
// mirrors, in the order of the download line.
func githubSources() []githubSource {
	configured, line := "", downloadLineAuto
	if database.GetDB() != nil {
		settings := &SettingService{}
		configured, _ = settings.GetGitHubMirror()
		line, _ = settings.GetDownloadLine()
	}
	github := githubSource{trusted: true}
	var mirrors []githubSource
	if configured != "" {
		mirrors = append(mirrors, githubSource{prefix: configured, trusted: true})
	}
	if line != downloadLineGitHub {
		for _, prefix := range builtinGitHubMirrors {
			if prefix != configured {
				mirrors = append(mirrors, githubSource{prefix: prefix, trusted: line == downloadLineCN})
			}
		}
	}
	if line == downloadLineCN {
		return append(mirrors, github)
	}
	return append([]githubSource{github}, mirrors...)
}

func urlHost(rawURL string) string {
	if parsed, err := url.Parse(rawURL); err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return rawURL
}

// githubLatestTagFromAny asks GitHub and every mirror at once and returns the
// first tag found. A mirror can only name a version here: downloading it
// still follows downloadGitHubAsset's rules.
func githubLatestTagFromAny(client *http.Client, repo string) (string, error) {
	sources := githubSources()
	type result struct {
		source string
		tag    string
		err    error
	}
	results := make(chan result, len(sources))
	for _, source := range sources {
		go func(source githubSource) {
			tag, err := githubLatestTag(client, source, repo)
			results <- result{source: source.name(), tag: tag, err: err}
		}(source)
	}
	var attempts githubAttempts
	for range sources {
		found := <-results
		if found.err == nil {
			return found.tag, nil
		}
		attempts.add(found.source, found.err)
	}
	return "", attempts.err("release lookup")
}

// githubAttempts collects why each source failed.
type githubAttempts []string

func (a *githubAttempts) add(source string, err error) {
	*a = append(*a, source+": "+shortNetError(err))
}

func (a githubAttempts) err(what string) error {
	if len(a) == 0 {
		return fmt.Errorf("%s failed", what)
	}
	return fmt.Errorf("%s failed (%s)", what, strings.Join(a, "; "))
}

// shortNetError keeps what matters from Go's long network errors, for
// example "timed out" instead of the whole dial error.
func shortNetError(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
		return "timed out"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "DNS lookup failed"
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Err != nil {
		return opErr.Err.Error()
	}
	// API errors can carry a whole response body.
	message := err.Error()
	if len(message) > 160 {
		message = message[:157] + "..."
	}
	return message
}

// githubLatestTag reads the stable tag GitHub's /releases/latest redirects to.
// Mirrors either pass the redirect on or follow it and return the release
// page, which names the tag as well.
func githubLatestTag(client *http.Client, source githubSource, repo string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), githubLookupTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.url(githubWebBase+"/"+repo+"/releases/latest"), nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "1s-ui")
	noRedirects := *client
	noRedirects.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := noRedirects.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var text string
	switch {
	case response.StatusCode >= 300 && response.StatusCode < 400:
		text = response.Header.Get("Location")
	case response.StatusCode == http.StatusOK:
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2<<20))
		text = string(body)
	default:
		return "", fmt.Errorf("%s", response.Status)
	}
	marker := "/" + repo + "/releases/tag/"
	index := strings.Index(text, marker)
	if index < 0 {
		return "", fmt.Errorf("no release tag in the response")
	}
	tag := text[index+len(marker):]
	if end := strings.IndexAny(tag, "\"'?#/<> \t\r\n"); end >= 0 {
		tag = tag[:end]
	}
	if unescaped, err := url.PathUnescape(tag); err == nil {
		tag = unescaped
	}
	if !validVersion(tag) {
		return "", fmt.Errorf("unexpected release tag %q", tag)
	}
	return tag, nil
}

// githubAssetURL is where GitHub serves a release asset.
func githubAssetURL(repo, tag, name string) string {
	return githubWebBase + "/" + repo + "/releases/download/" + url.PathEscape(tag) + "/" + url.PathEscape(name)
}

// isGitHubAssetURL accepts only assets of the given repository on GitHub.
func isGitHubAssetURL(rawURL, repo string) bool {
	parsed, err := url.Parse(rawURL)
	base, baseErr := url.Parse(githubWebBase)
	return err == nil && baseErr == nil && parsed.Scheme == base.Scheme && parsed.Host == base.Host &&
		strings.HasPrefix(parsed.Path, "/"+repo+"/releases/download/")
}

// downloadGitHubAsset saves an asset to a temporary file, from GitHub or a
// mirror. digest ("sha256:<hex>") is checked when known; without it only
// GitHub and trusted mirrors are used. status reports the source being tried.
func downloadGitHubAsset(client *http.Client, assetURL, digest string, maxSize int64, pattern string, status func(source string)) (string, error) {
	sources := githubSources()
	if digest == "" && isGitHubAssetURL(assetURL, panelRepo) && hasUntrustedSource(sources) {
		digest = releaseChecksum(client, sources, assetURL)
	}
	var usable []githubSource
	for _, source := range sources {
		if source.trusted || digest != "" {
			usable = append(usable, source)
		}
	}
	var attempts githubAttempts
	for index, source := range usable {
		if status != nil {
			status(source.name())
		}
		name, err := downloadGitHubAssetFrom(client, source.url(assetURL), digest, maxSize, pattern, index < len(usable)-1)
		if err == nil {
			return name, nil
		}
		attempts.add(source.name(), err)
	}
	return "", attempts.err("download")
}

func hasUntrustedSource(sources []githubSource) bool {
	for _, source := range sources {
		if !source.trusted {
			return true
		}
	}
	return false
}

// releaseChecksumsName is the file listing the SHA-256 of every asset of a
// panel release, as sha256sum prints it.
const releaseChecksumsName = "SHA256SUMS"

// releaseChecksum finds an asset's SHA-256 in its release's SHA256SUMS, for
// when the GitHub API is unreachable. A trusted source's copy is taken as it
// is; the built-in mirrors' copies count only when two of them agree.
func releaseChecksum(client *http.Client, sources []githubSource, assetURL string) string {
	slash := strings.LastIndex(assetURL, "/")
	name, err := url.PathUnescape(assetURL[slash+1:])
	if slash < 0 || err != nil {
		return ""
	}
	sumsURL := assetURL[:slash+1] + releaseChecksumsName
	type result struct {
		trusted bool
		digest  string
	}
	results := make(chan result, len(sources))
	for _, source := range sources {
		go func(source githubSource) {
			results <- result{trusted: source.trusted, digest: fetchReleaseChecksum(client, source.url(sumsURL), name)}
		}(source)
	}
	agreeing := map[string]int{}
	for range sources {
		found := <-results
		if found.digest == "" {
			continue
		}
		if found.trusted {
			return found.digest
		}
		agreeing[found.digest]++
		if agreeing[found.digest] >= 2 {
			return found.digest
		}
	}
	return ""
}

func fetchReleaseChecksum(client *http.Client, rawURL, name string) string {
	ctx, cancel := context.WithTimeout(context.Background(), githubLookupTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ""
	}
	request.Header.Set("User-Agent", "1s-ui")
	response, err := client.Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ""
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	return checksumFor(string(body), name)
}

// checksumFor reads name's line in sha256sum output as "sha256:<hex>".
func checksumFor(sums, name string) string {
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		if decoded, err := hex.DecodeString(fields[0]); err == nil && len(decoded) == sha256.Size {
			return "sha256:" + strings.ToLower(fields[0])
		}
	}
	return ""
}

// A download from a source that is not the last one is dropped for the next
// source when, after slowDownloadGrace, it has averaged under
// slowDownloadMinRate or stalled for slowDownloadGrace.
var (
	slowDownloadGrace         = 20 * time.Second
	slowDownloadMinRate int64 = 64 << 10 // bytes per second
	errDownloadTooSlow        = errors.New("too slow")
)

type byteCounter struct{ total atomic.Int64 }

func (c *byteCounter) Write(p []byte) (int, error) {
	c.total.Add(int64(len(p)))
	return len(p), nil
}

// watchDownloadSpeed cancels the download with errDownloadTooSlow when it
// crawls. The returned function stops the watch.
func watchDownloadSpeed(received *byteCounter, cancel context.CancelCauseFunc) func() {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(slowDownloadGrace / 4)
		defer ticker.Stop()
		start := time.Now()
		last, lastProgress := int64(0), start
		for {
			select {
			case <-done:
				return
			case now := <-ticker.C:
				got := received.total.Load()
				if got != last {
					last, lastProgress = got, now
				}
				elapsed := now.Sub(start)
				if elapsed < slowDownloadGrace {
					continue
				}
				if now.Sub(lastProgress) >= slowDownloadGrace || float64(got) < elapsed.Seconds()*float64(slowDownloadMinRate) {
					cancel(errDownloadTooSlow)
					return
				}
			}
		}
	}()
	return func() { close(done) }
}

func downloadGitHubAssetFrom(client *http.Client, rawURL, digest string, maxSize int64, pattern string, dropIfSlow bool) (string, error) {
	parent, cancelSlow := context.WithCancelCause(context.Background())
	defer cancelSlow(nil)
	ctx, cancel := context.WithTimeout(parent, githubDownloadTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", "1s-ui")
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s", response.Status)
	}
	if response.ContentLength > maxSize {
		return "", fmt.Errorf("file exceeds %d MiB", maxSize>>20)
	}
	file, err := os.CreateTemp("", pattern)
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
	received := &byteCounter{}
	if dropIfSlow {
		stop := watchDownloadSpeed(received, cancelSlow)
		defer stop()
	}
	written, err := io.Copy(io.MultiWriter(file, hash, received), io.LimitReader(response.Body, maxSize+1))
	if err != nil {
		if cause := context.Cause(ctx); errors.Is(cause, errDownloadTooSlow) {
			return "", cause
		}
		return "", err
	}
	if written > maxSize {
		return "", fmt.Errorf("file exceeds %d MiB", maxSize>>20)
	}
	if err := verifyReleaseDigest(digest, hash.Sum(nil)); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	success = true
	return name, nil
}

// verifyReleaseDigest checks a "sha256:<hex>" digest; an empty one passes.
func verifyReleaseDigest(digest string, actual []byte) error {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return nil
	}
	algorithm, expected, found := strings.Cut(digest, ":")
	if !found || !strings.EqualFold(algorithm, "sha256") {
		return fmt.Errorf("release uses an unsupported digest")
	}
	decoded, err := hex.DecodeString(strings.TrimSpace(expected))
	if err != nil || len(decoded) != sha256.Size {
		return fmt.Errorf("release digest is invalid")
	}
	if !strings.EqualFold(hex.EncodeToString(decoded), hex.EncodeToString(actual)) {
		return fmt.Errorf("checksum mismatch")
	}
	return nil
}
