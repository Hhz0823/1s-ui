package service

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
)

type PortTrafficResponse struct {
	SampledAt int64             `json:"sampled_at"`
	Items     []PortTrafficItem `json:"items"`
}

type PortTrafficItem struct {
	ID            uint   `json:"id"`
	Tag           string `json:"tag"`
	Type          string `json:"type"`
	CoreType      string `json:"core_type"`
	Listen        string `json:"listen"`
	Port          int    `json:"port"`
	Online        bool   `json:"online"`
	Supported     bool   `json:"supported"`
	UploadBPS     int64  `json:"upload_bps"`
	DownloadBPS   int64  `json:"download_bps"`
	UploadBytes   int64  `json:"upload_bytes"`
	DownloadBytes int64  `json:"download_bytes"`
	UploadLimit   int64  `json:"upload_limit"`
	DownloadLimit int64  `json:"download_limit"`
}

type PortTrafficService struct{}

var persistedInboundTrafficCache struct {
	sync.Mutex
	valid bool
	value map[string]trafficTotalsByDirection
}

func (s *PortTrafficService) GetPortTraffic() (*PortTrafficResponse, error) {
	var inbounds []model.Inbound
	if err := database.GetDB().Order("id ASC").Find(&inbounds).Error; err != nil {
		return nil, err
	}

	trafficAge, err := (&SettingService{}).GetTrafficAge()
	if err != nil {
		return nil, err
	}
	if trafficAge > 0 {
		statsPersistenceMu.Lock()
		defer statsPersistenceMu.Unlock()
	}
	sampledAt := time.Now().Unix()
	traffic := make(map[string]core.InboundTrafficSnapshot)
	if corePtr != nil && corePtr.IsRunning() && corePtr.GetInstance() != nil && corePtr.GetInstance().StatsTracker() != nil {
		sampledAt, traffic = corePtr.GetInstance().StatsTracker().InboundTrafficSnapshot()
	}
	persisted := make(map[string]trafficTotalsByDirection)
	if trafficAge > 0 {
		persisted, err = persistedInboundTraffic()
		if err != nil {
			return nil, err
		}
	}
	onlines, err := (&StatsService{}).GetOnlines()
	if err != nil {
		return nil, err
	}
	online := make(map[string]bool, len(onlines.Inbound))
	for _, tag := range onlines.Inbound {
		online[tag] = true
	}

	items := make([]PortTrafficItem, 0, len(inbounds))
	for _, inbound := range inbounds {
		listen, port := inboundListenPort(inbound.Options)
		if port <= 0 {
			continue
		}
		current := traffic[inbound.Tag]
		item := PortTrafficItem{
			ID: inbound.Id, Tag: inbound.Tag, Type: inbound.Type, CoreType: inbound.RuntimeCore(),
			Listen: listen, Port: port, Online: online[inbound.Tag],
			Supported: inbound.RuntimeCore() == model.CoreTypeSingBox,
			UploadBPS: current.UploadBPS, DownloadBPS: current.DownloadBPS,
			UploadLimit: inbound.UploadLimit, DownloadLimit: inbound.DownloadLimit,
		}
		item.UploadBytes, item.DownloadBytes = portTrafficTotals(persisted[inbound.Tag], current, trafficAge > 0)
		if !item.Supported {
			item.Online = false
			item.UploadBPS = 0
			item.DownloadBPS = 0
		}
		items = append(items, item)
	}
	return &PortTrafficResponse{SampledAt: sampledAt, Items: items}, nil
}

type trafficTotalsByDirection struct {
	upload   int64
	download int64
}

func persistedInboundTraffic() (map[string]trafficTotalsByDirection, error) {
	persistedInboundTrafficCache.Lock()
	defer persistedInboundTrafficCache.Unlock()
	if persistedInboundTrafficCache.valid {
		return persistedInboundTrafficCache.value, nil
	}
	var rows []struct {
		Tag       string
		Direction bool
		Traffic   int64
	}
	err := database.GetDB().Model(&model.Stats{}).
		Select("tag, direction, SUM(traffic) AS traffic").
		Where("resource = ?", "inbound").
		Group("tag, direction").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]trafficTotalsByDirection)
	for _, row := range rows {
		total := result[row.Tag]
		if row.Direction {
			total.upload = row.Traffic
		} else {
			total.download = row.Traffic
		}
		result[row.Tag] = total
	}
	persistedInboundTrafficCache.value = result
	persistedInboundTrafficCache.valid = true
	return result, nil
}

func invalidatePersistedInboundTraffic() {
	persistedInboundTrafficCache.Lock()
	persistedInboundTrafficCache.valid = false
	persistedInboundTrafficCache.value = nil
	persistedInboundTrafficCache.Unlock()
}

func portTrafficTotals(persisted trafficTotalsByDirection, current core.InboundTrafficSnapshot, persistenceEnabled bool) (int64, int64) {
	if persistenceEnabled {
		return persisted.upload + current.UploadPending, persisted.download + current.DownloadPending
	}
	return current.UploadSession, current.DownloadSession
}

func inboundListenPort(raw json.RawMessage) (string, int) {
	var options struct {
		Listen     string `json:"listen"`
		ListenPort int    `json:"listen_port"`
		Port       int    `json:"port"`
	}
	_ = json.Unmarshal(raw, &options)
	if options.ListenPort == 0 {
		options.ListenPort = options.Port
	}
	return options.Listen, options.ListenPort
}
