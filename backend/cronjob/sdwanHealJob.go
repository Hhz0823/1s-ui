package cronjob

import (
	"github.com/Hhz0823/1s-ui/service"
)

// SdwanHealJob moves SD-WAN traffic off a failed exit within seconds.
type SdwanHealJob struct {
	service.SdwanService
}

func NewSdwanHealJob() *SdwanHealJob {
	return &SdwanHealJob{}
}

func (s *SdwanHealJob) Run() {
	s.SdwanService.HealSdwan()
}
