package agent

type ResourceUsage struct {
	Used  uint64 `json:"used"`
	Total uint64 `json:"total"`
}

type NetworkUsage struct {
	Sent uint64 `json:"sent"`
	Recv uint64 `json:"recv"`
}

type LoadAverage struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

type CoreStatus struct {
	SingBoxRunning bool   `json:"singbox_running"`
	XrayRunning    bool   `json:"xray_running"`
	XrayVersion    string `json:"xray_version,omitempty"`
}

type PanelStatus struct {
	Installed        bool        `json:"installed"`
	Version          string      `json:"version,omitempty"`
	ControlAvailable bool        `json:"control_available"`
	ProtocolVersion  int         `json:"protocol_version,omitempty"`
	Capabilities     []string    `json:"capabilities,omitempty"`
	Cores            *CoreStatus `json:"cores,omitempty"`
}

// Report is the telemetry and capability payload agents push to the panel.
type Report struct {
	Hostname     string        `json:"hostname"`
	OS           string        `json:"os"`
	Arch         string        `json:"arch"`
	AgentVersion string        `json:"agent_version"`
	Uptime       uint64        `json:"uptime"`
	CPUPercent   float64       `json:"cpu_percent"`
	CPUCores     int           `json:"cpu_cores,omitempty"`
	Memory       ResourceUsage `json:"memory"`
	Swap         ResourceUsage `json:"swap,omitempty"`
	Disk         ResourceUsage `json:"disk"`
	Network      NetworkUsage  `json:"network"`
	// NetRate is bytes/sec estimated from consecutive samples on the agent side.
	NetRate      NetworkUsage `json:"net_rate,omitempty"`
	Load         LoadAverage  `json:"load"`
	ProcessCount int          `json:"process_count,omitempty"`
	IPv4         []string     `json:"ipv4,omitempty"`
	IPv6         []string     `json:"ipv6,omitempty"`
	Cores        CoreStatus   `json:"cores"`
	Panel        PanelStatus  `json:"panel,omitempty"`
	// ConnMode is set by the agent: "http" or "ws".
	ConnMode string `json:"conn_mode,omitempty"`
}

// HeartbeatResponse is returned after a successful report (HTTP or WS ack).
type HeartbeatResponse struct {
	ServerTime      int64 `json:"server_time"`
	IntervalSeconds int   `json:"interval_seconds,omitempty"`
}
