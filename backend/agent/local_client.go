package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const maxLocalRPCResponse = 2 * 1024 * 1024

var localPanelCache = struct {
	sync.Mutex
	path     string
	loadedAt time.Time
	status   PanelStatus
}{}

func DefaultLocalControlSocket() string {
	if value := os.Getenv("SUI_AGENT_LOCAL_SOCKET"); value != "" {
		return value
	}
	if value := os.Getenv("SUI_CONTROL_SOCKET"); value != "" {
		return value
	}
	if runtime.GOOS == "linux" {
		return "/run/s-ui/control.sock"
	}
	return filepath.Join(os.TempDir(), "1s-ui-control.sock")
}

func CallLocalRPC(ctx context.Context, socketPath string, request RPCRequest) RPCResponse {
	response := RPCResponse{ID: request.ID, OK: false, Code: http.StatusServiceUnavailable}
	if socketPath == "" {
		socketPath = DefaultLocalControlSocket()
	}
	payload, err := json.Marshal(request)
	if err != nil {
		response.Error = err.Error()
		return response
	}
	if len(payload) > 512*1024 {
		response.Code = http.StatusRequestEntityTooLarge
		response.Error = "local RPC request is too large"
		return response
	}
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: localRPCTimeout(request.Method)}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://local/v1/rpc", bytes.NewReader(payload))
	if err != nil {
		response.Error = err.Error()
		return response
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		response.Error = fmt.Sprintf("local 1S-UI control is unavailable: %v", err)
		return response
	}
	defer httpResponse.Body.Close()
	body, err := io.ReadAll(io.LimitReader(httpResponse.Body, maxLocalRPCResponse+1))
	if err != nil {
		response.Error = err.Error()
		return response
	}
	if len(body) > maxLocalRPCResponse {
		response.Code = http.StatusRequestEntityTooLarge
		response.Error = "local RPC response is too large"
		return response
	}
	if err := json.Unmarshal(body, &response); err != nil {
		response.Code = http.StatusBadGateway
		response.Error = "invalid response from local 1S-UI control"
	}
	return response
}

func localRPCTimeout(method string) time.Duration {
	switch method {
	case RPCMethodInboundQuickAdd, RPCMethodRelayCreate, RPCMethodRelayDelete:
		return 10 * time.Minute
	default:
		return 45 * time.Second
	}
}

func probeLocalPanel(socketPath string) PanelStatus {
	if socketPath == "" {
		socketPath = DefaultLocalControlSocket()
	}
	localPanelCache.Lock()
	defer localPanelCache.Unlock()
	if localPanelCache.path == socketPath && time.Since(localPanelCache.loadedAt) < 30*time.Second {
		return localPanelCache.status
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	response := CallLocalRPC(ctx, socketPath, RPCRequest{ID: "capability-probe", Method: RPCMethodCapabilities})
	status := PanelStatus{}
	if response.OK {
		_ = json.Unmarshal(response.Payload, &status)
	}
	localPanelCache.path = socketPath
	localPanelCache.loadedAt = time.Now()
	localPanelCache.status = status
	return status
}
