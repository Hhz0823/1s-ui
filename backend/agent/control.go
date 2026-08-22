package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/Hhz0823/1s-ui/config"
	"github.com/shirou/gopsutil/v4/process"
)

const maxExecOutput = 256 * 1024

// HandleCommand executes a panel control command on the agent host.
func HandleCommand(ctx context.Context, cmd Command) CommandResult {
	start := time.Now()
	result := CommandResult{ID: cmd.ID, Type: cmd.Type, OK: true}
	var err error
	var output string
	var code int

	switch cmd.Type {
	case CmdPing:
		output = "pong"
	case CmdReportNow:
		output = "report scheduled"
	case CmdSetInterval:
		// Interval is applied by the WS loop; acknowledge only.
		output = "interval updated"
	case CmdRestartAgent:
		output = "agent restarting"
		// Exit after response is written by caller; signal via special code.
		code = 77
	case CmdRestartXray:
		output, code, err = restartProcess(ctx, []string{"xray", "xray.exe"}, config.GetXrayPath())
	case CmdRestartSingBox:
		output, code, err = restartProcess(ctx, []string{"sing-box", "sing-box.exe", "sui", "s-ui"}, "")
	case CmdExec:
		command, _ := cmd.Args["command"].(string)
		output, code, err = runShell(ctx, command)
	default:
		err = fmt.Errorf("unknown command: %s", cmd.Type)
	}

	result.Elapsed = time.Since(start).Milliseconds()
	result.Output = output
	result.Code = code
	if err != nil {
		result.OK = false
		result.Error = err.Error()
	}
	return result
}

func runShell(ctx context.Context, command string) (string, int, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", 1, fmt.Errorf("empty command")
	}
	if len(command) > 4000 {
		return "", 1, fmt.Errorf("command too long")
	}
	timeout := 30 * time.Second
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(execCtx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(execCtx, "sh", "-c", command)
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := buf.String()
	if len(out) > maxExecOutput {
		out = out[:maxExecOutput] + "\n...[truncated]"
	}
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			code = 1
		}
		if execCtx.Err() == context.DeadlineExceeded {
			return out, code, fmt.Errorf("command timed out after %s", timeout)
		}
		// Still return stdout/stderr with non-zero code.
		return out, code, nil
	}
	return out, 0, nil
}

func restartProcess(ctx context.Context, names []string, binaryPath string) (string, int, error) {
	// Prefer systemd unit names when available.
	for _, unit := range []string{"xray", "sing-box", "s-ui", "sui"} {
		for _, n := range names {
			if strings.Contains(strings.ToLower(n), strings.TrimSuffix(unit, ".service")) {
				if out, err := trySystemctl(ctx, unit); err == nil {
					return out, 0, nil
				}
			}
		}
	}
	if binaryPath != "" {
		base := strings.ToLower(filepathBase(binaryPath))
		if strings.Contains(base, "xray") {
			if out, err := trySystemctl(ctx, "xray"); err == nil {
				return out, 0, nil
			}
		}
	}

	killed := 0
	processes, _ := process.Processes()
	nameSet := map[string]bool{}
	for _, n := range names {
		nameSet[strings.ToLower(n)] = true
	}
	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}
		if !nameSet[strings.ToLower(name)] {
			continue
		}
		// Do not kill ourselves.
		if int32(os.Getpid()) == p.Pid {
			continue
		}
		var sigErr error
		if runtime.GOOS == "windows" {
			sigErr = p.Kill()
		} else {
			sigErr = p.SendSignal(syscall.SIGTERM)
		}
		if sigErr == nil {
			killed++
		}
	}
	if killed == 0 {
		return "", 1, fmt.Errorf("no matching process found to restart")
	}
	return fmt.Sprintf("sent SIGTERM to %d process(es); supervisor should restart them", killed), 0, nil
}

func trySystemctl(ctx context.Context, unit string) (string, error) {
	execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(execCtx, "systemctl", "restart", unit)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return strings.TrimSpace(string(out)) + "\nrestarted unit " + unit, nil
}

func filepathBase(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}
