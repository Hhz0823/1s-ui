package service

import (
	"encoding/json"
	"fmt"
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
	// Monthly cap: bytes used this period, the cap, and when it resets.
	TrafficLimit    int64 `json:"traffic_limit"`
	TrafficUsed     int64 `json:"traffic_used"`
	TrafficResetDay int   `json:"traffic_reset_day"`
	NextReset       int64 `json:"next_reset"`
	Exhausted       bool  `json:"exhausted"`
	IPLimit         int   `json:"ip_limit"`
	ActiveIPs       int   `json:"active_ips"`
	// What the monitor app shows per node; none of it is a secret.
	Security   string `json:"security,omitempty"`
	Transport  string `json:"transport,omitempty"`
	Encryption bool   `json:"encryption,omitempty"`
	Users      int    `json:"users"`
	CDN        string `json:"cdn,omitempty"`
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
	activeIPs := make(map[string]int)
	if corePtr != nil && corePtr.IsRunning() && corePtr.GetInstance() != nil && corePtr.GetInstance().StatsTracker() != nil {
		sampledAt, traffic = corePtr.GetInstance().StatsTracker().InboundTrafficSnapshot()
		activeIPs = corePtr.GetInstance().StatsTracker().InboundActiveIPs()
	}
	now := time.Now().In(trafficLocation())
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

	reality, users := inboundSecurityAndUsers()
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
			TrafficLimit: inbound.TrafficLimit, TrafficResetDay: inbound.TrafficResetDay,
			TrafficUsed: inbound.TrafficUsed + current.UploadPending + current.DownloadPending,
			IPLimit:     inbound.IPLimit, ActiveIPs: activeIPs[inbound.Tag],
		}
		periodStart := trafficPeriodStart(now, inbound.TrafficResetDay)
		if inbound.TrafficPeriodStart < periodStart.Unix() {
			// The stats job has not rolled this period over yet.
			item.TrafficUsed = current.UploadPending + current.DownloadPending
		}
		item.NextReset = nextTrafficReset(periodStart, inbound.TrafficResetDay).Unix()
		item.Exhausted = item.TrafficLimit > 0 && item.TrafficUsed >= item.TrafficLimit
		item.UploadBytes, item.DownloadBytes = portTrafficTotals(persisted[inbound.Tag], current, trafficAge > 0)
		describeInbound(&item, inbound, reality)
		item.Users = users[inbound.Id]
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

// inboundSecurityAndUsers reads which TLS configs are REALITY and how many
// enabled users each inbound has.
func inboundSecurityAndUsers() (map[uint]bool, map[uint]int) {
	reality := map[uint]bool{}
	var configs []model.Tls
	if err := database.GetDB().Select("id", "server").Find(&configs).Error; err == nil {
		for _, config := range configs {
			var server struct {
				Reality struct {
					Enabled bool `json:"enabled"`
				} `json:"reality"`
			}
			if json.Unmarshal(config.Server, &server) == nil && server.Reality.Enabled {
				reality[config.Id] = true
			}
		}
	}
	users := map[uint]int{}
	var counts []struct {
		InboundId uint
		Users     int
	}
	if err := database.GetDB().Raw(`SELECT je.value AS inbound_id, COUNT(*) AS users
		FROM clients, json_each(clients.inbounds) AS je WHERE clients.enable = 1 GROUP BY je.value`).Scan(&counts).Error; err == nil {
		for _, count := range counts {
			users[count.InboundId] = count.Users
		}
	}
	return reality, users
}

func describeInbound(item *PortTrafficItem, inbound model.Inbound, reality map[uint]bool) {
	if inbound.TlsId > 0 {
		item.Security = "tls"
		if reality[inbound.TlsId] {
			item.Security = "reality"
		}
	}
	var options struct {
		Transport struct {
			Type string `json:"type"`
		} `json:"transport"`
		Decryption string `json:"decryption"`
		CDN        struct {
			Domain string `json:"domain"`
			Port   int    `json:"port"`
		} `json:"cdn"`
	}
	if json.Unmarshal(inbound.Options, &options) != nil {
		return
	}
	item.Transport = options.Transport.Type
	item.Encryption = options.Decryption != "" && options.Decryption != "none"
	if options.CDN.Domain != "" {
		item.CDN = fmt.Sprintf("%s:%d", options.CDN.Domain, options.CDN.Port)
	}
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
