package cronjob

import (
	"time"

	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/service"
)

// TLSRenewJob moves generated NaiveProxy certificates to a private CA and
// renews the short-lived server certificates such CAs issue.
type TLSRenewJob struct {
	service.ConfigService
}

func NewTLSRenewJob() *TLSRenewJob {
	return &TLSRenewJob{}
}

func (s *TLSRenewJob) Run() {
	now := time.Now()
	if migrated, err := s.ConfigService.MigrateGeneratedNaiveTLS(now); err != nil {
		logger.Warning("move generated Naive certificates to a private CA: ", err)
	} else if migrated > 0 {
		logger.Infof("moved %d generated Naive TLS configuration(s) to a private CA; import those nodes again", migrated)
	}
	if renewed, err := s.ConfigService.RenewGeneratedCertificates(now); err != nil {
		logger.Warning("renew generated certificates: ", err)
	} else if renewed > 0 {
		logger.Infof("renewed %d generated certificate(s)", renewed)
	}
}
