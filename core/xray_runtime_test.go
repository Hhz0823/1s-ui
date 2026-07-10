//go:build !openwrt_lite

package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestXrayValidate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX shell helper")
	}
	dir := t.TempDir()
	xrayPath := filepath.Join(dir, "xray")
	script := `#!/bin/sh
config=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-config" ]; then
    shift
    config="$1"
  fi
  shift
done
if grep -q invalid "$config"; then
  echo "invalid test config" >&2
  exit 1
fi
exit 0
`
	if err := os.WriteFile(xrayPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SUI_XRAY_PATH", xrayPath)
	t.Setenv("SUI_BIN_FOLDER", dir)

	xray := NewXrayRuntime()
	if err := xray.Validate([]byte(`{"valid":true}`)); err != nil {
		t.Fatalf("Validate(valid) error = %v", err)
	}
	err := xray.Validate([]byte(`{"invalid":true}`))
	if err == nil || !strings.Contains(err.Error(), "invalid test config") {
		t.Fatalf("Validate(invalid) error = %v", err)
	}
}
