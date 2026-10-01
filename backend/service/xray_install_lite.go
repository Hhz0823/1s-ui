//go:build openwrt_lite

package service

import "errors"

type XrayInstallService struct{}

type XrayInstallProgress struct {
	State   string `json:"state"`
	Version string `json:"version"`
	Message string `json:"message"`
	Started int64  `json:"started"`
}

type XrayInstallStatus struct {
	Supported    bool                `json:"supported"`
	CanInstall   bool                `json:"can_install"`
	Installed    bool                `json:"installed"`
	Disabled     bool                `json:"disabled"`
	OnDemand     bool                `json:"on_demand"`
	Running      bool                `json:"running"`
	HasInbounds  bool                `json:"has_inbounds"`
	LowResource  bool                `json:"low_resource"`
	ExclusiveRun bool                `json:"exclusive_run"`
	Architecture string              `json:"architecture"`
	Version      string              `json:"version"`
	Path         string              `json:"path"`
	Capability   string              `json:"capability"`
	CPUCores     int                 `json:"cpu_cores"`
	MemoryBytes  uint64              `json:"memory_bytes"`
	Install      XrayInstallProgress `json:"install"`
}

func (s *XrayInstallService) Status() XrayInstallStatus {
	return XrayInstallStatus{
		Disabled:   true,
		Capability: "OpenWrt Lite uses sing-box only",
		Install:    XrayInstallProgress{State: "idle"},
	}
}

func (s *XrayInstallService) StartInstall() (XrayInstallStatus, error) {
	return s.Status(), errors.New("OpenWrt Lite uses sing-box only")
}

func (s *XrayInstallService) SetEnabled(enabled bool) (XrayInstallStatus, error) {
	return s.Status(), errors.New("OpenWrt Lite uses sing-box only")
}

func (s *XrayInstallService) Uninstall() (XrayInstallStatus, error) {
	return s.Status(), errors.New("OpenWrt Lite uses sing-box only")
}

func isSingboxOnlyRuntime() bool { return true }

func switchToSingboxOnly() error { return nil }
