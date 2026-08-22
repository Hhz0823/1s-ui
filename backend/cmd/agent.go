package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	monitoragent "github.com/Hhz0823/1s-ui/agent"
)

func runAgent(panelURL, token, localSocket string, interval time.Duration, insecure, once bool) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Prefer WebSocket long connection (Nezha/Komari style); HTTP heartbeat is the fallback.
	config := monitoragent.ClientConfig{PanelURL: panelURL, Token: token, LocalSocket: localSocket, Interval: interval, Insecure: insecure, PreferWS: !once}
	var err error
	if once {
		err = monitoragent.SendOnce(ctx, config)
	} else {
		err = monitoragent.Run(ctx, config)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "agent error:", err)
		os.Exit(1)
	}
}
