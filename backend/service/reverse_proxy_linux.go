//go:build linux

package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/util/common"
)

const (
	caddyConfigPath         = "/etc/caddy/Caddyfile"
	nginxFrontendConfigPath = "/etc/nginx/sites-available/s-ui-frontend.conf"
	nginxPublicConfigPath   = "/etc/nginx/sites-available/s-ui-public.conf"
	frontendRootPath        = "/usr/local/s-ui/frontend"
)

func reverseProxyStatusPlatform(_ *ReverseProxyService) (*ReverseProxyStatus, error) {
	status, err := reverseProxyPanelSettings()
	if err != nil {
		return nil, err
	}

	_, systemctlErr := exec.LookPath("systemctl")
	_, caddyErr := exec.LookPath("caddy")
	_, nginxErr := exec.LookPath("nginx")
	status.Privileged = os.Geteuid() == 0
	status.Supported = status.Privileged && systemctlErr == nil
	status.CaddyInstalled = caddyErr == nil
	status.NginxInstalled = nginxErr == nil
	status.FrontendConfigPath = nginxFrontendConfigPath
	status.FrontendApplied = serviceIsActive("nginx") && frontendConfigIsManaged(nginxFrontendConfigPath) && !status.FrontendApplyRequired

	caddyManaged := proxyConfigIsManaged("caddy", caddyConfigPath)
	nginxManaged := proxyConfigIsManaged("nginx", nginxPublicConfigPath)
	switch {
	case caddyManaged:
		status.Engine = "caddy"
		status.Managed = true
		status.Running = serviceIsActive("caddy")
		status.PublicConfigPath = caddyConfigPath
	case nginxManaged:
		status.Engine = "nginx"
		status.Managed = true
		status.Running = serviceIsActive("nginx")
		status.PublicConfigPath = nginxPublicConfigPath
	case status.CaddyInstalled:
		status.Engine = "caddy"
		status.PublicConfigPath = caddyConfigPath
	default:
		status.Engine = "nginx"
		status.PublicConfigPath = nginxPublicConfigPath
	}

	status.Installed = (status.Engine == "caddy" && status.CaddyInstalled) ||
		(status.Engine == "nginx" && status.NginxInstalled)
	status.Enabled = status.Running && status.Managed
	status.PublicURL = reverseProxyPublicURL(status.Engine, status.Domain, status.PanelPath)

	switch {
	case !status.Privileged:
		status.Message = "s-ui must run as root to manage the public entry"
	case systemctlErr != nil:
		status.Message = "systemd is unavailable; frontend and public entry management is read-only"
	case !status.NginxInstalled:
		status.Message = "nginx is required for the split frontend gateway"
	case status.FrontendApplyRequired:
		status.Message = "frontend entry settings changed; apply the managed nginx frontend gateway"
	case !status.FrontendApplied:
		status.Message = "the managed nginx frontend gateway is not active"
	case status.Running && !status.Managed:
		status.Message = "the active public entry has custom configuration and will not be overwritten"
	}
	return status, nil
}

func applyReverseProxyPlatform(s *ReverseProxyService, proxy ReverseProxyConfig) (*ReverseProxyStatus, error) {
	status, err := reverseProxyStatusPlatform(s)
	if err != nil {
		return nil, err
	}
	if !status.Supported {
		return nil, common.NewError(status.Message)
	}
	if !status.NginxInstalled {
		return nil, common.NewError("nginx is required for the split frontend gateway")
	}
	if proxy.Engine == "caddy" && !status.CaddyInstalled {
		return nil, common.NewError("Caddy is not installed; install it first or rerun the full installer")
	}
	if proxy.Engine == "caddy" && proxyConfigIsManaged("nginx", nginxPublicConfigPath) {
		return nil, common.NewError("the managed nginx public entry is enabled; remove it before switching to Caddy")
	}
	if proxy.Engine == "nginx" && proxyConfigIsManaged("caddy", caddyConfigPath) && serviceIsActive("caddy") {
		return nil, common.NewError("the managed Caddy public entry is active; stop it before switching to nginx")
	}

	settings := &SettingService{}
	panelPort, err := settings.GetPort()
	if err != nil {
		return nil, err
	}
	panelPath, err := settings.GetWebPath()
	if err != nil {
		return nil, err
	}
	oldListen, err := settings.GetListen()
	if err != nil {
		return nil, err
	}
	oldDomain, err := settings.GetWebDomain()
	if err != nil {
		return nil, err
	}
	oldURI, err := settings.getString("webURI")
	if err != nil {
		return nil, err
	}

	frontendConfig, err := renderNginxFrontendGateway("127.0.0.1", panelPort, panelPath, proxy.Domain, frontendRootPath, apiListenAddress())
	if err != nil {
		return nil, err
	}
	publicPath := caddyConfigPath
	publicConfig := renderCaddyReverseProxy(proxy.Domain, panelPort)
	if proxy.Engine == "nginx" {
		publicPath = nginxPublicConfigPath
		publicConfig = renderNginxReverseProxy(proxy.Domain, panelPort)
	}

	oldFrontend, frontendExisted, err := readOptionalFile(nginxFrontendConfigPath)
	if err != nil {
		return nil, err
	}
	if frontendExisted && !frontendConfigIsManagedContent(oldFrontend) {
		return nil, common.NewErrorf("%s contains custom configuration; the panel will not overwrite it", nginxFrontendConfigPath)
	}
	oldPublic, publicExisted, err := readOptionalFile(publicPath)
	if err != nil {
		return nil, err
	}
	if !canReplaceReverseProxyConfig(proxy.Engine, oldPublic) {
		return nil, common.NewErrorf("%s contains custom configuration; the panel will not overwrite it", publicPath)
	}
	oldRuntimeConfig, runtimeConfigExisted, err := readOptionalFile(frontendRuntimeConfigPath)
	if err != nil {
		return nil, err
	}
	runtimeConfig, err := renderFrontendRuntimeConfig(panelPath, "")
	if err != nil {
		return nil, err
	}

	rollback := func() {
		restoreProxyConfig(nginxFrontendConfigPath, oldFrontend, frontendExisted)
		restoreProxyConfig(publicPath, oldPublic, publicExisted)
		restoreProxyConfig(frontendRuntimeConfigPath, oldRuntimeConfig, runtimeConfigExisted)
		if !frontendExisted {
			_ = os.Remove(filepath.Join("/etc/nginx/sites-enabled", filepath.Base(nginxFrontendConfigPath)))
		}
		if proxy.Engine == "nginx" && !publicExisted {
			_ = os.Remove(filepath.Join("/etc/nginx/sites-enabled", filepath.Base(nginxPublicConfigPath)))
		}
		_ = settings.SetWebListen(oldListen)
		_ = settings.SetWebDomain(oldDomain)
		_ = settings.SetWebURI(oldURI)
		_, _ = exec.Command("nginx", "-t").CombinedOutput()
		_, _ = exec.Command("systemctl", "reload", "nginx").CombinedOutput()
	}

	if err := writeAtomicFile(nginxFrontendConfigPath, []byte(frontendConfig), 0644); err != nil {
		return nil, err
	}
	if err := writeAtomicFile(frontendRuntimeConfigPath, runtimeConfig, 0644); err != nil {
		rollback()
		return nil, err
	}
	if err := enableNginxSite(nginxFrontendConfigPath); err != nil {
		rollback()
		return nil, err
	}
	if err := writeAtomicFile(publicPath, []byte(publicConfig), 0644); err != nil {
		rollback()
		return nil, err
	}
	if proxy.Engine == "nginx" {
		if err := enableNginxSite(nginxPublicConfigPath); err != nil {
			rollback()
			return nil, err
		}
	}
	if out, err := exec.Command("nginx", "-t").CombinedOutput(); err != nil {
		rollback()
		return nil, common.NewErrorf("nginx frontend configuration is invalid: %s", commandOutput(out, err))
	}
	if proxy.Engine == "caddy" {
		if out, err := validateReverseProxy("caddy", publicPath); err != nil {
			rollback()
			return nil, common.NewErrorf("Caddy public configuration is invalid: %s", commandOutput(out, err))
		}
	}

	if err := enableAndReloadService("nginx"); err != nil {
		rollback()
		return nil, err
	}
	if proxy.Engine == "caddy" {
		if err := enableAndReloadService("caddy"); err != nil {
			rollback()
			return nil, err
		}
	}

	publicURL := reverseProxyPublicURL(proxy.Engine, proxy.Domain, panelPath)
	if err := saveReverseProxyPanelSettings(settings, proxy.Domain, publicURL); err != nil {
		rollback()
		return nil, err
	}
	if err := ClearFrontendEntryApplyRequired(); err != nil {
		rollback()
		return nil, err
	}
	logger.Infof("public entry configured: engine=%s domain=%s frontend=127.0.0.1:%d api=%s", proxy.Engine, proxy.Domain, panelPort, apiListenAddress())
	return reverseProxyStatusPlatform(s)
}

func saveReverseProxyPanelSettings(settings *SettingService, domain, publicURL string) error {
	if err := settings.SetWebListen("127.0.0.1"); err != nil {
		return err
	}
	if err := settings.SetWebDomain(domain); err != nil {
		return err
	}
	return settings.SetWebURI(publicURL)
}

func enableAndReloadService(name string) error {
	if out, err := exec.Command("systemctl", "enable", name).CombinedOutput(); err != nil {
		return common.NewErrorf("failed to enable %s: %s", name, commandOutput(out, err))
	}
	action := "start"
	if serviceIsActive(name) {
		action = "reload"
	}
	if out, err := exec.Command("systemctl", action, name).CombinedOutput(); err != nil {
		return common.NewErrorf("failed to %s %s: %s", action, name, commandOutput(out, err))
	}
	return nil
}

func serviceIsActive(name string) bool {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	return exec.Command("systemctl", "is-active", "--quiet", name).Run() == nil
}

func proxyConfigIsManaged(engine, path string) bool {
	content, err := os.ReadFile(path)
	return err == nil && strings.TrimSpace(string(content)) != "" && canReplaceReverseProxyConfig(engine, content)
}

func frontendConfigIsManaged(path string) bool {
	content, err := os.ReadFile(path)
	return err == nil && frontendConfigIsManagedContent(content)
}

func frontendConfigIsManagedContent(content []byte) bool {
	trimmed := strings.TrimSpace(string(content))
	return strings.HasPrefix(trimmed, frontendManagedBegin) && strings.HasSuffix(trimmed, frontendManagedEnd)
}

func readOptionalFile(path string) ([]byte, bool, error) {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	return content, err == nil, err
}

func writeAtomicFile(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".1s-ui-proxy-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func restoreProxyConfig(path string, content []byte, existed bool) {
	if existed {
		if err := writeAtomicFile(path, content, 0644); err != nil {
			logger.Error("restore reverse proxy config failed: ", err)
		}
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logger.Error("remove failed reverse proxy config failed: ", err)
	}
}

func validateReverseProxy(engine, configPath string) ([]byte, error) {
	if engine == "caddy" {
		return exec.Command("caddy", "validate", "--config", configPath, "--adapter", "caddyfile").CombinedOutput()
	}
	return exec.Command("nginx", "-t").CombinedOutput()
}

func enableNginxSite(configPath string) error {
	enabledPath := filepath.Join("/etc/nginx/sites-enabled", filepath.Base(configPath))
	if err := os.MkdirAll(filepath.Dir(enabledPath), 0755); err != nil {
		return err
	}
	if target, err := os.Readlink(enabledPath); err == nil && target == configPath {
		return nil
	}
	if err := os.Remove(enabledPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(configPath, enabledPath)
}

func commandOutput(out []byte, err error) string {
	message := strings.TrimSpace(string(out))
	if message != "" {
		return message
	}
	if err != nil {
		return err.Error()
	}
	return "unknown error"
}
