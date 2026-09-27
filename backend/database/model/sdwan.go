package model

import "encoding/json"

// SdwanMember is a managed server whose SD-WAN uplinks the controller can use
// as egress paths. Paths holds one sing-box outbound (with credentials) per
// uplink protocol and is never sent to the browser.
type SdwanMember struct {
	Id        uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeId    uint            `json:"node_id" gorm:"uniqueIndex;not null"`
	Server    string          `json:"server" gorm:"size:255"`
	Paths     json.RawMessage `json:"-" gorm:"serializer:json"`
	CreatedAt int64           `json:"created_at"`
	UpdatedAt int64           `json:"updated_at"`
}
