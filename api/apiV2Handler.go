package api

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/util/common"

	"github.com/gin-gonic/gin"
)

type TokenInMemory struct {
	Token    string
	Expiry   int64
	Username string
}

type APIv2Handler struct {
	ApiService
	tokensMu sync.RWMutex
	tokens   []TokenInMemory
}

func NewAPIv2Handler(g *gin.RouterGroup) *APIv2Handler {
	a := &APIv2Handler{}
	a.ReloadTokens()
	a.initRouter(g)
	return a
}

func (a *APIv2Handler) initRouter(g *gin.RouterGroup) {
	g.Use(func(c *gin.Context) {
		a.checkToken(c)
	})
	g.POST("/:postAction", a.postHandler)
	g.GET("/:getAction", a.getHandler)
}

func (a *APIv2Handler) postHandler(c *gin.Context) {
	username := a.findUsername(c)
	action := c.Param("postAction")

	switch action {
	case "save":
		a.ApiService.Save(c, username)
	case "restartApp":
		a.ApiService.RestartApp(c)
	case "restartSb":
		a.ApiService.RestartSb(c)
	case "resetTraffic":
		a.ApiService.ResetTraffic(c)
	case "restartXray":
		a.ApiService.RestartXray(c)
	case "linkConvert":
		a.ApiService.LinkConvert(c)
	case "subConvert":
		a.ApiService.SubConvert(c)
	case "importdb":
		a.ApiService.ImportDb(c)
	default:
		jsonMsg(c, "failed", common.NewError("unknown action: ", action))
	}
}

func (a *APIv2Handler) getHandler(c *gin.Context) {
	action := c.Param("getAction")

	switch action {
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
	case "singbox-config":
		a.ApiService.GetSingboxConfig(c)
	case "xray-config":
		a.ApiService.GetXrayConfig(c)
	case "checkOutbound":
		a.ApiService.GetCheckOutbound(c)
	case "checkWarp":
		a.ApiService.GetCheckWarp(c)
	default:
		jsonMsg(c, "failed", common.NewError("unknown action: ", action))
	}
}

func (a *APIv2Handler) findUsername(c *gin.Context) string {
	token := c.Request.Header.Get("Token")
	now := time.Now().Unix()
	a.tokensMu.RLock()
	defer a.tokensMu.RUnlock()
	for _, t := range a.tokens {
		if t.Expiry > 0 && t.Expiry < now {
			continue
		}
		if t.Token == token {
			return t.Username
		}
	}
	return ""
}

func (a *APIv2Handler) checkToken(c *gin.Context) {
	username := a.findUsername(c)
	if username != "" {
		c.Next()
		return
	}
	jsonMsg(c, "", common.NewError("invalid token"))
	c.Abort()
}

func (a *APIv2Handler) ReloadTokens() {
	tokens, err := a.ApiService.LoadTokens()
	if err != nil {
		logger.Error("unable to load tokens: ", err)
		return
	}
	var newTokens []TokenInMemory
	if err = json.Unmarshal(tokens, &newTokens); err != nil {
		logger.Error("unable to load tokens: ", err)
		return
	}
	a.tokensMu.Lock()
	a.tokens = newTokens
	a.tokensMu.Unlock()
}
