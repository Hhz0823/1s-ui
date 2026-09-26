package util

import (
	"encoding/json"

	"github.com/Hhz0823/1s-ui/util/common"

	"github.com/Hhz0823/1s-ui/database/model"
)

// Fill Inbound's out_json
func FillOutJson(i *model.Inbound, hostname string) error {
	switch i.Type {
	case "direct", "tun", "redirect", "tproxy", "dokodemo-door":
		return nil
	}
	var outJson map[string]interface{}
	err := json.Unmarshal(i.OutJson, &outJson)
	if err != nil {
		return err
	}

	if outJson == nil {
		outJson = make(map[string]interface{})
	}

	if i.TlsId > 0 {
		addTls(&outJson, i.Tls)
	} else {
		delete(outJson, "tls")
	}

	inbound, err := i.MarshalFull()
	if err != nil {
		return err
	}

	outJson["type"] = i.Type
	outJson["tag"] = i.Tag
	outJson["server"] = hostname
	outJson["server_port"] = (*inbound)["listen_port"]
	if i.Type == "naive" {
		var addrs []map[string]interface{}
		if json.Unmarshal(i.Addrs, &addrs) == nil && len(addrs) > 0 {
			if server, ok := addrs[0]["server"].(string); ok && server != "" {
				outJson["server"] = server
			}
			if port, ok := addrs[0]["server_port"]; ok {
				outJson["server_port"] = port
			}
		}
	}

	switch i.Type {
	case "http", "socks", "mixed", "anytls":
	case "naive":
		naiveOut(&outJson, *inbound)
	case "shadowsocks":
		shadowsocksOut(&outJson, *inbound)
	case "shadowtls":
		shadowTlsOut(&outJson, *inbound)
	case "hysteria":
		hysteriaOut(&outJson, *inbound)
	case "hysteria2":
		hysteria2Out(&outJson, *inbound)
	case "tuic":
		tuicOut(&outJson, *inbound)
	case "vless":
		vlessOut(&outJson, *inbound)
	case "trojan":
		trojanOut(&outJson, *inbound)
	case "vmess":
		vmessOut(&outJson, *inbound)
	default:
		for key := range outJson {
			delete(outJson, key)
		}
	}

	i.OutJson, err = json.MarshalIndent(outJson, "", "  ")
	if err != nil {
		return err
	}

	return nil
}

// addTls function
func addTls(out *map[string]interface{}, tls *model.Tls) {
	if tls == nil {
		return
	}
	var tlsServer, tlsConfig map[string]interface{}
	err := json.Unmarshal(tls.Server, &tlsServer)
	if err != nil {
		return
	}
	if len(tls.Client) > 0 {
		if err = json.Unmarshal(tls.Client, &tlsConfig); err != nil {
			return
		}
	}
	if tlsConfig == nil {
		tlsConfig = map[string]interface{}{}
	}

	if enabled, ok := tlsServer["enabled"]; ok {
		tlsConfig["enabled"] = enabled
	}
	if serverName, ok := tlsServer["server_name"]; ok {
		tlsConfig["server_name"] = serverName
	}
	if alpn, ok := tlsServer["alpn"]; ok {
		tlsConfig["alpn"] = alpn
	}
	if minVersion, ok := tlsServer["min_version"]; ok {
		tlsConfig["min_version"] = minVersion
	}
	if maxVersion, ok := tlsServer["max_version"]; ok {
		tlsConfig["max_version"] = maxVersion
	}
	if certificate, ok := tlsServer["certificate"]; ok {
		tlsConfig["certificate"] = certificate
	}
	if cipherSuites, ok := tlsServer["cipher_suites"]; ok {
		tlsConfig["cipher_suites"] = cipherSuites
	}
	if reality, ok := tlsServer["reality"].(map[string]interface{}); ok {
		if enabled, _ := reality["enabled"].(bool); enabled {
			realityConfig, _ := tlsConfig["reality"].(map[string]interface{})
			if realityConfig == nil {
				realityConfig = map[string]interface{}{}
			}
			realityConfig["enabled"] = true
			if shortIDs, ok := reality["short_id"].([]interface{}); ok && len(shortIDs) > 0 {
				realityConfig["short_id"] = shortIDs[common.RandomInt(len(shortIDs))]
			}
			tlsConfig["reality"] = realityConfig
		}
	}
	if ech, ok := tlsServer["ech"].(map[string]interface{}); ok {
		if enabled, _ := ech["enabled"].(bool); enabled {
			echConfig, _ := tlsConfig["ech"].(map[string]interface{})
			if echConfig == nil {
				echConfig = map[string]interface{}{}
			}
			echConfig["enabled"] = true
			if value, exists := ech["pq_signature_schemes_enabled"]; exists {
				echConfig["pq_signature_schemes_enabled"] = value
			}
			if value, exists := ech["dynamic_record_sizing_disabled"]; exists {
				echConfig["dynamic_record_sizing_disabled"] = value
			}
			tlsConfig["ech"] = echConfig
		}
	}

	(*out)["tls"] = tlsConfig
}

func naiveOut(out *map[string]interface{}, inbound map[string]interface{}) {
	if tlsOptions, ok := (*out)["tls"].(map[string]interface{}); ok {
		for key := range tlsOptions {
			switch key {
			case "enabled", "server_name", "certificate", "certificate_path", "ech":
			default:
				delete(tlsOptions, key)
			}
		}
	}
	if network, ok := inbound["network"].(string); ok {
		switch network {
		case "udp":
			(*out)["quic"] = true
		case "tcp":
			(*out)["quic"] = false
		}
	}
	if quic_congestion_control, ok := inbound["quic_congestion_control"].(string); ok {
		(*out)["quic"] = true
		switch quic_congestion_control {
		case "bbr_standard":
			(*out)["quic_congestion_control"] = "bbr"
		case "bbr2_variant":
			(*out)["quic_congestion_control"] = "bbr2"
		default:
			(*out)["quic_congestion_control"] = quic_congestion_control
		}
	}

}

func shadowsocksOut(out *map[string]interface{}, inbound map[string]interface{}) {
	if method, ok := inbound["method"].(string); ok {
		(*out)["method"] = method
	}
}

func shadowTlsOut(out *map[string]interface{}, inbound map[string]interface{}) {
	if version, ok := inbound["version"].(float64); ok && int(version) == 3 {
		(*out)["version"] = 3
	} else {
		for key := range *out {
			delete(*out, key)
		}
	}
	(*out)["tls"] = map[string]interface{}{"enabled": true}
}

func hysteriaOut(out *map[string]interface{}, inbound map[string]interface{}) {
	delete(*out, "down_mbps")
	delete(*out, "up_mbps")
	delete(*out, "obfs")
	delete(*out, "recv_window_conn")
	delete(*out, "disable_mtu_discovery")

	if upMbps, ok := inbound["down_mbps"]; ok {
		(*out)["up_mbps"] = upMbps
	}
	if downMbps, ok := inbound["up_mbps"]; ok {
		(*out)["down_mbps"] = downMbps
	}
	if obfs, ok := inbound["obfs"]; ok {
		(*out)["obfs"] = obfs
	}
	if recvWindow, ok := inbound["recv_window_conn"]; ok {
		(*out)["recv_window_conn"] = recvWindow
	}
	if disableMTU, ok := inbound["disable_mtu_discovery"]; ok {
		(*out)["disable_mtu_discovery"] = disableMTU
	}
}

func hysteria2Out(out *map[string]interface{}, inbound map[string]interface{}) {
	delete(*out, "down_mbps")
	delete(*out, "up_mbps")
	delete(*out, "obfs")

	if upMbps, ok := inbound["down_mbps"]; ok {
		(*out)["up_mbps"] = upMbps
	}
	if downMbps, ok := inbound["up_mbps"]; ok {
		(*out)["down_mbps"] = downMbps
	}
	if obfs, ok := inbound["obfs"]; ok {
		(*out)["obfs"] = obfs
	}
}

func tuicOut(out *map[string]interface{}, inbound map[string]interface{}) {
	delete(*out, "zero_rtt_handshake")
	delete(*out, "heartbeat")
	if congestionControl, ok := inbound["congestion_control"].(string); ok {
		(*out)["congestion_control"] = congestionControl
	} else {
		(*out)["congestion_control"] = "cubic"
	}
	if zeroRTT, ok := inbound["zero_rtt_handshake"].(bool); ok {
		(*out)["zero_rtt_handshake"] = zeroRTT
	}
	if heartbeat, ok := inbound["heartbeat"]; ok {
		(*out)["heartbeat"] = heartbeat
	}
}

func vlessOut(out *map[string]interface{}, inbound map[string]interface{}) {
	delete(*out, "transport")
	if transport, ok := inbound["transport"]; ok {
		(*out)["transport"] = transport
	}
}

func trojanOut(out *map[string]interface{}, inbound map[string]interface{}) {
	delete(*out, "transport")
	if transport, ok := inbound["transport"]; ok {
		(*out)["transport"] = transport
	}
}

func vmessOut(out *map[string]interface{}, inbound map[string]interface{}) {
	(*out)["alter_id"] = 0
	delete(*out, "transport")
	if transport, ok := inbound["transport"]; ok {
		(*out)["transport"] = transport
	}
}

// NormalizeSingBoxOutbound removes panel-only or client-specific fields that
// sing-box rejects with "unknown field", so imported links, stored outbounds
// and JSON subscriptions stay loadable:
//   - certificate SHA-256 pins (pinned_peer_certificate_sha256 / pinSHA256)
//     are share-link metadata; sing-box only understands SPKI pins. When the
//     certificate itself is not embedded, verification falls back to insecure,
//     matching how v2rayN runs such nodes on its sing-box core.
//   - hysteria2 "fastopen" comes from share links and is not an outbound option.
//   - XTLS Vision only works on raw TCP with TLS/Reality.
func NormalizeSingBoxOutbound(outbound map[string]interface{}) {
	if outbound == nil {
		return
	}
	outboundType, _ := outbound["type"].(string)
	if tlsConfig, ok := outbound["tls"].(map[string]interface{}); ok {
		pinned := false
		for _, key := range []string{"pinned_peer_certificate_sha256", "pinSHA256", "pcs"} {
			if value, exists := tlsConfig[key]; exists {
				if len(stringList(value)) > 0 {
					pinned = true
				}
				delete(tlsConfig, key)
			}
		}
		if pinned && CertPEMFromTLS(tlsConfig) == "" && len(stringList(tlsConfig["certificate_public_key_sha256"])) == 0 {
			tlsConfig["insecure"] = true
		}
		if outboundType == "naive" {
			// sing-box refuses insecure Naive outbounds; the embedded certificate
			// is the only supported way to trust a self-signed server.
			delete(tlsConfig, "insecure")
		}
	}
	switch outboundType {
	case "hysteria2", "hysteria", "tuic":
		delete(outbound, "fastopen")
	case "vless":
		if flow, _ := outbound["flow"].(string); flow != "" && !vlessVisionAllowed(outbound) {
			delete(outbound, "flow")
		}
	}
	if transport, ok := outbound["transport"].(map[string]interface{}); ok {
		if transportType, _ := transport["type"].(string); transportType == "" {
			delete(outbound, "transport")
		}
	}
}

func vlessVisionAllowed(outbound map[string]interface{}) bool {
	tlsConfig, _ := outbound["tls"].(map[string]interface{})
	if enabled, _ := tlsConfig["enabled"].(bool); !enabled {
		return false
	}
	if transport, ok := outbound["transport"].(map[string]interface{}); ok {
		transportType, _ := transport["type"].(string)
		return transportType == "" || transportType == "tcp"
	}
	return true
}

// NormalizeSingBoxOutboundJSON applies NormalizeSingBoxOutbound to a raw JSON
// object and returns the original bytes when nothing had to change.
func NormalizeSingBoxOutboundJSON(raw []byte) ([]byte, error) {
	var outbound map[string]interface{}
	if err := json.Unmarshal(raw, &outbound); err != nil {
		return nil, err
	}
	before, err := json.Marshal(outbound)
	if err != nil {
		return nil, err
	}
	NormalizeSingBoxOutbound(outbound)
	after, err := json.Marshal(outbound)
	if err != nil {
		return nil, err
	}
	if string(before) == string(after) {
		return raw, nil
	}
	return after, nil
}
