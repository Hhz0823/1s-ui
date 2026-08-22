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
)

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
			if k == "upload_limit" || k == "download_limit" {
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
	combined["tls_id"] = i.TlsId
	combined["addrs"] = i.Addrs
	combined["out_json"] = i.OutJson

	if i.Options != nil {
		var restFields map[string]interface{}
		if err := json.Unmarshal(i.Options, &restFields); err != nil {
			return nil, err
		}

		for k, v := range restFields {
			if k == "upload_limit" || k == "download_limit" {
				continue
			}
			combined[k] = v
		}
	}
	return &combined, nil
}
