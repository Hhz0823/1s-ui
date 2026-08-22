package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Hhz0823/1s-ui/util/common"
)

const (
	defaultLocalAgentBinary   = "/usr/local/s-ui/sui-agent"
	defaultLocalAgentEnvFile  = "/etc/default/1s-ui-agent"
	defaultLocalControlSocket = "/run/s-ui/control.sock"
	localAgentServiceName     = "s-ui-agent.service"
)

type LocalAgentConnection struct {
	PanelURL string `json:"panel_url"`
	Version  string `json:"version"`
}

type LocalAgentConnectionStatus struct {
	Supported  bool   `json:"supported"`
	Installed  bool   `json:"installed"`
	Configured bool   `json:"configured"`
	Running    bool   `json:"running"`
	PanelURL   string `json:"panel_url"`
	Insecure   bool   `json:"insecure"`
}

type pairingAPIResponse struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
	Obj     struct {
		PanelURL string `json:"panel_url"`
		Token    string `json:"token"`
		Version  string `json:"version"`
	} `json:"obj"`
}

func (s *AgentService) ConnectLocalController(rawLink string, insecure bool) (*LocalAgentConnection, error) {
	if runtime.GOOS != "linux" {
		return nil, common.NewError("connecting this panel as a managed server requires Linux")
	}
	binaryPath := envOrDefault("SUI_AGENT_BINARY", defaultLocalAgentBinary)
	if info, err := os.Stat(binaryPath); err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		return nil, common.NewError("Agent component is not installed; install the managed-client package first")
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return nil, common.NewError("systemd is required to connect this managed server")
	}

	connection, token, err := exchangeAgentPairing(context.Background(), rawLink, insecure, nil)
	if err != nil {
		return nil, err
	}
	envPath := envOrDefault("SUI_AGENT_ENV_FILE", defaultLocalAgentEnvFile)
	controlSocket := envOrDefault("SUI_CONTROL_SOCKET", defaultLocalControlSocket)
	content := fmt.Sprintf(
		"SUI_AGENT_PANEL=%s\nSUI_AGENT_TOKEN=%s\nSUI_AGENT_INTERVAL=15s\nSUI_AGENT_INSECURE=%t\nSUI_AGENT_LOCAL_SOCKET=%s\n",
		connection.PanelURL, token, insecure, controlSocket,
	)

	checkCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	check := exec.CommandContext(checkCtx, binaryPath, "--interval", "15s", "--local-socket", controlSocket, "--once")
	check.Env = append(os.Environ(),
		"SUI_AGENT_PANEL="+connection.PanelURL,
		"SUI_AGENT_TOKEN="+token,
		fmt.Sprintf("SUI_AGENT_INSECURE=%t", insecure),
	)
	if output, err := check.CombinedOutput(); err != nil {
		return nil, common.NewErrorf("the controller accepted the connection API, but the Agent connection check failed: %s", limitedOutput(output, err))
	}
	if err := writePrivateFile(envPath, []byte(content)); err != nil {
		return nil, common.NewErrorf("pairing succeeded but Agent configuration could not be saved: %v", err)
	}

	commands := [][]string{
		{"daemon-reload"},
		{"enable", localAgentServiceName},
		{"restart", localAgentServiceName},
		{"is-active", "--quiet", localAgentServiceName},
	}
	for _, args := range commands {
		cmd := exec.Command("systemctl", args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, common.NewErrorf("Agent configuration was saved, but systemd could not start it: %s", limitedOutput(output, err))
		}
	}
	return connection, nil
}

func (s *AgentService) GetLocalControllerStatus() (*LocalAgentConnectionStatus, error) {
	status := &LocalAgentConnectionStatus{Supported: runtime.GOOS == "linux"}
	binaryPath := envOrDefault("SUI_AGENT_BINARY", defaultLocalAgentBinary)
	if info, err := os.Stat(binaryPath); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
		status.Installed = true
	}
	envPath := envOrDefault("SUI_AGENT_ENV_FILE", defaultLocalAgentEnvFile)
	content, err := os.ReadFile(envPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil {
		values := parseAgentEnvironment(content)
		panelURL, panelErr := validatePanelURL(values["SUI_AGENT_PANEL"])
		if panelErr == nil && validAgentCredential(values["SUI_AGENT_TOKEN"]) {
			status.Configured = true
			status.PanelURL = panelURL
			status.Insecure = strings.EqualFold(values["SUI_AGENT_INSECURE"], "true")
		}
	}
	if status.Supported {
		if _, err := exec.LookPath("systemctl"); err == nil {
			status.Running = exec.Command("systemctl", "is-active", "--quiet", localAgentServiceName).Run() == nil
		}
	}
	return status, nil
}

func (s *AgentService) DisconnectLocalController() (*LocalAgentConnectionStatus, error) {
	if runtime.GOOS != "linux" {
		return nil, common.NewError("disconnecting a managed server requires Linux")
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return nil, common.NewError("systemd is required to disconnect this managed server")
	}
	for _, args := range [][]string{{"stop", localAgentServiceName}, {"disable", localAgentServiceName}} {
		if output, err := exec.Command("systemctl", args...).CombinedOutput(); err != nil {
			message := strings.ToLower(string(output))
			if !strings.Contains(message, "not loaded") && !strings.Contains(message, "does not exist") {
				return nil, common.NewErrorf("could not %s Agent service: %s", args[0], limitedOutput(output, err))
			}
		}
	}
	envPath := envOrDefault("SUI_AGENT_ENV_FILE", defaultLocalAgentEnvFile)
	if err := os.Remove(envPath); err != nil && !os.IsNotExist(err) {
		return nil, common.NewErrorf("could not remove Agent connection: %v", err)
	}
	return s.GetLocalControllerStatus()
}

func parseAgentEnvironment(content []byte) map[string]string {
	values := make(map[string]string)
	for _, rawLine := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if !strings.HasPrefix(key, "SUI_AGENT_") {
			continue
		}
		values[key] = strings.Trim(strings.TrimSpace(value), "\"'")
	}
	return values
}

func exchangeAgentPairing(ctx context.Context, rawLink string, insecure bool, client *http.Client) (*LocalAgentConnection, string, error) {
	endpoint, code, err := parseAgentPairingLink(rawLink)
	if err != nil {
		return nil, "", err
	}
	payload, _ := json.Marshal(map[string]string{"code": code, "name": localAgentEnrollmentName()})
	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "1S-UI managed-server pairing")

	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		if insecure {
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 -- explicit administrator option
		}
		client = &http.Client{
			Transport: transport,
			Timeout:   15 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, "", common.NewErrorf("could not reach the controller connection API: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 32<<10))
	if err != nil {
		return nil, "", common.NewError("could not read the controller pairing response")
	}
	var result pairingAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, "", common.NewErrorf("controller returned an invalid connection response (HTTP %d)", response.StatusCode)
	}
	if response.StatusCode != http.StatusOK || !result.Success {
		message := strings.TrimSpace(result.Msg)
		if message == "" {
			message = response.Status
		}
		return nil, "", common.NewErrorf("controller rejected the connection API: %s", message)
	}
	panelURL, err := validatePanelURL(result.Obj.PanelURL)
	if err != nil {
		return nil, "", err
	}
	if !validAgentCredential(result.Obj.Token) {
		return nil, "", common.NewError("controller returned an invalid Agent token")
	}
	return &LocalAgentConnection{PanelURL: panelURL, Version: strings.TrimSpace(result.Obj.Version)}, result.Obj.Token, nil
}

func parseAgentPairingLink(rawLink string) (string, string, error) {
	rawLink = strings.TrimSpace(rawLink)
	if rawLink == "" || len(rawLink) > 2048 {
		return "", "", common.NewError("enter the public controller address")
	}
	parsed, err := url.Parse(rawLink)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", "", common.NewError("invalid controller connection address")
	}
	code := strings.TrimSpace(parsed.Fragment)
	parsed.Fragment = ""
	if code == "" {
		query := parsed.Query()
		code = strings.TrimSpace(query.Get("code"))
		query.Del("code")
		parsed.RawQuery = query.Encode()
	}
	if code != "" && !validAgentCredential(code) {
		return "", "", common.NewError("invalid controller connection key")
	}
	path := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(path, "/agent/v1/pair") && code == "" {
		return "", "", common.NewError("the one-time pairing endpoint requires its connection key")
	}
	if !strings.HasSuffix(path, "/agent/v1/pair") && !strings.HasSuffix(path, "/agent/v1/enroll") {
		parsed.Path = path + "/agent/v1/enroll"
		parsed.RawPath = ""
	}
	return parsed.String(), code, nil
}

func localAgentEnrollmentName() string {
	hostname, _ := os.Hostname()
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		hostname = "managed-server"
	}
	runes := []rune(hostname)
	if len(runes) > 80 {
		hostname = string(runes[:80])
	}
	return hostname
}

func validatePanelURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", common.NewError("controller returned an invalid panel URL")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", common.NewError("controller panel URL must not contain a query or fragment")
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	return parsed.String(), nil
}

func validAgentCredential(value string) bool {
	if len(value) < 32 || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func writePrivateFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".1s-ui-agent-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func limitedOutput(output []byte, fallback error) string {
	message := strings.TrimSpace(string(output))
	if message == "" && fallback != nil {
		message = fallback.Error()
	}
	if len(message) > 512 {
		message = message[:512]
	}
	return message
}
