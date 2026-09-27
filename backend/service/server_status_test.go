package service

import (
	"runtime"
	"testing"
)

func TestGetStatusReportsLoadAverages(t *testing.T) {
	status := *(&ServerService{}).GetStatus("lod")
	info, ok := status["lod"].(map[string]interface{})
	if !ok {
		t.Fatalf("status has no load info: %#v", status)
	}
	if cpus, _ := info["cpus"].(int); cpus < 1 {
		t.Fatalf("cpus = %#v", info["cpus"])
	}
	if runtime.GOOS != "linux" {
		return
	}
	for _, key := range []string{"load1", "load5", "load15"} {
		if value, ok := info[key].(float64); !ok || value < 0 {
			t.Fatalf("%s = %#v", key, info[key])
		}
	}
}

func TestGetSystemInfoReportsOperatingSystem(t *testing.T) {
	setupQuickAddTest(t) // host requirements read the settings table
	info := (&ServerService{}).GetSystemInfo()
	if runtime.GOOS != "linux" {
		return
	}
	if os, _ := info["os"].(string); os == "" {
		t.Fatalf("os = %#v", info["os"])
	}
	if kernel, _ := info["kernel"].(string); kernel == "" {
		t.Fatalf("kernel = %#v", info["kernel"])
	}
}
