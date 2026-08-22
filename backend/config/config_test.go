package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsXrayDisabledFromEnvironment(t *testing.T) {
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	t.Setenv("SUI_DISABLE_XRAY", "yes")
	if !IsXrayDisabled() {
		t.Fatal("SUI_DISABLE_XRAY=yes did not disable Xray")
	}
}

func TestIsXrayDisabledFromMarker(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dbDir)
	t.Setenv("SUI_DISABLE_XRAY", "")
	if err := os.WriteFile(filepath.Join(dbDir, ".disable_xray"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if !IsXrayDisabled() {
		t.Fatal(".disable_xray marker did not disable Xray")
	}
}

func TestXrayRuntimeEnabledMarkerOverridesLegacyEnvironment(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dbDir)
	t.Setenv("SUI_DISABLE_XRAY", "true")
	if err := os.WriteFile(filepath.Join(dbDir, ".xray_enabled"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if IsXrayDisabled() {
		t.Fatal("runtime-enabled marker did not override the legacy disabled environment")
	}
}

func TestIsXrayOnDemandFromEnvironment(t *testing.T) {
	t.Setenv("SUI_XRAY_ON_DEMAND", "yes")
	if !IsXrayOnDemand() {
		t.Fatal("SUI_XRAY_ON_DEMAND=yes did not enable on-demand mode")
	}
}

func TestIsXrayOnDemandFromMarker(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dbDir)
	t.Setenv("SUI_XRAY_ON_DEMAND", "")
	if err := os.WriteFile(filepath.Join(dbDir, ".xray_on_demand"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if !IsXrayOnDemand() {
		t.Fatal(".xray_on_demand marker did not enable on-demand mode")
	}
}

func TestIsControllerModeRequestedFromMarker(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dbDir)
	t.Setenv("SUI_CONTROLLER_MODE", "")
	if err := os.WriteFile(filepath.Join(dbDir, ".controller_mode"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if !IsControllerModeRequested() {
		t.Fatal(".controller_mode marker did not request controller mode")
	}
}
