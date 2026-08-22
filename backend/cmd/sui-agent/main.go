package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	monitoragent "github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/config"
)

func main() {
	panel := flag.String("panel", os.Getenv("SUI_AGENT_PANEL"), "panel base URL, including its path")
	token := flag.String("token", os.Getenv("SUI_AGENT_TOKEN"), "agent enrollment token")
	interval := flag.Duration("interval", envDuration("SUI_AGENT_INTERVAL", 15*time.Second), "heartbeat interval (5s-5m)")
	insecure := flag.Bool("insecure", envBool("SUI_AGENT_INSECURE"), "skip panel TLS certificate verification")
	localSocket := flag.String("local-socket", monitoragent.DefaultLocalControlSocket(), "local 1S-UI control socket")
	once := flag.Bool("once", false, "send one heartbeat and exit")
	version := flag.Bool("version", false, "show version")
	flag.Parse()

	if *version {
		fmt.Println(config.GetVersion())
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg := monitoragent.ClientConfig{
		PanelURL:    *panel,
		Token:       *token,
		Interval:    *interval,
		Insecure:    *insecure,
		LocalSocket: *localSocket,
		PreferWS:    !*once,
	}
	var err error
	if *once {
		err = monitoragent.SendOnce(ctx, cfg)
	} else {
		err = monitoragent.Run(ctx, cfg)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "agent error:", err)
		os.Exit(1)
	}
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(name string) bool {
	value, _ := strconv.ParseBool(os.Getenv(name))
	return value
}
