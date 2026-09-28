package util

import (
	"encoding/json"
	"strings"
)

// xhttpCDNExtra is the XHTTP "extra" a share link carries for a node whose
// downloads go through a CDN (the inbound's "cdn" option, see
// service/xhttp_cdn.go): downloadSettings for the CDN domain, which v2rayN,
// v2rayNG, PassWall / PassWall 2 and Anywhere apply. "" for other nodes.
func xhttpCDNExtra(inbound map[string]interface{}) string {
	cdn, _ := inbound["cdn"].(map[string]interface{})
	domain, _ := cdn["domain"].(string)
	port := naivePositiveInt(cdn["port"])
	transport, _ := inbound["transport"].(map[string]interface{})
	if transportType, _ := transport["type"].(string); domain == "" || port == 0 || transportType != "xhttp" {
		return ""
	}
	path, _ := transport["path"].(string)
	if path == "" {
		path = "/"
	}
	domain = strings.ToLower(domain)
	extra, err := json.Marshal(map[string]interface{}{"downloadSettings": map[string]interface{}{
		"address": domain, "port": port, "network": "xhttp", "security": "tls",
		"tlsSettings":   map[string]interface{}{"serverName": domain, "fingerprint": "chrome"},
		"xhttpSettings": map[string]interface{}{"path": path, "host": domain},
	}})
	if err != nil {
		return ""
	}
	return string(extra)
}
