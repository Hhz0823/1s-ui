package model

// ProxyClientSubscription is a node subscription of the panel's own proxy
// client (the PassWall / v2rayN-style client on a home NAS or router).
type ProxyClientSubscription struct {
	Id   uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	Name string `json:"name" gorm:"size:80;not null"`
	// URL often carries an account token; it is only shown to admins.
	URL         string `json:"url" gorm:"size:2048;not null"`
	UserAgent   string `json:"user_agent" gorm:"size:128;not null;default:''"`
	AutoUpdate  int    `json:"auto_update" gorm:"not null;default:24"` // hours, 0 = manual
	Enabled     bool   `json:"enabled" gorm:"not null;default:true"`
	UpdatedAt   int64  `json:"updated_at" gorm:"not null;default:0"`
	LastError   string `json:"last_error" gorm:"size:255;not null;default:''"`
	NodeCount   int    `json:"node_count" gorm:"not null;default:0"`
	Upload      int64  `json:"upload" gorm:"not null;default:0"`
	Download    int64  `json:"download" gorm:"not null;default:0"`
	Total       int64  `json:"total" gorm:"not null;default:0"`
	Expire      int64  `json:"expire" gorm:"not null;default:0"`
	SortOrder   int    `json:"sort_order" gorm:"not null;default:0"`
	CreatedAt   int64  `json:"created_at" gorm:"not null;default:0"`
	ProxyUpdate bool   `json:"proxy_update" gorm:"not null;default:false"`
}

// ProxyClientNode is one node of the proxy client: from a subscription or
// added by hand (SubscriptionId 0). Link carries the node's credentials.
type ProxyClientNode struct {
	Id             uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	SubscriptionId uint   `json:"subscription_id" gorm:"index;not null;default:0"`
	Name           string `json:"name" gorm:"size:128;not null"`
	Protocol       string `json:"protocol" gorm:"size:32;not null"`
	Host           string `json:"host" gorm:"size:255;not null"`
	Port           int    `json:"port" gorm:"not null;default:0"`
	Link           string `json:"-" gorm:"type:text;not null"`
	// Outbound is the sing-box outbound when the subscription served
	// sing-box JSON instead of share links.
	Outbound  string `json:"-" gorm:"type:text;not null;default:''"`
	SortOrder int    `json:"sort_order" gorm:"not null;default:0"`
	// Last latency test: TCPing to the server and a real request through it.
	TcpMs     int64  `json:"tcp_ms" gorm:"not null;default:0"`
	DelayMs   int64  `json:"delay_ms" gorm:"not null;default:0"`
	TestedAt  int64  `json:"tested_at" gorm:"not null;default:0"`
	TestError string `json:"test_error" gorm:"size:255;not null;default:''"`
}
