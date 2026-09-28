//go:build !openwrt_lite

package service

import (
	"fmt"

	"github.com/Hhz0823/1s-ui/util/common"
)

// xhttpCDNSocket is where a CDN node's XHTTP inbound listens.
func xhttpCDNSocket(inboundID uint) string {
	return fmt.Sprintf("@1s-ui-xhttp-%d", inboundID)
}

// splitXHTTPForCDN turns the built inbound of a CDN node into the XHTTP
// inbound and the entries that fall back to it.
func splitXHTTPForCDN(inboundID uint, config map[string]interface{}, cdn *xhttpCDN) ([]map[string]interface{}, error) {
	stream, _ := config["streamSettings"].(map[string]interface{})
	if stringValue(stream["network"], "") != "xhttp" {
		return nil, common.NewError("downlink through a CDN needs the XHTTP transport")
	}
	tag, _ := config["tag"].(string)
	security := stringValue(stream["security"], "none")
	xhttpSettings := map[string]interface{}{}
	if settings, ok := stream["xhttpSettings"].(map[string]interface{}); ok {
		for key, value := range settings {
			xhttpSettings[key] = value
		}
	}
	// Downloads arrive with the CDN domain as Host.
	delete(xhttpSettings, "host")
	xhttpStream := map[string]interface{}{"network": "xhttp", "xhttpSettings": xhttpSettings}
	if sockopt, ok := stream["sockopt"]; ok {
		xhttpStream["sockopt"] = sockopt
	}

	xhttp := map[string]interface{}{}
	for key, value := range config {
		xhttp[key] = value
	}
	xhttp["streamSettings"] = xhttpStream
	var configs []map[string]interface{}
	var dest interface{}
	switch security {
	case "none":
		// VLESS Encryption: the XHTTP inbound stays on the node's port.
		dest = toInt(config["port"])
	case "reality", "tls":
		socket := xhttpCDNSocket(inboundID)
		xhttp["listen"] = socket
		delete(xhttp, "port")
		dest = socket
		entryStream := map[string]interface{}{"network": "raw", "security": security}
		for _, key := range []string{"realitySettings", "tlsSettings", "sockopt"} {
			if value, ok := stream[key]; ok {
				entryStream[key] = value
			}
		}
		configs = append(configs, map[string]interface{}{
			"tag": tag + "-entry", "listen": config["listen"], "port": config["port"], "protocol": "vless",
			"settings":       map[string]interface{}{"clients": []interface{}{}, "decryption": "none", "fallbacks": []interface{}{map[string]interface{}{"dest": dest}}},
			"streamSettings": entryStream,
		})
	default:
		return nil, common.NewErrorf("downlink through a CDN does not support the %s security layer", security)
	}
	configs = append(configs, map[string]interface{}{
		"tag": tag + "-cdn", "listen": config["listen"], "port": cdn.Port, "protocol": "vless",
		"settings": map[string]interface{}{"clients": []interface{}{}, "decryption": "none", "fallbacks": []interface{}{map[string]interface{}{"dest": dest}}},
		"streamSettings": map[string]interface{}{"network": "raw", "security": "tls", "tlsSettings": map[string]interface{}{
			"alpn":         []string{"h2", "http/1.1"},
			"certificates": []interface{}{map[string]interface{}{"certificate": cdn.Certificate, "key": cdn.Key}},
		}},
	})
	// The XHTTP inbound comes last: it carries the node's tag and users.
	return append(configs, xhttp), nil
}
