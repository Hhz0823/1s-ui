package api

import (
	"time"

	"github.com/Hhz0823/1s-ui/service"

	"github.com/gin-gonic/gin"
)

func (a *ApiService) GetReverseProxy(c *gin.Context) {
	status, err := a.ReverseProxyService.GetStatus()
	jsonObj(c, status, err)
}

func (a *ApiService) SetReverseProxy(c *gin.Context) {
	var request service.ReverseProxyConfig
	if err := c.ShouldBindJSON(&request); err != nil {
		jsonObj(c, nil, err)
		return
	}
	status, err := a.ReverseProxyService.Apply(request)
	if err == nil {
		err = a.PanelService.RestartPanel(2 * time.Second)
	}
	jsonObj(c, status, err)
}
