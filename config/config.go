package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
