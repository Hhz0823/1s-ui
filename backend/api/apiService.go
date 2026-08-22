package api

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/config"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/service"
	"github.com/Hhz0823/1s-ui/util"
	"github.com/Hhz0823/1s-ui/util/common"

	"github.com/gin-gonic/gin"
)

type ApiService struct {
	service.SettingService
	service.UserService
	service.ConfigService
	service.ClientService
	service.TlsService
	service.InboundService
	service.OutboundService
	service.EndpointService
	service.ServicesService
	service.PanelService
	service.StatsService
	service.PortTrafficService
	service.ServerService
	service.AgentService
	service.ManagedAccessService
	service.ReverseProxyService
	service.UpdateService
	service.XrayInstallService
}

func (a *ApiService) GetVersion(c *gin.Context) {
	info, err := a.UpdateService.CheckVersion()
	jsonObj(c, info, err)
}

func (a *ApiService) UpdatePanel(c *gin.Context) {
	status, err := a.UpdateService.StartUpdate()
	jsonObj(c, status, err)
}

func (a *ApiService) GetXrayInstall(c *gin.Context) {
	jsonObj(c, a.XrayInstallService.Status(), nil)
}

func (a *ApiService) InstallXray(c *gin.Context) {
	status, err := a.XrayInstallService.StartInstall()
	jsonObj(c, status, err)
}

func (a *ApiService) SetXrayEnabled(c *gin.Context) {
	var request struct {
		Enabled bool `json:"enabled" form:"enabled"`
	}
	if err := c.ShouldBind(&request); err != nil {
		jsonObj(c, nil, err)
		return
	}
	status, err := a.XrayInstallService.SetEnabled(request.Enabled)
	jsonObj(c, status, err)
}

func (a *ApiService) UninstallXray(c *gin.Context) {
	status, err := a.XrayInstallService.Uninstall()
	jsonObj(c, status, err)
}

type createAgentRequest struct {
	Name string `json:"name"`
}

type updateAgentRequest struct {
	Name       string `json:"name"`
	PublicHost string `json:"public_host"`
}

type connectLocalAgentRequest struct {
	ConnectURL string `json:"connect_url"`
	Address    string `json:"address"`
	Key        string `json:"key"`
	PublicURL  string `json:"public_url"`
	Insecure   bool   `json:"insecure"`
}

type controllerModeRequest struct {
	Profile string `json:"profile" form:"profile"`
	Enabled *bool  `json:"enabled" form:"enabled"`
}

func (a *ApiService) GetAgents(c *gin.Context) {
	nodes, err := a.AgentService.List()
	jsonObj(c, nodes, err)
}

func (a *ApiService) GetPortTraffic(c *gin.Context) {
	result, err := a.PortTrafficService.GetPortTraffic()
	jsonObj(c, result, err)
}

func (a *ApiService) GetAgentPortTraffic(c *gin.Context) {
	id, err := parseAgentNodeID(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	response, err := a.AgentService.DispatchRPC(id, agent.RPCMethodPortTraffic, map[string]interface{}{}, GetLoginUser(c))
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	var result service.PortTrafficResponse
	err = json.Unmarshal(response.Payload, &result)
	jsonObj(c, result, err)
}

func (a *ApiService) ConnectLocalAgent(c *gin.Context) {
	var request connectLocalAgentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		jsonObj(c, nil, err)
		return
	}
	connectURL := strings.TrimSpace(request.ConnectURL)
	if connectURL == "" {
		connectURL = strings.TrimSpace(request.Address) + "#" + strings.TrimSpace(request.Key)
	}
	result, err := a.AgentService.ConnectLocalController(connectURL, request.Insecure, request.PublicURL)
	jsonObj(c, result, err)
}

func (a *ApiService) CreateAgentPanelAccess(c *gin.Context) {
	if err := a.SettingService.RequireControllerControl(); err != nil {
		jsonObj(c, nil, err)
		return
	}
	id, err := parseAgentNodeID(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	node, err := a.AgentService.Get(id)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	if !node.Managed || !node.Controllable {
		jsonObj(c, nil, common.NewError("managed server panel is offline or unavailable"))
		return
	}
	panelURL, err := service.NormalizePanelURL(node.Report.Panel.PublicURL)
	if err != nil {
		jsonObj(c, nil, common.NewError("managed server has not reported a public panel address"))
		return
	}
	response, err := a.AgentService.DispatchRPC(id, agent.RPCMethodPanelAccess, service.ManagedPanelAccessRequest{Actor: GetLoginUser(c)}, GetLoginUser(c))
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	var grant service.ManagedPanelAccessGrant
	if err := json.Unmarshal(response.Payload, &grant); err != nil || strings.TrimSpace(grant.Token) == "" || grant.ExpiresAt <= time.Now().Unix() {
		jsonObj(c, nil, common.NewError("managed server returned an invalid panel access grant"))
		return
	}
	jsonObj(c, map[string]interface{}{"panel_url": panelURL, "token": grant.Token, "expires_at": grant.ExpiresAt}, nil)
}

func (a *ApiService) GetLocalAgentConnection(c *gin.Context) {
	status, err := a.AgentService.GetLocalControllerStatus()
	jsonObj(c, status, err)
}

func (a *ApiService) DisconnectLocalAgent(c *gin.Context) {
	status, err := a.AgentService.DisconnectLocalController()
	jsonObj(c, status, err)
}

func (a *ApiService) GetControllerMode(c *gin.Context) {
	status, err := a.SettingService.GetControllerModeStatus()
	jsonObj(c, status, err)
}

func (a *ApiService) SetControllerMode(c *gin.Context) {
	var request controllerModeRequest
	if err := c.ShouldBind(&request); err != nil {
		jsonObj(c, nil, err)
		return
	}
	var status *service.ControllerModeStatus
	var err error
	if strings.TrimSpace(request.Profile) != "" {
		status, err = a.SettingService.SetControllerProfile(request.Profile)
	} else if request.Enabled != nil {
		status, err = a.SettingService.SetControllerMode(*request.Enabled)
	} else {
		err = common.NewError("profile or enabled is required")
	}
	if err == nil && status.Profile == service.ControllerProfileClient {
		a.AgentService.DisconnectAll("controller mode disabled")
	} else if err == nil && status.MonitorOnly {
		a.AgentService.CloseAllTerminals("controller switched to monitor profile")
	}
	jsonObj(c, status, err)
}

func (a *ApiService) GetAgent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		jsonObj(c, nil, common.NewError("invalid agent node id"))
		return
	}
	node, err := a.AgentService.Get(uint(id))
	jsonObj(c, node, err)
}

func (a *ApiService) UpdateAgent(c *gin.Context) {
	if err := a.SettingService.RequireControllerControl(); err != nil {
		jsonObj(c, nil, err)
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		jsonObj(c, nil, common.NewError("invalid agent node id"))
		return
	}
	var request updateAgentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		jsonObj(c, nil, err)
		return
	}
	node, err := a.AgentService.Update(uint(id), request.Name, request.PublicHost)
	jsonObj(c, node, err)
}

func (a *ApiService) GetAgentInbounds(c *gin.Context) {
	id, err := parseAgentNodeID(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	response, err := a.AgentService.DispatchRPC(id, agent.RPCMethodInboundList, map[string]interface{}{}, GetLoginUser(c))
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	var result service.RemoteInboundList
	err = json.Unmarshal(response.Payload, &result)
	jsonObj(c, result, err)
}

func (a *ApiService) GetAgentInboundEditor(c *gin.Context) {
	id, err := parseAgentNodeID(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	inboundID, err := strconv.ParseUint(c.Query("id"), 10, 32)
	if c.Query("id") == "" {
		inboundID = 0
		err = nil
	}
	if err != nil {
		jsonObj(c, nil, common.NewError("invalid inbound id"))
		return
	}
	response, err := a.AgentService.DispatchRPC(id, agent.RPCMethodInboundEdit, map[string]interface{}{"id": inboundID}, GetLoginUser(c))
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	var result service.RemoteInboundEditor
	err = json.Unmarshal(response.Payload, &result)
	jsonObj(c, result, err)
}

type saveAgentInboundRequest struct {
	Action           string          `json:"action"`
	Data             json.RawMessage `json:"data"`
	InitUsers        []uint          `json:"init_users"`
	ExpectedRevision uint64          `json:"expected_revision"`
}

func (a *ApiService) SaveAgentInbound(c *gin.Context) {
	id, err := parseAgentNodeID(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	var request saveAgentInboundRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 512<<10)
	if err := c.ShouldBindJSON(&request); err != nil {
		jsonObj(c, nil, err)
		return
	}
	node, err := a.AgentService.Get(id)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	publicHost := managedNodePublicHost(node)
	payload := service.RemoteInboundSaveRequest{
		Action: request.Action, Data: request.Data, InitUsers: request.InitUsers,
		ExpectedRevision: request.ExpectedRevision, Actor: GetLoginUser(c), PublicHost: publicHost,
	}
	response, err := a.AgentService.DispatchRPC(id, agent.RPCMethodInboundSave, payload, GetLoginUser(c))
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	var result service.RemoteInboundSaveResponse
	err = json.Unmarshal(response.Payload, &result)
	jsonObj(c, result, err)
}

func (a *ApiService) QuickAddAgentInbounds(c *gin.Context) {
	id, err := parseAgentNodeID(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	var request service.RemoteQuickAddRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	if err = c.ShouldBindJSON(&request); err != nil {
		jsonObj(c, nil, err)
		return
	}
	node, err := a.AgentService.Get(id)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	request.Actor = GetLoginUser(c)
	request.PublicHost = managedNodePublicHost(node)
	response, err := a.AgentService.DispatchRPC(id, agent.RPCMethodInboundQuickAdd, request, request.Actor)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	var result service.RemoteQuickAddResponse
	err = json.Unmarshal(response.Payload, &result)
	jsonObj(c, result, err)
}

func (a *ApiService) GetAgentRelayData(c *gin.Context) {
	id, err := parseAgentNodeID(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	response, err := a.AgentService.DispatchRPC(id, agent.RPCMethodRelayGet, map[string]interface{}{}, GetLoginUser(c))
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	var result service.RelayData
	err = json.Unmarshal(response.Payload, &result)
	jsonObj(c, result, err)
}

func (a *ApiService) CreateAgentRelay(c *gin.Context) {
	id, err := parseAgentNodeID(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	var request service.RelayCreateRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 512<<10)
	if err = c.ShouldBindJSON(&request); err != nil {
		jsonObj(c, nil, err)
		return
	}
	node, err := a.AgentService.Get(id)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	actor := GetLoginUser(c)
	payload := service.RemoteRelayCreateRequest{
		Request: request, Actor: actor, PublicHost: managedNodePublicHost(node),
	}
	response, err := a.AgentService.DispatchRPC(id, agent.RPCMethodRelayCreate, payload, actor)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	var result map[string]interface{}
	err = json.Unmarshal(response.Payload, &result)
	jsonObj(c, result, err)
}

func (a *ApiService) DeleteAgentRelay(c *gin.Context) {
	id, err := parseAgentNodeID(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	relayID, err := parseAgentRelayID(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	actor := GetLoginUser(c)
	response, err := a.AgentService.DispatchRPC(id, agent.RPCMethodRelayDelete, service.RemoteRelayDeleteRequest{ID: relayID, Actor: actor}, actor)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	var result map[string]bool
	err = json.Unmarshal(response.Payload, &result)
	jsonObj(c, result, err)
}

func (a *ApiService) ExportAgentRelayBitBrowser(c *gin.Context) {
	id, err := parseAgentNodeID(c)
	if err != nil {
		c.String(http.StatusBadRequest, "%s", err.Error())
		return
	}
	relayID, err := parseAgentRelayID(c)
	if err != nil {
		c.String(http.StatusBadRequest, "%s", err.Error())
		return
	}
	response, err := a.AgentService.DispatchRPC(id, agent.RPCMethodRelayExport, service.RemoteRelayExportRequest{ID: relayID}, GetLoginUser(c))
	if err != nil {
		c.String(http.StatusBadRequest, "%s", err.Error())
		return
	}
	var result service.RemoteRelayExportResponse
	if err = json.Unmarshal(response.Payload, &result); err != nil {
		c.String(http.StatusBadGateway, "%s", err.Error())
		return
	}
	data, err := base64.StdEncoding.DecodeString(result.ContentBase64)
	if err != nil {
		c.String(http.StatusBadGateway, "invalid managed relay export")
		return
	}
	c.Header("Content-Disposition", "attachment; filename=1s-ui-bitbrowser-relay-"+strconv.FormatUint(uint64(relayID), 10)+".xlsx")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

func parseAgentRelayID(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("relayId"), 10, 32)
	if err != nil || id == 0 {
		return 0, common.NewError("invalid relay pool id")
	}
	return uint(id), nil
}

func parseAgentNodeID(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		return 0, common.NewError("invalid agent node id")
	}
	return uint(id), nil
}

func managedNodePublicHost(node *service.AgentNodeView) string {
	if node == nil {
		return ""
	}
	if value := strings.TrimSpace(node.PublicHost); value != "" {
		return strings.Trim(value, "[]")
	}
	candidates := append([]string{node.RemoteIP}, node.Report.IPv4...)
	candidates = append(candidates, node.Report.IPv6...)
	for _, candidate := range candidates {
		candidate = strings.Trim(strings.TrimSpace(candidate), "[]")
		ip := net.ParseIP(candidate)
		if ip != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() {
			return candidate
		}
	}
	for _, candidate := range candidates {
		candidate = strings.Trim(strings.TrimSpace(candidate), "[]")
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

type agentCommandRequest struct {
	Type string                 `json:"type"`
	Args map[string]interface{} `json:"args"`
}

func (a *ApiService) ControlAgent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		jsonObj(c, nil, common.NewError("invalid agent node id"))
		return
	}
	var req agentCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonObj(c, nil, err)
		return
	}
	if strings.TrimSpace(req.Type) == "" {
		jsonObj(c, nil, common.NewError("command type is required"))
		return
	}
	result, err := a.AgentService.DispatchCommand(uint(id), strings.TrimSpace(req.Type), req.Args, GetLoginUser(c))
	jsonObj(c, result, err)
}

func (a *ApiService) ListAgentCommands(c *gin.Context) {
	jsonObj(c, a.AgentService.ListCommands(), nil)
}

type agentBatchCommandRequest struct {
	IDs  []uint                 `json:"ids"`
	Type string                 `json:"type"`
	Args map[string]interface{} `json:"args"`
}

func (a *ApiService) ControlAgentsBatch(c *gin.Context) {
	var req agentBatchCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonObj(c, nil, err)
		return
	}
	if strings.TrimSpace(req.Type) == "" {
		jsonObj(c, nil, common.NewError("command type is required"))
		return
	}
	if len(req.IDs) == 0 {
		jsonObj(c, nil, common.NewError("ids is required"))
		return
	}
	results, err := a.AgentService.DispatchBatch(req.IDs, strings.TrimSpace(req.Type), req.Args, GetLoginUser(c))
	jsonObj(c, results, err)
}

func (a *ApiService) CreateAgent(c *gin.Context) {
	if err := a.SettingService.RequireControllerMode(); err != nil {
		jsonObj(c, nil, err)
		return
	}
	var request createAgentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		jsonObj(c, nil, err)
		return
	}
	enrollment, err := a.AgentService.Create(request.Name)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	jsonObj(c, a.agentEnrollmentResponse(c, enrollment), nil)
}

func (a *ApiService) CreateAgentEnrollmentLink(c *gin.Context) {
	if err := a.SettingService.RequireControllerMode(); err != nil {
		jsonObj(c, nil, err)
		return
	}
	webPath, err := a.SettingService.GetWebPath()
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	panelURL := panelURLForRequest(c, webPath)
	expiresAt := a.SettingService.OpenAgentAddressPairing(0)
	result := agentConnectionResponse(panelURL)
	result["simple_address"] = panelURL
	result["pair_expires_at"] = expiresAt
	jsonObj(c, result, nil)
}

func (a *ApiService) RotateAgent(c *gin.Context) {
	if err := a.SettingService.RequireControllerControl(); err != nil {
		jsonObj(c, nil, err)
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		jsonObj(c, nil, common.NewError("invalid agent node id"))
		return
	}
	if _, err := a.AgentService.Get(uint(id)); err != nil {
		jsonObj(c, nil, err)
		return
	}
	webPath, err := a.SettingService.GetWebPath()
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	panelURL := panelURLForRequest(c, webPath)
	expiresAt := a.SettingService.OpenAgentAddressPairing(uint(id))
	result := agentConnectionResponse(panelURL)
	result["simple_address"] = panelURL
	result["pair_expires_at"] = expiresAt
	jsonObj(c, result, nil)
}

func (a *ApiService) DeleteAgent(c *gin.Context) {
	if err := a.SettingService.RequireControllerControl(); err != nil {
		jsonObj(c, nil, err)
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		jsonObj(c, nil, common.NewError("invalid agent node id"))
		return
	}
	jsonMsg(c, "agents", a.AgentService.Delete(uint(id)))
}

func (a *ApiService) agentEnrollmentResponse(c *gin.Context, enrollment *service.AgentEnrollment) map[string]interface{} {
	webPath, _ := a.SettingService.GetWebPath()
	panelURL := panelURLForRequest(c, webPath)
	pairURL := strings.TrimRight(panelURL, "/") + "/agent/v1/pair#" + enrollment.PairCode
	command := "bash <(curl -fsSL https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install-agent.sh) --connect " +
		shellQuote(pairURL) + " --version " + shellQuote(config.GetVersion())
	managedCommand := managedPanelInstallCommand(pairURL)
	legacyCommand := "bash <(curl -fsSL https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install-agent.sh) --panel " +
		shellQuote(panelURL) + " --token " + shellQuote(enrollment.Token) + " --version " + shellQuote(config.GetVersion())
	legacyManagedCommand := "bash <(curl -fsSL https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh) --managed-client -y --controller " +
		shellQuote(panelURL) + " --agent-token " + shellQuote(enrollment.Token) + " " + shellQuote(config.GetVersion())
	return map[string]interface{}{
		"node": enrollment.Node, "token": enrollment.Token,
		"panel_url": panelURL, "connect_url": pairURL, "pair_url": pairURL, "pair_expires_at": enrollment.PairExpiresAt,
		"command": command, "managed_command": managedCommand,
		"legacy_command": legacyCommand, "legacy_managed_command": legacyManagedCommand,
	}
}

func agentConnectionResponse(connectURL string) map[string]interface{} {
	command := "bash <(curl -fsSL https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install-agent.sh) --connect " +
		shellQuote(connectURL) + " --version " + shellQuote(config.GetVersion())
	managedCommand := managedPanelInstallCommand(connectURL)
	return map[string]interface{}{
		"connect_url": connectURL,
		"command":     command, "managed_command": managedCommand,
	}
}

func managedPanelInstallCommand(_ string) string {
	return "bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh)"
}

func panelURLForRequest(c *gin.Context, webPath string) string {
	scheme := "http"
	if requestIsHTTPS(c) {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host + webPath
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func (a *ApiService) LoadData(c *gin.Context) {
	data, err := a.getData(c)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, data, nil)
}

func (a *ApiService) getData(c *gin.Context) (interface{}, error) {
	data := make(map[string]interface{}, 0)
	serverTime := time.Now().UnixMilli()
	lu := c.Query("lu")
	isUpdated, err := a.ConfigService.CheckChanges(lu)
	if err != nil {
		return "", err
	}
	onlines, err := a.StatsService.GetOnlines()

	sysInfo := a.ServerService.GetSingboxInfo()
	if sysInfo["running"] == false {
		logs := a.ServerService.GetLogs("1", "error")
		if len(logs) > 0 {
			data["lastLog"] = logs[0]
		}
	}

	if err != nil {
		return "", err
	}
	data["lastUpdate"] = serverTime
	// Always expose host capacity check for the web UI banner.
	data["hostRequirements"] = a.ServerService.GetHostRequirements()
	controllerMode, err := a.SettingService.GetControllerModeStatus()
	if err != nil {
		return "", err
	}
	data["controllerMode"] = controllerMode
	if isUpdated {
		config, err := a.SettingService.GetConfig()
		if err != nil {
			return "", err
		}
		clients, err := a.ClientService.GetAll()
		if err != nil {
			return "", err
		}
		tlsConfigs, err := a.TlsService.GetAll()
		if err != nil {
			return "", err
		}
		inbounds, err := a.InboundService.GetAll()
		if err != nil {
			return "", err
		}
		outbounds, err := a.OutboundService.GetAll()
		if err != nil {
			return "", err
		}
		endpoints, err := a.EndpointService.GetAll()
		if err != nil {
			return "", err
		}
		services, err := a.ServicesService.GetAll()
		if err != nil {
			return "", err
		}
		subURI, err := a.SettingService.GetFinalSubURI(getHostname(c))
		if err != nil {
			return "", err
		}
		trafficAge, err := a.SettingService.GetTrafficAge()
		if err != nil {
			return "", err
		}
		data["config"] = json.RawMessage(config)
		data["clients"] = clients
		data["tls"] = tlsConfigs
		data["inbounds"] = inbounds
		data["outbounds"] = outbounds
		data["endpoints"] = endpoints
		data["services"] = services
		data["subURI"] = subURI
		data["enableTraffic"] = trafficAge > 0
		data["onlines"] = onlines
	} else {
		data["onlines"] = onlines
	}

	return data, nil
}

func (a *ApiService) LoadPartialData(c *gin.Context, objs []string) error {
	data := make(map[string]interface{}, 0)
	id := c.Query("id")

	for _, obj := range objs {
		switch obj {
		case "inbounds":
			inbounds, err := a.InboundService.Get(id)
			if err != nil {
				return err
			}
			data[obj] = inbounds
		case "outbounds":
			outbounds, err := a.OutboundService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = outbounds
		case "endpoints":
			endpoints, err := a.EndpointService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = endpoints
		case "services":
			services, err := a.ServicesService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = services
		case "tls":
			tlsConfigs, err := a.TlsService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = tlsConfigs
		case "clients":
			clients, err := a.ClientService.Get(id)
			if err != nil {
				return err
			}
			data[obj] = clients
		case "config":
			config, err := a.SettingService.GetConfig()
			if err != nil {
				return err
			}
			data[obj] = json.RawMessage(config)
		case "settings":
			settings, err := a.SettingService.GetAllSetting()
			if err != nil {
				return err
			}
			data[obj] = settings
		}
	}

	jsonObj(c, data, nil)
	return nil
}

func (a *ApiService) GetUsers(c *gin.Context) {
	users, err := a.UserService.GetUsers()
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, *users, nil)
}

func (a *ApiService) GetSettings(c *gin.Context) {
	data, err := a.SettingService.GetAllSetting()
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, data, err)
}

func (a *ApiService) GetStats(c *gin.Context) {
	resource := c.Query("resource")
	tag := c.Query("tag")
	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		limit = 100
	}
	start, _ := strconv.ParseInt(c.Query("start"), 10, 64)
	end, _ := strconv.ParseInt(c.Query("end"), 10, 64)
	data, err := a.StatsService.GetStats(resource, tag, limit, start, end)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, data, err)
}

func (a *ApiService) GetUserTraffic(c *gin.Context) {
	start, _ := strconv.ParseInt(c.Query("start"), 10, 64)
	end, _ := strconv.ParseInt(c.Query("end"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	data, err := a.StatsService.GetUserTrafficRanking(start, end, limit)
	jsonObj(c, data, err)
}

func (a *ApiService) GetStatus(c *gin.Context) {
	request := c.Query("r")
	result := a.ServerService.GetStatus(request)
	jsonObj(c, result, nil)
}

func (a *ApiService) GetOnlines(c *gin.Context) {
	onlines, err := a.StatsService.GetOnlines()
	jsonObj(c, onlines, err)
}

func (a *ApiService) GetLogs(c *gin.Context) {
	count := c.Query("c")
	level := c.Query("l")
	logs := a.ServerService.GetLogs(count, level)
	jsonObj(c, logs, nil)
}

func (a *ApiService) CheckChanges(c *gin.Context) {
	actor := c.Query("a")
	chngKey := c.Query("k")
	count := c.Query("c")
	changes := a.ConfigService.GetChanges(actor, chngKey, count)
	jsonObj(c, changes, nil)
}

func (a *ApiService) GetKeypairs(c *gin.Context) {
	kType := c.Query("k")
	options := c.Query("o")
	keypair := a.ServerService.GenKeypair(kType, options)
	jsonObj(c, keypair, nil)
}

func (a *ApiService) GetDb(c *gin.Context) {
	exclude := c.Query("exclude")
	db, err := database.GetDb(exclude)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=s-ui_"+time.Now().Format("20060102-150405")+".db")
	c.Writer.Write(db)
}

func (a *ApiService) postActions(c *gin.Context) (string, json.RawMessage, error) {
	var data map[string]json.RawMessage
	err := c.ShouldBind(&data)
	if err != nil {
		return "", nil, err
	}
	return string(data["action"]), data["data"], nil
}

func (a *ApiService) Login(c *gin.Context) {
	needsSetup, err := a.UserService.NeedsSetup()
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	if needsSetup {
		jsonMsg(c, "", common.NewError("panel setup is required"))
		return
	}
	remoteIP := getRemoteIp(c)
	loginUser, err := a.UserService.Login(c.Request.FormValue("user"), c.Request.FormValue("pass"), remoteIP)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}

	sessionMaxAge, err := a.SettingService.GetSessionMaxAge()
	if err != nil {
		logger.Infof("Unable to get session's max age from DB")
	}

	err = SetLoginUser(c, loginUser, sessionMaxAge)
	if err == nil {
		logger.Info("user ", loginUser, " login success")
	} else {
		logger.Warning("login failed: ", err)
	}

	jsonMsg(c, "", nil)
}

func (a *ApiService) SetupStatus(c *gin.Context) {
	required, err := a.UserService.NeedsSetup()
	jsonObj(c, map[string]bool{"required": required, "authenticated": IsLogin(c)}, err)
}

func (a *ApiService) Setup(c *gin.Context) {
	if !requestOriginAllowed(c) {
		jsonMsg(c, "", common.NewError("cross-site setup request rejected"))
		return
	}
	password := c.Request.FormValue("password")
	if password != c.Request.FormValue("confirmPassword") {
		jsonMsg(c, "", common.NewError("password confirmation does not match"))
		return
	}
	username, err := a.UserService.InitializeAdmin(c.Request.FormValue("username"), password)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	sessionMaxAge, err := a.SettingService.GetSessionMaxAge()
	if err != nil {
		logger.Infof("Unable to get session's max age from DB")
	}
	if err = SetLoginUser(c, username, sessionMaxAge); err != nil {
		jsonMsg(c, "", err)
		return
	}
	logger.Info("initial administrator created")
	jsonMsg(c, "", nil)
}

func (a *ApiService) ManagedLogin(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Referrer-Policy", "no-referrer")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	webPath, err := a.SettingService.GetWebPath()
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}
	loginPath := strings.TrimRight(webPath, "/") + "/login?managed=failed"
	username, err := a.ManagedAccessService.Consume(strings.TrimSpace(c.PostForm("token")))
	if err != nil {
		c.Redirect(http.StatusSeeOther, loginPath)
		return
	}
	sessionMaxAge, err := a.SettingService.GetSessionMaxAge()
	if err != nil {
		logger.Infof("Unable to get session's max age from DB")
	}
	if err := SetLoginUser(c, username, sessionMaxAge); err != nil {
		c.Redirect(http.StatusSeeOther, loginPath)
		return
	}
	c.Redirect(http.StatusSeeOther, webPath)
}

func (a *ApiService) ChangePass(c *gin.Context) {
	id := c.Request.FormValue("id")
	oldPass := c.Request.FormValue("oldPass")
	newUsername := c.Request.FormValue("newUsername")
	newPass := c.Request.FormValue("newPass")
	err := a.UserService.ChangePass(id, oldPass, newUsername, newPass)
	if err == nil {
		logger.Info("change user credentials success")
		jsonMsg(c, "save", nil)
	} else {
		logger.Warning("change user credentials failed:", err)
		jsonMsg(c, "", err)
	}
}

func (a *ApiService) Save(c *gin.Context, loginUser string) {
	hostname := getHostname(c)
	obj := c.Request.FormValue("object")
	act := c.Request.FormValue("action")
	data := c.Request.FormValue("data")
	initUsers := c.Request.FormValue("initUsers")
	objs, err := a.ConfigService.Save(obj, act, json.RawMessage(data), initUsers, loginUser, hostname)
	if err != nil {
		jsonMsg(c, "save", err)
		return
	}
	err = a.LoadPartialData(c, objs)
	if err != nil {
		jsonMsg(c, obj, err)
	}
}

func (a *ApiService) RestartApp(c *gin.Context) {
	err := a.PanelService.RestartPanel(3)
	jsonMsg(c, "restartApp", err)
}

func (a *ApiService) RestartSb(c *gin.Context) {
	err := a.ConfigService.RestartCore()
	jsonMsg(c, "restartSb", err)
}

func (a *ApiService) ResetTraffic(c *gin.Context) {
	if err := a.ClientService.ResetAllClientsTraffic(); err != nil {
		jsonMsg(c, "resetTraffic", err)
		return
	}
	err := a.ConfigService.RestartCore()
	jsonMsg(c, "resetTraffic", err)
}

func (a *ApiService) RestartXray(c *gin.Context) {
	err := a.ConfigService.RestartXrayCore()
	jsonMsg(c, "restartXray", err)
}

func (a *ApiService) StopXray(c *gin.Context) {
	err := a.ConfigService.StopXrayCore()
	jsonMsg(c, "stopXray", err)
}

func (a *ApiService) LinkConvert(c *gin.Context) {
	link := c.Request.FormValue("link")
	result, _, err := util.GetOutbound(link, 0)
	jsonObj(c, result, err)
}

func (a *ApiService) SubConvert(c *gin.Context) {
	link := c.Request.FormValue("link")
	result, err := util.GetExternalSub(link)
	jsonObj(c, result, err)
}

func (a *ApiService) ImportDb(c *gin.Context) {
	file, _, err := c.Request.FormFile("db")
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	defer file.Close()
	err = database.ImportDB(file)
	jsonMsg(c, "", err)
}

func (a *ApiService) Logout(c *gin.Context) {
	loginUser := GetLoginUser(c)
	if loginUser != "" {
		logger.Infof("user %s logout", loginUser)
	}
	ClearSession(c)
	jsonMsg(c, "", nil)
}

func (a *ApiService) LoadTokens() ([]byte, error) {
	return a.UserService.LoadTokens()
}

func (a *ApiService) GetTokens(c *gin.Context) {
	loginUser := GetLoginUser(c)
	tokens, err := a.UserService.GetUserTokens(loginUser)
	jsonObj(c, tokens, err)
}

func (a *ApiService) AddToken(c *gin.Context) {
	loginUser := GetLoginUser(c)
	expiry := c.Request.FormValue("expiry")
	expiryInt, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	desc := c.Request.FormValue("desc")
	token, err := a.UserService.AddToken(loginUser, expiryInt, desc)
	jsonObj(c, token, err)
}

func (a *ApiService) DeleteToken(c *gin.Context) {
	tokenId := c.Request.FormValue("id")
	err := a.UserService.DeleteToken(tokenId)
	jsonMsg(c, "", err)
}

func (a *ApiService) GetSingboxConfig(c *gin.Context) {
	rawConfig, err := a.ConfigService.GetConfig("")
	if err != nil {
		c.Status(400)
		c.Writer.WriteString(err.Error())
		return
	}
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=config_"+time.Now().Format("20060102-150405")+".json")
	c.Writer.Write(*rawConfig)
}

func (a *ApiService) GetXrayConfig(c *gin.Context) {
	rawConfig, err := a.ConfigService.GetXrayConfig()
	if err != nil {
		c.Status(400)
		c.Writer.WriteString(err.Error())
		return
	}
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=xray_config_"+time.Now().Format("20060102-150405")+".json")
	c.Writer.Write(*rawConfig)
}

func (a *ApiService) GetCheckXray(c *gin.Context) {
	jsonObj(c, a.ConfigService.CheckXray(), nil)
}

func (a *ApiService) GetCheckOutbound(c *gin.Context) {
	tag := c.Query("tag")
	link := c.Query("link")
	result := a.ConfigService.CheckOutbound(tag, link)
	jsonObj(c, result, nil)
}

func (a *ApiService) GetCheckWarp(c *gin.Context) {
	tag := c.Query("tag")
	link := c.Query("link")
	result := a.ConfigService.CheckWarp(tag, link)
	jsonObj(c, result, nil)
}

func (a *ApiService) GetRelayData(c *gin.Context) {
	data, err := a.ConfigService.GetRelayData()
	jsonObj(c, data, err)
}

func (a *ApiService) ExportRelayBitBrowser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.String(http.StatusBadRequest, "invalid relay pool id")
		return
	}
	data, err := a.ConfigService.GetRelayBitBrowserExport(uint(id))
	if err != nil {
		c.String(http.StatusBadRequest, "%s", err.Error())
		return
	}
	c.Header("Content-Disposition", "attachment; filename=1s-ui-bitbrowser-relay-"+strconv.FormatUint(id, 10)+".xlsx")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

func (a *ApiService) CreateRelay(c *gin.Context, loginUser string) {
	var req service.RelayCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, "relay", err)
		return
	}
	pool, err := a.ConfigService.CreateRelay(req, loginUser, getHostname(c))
	if err != nil {
		jsonMsg(c, "relay", err)
		return
	}
	jsonObj(c, pool, nil)
}

func (a *ApiService) DeleteRelay(c *gin.Context, loginUser string) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		jsonMsg(c, "relay", common.NewError("invalid relay pool id"))
		return
	}
	err = a.ConfigService.DeleteRelay(uint(id), loginUser)
	jsonMsg(c, "relay", err)
}

type PinnedSha256Request struct {
	Cert       string `json:"cert"`
	CertPath   string `json:"certPath"`
	ServerName string `json:"serverName"`
}

func (a *ApiService) PinnedSha256(c *gin.Context) {
	var req PinnedSha256Request
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, "pinnedSha256", err)
		return
	}
	var certPEM []byte
	if req.CertPath != "" {
		data, err := os.ReadFile(req.CertPath)
		if err != nil {
			jsonMsg(c, "pinnedSha256", common.NewError("failed to read certificate: ", err.Error()))
			return
		}
		certPEM = data
	} else if req.Cert != "" {
		certPEM = []byte(req.Cert)
	} else if req.ServerName != "" {
		addr := req.ServerName
		if !strings.Contains(addr, ":") {
			addr = addr + ":443"
		}
		conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			jsonMsg(c, "pinnedSha256", common.NewError("failed to connect: ", err.Error()))
			return
		}
		defer conn.Close()
		certs := conn.ConnectionState().PeerCertificates
		if len(certs) == 0 {
			jsonMsg(c, "pinnedSha256", common.NewError("no peer certificates found"))
			return
		}
		certPEM = certs[0].Raw
	}
	if len(certPEM) == 0 {
		jsonMsg(c, "pinnedSha256", common.NewError("no certificate provided"))
		return
	}
	block, _ := pem.Decode(certPEM)
	var derBytes []byte
	if block != nil {
		derBytes = block.Bytes
	} else {
		derBytes = certPEM
	}
	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		jsonMsg(c, "pinnedSha256", common.NewError("failed to parse certificate: ", err.Error()))
		return
	}
	hash := sha256.Sum256(cert.Raw)
	sha256Base64 := base64.StdEncoding.EncodeToString(hash[:])
	jsonObj(c, []string{sha256Base64}, nil)
}

type SysctlRequest struct {
	CongestionAlgo string `json:"congestionAlgo"`
	Qdisc          string `json:"qdisc"`
}

func (a *ApiService) SetSysctl(c *gin.Context) {
	if runtime.GOOS != "linux" {
		jsonMsg(c, "setSysctl", common.NewError("sysctl is only supported on Linux"))
		return
	}

	algo := strings.TrimSpace(c.PostForm("congestionAlgo"))
	qdisc := strings.TrimSpace(c.PostForm("qdisc"))

	validAlgos := map[string]bool{"bbr": true, "bbr2": true, "bbr3": true, "bbr2plus": true, "bbrplus": true, "cubic": true}
	if algo != "" && !validAlgos[algo] {
		jsonMsg(c, "setSysctl", common.NewError("unsupported algo: ", algo))
		return
	}

	validQdisc := map[string]bool{"fq": true, "cake": true, "": true}
	if !validQdisc[qdisc] {
		jsonMsg(c, "setSysctl", common.NewError("unsupported qdisc: ", qdisc, ", supported: fq, cake"))
		return
	}

	var results []string
	var failures []string
	algoApplied := false
	qdiscApplied := qdisc == ""

	if algo != "" {
		tryLoadKernelModule("tcp_" + algo)

		available, out, err := readSysctl("net.ipv4.tcp_available_congestion_control")
		if err != nil {
			msg := "当前系统不支持通过 sysctl 设置 TCP 拥塞控制: " + commandMessage(out, err)
			logger.Warning(msg)
			failures = append(failures, msg)
			results = append(results, "失败: "+msg)
		} else if !sysctlListHas(available, algo) {
			msg := "当前内核不支持 " + algo + "，可用算法: " + available
			failures = append(failures, msg)
			results = append(results, "失败: "+msg)
		} else if out, err := writeSysctl("net.ipv4.tcp_congestion_control", algo); err != nil {
			msg := "设置 TCP 拥塞控制失败: " + commandMessage(out, err)
			logger.Warning(msg)
			failures = append(failures, msg)
			results = append(results, "失败: "+msg)
		} else {
			algoApplied = true
			results = append(results, strings.TrimSpace(string(out)))
		}
	}

	if qdisc != "" {
		if qdisc == "cake" {
			tryLoadKernelModule("sch_cake")
		}
		out, err := writeSysctl("net.core.default_qdisc", qdisc)
		if err != nil {
			msg := "设置队列调度失败: " + commandMessage(out, err)
			logger.Warning(msg)
			failures = append(failures, msg)
			results = append(results, "失败: "+msg)
		} else {
			qdiscApplied = true
			results = append(results, strings.TrimSpace(string(out)))
		}
	}

	db := database.GetDB()
	if algoApplied {
		db.Exec("DELETE FROM settings WHERE key = ?", "congestionAlgo")
		db.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", "congestionAlgo", algo)
	}
	if qdiscApplied {
		db.Exec("DELETE FROM settings WHERE key = ?", "qdisc")
		if qdisc != "" {
			db.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", "qdisc", qdisc)
		}
	}

	if len(failures) > 0 {
		c.JSON(http.StatusOK, Msg{
			Success: false,
			Msg:     strings.Join(failures, "\n"),
			Obj:     results,
		})
		return
	}

	jsonObj(c, results, nil)
}

func readSysctl(key string) (string, []byte, error) {
	out, err := exec.Command("sysctl", "-n", key).CombinedOutput()
	return strings.TrimSpace(string(out)), out, err
}

func writeSysctl(key string, value string) ([]byte, error) {
	return exec.Command("sysctl", "-w", key+"="+value).CombinedOutput()
}

func commandMessage(out []byte, err error) string {
	msg := strings.TrimSpace(string(out))
	if msg != "" {
		return msg
	}
	if err != nil {
		return err.Error()
	}
	return "unknown error"
}

func sysctlListHas(list string, value string) bool {
	for _, item := range strings.Fields(list) {
		if item == value {
			return true
		}
	}
	return false
}

func tryLoadKernelModule(module string) {
	module = strings.TrimSpace(module)
	if module == "" {
		return
	}
	if err := exec.Command("modprobe", module).Run(); err != nil {
		logger.Debug("modprobe ", module, " skipped: ", err)
	}
}
