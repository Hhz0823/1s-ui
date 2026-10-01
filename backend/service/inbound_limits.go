package service

import (
	"time"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util/common"

	"gorm.io/gorm"
)

// inboundCoreLimit converts the stored limits of a sing-box inbound into what
// the stats tracker enforces.
func inboundCoreLimit(inbound model.Inbound) core.InboundBandwidthLimit {
	limit := core.InboundBandwidthLimit{
		Upload:   inbound.UploadLimit,
		Download: inbound.DownloadLimit,
		MaxIPs:   inbound.IPLimit,
	}
	if inbound.TrafficLimit > 0 {
		limit.QuotaEnabled = true
		limit.QuotaRemaining = inbound.TrafficLimit - inbound.TrafficUsed
	}
	return limit
}

// trafficPeriodStart returns the start of the monthly period containing now
// for a cap that resets on resetDay. Months shorter than resetDay reset on
// their last day.
func trafficPeriodStart(now time.Time, resetDay int) time.Time {
	if resetDay < 1 {
		resetDay = 1
	}
	start := monthDay(now.Year(), now.Month(), resetDay, now.Location())
	if now.Before(start) {
		start = monthDay(now.Year(), now.Month()-1, resetDay, now.Location())
	}
	return start
}

// nextTrafficReset returns when the period that started at start ends.
func nextTrafficReset(start time.Time, resetDay int) time.Time {
	if resetDay < 1 {
		resetDay = 1
	}
	return monthDay(start.Year(), start.Month()+1, resetDay, start.Location())
}

func monthDay(year int, month time.Month, day int, loc *time.Location) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	last := first.AddDate(0, 1, -1).Day()
	if day > last {
		day = last
	}
	return time.Date(first.Year(), first.Month(), day, 0, 0, 0, 0, loc)
}

func trafficLocation() *time.Location {
	loc, err := (&SettingService{}).GetTimeLocation()
	if err != nil || loc == nil {
		return time.Local
	}
	return loc
}

// RollTrafficPeriods starts a new period, with nothing used, for every
// inbound whose reset day has passed.
func (s *InboundService) RollTrafficPeriods(db *gorm.DB, now time.Time) error {
	var inbounds []model.Inbound
	err := db.Model(&model.Inbound{}).Select("id", "traffic_reset_day", "traffic_period_start").Find(&inbounds).Error
	if err != nil {
		return err
	}
	for _, inbound := range inbounds {
		start := trafficPeriodStart(now, inbound.TrafficResetDay).Unix()
		if inbound.TrafficPeriodStart >= start {
			continue
		}
		err = db.Model(&model.Inbound{}).Where("id = ? AND traffic_period_start < ?", inbound.Id, start).
			Updates(map[string]interface{}{"traffic_used": 0, "traffic_period_start": start}).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// EnforceTrafficLimits rolls finished periods over and pushes the remaining
// monthly traffic of every inbound to the core. The stats job runs it right
// after flushing the counters, so the stored usage is current.
func (s *InboundService) EnforceTrafficLimits() error {
	db := database.GetDB()
	if err := s.RollTrafficPeriods(db, time.Now().In(trafficLocation())); err != nil {
		return err
	}
	return s.SyncCoreBandwidthLimits(db)
}

// ResetTrafficUsage clears the monthly usage of one inbound and reopens it
// if it had reached its cap.
func (s *InboundService) ResetTrafficUsage(id uint) error {
	db := database.GetDB()
	var inbound model.Inbound
	if err := db.Model(&model.Inbound{}).Where("id = ?", id).First(&inbound).Error; err != nil {
		return common.NewError("inbound not found")
	}
	start := trafficPeriodStart(time.Now().In(trafficLocation()), inbound.TrafficResetDay).Unix()
	err := db.Model(&model.Inbound{}).Where("id = ?", id).
		Updates(map[string]interface{}{"traffic_used": 0, "traffic_period_start": start}).Error
	if err != nil {
		return err
	}
	return s.SyncCoreBandwidthLimits(db)
}
