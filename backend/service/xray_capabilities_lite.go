//go:build openwrt_lite

package service

type XrayCapability struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Supported   bool   `json:"supported"`
	Shareable   bool   `json:"shareable,omitempty"`
	Recommended bool   `json:"recommended,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

type XraySelfCheck struct {
	Healthy         bool             `json:"healthy"`
	Disabled        bool             `json:"disabled"`
	BinaryAvailable bool             `json:"binary_available"`
	Version         string           `json:"version,omitempty"`
	Path            string           `json:"path,omitempty"`
	ConfigValid     bool             `json:"config_valid"`
	HasInbounds     bool             `json:"has_inbounds"`
	Running         bool             `json:"running"`
	Error           string           `json:"error,omitempty"`
	Protocols       []XrayCapability `json:"protocols"`
	Transports      []XrayCapability `json:"transports"`
}

func (s *ConfigService) CheckXray() XraySelfCheck {
	return XraySelfCheck{Disabled: true, Error: "Xray-core is disabled in OpenWrt Lite build"}
}
