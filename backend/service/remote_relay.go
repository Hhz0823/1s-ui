package service

import (
	"encoding/base64"

	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util/common"
)

type RemoteRelayCreateRequest struct {
	Request    RelayCreateRequest `json:"request"`
	Actor      string             `json:"actor"`
	PublicHost string             `json:"public_host"`
}

type RemoteRelayDeleteRequest struct {
	ID    uint   `json:"id"`
	Actor string `json:"actor"`
}

type RemoteRelayExportRequest struct {
	ID uint `json:"id"`
}

type RemoteRelayExportResponse struct {
	ContentBase64 string `json:"content_base64"`
}

func (s *LocalControlService) CreateRemoteRelay(request RemoteRelayCreateRequest) (*model.RelayPool, error) {
	actor, err := normalizeRemoteActor(request.Actor)
	if err != nil {
		return nil, err
	}
	publicHost, err := normalizeAgentPublicHost(request.PublicHost)
	if err != nil {
		return nil, err
	}
	if publicHost == "" {
		return nil, common.NewError("managed server public host is required before creating relay nodes")
	}
	request.Request.PublicHost = publicHost
	return s.ConfigService.CreateRelay(request.Request, "agent:"+actor, publicHost)
}

func (s *LocalControlService) DeleteRemoteRelay(request RemoteRelayDeleteRequest) error {
	if request.ID == 0 {
		return common.NewError("invalid relay pool id")
	}
	actor, err := normalizeRemoteActor(request.Actor)
	if err != nil {
		return err
	}
	return s.ConfigService.DeleteRelay(request.ID, "agent:"+actor)
}

func (s *LocalControlService) ExportRemoteRelay(request RemoteRelayExportRequest) (*RemoteRelayExportResponse, error) {
	if request.ID == 0 {
		return nil, common.NewError("invalid relay pool id")
	}
	data, err := s.ConfigService.GetRelayBitBrowserExport(request.ID)
	if err != nil {
		return nil, err
	}
	return &RemoteRelayExportResponse{ContentBase64: base64.StdEncoding.EncodeToString(data)}, nil
}
