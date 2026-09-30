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

	// Display metadata set by the admin on the monitor page.
	Group            string  `json:"group" gorm:"size:40;not null;default:''"`
	Tags             string  `json:"tags" gorm:"size:255;not null;default:''"`
	Region           string  `json:"region" gorm:"size:8;not null;default:''"`
	Remark           string  `json:"remark" gorm:"size:255;not null;default:''"`
	Price            float64 `json:"price" gorm:"not null;default:0"`
	Currency         string  `json:"currency" gorm:"size:8;not null;default:''"`
	BillingCycle     int     `json:"billing_cycle" gorm:"not null;default:30"`
	ExpireAt         int64   `json:"expire_at" gorm:"not null;default:0"`
	SortWeight       int     `json:"sort_weight" gorm:"not null;default:0"`
	TrafficLimit     uint64  `json:"traffic_limit" gorm:"not null;default:0"`
	TrafficLimitType string  `json:"traffic_limit_type" gorm:"size:8;not null;default:'sum'"`
	TrafficResetDay  int     `json:"traffic_reset_day" gorm:"not null;default:1"`
}

// AgentMetric is one stored monitor sample. Recent rows are per minute
// (Resolution 60); older ones are merged into 15-minute rows.
type AgentMetric struct {
	Id          uint    `gorm:"primaryKey;autoIncrement"`
	NodeId      uint    `gorm:"index:idx_agent_metric_node_time,priority:1;not null"`
	Time        int64   `gorm:"index:idx_agent_metric_node_time,priority:2;not null"`
	Resolution  int64   `gorm:"not null;default:60"`
	Samples     int     `gorm:"not null;default:0"`
	CPU         float64 `gorm:"not null;default:0"`
	Mem         float64 `gorm:"not null;default:0"`
	Swap        float64 `gorm:"not null;default:0"`
	Disk        float64 `gorm:"not null;default:0"`
	Load1       float64 `gorm:"not null;default:0"`
	Processes   float64 `gorm:"not null;default:0"`
	TCP         float64 `gorm:"not null;default:0"`
	UDP         float64 `gorm:"not null;default:0"`
	NetSentRate float64 `gorm:"not null;default:0"`
	NetRecvRate float64 `gorm:"not null;default:0"`
	NetSent     uint64  `gorm:"not null;default:0"`
	NetRecv     uint64  `gorm:"not null;default:0"`
	PingMs      float64 `gorm:"not null;default:0"`
	PingLoss    float64 `gorm:"not null;default:0"`
}
