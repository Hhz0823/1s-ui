package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Hhz0823/1s-ui/service"

	"github.com/gin-gonic/gin"
)

// registerMonitorRoutes serves the read-only API used by the mobile monitor
// app. It authenticates with the monitor key only, never with admin tokens or
// sessions, so a lost phone can at most read server status.
func registerMonitorRoutes(g *gin.RouterGroup) {
	var monitor service.MonitorService
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
