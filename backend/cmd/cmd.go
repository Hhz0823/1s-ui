package cmd

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	monitoragent "github.com/Hhz0823/1s-ui/agent"
	"github.com/Hhz0823/1s-ui/cmd/migration"
	"github.com/Hhz0823/1s-ui/config"
)

func ParseCmd() {
	var showVersion bool
	flag.BoolVar(&showVersion, "v", false, "show version")

	adminCmd := flag.NewFlagSet("admin", flag.ExitOnError)
	settingCmd := flag.NewFlagSet("setting", flag.ExitOnError)
	agentCmd := flag.NewFlagSet("agent", flag.ExitOnError)

	var username string
	var password string
	var port int
	var path string
	var subPort int
	var subPath string
	var listen string
	var domain string
	var webURI string
	var reset bool
	var show bool
	var agentPanel string
	var agentToken string
	var agentLocalSocket string
	var agentInterval time.Duration
	var agentInsecure bool
	var agentOnce bool
	settingCmd.BoolVar(&reset, "reset", false, "reset all settings")
	settingCmd.BoolVar(&show, "show", false, "show current settings")
	settingCmd.IntVar(&port, "port", 0, "set panel port")
	settingCmd.StringVar(&path, "path", "", "set panel path")
	settingCmd.IntVar(&subPort, "subPort", 0, "set sub port")
	settingCmd.StringVar(&subPath, "subPath", "", "set sub path")
	settingCmd.StringVar(&listen, "listen", "", "set panel listen IP (127.0.0.1 for reverse proxy; - to clear)")
	settingCmd.StringVar(&domain, "domain", "", "set panel domain (- to clear)")
	settingCmd.StringVar(&webURI, "uri", "", "set public panel URI (- to clear)")

	adminCmd.BoolVar(&show, "show", false, "show first admin credentials")
	adminCmd.BoolVar(&reset, "reset", false, "reset first admin credentials")
	adminCmd.StringVar(&username, "username", "", "set login username")
	adminCmd.StringVar(&password, "password", "", "set login password")
	agentCmd.StringVar(&agentPanel, "panel", os.Getenv("SUI_AGENT_PANEL"), "panel base URL, including its path")
	agentCmd.StringVar(&agentToken, "token", os.Getenv("SUI_AGENT_TOKEN"), "agent enrollment token")
	agentCmd.StringVar(&agentLocalSocket, "local-socket", monitoragent.DefaultLocalControlSocket(), "local 1S-UI control socket")
	agentCmd.DurationVar(&agentInterval, "interval", 15*time.Second, "heartbeat interval (5s-5m)")
	agentCmd.BoolVar(&agentInsecure, "insecure", false, "skip panel TLS certificate verification")
	agentCmd.BoolVar(&agentOnce, "once", false, "send one heartbeat and exit")

	oldUsage := flag.Usage
	flag.Usage = func() {
		oldUsage()
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("    admin          set/reset/show first admin credentials")
		fmt.Println("    agent          connect this host to a 1S-UI panel")
		fmt.Println("    uri            Show panel URI")
		fmt.Println("    migrate        migrate form older version")
		fmt.Println("    setting        set/reset/show settings")
		fmt.Println()
		adminCmd.Usage()
		fmt.Println()
		settingCmd.Usage()
		fmt.Println()
		agentCmd.Usage()
	}

	flag.Parse()
	if showVersion {
		fmt.Println("S-UI Panel\t", config.GetVersion())
		info, ok := debug.ReadBuildInfo()
		if ok {
			for _, dep := range info.Deps {
				if dep.Path == "github.com/sagernet/sing-box" {
					fmt.Println("Sing-Box\t", dep.Version)
					break
				}
			}
		}
		return
	}

	switch os.Args[1] {
	case "admin":
		err := adminCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		switch {
		case show:
			showAdmin()
		case reset:
			resetAdmin()
		default:
			updateAdmin(username, password)
			showAdmin()
		}

	case "uri":
		getPanelURI()

	case "migrate":
		migration.MigrateDb()

	case "setting":
		err := settingCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		switch {
		case show:
			showSetting()
		case reset:
			resetSetting()
		default:
			updateSetting(port, path, subPort, subPath, listen, domain, webURI)
			showSetting()
		}
	case "agent":
		if err := agentCmd.Parse(os.Args[2:]); err != nil {
			fmt.Println(err)
			return
		}
		runAgent(agentPanel, agentToken, agentLocalSocket, agentInterval, agentInsecure, agentOnce)
	default:
		fmt.Println("Invalid subcommands")
		flag.Usage()
	}
}
