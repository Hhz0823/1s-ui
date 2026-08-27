package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/config"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util/common"
)

type LocalControlService struct {
	ConfigService
	InboundService
	TlsService
	ClientService
	PortTrafficService
	ManagedAccessService
}

type RemoteInboundList struct {
	Revision uint64                   `json:"revision"`
	Inbounds []map[string]interface{} `json:"inbounds"`
	TLS      []model.Tls              `json:"tls"`
}

type RemoteInboundEditor struct {
	Revision uint64                   `json:"revision"`
	Inbound  map[string]interface{}   `json:"inbound,omitempty"`
	Tags     []string                 `json:"tags"`
	TLS      []model.Tls              `json:"tls"`
	Clients  []model.Client           `json:"clients"`
	Inbounds []map[string]interface{} `json:"inbounds"`
}

type RemoteInboundSaveRequest struct {
	Action           string          `json:"action"`
	Data             json.RawMessage `json:"data"`
	InitUsers        []uint          `json:"init_users,omitempty"`
	ExpectedRevision uint64          `json:"expected_revision"`
	Actor            string          `json:"actor"`
	PublicHost       string          `json:"public_host"`
}

type RemoteInboundSaveResponse struct {
	Revision uint64 `json:"revision"`
}

func (s *LocalControlService) Capabilities() agent.PanelStatus {
	cores := &agent.CoreStatus{}
	if corePtr != nil {
		cores.SingBoxRunning = corePtr.IsRunning()
	}
	if xrayPtr != nil {
		cores.XrayRunning = xrayPtr.IsRunning()
	}
	return agent.PanelStatus{
		Installed:        true,
		Version:          config.GetVersion(),
		ControlAvailable: true,
		ProtocolVersion:  agent.ProtocolVersion,
		Cores:            cores,
		Capabilities: []string{
			agent.CapabilityMetricsV1,
			agent.CapabilityLatencyV1,
			agent.CapabilityInboundReadV1,
			agent.CapabilityInboundWriteV1,
			agent.CapabilityQuickAddV1,
			agent.CapabilityQuickAddNaiveV1,
			agent.CapabilityRelayV1,
			agent.CapabilityPortTrafficV1,
			agent.CapabilityPanelAccessV1,
		},
	}
}

func (s *LocalControlService) ListInbounds() (*RemoteInboundList, error) {
	revision, err := s.ConfigService.CurrentRevision()
	if err != nil {
		return nil, err
	}
	inbounds, err := s.InboundService.Get("")
	if err != nil {
		return nil, err
	}
	items := []map[string]interface{}{}
	if inbounds != nil {
		items = *inbounds
	}
	tlsConfigs, err := s.TlsService.GetAll()
	if err != nil {
		return nil, err
	}
	if tlsConfigs == nil {
		tlsConfigs = []model.Tls{}
	}
	return &RemoteInboundList{Revision: revision, Inbounds: items, TLS: tlsConfigs}, nil
}

func (s *LocalControlService) InboundEditor(id uint) (*RemoteInboundEditor, error) {
	revision, err := s.ConfigService.CurrentRevision()
	if err != nil {
		return nil, err
	}
	inbounds, err := s.InboundService.Get("")
	if err != nil {
		return nil, err
	}
	items := []map[string]interface{}{}
	if inbounds != nil {
		items = *inbounds
	}
	result := &RemoteInboundEditor{Revision: revision, Inbounds: items, Tags: []string{}}
	for i := range items {
		if tag, ok := items[i]["tag"].(string); ok {
			result.Tags = append(result.Tags, tag)
		}
		if id > 0 && inboundMapID(items[i]) == id {
			result.Inbound = items[i]
		}
	}
	if id > 0 && result.Inbound == nil {
		return nil, common.NewError("inbound not found")
	}
	result.TLS, err = s.TlsService.GetAll()
	if err != nil {
		return nil, err
	}
	clients, err := s.ClientService.GetAll()
	if err != nil {
		return nil, err
	}
	if clients != nil {
		result.Clients = *clients
	}
	if result.TLS == nil {
		result.TLS = []model.Tls{}
	}
	if result.Clients == nil {
		result.Clients = []model.Client{}
	}
	return result, nil
}

func inboundMapID(item map[string]interface{}) uint {
	switch value := item["id"].(type) {
	case uint:
		return value
	case uint64:
		return uint(value)
	case int:
		return uint(value)
	case int64:
		return uint(value)
	case float64:
		return uint(value)
	default:
		return 0
	}
}

func (s *LocalControlService) SaveInbound(request RemoteInboundSaveRequest) (*RemoteInboundSaveResponse, error) {
	if request.Action != "new" && request.Action != "edit" && request.Action != "del" {
		return nil, common.NewError("unsupported inbound action")
	}
	if len(request.Data) == 0 || len(request.Data) > 512*1024 {
		return nil, common.NewError("invalid inbound payload")
	}
	actor, err := normalizeRemoteActor(request.Actor)
	if err != nil {
		return nil, err
	}
	publicHost, err := normalizeAgentPublicHost(request.PublicHost)
	if err != nil {
		return nil, err
	}
	if request.Action != "del" && publicHost == "" {
		return nil, common.NewError("managed server public host is required before saving an inbound")
	}
	initUsers := make([]string, 0, len(request.InitUsers))
	for _, id := range request.InitUsers {
		if id > 0 {
			initUsers = append(initUsers, strconv.FormatUint(uint64(id), 10))
		}
	}
	_, revision, err := s.ConfigService.SaveWithRevision(
		request.ExpectedRevision,
		"inbounds",
		request.Action,
		request.Data,
		strings.Join(initUsers, ","),
		fmt.Sprintf("agent:%s", actor),
		publicHost,
	)
	if err != nil {
		return nil, err
	}
	return &RemoteInboundSaveResponse{Revision: revision}, nil
}
