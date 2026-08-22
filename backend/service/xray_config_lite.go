//go:build openwrt_lite

package service

import "gorm.io/gorm"

func (s *ConfigService) HasXrayInbounds() (bool, error) {
	return false, nil
}

func (s *ConfigService) GetXrayConfig() (*[]byte, error) {
	rawConfig := []byte("{}")
	return &rawConfig, nil
}

func (s *ConfigService) validateXrayConfig(db *gorm.DB) error {
	return nil
}
