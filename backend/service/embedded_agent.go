package service

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/util/common"
)

// On devices without systemd, such as OpenWrt, the panel runs the agent in
// its own process (SUI_AGENT_EMBEDDED=true): one Go runtime instead of two,
// which saves about 15 MB on a router. The connection comes from the same
// env file the standalone agent reads, so pairing works the same way.

// EmbeddedAgentEnabled reports whether this panel runs the agent itself.
func EmbeddedAgentEnabled() bool {
	enabled, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv("SUI_AGENT_EMBEDDED")))
	return enabled
}

var embeddedAgent = struct {
	sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}{}

// StartEmbeddedAgent (re)starts the agent from the env file. Without a saved
// connection there is nothing to start.
func StartEmbeddedAgent() error {
	StopEmbeddedAgent()
	content, err := os.ReadFile(envOrDefault("SUI_AGENT_ENV_FILE", defaultLocalAgentEnvFile))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	cfg, err := embeddedAgentConfig(parseAgentEnvironment(content))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	embeddedAgent.Lock()
	embeddedAgent.cancel, embeddedAgent.done = cancel, done
	embeddedAgent.Unlock()
	go func() {
		defer close(done)
		for ctx.Err() == nil {
			if err := agent.Run(ctx, cfg); err != nil && ctx.Err() == nil {
				logger.Warning("embedded agent: ", err)
			}
			select {
			case <-ctx.Done():
			case <-time.After(5 * time.Second):
			}
		}
	}()
	logger.Info("embedded agent connecting to ", cfg.PanelURL)
	return nil
}

// StopEmbeddedAgent disconnects the agent and waits for it to finish.
func StopEmbeddedAgent() {
	embeddedAgent.Lock()
	cancel, done := embeddedAgent.cancel, embeddedAgent.done
	embeddedAgent.cancel, embeddedAgent.done = nil, nil
	embeddedAgent.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}

// EmbeddedAgentRunning reports whether the embedded agent is started.
func EmbeddedAgentRunning() bool {
	embeddedAgent.Lock()
	defer embeddedAgent.Unlock()
	return embeddedAgent.cancel != nil
}

func embeddedAgentConfig(values map[string]string) (agent.ClientConfig, error) {
	panelURL, err := validatePanelURL(values["SUI_AGENT_PANEL"])
	if err != nil {
		return agent.ClientConfig{}, err
	}
	if !validAgentCredential(values["SUI_AGENT_TOKEN"]) {
		return agent.ClientConfig{}, common.NewError("the saved agent token is invalid; connect to the controller again")
	}
	interval := 15 * time.Second
	if parsed, err := time.ParseDuration(values["SUI_AGENT_INTERVAL"]); err == nil && parsed >= 5*time.Second && parsed <= 5*time.Minute {
		interval = parsed
	}
	insecure, _ := strconv.ParseBool(values["SUI_AGENT_INSECURE"])
	return agent.ClientConfig{
		PanelURL: panelURL, Token: values["SUI_AGENT_TOKEN"], Interval: interval, Insecure: insecure,
		LocalSocket: envOrDefault("SUI_CONTROL_SOCKET", defaultLocalControlSocket),
		PublicURL:   strings.TrimSpace(values["SUI_AGENT_PUBLIC_URL"]), PreferWS: true, Embedded: true,
		RestartCore: (&ConfigService{}).RestartCore,
	}, nil
}
