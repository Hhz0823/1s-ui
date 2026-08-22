package model

import "encoding/json"

type AgentNode struct {
	Id            uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	Name          string          `json:"name" gorm:"size:80;not null"`
	TokenHash     string          `json:"-" gorm:"size:64;uniqueIndex;not null"`
	PairCodeHash  string          `json:"-" gorm:"size:64;index"`
	PairExpiresAt int64           `json:"-" gorm:"index;not null;default:0"`
	CreatedAt     int64           `json:"created_at" gorm:"not null"`
	LastSeen      int64           `json:"last_seen" gorm:"index;not null;default:0"`
	RemoteIP      string          `json:"remote_ip" gorm:"size:64"`
	PublicHost    string          `json:"public_host" gorm:"size:255"`
	Version       string          `json:"version" gorm:"size:64"`
	Report        json.RawMessage `json:"report" gorm:"serializer:json"`
}
