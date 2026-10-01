package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/service"
)

const maxLocalControlBody = 512 * 1024

func controlSocketPath() string {
	if value := os.Getenv("SUI_CONTROL_SOCKET"); value != "" {
		return value
	}
	if runtime.GOOS == "linux" {
		return "/run/s-ui/control.sock"
	}
	return filepath.Join(os.TempDir(), "1s-ui-control.sock")
}

func (s *Server) startControlSocket() error {
	if runtime.GOOS == "windows" {
		return nil
	}
	path := controlSocketPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return errors.New("local control path exists and is not a socket")
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	if err := os.Chmod(path, 0600); err != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		return err
	}

	local := &service.LocalControlService{}
	handler := http.NewServeMux()
	handler.HandleFunc("POST /v1/rpc", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxLocalControlBody)
		defer r.Body.Close()
		var request agent.RPCRequest
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&request); err != nil {
			writeLocalRPC(w, agent.RPCResponse{ID: request.ID, OK: false, Error: "invalid local RPC request", Code: 400})
			return
		}
		response := agent.RPCResponse{ID: request.ID, OK: true}
		var result interface{}
		var callErr error
		switch request.Method {
		case agent.RPCMethodCapabilities:
			result = local.Capabilities()
		case agent.RPCMethodPortTraffic:
			result, callErr = local.PortTrafficService.GetPortTraffic()
		case agent.RPCMethodPanelAccess:
			var payload service.ManagedPanelAccessRequest
			callErr = decodeLocalRPCPayload(request.Payload, &payload)
			if callErr == nil {
				result, callErr = local.ManagedAccessService.Issue(payload)
			}
		case agent.RPCMethodInboundList:
			result, callErr = local.ListInbounds()
		case agent.RPCMethodInboundEdit:
			var payload struct {
				ID uint `json:"id"`
			}
			callErr = decodeLocalRPCPayload(request.Payload, &payload)
			if callErr == nil {
				result, callErr = local.InboundEditor(payload.ID)
			}
		case agent.RPCMethodInboundSave:
			var payload service.RemoteInboundSaveRequest
			callErr = decodeLocalRPCPayload(request.Payload, &payload)
			if callErr == nil {
				result, callErr = local.SaveInbound(payload)
			}
		case agent.RPCMethodInboundQuickAdd:
			var payload service.RemoteQuickAddRequest
			callErr = decodeLocalRPCPayload(request.Payload, &payload)
			if callErr == nil {
				result, callErr = local.QuickAddInbounds(payload)
			}
		case agent.RPCMethodRelayGet:
			result, callErr = local.ConfigService.GetRelayData()
		case agent.RPCMethodRelayCreate:
			var payload service.RemoteRelayCreateRequest
			callErr = decodeLocalRPCPayload(request.Payload, &payload)
			if callErr == nil {
				result, callErr = local.CreateRemoteRelay(payload)
			}
		case agent.RPCMethodRelayDelete:
			var payload service.RemoteRelayDeleteRequest
			callErr = decodeLocalRPCPayload(request.Payload, &payload)
			if callErr == nil {
				callErr = local.DeleteRemoteRelay(payload)
				result = map[string]bool{"deleted": callErr == nil}
			}
		case agent.RPCMethodSdwanProvision:
			var payload service.SdwanProvisionRequest
			callErr = decodeLocalRPCPayload(request.Payload, &payload)
			if callErr == nil {
				result, callErr = local.ProvisionSdwanUplink(payload)
			}
		case agent.RPCMethodSdwanRemove:
			var payload service.SdwanRemoveRequest
			callErr = decodeLocalRPCPayload(request.Payload, &payload)
			if callErr == nil {
				result, callErr = local.RemoveSdwanUplink(payload)
			}
		case agent.RPCMethodSdwanDiagnose:
			result, callErr = local.DiagnoseSdwanUplink()
		case agent.RPCMethodSdwanTune:
			result = service.ApplySdwanTuning()
		case agent.RPCMethodProxyProbe:
			var payload service.ProxyProbeRequest
			callErr = decodeLocalRPCPayload(request.Payload, &payload)
			if callErr == nil {
				result, callErr = service.RunProxyProbes(payload)
			}
		case agent.RPCMethodSpeedtestStart:
			var payload service.SpeedtestStartRequest
			callErr = decodeLocalRPCPayload(request.Payload, &payload)
			if callErr == nil {
				result, callErr = service.StartLocalSpeedtest(payload.Port)
			}
		case agent.RPCMethodRelayExport:
			var payload service.RemoteRelayExportRequest
			callErr = decodeLocalRPCPayload(request.Payload, &payload)
			if callErr == nil {
				result, callErr = local.ExportRemoteRelay(payload)
			}
		default:
			callErr = errors.New("unsupported local RPC method")
		}
		if callErr != nil {
			response.OK = false
			response.Error = callErr.Error()
			response.Code = 500
			var conflict *service.ConfigRevisionConflictError
			if errors.As(callErr, &conflict) {
				response.Code = http.StatusConflict
			}
			writeLocalRPC(w, response)
			return
		}
		response.Payload, callErr = json.Marshal(result)
		if callErr != nil {
			response.OK = false
			response.Error = "failed to encode local RPC response"
			response.Code = 500
			response.Payload = nil
		}
		writeLocalRPC(w, response)
	})

	s.controlSocketPath = path
	s.controlListener = listener
	s.controlServer = &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      10 * time.Minute,
		IdleTimeout:       30 * time.Second,
	}
	go func() {
		if err := s.controlServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Warning("local control socket stopped: ", err)
		}
	}()
	logger.Info("local managed-node control socket ready at ", path)
	return nil
}

func (s *Server) stopControlSocket() error {
	if s.controlServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.controlServer.Shutdown(ctx)
	if s.controlListener != nil {
		_ = s.controlListener.Close()
	}
	if s.controlSocketPath != "" {
		_ = os.Remove(s.controlSocketPath)
	}
	return err
}

func decodeLocalRPCPayload(raw []byte, target interface{}) error {
	if len(raw) == 0 {
		return errors.New("missing local RPC payload")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeLocalRPC(w http.ResponseWriter, response agent.RPCResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
