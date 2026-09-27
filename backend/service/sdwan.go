package service

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/util"
	"github.com/Hhz0823/1s-ui/util/common"

	"gorm.io/gorm"
)

// SD-WAN turns the controller into a smart gateway: every managed server gets
// private uplinks (by default VLESS Reality over TCP and Hysteria2 over QUIC),
// the controller keeps every path in a sing-box urltest group and routes the
// selected entry inbounds through whichever path is currently fastest, failing
// over automatically when a path or server breaks.
const (
	sdwanConfigKey        = "sdwanConfig"
	SdwanGroupTag         = "sdwan-auto"
	sdwanMemberTagPrefix  = "sdwan-node-"
	SdwanModeAuto         = "auto"
	defaultSdwanTestURL   = "https://www.gstatic.com/generate_204"
	defaultSdwanSpeedURL  = "https://speed.cloudflare.com/__down?bytes=10000000"
	defaultSdwanInterval  = 60
	defaultSdwanTolerance = 50
	maxSdwanEntryInbounds = 256
)

type SdwanSettings struct {
	// Enabled routes the entry inbounds through the SD-WAN group. Paths and
	// the group exist whenever members are configured, so latency can be
	// checked (and custom rules can use sdwan-auto) before switching traffic.
	Enabled       bool     `json:"enabled"`
	Mode          string   `json:"mode"`
	EntryInbounds []string `json:"entry_inbounds"`
	TestURL       string   `json:"test_url"`
	SpeedTestURL  string   `json:"speed_test_url"`
	Interval      int      `json:"interval"`
	Tolerance     int      `json:"tolerance"`
	RealityServer string   `json:"reality_server"`
	IncludeDirect bool     `json:"include_direct"`
	RulesFirst    bool     `json:"rules_first"`
}

// SdwanPath is one uplink of a member server as the controller dials it.
type SdwanPath struct {
	Protocol string          `json:"protocol"`
	Port     int             `json:"port"`
	Detail   string          `json:"detail,omitempty"`
	Outbound json.RawMessage `json:"outbound"`
	Disabled bool            `json:"disabled,omitempty"`
	Reason   string          `json:"reason,omitempty"`
}

type SdwanPathView struct {
	Protocol string  `json:"protocol"`
	Tag      string  `json:"tag"`
	Port     int     `json:"port"`
	Detail   string  `json:"detail,omitempty"`
	Disabled bool    `json:"disabled"`
	Reason   string  `json:"reason,omitempty"`
	Delay    *uint16 `json:"delay,omitempty"`
	DelayAt  int64   `json:"delay_at,omitempty"`
	Selected bool    `json:"selected"`
}

type SdwanMemberView struct {
	NodeID      uint             `json:"node_id"`
	Name        string           `json:"name"`
	Server      string           `json:"server"`
	UpdatedAt   int64            `json:"updated_at"`
	Online      bool             `json:"online"`
	Managed     bool             `json:"managed"`
	Latency     AgentLatencyView `json:"latency"`
	CPUPercent  float64          `json:"cpu_percent"`
	NetSentRate uint64           `json:"net_sent_rate"`
	NetRecvRate uint64           `json:"net_recv_rate"`
	Paths       []SdwanPathView  `json:"paths"`
	Desired     []string         `json:"desired"`
	Selected    bool             `json:"selected"`
	NeedsResync bool             `json:"needs_resync"`
}

type SdwanNodeOption struct {
	NodeID     uint   `json:"node_id"`
	Name       string `json:"name"`
	Online     bool   `json:"online"`
	Managed    bool   `json:"managed"`
	Supported  bool   `json:"supported"`
	PublicHost string `json:"public_host"`
	Member     bool   `json:"member"`
}

type SdwanState struct {
	Settings     SdwanSettings     `json:"settings"`
	Members      []SdwanMemberView `json:"members"`
	Nodes        []SdwanNodeOption `json:"nodes"`
	Inbounds     []string          `json:"inbounds"`
	Protocols    []string          `json:"protocols"`
	GroupTag     string            `json:"group_tag"`
	Active       bool              `json:"active"`
	Selected     string            `json:"selected,omitempty"`
	SelectedName string            `json:"selected_name,omitempty"`
	CoreRunning  bool              `json:"core_running"`
	CanControl   bool              `json:"can_control"`
	Warnings     []string          `json:"warnings,omitempty"`
	Job          *SdwanJob         `json:"job,omitempty"`
}

type SdwanService struct {
	agents AgentService
	config ConfigService
	// restartCore applies a new SD-WAN topology; rpc, listNodes and getNode
	// reach the managed servers. All are replaceable in tests.
	restartCore func() error
	rpc         func(nodeID uint, method string, payload interface{}, actor string) (*agent.RPCResponse, error)
	listNodes   func() ([]AgentNodeView, error)
	getNode     func(id uint) (*AgentNodeView, error)
}

func (s *SdwanService) dispatch(nodeID uint, method string, payload interface{}, actor string) (*agent.RPCResponse, error) {
	if s.rpc != nil {
		return s.rpc(nodeID, method, payload, actor)
	}
	return s.agents.DispatchRPC(nodeID, method, payload, actor)
}

func (s *SdwanService) nodes() ([]AgentNodeView, error) {
	if s.listNodes != nil {
		return s.listNodes()
	}
	return s.agents.List()
}

func (s *SdwanService) node(id uint) (*AgentNodeView, error) {
	if s.getNode != nil {
		return s.getNode(id)
	}
	return s.agents.Get(id)
}

var sdwanMu sync.Mutex

var sdwanShortProtocol = map[string]string{
	SdwanProtocolReality:     "reality",
	SdwanProtocolHysteria2:   "hy2",
	SdwanProtocolShadowsocks: "ss",
}

func sdwanPathTag(nodeID uint, protocol string) string {
	return fmt.Sprintf("%s%d-%s", sdwanMemberTagPrefix, nodeID, sdwanShortProtocol[protocol])
}

func pathProtocols(paths []SdwanPath) []string {
	protocols := make([]string, 0, len(paths))
	for _, path := range paths {
		protocols = append(protocols, path.Protocol)
	}
	return protocols
}

func memberPaths(member model.SdwanMember) []SdwanPath {
	var paths []SdwanPath
	if len(member.Paths) > 0 {
		_ = json.Unmarshal(member.Paths, &paths)
	}
	return paths
}

func setMemberPaths(member *model.SdwanMember, paths []SdwanPath) error {
	raw, err := json.Marshal(paths)
	if err != nil {
		return err
	}
	member.Paths = raw
	return nil
}

func defaultSdwanSettings() SdwanSettings {
	return SdwanSettings{
		Mode:          SdwanModeAuto,
		EntryInbounds: []string{},
		TestURL:       defaultSdwanTestURL,
		SpeedTestURL:  defaultSdwanSpeedURL,
		Interval:      defaultSdwanInterval,
		Tolerance:     defaultSdwanTolerance,
	}
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func normalizeSdwanSettings(settings *SdwanSettings) error {
	settings.Mode = strings.ToLower(strings.TrimSpace(settings.Mode))
	if settings.Mode == "" {
		settings.Mode = SdwanModeAuto
	}
	if settings.Mode != SdwanModeAuto {
		protocol, err := normalizeSdwanProtocol(settings.Mode)
		if err != nil {
			return err
		}
		settings.Mode = protocol
	}
	settings.TestURL = strings.TrimSpace(settings.TestURL)
	if settings.TestURL == "" {
		settings.TestURL = defaultSdwanTestURL
	}
	if !validHTTPURL(settings.TestURL) {
		return common.NewError("SD-WAN test URL must be an http(s) URL")
	}
	settings.SpeedTestURL = strings.TrimSpace(settings.SpeedTestURL)
	if settings.SpeedTestURL == "" {
		settings.SpeedTestURL = defaultSdwanSpeedURL
	}
	if !validHTTPURL(settings.SpeedTestURL) {
		return common.NewError("SD-WAN speed test URL must be an http(s) URL")
	}
	if settings.Interval == 0 {
		settings.Interval = defaultSdwanInterval
	}
	if settings.Interval < 10 || settings.Interval > 1800 {
		return common.NewError("SD-WAN test interval must be between 10 and 1800 seconds")
	}
	if settings.Tolerance == 0 {
		settings.Tolerance = defaultSdwanTolerance
	}
	if settings.Tolerance < 1 || settings.Tolerance > 5000 {
		return common.NewError("SD-WAN tolerance must be between 1 and 5000 ms")
	}
	realityServer, err := normalizeRealityServer(settings.RealityServer)
	if err != nil {
		return err
	}
	settings.RealityServer = realityServer
	entries := make([]string, 0, len(settings.EntryInbounds))
	for _, tag := range settings.EntryInbounds {
		tag = strings.TrimSpace(tag)
		if tag == "" || slices.Contains(entries, tag) {
			continue
		}
		if len(tag) > 255 || strings.IndexFunc(tag, unicode.IsControl) >= 0 {
			return common.NewErrorf("invalid SD-WAN entry inbound %q", tag)
		}
		entries = append(entries, tag)
	}
	if len(entries) > maxSdwanEntryInbounds {
		return common.NewError("too many SD-WAN entry inbounds")
	}
	settings.EntryInbounds = entries
	return nil
}

func loadSdwanSettings(db *gorm.DB) (SdwanSettings, error) {
	settings := defaultSdwanSettings()
	var rows []model.Setting
	if err := db.Where("key = ?", sdwanConfigKey).Limit(1).Find(&rows).Error; err != nil {
		return settings, err
	}
	if len(rows) > 0 && strings.TrimSpace(rows[0].Value) != "" {
		if err := json.Unmarshal([]byte(rows[0].Value), &settings); err != nil {
			return defaultSdwanSettings(), err
		}
	}
	if settings.EntryInbounds == nil {
		settings.EntryInbounds = []string{}
	}
	if err := normalizeSdwanSettings(&settings); err != nil {
		return defaultSdwanSettings(), err
	}
	return settings, nil
}

func storeSdwanSettings(db *gorm.DB, settings SdwanSettings) error {
	raw, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	var existing model.Setting
	err = db.Where("key = ?", sdwanConfigKey).First(&existing).Error
	if database.IsNotFound(err) {
		return db.Create(&model.Setting{Key: sdwanConfigKey, Value: string(raw)}).Error
	}
	if err != nil {
		return err
	}
	return db.Model(&model.Setting{}).Where("id = ?", existing.Id).Update("value", string(raw)).Error
}

func (s *SdwanService) apply() error {
	if s.restartCore != nil {
		return s.restartCore()
	}
	if corePtr == nil {
		return nil
	}
	return s.config.RestartCore()
}

func sdwanNodeSupported(node *AgentNodeView) bool {
	return node != nil && node.Managed && slices.Contains(node.Report.Panel.Capabilities, agent.CapabilitySdwanV2)
}

// sdwanProtocolsFor resolves the configured mode into the uplink protocols that
// both the managed server and this controller can run. "auto" uses Reality
// (TCP, most robust and secure) plus Hysteria2 (QUIC, fastest on lossy
// links); Shadowsocks 2022 is the fallback when neither is available.
func sdwanProtocolsFor(mode string, capabilities []string) []string {
	usable := func(protocol string) bool {
		if !sdwanProtocolAvailable(protocol) {
			return false
		}
		switch protocol {
		case SdwanProtocolReality:
			return slices.Contains(capabilities, agent.CapabilitySdwanReality)
		case SdwanProtocolHysteria2:
			return slices.Contains(capabilities, agent.CapabilitySdwanHysteria2)
		}
		return true
	}
	if mode != SdwanModeAuto {
		if usable(mode) {
			return []string{mode}
		}
		return []string{SdwanProtocolShadowsocks}
	}
	var protocols []string
	for _, protocol := range []string{SdwanProtocolReality, SdwanProtocolHysteria2} {
		if usable(protocol) {
			protocols = append(protocols, protocol)
		}
	}
	if len(protocols) == 0 {
		protocols = []string{SdwanProtocolShadowsocks}
	}
	return protocols
}

// SdwanState reports configuration, members, runtime selection and candidates.
func (s *SdwanService) SdwanState() (*SdwanState, error) {
	db := database.GetDB()
	state := &SdwanState{
		GroupTag: SdwanGroupTag, Members: []SdwanMemberView{}, Nodes: []SdwanNodeOption{},
		Inbounds: []string{}, Protocols: localSdwanProtocols(), Job: currentSdwanJob(),
	}
	settings, err := loadSdwanSettings(db)
	if err != nil {
		state.Warnings = append(state.Warnings, "invalid stored SD-WAN settings were reset: "+err.Error())
	}
	state.Settings = settings
	if status, statusErr := (&SettingService{}).GetControllerModeStatus(); statusErr == nil {
		state.CanControl = status.Enabled && status.CanControl
	}
	if corePtr != nil {
		state.CoreRunning = corePtr.IsRunning()
	}

	var inboundTags []string
	if err = db.Model(&model.Inbound{}).
		Where("core_type = ? OR core_type = '' OR core_type IS NULL", model.CoreTypeSingBox).
		Order("id").Pluck("tag", &inboundTags).Error; err != nil {
		return nil, err
	}
	for _, tag := range inboundTags {
		if !isSdwanUplinkTag(tag) {
			state.Inbounds = append(state.Inbounds, tag)
		}
	}

	nodes, err := s.nodes()
	if err != nil {
		return nil, err
	}
	nodeByID := make(map[uint]AgentNodeView, len(nodes))
	for _, node := range nodes {
		nodeByID[node.Id] = node
	}

	var members []model.SdwanMember
	if err = db.Order("node_id").Find(&members).Error; err != nil {
		return nil, err
	}
	memberIDs := map[uint]bool{}

	var groupStatus *core.OutboundGroupStatus
	if state.CoreRunning {
		if status, statusErr := corePtr.GroupStatus(SdwanGroupTag); statusErr == nil {
			groupStatus = status
			state.Active = true
			state.Selected = status.Now
		}
	}
	for _, member := range members {
		memberIDs[member.NodeId] = true
		node := nodeByID[member.NodeId]
		paths := memberPaths(member)
		view := SdwanMemberView{
			NodeID: member.NodeId, Name: node.Name, Server: member.Server, UpdatedAt: member.UpdatedAt,
			Online: node.Online, Managed: node.Managed, Latency: node.Latency,
			CPUPercent: node.Report.CPUPercent, NetSentRate: node.Report.NetRate.Sent, NetRecvRate: node.Report.NetRate.Recv,
			Paths:   []SdwanPathView{},
			Desired: sdwanProtocolsFor(settings.Mode, node.Report.Panel.Capabilities),
		}
		if view.Name == "" {
			view.Name = fmt.Sprintf("#%d", member.NodeId)
		}
		view.NeedsResync = !sameProtocolSet(pathProtocols(paths), view.Desired)
		for _, path := range paths {
			pathView := SdwanPathView{
				Protocol: path.Protocol, Tag: sdwanPathTag(member.NodeId, path.Protocol), Port: path.Port,
				Detail: path.Detail, Disabled: path.Disabled, Reason: path.Reason,
			}
			if groupStatus != nil {
				if delay, ok := groupStatus.Delays[pathView.Tag]; ok {
					value := delay.Delay
					pathView.Delay = &value
					pathView.DelayAt = delay.Time.Unix()
				}
				pathView.Selected = groupStatus.Now == pathView.Tag
			}
			if pathView.Selected {
				view.Selected = true
				state.SelectedName = view.Name + " · " + path.Protocol
			}
			view.Paths = append(view.Paths, pathView)
		}
		state.Members = append(state.Members, view)
	}
	if state.Selected == "direct" {
		state.SelectedName = "direct"
	}
	for _, node := range nodes {
		nodeCopy := node
		state.Nodes = append(state.Nodes, SdwanNodeOption{
			NodeID: node.Id, Name: node.Name, Online: node.Online, Managed: node.Managed,
			Supported: sdwanNodeSupported(&nodeCopy), PublicHost: ManagedNodePublicHost(&nodeCopy),
			Member: memberIDs[node.Id],
		})
	}
	return state, nil
}

// SaveSdwanSettings stores the settings, re-provisions members when the uplink
// mode or Reality target changed, and restarts sing-box so routing follows.
func (s *SdwanService) SaveSdwanSettings(settings SdwanSettings, actor string) (*SdwanState, error) {
	if err := (&SettingService{}).RequireControllerControl(); err != nil {
		return nil, err
	}
	if sdwanJobRunning() {
		return nil, errSdwanJobRunning
	}
	if err := normalizeSdwanSettings(&settings); err != nil {
		return nil, err
	}
	sdwanMu.Lock()
	db := database.GetDB()
	previous, _ := loadSdwanSettings(db)
	if err := storeSdwanSettings(db, settings); err != nil {
		sdwanMu.Unlock()
		return nil, err
	}
	var warnings []string
	if previous.Mode != settings.Mode || previous.RealityServer != settings.RealityServer {
		warnings = s.resyncLocked(settings, actor)
	}
	applyErr := s.apply()
	sdwanMu.Unlock()
	if applyErr != nil {
		return nil, common.NewError("SD-WAN settings saved but sing-box failed to restart: ", applyErr.Error())
	}
	return s.stateWithWarnings(warnings)
}

// AddSdwanMember provisions the uplinks on a managed server and adds its paths
// to the group. Calling it again refreshes them (e.g. after an IP change).
func (s *SdwanService) AddSdwanMember(nodeID uint, actor string) (*SdwanState, error) {
	if err := (&SettingService{}).RequireControllerControl(); err != nil {
		return nil, err
	}
	if sdwanJobRunning() {
		return nil, errSdwanJobRunning
	}
	sdwanMu.Lock()
	settings, _ := loadSdwanSettings(database.GetDB())
	err := s.provisionLocked(nodeID, settings, actor, false)
	if err == nil {
		err = s.apply()
		if err != nil {
			err = common.NewError("server added but sing-box failed to restart: ", err.Error())
		}
	}
	sdwanMu.Unlock()
	if err != nil {
		return nil, err
	}
	return s.SdwanState()
}

// RemoveSdwanMember removes a member and deletes its uplinks on the managed
// server when that server is reachable.
func (s *SdwanService) RemoveSdwanMember(nodeID uint, actor string) (*SdwanState, error) {
	if err := (&SettingService{}).RequireControllerControl(); err != nil {
		return nil, err
	}
	if sdwanJobRunning() {
		return nil, errSdwanJobRunning
	}
	sdwanMu.Lock()
	db := database.GetDB()
	result := db.Where("node_id = ?", nodeID).Delete(&model.SdwanMember{})
	if result.Error != nil {
		sdwanMu.Unlock()
		return nil, result.Error
	}
	var warnings []string
	if node, err := s.node(nodeID); err == nil && sdwanNodeSupported(node) {
		request := SdwanRemoveRequest{Actor: actor, PublicHost: ManagedNodePublicHost(node)}
		if _, rpcErr := s.dispatch(nodeID, agent.RPCMethodSdwanRemove, request, actor); rpcErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: uplinks were not removed on the server: %v", node.Name, rpcErr))
		}
	} else {
		warnings = append(warnings, "the server is offline; remove its sdwan-uplink inbounds manually if they still exist")
	}
	applyErr := s.apply()
	sdwanMu.Unlock()
	if applyErr != nil {
		return nil, common.NewError("server removed but sing-box failed to restart: ", applyErr.Error())
	}
	return s.stateWithWarnings(warnings)
}

// ResyncSdwan re-provisions every member with the current settings.
func (s *SdwanService) ResyncSdwan(actor string) (*SdwanState, error) {
	if err := (&SettingService{}).RequireControllerControl(); err != nil {
		return nil, err
	}
	if sdwanJobRunning() {
		return nil, errSdwanJobRunning
	}
	sdwanMu.Lock()
	settings, _ := loadSdwanSettings(database.GetDB())
	warnings := s.resyncLocked(settings, actor)
	applyErr := s.apply()
	sdwanMu.Unlock()
	if applyErr != nil {
		return nil, common.NewError("SD-WAN synced but sing-box failed to restart: ", applyErr.Error())
	}
	return s.stateWithWarnings(warnings)
}

// TestSdwan re-measures every path through the running group right now and
// lets the group switch to the fastest one.
func (s *SdwanService) TestSdwan() (*SdwanState, error) {
	if corePtr == nil || !corePtr.IsRunning() {
		return nil, common.NewError("sing-box is not running")
	}
	if err := corePtr.GroupCheckNow(SdwanGroupTag); err != nil {
		return nil, common.NewError("SD-WAN latency test failed: ", err.Error())
	}
	return s.SdwanState()
}

// HealSdwan is run by a short cron job so a failed exit is replaced within
// seconds instead of waiting for the next periodic test.
func (s *SdwanService) HealSdwan() {
	if corePtr == nil || !corePtr.IsRunning() {
		return
	}
	if switched, err := corePtr.GroupHeal(SdwanGroupTag); err == nil && switched {
		if status, statusErr := corePtr.GroupStatus(SdwanGroupTag); statusErr == nil {
			logger.Info("SD-WAN re-selected exit after a failure: ", status.Now)
		}
	}
}

func (s *SdwanService) stateWithWarnings(warnings []string) (*SdwanState, error) {
	state, err := s.SdwanState()
	if err != nil {
		return nil, err
	}
	state.Warnings = append(state.Warnings, warnings...)
	return state, nil
}

func (s *SdwanService) resyncLocked(settings SdwanSettings, actor string) []string {
	var members []model.SdwanMember
	if err := database.GetDB().Order("node_id").Find(&members).Error; err != nil {
		return []string{err.Error()}
	}
	var warnings []string
	for _, member := range members {
		if err := s.provisionLocked(member.NodeId, settings, actor, false); err != nil {
			warnings = append(warnings, fmt.Sprintf("#%d: %v", member.NodeId, err))
		}
	}
	return warnings
}

// provisionLocked (re)creates the uplinks of one member; rotate replaces the
// existing uplinks and credentials with new ones on new ports.
func (s *SdwanService) provisionLocked(nodeID uint, settings SdwanSettings, actor string, rotate bool) error {
	node, err := s.node(nodeID)
	if err != nil {
		return common.NewError("managed server not found")
	}
	if !node.Managed {
		return common.NewErrorf("%s is offline or not managed by this controller", node.Name)
	}
	if !sdwanNodeSupported(node) {
		return common.NewErrorf("%s runs a 1S-UI version without SD-WAN support; update it first", node.Name)
	}
	publicHost := ManagedNodePublicHost(node)
	if publicHost == "" {
		return common.NewErrorf("%s has no public address; set one on the server page", node.Name)
	}
	request := SdwanProvisionRequest{
		Protocols:     sdwanProtocolsFor(settings.Mode, node.Report.Panel.Capabilities),
		RealityServer: settings.RealityServer,
		Rotate:        rotate,
		Actor:         actor,
		PublicHost:    publicHost,
	}
	response, err := s.dispatch(nodeID, agent.RPCMethodSdwanProvision, request, actor)
	if err != nil {
		return err
	}
	var provisioned SdwanProvisionResponse
	if err = json.Unmarshal(response.Payload, &provisioned); err != nil {
		return err
	}
	return saveSdwanMemberUplinks(nodeID, provisioned)
}

func saveSdwanMemberUplinks(nodeID uint, provisioned SdwanProvisionResponse) error {
	paths := make([]SdwanPath, 0, len(provisioned.Uplinks))
	for _, uplink := range provisioned.Uplinks {
		protocol, err := normalizeSdwanProtocol(uplink.Protocol)
		if err != nil {
			return err
		}
		outbound, err := sdwanMemberOutbound(sdwanPathTag(nodeID, protocol), uplink.Outbound)
		if err != nil {
			return common.NewErrorf("the %s uplink is unusable: %v", protocol, err)
		}
		paths = append(paths, SdwanPath{Protocol: protocol, Port: uplink.Port, Detail: uplink.Detail, Outbound: outbound})
	}
	if len(paths) == 0 {
		return common.NewError("the server returned no SD-WAN uplinks")
	}
	db := database.GetDB()
	var existing model.SdwanMember
	err := db.Where("node_id = ?", nodeID).First(&existing).Error
	if err != nil && !database.IsNotFound(err) {
		return err
	}
	found := err == nil
	if found {
		// A path taken out of the group (e.g. Hysteria2 behind a UDP
		// firewall) stays out while its uplink is unchanged.
		for _, previous := range memberPaths(existing) {
			for index := range paths {
				if paths[index].Protocol == previous.Protocol && paths[index].Port == previous.Port &&
					string(paths[index].Outbound) == string(previous.Outbound) {
					paths[index].Disabled, paths[index].Reason = previous.Disabled, previous.Reason
				}
			}
		}
	}
	now := time.Now().Unix()
	member := model.SdwanMember{NodeId: nodeID, Server: provisioned.Server, CreatedAt: now, UpdatedAt: now}
	if err = setMemberPaths(&member, paths); err != nil {
		return err
	}
	if !found {
		return db.Create(&member).Error
	}
	return db.Model(&existing).Updates(map[string]interface{}{
		"server": member.Server, "paths": member.Paths, "updated_at": now,
	}).Error
}

// updateSdwanPathState enables or disables one path of a member.
func updateSdwanPathState(nodeID uint, protocol string, disabled bool, reason string) error {
	db := database.GetDB()
	var member model.SdwanMember
	if err := db.Where("node_id = ?", nodeID).First(&member).Error; err != nil {
		return err
	}
	paths := memberPaths(member)
	for index := range paths {
		if paths[index].Protocol == protocol {
			paths[index].Disabled = disabled
			paths[index].Reason = reason
		}
	}
	if err := setMemberPaths(&member, paths); err != nil {
		return err
	}
	return db.Model(&member).Updates(map[string]interface{}{"paths": member.Paths, "updated_at": time.Now().Unix()}).Error
}

// sdwanMemberOutbound tags and validates an uplink outbound so a broken path
// can never stop sing-box from starting.
func sdwanMemberOutbound(tag string, raw json.RawMessage) (json.RawMessage, error) {
	var outbound map[string]interface{}
	if err := json.Unmarshal(raw, &outbound); err != nil {
		return nil, err
	}
	switch outbound["type"] {
	case "vless", "hysteria2", "shadowsocks":
	default:
		return nil, common.NewErrorf("unexpected outbound type %v", outbound["type"])
	}
	for _, key := range []string{"detour", "bind_interface", "inet4_bind_address", "inet6_bind_address", "routing_mark", "netns"} {
		delete(outbound, key)
	}
	outbound["tag"] = tag
	util.NormalizeSingBoxOutbound(outbound)
	tagged, err := json.Marshal(outbound)
	if err != nil {
		return nil, err
	}
	if err = core.ValidateOutboundJSON(tagged); err != nil {
		return nil, err
	}
	return tagged, nil
}

func configTags(items []json.RawMessage) map[string]string {
	tags := make(map[string]string, len(items))
	for _, item := range items {
		var fields struct {
			Type string `json:"type"`
			Tag  string `json:"tag"`
		}
		if json.Unmarshal(item, &fields) == nil && fields.Tag != "" {
			tags[fields.Tag] = fields.Type
		}
	}
	return tags
}

// sdwanActivePaths returns the validated outbounds of every path this
// controller build can dial, and the tags of the enabled ones that join the
// group. Disabled paths keep their outbound so detection can re-test them.
func sdwanActivePaths(members []model.SdwanMember, existing map[string]string) ([]string, []json.RawMessage) {
	var groupTags []string
	var outbounds []json.RawMessage
	for _, member := range members {
		for _, path := range memberPaths(member) {
			tag := sdwanPathTag(member.NodeId, path.Protocol)
			if !sdwanProtocolAvailable(path.Protocol) {
				continue
			}
			if _, taken := existing[tag]; taken {
				logger.Warning("SD-WAN path skipped: outbound tag ", tag, " is already used")
				continue
			}
			outbound, err := sdwanMemberOutbound(tag, path.Outbound)
			if err != nil {
				logger.Warning("SD-WAN path ", tag, " skipped: ", err)
				continue
			}
			outbounds = append(outbounds, outbound)
			if !path.Disabled {
				groupTags = append(groupTags, tag)
			}
		}
	}
	return groupTags, outbounds
}

// applySdwanConfig adds path outbounds, the urltest group and, when routing is
// enabled, the entry rule to a sing-box configuration being built.
func applySdwanConfig(db *gorm.DB, singboxConfig *SingBoxConfig) {
	var members []model.SdwanMember
	if err := db.Order("node_id").Find(&members).Error; err != nil || len(members) == 0 {
		return
	}
	settings, err := loadSdwanSettings(db)
	if err != nil {
		logger.Warning("SD-WAN settings are invalid, using defaults: ", err)
	}
	existing := configTags(singboxConfig.Outbounds)
	for tag, kind := range configTags(singboxConfig.Endpoints) {
		existing[tag] = kind
	}
	if _, taken := existing[SdwanGroupTag]; taken {
		logger.Warning("SD-WAN disabled: outbound tag ", SdwanGroupTag, " is already used")
		return
	}
	pathTags, pathOutbounds := sdwanActivePaths(members, existing)
	if len(pathTags) == 0 {
		// Every path is disabled: keep them dialable for detection only.
		singboxConfig.Outbounds = append(singboxConfig.Outbounds, pathOutbounds...)
		return
	}
	groupMembers := append([]string(nil), pathTags...)
	if settings.IncludeDirect && existing["direct"] == "direct" {
		groupMembers = append(groupMembers, "direct")
	}
	group, err := json.Marshal(map[string]interface{}{
		"type":      "urltest",
		"tag":       SdwanGroupTag,
		"outbounds": groupMembers,
		"url":       settings.TestURL,
		"interval":  fmt.Sprintf("%ds", settings.Interval),
		"tolerance": settings.Tolerance,
		// Keep measuring even while idle so failover and the dashboard stay fresh.
		"idle_timeout": fmt.Sprintf("%ds", max(settings.Interval*30, 1800)),
	})
	if err != nil {
		return
	}
	singboxConfig.Outbounds = append(singboxConfig.Outbounds, pathOutbounds...)
	singboxConfig.Outbounds = append(singboxConfig.Outbounds, group)

	if !settings.Enabled {
		return
	}
	inbounds := configTags(singboxConfig.Inbounds)
	var entries []string
	for _, tag := range settings.EntryInbounds {
		if _, ok := inbounds[tag]; ok && !isSdwanUplinkTag(tag) {
			entries = append(entries, tag)
		}
	}
	if len(entries) == 0 {
		return
	}
	route, err := injectSdwanRouteRule(singboxConfig.Route, entries, settings.RulesFirst)
	if err != nil {
		logger.Warning("SD-WAN route rule skipped: ", err)
		return
	}
	singboxConfig.Route = route
}

// injectSdwanRouteRule places the entry rule after leading non-final actions
// (sniff, resolve, DNS hijack) so they keep working, or after every custom rule
// when rulesFirst is set.
func injectSdwanRouteRule(route json.RawMessage, entries []string, rulesFirst bool) (json.RawMessage, error) {
	routeMap := map[string]interface{}{}
	if trimmed := strings.TrimSpace(string(route)); trimmed != "" && trimmed != "null" {
		if err := json.Unmarshal(route, &routeMap); err != nil {
			return nil, err
		}
	}
	rules, _ := routeMap["rules"].([]interface{})
	index := len(rules)
	if !rulesFirst {
		index = 0
		for index < len(rules) {
			rule, _ := rules[index].(map[string]interface{})
			action, _ := rule["action"].(string)
			if action != "sniff" && action != "resolve" && action != "route-options" && action != "hijack-dns" {
				break
			}
			index++
		}
	}
	rule := map[string]interface{}{"inbound": entries, "action": "route", "outbound": SdwanGroupTag}
	updated := make([]interface{}, 0, len(rules)+1)
	updated = append(updated, rules[:index]...)
	updated = append(updated, rule)
	updated = append(updated, rules[index:]...)
	routeMap["rules"] = updated
	return json.Marshal(routeMap)
}
