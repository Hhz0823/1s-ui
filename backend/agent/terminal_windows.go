//go:build windows

package agent

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// Windows uses piped cmd.exe (no full PTY). Good enough for basic interactive use.
type localTerminal struct {
	id     string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	once   sync.Once
}

func startLocalTerminal(id string, cols, rows uint16) (*localTerminal, error) {
	cmd := exec.Command("cmd.exe")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	_ = cols
	_ = rows
	return &localTerminal{id: id, cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

func (t *localTerminal) Write(data []byte) (int, error) {
	if t == nil || t.stdin == nil {
		return 0, fmt.Errorf("terminal closed")
	}
	return t.stdin.Write(data)
}

func (t *localTerminal) Resize(cols, rows uint16) error {
	return nil
}

func (t *localTerminal) Close() {
	t.once.Do(func() {
		if t.stdin != nil {
			_ = t.stdin.Close()
		}
		if t.stdout != nil {
			_ = t.stdout.Close()
		}
		if t.cmd != nil && t.cmd.Process != nil {
			_ = t.cmd.Process.Kill()
			_, _ = t.cmd.Process.Wait()
		}
	})
}

func pumpTerminalOutput(ctx context.Context, t *localTerminal, write func(interface{}) error) {
	buf := make([]byte, 4096)
	for {
		if ctx.Err() != nil {
			return
		}
		n, err := t.stdout.Read(buf)
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
