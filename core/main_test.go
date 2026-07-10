package core

import (
	"testing"

	"github.com/Hhz0823/1s-ui/logger"

	"github.com/op/go-logging"
)

func TestCoreStartRejectsInvalidConfig(t *testing.T) {
	logger.InitLogger(logging.CRITICAL)
	core := NewCore()
	if err := core.Start([]byte(`{"inbounds": [`)); err == nil {
		t.Fatal("Start() accepted invalid JSON")
	}
	if core.IsRunning() {
		t.Fatal("core is running after invalid configuration")
	}
}
