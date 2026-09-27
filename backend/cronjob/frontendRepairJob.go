package cronjob

import (
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/service"
)

// FrontendRepairJob installs the web UI of the running panel's release when
// the installed UI is another version, as after an update by a panel whose
// updater replaced only the binary.
type FrontendRepairJob struct {
	service.UpdateService
}

func NewFrontendRepairJob() *FrontendRepairJob {
	return &FrontendRepairJob{}
}

func (s *FrontendRepairJob) Run() {
	if _, err := s.UpdateService.RepairFrontend(); err != nil {
		logger.Warning("install the web UI of this panel version: ", err)
	}
}
