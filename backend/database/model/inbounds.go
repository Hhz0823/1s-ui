package model

import (
	"encoding/json"
	"fmt"
	"math"
)

type Inbound struct {
	Id       uint   `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Type     string `json:"type" form:"type"`
	Tag      string `json:"tag" form:"tag" gorm:"unique"`
	CoreType string `json:"core_type" form:"core_type" gorm:"default:sing-box;index"`

	UploadLimit   int64 `json:"upload_limit" form:"upload_limit" gorm:"default:0;not null"`
	DownloadLimit int64 `json:"download_limit" form:"download_limit" gorm:"default:0;not null"`

	// Monthly traffic cap in bytes (upload + download); 0 means unlimited.
	TrafficLimit int64 `json:"traffic_limit" form:"traffic_limit" gorm:"default:0;not null"`
	// Day of month (1-31) the cap resets; months without that day reset on their last day.
	TrafficResetDay int `json:"traffic_reset_day" form:"traffic_reset_day" gorm:"default:1;not null"`
	// Traffic counted since TrafficPeriodStart. Maintained by the stats job, never by the editor.
	TrafficUsed        int64 `json:"traffic_used" form:"-" gorm:"default:0;not null"`
	TrafficPeriodStart int64 `json:"traffic_period_start" form:"-" gorm:"default:0;not null"`
	// Maximum distinct client IPs with open connections; 0 means unlimited.
	IPLimit int `json:"ip_limit" form:"ip_limit" gorm:"column:ip_limit;default:0;not null"`

	// Foreign key to tls table
	TlsId uint `json:"tls_id" form:"tls_id"`
	Tls   *Tls `json:"tls" form:"tls" gorm:"foreignKey:TlsId;references:Id"`

	Addrs   json.RawMessage `json:"addrs" form:"addrs"`
	OutJson json.RawMessage `json:"out_json" form:"out_json"`
	Options json.RawMessage `json:"-" form:"-"`
}

const (
	CoreTypeSingBox = "sing-box"
	CoreTypeXray    = "xray"
	// 10 GiB/s is above practical panel use while still preventing absurd values.
	MaxInboundBandwidthLimit int64 = 10 * 1024 * 1024 * 1024
	// 1 PiB per month is far beyond any VPS plan.
	MaxInboundTrafficLimit int64 = 1 << 50
	MaxInboundIPLimit            = 100000
)

// inboundLimitKeys are stored in columns, never in Options or the core config.
var inboundLimitKeys = map[string]bool{
	"upload_limit": true, "download_limit": true,
	"traffic_limit": true, "traffic_reset_day": true, "traffic_used": true,
	"traffic_period_start": true, "ip_limit": true,
}

// HasSingBoxOnlyLimits reports limits that only the sing-box runtime enforces.
func (i Inbound) HasSingBoxOnlyLimits() bool {
	return i.UploadLimit > 0 || i.DownloadLimit > 0 || i.TrafficLimit > 0 || i.IPLimit > 0
}

func (i Inbound) RuntimeCore() string {
	if i.CoreType == "" {
		return CoreTypeSingBox
	}
	return i.CoreType
}

func (i *Inbound) UnmarshalJSON(data []byte) error {
	var err error
	var raw map[string]interface{}
	if err = json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Extract fixed fields and store the rest in Options
	if val, exists := raw["id"].(float64); exists {
		i.Id = uint(val)
	}
	delete(raw, "id")
	i.Type, _ = raw["type"].(string)
	delete(raw, "type")
	i.Tag, _ = raw["tag"].(string)
	delete(raw, "tag")
	i.CoreType, _ = raw["core_type"].(string)
	if i.CoreType == "" {
		i.CoreType = CoreTypeSingBox
	}
	delete(raw, "core_type")

	i.UploadLimit, err = inboundBandwidthLimit(raw["upload_limit"], "upload_limit")
	if err != nil {
		return err
	}
	delete(raw, "upload_limit")
	i.DownloadLimit, err = inboundBandwidthLimit(raw["download_limit"], "download_limit")
	if err != nil {
		return err
	}
	delete(raw, "download_limit")
	i.TrafficLimit, err = inboundIntegerField(raw["traffic_limit"], "traffic_limit", MaxInboundTrafficLimit)
	if err != nil {
		return err
	}
	resetDay, err := inboundIntegerField(raw["traffic_reset_day"], "traffic_reset_day", 31)
	if err != nil {
		return err
	}
	i.TrafficResetDay = int(resetDay)
	if i.TrafficResetDay < 1 {
		i.TrafficResetDay = 1
	}
	ipLimit, err := inboundIntegerField(raw["ip_limit"], "ip_limit", MaxInboundIPLimit)
	if err != nil {
		return err
	}
	i.IPLimit = int(ipLimit)
	for key := range inboundLimitKeys {
		delete(raw, key)
	}

	// TlsId
	if val, exists := raw["tls_id"].(float64); exists {
		i.TlsId = uint(val)
	}
	delete(raw, "tls_id")
	delete(raw, "tls")
	delete(raw, "users")

	// Addrs
	i.Addrs, _ = json.MarshalIndent(raw["addrs"], "", "  ")
	delete(raw, "addrs")

	// OutJson
	i.OutJson, _ = json.MarshalIndent(raw["out_json"], "", "  ")
	delete(raw, "out_json")

	// Remaining fields
	i.Options, err = json.MarshalIndent(raw, "", "  ")
	return err
}

func inboundIntegerField(value interface{}, name string, max int64) (int64, error) {
	if value == nil {
		return 0, nil
	}
	number, ok := value.(float64)
	if !ok || math.Trunc(number) != number || number < 0 || number > float64(max) {
		return 0, fmt.Errorf("%s must be an integer between 0 and %d", name, max)
	}
	return int64(number), nil
}

func inboundBandwidthLimit(value interface{}, name string) (int64, error) {
	if value == nil {
		return 0, nil
	}
	number, ok := value.(float64)
	if !ok || math.Trunc(number) != number || number < 0 || number > float64(MaxInboundBandwidthLimit) {
		return 0, fmt.Errorf("%s must be an integer between 0 and %d bytes/s", name, MaxInboundBandwidthLimit)
	}
	return int64(number), nil
}

// MarshalJSON customizes marshalling
func (i Inbound) MarshalJSON() ([]byte, error) {
	// Combine fixed fields and dynamic fields into one map
	combined := make(map[string]interface{})
	combined["type"] = i.Type
	combined["tag"] = i.Tag
	if i.Tls != nil {
		var tls map[string]interface{}
		if err := json.Unmarshal(i.Tls.Server, &tls); err == nil && i.Type == "tuic" {
			hasALPN := false
			switch alpn := tls["alpn"].(type) {
			case []interface{}:
				hasALPN = len(alpn) > 0
			case []string:
				hasALPN = len(alpn) > 0
			case string:
				hasALPN = alpn != ""
			}
			if !hasALPN {
				tls["alpn"] = []string{"h3"}
			}
			combined["tls"] = tls
		} else {
			combined["tls"] = i.Tls.Server
		}
	}

	if i.Options != nil {
		var restFields map[string]json.RawMessage
		if err := json.Unmarshal(i.Options, &restFields); err != nil {
			return nil, err
		}

		for k, v := range restFields {
			if inboundLimitKeys[k] {
				continue
			}
			combined[k] = v
		}
	}

	return json.Marshal(combined)
}

func (i Inbound) MarshalFull() (*map[string]interface{}, error) {
	combined := make(map[string]interface{})
	combined["id"] = i.Id
	combined["type"] = i.Type
	combined["tag"] = i.Tag
	combined["core_type"] = i.RuntimeCore()
	combined["upload_limit"] = i.UploadLimit
	combined["download_limit"] = i.DownloadLimit
	combined["traffic_limit"] = i.TrafficLimit
	combined["traffic_reset_day"] = i.TrafficResetDay
	combined["traffic_used"] = i.TrafficUsed
	combined["traffic_period_start"] = i.TrafficPeriodStart
	combined["ip_limit"] = i.IPLimit
	combined["tls_id"] = i.TlsId
	combined["addrs"] = i.Addrs
	combined["out_json"] = i.OutJson

	if i.Options != nil {
		var restFields map[string]interface{}
		if err := json.Unmarshal(i.Options, &restFields); err != nil {
			return nil, err
		}

		for k, v := range restFields {
			if inboundLimitKeys[k] {
				continue
			}
			combined[k] = v
		}
	}
	return &combined, nil
}
