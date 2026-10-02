package service

import (
	"encoding/json"
	"net/url"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/util/common"
)

// ProxyClientDevice is a device whose proxy client the controller can use.
type ProxyClientDevice struct {
	Id        uint   `json:"id"`
	Name      string `json:"name"`
	Online    bool   `json:"online"`
	Supported bool   `json:"supported"`
	OS        string `json:"os,omitempty"`
	Platform  string `json:"platform,omitempty"`
}

// ProxyClientDevices lists this panel and the managed servers, marking the
// ones whose panel has the proxy client.
func (s *ProxyClientService) ProxyClientDevices() ([]ProxyClientDevice, error) {
	devices := []ProxyClientDevice{{Id: 0, Name: localMonitorNode(false).Name, Online: true, Supported: true, OS: "local"}}
	nodes, err := (&AgentService{}).List()
	if err != nil {
		return devices, nil
	}
	for _, node := range nodes {
		devices = append(devices, ProxyClientDevice{
			Id: node.Id, Name: node.Name, Online: node.Online,
			Supported: hasCapability(node.Report.Panel.Capabilities, agent.CapabilityProxyClientV1),
			OS:        node.Report.OS, Platform: node.Report.Platform,
		})
	}
	return devices, nil
}

// CallProxyClient runs a client action on this panel (0) or on a managed
// server. The result is the action's JSON.
func (s *ProxyClientService) CallProxyClient(serverID uint, call ProxyClientCall) (json.RawMessage, error) {
	if serverID == 0 {
		result, err := s.Call(call)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	}
	agents := &AgentService{}
	view, err := agents.Get(serverID)
	if err != nil {
		return nil, common.NewError("server not found")
	}
	if view.Online && !hasCapability(view.Report.Panel.Capabilities, agent.CapabilityProxyClientV1) {
		return nil, common.NewError("the panel on this server has no proxy client yet; update it")
	}
	var response *agent.RPCResponse
	if call.Action == "state" {
		response, err = agents.DispatchRPC(serverID, agent.RPCMethodClientGet, map[string]interface{}{}, "proxy-client")
	} else {
		response, err = agents.DispatchRPC(serverID, agent.RPCMethodClientCall, call, "proxy-client")
	}
	if err != nil {
		return nil, err
	}
	return response.Payload, nil
}

// ProxyClientAppActions are what the monitor app may do with its key.
var ProxyClientAppActions = map[string]bool{
	"state": true, "exit": true, "select": true, "test": true, "subscription.update": true, "mode": true,
}

// RedactProxyClientResult removes what the monitor key must not read from a
// client result: subscription addresses (they carry account tokens) and the
// proxy port password. Results that are not a state pass through.
func RedactProxyClientResult(raw json.RawMessage) (json.RawMessage, error) {
	var probe map[string]json.RawMessage
	if json.Unmarshal(raw, &probe) != nil || probe["settings"] == nil || probe["subscriptions"] == nil {
		return raw, nil
	}
	var state ProxyClientState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	state.Settings.MixedPassword = ""
	for i := range state.Subscriptions {
		if parsed, err := url.Parse(state.Subscriptions[i].URL); err == nil && parsed.Host != "" {
			state.Subscriptions[i].URL = parsed.Scheme + "://" + parsed.Host + "/…"
		} else {
			state.Subscriptions[i].URL = ""
		}
	}
	return json.Marshal(state)
}

// SetMode turns the client on or off and changes its routing mode, without
// touching the rest of the settings.
func (s *ProxyClientService) SetMode(enabled *bool, mode string) (*ProxyClientState, error) {
	settings := loadProxyClientSettings(database.GetDB())
	if enabled != nil {
		settings.Enabled = *enabled
	}
	if mode != "" {
		settings.Mode = mode
	}
	return s.SaveSettings(settings)
}
