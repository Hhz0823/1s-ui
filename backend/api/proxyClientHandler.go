package api

import (
	"encoding/json"
	"strconv"

	"github.com/Hhz0823/1s-ui/service"

	"github.com/gin-gonic/gin"
)

// The proxy client page works on this panel (server=0) or, from a
// controller, on a managed device over the agent channel.

func proxyClientServer(c *gin.Context) (uint, bool) {
	value := c.Query("server")
	if value == "" {
		return 0, true
	}
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		jsonMsg(c, "", err)
		return 0, false
	}
	return uint(id), true
}

func writeProxyClientResult(c *gin.Context, raw json.RawMessage, err error) {
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	jsonObj(c, raw, nil)
}

func (a *ApiService) GetProxyClient(c *gin.Context) {
	server, ok := proxyClientServer(c)
	if !ok {
		return
	}
	var clients service.ProxyClientService
	raw, err := clients.CallProxyClient(server, service.ProxyClientCall{Action: "state"})
	writeProxyClientResult(c, raw, err)
}

func (a *ApiService) CallProxyClient(c *gin.Context) {
	server, ok := proxyClientServer(c)
	if !ok {
		return
	}
	var call service.ProxyClientCall
	if err := c.ShouldBindJSON(&call); err != nil {
		jsonMsg(c, "", err)
		return
	}
	var clients service.ProxyClientService
	raw, err := clients.CallProxyClient(server, call)
	writeProxyClientResult(c, raw, err)
}

func (a *ApiService) GetProxyClientDevices(c *gin.Context) {
	var clients service.ProxyClientService
	devices, err := clients.ProxyClientDevices()
	jsonObj(c, devices, err)
}

// registerMonitorClientRoutes lets the monitor app read the proxy client of
// this panel and its servers and, unless the admin turned it off, switch
// nodes and modes. Subscription addresses and passwords never reach it.
func registerMonitorClientRoutes(group *gin.RouterGroup, monitor *service.MonitorService) {
	var clients service.ProxyClientService
	group.GET("/servers/:id/client", func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		raw, err := clients.CallProxyClient(uint(id), service.ProxyClientCall{Action: "state"})
		if err == nil {
			raw, err = service.RedactProxyClientResult(raw)
		}
		writeProxyClientResult(c, raw, err)
	})
	group.POST("/servers/:id/client", func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		var call service.ProxyClientCall
		if err := c.ShouldBindJSON(&call); err != nil {
			jsonMsg(c, "", err)
			return
		}
		if !service.ProxyClientAppActions[call.Action] {
			monitorForbidden(c, "the app cannot do this; use the panel")
			return
		}
		if !service.ProxyClientReadOnly(call.Action) && !monitor.SettingService.GetMonitorAppSettings().Client {
			monitorForbidden(c, "managing the proxy client from the app is turned off in the panel")
			return
		}
		raw, err := clients.CallProxyClient(uint(id), call)
		if err == nil {
			raw, err = service.RedactProxyClientResult(raw)
		}
		writeProxyClientResult(c, raw, err)
	})
}
