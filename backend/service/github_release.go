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
//     known from GitHub or built into the panel; the administrator's own
//     mirror is trusted like GitHub.
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
// mirrors, in that order.
func githubSources() []githubSource {
	sources := []githubSource{{trusted: true}}
	configured := ""
	if database.GetDB() != nil {
		configured, _ = (&SettingService{}).GetGitHubMirror()
	}
	if configured != "" {
		sources = append(sources, githubSource{prefix: configured, trusted: true})
	}
	for _, prefix := range builtinGitHubMirrors {
		if prefix != configured {
			sources = append(sources, githubSource{prefix: prefix})
		}
	}
	return sources
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
// GitHub and the administrator's mirror are used. status reports the source
// being tried.
func downloadGitHubAsset(client *http.Client, assetURL, digest string, maxSize int64, pattern string, status func(source string)) (string, error) {
	var attempts githubAttempts
	for _, source := range githubSources() {
		if !source.trusted && digest == "" {
			continue
		}
		if status != nil {
			status(source.name())
		}
		name, err := downloadGitHubAssetFrom(client, source.url(assetURL), digest, maxSize, pattern)
		if err == nil {
			return name, nil
		}
		attempts.add(source.name(), err)
	}
	return "", attempts.err("download")
}

func downloadGitHubAssetFrom(client *http.Client, rawURL, digest string, maxSize int64, pattern string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), githubDownloadTimeout)
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
	written, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(response.Body, maxSize+1))
	if err != nil {
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
