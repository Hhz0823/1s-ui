package service

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util/common"
	"github.com/coder/websocket"
	"github.com/gofrs/uuid/v5"
)

const (
	agentCommandTimeout  = 45 * time.Second
	agentCommandHistory  = 50
	agentLatencyInterval = 10 * time.Second
	agentLatencyTimeout  = 5 * time.Second
	agentLatencyHistory  = 120
)

// browserTerminal bridges a panel browser WebSocket to a remote agent PTY session.
type browserTerminal struct {
	id      string
	nodeID  uint
	conn    *websocket.Conn
	writeMu sync.Mutex
	cancel  context.CancelFunc
}

// agentSession holds a live WebSocket to a remote agent for control + telemetry.
type agentSession struct {
	nodeID       uint
	conn         *websocket.Conn
	writeMu      sync.Mutex
	pending      map[string]chan agent.CommandResult
	pendingMu    sync.Mutex
	rpcPending   map[string]chan agent.RPCResponse
	rpcPendingMu sync.Mutex
	pings        map[string]time.Time
	pingMu       sync.Mutex
	terms        map[string]*browserTerminal
	termsMu      sync.Mutex
	cancel       context.CancelFunc
}

// BatchCommandResult is one node's outcome in a multi-node control fan-out.
type BatchCommandResult struct {
	NodeID uint                 `json:"node_id"`
	Name   string               `json:"name,omitempty"`
	OK     bool                 `json:"ok"`
	Error  string               `json:"error,omitempty"`
	Result *agent.CommandResult `json:"result,omitempty"`
}

type agentCommandLog struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Args      map[string]interface{} `json:"args,omitempty"`
	OK        bool                   `json:"ok"`
	Output    string                 `json:"output,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Code      int                    `json:"code,omitempty"`
	Elapsed   int64                  `json:"elapsed_ms,omitempty"`
	CreatedAt int64                  `json:"created_at"`
	Actor     string                 `json:"actor,omitempty"`
}

type agentLatencySample struct {
	Time int64
	MS   int64
	OK   bool
}

type AgentLatencyView struct {
	LastMS    *int64  `json:"last_ms"`
	AverageMS float64 `json:"average_ms"`
	P95MS     int64   `json:"p95_ms"`
	LossPct   float64 `json:"loss_pct"`
	Samples   int     `json:"samples"`
	UpdatedAt int64   `json:"updated_at"`
}

var (
	agentHubMu     sync.RWMutex
	agentSessions  = map[uint]*agentSession{}
	agentCmdLogMu  sync.RWMutex
	agentCmdLogs   = map[uint][]agentCommandLog{}
	agentLatencyMu sync.RWMutex
	agentLatencies = map[uint][]agentLatencySample{}
)

func (s *AgentService) RegisterSession(nodeID uint, conn *websocket.Conn) (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	session := &agentSession{
		nodeID:     nodeID,
		conn:       conn,
		pending:    make(map[string]chan agent.CommandResult),
		rpcPending: make(map[string]chan agent.RPCResponse),
		pings:      make(map[string]time.Time),
		terms:      make(map[string]*browserTerminal),
		cancel:     cancel,
	}
	agentHubMu.Lock()
	if old, ok := agentSessions[nodeID]; ok {
		old.cancel()
		_ = old.conn.Close(websocket.StatusGoingAway, "replaced")
	}
	agentSessions[nodeID] = session
	agentHubMu.Unlock()
	s.SetWSConnected(nodeID, true)
	go s.runLatencyProbe(ctx, session)

	unregister := func() {
		agentHubMu.Lock()
		current, ok := agentSessions[nodeID]
		if ok && current == session {
			delete(agentSessions, nodeID)
		}
		agentHubMu.Unlock()
		session.failPending("agent disconnected")
		session.failRPCPending("agent disconnected")
		session.failPings()
		session.closeAllTerminals("agent disconnected")
		s.SetWSConnected(nodeID, false)
		cancel()
	}
	return ctx, unregister
}

func (s *AgentService) DisconnectAll(reason string) {
	if reason == "" {
		reason = "controller unavailable"
	}
	agentHubMu.RLock()
	sessions := make([]*agentSession, 0, len(agentSessions))
	for _, session := range agentSessions {
		sessions = append(sessions, session)
	}
	agentHubMu.RUnlock()
	for _, session := range sessions {
		session.cancel()
		_ = session.conn.Close(websocket.StatusGoingAway, reason)
	}
}

func (s *AgentService) CloseAllTerminals(reason string) {
	agentHubMu.RLock()
	sessions := make([]*agentSession, 0, len(agentSessions))
	for _, session := range agentSessions {
		sessions = append(sessions, session)
	}
	agentHubMu.RUnlock()
	for _, session := range sessions {
		session.closeAllTerminals(reason)
	}
}

func (s *agentSession) failRPCPending(msg string) {
	s.rpcPendingMu.Lock()
	defer s.rpcPendingMu.Unlock()
	for id, ch := range s.rpcPending {
		select {
		case ch <- agent.RPCResponse{ID: id, OK: false, Error: msg, Code: 503}:
		default:
		}
		close(ch)
		delete(s.rpcPending, id)
	}
}

func (s *agentSession) failPings() {
	s.pingMu.Lock()
	count := len(s.pings)
	s.pings = make(map[string]time.Time)
	s.pingMu.Unlock()
	for i := 0; i < count; i++ {
		appendAgentLatency(s.nodeID, 0, false)
	}
}

func (s *agentSession) closeAllTerminals(reason string) {
	s.termsMu.Lock()
	defer s.termsMu.Unlock()
	for id, term := range s.terms {
		_ = term.writeJSON(map[string]interface{}{"type": agent.MsgTypeTerminalClosed, "id": id, "error": reason})
		term.cancel()
		_ = term.conn.Close(websocket.StatusGoingAway, reason)
		delete(s.terms, id)
	}
}

func (s *agentSession) failPending(msg string) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	for id, ch := range s.pending {
		select {
		case ch <- agent.CommandResult{ID: id, OK: false, Error: msg}:
		default:
		}
		close(ch)
		delete(s.pending, id)
	}
}

func (s *agentSession) WriteJSON(ctx context.Context, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return s.conn.Write(writeCtx, websocket.MessageText, data)
}

func (s *AgentService) HandleCommandResult(result agent.CommandResult) {
	agentHubMu.RLock()
	var target *agentSession
	for _, sess := range agentSessions {
		sess.pendingMu.Lock()
		_, ok := sess.pending[result.ID]
		sess.pendingMu.Unlock()
		if ok {
			target = sess
			break
		}
	}
	agentHubMu.RUnlock()
	if target == nil {
		return
	}
	target.pendingMu.Lock()
	ch, ok := target.pending[result.ID]
	if ok {
		delete(target.pending, result.ID)
	}
	target.pendingMu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- result:
	default:
	}
	close(ch)
}

func (s *AgentService) HandlePong(nodeID uint, id string) {
	if id == "" {
		return
	}
	agentHubMu.RLock()
	session := agentSessions[nodeID]
	agentHubMu.RUnlock()
	if session == nil {
		return
	}
	session.pingMu.Lock()
	started, ok := session.pings[id]
	if ok {
		delete(session.pings, id)
	}
	session.pingMu.Unlock()
	if !ok {
		return
	}
	elapsed := time.Since(started).Milliseconds()
	if elapsed < 0 {
		elapsed = 0
	}
	appendAgentLatency(nodeID, elapsed, true)
}

func (s *AgentService) HandleRPCResponse(nodeID uint, response agent.RPCResponse) {
	agentHubMu.RLock()
	session := agentSessions[nodeID]
	agentHubMu.RUnlock()
	if session == nil || response.ID == "" {
		return
	}
	session.rpcPendingMu.Lock()
	ch, ok := session.rpcPending[response.ID]
	if ok {
		delete(session.rpcPending, response.ID)
	}
	session.rpcPendingMu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- response:
	default:
	}
	close(ch)
}

func (s *AgentService) runLatencyProbe(ctx context.Context, session *agentSession) {
	probe := func() {
		now := time.Now()
		session.pingMu.Lock()
		for id, started := range session.pings {
			if now.Sub(started) >= agentLatencyTimeout {
				delete(session.pings, id)
				appendAgentLatency(session.nodeID, 0, false)
			}
		}
		if len(session.pings) > 0 {
			session.pingMu.Unlock()
			return
		}
		id := uuid.Must(uuid.NewV4()).String()
		session.pings[id] = now
		session.pingMu.Unlock()
		if err := session.WriteJSON(ctx, map[string]interface{}{
			"type": agent.MsgTypePing,
			"id":   id,
			"time": now.UnixMilli(),
		}); err != nil {
			session.pingMu.Lock()
			delete(session.pings, id)
			session.pingMu.Unlock()
			appendAgentLatency(session.nodeID, 0, false)
		}
	}

	probe()
	ticker := time.NewTicker(agentLatencyInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			probe()
		}
	}
}

func appendAgentLatency(nodeID uint, ms int64, ok bool) {
	agentLatencyMu.Lock()
	defer agentLatencyMu.Unlock()
	samples := append(agentLatencies[nodeID], agentLatencySample{Time: time.Now().Unix(), MS: ms, OK: ok})
	if len(samples) > agentLatencyHistory {
		samples = samples[len(samples)-agentLatencyHistory:]
	}
	agentLatencies[nodeID] = samples
	recordAgentPing(nodeID, ms, ok)
}

// AgentLatencyPoint is one probe for the live latency chart; failed probes
// have OK false.
type AgentLatencyPoint struct {
	Time int64 `json:"time"`
	MS   int64 `json:"ms"`
	OK   bool  `json:"ok"`
}

func getAgentLatencyLog(nodeID uint) []AgentLatencyPoint {
	agentLatencyMu.RLock()
	defer agentLatencyMu.RUnlock()
	samples := agentLatencies[nodeID]
	out := make([]AgentLatencyPoint, 0, len(samples))
	for _, sample := range samples {
		out = append(out, AgentLatencyPoint{Time: sample.Time, MS: sample.MS, OK: sample.OK})
	}
	return out
}

func getAgentLatency(nodeID uint) AgentLatencyView {
	agentLatencyMu.RLock()
	samples := append([]agentLatencySample(nil), agentLatencies[nodeID]...)
	agentLatencyMu.RUnlock()
	view := AgentLatencyView{Samples: len(samples)}
	if len(samples) == 0 {
		return view
	}
	view.UpdatedAt = samples[len(samples)-1].Time
	values := make([]int64, 0, len(samples))
	failed := 0
	for _, sample := range samples {
		if !sample.OK {
			failed++
			continue
		}
		value := sample.MS
		values = append(values, value)
		view.LastMS = &value
		view.AverageMS += float64(value)
	}
	view.LossPct = float64(failed) * 100 / float64(len(samples))
	if len(values) == 0 {
		return view
	}
	view.AverageMS /= float64(len(values))
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	index := (95*len(values)+99)/100 - 1
	if index < 0 {
		index = 0
	}
	view.P95MS = values[index]
	return view
}

// DispatchCommand sends a control command to an online agent and waits for the result.
func (s *AgentService) DispatchCommand(nodeID uint, cmdType string, args map[string]interface{}, actor string) (*agent.CommandResult, error) {
	if err := (&SettingService{}).RequireControllerControl(); err != nil {
		return nil, err
	}
	if err := validateAgentCommand(cmdType, args); err != nil {
		return nil, err
	}
	agentHubMu.RLock()
	session := agentSessions[nodeID]
	agentHubMu.RUnlock()
	if session == nil {
		return nil, common.NewError("agent is not connected via WebSocket; remote control requires an online WS session")
	}

	id := uuid.Must(uuid.NewV4()).String()
	started := time.Now()
	resultCh := make(chan agent.CommandResult, 1)
	session.pendingMu.Lock()
	session.pending[id] = resultCh
	session.pendingMu.Unlock()

	payload := map[string]interface{}{
		"type":    agent.MsgTypeCommand,
		"id":      id,
		"command": cmdType,
		"args":    args,
		"time":    time.Now().Unix(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err := session.WriteJSON(ctx, payload)
	cancel()
	if err != nil {
		session.pendingMu.Lock()
		delete(session.pending, id)
		session.pendingMu.Unlock()
		close(resultCh)
		return nil, common.NewErrorf("failed to send command: %v", err)
	}

	timer := time.NewTimer(agentCommandTimeout)
	defer timer.Stop()
	select {
	case result := <-resultCh:
		result.ID = id
		result.Type = cmdType
		if cmdType == agent.CmdPing {
			result.Elapsed = time.Since(started).Milliseconds()
		}
		appendAgentCommandLog(nodeID, agentCommandLog{
			ID: id, Type: cmdType, Args: args, OK: result.OK, Output: result.Output,
			Error: result.Error, Code: result.Code, Elapsed: result.Elapsed,
			CreatedAt: time.Now().Unix(), Actor: actor,
		})
		return &result, nil
	case <-timer.C:
		session.pendingMu.Lock()
		delete(session.pending, id)
		session.pendingMu.Unlock()
		appendAgentCommandLog(nodeID, agentCommandLog{
			ID: id, Type: cmdType, Args: args, OK: false, Error: "command timed out",
			CreatedAt: time.Now().Unix(), Actor: actor,
		})
		return nil, common.NewError("agent command timed out")
	}
}

func (s *AgentService) DispatchRPC(nodeID uint, method string, payload interface{}, actor string) (*agent.RPCResponse, error) {
	if !validAgentRPCMethod(method) {
		return nil, common.NewError("unsupported managed-node RPC method")
	}
	status, err := (&SettingService{}).GetControllerModeStatus()
	if err != nil {
		return nil, err
	}
	if !status.Enabled {
		return nil, common.NewError("controller mode is disabled")
	}
	if !status.CanControl && method != agent.RPCMethodCapabilities && method != agent.RPCMethodPortTraffic {
		return nil, common.NewError("controller monitor profile only allows read-only metrics")
	}
	agentHubMu.RLock()
	session := agentSessions[nodeID]
	agentHubMu.RUnlock()
	if session == nil {
		return nil, common.NewError("managed server is offline or not connected via WebSocket")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if len(raw) > 512*1024 {
		return nil, common.NewError("managed-node RPC payload is too large")
	}
	id := uuid.Must(uuid.NewV4()).String()
	resultCh := make(chan agent.RPCResponse, 1)
	session.rpcPendingMu.Lock()
	session.rpcPending[id] = resultCh
	session.rpcPendingMu.Unlock()
	request := map[string]interface{}{
		"type": agent.MsgTypeRPCRequest, "id": id, "method": method,
		"payload": json.RawMessage(raw), "time": time.Now().Unix(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = session.WriteJSON(ctx, request)
	cancel()
	if err != nil {
		session.rpcPendingMu.Lock()
		delete(session.rpcPending, id)
		session.rpcPendingMu.Unlock()
		close(resultCh)
		return nil, common.NewErrorf("failed to send managed-node RPC: %v", err)
	}
	timer := time.NewTimer(agentRPCTimeout(method))
	defer timer.Stop()
	select {
	case response := <-resultCh:
		response.ID = id
		if agentRPCMutatesConfig(method) || !response.OK {
			appendAgentCommandLog(nodeID, agentCommandLog{
				ID: id, Type: "rpc/" + method, OK: response.OK, Error: response.Error,
				CreatedAt: time.Now().Unix(), Actor: actor,
			})
		}
		if !response.OK {
			if response.Error == "" {
				response.Error = "managed-node RPC failed"
			}
			return &response, common.NewError(response.Error)
		}
		return &response, nil
	case <-timer.C:
		session.rpcPendingMu.Lock()
		delete(session.rpcPending, id)
		session.rpcPendingMu.Unlock()
		appendAgentCommandLog(nodeID, agentCommandLog{
			ID: id, Type: "rpc/" + method, OK: false, Error: "managed-node RPC timed out",
			CreatedAt: time.Now().Unix(), Actor: actor,
		})
		return nil, common.NewError("managed-node RPC timed out")
	}
}

func validAgentRPCMethod(method string) bool {
	switch method {
	case agent.RPCMethodCapabilities,
		agent.RPCMethodPortTraffic,
		agent.RPCMethodInboundList,
		agent.RPCMethodInboundEdit,
		agent.RPCMethodInboundSave,
		agent.RPCMethodInboundQuickAdd,
		agent.RPCMethodRelayGet,
		agent.RPCMethodRelayCreate,
		agent.RPCMethodRelayDelete,
		agent.RPCMethodRelayExport,
		agent.RPCMethodPanelAccess,
		agent.RPCMethodSdwanProvision,
		agent.RPCMethodSdwanRemove,
		agent.RPCMethodSdwanDiagnose,
		agent.RPCMethodSdwanTune:
		return true
	default:
		return false
	}
}

func agentRPCTimeout(method string) time.Duration {
	switch method {
	case agent.RPCMethodInboundQuickAdd, agent.RPCMethodRelayCreate, agent.RPCMethodRelayDelete:
		return 10 * time.Minute
	case agent.RPCMethodSdwanProvision, agent.RPCMethodSdwanRemove, agent.RPCMethodSdwanTune:
		return 3 * time.Minute
	default:
		return agentCommandTimeout
	}
}

func agentRPCMutatesConfig(method string) bool {
	switch method {
	case agent.RPCMethodInboundSave, agent.RPCMethodInboundQuickAdd, agent.RPCMethodRelayCreate, agent.RPCMethodRelayDelete,
		agent.RPCMethodSdwanProvision, agent.RPCMethodSdwanRemove, agent.RPCMethodSdwanTune:
		return true
	default:
		return false
	}
}

func validateAgentCommand(cmdType string, args map[string]interface{}) error {
	switch cmdType {
	case agent.CmdReportNow, agent.CmdRestartAgent, agent.CmdRestartXray, agent.CmdRestartSingBox, agent.CmdPing:
		return nil
	case agent.CmdSetInterval:
		sec := 0
		if args != nil {
			switch v := args["seconds"].(type) {
			case float64:
				sec = int(v)
			case int:
				sec = v
			case json.Number:
				i, _ := v.Int64()
				sec = int(i)
			}
		}
		if sec < 5 || sec > 300 {
			return common.NewError("interval seconds must be between 5 and 300")
		}
		return nil
	case agent.CmdExec:
		if args == nil {
			return common.NewError("exec requires args.command")
		}
		cmd, _ := args["command"].(string)
		if len(cmd) == 0 || len(cmd) > 4000 {
			return common.NewError("exec command must be 1-4000 characters")
		}
		return nil
	default:
		return common.NewErrorf("unsupported agent command: %s", cmdType)
	}
}

func appendAgentCommandLog(nodeID uint, entry agentCommandLog) {
	// Cap huge outputs in history.
	if len(entry.Output) > 16*1024 {
		entry.Output = entry.Output[:16*1024] + "\n...[truncated]"
	}
	agentCmdLogMu.Lock()
	defer agentCmdLogMu.Unlock()
	logs := append(agentCmdLogs[nodeID], entry)
	if len(logs) > agentCommandHistory {
		logs = logs[len(logs)-agentCommandHistory:]
	}
	agentCmdLogs[nodeID] = logs
}

func getAgentCommandLogs(nodeID uint) []agentCommandLog {
	agentCmdLogMu.RLock()
	defer agentCmdLogMu.RUnlock()
	src := agentCmdLogs[nodeID]
	if len(src) == 0 {
		return nil
	}
	out := make([]agentCommandLog, len(src))
	copy(out, src)
	return out
}

func (s *AgentService) IsWSConnected(nodeID uint) bool {
	agentHubMu.RLock()
	defer agentHubMu.RUnlock()
	_, ok := agentSessions[nodeID]
	return ok
}

func (s *AgentService) WriteToSession(ctx context.Context, nodeID uint, v interface{}) error {
	agentHubMu.RLock()
	session := agentSessions[nodeID]
	agentHubMu.RUnlock()
	if session == nil {
		return common.NewError("agent session not found")
	}
	return session.WriteJSON(ctx, v)
}

// ListCommands returns the supported remote control commands for the UI.
func (s *AgentService) ListCommands() []map[string]string {
	return []map[string]string{
		{"type": agent.CmdReportNow, "name": "Refresh metrics"},
		{"type": agent.CmdPing, "name": "Ping agent"},
		{"type": agent.CmdSetInterval, "name": "Set report interval"},
		{"type": agent.CmdRestartXray, "name": "Restart Xray"},
		{"type": agent.CmdRestartSingBox, "name": "Restart sing-box"},
		{"type": agent.CmdRestartAgent, "name": "Restart agent"},
		{"type": agent.CmdExec, "name": "Run shell command"},
	}
}

// DispatchBatch runs the same command on many agents in parallel.
func (s *AgentService) DispatchBatch(ids []uint, cmdType string, args map[string]interface{}, actor string) ([]BatchCommandResult, error) {
	if err := (&SettingService{}).RequireControllerControl(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > 100 {
		ids = ids[:100]
	}
	// Resolve names once.
	nameByID := map[uint]string{}
	var nodes []model.AgentNode
	if err := database.GetDB().Select("id", "name").Where("id IN ?", ids).Find(&nodes).Error; err == nil {
		for _, n := range nodes {
			nameByID[n.Id] = n.Name
		}
	}

	results := make([]BatchCommandResult, len(ids))
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(idx int, nodeID uint) {
			defer wg.Done()
			item := BatchCommandResult{NodeID: nodeID, Name: nameByID[nodeID]}
			res, err := s.DispatchCommand(nodeID, cmdType, args, actor)
			if err != nil {
				item.OK = false
				item.Error = err.Error()
			} else {
				item.OK = res.OK
				item.Result = res
				if !res.OK && res.Error != "" {
					item.Error = res.Error
				}
			}
			results[idx] = item
		}(i, id)
	}
	wg.Wait()
	return results, nil
}

func (t *browserTerminal) writeJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return t.conn.Write(ctx, websocket.MessageText, data)
}

// AttachBrowserTerminal bridges a browser WS to a remote agent PTY.
func (s *AgentService) AttachBrowserTerminal(nodeID uint, browser *websocket.Conn, cols, rows int) (string, error) {
	if err := (&SettingService{}).RequireControllerControl(); err != nil {
		return "", err
	}
	agentHubMu.RLock()
	session := agentSessions[nodeID]
	agentHubMu.RUnlock()
	if session == nil {
		return "", common.NewError("agent is not connected via WebSocket")
	}
	if cols < 20 {
		cols = 80
	}
	if rows < 5 {
		rows = 24
	}
	id := uuid.Must(uuid.NewV4()).String()
	_, cancel := context.WithCancel(context.Background())
	term := &browserTerminal{id: id, nodeID: nodeID, conn: browser, cancel: cancel}

	session.termsMu.Lock()
	session.terms[id] = term
	session.termsMu.Unlock()

	ctx, ccancel := context.WithTimeout(context.Background(), 10*time.Second)
	err := session.WriteJSON(ctx, map[string]interface{}{
		"type": agent.MsgTypeTerminalOpen,
		"id":   id,
		"cols": cols,
		"rows": rows,
	})
	ccancel()
	if err != nil {
		session.termsMu.Lock()
		delete(session.terms, id)
		session.termsMu.Unlock()
		cancel()
		return "", common.NewErrorf("open terminal failed: %v", err)
	}
	return id, nil
}

func (s *AgentService) DetachBrowserTerminal(nodeID uint, termID string) {
	agentHubMu.RLock()
	session := agentSessions[nodeID]
	agentHubMu.RUnlock()
	if session == nil {
		return
	}
	session.termsMu.Lock()
	term := session.terms[termID]
	delete(session.terms, termID)
	session.termsMu.Unlock()
	if term != nil {
		term.cancel()
	}
	_ = s.WriteToSession(context.Background(), nodeID, map[string]interface{}{
		"type": agent.MsgTypeTerminalClose,
		"id":   termID,
	})
}

func (s *AgentService) ForwardTerminalInput(nodeID uint, termID, dataB64 string) error {
	if err := (&SettingService{}).RequireControllerControl(); err != nil {
		return err
	}
	return s.WriteToSession(context.Background(), nodeID, map[string]interface{}{
		"type": agent.MsgTypeTerminalInput,
		"id":   termID,
		"data": dataB64,
	})
}

func (s *AgentService) ForwardTerminalResize(nodeID uint, termID string, cols, rows int) error {
	if err := (&SettingService{}).RequireControllerControl(); err != nil {
		return err
	}
	return s.WriteToSession(context.Background(), nodeID, map[string]interface{}{
		"type": agent.MsgTypeTerminalResize,
		"id":   termID,
		"cols": cols,
		"rows": rows,
	})
}

// HandleTerminalFromAgent routes PTY frames from agent WS to the matching browser terminal.
func (s *AgentService) HandleTerminalFromAgent(nodeID uint, msgType, termID, data, errMsg string) {
	agentHubMu.RLock()
	session := agentSessions[nodeID]
	agentHubMu.RUnlock()
	if session == nil {
		return
	}
	session.termsMu.Lock()
	term := session.terms[termID]
	session.termsMu.Unlock()
	if term == nil {
		return
	}
	payload := map[string]interface{}{"type": msgType, "id": termID}
	if data != "" {
		payload["data"] = data
	}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	_ = term.writeJSON(payload)
	if msgType == agent.MsgTypeTerminalClosed {
		session.termsMu.Lock()
		delete(session.terms, termID)
		session.termsMu.Unlock()
		term.cancel()
	}
}
