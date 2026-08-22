package service

import (
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/Hhz0823/1s-ui/config"
	"github.com/Hhz0823/1s-ui/util/common"
)

const (
	reverseProxyManagedBegin  = "# BEGIN 1S-UI MANAGED REVERSE PROXY"
	reverseProxyManagedEnd    = "# END 1S-UI MANAGED REVERSE PROXY"
	frontendManagedBegin      = "# BEGIN 1S-UI MANAGED FRONTEND GATEWAY"
	frontendManagedEnd        = "# END 1S-UI MANAGED FRONTEND GATEWAY"
	frontendRuntimeConfigPath = "/usr/local/s-ui/frontend-runtime/config.js"
)

var reverseProxyDomainLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
var frontendPathPattern = regexp.MustCompile(`^/[A-Za-z0-9._~%/-]*/$`)

type ReverseProxyConfig struct {
	Engine string `json:"engine"`
	Domain string `json:"domain"`
}

type ReverseProxyStatus struct {
	Supported             bool   `json:"supported"`
	Privileged            bool   `json:"privileged"`
	Enabled               bool   `json:"enabled"`
	Installed             bool   `json:"installed"`
	Running               bool   `json:"running"`
	Managed               bool   `json:"managed"`
	CaddyInstalled        bool   `json:"caddyInstalled"`
	NginxInstalled        bool   `json:"nginxInstalled"`
	Engine                string `json:"engine"`
	Domain                string `json:"domain"`
	PanelListen           string `json:"panelListen"`
	PanelPort             int    `json:"panelPort"`
	PanelPath             string `json:"panelPath"`
	PublicURL             string `json:"publicUrl"`
	Message               string `json:"message"`
	SplitDeployment       bool   `json:"splitDeployment"`
	APIListen             string `json:"apiListen"`
	FrontendApplied       bool   `json:"frontendApplied"`
	FrontendApplyRequired bool   `json:"frontendApplyRequired"`
	FrontendConfigPath    string `json:"frontendConfigPath"`
	PublicConfigPath      string `json:"publicConfigPath"`
}

type ReverseProxyService struct{}

func (s *ReverseProxyService) GetStatus() (*ReverseProxyStatus, error) {
	return reverseProxyStatusPlatform(s)
}

func (s *ReverseProxyService) Apply(config ReverseProxyConfig) (*ReverseProxyStatus, error) {
	engine := strings.ToLower(strings.TrimSpace(config.Engine))
	if engine != "caddy" && engine != "nginx" {
		return nil, common.NewError("reverse proxy engine must be caddy or nginx")
	}
	domain, err := normalizeReverseProxyDomain(config.Domain)
	if err != nil {
		return nil, err
	}
	config.Engine = engine
	config.Domain = domain
	return applyReverseProxyPlatform(s, config)
}

func reverseProxyPanelSettings() (*ReverseProxyStatus, error) {
	settings := &SettingService{}
	listen, err := settings.GetListen()
	if err != nil {
		return nil, err
	}
	port, err := settings.GetPort()
	if err != nil {
		return nil, err
	}
	path, err := settings.GetWebPath()
	if err != nil {
		return nil, err
	}
	domain, err := settings.GetWebDomain()
	if err != nil {
		return nil, err
	}
	return &ReverseProxyStatus{
		Engine:                "caddy",
		Domain:                strings.TrimSpace(domain),
		PanelListen:           strings.TrimSpace(listen),
		PanelPort:             port,
		PanelPath:             path,
		SplitDeployment:       true,
		APIListen:             apiListenAddress(),
		FrontendApplyRequired: FrontendEntryApplyRequired(),
	}, nil
}

func apiListenAddress() string {
	port, err := config.GetAPIPort()
	if err != nil {
		return "invalid"
	}
	return net.JoinHostPort(config.GetAPIListen(), strconv.Itoa(port))
}

func normalizeReverseProxyDomain(value string) (string, error) {
	domain := strings.ToLower(strings.TrimSpace(value))
	domain = strings.TrimSuffix(domain, ".")
	if domain == "" {
		return "", nil
	}
	if len(domain) > 253 || strings.ContainsAny(domain, "/:@[] \t\r\n") {
		return "", common.NewError("domain must be a hostname without scheme, port, or path")
	}
	for _, label := range strings.Split(domain, ".") {
		if !reverseProxyDomainLabel.MatchString(label) {
			return "", common.NewError("invalid reverse proxy domain")
		}
	}
	return domain, nil
}

func reverseProxyPublicURL(engine, domain, panelPath string) string {
	if domain == "" {
		return ""
	}
	scheme := "http"
	if engine == "caddy" {
		scheme = "https"
	}
	return scheme + "://" + domain + normalizePanelPath(panelPath)
}

func normalizePanelPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/app/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

func validateFrontendPanelPath(value string) (string, error) {
	value = normalizePanelPath(value)
	if !frontendPathPattern.MatchString(value) || strings.Contains(value, "//") {
		return "", common.NewError("panel path contains characters unsupported by the managed frontend gateway")
	}
	return value, nil
}

func renderFrontendRuntimeConfig(panelPath, backendURL string) ([]byte, error) {
	panelPath, err := validateFrontendPanelPath(panelPath)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(struct {
		BasePath   string `json:"basePath"`
		BackendURL string `json:"backendUrl"`
	}{BasePath: panelPath, BackendURL: strings.TrimSpace(backendURL)})
	if err != nil {
		return nil, err
	}
	return []byte("window.__SUI_CONFIG__ = Object.freeze(" + string(payload) + ");\n"), nil
}

func renderNginxFrontendGateway(listen string, panelPort int, panelPath, domain, frontendRoot, apiAddress string) (string, error) {
	panelPath, err := validateFrontendPanelPath(panelPath)
	if err != nil {
		return "", err
	}
	if panelPort < 1 || panelPort > 65535 {
		return "", common.NewError("panel port must be between 1 and 65535")
	}
	listen = strings.TrimSpace(listen)
	listenLines := fmt.Sprintf("    listen %d;\n    listen [::]:%d;", panelPort, panelPort)
	if listen != "" && listen != "0.0.0.0" && listen != "::" {
		ip := net.ParseIP(strings.Trim(listen, "[]"))
		if ip == nil {
			return "", common.NewError("panel listen must be an IP address")
		}
		if ip.To4() == nil {
			listenLines = fmt.Sprintf("    listen [%s]:%d;", strings.Trim(listen, "[]"), panelPort)
		} else {
			listenLines = fmt.Sprintf("    listen %s:%d;", listen, panelPort)
		}
	}
	domain, err = normalizeReverseProxyDomain(domain)
	if err != nil {
		return "", err
	}
	serverName := "_"
	if domain != "" {
		serverName = domain
	}
	frontendRoot = strings.TrimSuffix(frontendRoot, "/")
	legacyPrefix := panelPath
	canonical := []string{"/api/", "/apiv2/", "/agent/v1/"}
	legacy := []string{legacyPrefix + "api/", legacyPrefix + "apiv2/", legacyPrefix + "agent/v1/"}
	locations := make([]string, 0, 6)
	seen := make(map[string]struct{})
	for _, prefix := range append(canonical, legacy...) {
		if _, ok := seen[prefix]; ok {
			continue
		}
		seen[prefix] = struct{}{}
		locations = append(locations, fmt.Sprintf(`    location ^~ %s {
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $http_x_forwarded_proto;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 3600s;
        proxy_pass http://%s;
    }
`, prefix, apiAddress))
	}
	redirect := ""
	if panelPath != "/" {
		redirect = fmt.Sprintf("    location = %s { return 308 %s; }\n", strings.TrimSuffix(panelPath, "/"), panelPath)
	}
	return fmt.Sprintf(`%s
server {
%s
    server_name %s;
    client_max_body_size 32m;

    location = /.well-known/1s-ui/config.js {
        alias %s;
        default_type application/javascript;
        add_header Cache-Control "no-store" always;
    }

%s
%s    location ^~ %s {
        alias %s/;
        try_files $uri $uri/ %sindex.html;
    }
}
%s
`, frontendManagedBegin, listenLines, serverName, frontendRuntimeConfigPath, strings.Join(locations, "\n"), redirect, panelPath, frontendRoot, panelPath, frontendManagedEnd), nil
}

func renderCaddyReverseProxy(domain string, panelPort int) string {
	site := ":80"
	if domain != "" {
		site = domain
	}
	return fmt.Sprintf(`%s
%s {
	encode gzip
	reverse_proxy 127.0.0.1:%d
}
%s
`, reverseProxyManagedBegin, site, panelPort, reverseProxyManagedEnd)
}

func renderNginxReverseProxy(domain string, panelPort int) string {
	serverName := "_"
	if domain != "" {
		serverName = domain
	}
	return fmt.Sprintf(`%s
server {
    listen 80;
    listen [::]:80;
    server_name %s;

    client_max_body_size 32m;

    location / {
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_pass http://127.0.0.1:%d;
    }
}
%s
`, reverseProxyManagedBegin, serverName, panelPort, reverseProxyManagedEnd)
}

func canReplaceReverseProxyConfig(engine string, content []byte) bool {
	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return true
	}
	if strings.HasPrefix(trimmed, reverseProxyManagedBegin) &&
		strings.HasSuffix(trimmed, reverseProxyManagedEnd) {
		return true
	}
	switch engine {
	case "caddy":
		return isLegacyCaddyReverseProxy(trimmed)
	case "nginx":
		return isLegacyNginxReverseProxy(trimmed)
	default:
		return false
	}
}

func isLegacyCaddyReverseProxy(content string) bool {
	if strings.Count(content, "reverse_proxy") != 1 ||
		!strings.Contains(content, "reverse_proxy 127.0.0.1:") ||
		!strings.Contains(content, "encode gzip") ||
		!strings.Contains(content, "header_up X-Real-IP {remote_host}") ||
		!strings.Contains(content, "header_up X-Forwarded-For {remote_host}") ||
		!strings.Contains(content, "header_up X-Forwarded-Proto {scheme}") {
		return false
	}
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		switch {
		case line == "", line == "{", line == "}":
		case strings.HasPrefix(line, "email "):
		case line == "encode gzip":
		case strings.HasPrefix(line, "reverse_proxy 127.0.0.1:"):
			port := strings.TrimSuffix(strings.TrimPrefix(line, "reverse_proxy 127.0.0.1:"), " {")
			if _, err := strconv.Atoi(port); err != nil {
				return false
			}
		case strings.HasSuffix(line, " {"):
		case strings.HasPrefix(line, "header_up X-Real-IP "):
		case strings.HasPrefix(line, "header_up X-Forwarded-For "):
		case strings.HasPrefix(line, "header_up X-Forwarded-Proto "):
		default:
			return false
		}
	}
	return true
}

func isLegacyNginxReverseProxy(content string) bool {
	if strings.Count(content, "server {") != 1 ||
		strings.Count(content, "proxy_pass ") != 1 ||
		!strings.Contains(content, "proxy_pass http://127.0.0.1:") ||
		!strings.Contains(content, "client_max_body_size 32m;") ||
		!strings.Contains(content, "proxy_set_header X-Real-IP $remote_addr;") ||
		!strings.Contains(content, "proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;") {
		return false
	}
	return !strings.Contains(content, "include ") && !strings.Contains(content, "root ")
}
