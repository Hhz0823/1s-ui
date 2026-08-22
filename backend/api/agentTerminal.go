package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

// AgentTerminal upgrades a browser WebSocket and bridges an interactive PTY
// on the remote agent (Nezha-style terminal).
func (a *ApiService) AgentTerminal(c *gin.Context) {
	if !IsLogin(c) {
		c.JSON(http.StatusUnauthorized, Msg{Success: false, Msg: "login required"})
		return
	}
	if err := a.SettingService.RequireControllerControl(); err != nil {
		c.JSON(http.StatusForbidden, Msg{Success: false, Msg: err.Error()})
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "invalid agent node id"})
		return
	}
	nodeID := uint(id)
	if !a.AgentService.IsWSConnected(nodeID) {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "agent is not connected via WebSocket"})
		return
	}

	cols, _ := strconv.Atoi(c.DefaultQuery("cols", "80"))
	rows, _ := strconv.Atoi(c.DefaultQuery("rows", "24"))

	// Cross-origin terminals are limited to the same exact allowlist used by
	// credentialed browser API requests. Same-origin remains allowed by default.
	conn, err := websocket.Accept(c.Writer, c.Request, browserWebSocketAcceptOptions(c))
	if err != nil {
		logger.Warning("browser terminal accept failed: ", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "bye")
	conn.SetReadLimit(256 << 10)

	termID, err := a.AgentService.AttachBrowserTerminal(nodeID, conn, cols, rows)
	if err != nil {
		_ = writeBrowserTerm(conn, map[string]interface{}{"type": agent.MsgTypeTerminalClosed, "error": err.Error()})
		return
	}
	defer a.AgentService.DetachBrowserTerminal(nodeID, termID)

	_ = writeBrowserTerm(conn, map[string]interface{}{
		"type": agent.MsgTypeTerminalOpened,
		"id":   termID,
	})

	ctx := c.Request.Context()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var msg struct {
			Type string `json:"type"`
			Data string `json:"data"`
			Cols int    `json:"cols"`
			Rows int    `json:"rows"`
		}
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		switch msg.Type {
		case agent.MsgTypeTerminalInput, "input":
			payload := msg.Data
			if payload != "" && !isLikelyBase64(payload) {
				payload = base64.StdEncoding.EncodeToString([]byte(payload))
			}
			_ = a.AgentService.ForwardTerminalInput(nodeID, termID, payload)
		case agent.MsgTypeTerminalResize, "resize":
			_ = a.AgentService.ForwardTerminalResize(nodeID, termID, msg.Cols, msg.Rows)
		case agent.MsgTypeTerminalClose, "close":
			return
		case agent.MsgTypePing:
			_ = writeBrowserTerm(conn, map[string]interface{}{"type": agent.MsgTypePong})
		}
	}
}

func writeBrowserTerm(conn *websocket.Conn, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, data)
}

func isLikelyBase64(s string) bool {
	if len(s) < 4 || len(s)%4 != 0 {
		return false
	}
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '+' || r == '/' || r == '=' {
			continue
		}
		return false
	}
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}
