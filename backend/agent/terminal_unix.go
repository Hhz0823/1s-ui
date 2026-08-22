//go:build !windows

package agent

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

type localTerminal struct {
	id   string
	ptmx *os.File
	cmd  *exec.Cmd
	once sync.Once
}

func startLocalTerminal(id string, cols, rows uint16) (*localTerminal, error) {
	if cols < 20 {
		cols = 80
	}
	if rows < 5 {
		rows = 24
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
		if _, err := os.Stat(shell); err != nil {
			shell = "/bin/sh"
		}
	}
	cmd := exec.Command(shell, "-l")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: rows, Cols: cols})
	return &localTerminal{id: id, ptmx: ptmx, cmd: cmd}, nil
}

func (t *localTerminal) Write(data []byte) (int, error) {
	if t == nil || t.ptmx == nil {
		return 0, fmt.Errorf("terminal closed")
	}
	return t.ptmx.Write(data)
}

func (t *localTerminal) Resize(cols, rows uint16) error {
	if t == nil || t.ptmx == nil {
		return fmt.Errorf("terminal closed")
	}
	if cols < 20 {
		cols = 80
	}
	if rows < 5 {
		rows = 24
	}
	return pty.Setsize(t.ptmx, &pty.Winsize{Rows: rows, Cols: cols})
}

func (t *localTerminal) Close() {
	t.once.Do(func() {
		if t.ptmx != nil {
			_ = t.ptmx.Close()
		}
		if t.cmd != nil && t.cmd.Process != nil {
			_ = t.cmd.Process.Kill()
			_, _ = t.cmd.Process.Wait()
		}
	})
}

// pumpTerminalOutput reads PTY stdout and sends base64 chunks via write fn.
func pumpTerminalOutput(ctx context.Context, t *localTerminal, write func(interface{}) error) {
	buf := make([]byte, 4096)
	for {
		if ctx.Err() != nil {
			t.Close()
			return
		}
		n, err := t.ptmx.Read(buf)
		if n > 0 {
			_ = write(map[string]interface{}{
				"type": MsgTypeTerminalOutput,
				"id":   t.id,
				"data": base64.StdEncoding.EncodeToString(buf[:n]),
			})
		}
		if err != nil {
			msg := map[string]interface{}{"type": MsgTypeTerminalClosed, "id": t.id}
			if err != io.EOF {
				msg["error"] = err.Error()
			}
			_ = write(msg)
			t.Close()
			return
		}
	}
}
