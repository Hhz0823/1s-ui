package cronjob

import (
	"github.com/Hhz0823/1s-ui/config"
	"github.com/Hhz0823/1s-ui/service"
)

type CheckCoreJob struct {
	service.ConfigService
}

func NewCheckCoreJob() *CheckCoreJob {
	return &CheckCoreJob{}
}

func (s *CheckCoreJob) Run() {
	// Respect safe-mode installs that intentionally do not auto-start cores,
	// unless the proxy client, which runs in sing-box, is switched on.
	if config.IsSkipCore() && !service.ProxyClientWantsCore() {
		return
	}
	s.ConfigService.StartCore()
}
