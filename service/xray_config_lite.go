//go:build openwrt_lite

package service

func (s *ConfigService) HasXrayInbounds() (bool, error) {
	return false, nil
}

func (s *ConfigService) GetXrayConfig() (*[]byte, error) {
	rawConfig := []byte("{}")
	return &rawConfig, nil
}
