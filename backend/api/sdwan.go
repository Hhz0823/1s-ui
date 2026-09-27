package api

import (
	"errors"
	"io"
	"net/http"

	"github.com/Hhz0823/1s-ui/service"

	"github.com/gin-gonic/gin"
)

func (a *ApiService) GetSdwan(c *gin.Context) {
	state, err := a.SdwanService.SdwanState()
	jsonObj(c, state, err)
}

func (a *ApiService) PostSdwanSettings(c *gin.Context) {
	var settings service.SdwanSettings
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	if err := c.ShouldBindJSON(&settings); err != nil {
		jsonObj(c, nil, err)
		return
	}
	state, err := a.SdwanService.SaveSdwanSettings(settings, GetLoginUser(c))
	jsonObj(c, state, err)
}

func (a *ApiService) PostSdwanMember(c *gin.Context) {
	id, err := parseAgentNodeID(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	state, err := a.SdwanService.AddSdwanMember(id, GetLoginUser(c))
	jsonObj(c, state, err)
}

func (a *ApiService) DeleteSdwanMember(c *gin.Context) {
	id, err := parseAgentNodeID(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	state, err := a.SdwanService.RemoveSdwanMember(id, GetLoginUser(c))
	jsonObj(c, state, err)
}

func (a *ApiService) PostSdwanResync(c *gin.Context) {
	state, err := a.SdwanService.ResyncSdwan(GetLoginUser(c))
	jsonObj(c, state, err)
}

func (a *ApiService) PostSdwanTest(c *gin.Context) {
	state, err := a.SdwanService.TestSdwan()
	jsonObj(c, state, err)
}

type sdwanJobRequest struct {
	Bandwidth bool `json:"bandwidth"`
}

func (a *ApiService) startSdwanJob(c *gin.Context, kind string) {
	var request sdwanJobRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	if err := c.ShouldBindJSON(&request); err != nil && !errors.Is(err, io.EOF) {
		jsonObj(c, nil, err)
		return
	}
	job, err := a.SdwanService.StartSdwanJob(kind, request.Bandwidth, GetLoginUser(c))
	jsonObj(c, job, err)
}

func (a *ApiService) PostSdwanDiagnose(c *gin.Context) {
	a.startSdwanJob(c, service.SdwanJobDiagnose)
}

func (a *ApiService) PostSdwanOptimize(c *gin.Context) {
	a.startSdwanJob(c, service.SdwanJobOptimize)
}

func (a *ApiService) GetSdwanJob(c *gin.Context) {
	jsonObj(c, a.SdwanService.SdwanJob(), nil)
}
