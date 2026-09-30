package service

import (
	"sync"
	"time"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type onlines struct {
	Inbound  []string `json:"inbound,omitempty"`
	User     []string `json:"user,omitempty"`
	Outbound []string `json:"outbound,omitempty"`
}

var onlineResources = struct {
	sync.RWMutex
	value onlines
}{}

var statsPersistenceMu sync.Mutex

type StatsService struct {
}

const maxUserTrafficRange = 90 * 24 * time.Hour

type UserTrafficRankingItem struct {
	Rank                       int    `json:"rank"`
	Name                       string `json:"name"`
	Group                      string `json:"group"`
	Description                string `json:"description"`
	Exists                     bool   `json:"exists"`
	Enabled                    bool   `json:"enabled"`
	Online                     bool   `json:"online"`
	UploadBytes                int64  `json:"upload_bytes"`
	DownloadBytes              int64  `json:"download_bytes"`
	TotalBytes                 int64  `json:"total_bytes"`
	AverageUploadBytesPerSec   int64  `json:"average_upload_bytes_per_sec"`
	AverageDownloadBytesPerSec int64  `json:"average_download_bytes_per_sec"`
	PeakUploadBytesPerSec      int64  `json:"peak_upload_bytes_per_sec"`
	PeakDownloadBytesPerSec    int64  `json:"peak_download_bytes_per_sec"`
	LastActive                 int64  `json:"last_active"`
}

type UserTrafficSummary struct {
	ActiveUsers                int   `json:"active_users"`
	UploadBytes                int64 `json:"upload_bytes"`
	DownloadBytes              int64 `json:"download_bytes"`
	TotalBytes                 int64 `json:"total_bytes"`
	AverageUploadBytesPerSec   int64 `json:"average_upload_bytes_per_sec"`
	AverageDownloadBytesPerSec int64 `json:"average_download_bytes_per_sec"`
	PeakUploadBytesPerSec      int64 `json:"peak_upload_bytes_per_sec"`
	PeakDownloadBytesPerSec    int64 `json:"peak_download_bytes_per_sec"`
}

type UserTrafficRankingResponse struct {
	Enabled       bool                     `json:"enabled"`
	Start         int64                    `json:"start"`
	End           int64                    `json:"end"`
	BucketSeconds int64                    `json:"bucket_seconds"`
	RetentionDays int                      `json:"retention_days"`
	Summary       UserTrafficSummary       `json:"summary"`
	Items         []UserTrafficRankingItem `json:"items"`
}

func (s *StatsService) SaveStats(enableTraffic bool, bucketSeconds int64) (err error) {
	if corePtr == nil || !corePtr.IsRunning() {
		return nil
	}
	box := corePtr.GetInstance()
	if box == nil {
		return nil
	}
	st := box.StatsTracker()
	if st == nil {
		return nil
	}
	statsPersistenceMu.Lock()
	defer statsPersistenceMu.Unlock()
	stats := st.GetStats()

	nextOnlines := onlines{}
	if len(*stats) == 0 {
		setOnlineResources(nextOnlines)
		return nil
	}

	db := database.GetDB()
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback().Error
			return
		}
		err = tx.Commit().Error
		if err == nil && enableTraffic {
			invalidatePersistedInboundTraffic()
		}
	}()

	now := time.Now().Unix()

	type traffic struct{ up, down int64 }
	userTraffic := map[string]*traffic{}
	inboundTraffic := map[string]int64{}
	seenInbound := map[string]bool{}
	seenOutbound := map[string]bool{}
	for _, stat := range *stats {
		switch stat.Resource {
		case "inbound":
			inboundTraffic[stat.Tag] += stat.Traffic
			if !seenInbound[stat.Tag] {
				seenInbound[stat.Tag] = true
				nextOnlines.Inbound = append(nextOnlines.Inbound, stat.Tag)
			}
		case "outbound":
			if !seenOutbound[stat.Tag] {
				seenOutbound[stat.Tag] = true
				nextOnlines.Outbound = append(nextOnlines.Outbound, stat.Tag)
			}
		case "user":
			t, ok := userTraffic[stat.Tag]
			if !ok {
				t = &traffic{}
				userTraffic[stat.Tag] = t
				nextOnlines.User = append(nextOnlines.User, stat.Tag)
			}
			if stat.Direction {
				t.up += stat.Traffic
			} else {
				t.down += stat.Traffic
			}
		}
	}
	setOnlineResources(nextOnlines)

	for name, t := range userTraffic {
		update := map[string]interface{}{"online_at": now}
		if t.up > 0 {
			update["up"] = gorm.Expr("up + ?", t.up)
		}
		if t.down > 0 {
			update["down"] = gorm.Expr("down + ?", t.down)
		}
		err = tx.Model(model.Client{}).Where("name = ?", name).Updates(update).Error
		if err != nil {
			return err
		}
	}

	// Monthly usage for per-port traffic caps, counted even without history.
	for tag, total := range inboundTraffic {
		if total <= 0 {
			continue
		}
		err = tx.Model(model.Inbound{}).Where("tag = ?", tag).
			Update("traffic_used", gorm.Expr("traffic_used + ?", total)).Error
		if err != nil {
			return err
		}
	}

	if !enableTraffic {
		return nil
	}

	if bucketSeconds < 1 {
		bucketSeconds = 1
	}
	bucket := now - (now % bucketSeconds)
	for i := range *stats {
		(*stats)[i].DateTime = bucket
	}
	err = tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "resource"}, {Name: "tag"}, {Name: "date_time"}, {Name: "direction"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"traffic": gorm.Expr("stats.traffic + excluded.traffic")}),
	}).Create(&stats).Error
	return err
}

func (s *StatsService) GetStats(resource string, tag string, limit int, start int64, end int64) (any, error) {
	var startTime, endTime int64
	if start > 0 && end > start {
		startTime, endTime = start, end
	} else {
		endTime = time.Now().Unix()
		startTime = endTime - (int64(limit) * 3600)
	}

	bucketSeconds, _ := (&SettingService{}).GetStatsBucketSeconds()
	if bucketSeconds < 1 {
		bucketSeconds = 1
	}
	numBuckets := 360
	if maxBuckets := (endTime - startTime) / bucketSeconds; maxBuckets < int64(numBuckets) {
		numBuckets = int(maxBuckets)
	}
	if numBuckets < 1 {
		numBuckets = 1
	}
	if resource == "user" {
		return s.getAggregateUserStats(startTime, endTime, numBuckets, tag)
	}

	var result []model.Stats
	db := database.GetDB()
	resources := []string{resource}
	if resource == "endpoint" {
		resources = []string{"inbound", "outbound"}
	}
	err := db.Model(model.Stats{}).Where("resource in ? AND tag = ? AND date_time > ? AND date_time <= ?", resources, tag, startTime, endTime).Order("date_time ASC").Scan(&result).Error
	if err != nil {
		return nil, err
	}

	return s.downsampleStats(result, startTime, endTime, numBuckets), nil
}

func (s *StatsService) getAggregateUserStats(startTime, endTime int64, numBuckets int, tag string) (any, error) {
	bucketSpan := (endTime - startTime) / int64(numBuckets)
	if bucketSpan < 1 {
		bucketSpan = 1
	}
	type bucketRow struct {
		Bucket   int64
		Upload   int64
		Download int64
	}
	var rows []bucketRow
	query := `SELECT CAST((date_time - ?) / ? AS INTEGER) AS bucket,
		SUM(CASE WHEN direction = 1 THEN traffic ELSE 0 END) AS upload,
		SUM(CASE WHEN direction = 0 THEN traffic ELSE 0 END) AS download
		FROM stats
		WHERE resource = 'user' AND date_time > ? AND date_time <= ?`
	args := []any{startTime, bucketSpan, startTime, endTime}
	if tag != "" {
		query += ` AND tag = ?`
		args = append(args, tag)
	}
	query += ` GROUP BY bucket ORDER BY bucket`
	err := database.GetDB().Raw(query, args...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]int64, len(rows))
	for _, row := range rows {
		bucket := row.Bucket
		if bucket < 0 {
			bucket = 0
		}
		if bucket >= int64(numBuckets) {
			bucket = int64(numBuckets) - 1
		}
		if _, ok := result[bucket]; !ok {
			result[bucket] = []int64{0, 0}
		}
		result[bucket][0] += row.Upload
		result[bucket][1] += row.Download
	}
	return map[string]any{"stats": result, "startTime": startTime, "bucketSpan": bucketSpan, "numBuckets": numBuckets}, nil
}

func (s *StatsService) GetUserTrafficRanking(start, end int64, limit int) (*UserTrafficRankingResponse, error) {
	start, end, err := normalizeUserTrafficWindow(start, end)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	bucketSeconds, err := (&SettingService{}).GetStatsBucketSeconds()
	if err != nil {
		return nil, err
	}
	retentionDays, err := (&SettingService{}).GetTrafficAge()
	if err != nil {
		return nil, err
	}
	response := &UserTrafficRankingResponse{
		Enabled: retentionDays > 0, Start: start, End: end,
		BucketSeconds: bucketSeconds, RetentionDays: retentionDays,
		Items: []UserTrafficRankingItem{},
	}
	if retentionDays <= 0 {
		return response, nil
	}
	retentionStart := time.Now().AddDate(0, 0, -retentionDays).Unix()
	if end <= retentionStart {
		return response, nil
	}
	if start < retentionStart {
		start = retentionStart
		response.Start = start
	}

	type aggregateRow struct {
		Name               string
		Group              string
		Description        string
		Exists             int
		Enabled            int
		UploadBytes        int64
		DownloadBytes      int64
		PeakUploadBucket   int64
		PeakDownloadBucket int64
		LastActive         int64
	}
	var rows []aggregateRow
	db := database.GetDB()
	err = db.Raw(`WITH user_buckets AS (
		SELECT tag, date_time,
			SUM(CASE WHEN direction = 1 THEN traffic ELSE 0 END) AS upload_bytes,
			SUM(CASE WHEN direction = 0 THEN traffic ELSE 0 END) AS download_bytes
		FROM stats
		WHERE resource = 'user' AND date_time >= ? AND date_time < ?
		GROUP BY tag, date_time
	)
	SELECT b.tag AS name,
		MAX(COALESCE(c."group", '')) AS "group",
		MAX(COALESCE(c."desc", '')) AS description,
		MAX(CASE WHEN c.id IS NULL THEN 0 ELSE 1 END) AS "exists",
		MAX(CASE WHEN c.enable = 1 THEN 1 ELSE 0 END) AS enabled,
		SUM(b.upload_bytes) AS upload_bytes,
		SUM(b.download_bytes) AS download_bytes,
		MAX(b.upload_bytes) AS peak_upload_bucket,
		MAX(b.download_bytes) AS peak_download_bucket,
		MAX(b.date_time) AS last_active
	FROM user_buckets b
	LEFT JOIN clients c ON c.name = b.tag
	GROUP BY b.tag
	ORDER BY (SUM(b.upload_bytes) + SUM(b.download_bytes)) DESC, b.tag ASC
	LIMIT ?`, start, end, limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	type summaryRow struct {
		ActiveUsers        int
		UploadBytes        int64
		DownloadBytes      int64
		PeakUploadBucket   int64
		PeakDownloadBucket int64
	}
	var summary summaryRow
	err = db.Raw(`WITH user_buckets AS (
		SELECT tag, date_time,
			SUM(CASE WHEN direction = 1 THEN traffic ELSE 0 END) AS upload_bytes,
			SUM(CASE WHEN direction = 0 THEN traffic ELSE 0 END) AS download_bytes
		FROM stats
		WHERE resource = 'user' AND date_time >= ? AND date_time < ?
		GROUP BY tag, date_time
	), global_buckets AS (
		SELECT date_time, SUM(upload_bytes) AS upload_bytes, SUM(download_bytes) AS download_bytes
		FROM user_buckets GROUP BY date_time
	)
	SELECT
		(SELECT COUNT(DISTINCT tag) FROM user_buckets) AS active_users,
		COALESCE(SUM(upload_bytes), 0) AS upload_bytes,
		COALESCE(SUM(download_bytes), 0) AS download_bytes,
		COALESCE(MAX(upload_bytes), 0) AS peak_upload_bucket,
		COALESCE(MAX(download_bytes), 0) AS peak_download_bucket
	FROM global_buckets`, start, end).Scan(&summary).Error
	if err != nil {
		return nil, err
	}

	duration := end - start
	onlines, err := s.GetOnlines()
	if err != nil {
		return nil, err
	}
	onlineUsers := make(map[string]bool, len(onlines.User))
	for _, name := range onlines.User {
		onlineUsers[name] = true
	}
	response.Items = make([]UserTrafficRankingItem, 0, len(rows))
	for index, row := range rows {
		response.Items = append(response.Items, UserTrafficRankingItem{
			Rank: index + 1, Name: row.Name, Group: row.Group, Description: row.Description,
			Exists: row.Exists != 0, Enabled: row.Enabled != 0, Online: onlineUsers[row.Name],
			UploadBytes: row.UploadBytes, DownloadBytes: row.DownloadBytes,
			TotalBytes:                 row.UploadBytes + row.DownloadBytes,
			AverageUploadBytesPerSec:   bytesPerSecond(row.UploadBytes, duration),
			AverageDownloadBytesPerSec: bytesPerSecond(row.DownloadBytes, duration),
			PeakUploadBytesPerSec:      bytesPerSecond(row.PeakUploadBucket, bucketSeconds),
			PeakDownloadBytesPerSec:    bytesPerSecond(row.PeakDownloadBucket, bucketSeconds),
			LastActive:                 row.LastActive,
		})
	}
	response.Summary = UserTrafficSummary{
		ActiveUsers: summary.ActiveUsers,
		UploadBytes: summary.UploadBytes, DownloadBytes: summary.DownloadBytes,
		TotalBytes:                 summary.UploadBytes + summary.DownloadBytes,
		AverageUploadBytesPerSec:   bytesPerSecond(summary.UploadBytes, duration),
		AverageDownloadBytesPerSec: bytesPerSecond(summary.DownloadBytes, duration),
		PeakUploadBytesPerSec:      bytesPerSecond(summary.PeakUploadBucket, bucketSeconds),
		PeakDownloadBytesPerSec:    bytesPerSecond(summary.PeakDownloadBucket, bucketSeconds),
	}
	return response, nil
}

func normalizeUserTrafficWindow(start, end int64) (int64, int64, error) {
	now := time.Now().Unix()
	if end <= 0 || end > now {
		end = now
	}
	if start <= 0 {
		start = end - int64(24*time.Hour/time.Second)
	}
	if start >= end {
		return 0, 0, common.NewError("traffic range start must be before end")
	}
	if time.Duration(end-start)*time.Second > maxUserTrafficRange {
		return 0, 0, common.NewError("traffic range cannot exceed 90 days")
	}
	return start, end, nil
}

func bytesPerSecond(bytes, seconds int64) int64 {
	if bytes <= 0 || seconds <= 0 {
		return 0
	}
	return bytes / seconds
}

func (s *StatsService) downsampleStats(stats []model.Stats, startTime, endTime int64, numBuckets int) any {
	result := make(map[int64][]int64)
	bucketSpan := (endTime - startTime) / int64(numBuckets)
	if bucketSpan == 0 {
		bucketSpan = 1
	}

	for _, r := range stats {
		bucket := (r.DateTime - startTime) / bucketSpan
		if bucket < 0 {
			bucket = 0
		}
		if bucket >= int64(numBuckets) {
			bucket = int64(numBuckets) - 1
		}
		if _, ok := result[bucket]; !ok {
			result[bucket] = []int64{0, 0}
		}
		if r.Direction {
			result[bucket][0] += r.Traffic
		} else {
			result[bucket][1] += r.Traffic
		}
	}

	return map[string]any{"stats": result, "startTime": startTime, "bucketSpan": bucketSpan, "numBuckets": numBuckets}
}

func (s *StatsService) GetOnlines() (onlines, error) {
	onlineResources.RLock()
	defer onlineResources.RUnlock()
	return cloneOnlines(onlineResources.value), nil
}

func setOnlineResources(value onlines) {
	onlineResources.Lock()
	onlineResources.value = cloneOnlines(value)
	onlineResources.Unlock()
}

func cloneOnlines(value onlines) onlines {
	return onlines{
		Inbound:  append([]string(nil), value.Inbound...),
		User:     append([]string(nil), value.User...),
		Outbound: append([]string(nil), value.Outbound...),
	}
}
func (s *StatsService) DelOldStats(days int) error {
	statsPersistenceMu.Lock()
	defer statsPersistenceMu.Unlock()
	oldTime := time.Now().AddDate(0, 0, -(days)).Unix()
	db := database.GetDB()
	if err := db.Where("date_time < ?", oldTime).Delete(model.Stats{}).Error; err != nil {
		return err
	}
	invalidatePersistedInboundTraffic()
	return nil
}
