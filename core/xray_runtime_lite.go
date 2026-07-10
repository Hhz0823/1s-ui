//go:build openwrt_lite

package core

type XrayRuntime struct{}

func NewXrayRuntime() *XrayRuntime {
	return &XrayRuntime{}
}

func (r *XrayRuntime) Validate(rawConfig []byte) error {
	return nil
}

func (r *XrayRuntime) Start(rawConfig []byte) error {
	return nil
}

func (r *XrayRuntime) Stop() error {
	return nil
}

func (r *XrayRuntime) Restart(rawConfig []byte) error {
	return nil
}

func (r *XrayRuntime) IsRunning() bool {
	return false
}

func (r *XrayRuntime) Status() map[string]interface{} {
	return map[string]interface{}{
		"running":     false,
		"path":        "",
		"config_path": "",
		"last_error":  "Xray-core is disabled in OpenWrt Lite build",
		"last_output": "",
		"stats": map[string]interface{}{
			"Uptime": uint32(0),
		},
	}
}
