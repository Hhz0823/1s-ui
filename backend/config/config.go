package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

//go:embed version
var version string

//go:embed name
var name string

type LogLevel string

const (
	Debug LogLevel = "debug"
	Info  LogLevel = "info"
	Warn  LogLevel = "warn"
	Error LogLevel = "error"
)

func GetVersion() string {
	return strings.TrimSpace(version)
}

func GetName() string {
	return strings.TrimSpace(name)
}

func GetLogLevel() LogLevel {
	if IsDebug() {
		return Debug
	}
	logLevel := os.Getenv("SUI_LOG_LEVEL")
	if logLevel == "" {
		return Info
	}
	return LogLevel(logLevel)
}

func IsDebug() bool {
	return os.Getenv("SUI_DEBUG") == "true"
}

func GetAPIListen() string {
	if listen := strings.TrimSpace(os.Getenv("SUI_API_LISTEN")); listen != "" {
		return listen
	}
	return "127.0.0.1"
}

func GetAPIPort() (int, error) {
	value := strings.TrimSpace(os.Getenv("SUI_API_PORT"))
	if value == "" {
		return 2097, nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 0 || port > 65535 {
		return 0, fmt.Errorf("invalid SUI_API_PORT %q", value)
	}
	return port, nil
}

// IsSkipCore reports whether proxy cores should not be started at process boot.
// Useful on low-memory VPS so the panel web UI can come up without loading
// sing-box/Xray into RAM. Cores can still be started later from the panel API.
//
// Set with environment variable SUI_SKIP_CORE=true|1|yes
// or create an empty marker file next to the DB: <db-folder>/.skip_core
func IsSkipCore() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SUI_SKIP_CORE"))) {
	case "1", "true", "yes", "on":
		return true
	}
	marker := filepath.Join(GetDBFolderPath(), ".skip_core")
	if _, err := os.Stat(marker); err == nil {
		return true
	}
	return false
}

// IsXrayDisabled reports whether Xray-core is unavailable for this panel.
// Low-resource Linux installations use this guard to prevent an Xray binary
// left by an older release from being started alongside sing-box.
//
// Set with environment variable SUI_DISABLE_XRAY=true|1|yes
// or create an empty marker file next to the DB: <db-folder>/.disable_xray
func IsXrayDisabled() bool {
	// A runtime marker is controlled from the Web panel and must override the
	// installer's legacy environment value after an upgrade.
	runtimeEnabled := filepath.Join(GetDBFolderPath(), ".xray_enabled")
	if _, err := os.Stat(runtimeEnabled); err == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SUI_DISABLE_XRAY"))) {
	case "1", "true", "yes", "on":
		return true
	}
	marker := filepath.Join(GetDBFolderPath(), ".disable_xray")
	if _, err := os.Stat(marker); err == nil {
		return true
	}
	return false
}

// IsXrayOnDemand reports whether Xray is installed but should only be started
// after an explicit panel action. This keeps sing-box as the low-memory
// default while preserving the historical dual-core option.
//
// Set with environment variable SUI_XRAY_ON_DEMAND=true|1|yes
// or create an empty marker file next to the DB: <db-folder>/.xray_on_demand
func IsXrayOnDemand() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SUI_XRAY_ON_DEMAND"))) {
	case "1", "true", "yes", "on":
		return true
	}
	marker := filepath.Join(GetDBFolderPath(), ".xray_on_demand")
	if _, err := os.Stat(marker); err == nil {
		return true
	}
	return false
}

// IsControllerModeRequested keeps the legacy --full installation behavior
// while fresh unified installs remain clients. A persisted panel setting can
// still explicitly disable the controller after installation.
func IsControllerModeRequested() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SUI_CONTROLLER_MODE"))) {
	case "1", "true", "yes", "on":
		return true
	}
	marker := filepath.Join(GetDBFolderPath(), ".controller_mode")
	_, err := os.Stat(marker)
	return err == nil
}

func GetDBFolderPath() string {
	dbFolderPath := os.Getenv("SUI_DB_FOLDER")
	if dbFolderPath == "" {
		dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			// Cross-platform fallback path
			if runtime.GOOS == "windows" {
				return "C:\\Program Files\\s-ui\\db"
			}
			return "/usr/local/s-ui/db"
		}
		dbFolderPath = filepath.Join(dir, "db")
	}
	return dbFolderPath
}

func GetDBPath() string {
	return fmt.Sprintf("%s/%s.db", GetDBFolderPath(), GetName())
}

func GetBinFolderPath() string {
	binFolderPath := os.Getenv("SUI_BIN_FOLDER")
	if binFolderPath == "" {
		dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			if runtime.GOOS == "windows" {
				return "C:\\Program Files\\s-ui\\bin"
			}
			return "/usr/local/s-ui/bin"
		}
		binFolderPath = filepath.Join(dir, "bin")
	}
	return binFolderPath
}

func GetXrayPath() string {
	xrayPath := os.Getenv("SUI_XRAY_PATH")
	if xrayPath != "" {
		return xrayPath
	}
	name := "xray"
	if runtime.GOOS == "windows" {
		name = "xray.exe"
	}
	return filepath.Join(GetBinFolderPath(), name)
}

func GetXrayConfigPath() string {
	xrayConfigPath := os.Getenv("SUI_XRAY_CONFIG")
	if xrayConfigPath != "" {
		return xrayConfigPath
	}
	return filepath.Join(GetBinFolderPath(), "xray.json")
}
