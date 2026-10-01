package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Hhz0823/1s-ui/service"

	"github.com/gin-gonic/gin"
)

// registerMonitorRoutes serves the API used by the mobile monitor app. It
// authenticates with the monitor key only, never with admin tokens or
// sessions: a lost phone can read server status and node summaries, and,
// unless the admin turned it off, manage proxy monitors and start speed
// tests. It can never read node credentials or change the panel.
func registerMonitorRoutes(g *gin.RouterGroup) {
	var monitor service.MonitorService
	var proxies service.ProxyMonitorService
	group := g.Group("/monitor")
	group.Use(func(c *gin.Context) {
		if err := monitor.ValidateMonitorKey(monitorKeyFromRequest(c)); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Msg{Success: false, Msg: err.Error()})
			return
		}
		c.Next()
	})
	group.GET("/servers", func(c *gin.Context) {
		result, err := monitor.Overview()
		jsonObj(c, result, err)
	})
	group.GET("/servers/:id", func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		result, err := monitor.Server(uint(id))
		jsonObj(c, result, err)
	})
	group.GET("/servers/:id/nodes", func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		result, err := monitor.Nodes(uint(id))
		jsonObj(c, result, err)
	})
	group.POST("/servers/:id/speedtest", func(c *gin.Context) {
		if !monitor.SettingService.GetMonitorAppSettings().Speedtest {
			monitorForbidden(c, "speed tests from the app are turned off in the panel")
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		result, err := monitor.StartSpeedtest(uint(id))
		jsonObj(c, result, err)
	})

	group.POST("/servers/:id/speedtest/relay", func(c *gin.Context) {
		if !monitor.SettingService.GetMonitorAppSettings().Speedtest {
			monitorForbidden(c, "speed tests from the app are turned off in the panel")
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		var options service.RelaySpeedtestOptions
		if err := c.ShouldBindJSON(&options); err != nil {
			jsonMsg(c, "", err)
			return
		}
		result, err := monitor.StartRelaySpeedtest(uint(id), options)
		jsonObj(c, result, err)
	})
	group.GET("/speedtests/:job", func(c *gin.Context) {
		result, err := monitor.RelaySpeedtest(c.Param("job"))
		jsonObj(c, result, err)
	})

	group.GET("/proxies", func(c *gin.Context) {
		result, err := proxies.List()
		jsonObj(c, result, err)
	})
	group.GET("/proxies/:id", func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		rangeSeconds, _ := strconv.ParseInt(c.Query("range"), 10, 64)
		result, err := proxies.Detail(uint(id), rangeSeconds)
		jsonObj(c, result, err)
	})
	manage := group.Group("/proxies")
	manage.Use(func(c *gin.Context) {
		if !monitor.SettingService.GetMonitorAppSettings().Proxies {
			monitorForbidden(c, "managing proxy monitors from the app is turned off in the panel")
			return
		}
		c.Next()
	})
	manage.POST("", func(c *gin.Context) {
		var input service.ProxyMonitorInput
		if err := c.ShouldBindJSON(&input); err != nil {
			jsonMsg(c, "", err)
			return
		}
		result, err := proxies.Save(input)
		jsonObj(c, result, err)
	})
	manage.POST("/test", func(c *gin.Context) {
		var input service.ProxyMonitorInput
		if err := c.ShouldBindJSON(&input); err != nil {
			jsonMsg(c, "", err)
			return
		}
		withProbeSlot(c, func() (interface{}, error) { return proxies.Test(input) })
	})
	manage.POST("/:id/check", func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		withProbeSlot(c, func() (interface{}, error) { return proxies.CheckNow(uint(id)) })
	})
	manage.POST("/:id/delete", func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		jsonMsg(c, "", proxies.Delete(uint(id)))
	})
}

func monitorForbidden(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusForbidden, Msg{Success: false, Msg: message})
}

// probeSlots bounds on-demand proxy checks, which run up to 15 seconds each.
var probeSlots = make(chan struct{}, 4)

func withProbeSlot(c *gin.Context, run func() (interface{}, error)) {
	select {
	case probeSlots <- struct{}{}:
		defer func() { <-probeSlots }()
		result, err := run()
		jsonObj(c, result, err)
	default:
		jsonObj(c, nil, errors.New("too many proxy checks at once; try again shortly"))
	}
}

func monitorKeyFromRequest(c *gin.Context) string {
	if key := strings.TrimSpace(c.GetHeader("X-Monitor-Key")); key != "" {
		return key
	}
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if len(auth) > 7 && strings.EqualFold(auth[:7], "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

func (a *ApiService) GetMonitorKey(c *gin.Context) {
	jsonObj(c, a.SettingService.GetMonitorKeyStatus(), nil)
}

func (a *ApiService) RotateMonitorKey(c *gin.Context) {
	key, created, err := a.SettingService.RotateMonitorKey()
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	jsonObj(c, map[string]interface{}{"key": key, "created_at": created}, nil)
}

func (a *ApiService) DisableMonitorKey(c *gin.Context) {
	jsonMsg(c, "", a.SettingService.DisableMonitorKey())
}

func (a *ApiService) SetMonitorAppSettings(c *gin.Context) {
	var settings service.MonitorAppSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		jsonMsg(c, "", err)
		return
	}
	if err := a.SettingService.SetMonitorAppSettings(settings); err != nil {
		jsonObj(c, nil, err)
		return
	}
	jsonObj(c, a.SettingService.GetMonitorKeyStatus(), nil)
}

// Proxy monitors on the web UI: the same service as the app, under the
// admin session.

func (a *ApiService) GetProxyMonitors(c *gin.Context) {
	var proxies service.ProxyMonitorService
	result, err := proxies.List()
	jsonObj(c, result, err)
}

func (a *ApiService) GetProxyMonitor(c *gin.Context) {
	var proxies service.ProxyMonitorService
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	rangeSeconds, _ := strconv.ParseInt(c.Query("range"), 10, 64)
	result, err := proxies.Detail(uint(id), rangeSeconds)
	jsonObj(c, result, err)
}

func (a *ApiService) SaveProxyMonitor(c *gin.Context) {
	var proxies service.ProxyMonitorService
	var input service.ProxyMonitorInput
	if err := c.ShouldBindJSON(&input); err != nil {
		jsonMsg(c, "", err)
		return
	}
	result, err := proxies.Save(input)
	jsonObj(c, result, err)
}

func (a *ApiService) TestProxyMonitor(c *gin.Context) {
	var proxies service.ProxyMonitorService
	var input service.ProxyMonitorInput
	if err := c.ShouldBindJSON(&input); err != nil {
		jsonMsg(c, "", err)
		return
	}
	withProbeSlot(c, func() (interface{}, error) { return proxies.Test(input) })
}

func (a *ApiService) CheckProxyMonitor(c *gin.Context) {
	var proxies service.ProxyMonitorService
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	withProbeSlot(c, func() (interface{}, error) { return proxies.CheckNow(uint(id)) })
}

func (a *ApiService) DeleteProxyMonitor(c *gin.Context) {
	var proxies service.ProxyMonitorService
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonMsg(c, "", proxies.Delete(uint(id)))
}

// RelaySpeedtestRequest starts a speed test from one server to another.
type RelaySpeedtestRequest struct {
	ServerId uint `json:"server_id"`
	service.RelaySpeedtestOptions
}

func (a *ApiService) StartRelaySpeedtest(c *gin.Context) {
	var monitor service.MonitorService
	var request RelaySpeedtestRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		jsonMsg(c, "", err)
		return
	}
	result, err := monitor.StartRelaySpeedtest(request.ServerId, request.RelaySpeedtestOptions)
	jsonObj(c, result, err)
}

func (a *ApiService) GetRelaySpeedtest(c *gin.Context) {
	var monitor service.MonitorService
	result, err := monitor.RelaySpeedtest(c.Param("id"))
	jsonObj(c, result, err)
}
