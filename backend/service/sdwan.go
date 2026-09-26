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
// a private uplink, the controller keeps them in a sing-box urltest group and
// routes the selected entry inbounds through whichever server currently has
// the lowest latency, failing over automatically when a server goes down.
const (
	sdwanConfigKey        = "sdwanConfig"
	SdwanGroupTag         = "sdwan-auto"
	sdwanMemberTagPrefix  = "sdwan-node-"
	defaultSdwanTestURL   = "https://www.gstatic.com/generate_204"
	defaultSdwanInterval  = 60
	defaultSdwanTolerance = 50
	maxSdwanEntryInbounds = 256
)

type SdwanSettings struct {
	// Enabled routes the entry inbounds through the SD-WAN group. Members and
	// the group exist whenever members are configured, so latency can be
	// checked (and custom rules can use sdwan-auto) before switching traffic.
	Enabled       bool     `json:"enabled"`
	Protocol      string   `json:"protocol"`
	EntryInbounds []string `json:"entry_inbounds"`
	TestURL       string   `json:"test_url"`
	Interval      int      `json:"interval"`
	Tolerance     int      `json:"tolerance"`
	IncludeDirect bool     `json:"include_direct"`
	RulesFirst    bool     `json:"rules_first"`
}

type SdwanMemberView struct {
	NodeID      uint             `json:"node_id"`
	Name        string           `json:"name"`
	Tag         string           `json:"tag"`
	Protocol    string           `json:"protocol"`
	Server      string           `json:"server"`
	Port        int              `json:"port"`
	UpdatedAt   int64            `json:"updated_at"`
	Online      bool             `json:"online"`
	Managed     bool             `json:"managed"`
	Latency     AgentLatencyView `json:"latency"`
	CPUPercent  float64          `json:"cpu_percent"`
	NetSentRate uint64           `json:"net_sent_rate"`
	NetRecvRate uint64           `json:"net_recv_rate"`
	Delay       *uint16          `json:"delay,omitempty"`
	DelayAt     int64            `json:"delay_at,omitempty"`
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
	GroupTag     string            `json:"group_tag"`
	Active       bool              `json:"active"`
	Selected     string            `json:"selected,omitempty"`
	SelectedName string            `json:"selected_name,omitempty"`
	CoreRunning  bool              `json:"core_running"`
	CanControl   bool              `json:"can_control"`
	Warnings     []string          `json:"warnings,omitempty"`
}

type SdwanService struct {
	agents AgentService
	config ConfigService
	// restartCore applies a new SD-WAN topology; replaceable in tests.
	restartCore func() error
}

var sdwanMu sync.Mutex

func sdwanMemberTag(nodeID uint) string {
	return fmt.Sprintf("%s%d", sdwanMemberTagPrefix, nodeID)
}

func defaultSdwanSettings() SdwanSettings {
	return SdwanSettings{
		Protocol:      SdwanProtocolShadowsocks,
		EntryInbounds: []string{},
		TestURL:       defaultSdwanTestURL,
		Interval:      defaultSdwanInterval,
		Tolerance:     defaultSdwanTolerance,
	}
}

func normalizeSdwanSettings(settings *SdwanSettings) error {
	protocol, err := normalizeSdwanProtocol(settings.Protocol)
	if err != nil {
		return err
	}
	settings.Protocol = protocol
	settings.TestURL = strings.TrimSpace(settings.TestURL)
	if settings.TestURL == "" {
		settings.TestURL = defaultSdwanTestURL
	}
	parsed, err := url.Parse(settings.TestURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return common.NewError("SD-WAN test URL must be an http(s) URL")
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
	return node != nil && node.Managed && slices.Contains(node.Report.Panel.Capabilities, agent.CapabilitySdwanV1)
}

// SdwanState reports configuration, members, runtime selection and candidates.
func (s *SdwanService) SdwanState() (*SdwanState, error) {
	db := database.GetDB()
	state := &SdwanState{GroupTag: SdwanGroupTag, Members: []SdwanMemberView{}, Nodes: []SdwanNodeOption{}, Inbounds: []string{}}
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
		Where("tag <> ?", sdwanUplinkTag).
		Order("id").Pluck("tag", &inboundTags).Error; err != nil {
		return nil, err
	}
	state.Inbounds = append(state.Inbounds, inboundTags...)

	nodes, err := s.agents.List()
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
		view := SdwanMemberView{
			NodeID: member.NodeId, Name: node.Name, Tag: sdwanMemberTag(member.NodeId),
			Protocol: member.Protocol, Server: member.Server, Port: member.Port, UpdatedAt: member.UpdatedAt,
			Online: node.Online, Managed: node.Managed, Latency: node.Latency,
			CPUPercent: node.Report.CPUPercent, NetSentRate: node.Report.NetRate.Sent, NetRecvRate: node.Report.NetRate.Recv,
			NeedsResync: member.Protocol != settings.Protocol,
		}
		if view.Name == "" {
			view.Name = fmt.Sprintf("#%d", member.NodeId)
		}
		if groupStatus != nil {
			if delay, ok := groupStatus.Delays[view.Tag]; ok {
				value := delay.Delay
				view.Delay = &value
				view.DelayAt = delay.Time.Unix()
			}
			view.Selected = groupStatus.Now == view.Tag
			if view.Selected {
				state.SelectedName = view.Name
			}
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
// protocol changed, and restarts sing-box so routing follows the new topology.
func (s *SdwanService) SaveSdwanSettings(settings SdwanSettings, actor string) (*SdwanState, error) {
	if err := (&SettingService{}).RequireControllerControl(); err != nil {
		return nil, err
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
	if previous.Protocol != settings.Protocol {
		warnings = s.resyncLocked(settings, actor)
	}
	applyErr := s.apply()
	sdwanMu.Unlock()
	if applyErr != nil {
		return nil, common.NewError("SD-WAN settings saved but sing-box failed to restart: ", applyErr.Error())
	}
	return s.stateWithWarnings(warnings)
}

// AddSdwanMember provisions the SD-WAN uplink on a managed server and adds it
// to the group. Calling it again refreshes the member (e.g. after an IP change).
func (s *SdwanService) AddSdwanMember(nodeID uint, actor string) (*SdwanState, error) {
	if err := (&SettingService{}).RequireControllerControl(); err != nil {
		return nil, err
	}
	sdwanMu.Lock()
	settings, _ := loadSdwanSettings(database.GetDB())
	err := s.provisionLocked(nodeID, settings, actor)
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

// RemoveSdwanMember removes a member and deletes its uplink on the managed
// server when that server is reachable.
func (s *SdwanService) RemoveSdwanMember(nodeID uint, actor string) (*SdwanState, error) {
	if err := (&SettingService{}).RequireControllerControl(); err != nil {
		return nil, err
	}
	sdwanMu.Lock()
	db := database.GetDB()
	result := db.Where("node_id = ?", nodeID).Delete(&model.SdwanMember{})
	if result.Error != nil {
		sdwanMu.Unlock()
		return nil, result.Error
	}
	var warnings []string
	if node, err := s.agents.Get(nodeID); err == nil && sdwanNodeSupported(node) {
		request := SdwanRemoveRequest{Actor: actor, PublicHost: ManagedNodePublicHost(node)}
		if _, rpcErr := s.agents.DispatchRPC(nodeID, agent.RPCMethodSdwanRemove, request, actor); rpcErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: uplink was not removed on the server: %v", node.Name, rpcErr))
		}
	} else {
		warnings = append(warnings, "the server is offline; remove the sdwan-uplink inbound on it manually if it still exists")
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

// TestSdwan re-measures every member through the running group right now and
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
		if err := s.provisionLocked(member.NodeId, settings, actor); err != nil {
			warnings = append(warnings, fmt.Sprintf("#%d: %v", member.NodeId, err))
		}
	}
	return warnings
}

func (s *SdwanService) provisionLocked(nodeID uint, settings SdwanSettings, actor string) error {
	node, err := s.agents.Get(nodeID)
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
	request := SdwanProvisionRequest{Protocol: settings.Protocol, Actor: actor, PublicHost: publicHost}
	response, err := s.agents.DispatchRPC(nodeID, agent.RPCMethodSdwanProvision, request, actor)
	if err != nil {
		return err
	}
	var provisioned SdwanProvisionResponse
	if err = json.Unmarshal(response.Payload, &provisioned); err != nil {
		return err
	}
	outbound, err := sdwanMemberOutbound(nodeID, provisioned.Outbound)
	if err != nil {
		return common.NewErrorf("%s returned an unusable uplink: %v", node.Name, err)
	}
	now := time.Now().Unix()
	member := model.SdwanMember{
		NodeId: nodeID, Protocol: provisioned.Protocol, Server: provisioned.Server, Port: provisioned.Port,
		Outbound: outbound, CreatedAt: now, UpdatedAt: now,
	}
	db := database.GetDB()
	var existing model.SdwanMember
	err = db.Where("node_id = ?", nodeID).First(&existing).Error
	if database.IsNotFound(err) {
		return db.Create(&member).Error
	}
	if err != nil {
		return err
	}
	return db.Model(&existing).Updates(map[string]interface{}{
		"protocol": member.Protocol, "server": member.Server, "port": member.Port,
		"outbound": member.Outbound, "updated_at": now,
	}).Error
}

// sdwanMemberOutbound tags and validates an uplink outbound so a broken member
// can never stop sing-box from starting.
func sdwanMemberOutbound(nodeID uint, raw json.RawMessage) (json.RawMessage, error) {
	var outbound map[string]interface{}
	if err := json.Unmarshal(raw, &outbound); err != nil {
		return nil, err
	}
	switch outbound["type"] {
	case SdwanProtocolShadowsocks, SdwanProtocolHysteria2:
	default:
		return nil, common.NewErrorf("unexpected outbound type %v", outbound["type"])
	}
	for _, key := range []string{"detour", "bind_interface", "inet4_bind_address", "inet6_bind_address", "routing_mark", "netns"} {
		delete(outbound, key)
	}
	outbound["tag"] = sdwanMemberTag(nodeID)
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

// applySdwanConfig adds member outbounds, the urltest group and, when routing
// is enabled, the entry rule to a sing-box configuration being built.
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
	var memberTags []string
	var memberOutbounds []json.RawMessage
	for _, member := range members {
		tag := sdwanMemberTag(member.NodeId)
		if _, taken := existing[tag]; taken {
			logger.Warning("SD-WAN member skipped: outbound tag ", tag, " is already used")
			continue
		}
		outbound, err := sdwanMemberOutbound(member.NodeId, member.Outbound)
		if err != nil {
			logger.Warning("SD-WAN member ", tag, " skipped: ", err)
			continue
		}
		memberTags = append(memberTags, tag)
		memberOutbounds = append(memberOutbounds, outbound)
	}
	if len(memberTags) == 0 {
		return
	}
	groupMembers := append([]string(nil), memberTags...)
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
	singboxConfig.Outbounds = append(singboxConfig.Outbounds, memberOutbounds...)
	singboxConfig.Outbounds = append(singboxConfig.Outbounds, group)

	if !settings.Enabled {
		return
	}
	inbounds := configTags(singboxConfig.Inbounds)
	var entries []string
	for _, tag := range settings.EntryInbounds {
		if _, ok := inbounds[tag]; ok && tag != sdwanUplinkTag {
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
