package agent

// Control protocol (Nezha/Komari-style): panel → agent commands, agent → results.
// All control requires an authenticated WebSocket session.

const (
	ProtocolVersion = 3

	MsgTypeReport        = "report"
	MsgTypePing          = "ping"
	MsgTypePong          = "pong"
	MsgTypeAck           = "ack"
	MsgTypeConfig        = "config"
	MsgTypeError         = "error"
	MsgTypeCommand       = "command"
	MsgTypeCommandResult = "command_result"
	MsgTypeRPCRequest    = "rpc_request"
	MsgTypeRPCResponse   = "rpc_response"

	// Interactive PTY terminal (Nezha-style).
	MsgTypeTerminalOpen   = "terminal_open"
	MsgTypeTerminalOpened = "terminal_opened"
	MsgTypeTerminalInput  = "terminal_input"
	MsgTypeTerminalOutput = "terminal_output"
	MsgTypeTerminalResize = "terminal_resize"
	MsgTypeTerminalClose  = "terminal_close"
	MsgTypeTerminalClosed = "terminal_closed"
)

const (
	CapabilityMetricsV1       = "metrics.v1"
	CapabilityLatencyV1       = "latency.v1"
	CapabilityInboundReadV1   = "inbounds.read.v1"
	CapabilityInboundWriteV1  = "inbounds.write.v1"
	CapabilityQuickAddV1      = "inbounds.quick_add.v1"
	CapabilityQuickAddNaiveV1 = "inbounds.quick_add.naive.v1"
	// VLESS quick add understands vless_variant (REALITY, XHTTP, Vision and
	// VLESS Encryption) and reality_server.
	CapabilityQuickAddVlessV2 = "inbounds.quick_add.vless.v2"
	// The reality-xhttp-vision variant: REALITY + XHTTP + Vision with VLESS
	// Encryption underneath.
	CapabilityQuickAddVlessXHTTPVision = "inbounds.quick_add.vless.xhttp_vision"
	// XHTTP downloads through a CDN domain (cdn_domain, cdn_port).
	CapabilityQuickAddVlessCDN = "inbounds.quick_add.vless.cdn"
	CapabilityRelayV1          = "relay.v1"
	CapabilityPortTrafficV1    = "port.traffic.v1"
	CapabilityPanelAccessV1    = "panel.access.v1"
	// SD-WAN uplinks: v2 = multi-protocol uplinks, diagnostics and tuning;
	// reality/hysteria2 are only advertised when the build supports them.
	CapabilitySdwanV2        = "sdwan.v2"
	CapabilitySdwanReality   = "sdwan.reality"
	CapabilitySdwanHysteria2 = "sdwan.hysteria2"
	// SOCKS5/HTTP proxy checks run on this server (probe.proxies).
	CapabilityProxyProbeV1 = "probe.proxy.v1"
	// probe.proxies also checks nodes from share links (type "node").
	CapabilityProxyProbeNodeV1 = "probe.node.v1"
	// nodes.link: a share link of one inbound, for node monitors.
	CapabilityNodeLinkV1 = "nodes.link.v1"
	// Speed test sessions for the monitor app (speedtest.start).
	CapabilitySpeedtestV1 = "speedtest.v1"
	// This server can run speed tests against another one (speedtest.run),
	// so a home relay can measure its line to a VPS.
	CapabilitySpeedtestClientV1 = "speedtest.client.v1"
	// The panel's PassWall-style proxy client (client.get, client.call).
	CapabilityProxyClientV1 = "client.v1"
)

const (
	RPCMethodCapabilities    = "capabilities.get"
	RPCMethodInboundList     = "inbounds.list"
	RPCMethodInboundEdit     = "inbounds.editor"
	RPCMethodInboundSave     = "inbounds.save"
	RPCMethodInboundQuickAdd = "inbounds.quick_add"
	RPCMethodRelayGet        = "relay.get"
	RPCMethodRelayCreate     = "relay.create"
	RPCMethodRelayDelete     = "relay.delete"
	RPCMethodRelayExport     = "relay.bitbrowser_export"
	RPCMethodPortTraffic     = "port.traffic"
	RPCMethodPanelAccess     = "panel.access"
	RPCMethodSdwanProvision  = "sdwan.provision"
	RPCMethodSdwanRemove     = "sdwan.remove"
	RPCMethodSdwanDiagnose   = "sdwan.diagnose"
	RPCMethodSdwanTune       = "sdwan.tune"
	RPCMethodProxyProbe      = "probe.proxies"
	RPCMethodSpeedtestStart  = "speedtest.start"
	RPCMethodNodeLink        = "nodes.link"
	RPCMethodSpeedtestRun    = "speedtest.run"
	RPCMethodClientGet       = "client.get"
	RPCMethodClientCall      = "client.call"
)

// Command types the panel may send to an online agent.
const (
	CmdReportNow      = "report_now"
	CmdSetInterval    = "set_interval"
	CmdRestartAgent   = "restart_agent"
	CmdRestartXray    = "restart_xray"
	CmdRestartSingBox = "restart_singbox"
	CmdExec           = "exec"
	CmdPing           = "ping"
)

// Command is a panel → agent control request.
type Command struct {
	ID   string                 `json:"id"`
	Type string                 `json:"type"`
	Args map[string]interface{} `json:"args,omitempty"`
}

// CommandResult is an agent → panel response.
type CommandResult struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	OK      bool   `json:"ok"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
	Code    int    `json:"code,omitempty"`
	Elapsed int64  `json:"elapsed_ms"`
}

type RPCRequest struct {
	ID      string         `json:"id"`
	Method  string         `json:"method"`
	Payload jsonRawMessage `json:"payload,omitempty"`
}

type RPCResponse struct {
	ID      string         `json:"id"`
	OK      bool           `json:"ok"`
	Payload jsonRawMessage `json:"payload,omitempty"`
	Error   string         `json:"error,omitempty"`
	Code    int            `json:"code,omitempty"`
}

// Wire envelope used on the agent WebSocket.
type WireEnvelope struct {
	Type    string         `json:"type"`
	Payload jsonRawMessage `json:"payload,omitempty"`
	Time    int64          `json:"time,omitempty"`
	// Flattened fields for simple messages (config/ack/command).
	ID              string                 `json:"id,omitempty"`
	Command         string                 `json:"command,omitempty"`
	Method          string                 `json:"method,omitempty"`
	Args            map[string]interface{} `json:"args,omitempty"`
	ServerTime      int64                  `json:"server_time,omitempty"`
	IntervalSeconds int                    `json:"interval_seconds,omitempty"`
	Error           string                 `json:"error,omitempty"`
	OK              bool                   `json:"ok,omitempty"`
	Output          string                 `json:"output,omitempty"`
	Code            int                    `json:"code,omitempty"`
	Elapsed         int64                  `json:"elapsed_ms,omitempty"`
}

// jsonRawMessage avoids an import cycle with encoding/json aliases in docs.
type jsonRawMessage = []byte
