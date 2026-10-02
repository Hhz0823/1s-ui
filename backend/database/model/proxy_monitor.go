package model

// ProxyMonitor is a SOCKS5 or HTTP proxy, or a node (Type "node": VLESS,
// VMess, Trojan, Shadowsocks, Hysteria2 … from its share link), that the
// panel checks on a schedule. The check runs on the panel host (ServerId 0)
// or on a managed server, so a proxy that only accepts its relay server's IP,
// or a node as seen from a home network, can be watched from there.
type ProxyMonitor struct {
	Id        uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string `json:"name" gorm:"size:80;not null"`
	Type      string `json:"type" gorm:"size:16;not null"`
	Host      string `json:"host" gorm:"size:255;not null"`
	Port      int    `json:"port" gorm:"not null"`
	Username  string `json:"username" gorm:"size:255;not null;default:''"`
	Password  string `json:"-" gorm:"size:255;not null;default:''"`
	Target    string `json:"target" gorm:"size:2048;not null;default:''"`
	ServerId  uint   `json:"server_id" gorm:"not null;default:0"`
	Interval  int    `json:"interval" gorm:"not null;default:60"`
	Enabled   bool   `json:"enabled" gorm:"not null;default:true"`
	SortOrder int    `json:"sort_order" gorm:"not null;default:0"`
	CreatedAt int64  `json:"created_at" gorm:"not null;default:0"`
	// Link is a node's share link. It carries the node's credentials, so it
	// is only sent to the server running the check, never to clients.
	Link     string `json:"-" gorm:"type:text;not null;default:''"`
	Protocol string `json:"protocol" gorm:"size:32;not null;default:''"`
	// A node picked from a server's inbounds keeps its source, so its link
	// follows changes made on that server (NodeServerId 0 is this panel).
	NodeServerId  uint  `json:"node_server_id" gorm:"not null;default:0"`
	NodeInboundId uint  `json:"node_inbound_id" gorm:"not null;default:0"`
	LinkUpdatedAt int64 `json:"-" gorm:"not null;default:0"`
}

// ProxyMonitorResult is one check of a ProxyMonitor.
type ProxyMonitorResult struct {
	Id          uint   `gorm:"primaryKey;autoIncrement"`
	MonitorId   uint   `gorm:"index:idx_proxy_monitor_result,priority:1;not null"`
	Time        int64  `gorm:"index:idx_proxy_monitor_result,priority:2;not null"`
	OK          bool   `gorm:"not null;default:false"`
	LatencyMs   int64  `gorm:"not null;default:0"`
	ConnectMs   int64  `gorm:"not null;default:0"`
	HandshakeMs int64  `gorm:"not null;default:0"`
	TLSMs       int64  `gorm:"not null;default:0"`
	TTFBMs      int64  `gorm:"not null;default:0"`
	Status      int    `gorm:"not null;default:0"`
	ExitIP      string `gorm:"size:64;not null;default:''"`
	Country     string `gorm:"size:8;not null;default:''"`
	Stage       string `gorm:"size:16;not null;default:''"`
	Error       string `gorm:"size:255;not null;default:''"`
}
