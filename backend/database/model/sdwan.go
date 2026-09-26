package model

import "encoding/json"

// SdwanMember is a managed server whose SD-WAN uplink the controller can use
// as an egress path. Outbound holds the sing-box outbound (with credentials)
// returned by the managed server and is never sent to the browser.
type SdwanMember struct {
	Id        uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeId    uint            `json:"node_id" gorm:"uniqueIndex;not null"`
	Protocol  string          `json:"protocol" gorm:"size:32"`
	Server    string          `json:"server" gorm:"size:255"`
	Port      int             `json:"port"`
	Outbound  json.RawMessage `json:"-" gorm:"serializer:json"`
	CreatedAt int64           `json:"created_at"`
	UpdatedAt int64           `json:"updated_at"`
}
