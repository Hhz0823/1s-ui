package api

import (
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

const originPolicyContextKey = "sui-origin-policy"

// OriginPolicy is the single source of truth for credentialed browser CORS,
// CSRF checks, and browser WebSocket origins.
type OriginPolicy struct {
	allowed           map[string]struct{}
	websocketPatterns []string
}

func NewOriginPolicy(value string) (*OriginPolicy, error) {
	policy := &OriginPolicy{allowed: make(map[string]struct{})}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		origin, err := normalizeOrigin(item)
		if err != nil {
			return nil, err
		}
		policy.allowed[origin] = struct{}{}
	}
	for origin := range policy.allowed {
		policy.websocketPatterns = append(policy.websocketPatterns, escapeWebSocketPattern(origin))
	}
	return policy, nil
}

func NewOriginPolicyFromEnv() (*OriginPolicy, error) {
	return NewOriginPolicy(os.Getenv("SUI_ALLOWED_ORIGINS"))
}

func normalizeOrigin(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" ||
		strings.ContainsAny(value, "*?") {
		return "", &url.Error{Op: "parse SUI_ALLOWED_ORIGINS", URL: value, Err: errInvalidOrigin}
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), nil
}

var errInvalidOrigin = invalidOriginError{}

type invalidOriginError struct{}

func (invalidOriginError) Error() string {
	return "origin must be an exact http(s) origin without path, query, fragment, or wildcard"
}

func escapeWebSocketPattern(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `*`, `\*`)
	value = strings.ReplaceAll(value, `?`, `\?`)
	return strings.ReplaceAll(value, `[`, `\[`)
}

func (p *OriginPolicy) requestAllowed(c *gin.Context) bool {
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin == "" {
		return !strings.EqualFold(strings.TrimSpace(c.GetHeader("Sec-Fetch-Site")), "cross-site")
	}
	normalized, err := normalizeOrigin(origin)
	if err != nil {
		return false
	}
	if normalized == requestOrigin(c) {
		return true
	}
	_, ok := p.allowed[normalized]
	return ok
}

func requestOrigin(c *gin.Context) string {
	scheme := "http"
	if requestIsHTTPS(c) {
		scheme = "https"
	}
	return scheme + "://" + strings.ToLower(c.Request.Host)
}

func apiPathMatches(requestPath string, prefixes []string) bool {
	cleaned := path.Clean(requestPath)
	for _, prefix := range prefixes {
		prefix = strings.TrimSuffix(prefix, "/")
		if cleaned == prefix || strings.HasPrefix(cleaned, prefix+"/") {
			return true
		}
	}
	return false
}

func (p *OriginPolicy) CORSMiddleware(prefixes []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(originPolicyContextKey, p)
		if !apiPathMatches(c.Request.URL.Path, prefixes) {
			c.Next()
			return
		}

		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" {
			if !p.requestAllowed(c) {
				c.AbortWithStatusJSON(http.StatusForbidden, Msg{Success: false, Msg: "origin is not allowed"})
				return
			}
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Expose-Headers", "Content-Disposition")
			c.Header("Vary", "Origin")
		}

		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Token, X-Requested-With")
			c.Header("Access-Control-Max-Age", "600")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (p *OriginPolicy) BrowserCSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(originPolicyContextKey, p)
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if c.GetHeader("X-Requested-With") != "XMLHttpRequest" {
			c.AbortWithStatusJSON(http.StatusForbidden, Msg{Success: false, Msg: "X-Requested-With is required"})
			return
		}
		if !p.requestAllowed(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, Msg{Success: false, Msg: "origin is not allowed"})
			return
		}
		c.Next()
	}
}

func originPolicyFromContext(c *gin.Context) *OriginPolicy {
	if value, ok := c.Get(originPolicyContextKey); ok {
		if policy, ok := value.(*OriginPolicy); ok {
			return policy
		}
	}
	policy, err := NewOriginPolicyFromEnv()
	if err != nil {
		return &OriginPolicy{allowed: make(map[string]struct{})}
	}
	return policy
}

func browserWebSocketAcceptOptions(c *gin.Context) *websocket.AcceptOptions {
	policy := originPolicyFromContext(c)
	patterns := append([]string(nil), policy.websocketPatterns...)
	return &websocket.AcceptOptions{OriginPatterns: patterns}
}
