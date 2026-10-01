package api

import (
	"strings"

	"github.com/Hhz0823/1s-ui/util/common"

	"github.com/gin-gonic/gin"
)

type APIHandler struct {
	ApiService
	apiv2 *APIv2Handler
}

func NewAPIHandler(g *gin.RouterGroup, a2 *APIv2Handler, policy *OriginPolicy) {
	a := &APIHandler{
		apiv2: a2,
	}
	g.Use(policy.BrowserCSRFMiddleware())
	a.initRouter(g)
}

func (a *APIHandler) initRouter(g *gin.RouterGroup) {
	g.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		action := path[strings.LastIndex(path, "/")+1:]
		if action != "login" && action != "logout" && action != "setup" && action != "setup-status" && action != "managed-login" {
			checkLogin(c)
		}
	})
	g.POST("/:postAction", a.postHandler)
	g.POST("/relay/:id/delete", func(c *gin.Context) { a.ApiService.DeleteRelay(c, GetLoginUser(c)) })
	g.POST("/relay/create", func(c *gin.Context) { a.ApiService.CreateRelay(c, GetLoginUser(c)) })
	g.GET("/relay/:id/bitbrowser.xlsx", a.ApiService.ExportRelayBitBrowser)
	g.GET("/agents", a.ApiService.GetAgents)
	g.GET("/agents/commands", a.ApiService.ListAgentCommands)
	g.POST("/agents/batch-command", a.ApiService.ControlAgentsBatch)
	g.GET("/agents/:id", a.ApiService.GetAgent)
	g.GET("/agents/:id/port-traffic", a.ApiService.GetAgentPortTraffic)
	g.GET("/agents/:id/metrics", a.ApiService.GetAgentMetrics)
	g.POST("/agents/:id/panel-access", a.ApiService.CreateAgentPanelAccess)
	g.PATCH("/agents/:id", a.ApiService.UpdateAgent)
	g.GET("/agents/:id/inbounds", a.ApiService.GetAgentInbounds)
	g.GET("/agents/:id/inbounds/editor", a.ApiService.GetAgentInboundEditor)
	g.POST("/agents/:id/inbounds/save", a.ApiService.SaveAgentInbound)
	g.POST("/agents/:id/inbounds/quick-add", a.ApiService.QuickAddAgentInbounds)
	g.POST("/inbounds/quick-add", a.ApiService.QuickAddLocalInbounds)
	g.GET("/agents/:id/relay", a.ApiService.GetAgentRelayData)
	g.POST("/agents/:id/relay/create", a.ApiService.CreateAgentRelay)
	g.POST("/agents/:id/relay/:relayId/delete", a.ApiService.DeleteAgentRelay)
	g.GET("/agents/:id/relay/:relayId/bitbrowser.xlsx", a.ApiService.ExportAgentRelayBitBrowser)
	g.GET("/agents/:id/terminal", a.ApiService.AgentTerminal)
	g.POST("/agents", a.ApiService.CreateAgent)
	g.POST("/agents/enrollment-link", a.ApiService.CreateAgentEnrollmentLink)
	g.GET("/agents/enrollment-key", a.ApiService.GetAgentEnrollmentKey)
	g.POST("/agents/enrollment-key", a.ApiService.CreateAgentEnrollmentKey)
	g.POST("/agents/enrollment-key/revoke", a.ApiService.RevokeAgentEnrollmentKey)
	g.POST("/agents/connect-local", a.ApiService.ConnectLocalAgent)
	g.GET("/agents/local-connection", a.ApiService.GetLocalAgentConnection)
	g.POST("/agents/disconnect-local", a.ApiService.DisconnectLocalAgent)
	g.POST("/agents/:id/command", a.ApiService.ControlAgent)
	g.POST("/agents/:id/rotate", a.ApiService.RotateAgent)
	g.POST("/agents/:id/delete", a.ApiService.DeleteAgent)
	g.GET("/sdwan", a.ApiService.GetSdwan)
	g.POST("/sdwan/settings", a.ApiService.PostSdwanSettings)
	g.POST("/sdwan/resync", a.ApiService.PostSdwanResync)
	g.POST("/sdwan/test", a.ApiService.PostSdwanTest)
	g.POST("/sdwan/diagnose", a.ApiService.PostSdwanDiagnose)
	g.POST("/sdwan/optimize", a.ApiService.PostSdwanOptimize)
	g.GET("/sdwan/job", a.ApiService.GetSdwanJob)
	g.POST("/sdwan/members/:id", a.ApiService.PostSdwanMember)
	g.POST("/sdwan/members/:id/delete", a.ApiService.DeleteSdwanMember)
	g.GET("/reverse-proxy", a.ApiService.GetReverseProxy)
	g.POST("/reverse-proxy", a.ApiService.SetReverseProxy)
	g.GET("/controller-mode", a.ApiService.GetControllerMode)
	g.POST("/controller-mode", a.ApiService.SetControllerMode)
	g.GET("/port-traffic", a.ApiService.GetPortTraffic)
	g.POST("/port-traffic/:id/reset", a.ApiService.ResetPortTraffic)
	g.GET("/monitor-key", a.ApiService.GetMonitorKey)
	g.POST("/monitor-key", a.ApiService.RotateMonitorKey)
	g.POST("/monitor-key/delete", a.ApiService.DisableMonitorKey)
	g.POST("/monitor-key/settings", a.ApiService.SetMonitorAppSettings)
	g.GET("/proxy-monitors", a.ApiService.GetProxyMonitors)
	g.POST("/proxy-monitors", a.ApiService.SaveProxyMonitor)
	g.POST("/proxy-monitors/test", a.ApiService.TestProxyMonitor)
	g.GET("/proxy-monitors/:id", a.ApiService.GetProxyMonitor)
	g.POST("/proxy-monitors/:id/check", a.ApiService.CheckProxyMonitor)
	g.POST("/proxy-monitors/:id/delete", a.ApiService.DeleteProxyMonitor)
	g.POST("/relay-speedtests", a.ApiService.StartRelaySpeedtest)
	g.GET("/relay-speedtests/:id", a.ApiService.GetRelaySpeedtest)
	g.GET("/proxy-client", a.ApiService.GetProxyClient)
	g.POST("/proxy-client", a.ApiService.CallProxyClient)
	g.GET("/proxy-client/devices", a.ApiService.GetProxyClientDevices)
	g.GET("/xray-install", a.ApiService.GetXrayInstall)
	g.POST("/xray-install", a.ApiService.InstallXray)
	g.POST("/xray-enabled", a.ApiService.SetXrayEnabled)
	g.POST("/xray-uninstall", a.ApiService.UninstallXray)
	g.GET("/:getAction", a.getHandler)
}

func (a *APIHandler) postHandler(c *gin.Context) {
	loginUser := GetLoginUser(c)
	action := c.Param("postAction")

	switch action {
	case "login":
		a.ApiService.Login(c)
	case "setup":
		a.ApiService.Setup(c)
	case "managed-login":
		a.ApiService.ManagedLogin(c)
	case "changePass":
		a.ApiService.ChangePass(c)
	case "save":
		a.ApiService.Save(c, loginUser)
	case "restartApp":
		a.ApiService.RestartApp(c)
	case "restartSb":
		a.ApiService.RestartSb(c)
	case "resetTraffic":
		a.ApiService.ResetTraffic(c)
	case "restartXray":
		a.ApiService.RestartXray(c)
	case "stopXray":
		a.ApiService.StopXray(c)
	case "update":
		a.ApiService.UpdatePanel(c)
	case "linkConvert":
		a.ApiService.LinkConvert(c)
	case "subConvert":
		a.ApiService.SubConvert(c)
	case "importdb":
		a.ApiService.ImportDb(c)
	case "addToken":
		a.ApiService.AddToken(c)
		a.apiv2.ReloadTokens()
	case "deleteToken":
		a.ApiService.DeleteToken(c)
		a.apiv2.ReloadTokens()
	case "setSysctl":
		a.ApiService.SetSysctl(c)
	case "pinnedSha256":
		a.ApiService.PinnedSha256(c)
	default:
		jsonMsg(c, "failed", common.NewError("unknown action: ", action))
	}
}

func (a *APIHandler) getHandler(c *gin.Context) {
	action := c.Param("getAction")

	switch action {
	case "logout":
		a.ApiService.Logout(c)
	case "setup-status":
		a.ApiService.SetupStatus(c)
	case "load":
		a.ApiService.LoadData(c)
	case "inbounds", "outbounds", "endpoints", "services", "tls", "clients", "config":
		err := a.ApiService.LoadPartialData(c, []string{action})
		if err != nil {
			jsonMsg(c, action, err)
		}
		return
	case "users":
		a.ApiService.GetUsers(c)
	case "settings":
		a.ApiService.GetSettings(c)
	case "stats":
		a.ApiService.GetStats(c)
	case "user-traffic":
		a.ApiService.GetUserTraffic(c)
	case "status":
		a.ApiService.GetStatus(c)
	case "onlines":
		a.ApiService.GetOnlines(c)
	case "logs":
		a.ApiService.GetLogs(c)
	case "changes":
		a.ApiService.CheckChanges(c)
	case "keypairs":
		a.ApiService.GetKeypairs(c)
	case "getdb":
		a.ApiService.GetDb(c)
	case "tokens":
		a.ApiService.GetTokens(c)
	case "singbox-config":
		a.ApiService.GetSingboxConfig(c)
	case "xray-config":
		a.ApiService.GetXrayConfig(c)
	case "checkXray":
		a.ApiService.GetCheckXray(c)
	case "version":
		a.ApiService.GetVersion(c)
	case "checkOutbound":
		a.ApiService.GetCheckOutbound(c)
	case "checkWarp":
		a.ApiService.GetCheckWarp(c)
	case "relay":
		a.ApiService.GetRelayData(c)
	default:
		jsonMsg(c, "failed", common.NewError("unknown action: ", action))
	}
}
