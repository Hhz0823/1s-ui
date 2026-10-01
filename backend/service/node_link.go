package service

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/util/common"
)

// nodeLinkRefresh is how often a node monitor picked from a server's
// inbounds re-reads its share link, so port or user changes follow along.
const nodeLinkRefresh = 10 * time.Minute

// NodeLinkRequest asks a panel for a working share link of one inbound.
type NodeLinkRequest struct {
	InboundId uint `json:"inbound_id"`
}

// NodeLinkResponse is the first enabled user's link of an inbound. It holds
// that user's credentials and only travels between the panel and the
// controller over the agent channel.
type NodeLinkResponse struct {
	Link string `json:"link"`
	Tag  string `json:"tag"`
	NodeLinkInfo
}

// LocalNodeLink finds a share link of one of this panel's inbounds.
func LocalNodeLink(inboundID uint) (*NodeLinkResponse, error) {
	db := database.GetDB()
	var inbound model.Inbound
	if err := db.Where("id = ?", inboundID).First(&inbound).Error; err != nil {
		return nil, common.NewError("inbound not found")
	}
	var clients []model.Client
	if err := db.Where("enable = ?", true).Order("id ASC").Find(&clients).Error; err != nil {
		return nil, err
	}
	for _, client := range clients {
		var inboundIDs []uint
		if json.Unmarshal(client.Inbounds, &inboundIDs) != nil || !containsUint(inboundIDs, inboundID) {
			continue
		}
		var links []map[string]string
		if json.Unmarshal(client.Links, &links) != nil {
			continue
		}
		for _, link := range links {
			if link["type"] != "local" || link["remark"] != inbound.Tag {
				continue
			}
			if _, info, err := ParseNodeLink(link["uri"]); err == nil {
				return &NodeLinkResponse{Link: strings.TrimSpace(link["uri"]), Tag: inbound.Tag, NodeLinkInfo: info}, nil
			}
		}
	}
	return nil, common.NewError("no enabled user of this inbound has a share link")
}

func containsUint(values []uint, value uint) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

// resolveNodeLink reads an inbound's share link on this panel (server 0) or
// on a managed server.
func (s *ProxyMonitorService) resolveNodeLink(serverID, inboundID uint) (*NodeLinkResponse, error) {
	if serverID == 0 {
		return LocalNodeLink(inboundID)
	}
	view, err := s.AgentService.Get(serverID)
	if err != nil {
		return nil, common.NewError("the server of this node does not exist")
	}
	if view.Online && !hasCapability(view.Report.Panel.Capabilities, agent.CapabilityNodeLinkV1) {
		return nil, common.NewError("the panel on the node's server is too old to share node links; update it")
	}
	response, err := s.AgentService.DispatchRPC(serverID, agent.RPCMethodNodeLink, NodeLinkRequest{InboundId: inboundID}, "proxy-monitor")
	if err != nil {
		return nil, err
	}
	var result NodeLinkResponse
	if err := json.Unmarshal(response.Payload, &result); err != nil || result.Link == "" {
		return nil, common.NewError("the server returned an invalid node link")
	}
	// Trust the link, not the server's description of it.
	if _, info, err := ParseNodeLink(result.Link); err != nil {
		return nil, err
	} else {
		result.NodeLinkInfo = info
	}
	return &result, nil
}

func hasCapability(capabilities []string, want string) bool {
	for _, capability := range capabilities {
		if capability == want {
			return true
		}
	}
	return false
}

// RefreshNodeLinks re-reads the links of node monitors picked from a
// server's inbounds. A server that cannot be asked keeps the stored link.
func (s *ProxyMonitorService) RefreshNodeLinks(now time.Time) {
	var monitors []model.ProxyMonitor
	cutoff := now.Add(-nodeLinkRefresh).Unix()
	if err := database.GetDB().Where("node_inbound_id <> 0 AND link_updated_at < ?", cutoff).Find(&monitors).Error; err != nil {
		logger.Warning("load node monitors: ", err)
		return
	}
	for _, monitor := range monitors {
		resolved, err := s.resolveNodeLink(monitor.NodeServerId, monitor.NodeInboundId)
		if err != nil {
			logger.Debug("refresh node link of monitor ", monitor.Id, ": ", err)
			continue
		}
		updates := map[string]interface{}{
			"link": resolved.Link, "protocol": resolved.Protocol, "host": resolved.Host,
			"port": resolved.Port, "link_updated_at": now.Unix(),
		}
		if err := database.GetDB().Model(&model.ProxyMonitor{}).Where("id = ?", monitor.Id).Updates(updates).Error; err != nil {
			logger.Warning("store node link: ", err)
		}
	}
}
