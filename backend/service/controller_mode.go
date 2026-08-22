package service

import (
	"strings"

	"github.com/Hhz0823/1s-ui/config"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util/common"
)

const (
	controllerModeKey      = "controllerMode"
	controllerModeAuto     = "auto"
	controllerModeEnabled  = "enabled"
	controllerModeDisabled = "disabled"

	ControllerProfileClient  = "client"
	ControllerProfileFull    = "full"
	ControllerProfileMonitor = "monitor"
)

type ControllerModeStatus struct {
	Mode           string `json:"mode"`
	Profile        string `json:"profile"`
	Enabled        bool   `json:"enabled"`
	MonitorOnly    bool   `json:"monitor_only"`
	CanControl     bool   `json:"can_control"`
	Configured     string `json:"configured"`
	Inherited      bool   `json:"inherited"`
	CanEnable      bool   `json:"can_enable"`
	AgentCount     int    `json:"agent_count"`
	CPUCores       int    `json:"cpu_cores"`
	MemoryBytes    uint64 `json:"memory_bytes"`
	MinCPUCores    int    `json:"min_cpu_cores"`
	MinMemoryBytes uint64 `json:"min_memory_bytes"`
}

func (s *SettingService) GetControllerModeStatus() (*ControllerModeStatus, error) {
	configured, err := s.getString(controllerModeKey)
	if err != nil {
		return nil, err
	}
	configured = strings.ToLower(strings.TrimSpace(configured))

	agentCount := 0
	if db := database.GetDB(); db != nil {
		var count int64
		if err := db.Model(&model.AgentNode{}).Count(&count).Error; err != nil {
			return nil, err
		}
		agentCount = int(count)
	}
	enrollmentHash, _ := s.getString(agentEnrollmentKey)
	legacyController := agentCount > 0 || len(strings.TrimSpace(enrollmentHash)) == 64 || config.IsControllerModeRequested()
	profile := ControllerProfileClient
	inherited := false
	switch configured {
	case ControllerProfileFull, controllerModeEnabled:
		profile = ControllerProfileFull
	case ControllerProfileMonitor:
		profile = ControllerProfileMonitor
	case ControllerProfileClient, controllerModeDisabled:
		profile = ControllerProfileClient
	default:
		configured = controllerModeAuto
		if legacyController {
			profile = ControllerProfileFull
			inherited = true
		}
	}
	enabled := profile != ControllerProfileClient
	cpuCount, memoryBytes := currentClusterCapacity()

	mode := "client"
	if enabled {
		mode = "controller"
	}
	return &ControllerModeStatus{
		Mode:           mode,
		Profile:        profile,
		Enabled:        enabled,
		MonitorOnly:    profile == ControllerProfileMonitor,
		CanControl:     profile == ControllerProfileFull,
		Configured:     configured,
		Inherited:      inherited,
		CanEnable:      meetsClusterRequirements(cpuCount, memoryBytes),
		AgentCount:     agentCount,
		CPUCores:       cpuCount,
		MemoryBytes:    memoryBytes,
		MinCPUCores:    MinClusterCPUCores,
		MinMemoryBytes: MinClusterMemBytes,
	}, nil
}

func (s *SettingService) IsControllerModeEnabled() (bool, error) {
	status, err := s.GetControllerModeStatus()
	if err != nil {
		return false, err
	}
	return status.Enabled, nil
}

func (s *SettingService) RequireControllerMode() error {
	enabled, err := s.IsControllerModeEnabled()
	if err != nil {
		return err
	}
	if !enabled {
		return common.NewError("controller mode is disabled; enable it in panel settings first")
	}
	return nil
}

func (s *SettingService) RequireControllerControl() error {
	status, err := s.GetControllerModeStatus()
	if err != nil {
		return err
	}
	if !status.CanControl {
		return common.NewError("controller profile does not allow remote control")
	}
	return nil
}

func (s *SettingService) SetControllerMode(enabled bool) (*ControllerModeStatus, error) {
	profile := ControllerProfileClient
	configured := controllerModeDisabled
	if enabled {
		profile = ControllerProfileFull
		configured = controllerModeEnabled
	}
	return s.setControllerProfile(profile, configured)
}

func (s *SettingService) SetControllerProfile(profile string) (*ControllerModeStatus, error) {
	profile = strings.ToLower(strings.TrimSpace(profile))
	switch profile {
	case ControllerProfileClient, ControllerProfileFull, ControllerProfileMonitor:
	default:
		return nil, common.NewError("controller profile must be client, full, or monitor")
	}
	return s.setControllerProfile(profile, profile)
}

func (s *SettingService) setControllerProfile(profile, configured string) (*ControllerModeStatus, error) {
	s.CloseAgentAddressPairing()
	if profile != ControllerProfileClient {
		cpuCount, memoryBytes := currentClusterCapacity()
		if !meetsClusterRequirements(cpuCount, memoryBytes) {
			return nil, common.NewErrorf(
				"controller mode requires at least %d CPU cores and 2 GiB memory; current host: %d CPU cores / %.2f GiB",
				MinClusterCPUCores,
				cpuCount,
				float64(memoryBytes)/(1024*1024*1024),
			)
		}
		if err := s.setString(controllerModeKey, configured); err != nil {
			return nil, err
		}
	} else {
		if err := s.setString(controllerModeKey, configured); err != nil {
			return nil, err
		}
		// Revoke reusable and outstanding one-time enrollment credentials while
		// preserving node records, names, and monitoring history for re-enable.
		if err := s.setString(agentEnrollmentKey, ""); err != nil {
			return nil, err
		}
		if db := database.GetDB(); db != nil {
			if err := db.Model(&model.AgentNode{}).Where("1 = 1").Updates(map[string]interface{}{
				"pair_code_hash":  "",
				"pair_expires_at": 0,
			}).Error; err != nil {
				return nil, err
			}
		}
	}
	invalidateHostRequirementsCache()
	return s.GetControllerModeStatus()
}
