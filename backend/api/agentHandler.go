package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/config"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/service"
	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

type AgentHandler struct {
	service.AgentService
	service.SettingService
}

func NewAgentHandler(group *gin.RouterGroup) {
	handler := &AgentHandler{}
	group.POST("/pair", handler.Pair)
	group.POST("/enroll", handler.Enroll)
	group.POST("/heartbeat", handler.Heartbeat)
	group.GET("/ws", handler.WebSocket)
}

type pairAgentRequest struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

func (h *AgentHandler) Pair(c *gin.Context) {
	if !h.requireControllerMode(c) {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	var request pairAgentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "invalid pairing request"})
		return
	}
	webPath, err := h.SettingService.GetWebPath()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Msg{Success: false, Msg: "panel configuration is unavailable"})
		return
	}
	enrollment, err := h.AgentService.ConsumePairingCode(request.Code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, Msg{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Msg{Success: true, Obj: map[string]interface{}{
		"panel_url": panelURLForRequest(c, webPath),
		"token":     enrollment.Token,
		"version":   config.GetVersion(),
		"node_id":   enrollment.Node.Id,
	}})
}

func (h *AgentHandler) Enroll(c *gin.Context) {
	if !h.requireControllerMode(c) {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	var request pairAgentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "invalid enrollment request"})
		return
	}
	webPath, err := h.SettingService.GetWebPath()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Msg{Success: false, Msg: "panel configuration is unavailable"})
		return
	}
	var enrollment *service.AgentEnrollment
	if strings.TrimSpace(request.Code) == "" {
		targetNodeID, pairErr := h.SettingService.ConsumeAgentAddressPairing()
		if pairErr != nil {
			c.JSON(http.StatusUnauthorized, Msg{Success: false, Msg: pairErr.Error()})
			return
		}
		if targetNodeID == 0 {
			enrollment, err = h.AgentService.CreateConnected(request.Name)
		} else {
			enrollment, err = h.AgentService.Reconnect(targetNodeID)
		}
	} else {
		if err := h.SettingService.ValidateAgentEnrollmentKey(request.Code); err != nil {
			c.JSON(http.StatusUnauthorized, Msg{Success: false, Msg: err.Error()})
			return
		}
		enrollment, err = h.AgentService.CreateConnected(request.Name)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Msg{Success: true, Obj: map[string]interface{}{
		"panel_url": panelURLForRequest(c, webPath),
		"token":     enrollment.Token,
		"version":   config.GetVersion(),
		"node_id":   enrollment.Node.Id,
	}})
}

func (h *AgentHandler) Heartbeat(c *gin.Context) {
	if !h.requireControllerMode(c) {
		return
	}
	authorization := strings.TrimSpace(c.GetHeader("Authorization"))
	if !strings.HasPrefix(authorization, "Bearer ") {
		c.JSON(http.StatusUnauthorized, Msg{Success: false, Msg: "missing agent authorization"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var report agent.Report
	if err := c.ShouldBindJSON(&report); err != nil {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "invalid agent report"})
		return
	}
	if report.ConnMode == "" {
		report.ConnMode = "http"
	}
	resp, err := h.AgentService.Heartbeat(strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer ")), getRemoteIp(c), report)
	if err != nil {
		c.JSON(http.StatusUnauthorized, Msg{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Msg{Success: true, Obj: resp})
}

// WebSocket is the long-lived control + metrics channel (Nezha/Komari style).
func (h *AgentHandler) WebSocket(c *gin.Context) {
	if !h.requireControllerMode(c) {
		return
	}
	token := extractAgentToken(c)
	nodeID, err := h.AgentService.AuthenticateToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, Msg{Success: false, Msg: err.Error()})
		return
	}

	// The default origin policy accepts non-browser agents without Origin and
	// same-origin browser requests, while rejecting cross-site WS hijacking.
	conn, err := websocket.Accept(c.Writer, c.Request, nil)
	if err != nil {
		logger.Warning("agent websocket accept failed: ", err)
		return
	}
	conn.SetReadLimit(2 << 20)

	sessionCtx, unregister := h.AgentService.RegisterSession(nodeID, conn)
	defer func() {
		unregister()
		_ = conn.Close(websocket.StatusNormalClosure, "bye")
	}()

	remoteIP := getRemoteIp(c)
	reqCtx := c.Request.Context()

	write := func(v interface{}) error {
		return h.AgentService.WriteToSession(sessionCtx, nodeID, v)
	}

	_ = write(map[string]interface{}{
		"type":             agent.MsgTypeConfig,
		"server_time":      time.Now().Unix(),
		"interval_seconds": 15,
	})

	for {
		msgType, data, err := conn.Read(reqCtx)
		if err != nil {
			return
		}
		if msgType != websocket.MessageText && msgType != websocket.MessageBinary {
			continue
		}
		var envelope struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
			ID      string          `json:"id"`
			Command string          `json:"command"`
			Method  string          `json:"method"`
			OK      bool            `json:"ok"`
			Output  string          `json:"output"`
			Error   string          `json:"error"`
			Code    int             `json:"code"`
			Elapsed int64           `json:"elapsed_ms"`
			Data    string          `json:"data"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			_ = write(map[string]interface{}{"type": agent.MsgTypeError, "error": "invalid message"})
			continue
		}
		switch envelope.Type {
		case agent.MsgTypeReport:
			var report agent.Report
			if len(envelope.Payload) == 0 {
				_ = json.Unmarshal(data, &report)
			} else if err := json.Unmarshal(envelope.Payload, &report); err != nil {
				_ = write(map[string]interface{}{"type": agent.MsgTypeError, "error": "invalid report"})
				continue
			}
			report.ConnMode = "ws"
			if _, err := h.AgentService.Heartbeat(token, remoteIP, report); err != nil {
				_ = write(map[string]interface{}{"type": agent.MsgTypeError, "error": err.Error()})
				continue
			}
			_ = write(map[string]interface{}{
				"type":             agent.MsgTypeAck,
				"server_time":      time.Now().Unix(),
				"interval_seconds": 15,
			})
		case agent.MsgTypeCommandResult:
			h.AgentService.HandleCommandResult(agent.CommandResult{
				ID:      envelope.ID,
				Type:    envelope.Command,
				OK:      envelope.OK,
				Output:  envelope.Output,
				Error:   envelope.Error,
				Code:    envelope.Code,
				Elapsed: envelope.Elapsed,
			})
		case agent.MsgTypeRPCResponse:
			h.AgentService.HandleRPCResponse(nodeID, agent.RPCResponse{
				ID: envelope.ID, OK: envelope.OK, Payload: envelope.Payload,
				Error: envelope.Error, Code: envelope.Code,
			})
		case agent.MsgTypeTerminalOpened, agent.MsgTypeTerminalOutput, agent.MsgTypeTerminalClosed:
			h.AgentService.HandleTerminalFromAgent(nodeID, envelope.Type, envelope.ID, envelope.Data, envelope.Error)
		case agent.MsgTypePing:
			_ = write(map[string]interface{}{"type": agent.MsgTypePong, "id": envelope.ID, "server_time": time.Now().Unix()})
		case agent.MsgTypePong:
			h.AgentService.HandlePong(nodeID, envelope.ID)
		default:
			_ = write(map[string]interface{}{"type": agent.MsgTypeError, "error": "unknown type"})
		}
	}
}

func (h *AgentHandler) requireControllerMode(c *gin.Context) bool {
	enabled, err := h.SettingService.IsControllerModeEnabled()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Msg{Success: false, Msg: "controller mode state is unavailable"})
		return false
	}
	if !enabled {
		c.JSON(http.StatusServiceUnavailable, Msg{Success: false, Msg: "controller mode is disabled"})
		return false
	}
	return true
}

func extractAgentToken(c *gin.Context) string {
	authorization := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(authorization, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	}
	if q := strings.TrimSpace(c.Query("token")); q != "" {
		return q
	}
	return ""
}
