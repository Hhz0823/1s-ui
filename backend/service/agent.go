package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util/common"
	"gorm.io/gorm"
)

const (
	agentOnlineWindow    = 45 * time.Second
	agentHistoryLimit    = 120
	agentDefaultInterval = 15
	agentPairingTTL      = 15 * time.Minute
	agentAddressPairTTL  = 5 * time.Minute
	agentEnrollmentKey   = "agentEnrollmentKeyHash"
)

type AgentNodeView struct {
	Id           uint                `json:"id"`
	Name         string              `json:"name"`
	CreatedAt    int64               `json:"created_at"`
	LastSeen     int64               `json:"last_seen"`
	RemoteIP     string              `json:"remote_ip"`
	PublicHost   string              `json:"public_host"`
	Version      string              `json:"version"`
	Online       bool                `json:"online"`
	ConnMode     string              `json:"conn_mode,omitempty"`
	Report       agent.Report        `json:"report"`
	History      []AgentMetricSample `json:"history,omitempty"`
	WSConnected  bool                `json:"ws_connected,omitempty"`
	Controllable bool                `json:"controllable"`
	Commands     []agentCommandLog   `json:"commands,omitempty"`
	Latency      AgentLatencyView    `json:"latency"`
	Managed      bool                `json:"managed"`
}

type AgentEnrollment struct {
	Node          AgentNodeView `json:"node"`
	Token         string        `json:"token"`
	PairCode      string        `json:"pair_code"`
	PairExpiresAt int64         `json:"pair_expires_at"`
}

type AgentMetricSample struct {
	Time         int64   `json:"time"`
	CPUPercent   float64 `json:"cpu_percent"`
	MemPercent   float64 `json:"mem_percent"`
	SwapPercent  float64 `json:"swap_percent"`
	DiskPercent  float64 `json:"disk_percent"`
	ProcessCount int     `json:"process_count"`
	NetSent      uint64  `json:"net_sent_rate"`
	NetRecv      uint64  `json:"net_recv_rate"`
}

type AgentService struct {
	capacityProvider func() (int, uint64)
}

var (
	agentHistoryMu      sync.RWMutex
	agentHistory        = map[uint][]AgentMetricSample{}
	agentWSMu           sync.RWMutex
	agentWSLive         = map[uint]bool{}
	agentAddressPairing = struct {
		sync.Mutex
		expiresAt    time.Time
		targetNodeID uint
	}{}
)

func (s *AgentService) List() ([]AgentNodeView, error) {
	var nodes []model.AgentNode
	if err := database.GetDB().Order("name ASC, id ASC").Find(&nodes).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	result := make([]AgentNodeView, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, agentNodeView(node, now, false))
	}
	return result, nil
}

func (s *AgentService) Get(id uint) (*AgentNodeView, error) {
	var node model.AgentNode
	if err := database.GetDB().First(&node, id).Error; err != nil {
		return nil, err
	}
	view := agentNodeView(node, time.Now(), true)
	view.Commands = getAgentCommandLogs(id)
	return &view, nil
}

func (s *AgentService) Create(name string) (*AgentEnrollment, error) {
	return s.create(name, true)
}

// CreateConnected creates a node whose token is returned directly to an
// authenticated enrollment client. It does not leave an unused pairing code.
func (s *AgentService) CreateConnected(name string) (*AgentEnrollment, error) {
	return s.create(name, false)
}

// OpenAgentAddressPairing allows exactly one address-only enrollment. Keeping
// the window in memory makes a restart revoke it automatically.
func (s *SettingService) OpenAgentAddressPairing(targetNodeID uint) int64 {
	agentAddressPairing.Lock()
	defer agentAddressPairing.Unlock()
	agentAddressPairing.expiresAt = time.Now().Add(agentAddressPairTTL)
	agentAddressPairing.targetNodeID = targetNodeID
	return agentAddressPairing.expiresAt.Unix()
}

func (s *SettingService) ConsumeAgentAddressPairing() (uint, error) {
	agentAddressPairing.Lock()
	defer agentAddressPairing.Unlock()
	if agentAddressPairing.expiresAt.IsZero() || time.Now().After(agentAddressPairing.expiresAt) {
		agentAddressPairing.expiresAt = time.Time{}
		agentAddressPairing.targetNodeID = 0
		return 0, common.NewError("address connection window is closed or expired")
	}
	targetNodeID := agentAddressPairing.targetNodeID
	agentAddressPairing.expiresAt = time.Time{}
	agentAddressPairing.targetNodeID = 0
	return targetNodeID, nil
}

func (s *SettingService) CloseAgentAddressPairing() {
	agentAddressPairing.Lock()
	agentAddressPairing.expiresAt = time.Time{}
	agentAddressPairing.targetNodeID = 0
	agentAddressPairing.Unlock()
}

func (s *AgentService) create(name string, includePairing bool) (*AgentEnrollment, error) {
	name, err := normalizeAgentName(name)
	if err != nil {
		return nil, err
	}
	cpuCount, memTotal := currentClusterCapacity()
	if s.capacityProvider != nil {
		cpuCount, memTotal = s.capacityProvider()
	}
	if err := controllerHostError(cpuCount, memTotal, singboxOnlyRuntime()); err != nil {
		return nil, err
	}
	token, hash, err := newAgentToken()
	if err != nil {
		return nil, err
	}
	var pairCode, pairHash string
	var pairExpiresAt int64
	if includePairing {
		pairCode, pairHash, err = newAgentToken()
		if err != nil {
			return nil, err
		}
		pairExpiresAt = time.Now().Add(agentPairingTTL).Unix()
	}
	node := model.AgentNode{
		Name: name, TokenHash: hash, PairCodeHash: pairHash, PairExpiresAt: pairExpiresAt,
		CreatedAt: time.Now().Unix(), Report: json.RawMessage(`{}`),
	}
	if err := database.GetDB().Create(&node).Error; err != nil {
		return nil, err
	}
	invalidateHostRequirementsCache()
	view := agentNodeView(node, time.Now(), false)
	return &AgentEnrollment{Node: view, Token: token, PairCode: pairCode, PairExpiresAt: pairExpiresAt}, nil
}

func (s *AgentService) Rotate(id uint) (*AgentEnrollment, error) {
	if err := (&SettingService{}).RequireControllerControl(); err != nil {
		return nil, err
	}
	return s.rotateToken(id, true)
}

// Reconnect rotates an existing node only after an authenticated administrator
// has opened and the child has consumed its address-only connection window.
func (s *AgentService) Reconnect(id uint) (*AgentEnrollment, error) {
	return s.rotateToken(id, false)
}

func (s *AgentService) rotateToken(id uint, includePairing bool) (*AgentEnrollment, error) {
	var node model.AgentNode
	if err := database.GetDB().First(&node, id).Error; err != nil {
		return nil, err
	}
	token, hash, err := newAgentToken()
	if err != nil {
		return nil, err
	}
	var pairCode, pairHash string
	var pairExpiresAt int64
	if includePairing {
		pairCode, pairHash, err = newAgentToken()
		if err != nil {
			return nil, err
		}
		pairExpiresAt = time.Now().Add(agentPairingTTL).Unix()
	}
	if err := database.GetDB().Model(&node).Updates(map[string]interface{}{
		"token_hash": hash, "pair_code_hash": pairHash, "pair_expires_at": pairExpiresAt, "last_seen": 0,
	}).Error; err != nil {
		return nil, err
	}
	node.TokenHash = hash
	node.PairCodeHash = pairHash
	node.PairExpiresAt = pairExpiresAt
	node.LastSeen = 0
	view := agentNodeView(node, time.Now(), false)
	return &AgentEnrollment{Node: view, Token: token, PairCode: pairCode, PairExpiresAt: pairExpiresAt}, nil
}

// ConsumePairingCode exchanges a short-lived, single-use code for a fresh
// Agent token. Rotating during the exchange invalidates the temporary token
// shown by older controller UIs.
func (s *AgentService) ConsumePairingCode(code string) (*AgentEnrollment, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, common.NewError("missing agent pairing code")
	}
	if len(code) < 32 || len(code) > 128 {
		return nil, common.NewError("invalid or expired agent pairing code")
	}

	token, tokenHash, err := newAgentToken()
	if err != nil {
		return nil, err
	}
	pairHash := hashAgentToken(code)
	now := time.Now().Unix()
	var node model.AgentNode
	err = database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("pair_code_hash = ? AND pair_expires_at >= ?", pairHash, now).First(&node).Error; err != nil {
			return common.NewError("invalid or expired agent pairing code")
		}
		result := tx.Model(&model.AgentNode{}).
			Where("id = ? AND pair_code_hash = ? AND pair_expires_at >= ?", node.Id, pairHash, now).
			Updates(map[string]interface{}{
				"token_hash": tokenHash, "pair_code_hash": "", "pair_expires_at": 0, "last_seen": 0,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return common.NewError("invalid or expired agent pairing code")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	node.TokenHash = tokenHash
	node.PairCodeHash = ""
	node.PairExpiresAt = 0
	node.LastSeen = 0
	view := agentNodeView(node, time.Now(), false)
	return &AgentEnrollment{Node: view, Token: token}, nil
}

func (s *AgentService) Update(id uint, name, publicHost string) (*AgentNodeView, error) {
	if err := (&SettingService{}).RequireControllerControl(); err != nil {
		return nil, err
	}
	name, err := normalizeAgentName(name)
	if err != nil {
		return nil, err
	}
	publicHost, err = normalizeAgentPublicHost(publicHost)
	if err != nil {
		return nil, err
	}
	var node model.AgentNode
	if err := database.GetDB().First(&node, id).Error; err != nil {
		return nil, err
	}
	if err := database.GetDB().Model(&node).Updates(map[string]interface{}{
		"name": name, "public_host": publicHost,
	}).Error; err != nil {
		return nil, err
	}
	node.Name = name
	node.PublicHost = publicHost
	view := agentNodeView(node, time.Now(), true)
	return &view, nil
}

func (s *AgentService) Delete(id uint) error {
	if err := (&SettingService{}).RequireControllerControl(); err != nil {
		return err
	}
	result := database.GetDB().Delete(&model.AgentNode{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return common.NewError("agent node not found")
	}
	invalidateHostRequirementsCache()
	agentHistoryMu.Lock()
	delete(agentHistory, id)
	agentHistoryMu.Unlock()
	agentWSMu.Lock()
	delete(agentWSLive, id)
	agentWSMu.Unlock()
	agentCmdLogMu.Lock()
	delete(agentCmdLogs, id)
	agentCmdLogMu.Unlock()
	agentLatencyMu.Lock()
	delete(agentLatencies, id)
	agentLatencyMu.Unlock()
	return nil
}

func (s *AgentService) Heartbeat(token, remoteIP string, report agent.Report) (*agent.HeartbeatResponse, error) {
	nodeID, err := s.applyReport(token, remoteIP, report)
	if err != nil {
		return nil, err
	}
	_ = nodeID
	return &agent.HeartbeatResponse{
		ServerTime:      time.Now().Unix(),
		IntervalSeconds: agentDefaultInterval,
	}, nil
}

func (s *AgentService) applyReport(token, remoteIP string, report agent.Report) (uint, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, common.NewError("missing agent token")
	}
	if len(report.Hostname) > 255 || len(report.AgentVersion) > 64 || len(report.OS) > 32 || len(report.Arch) > 32 {
		return 0, common.NewError("invalid agent report")
	}
	if report.ConnMode != "" && report.ConnMode != "http" && report.ConnMode != "ws" {
		return 0, common.NewError("invalid agent conn_mode")
	}
	payload, err := json.Marshal(report)
	if err != nil {
		return 0, err
	}
	hash := hashAgentToken(token)
	var node model.AgentNode
	if err := database.GetDB().Where("token_hash = ?", hash).First(&node).Error; err != nil {
		return 0, common.NewError("invalid agent token")
	}
	result := database.GetDB().Model(&node).Updates(map[string]interface{}{
		"last_seen": time.Now().Unix(),
		"remote_ip": strings.TrimSpace(remoteIP),
		"version":   report.AgentVersion,
		"report":    payload,
	})
	if result.Error != nil {
		return 0, result.Error
	}
	appendAgentHistory(node.Id, report)
	return node.Id, nil
}

func (s *AgentService) AuthenticateToken(token string) (uint, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, common.NewError("missing agent token")
	}
	hash := hashAgentToken(token)
	var node model.AgentNode
	if err := database.GetDB().Select("id").Where("token_hash = ?", hash).First(&node).Error; err != nil {
		return 0, common.NewError("invalid agent token")
	}
	return node.Id, nil
}

func (s *AgentService) SetWSConnected(id uint, connected bool) {
	agentWSMu.Lock()
	defer agentWSMu.Unlock()
	if connected {
		agentWSLive[id] = true
	} else {
		delete(agentWSLive, id)
	}
}

func appendAgentHistory(id uint, report agent.Report) {
	sample := AgentMetricSample{
		Time:         time.Now().Unix(),
		CPUPercent:   report.CPUPercent,
		ProcessCount: report.ProcessCount,
		NetSent:      report.NetRate.Sent,
		NetRecv:      report.NetRate.Recv,
	}
	if report.Memory.Total > 0 {
		sample.MemPercent = float64(report.Memory.Used) * 100 / float64(report.Memory.Total)
	}
	if report.Swap.Total > 0 {
		sample.SwapPercent = float64(report.Swap.Used) * 100 / float64(report.Swap.Total)
	}
	if report.Disk.Total > 0 {
		sample.DiskPercent = float64(report.Disk.Used) * 100 / float64(report.Disk.Total)
	}
	agentHistoryMu.Lock()
	defer agentHistoryMu.Unlock()
	history := append(agentHistory[id], sample)
	if len(history) > agentHistoryLimit {
		history = history[len(history)-agentHistoryLimit:]
	}
	agentHistory[id] = history
}

func getAgentHistory(id uint) []AgentMetricSample {
	agentHistoryMu.RLock()
	defer agentHistoryMu.RUnlock()
	src := agentHistory[id]
	if len(src) == 0 {
		return nil
	}
	out := make([]AgentMetricSample, len(src))
	copy(out, src)
	return out
}

func agentNodeView(node model.AgentNode, now time.Time, withHistory bool) AgentNodeView {
	view := AgentNodeView{
		Id: node.Id, Name: node.Name, CreatedAt: node.CreatedAt, LastSeen: node.LastSeen,
		RemoteIP: node.RemoteIP, PublicHost: node.PublicHost, Version: node.Version,
	}
	if len(node.Report) > 0 {
		_ = json.Unmarshal(node.Report, &view.Report)
		view.ConnMode = view.Report.ConnMode
	}
	view.Online = node.LastSeen > 0 && now.Sub(time.Unix(node.LastSeen, 0)) <= agentOnlineWindow
	agentHubMu.RLock()
	_, live := agentSessions[node.Id]
	agentHubMu.RUnlock()
	view.WSConnected = live
	view.Controllable = live
	view.Managed = live && view.Report.Panel.ControlAvailable
	view.Latency = getAgentLatency(node.Id)
	if withHistory {
		view.History = getAgentHistory(node.Id)
	}
	return view
}

// ManagedNodePublicHost returns the address other machines should use to reach
// a managed server: the configured public host, else its best public IP.
func ManagedNodePublicHost(node *AgentNodeView) string {
	if node == nil {
		return ""
	}
	if value := strings.TrimSpace(node.PublicHost); value != "" {
		return strings.Trim(value, "[]")
	}
	candidates := append([]string{node.RemoteIP}, node.Report.IPv4...)
	candidates = append(candidates, node.Report.IPv6...)
	for _, candidate := range candidates {
		candidate = strings.Trim(strings.TrimSpace(candidate), "[]")
		ip := net.ParseIP(candidate)
		if ip != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() {
			return candidate
		}
	}
	for _, candidate := range candidates {
		candidate = strings.Trim(strings.TrimSpace(candidate), "[]")
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func normalizeAgentPublicHost(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if len(value) > 255 || strings.ContainsAny(value, " /?#@\t\r\n") {
		return "", common.NewError("public host must be a hostname or IP address without scheme, path, or port")
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
	}
	if strings.Contains(value, ":") && net.ParseIP(value) == nil {
		return "", common.NewError("public host must not include a port")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", common.NewError("public host contains control characters")
		}
	}
	return value, nil
}

func normalizeAgentName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > 80 {
		return "", common.NewError("agent name must contain 1-80 characters")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", common.NewError("agent name contains control characters")
		}
	}
	return value, nil
}

func newAgentToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate agent token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, hashAgentToken(token), nil
}

func hashAgentToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *SettingService) RotateAgentEnrollmentKey() (string, error) {
	token, hash, err := newAgentToken()
	if err != nil {
		return "", err
	}
	if err := s.setString(agentEnrollmentKey, hash); err != nil {
		return "", err
	}
	return token, nil
}

func (s *SettingService) ValidateAgentEnrollmentKey(token string) error {
	token = strings.TrimSpace(token)
	if !validAgentCredential(token) {
		return common.NewError("invalid controller enrollment key")
	}
	expected, err := s.getString(agentEnrollmentKey)
	if err != nil || len(expected) != sha256.Size*2 {
		return common.NewError("invalid controller enrollment key")
	}
	actual := hashAgentToken(token)
	if subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
		return common.NewError("invalid controller enrollment key")
	}
	return nil
}
