//go:build !openwrt_lite

package core

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/Hhz0823/1s-ui/config"
	"github.com/Hhz0823/1s-ui/logger"
)

type XrayRuntime struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	cancel     context.CancelFunc
	done       chan error
	startedAt  time.Time
	xrayPath   string
	configPath string
	lastError  string
}

func NewXrayRuntime() *XrayRuntime {
	return &XrayRuntime{
		xrayPath:   config.GetXrayPath(),
		configPath: config.GetXrayConfigPath(),
	}
}

func (r *XrayRuntime) Start(rawConfig []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isRunningLocked() {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(r.configPath), 0750); err != nil {
		r.lastError = err.Error()
		return err
	}
	if err := os.WriteFile(r.configPath, rawConfig, 0600); err != nil {
		r.lastError = err.Error()
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, r.xrayPath, "run", "-config", r.configPath)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		r.lastError = err.Error()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		r.lastError = err.Error()
		return err
	}

	if err := cmd.Start(); err != nil {
		cancel()
		r.lastError = err.Error()
		return err
	}

	r.cmd = cmd
	r.cancel = cancel
	r.done = make(chan error, 1)
	r.startedAt = time.Now()
	r.lastError = ""

	go r.logPipe("xray", stdout)
	go r.logPipe("xray", stderr)
	go func() {
		err := cmd.Wait()
		r.mu.Lock()
		if err != nil {
			r.lastError = err.Error()
			logger.Warning("xray stopped: ", err)
		} else {
			logger.Info("xray stopped")
		}
		r.cmd = nil
		r.cancel = nil
		r.done <- err
		r.mu.Unlock()
	}()

	logger.Info("xray started")
	return nil
}

func (r *XrayRuntime) Stop() error {
	r.mu.Lock()
	if !r.isRunningLocked() {
		r.mu.Unlock()
		return nil
	}
	cancel := r.cancel
	cmd := r.cmd
	done := r.done
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	select {
	case <-done:
		return nil
	case <-time.After(2 * time.Second):
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return nil
	}
}

func (r *XrayRuntime) Restart(rawConfig []byte) error {
	if err := r.Stop(); err != nil {
		logger.Warning("stop xray during restart: ", err)
	}
	return r.Start(rawConfig)
}

func (r *XrayRuntime) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.isRunningLocked()
}

func (r *XrayRuntime) Status() map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()

	uptime := uint32(0)
	if r.isRunningLocked() {
		uptime = uint32(time.Since(r.startedAt).Seconds())
	}
	return map[string]interface{}{
		"running":     r.isRunningLocked(),
		"path":        r.xrayPath,
		"config_path": r.configPath,
		"last_error":  r.lastError,
		"stats": map[string]interface{}{
			"Uptime": uptime,
		},
	}
}

func (r *XrayRuntime) isRunningLocked() bool {
	return r.cmd != nil && r.cmd.Process != nil
}

func (r *XrayRuntime) logPipe(prefix string, pipe interface{ Read([]byte) (int, error) }) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		logger.Info(prefix, ": ", scanner.Text())
	}
}
