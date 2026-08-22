//go:build !linux

package service

import "github.com/Hhz0823/1s-ui/util/common"

func reverseProxyStatusPlatform(_ *ReverseProxyService) (*ReverseProxyStatus, error) {
	status, err := reverseProxyPanelSettings()
	if err != nil {
		return nil, err
	}
	status.Message = "system reverse proxy management is only supported on Linux with systemd"
	return status, nil
}

func applyReverseProxyPlatform(_ *ReverseProxyService, _ ReverseProxyConfig) (*ReverseProxyStatus, error) {
	return nil, common.NewError("system reverse proxy management is only supported on Linux with systemd")
}
