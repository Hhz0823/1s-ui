package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util/common"
)

var InboundTypeWithLink = []string{"socks", "http", "mixed", "shadowsocks", "naive", "hysteria", "hysteria2", "anytls", "tuic", "vless", "trojan", "vmess"}

// Note: dokodemo-door and wireguard are operational inbounds without share-link clients.

type LinkParam struct {
	Key   string
	Value string
}

func removeLinkParam(params []LinkParam, key string) []LinkParam {
	filtered := params[:0]
	for _, param := range params {
		if param.Key != key {
			filtered = append(filtered, param)
		}
	}
	return filtered
}

func joinRemark(clientRemark, inboundRemark string) string {
	if clientRemark != "" {
		return clientRemark + "-" + inboundRemark
	}
	return inboundRemark
}

func firstUserConfig(userConfig map[string]map[string]interface{}, keys ...string) map[string]interface{} {
	for _, key := range keys {
		if cfg, ok := userConfig[key]; ok && cfg != nil {
			return cfg
		}
	}
	return nil
}

func LinkGenerator(clientConfig json.RawMessage, i *model.Inbound, hostname string, clientRemark string) []string {
	inbound, err := i.MarshalFull()
	if err != nil {
		return []string{}
	}

	var tls map[string]interface{}
	if i.TlsId > 0 {
		tls = prepareTls(i.Tls)
	}

	var userConfig map[string]map[string]interface{}
	if err := json.Unmarshal(clientConfig, &userConfig); err != nil {
		return []string{}
	}

	var Addrs []map[string]interface{}
	if err := json.Unmarshal(i.Addrs, &Addrs); err != nil {
		return []string{}
	}
	if len(Addrs) == 0 {
		Addrs = append(Addrs, map[string]interface{}{
			"server":      hostname,
			"server_port": (*inbound)["listen_port"],
			"remark":      joinRemark(clientRemark, i.Tag),
		})
		if i.TlsId > 0 {
			Addrs[0]["tls"] = tls
		}
	} else {
		for index, addr := range Addrs {
			addrRemark, _ := addr["remark"].(string)
			Addrs[index]["remark"] = joinRemark(clientRemark, i.Tag+addrRemark)
			if i.TlsId > 0 {
				newTls := map[string]interface{}{}
				for k, v := range tls {
					newTls[k] = v
				}

				// Override tls
				if addrTls, ok := addr["tls"].(map[string]interface{}); ok {
					for k, v := range addrTls {
						newTls[k] = v
					}
				}
				Addrs[index]["tls"] = newTls
			}
		}
	}

	if i.RuntimeCore() == model.CoreTypeXray {
		switch i.Type {
		case "socks":
			return socksLink(firstUserConfig(userConfig, "socks", "mixed"), Addrs)
		case "http":
			return httpLink(firstUserConfig(userConfig, "http", "mixed"), Addrs)
		case "mixed":
			return append(
				socksLink(firstUserConfig(userConfig, "socks", "mixed"), Addrs),
				httpLink(firstUserConfig(userConfig, "http", "mixed"), Addrs)...,
			)
		case "shadowsocks":
			return shadowsocksLink(userConfig, *inbound, Addrs)
		case "vless":
			return xrayVlessLink(userConfig["vless"], *inbound, Addrs)
		case "vmess":
			return xrayVmessLink(userConfig["vmess"], *inbound, Addrs)
		case "trojan":
			return xrayTrojanLink(userConfig["trojan"], *inbound, Addrs)
		case "hysteria2":
			return hysteria2Link(userConfig["hysteria2"], *inbound, Addrs)
		}
		return []string{}
	}

	switch i.Type {
	case "socks":
		return socksLink(userConfig["socks"], Addrs)
	case "http":
		return httpLink(userConfig["http"], Addrs)
	case "mixed":
		return append(
			socksLink(userConfig["socks"], Addrs),
			httpLink(userConfig["http"], Addrs)...,
		)
	case "shadowsocks":
		return shadowsocksLink(userConfig, *inbound, Addrs)
	case "naive":
		return naiveLink(userConfig["naive"], *inbound, Addrs)
	case "hysteria":
		return hysteriaLink(userConfig["hysteria"], *inbound, Addrs)
	case "hysteria2":
		return hysteria2Link(userConfig["hysteria2"], *inbound, Addrs)
	case "tuic":
		return tuicLink(userConfig["tuic"], *inbound, Addrs)
	case "vless":
		return vlessLink(userConfig["vless"], *inbound, Addrs)
	case "anytls":
		return anytlsLink(userConfig["anytls"], Addrs)
	case "trojan":
		return trojanLink(userConfig["trojan"], *inbound, Addrs)
	case "vmess":
		return vmessLink(userConfig["vmess"], *inbound, Addrs)
	}

	return []string{}
}

func prepareTls(t *model.Tls) map[string]interface{} {
	var iTls, oTls map[string]interface{}
	if err := json.Unmarshal(t.Client, &oTls); err != nil {
		return nil
	}
	if err := json.Unmarshal(t.Server, &iTls); err != nil {
		return nil
	}

	if oTls["certificate_public_key_sha256"] != nil {
		if pin := CertSha256Hex(CertPEMFromTLS(iTls)); pin != "" {
			oTls["pinSHA256"] = pin
		}
	}

	for k, v := range iTls {
		switch k {
		case "enabled", "server_name", "alpn":
			oTls[k] = v
		case "reality":
			reality := v.(map[string]interface{})
			clientReality := oTls["reality"].(map[string]interface{})
			clientReality["enabled"] = reality["enabled"]
			if shortIDs, hasSIds := reality["short_id"].([]interface{}); hasSIds && len(shortIDs) > 0 {
				clientReality["short_id"] = shortIDs[common.RandomInt(len(shortIDs))]
			}
			oTls["reality"] = clientReality
		}
	}
	return oTls
}

func socksLink(userConfig map[string]interface{}, addrs []map[string]interface{}) []string {
	var links []string
	for _, addr := range addrs {
		username, _ := userConfig["username"].(string)
		password, _ := userConfig["password"].(string)
		links = append(links, fmt.Sprintf(
			"socks5://%s:%s@%s",
			escapeLinkUserInfo(username),
			escapeLinkUserInfo(password),
			linkHostPort(addr),
		))
	}
	return links
}

func httpLink(userConfig map[string]interface{}, addrs []map[string]interface{}) []string {
	var links []string
	protocol := "http"
	for _, addr := range addrs {
		if addr["tls"] != nil {
			protocol = "https"
		}
		username, _ := userConfig["username"].(string)
		password, _ := userConfig["password"].(string)
		links = append(links, fmt.Sprintf(
			"%s://%s:%s@%s",
			protocol,
			escapeLinkUserInfo(username),
			escapeLinkUserInfo(password),
			linkHostPort(addr),
		))
	}
	return links
}

func shadowsocksLink(
	userConfig map[string]map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	var userPass []string
	method, _ := inbound["method"].(string)
	if strings.HasPrefix(method, "2022") {
		inbPass, _ := inbound["password"].(string)
		userPass = append(userPass, inbPass)
	}
	var pass string
	if method == "2022-blake3-aes-128-gcm" {
		pass, _ = userConfig["shadowsocks16"]["password"].(string)
	} else {
		pass, _ = userConfig["shadowsocks"]["password"].(string)
	}
	userPass = append(userPass, pass)

	uriBase := fmt.Sprintf("ss://%s", toBase64([]byte(fmt.Sprintf("%s:%s", method, strings.Join(userPass, ":")))))

	var links []string
	for _, addr := range addrs {
		port, _ := addr["server_port"].(float64)
		links = append(links, fmt.Sprintf("%s@%s:%.0f#%s", uriBase, addr["server"].(string), port, addr["remark"].(string)))
	}
	return links
}

func naiveLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	username, _ := userConfig["username"].(string)

	baseUri := "naive+https://"
	var links []string

	for _, addr := range addrs {
		var params []LinkParam
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			// sing-box rejects insecure on Naive outbounds. A trusted
			// certificate or a client that supports the exported pin is required.
			getTlsParams(&params, tls, "")
			for _, param := range params {
				if param.Key == "sni" {
					// peer keeps older 1S-UI importers compatible.
					params = append(params, LinkParam{"peer", param.Value})
					break
				}
			}
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params = append(params, LinkParam{"tfo", "1"})
		} else {
			params = append(params, LinkParam{"tfo", "0"})
		}

		uri := fmt.Sprintf(
			"%s%s:%s@%s",
			baseUri,
			escapeLinkUserInfo(username),
			escapeLinkUserInfo(password),
			linkHostPort(addr),
		)
		links = append(links, addRawParams(uri, params, addr["remark"].(string)))
	}
	return links
}

func hysteriaLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	baseUri := "hysteria://"
	var links []string

	for _, addr := range addrs {
		var params []LinkParam
		if upmbps, ok := inbound["up_mbps"].(float64); ok {
			params = append(params, LinkParam{"downmbps", fmt.Sprintf("%.0f", upmbps)})
		}
		if downmbps, ok := inbound["down_mbps"].(float64); ok {
			params = append(params, LinkParam{"upmbps", fmt.Sprintf("%.0f", downmbps)})
		}
		if auth, ok := userConfig["auth_str"].(string); ok {
			params = append(params, LinkParam{"auth", auth})
		}
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "insecure")
		}
		if obfs, ok := inbound["obfs"].(string); ok {
			params = append(params, LinkParam{"obfs", obfs})
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params = append(params, LinkParam{"fastopen", "1"})
		} else {
			params = append(params, LinkParam{"fastopen", "0"})
		}
		var outJson map[string]interface{}
		if err := json.Unmarshal(inbound["out_json"].(json.RawMessage), &outJson); err != nil {
			return []string{} // Handle error
		}
		if mport, ok := outJson["server_ports"].([]interface{}); ok {
			mportList := make([]string, len(mport))
			for i, v := range mport {
				mportList[i] = v.(string)
			}
			params = append(params, LinkParam{"mport", strings.Join(mportList, ",")})
		}

		port, _ := addr["server_port"].(float64)
		uri := fmt.Sprintf("%s%s:%.0f", baseUri, addr["server"].(string), port)
		links = append(links, addParams(uri, params, addr["remark"].(string)))
	}

	return links
}

func hysteria2Link(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	var links []string

	for _, addr := range addrs {
		var params []LinkParam
		if upmbps, ok := inbound["up_mbps"].(float64); ok {
			params = append(params, LinkParam{"downmbps", fmt.Sprintf("%.0f", upmbps)})
		}
		if downmbps, ok := inbound["down_mbps"].(float64); ok {
			params = append(params, LinkParam{"upmbps", fmt.Sprintf("%.0f", downmbps)})
		}
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "insecure")
			params = removeLinkParam(params, "pcs")
			if pinSHA256 := pinnedPeerCertSha256ForLink(getPinnedPeerCertSha256(tls)); pinSHA256 != "" {
				params = append(params, LinkParam{"pinSHA256", pinSHA256})
			} else if pinSHA256, ok := tls["pinSHA256"].(string); ok && pinSHA256 != "" {
				params = append(params, LinkParam{"pinSHA256", pinSHA256})
			}
		}
		if obfs, ok := inbound["obfs"].(map[string]interface{}); ok {
			if obfsType, ok := obfs["type"].(string); ok {
				params = append(params, LinkParam{"obfs", obfsType})
			}
			if obfsPassword, ok := obfs["password"].(string); ok {
				params = append(params, LinkParam{"obfs-password", obfsPassword})
			}
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params = append(params, LinkParam{"fastopen", "1"})
		} else {
			params = append(params, LinkParam{"fastopen", "0"})
		}
		var outJson map[string]interface{}
		if err := json.Unmarshal(inbound["out_json"].(json.RawMessage), &outJson); err != nil {
			return []string{} // Handle error
		}
		if mport, ok := outJson["server_ports"].([]interface{}); ok {
			mportList := make([]string, len(mport))
			for i, v := range mport {
				mportList[i] = v.(string)
			}
			params = append(params, LinkParam{"mport", strings.Join(mportList, ",")})
		}

		port, _ := addr["server_port"].(float64)
		server := strings.Trim(addr["server"].(string), "[]")
		uri := fmt.Sprintf(
			"hysteria2://%s@%s",
			escapeLinkUserInfo(password),
			net.JoinHostPort(server, fmt.Sprintf("%.0f", port)),
		)
		links = append(links, addRawParams(uri, params, addr["remark"].(string)))
	}

	return links
}

func anytlsLink(
	userConfig map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	var links []string

	for _, addr := range addrs {
		var params []LinkParam
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "insecure")
			ensurePinnedTLSClientCompatibility(&params)
		}

		uri := fmt.Sprintf("anytls://%s@%s", escapeLinkUserInfo(password), linkHostPort(addr))
		links = append(links, addRawParams(uri, params, addr["remark"].(string)))
	}

	return links
}

func tuicLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	uuid, _ := userConfig["uuid"].(string)
	var links []string

	for _, addr := range addrs {
		var params []LinkParam
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "insecure")
			ensurePinnedTLSClientCompatibility(&params)
			if !hasLinkParam(params, "alpn") {
				params = append(params, LinkParam{"alpn", "h3"})
			}
		}
		if congestionControl, ok := inbound["congestion_control"].(string); ok {
			params = append(params, LinkParam{"congestion_control", congestionControl})
		}

		uri := fmt.Sprintf(
			"tuic://%s:%s@%s",
			escapeLinkUserInfo(uuid),
			escapeLinkUserInfo(password),
			linkHostPort(addr),
		)
		links = append(links, addRawParams(uri, params, addr["remark"].(string)))
	}

	return links
}

func vlessLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	uuid, _ := userConfig["uuid"].(string)
	baseParams := getTransportParams(inbound["transport"])
	var links []string

	for _, addr := range addrs {
		params := make([]LinkParam, len(baseParams))
		copy(params, baseParams)
		if tls, ok := addr["tls"].(map[string]interface{}); ok && tls["enabled"].(bool) {
			getTlsParams(&params, tls, "allowInsecure")
			if flow, ok := userConfig["flow"].(string); ok {
				params = append(params, LinkParam{"flow", flow})
			}
		}
		port, _ := addr["server_port"].(float64)
		uri := fmt.Sprintf("vless://%s@%s:%.0f", uuid, addr["server"].(string), port)
		uri = addParams(uri, params, addr["remark"].(string))
		links = append(links, uri)
	}

	return links
}

func xrayVlessLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	uuid, _ := userConfig["uuid"].(string)
	baseParams := getXrayTransportParams(inbound["transport"])
	var links []string

	for _, addr := range addrs {
		params := make([]LinkParam, len(baseParams))
		copy(params, baseParams)
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			if enabled, _ := tls["enabled"].(bool); enabled {
				getXrayTlsParams(&params, tls, "allowInsecure")
				if isTcpTransport(params) {
					if flow, ok := userConfig["flow"].(string); ok {
						params = append(params, LinkParam{"flow", flow})
					}
				}
			}
		}
		port, _ := addr["server_port"].(float64)
		uri := fmt.Sprintf("vless://%s@%s:%.0f", uuid, addr["server"].(string), port)
		uri = addParams(uri, params, addr["remark"].(string))
		links = append(links, uri)
	}

	return links
}

func xrayTrojanLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	baseParams := getXrayTransportParams(inbound["transport"])
	var links []string

	for _, addr := range addrs {
		params := make([]LinkParam, len(baseParams))
		copy(params, baseParams)
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			if enabled, _ := tls["enabled"].(bool); enabled {
				getXrayTlsParams(&params, tls, "allowInsecure")
			}
		}
		uri := fmt.Sprintf("trojan://%s@%s", escapeLinkUserInfo(password), linkHostPort(addr))
		uri = addRawParams(uri, params, addr["remark"].(string))
		links = append(links, uri)
	}

	return links
}

func trojanLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {
	password, _ := userConfig["password"].(string)
	baseParams := getTransportParams(inbound["transport"])
	var links []string

	for _, addr := range addrs {
		params := make([]LinkParam, len(baseParams))
		copy(params, baseParams)
		if tls, ok := addr["tls"].(map[string]interface{}); ok && tls["enabled"].(bool) {
			getTlsParams(&params, tls, "allowInsecure")
		}
		uri := fmt.Sprintf("trojan://%s@%s", escapeLinkUserInfo(password), linkHostPort(addr))
		uri = addRawParams(uri, params, addr["remark"].(string))
		links = append(links, uri)
	}

	return links
}

func xrayVmessLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	uuid, _ := userConfig["uuid"].(string)
	transportParams := getXrayTransportParams(inbound["transport"])
	var links []string

	baseParams := map[string]interface{}{
		"v":   "2",
		"id":  uuid,
		"aid": 0,
	}

	var net, host, path, serviceName, mode string
	for _, p := range transportParams {
		switch p.Key {
		case "type":
			net = p.Value
		case "host":
			host = p.Value
		case "path":
			path = p.Value
		case "serviceName":
			serviceName = p.Value
		case "mode":
			mode = p.Value
		}
	}
	if net == "" || net == "raw" {
		net = "tcp"
	}
	baseParams["net"] = net
	if host != "" {
		baseParams["host"] = host
	}
	if path != "" {
		baseParams["path"] = path
	}
	if serviceName != "" {
		baseParams["path"] = serviceName
	}
	if mode != "" {
		baseParams["mode"] = mode
	}

	for _, addr := range addrs {
		obj := make(map[string]interface{})
		for k, v := range baseParams {
			obj[k] = v
		}

		obj["add"], _ = addr["server"].(string)
		port, _ := addr["server_port"].(float64)
		obj["port"] = fmt.Sprintf("%.0f", port)
		obj["ps"], _ = addr["remark"].(string)
		populateXrayVmessTlsParams(obj, addr["tls"])

		jsonStr, _ := json.Marshal(obj)

		uri := fmt.Sprintf("vmess://%s", toBase64(jsonStr))
		links = append(links, uri)
	}
	return links
}

func vmessLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	uuid, _ := userConfig["uuid"].(string)
	transportParams := getTransportParams(inbound["transport"])
	var links []string

	baseParams := map[string]interface{}{
		"v":   "2",
		"id":  uuid,
		"aid": 0,
	}

	var net, typ, host, path string
	for _, p := range transportParams {
		switch p.Key {
		case "type":
			net = p.Value
		case "headerType":
			typ = p.Value
		case "host":
			host = p.Value
		case "path":
			path = p.Value
		}
	}

	if net == "tcp" {
		baseParams["net"] = "tcp"
	} else {
		baseParams["net"] = net
	}

	for _, addr := range addrs {
		obj := make(map[string]interface{})
		for k, v := range baseParams {
			obj[k] = v
		}

		obj["add"], _ = addr["server"].(string)
		port, _ := addr["server_port"].(float64)
		obj["port"] = fmt.Sprintf("%.0f", port)
		obj["ps"], _ = addr["remark"].(string)
		if typ != "" {
			obj["type"] = typ
		}
		if host != "" {
			obj["host"] = host
		}
		if path != "" {
			obj["path"] = path
		}
		populateVmessTlsParams(obj, addr["tls"])

		jsonStr, _ := json.Marshal(obj)

		uri := fmt.Sprintf("vmess://%s", toBase64(jsonStr))
		links = append(links, uri)
	}
	return links
}

func populateVmessTlsParams(obj map[string]interface{}, tlsConfig interface{}) {
	if tlsMap, ok := tlsConfig.(map[string]interface{}); ok && tlsMap["enabled"].(bool) {
		obj["tls"] = "tls"
		var tlsParams []LinkParam
		getTlsParams(&tlsParams, tlsMap, "allowInsecure")
		for _, p := range tlsParams {
			switch p.Key {
			case "security":
				// ignore, as "tls" is already set
			case "allowInsecure":
				obj["allowInsecure"] = 1
			case "sni":
				obj["sni"] = p.Value
			case "fp":
				obj["fp"] = p.Value
			case "alpn":
				obj["alpn"] = p.Value
			case "pcs":
				obj["pcs"] = p.Value
			}
		}
	} else {
		obj["tls"] = "none"
	}
}

func populateXrayVmessTlsParams(obj map[string]interface{}, tlsConfig interface{}) {
	if tlsMap, ok := tlsConfig.(map[string]interface{}); ok && tlsMap["enabled"].(bool) {
		obj["tls"] = "tls"
		var tlsParams []LinkParam
		getXrayTlsParams(&tlsParams, tlsMap, "allowInsecure")
		for _, p := range tlsParams {
			switch p.Key {
			case "security":
			case "allowInsecure":
				obj["allowInsecure"] = 1
			case "sni":
				obj["sni"] = p.Value
			case "fp":
				obj["fp"] = p.Value
			case "alpn":
				obj["alpn"] = p.Value
			case "pcs":
				obj["pinSHA256"] = p.Value
				obj["pcs"] = p.Value
			}
		}
	} else {
		obj["tls"] = "none"
	}
}

func toBase64(d []byte) string {
	return base64.StdEncoding.EncodeToString(d)
}

func addParams(uri string, params []LinkParam, remark string) string {
	URL, _ := url.Parse(uri)
	URL.RawQuery = encodeLinkParams(params)
	URL.Fragment = remark
	return URL.String()
}

func addRawParams(uri string, params []LinkParam, remark string) string {
	if query := encodeLinkParams(params); query != "" {
		uri += "?" + query
	}
	if remark != "" {
		uri += "#" + url.PathEscape(remark)
	}
	return uri
}

func encodeLinkParams(params []LinkParam) string {
	var q []string
	for _, p := range params {
		switch p.Key {
		case "mport", "alpn":
			q = append(q, fmt.Sprintf("%s=%s", p.Key, p.Value))
		default:
			q = append(q, fmt.Sprintf("%s=%s", p.Key, url.QueryEscape(p.Value)))
		}
	}
	return strings.Join(q, "&")
}

func escapeLinkUserInfo(value string) string {
	// QueryEscape covers every base64 separator; userinfo requires %20 for spaces.
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

func linkHostPort(addr map[string]interface{}) string {
	server := strings.Trim(addr["server"].(string), "[]")
	port, _ := addr["server_port"].(float64)
	return net.JoinHostPort(server, fmt.Sprintf("%.0f", port))
}

func getTransportParams(t interface{}) []LinkParam {
	var params []LinkParam
	trasport, _ := t.(map[string]interface{})
	var transportType string
	if tt, ok := trasport["type"].(string); ok {
		transportType = tt
	} else {
		transportType = "tcp"
	}
	if transportType == "http" {
		params = append(params, LinkParam{"type", "tcp"})
		params = append(params, LinkParam{"headerType", "http"})
	} else {
		params = append(params, LinkParam{"type", transportType})
	}
	if transportType == "tcp" {
		return params
	}

	switch transportType {
	case "http":
		if host, ok := trasport["host"].([]interface{}); ok {
			var hosts []string
			for _, v := range host {
				hosts = append(hosts, v.(string))
			}
			params = append(params, LinkParam{"host", strings.Join(hosts, ",")})
		}
		if path, ok := trasport["path"].(string); ok {
			params = append(params, LinkParam{"path", path})
		}
	case "ws":
		if path, ok := trasport["path"].(string); ok {
			params = append(params, LinkParam{"path", path})
		}
		if headers, ok := trasport["headers"].(map[string]interface{}); ok {
			if host, ok := headers["Host"].(string); ok {
				params = append(params, LinkParam{"host", host})
			}
		}
	case "grpc":
		if serviceName, ok := trasport["service_name"].(string); ok {
			params = append(params, LinkParam{"serviceName", serviceName})
		}
	case "httpupgrade":
		if host, ok := trasport["host"].(string); ok {
			params = append(params, LinkParam{"host", host})
		}
		if path, ok := trasport["path"].(string); ok {
			params = append(params, LinkParam{"path", path})
		}
	}
	return params
}

func getXrayTransportParams(t interface{}) []LinkParam {
	var params []LinkParam
	transport, _ := t.(map[string]interface{})
	transportType := "xhttp"
	if tt, ok := transport["type"].(string); ok && tt != "" {
		transportType = tt
	}
	params = append(params, LinkParam{"type", transportType})
	if transportType == "tcp" || transportType == "raw" {
		return params
	}

	switch transportType {
	case "xhttp":
		if host, ok := transport["host"].(string); ok && host != "" {
			params = append(params, LinkParam{"host", host})
		}
		if path, ok := transport["path"].(string); ok && path != "" {
			params = append(params, LinkParam{"path", path})
		}
		if mode, ok := transport["mode"].(string); ok && mode != "" {
			params = append(params, LinkParam{"mode", mode})
		}
	case "ws", "httpupgrade":
		if host, ok := transport["host"].(string); ok && host != "" {
			params = append(params, LinkParam{"host", host})
		}
		if path, ok := transport["path"].(string); ok && path != "" {
			params = append(params, LinkParam{"path", path})
		}
	case "grpc":
		if serviceName, ok := transport["service_name"].(string); ok {
			params = append(params, LinkParam{"serviceName", serviceName})
		}
	case "kcp", "mkcp":
		params[0].Value = "kcp"
	case "hysteria":
		params[0].Value = "hysteria"
	}
	return params
}

func isTcpTransport(params []LinkParam) bool {
	for _, p := range params {
		if p.Key == "type" {
			return p.Value == "tcp" || p.Value == "raw"
		}
	}
	return true
}

func getTlsParams(params *[]LinkParam, tls map[string]interface{}, insecureKey string) {
	if reality, ok := tls["reality"].(map[string]interface{}); ok && reality["enabled"].(bool) {
		*params = append(*params, LinkParam{"security", "reality"})
		if pbk, ok := reality["public_key"].(string); ok {
			*params = append(*params, LinkParam{"pbk", pbk})
		}
		if sid, ok := reality["short_id"].(string); ok {
			*params = append(*params, LinkParam{"sid", sid})
		}
	} else {
		*params = append(*params, LinkParam{"security", "tls"})
		if insecure, ok := tls["insecure"].(bool); ok && insecure {
			*params = append(*params, LinkParam{insecureKey, "1"})
		}
		if disableSni, ok := tls["disable_sni"].(bool); ok && disableSni {
			*params = append(*params, LinkParam{"disable_sni", "1"})
		}
	}
	if utls, ok := tls["utls"].(map[string]interface{}); ok {
		if fingerprint, ok := utls["fingerprint"].(string); ok {
			*params = append(*params, LinkParam{"fp", fingerprint})
		}
	}
	if sni, ok := tls["server_name"].(string); ok {
		*params = append(*params, LinkParam{"sni", sni})
	}
	if alpn, ok := tls["alpn"].([]interface{}); ok {
		alpnList := make([]string, len(alpn))
		for i, v := range alpn {
			alpnList[i] = v.(string)
		}
		*params = append(*params, LinkParam{"alpn", strings.Join(alpnList, ",")})
	}
	if pcs := getPinnedPeerCertSha256(tls); pcs != "" {
		*params = append(*params, LinkParam{"pcs", pinnedPeerCertSha256ForLink(pcs)})
	}
}

func getXrayTlsParams(params *[]LinkParam, tls map[string]interface{}, insecureKey string) {
	getTlsParams(params, tls, insecureKey)
	if !hasLinkParam(*params, "pcs") {
		if pin, ok := tls["pinSHA256"].(string); ok {
			if normalized := xrayPinSHA256ForLink(pin); normalized != "" {
				*params = append(*params, LinkParam{"pcs", normalized})
			}
		}
	}
}

func hasLinkParam(params []LinkParam, key string) bool {
	for _, param := range params {
		if param.Key == key {
			return true
		}
	}
	return false
}

func ensurePinnedTLSClientCompatibility(params *[]LinkParam) {
	// v2rayN 7.23.x does not pass pcs to sing-box outbounds. Retain the pin for
	// capable clients and add the compatibility fallback for generated certs.
	if hasLinkParam(*params, "pcs") && !hasLinkParam(*params, "insecure") {
		*params = append(*params, LinkParam{"insecure", "1"})
	}
}

func xrayPinSHA256ForLink(value string) string {
	converted := pinnedPeerCertSha256ForLink(value)
	if _, normalized, ok := decodePinnedPeerCertSha256Hex(converted); ok {
		return normalized
	}
	return ""
}

func getPinnedPeerCertSha256(tls map[string]interface{}) string {
	switch values := tls["pinned_peer_certificate_sha256"].(type) {
	case []interface{}:
		for _, value := range values {
			if sha, ok := value.(string); ok && sha != "" {
				return sha
			}
		}
	case []string:
		for _, sha := range values {
			if sha != "" {
				return sha
			}
		}
	case string:
		return values
	}
	return ""
}
