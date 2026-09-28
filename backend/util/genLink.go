package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"sort"
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

const (
	v2rayNNaiveConfigType = 12
	v2rayNSingBoxCoreType = 24
	v2rayNConfigVersion   = 4
)

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
	if i.TlsId > 0 && i.Tls != nil {
		tls = prepareTls(i.Tls)
	}

	var userConfig map[string]map[string]interface{}
	if err := json.Unmarshal(clientConfig, &userConfig); err != nil {
		return []string{}
	}

	var Addrs []map[string]interface{}
	if len(i.Addrs) > 0 {
		if err := json.Unmarshal(i.Addrs, &Addrs); err != nil {
			return []string{}
		}
	}
	if len(Addrs) == 0 {
		Addrs = append(Addrs, map[string]interface{}{
			"server":      hostname,
			"server_port": (*inbound)["listen_port"],
			"remark":      joinRemark(clientRemark, i.Tag),
		})
		if tls != nil {
			Addrs[0]["tls"] = tls
		}
	} else {
		for index, addr := range Addrs {
			addrRemark, _ := addr["remark"].(string)
			Addrs[index]["remark"] = joinRemark(clientRemark, i.Tag+addrRemark)
			if _, hasServer := addr["server"].(string); !hasServer {
				Addrs[index]["server"] = hostname
			}
			if tls != nil {
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
		v2rayNLinks := naiveV2rayNLinks(userConfig["naive"], *inbound, Addrs)
		if naiveRequiresEmbeddedCertificate(Addrs) && len(v2rayNLinks) > 0 {
			return v2rayNLinks
		}
		return append(v2rayNLinks, naiveLink(userConfig["naive"], *inbound, Addrs)...)
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
	if certificate := CertPEMFromTLS(iTls); certificate != "" {
		// Public certificate only. v2rayN's Naive importer needs the PEM to
		// validate generated certificates without disabling TLS verification;
		// for a generated chain that is its CA, which outlives renewals.
		if root := PrivateRootPEM(certificate); root != "" {
			certificate = root
		}
		oTls["certificate"] = certificate
	}

	for k, v := range iTls {
		switch k {
		case "enabled", "server_name", "alpn":
			oTls[k] = v
		case "reality":
			reality, _ := v.(map[string]interface{})
			if reality == nil {
				continue
			}
			clientReality, _ := oTls["reality"].(map[string]interface{})
			if clientReality == nil {
				clientReality = map[string]interface{}{}
			}
			clientReality["enabled"] = reality["enabled"]
			if shortIDs, hasSIds := reality["short_id"].([]interface{}); hasSIds && len(shortIDs) > 0 {
				clientReality["short_id"] = shortIDs[common.RandomInt(len(shortIDs))]
			}
			oTls["reality"] = clientReality
		}
	}
	return oTls
}

// socksLink uses the v2rayN/v2rayNG share format: socks://BASE64URL(user:pass)@host:port.
// v2rayN does not recognise the socks5:// scheme at all, and Shadowrocket reads
// the same shape, so a single format keeps both clients importable.
func socksLink(userConfig map[string]interface{}, addrs []map[string]interface{}) []string {
	var links []string
	username, _ := userConfig["username"].(string)
	password, _ := userConfig["password"].(string)
	for _, addr := range addrs {
		userInfo := ""
		if username != "" || password != "" {
			userInfo = base64.RawURLEncoding.EncodeToString([]byte(username+":"+password)) + "@"
		}
		links = append(links, addRawParams("socks://"+userInfo+linkHostPort(addr), nil, linkRemark(addr)))
	}
	return links
}

func httpLink(userConfig map[string]interface{}, addrs []map[string]interface{}) []string {
	var links []string
	username, _ := userConfig["username"].(string)
	password, _ := userConfig["password"].(string)
	for _, addr := range addrs {
		protocol := "http"
		if tlsConfig, ok := addr["tls"].(map[string]interface{}); ok && tlsEnabled(tlsConfig) {
			protocol = "https"
		}
		userInfo := ""
		if username != "" || password != "" {
			userInfo = escapeLinkUserInfo(username) + ":" + escapeLinkUserInfo(password) + "@"
		}
		links = append(links, addRawParams(protocol+"://"+userInfo+linkHostPort(addr), nil, linkRemark(addr)))
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

	// SIP002 userinfo must be URL-safe base64. Standard base64 can contain "/",
	// which ends the authority and makes v2rayN/Shadowrocket reject the link.
	userInfo := base64.RawURLEncoding.EncodeToString([]byte(method + ":" + strings.Join(userPass, ":")))

	var links []string
	for _, addr := range addrs {
		links = append(links, addRawParams("ss://"+userInfo+"@"+linkHostPort(addr), nil, linkRemark(addr)))
	}
	return links
}

func linkRemark(addr map[string]interface{}) string {
	remark, _ := addr["remark"].(string)
	return remark
}

func tlsEnabled(tlsConfig map[string]interface{}) bool {
	if tlsConfig == nil {
		return false
	}
	enabled, ok := tlsConfig["enabled"].(bool)
	// Addresses may override only part of the TLS object; treat a TLS object
	// without an explicit flag as enabled, as the inbound owns the switch.
	return enabled || !ok
}

// tlsNeedsInsecureCompat reports whether a share link must carry the client
// "skip verification" flag. Clients such as Shadowrocket, and the sing-box core
// inside v2rayN, ignore certificate pins, so self-signed certificates would fail
// verification there. The pin is still exported for clients that enforce it.
func tlsNeedsInsecureCompat(tlsConfig map[string]interface{}) bool {
	if tlsConfig == nil {
		return false
	}
	if reality, ok := tlsConfig["reality"].(map[string]interface{}); ok {
		if enabled, _ := reality["enabled"].(bool); enabled {
			return false
		}
	}
	if insecure, _ := tlsConfig["insecure"].(bool); insecure {
		return true
	}
	if getPinnedPeerCertSha256(tlsConfig) != "" {
		return true
	}
	if pin, _ := tlsConfig["pinSHA256"].(string); strings.TrimSpace(pin) != "" {
		return true
	}
	return CertIsSelfSigned(CertPEMFromTLS(tlsConfig))
}

func decodeOutJSON(inbound map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	switch raw := inbound["out_json"].(type) {
	case json.RawMessage:
		_ = json.Unmarshal(raw, &result)
	case []byte:
		_ = json.Unmarshal(raw, &result)
	case string:
		_ = json.Unmarshal([]byte(raw), &result)
	case map[string]interface{}:
		return raw
	}
	if result == nil {
		result = map[string]interface{}{}
	}
	return result
}

func stringList(value interface{}) []string {
	switch values := value.(type) {
	case []interface{}:
		result := make([]string, 0, len(values))
		for _, item := range values {
			if text, ok := item.(string); ok && text != "" {
				result = append(result, text)
			}
		}
		return result
	case []string:
		return values
	case string:
		if values != "" {
			return []string{values}
		}
	}
	return nil
}

func naiveLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	username, _ := userConfig["username"].(string)

	outboundOptions := naiveOutboundOptions(inbound)
	schemes := []string{"naive+https://"}
	network, _ := inbound["network"].(string)
	if network == "udp" {
		schemes = []string{"naive+quic://"}
	} else if network == "" {
		if quic, _ := outboundOptions["quic"].(bool); quic {
			schemes = []string{"naive+quic://"}
		} else {
			schemes = []string{"naive+https://", "naive+quic://"}
		}
	}
	extraHeaders := naiveExtraHeadersForLink(outboundOptions["extra_headers"])
	var links []string

	for _, addr := range addrs {
		var params []LinkParam
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			// sing-box rejects insecure on Naive outbounds. A trusted
			// certificate or a client that supports the exported pin is required.
			getTlsParams(&params, tls, "insecure")
			for _, unsupported := range []string{"insecure", "alpn", "fp", "disable_sni"} {
				params = removeLinkParam(params, unsupported)
			}
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
		if extraHeaders != "" {
			params = append(params, LinkParam{"extra-headers", extraHeaders})
		}

		for _, scheme := range schemes {
			uri := fmt.Sprintf(
				"%s%s:%s@%s",
				scheme,
				escapeLinkUserInfo(username),
				escapeLinkUserInfo(password),
				linkHostPort(addr),
			)
			links = append(links, addRawParams(uri, params, addr["remark"].(string)))
		}
	}
	return links
}

func naiveOutboundOptions(inbound map[string]interface{}) map[string]interface{} {
	outboundOptions := map[string]interface{}{}
	switch raw := inbound["out_json"].(type) {
	case json.RawMessage:
		_ = json.Unmarshal(raw, &outboundOptions)
	case []byte:
		_ = json.Unmarshal(raw, &outboundOptions)
	case map[string]interface{}:
		outboundOptions = raw
	}
	return outboundOptions
}

func naiveV2rayNLinks(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	username, _ := userConfig["username"].(string)
	password, _ := userConfig["password"].(string)
	options := naiveOutboundOptions(inbound)

	quicModes := []bool{false}
	network, _ := inbound["network"].(string)
	switch network {
	case "udp":
		quicModes = []bool{true}
	case "":
		if quic, _ := options["quic"].(bool); quic {
			quicModes = []bool{true}
		} else {
			quicModes = []bool{false, true}
		}
	}

	uot := naiveUDPOverTCPEnabled(options["udp_over_tcp"])
	congestionControl := naiveCongestionControl(options, inbound)
	insecureConcurrency := naivePositiveInt(options["insecure_concurrency"])
	links := make([]string, 0, len(addrs)*len(quicModes))
	for _, addr := range addrs {
		for _, quic := range quicModes {
			profile := map[string]interface{}{
				"ConfigType":     v2rayNNaiveConfigType,
				"CoreType":       v2rayNSingBoxCoreType,
				"ConfigVersion":  v2rayNConfigVersion,
				"Address":        strings.Trim(addr["server"].(string), "[]"),
				"Port":           int(addr["server_port"].(float64)),
				"Username":       username,
				"Password":       password,
				"Remarks":        addr["remark"].(string),
				"StreamSecurity": "tls",
			}
			protocolExtra := map[string]interface{}{
				"Uot":       uot,
				"NaiveQuic": quic,
			}
			if quic {
				protocolExtra["CongestionControl"] = congestionControl
			}
			if insecureConcurrency > 0 {
				protocolExtra["InsecureConcurrency"] = insecureConcurrency
			}
			profile["ProtoExtraObj"] = protocolExtra

			if tls, ok := addr["tls"].(map[string]interface{}); ok {
				if sni, _ := tls["server_name"].(string); sni != "" {
					profile["Sni"] = sni
				} else {
					profile["Sni"] = profile["Address"]
				}
				if certificate := CertPEMFromTLS(tls); certificate != "" {
					profile["Cert"] = certificate
				}
			}

			encoded, err := json.Marshal(profile)
			if err != nil {
				continue
			}
			links = append(links, "v2rayn://naive/"+base64.RawURLEncoding.EncodeToString(encoded))
		}
	}
	return links
}

func naiveUDPOverTCPEnabled(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case map[string]interface{}:
		enabled, _ := typed["enabled"].(bool)
		return enabled
	default:
		return false
	}
}

func naiveRequiresEmbeddedCertificate(addrs []map[string]interface{}) bool {
	for _, addr := range addrs {
		tlsConfig, _ := addr["tls"].(map[string]interface{})
		if CertIsSelfSigned(CertPEMFromTLS(tlsConfig)) {
			return true
		}
	}
	return false
}

func naivePositiveInt(value interface{}) int {
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return typed
		}
	case float64:
		if typed > 0 {
			return int(typed)
		}
	}
	return 0
}

func naiveCongestionControl(options, inbound map[string]interface{}) string {
	value, _ := options["quic_congestion_control"].(string)
	if value == "" {
		value, _ = inbound["quic_congestion_control"].(string)
	}
	switch value {
	case "bbr", "bbr2", "cubic", "reno":
		return value
	default:
		return "bbr"
	}
}

func naiveExtraHeadersForLink(value interface{}) string {
	headers := map[string]string{}
	switch values := value.(type) {
	case map[string]interface{}:
		for name, rawValue := range values {
			if headerValue, ok := rawValue.(string); ok && validNaiveExtraHeader(name, headerValue) {
				headers[name] = headerValue
			}
		}
	case map[string]string:
		for name, headerValue := range values {
			if validNaiveExtraHeader(name, headerValue) {
				headers[name] = headerValue
			}
		}
	}
	if len(headers) == 0 {
		return ""
	}
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, name+": "+headers[name])
		if len(lines) == 32 {
			break
		}
	}
	return strings.Join(lines, "\r\n")
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
		if mport := serverPortsForLink(decodeOutJSON(inbound)); mport != "" {
			params = append(params, LinkParam{"mport", mport})
		}

		uri := baseUri + linkHostPort(addr)
		links = append(links, addParams(uri, params, linkRemark(addr)))
	}

	return links
}

// serverPortsForLink converts sing-box port hopping ranges (1000:2000) to the
// share-link form (1000-2000) understood by v2rayN, Shadowrocket and Hysteria.
func serverPortsForLink(outJson map[string]interface{}) string {
	ports := stringList(outJson["server_ports"])
	if len(ports) == 0 {
		return ""
	}
	converted := make([]string, 0, len(ports))
	for _, portRange := range ports {
		converted = append(converted, strings.ReplaceAll(strings.TrimSpace(portRange), ":", "-"))
	}
	return strings.Join(converted, ",")
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
		if mport := serverPortsForLink(decodeOutJSON(inbound)); mport != "" {
			params = append(params, LinkParam{"mport", mport})
		}

		uri := fmt.Sprintf("hysteria2://%s@%s", escapeLinkUserInfo(password), linkHostPort(addr))
		links = append(links, addRawParams(uri, params, linkRemark(addr)))
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
		}

		uri := fmt.Sprintf("anytls://%s@%s", escapeLinkUserInfo(password), linkHostPort(addr))
		links = append(links, addRawParams(uri, params, linkRemark(addr)))
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
			if !hasLinkParam(params, "alpn") {
				params = append(params, LinkParam{"alpn", "h3"})
			}
		}
		if congestionControl, ok := inbound["congestion_control"].(string); ok && congestionControl != "" {
			params = append(params, LinkParam{"congestion_control", congestionControl})
		}

		uri := fmt.Sprintf(
			"tuic://%s:%s@%s",
			escapeLinkUserInfo(uuid),
			escapeLinkUserInfo(password),
			linkHostPort(addr),
		)
		links = append(links, addRawParams(uri, params, linkRemark(addr)))
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
		params = append([]LinkParam{{"encryption", "none"}}, params...)
		if tls, ok := addr["tls"].(map[string]interface{}); ok && tlsEnabled(tls) {
			params = adjustHTTPTransportParams(params, true)
			getTlsParams(&params, tls, "allowInsecure")
			// XTLS Vision only works on raw TCP. Exporting it for WS/gRPC makes
			// clients refuse the node or the server report a flow mismatch.
			if flow, ok := userConfig["flow"].(string); ok && flow != "" && isTcpTransport(params) {
				params = append(params, LinkParam{"flow", flow})
			}
		}
		uri := fmt.Sprintf("vless://%s@%s", uuid, linkHostPort(addr))
		uri = addParams(uri, params, linkRemark(addr))
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
	encryption := xrayVlessClientEncryption(inbound)
	var links []string

	for _, addr := range addrs {
		params := make([]LinkParam, len(baseParams))
		copy(params, baseParams)
		params = append([]LinkParam{{"encryption", encryption}}, params...)
		tlsOn := false
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			if enabled, _ := tls["enabled"].(bool); enabled {
				getXrayTlsParams(&params, tls, "allowInsecure")
				tlsOn = true
			}
		}
		if !hasLinkParam(params, "security") {
			params = append(params, LinkParam{"security", "none"})
		}
		if flow, ok := userConfig["flow"].(string); ok && flow != "" &&
			XrayVlessVisionAllowed(linkTransportType(params), encryption != "none", tlsOn) {
			params = append(params, LinkParam{"flow", flow})
		}
		if extra := xhttpCDNExtra(inbound); extra != "" {
			params = append(params, LinkParam{"extra", extra})
		}
		uri := fmt.Sprintf("vless://%s@%s", uuid, linkHostPort(addr))
		uri = addParams(uri, params, linkRemark(addr))
		links = append(links, uri)
	}

	return links
}

// xrayVlessClientEncryption is the client half of VLESS Encryption kept with
// an inbound whose decryption is enabled, or "none".
func xrayVlessClientEncryption(inbound map[string]interface{}) string {
	decryption, _ := inbound["decryption"].(string)
	encryption, _ := inbound["encryption"].(string)
	if decryption == "" || decryption == "none" || encryption == "" {
		return "none"
	}
	return encryption
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
		if !hasLinkParam(params, "security") {
			params = append(params, LinkParam{"security", "none"})
		}
		uri := fmt.Sprintf("trojan://%s@%s", escapeLinkUserInfo(password), linkHostPort(addr))
		uri = addRawParams(uri, params, linkRemark(addr))
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
		if tls, ok := addr["tls"].(map[string]interface{}); ok && tlsEnabled(tls) {
			params = adjustHTTPTransportParams(params, true)
			getTlsParams(&params, tls, "allowInsecure")
		} else {
			// Trojan links default to TLS in every client; say so explicitly.
			params = append(params, LinkParam{"security", "none"})
		}
		uri := fmt.Sprintf("trojan://%s@%s", escapeLinkUserInfo(password), linkHostPort(addr))
		uri = addRawParams(uri, params, linkRemark(addr))
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
		"v":    "2",
		"id":   uuid,
		"aid":  "0",
		"scy":  "auto",
		"type": "none",
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

		obj["add"] = strings.Trim(linkServer(addr), "[]")
		obj["port"] = linkPort(addr)
		obj["ps"] = linkRemark(addr)
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

	for _, addr := range addrs {
		tls, _ := addr["tls"].(map[string]interface{})
		params := adjustHTTPTransportParams(append([]LinkParam(nil), transportParams...), tls != nil && tlsEnabled(tls))
		obj := map[string]interface{}{
			"v":    "2",
			"id":   uuid,
			"aid":  "0",
			"scy":  "auto",
			"type": "none",
		}
		for _, p := range params {
			switch p.Key {
			case "type":
				switch p.Value {
				case "http":
					obj["net"] = "h2"
				default:
					obj["net"] = p.Value
				}
			case "headerType":
				obj["type"] = p.Value
			case "host":
				obj["host"] = p.Value
			case "path":
				obj["path"] = p.Value
			case "serviceName":
				obj["path"] = p.Value
			}
		}

		obj["add"] = strings.Trim(linkServer(addr), "[]")
		obj["port"] = linkPort(addr)
		obj["ps"] = linkRemark(addr)
		populateVmessTlsParams(obj, addr["tls"])

		jsonStr, _ := json.Marshal(obj)

		uri := fmt.Sprintf("vmess://%s", toBase64(jsonStr))
		links = append(links, uri)
	}
	return links
}

func populateVmessTlsParams(obj map[string]interface{}, tlsConfig interface{}) {
	if tlsMap, ok := tlsConfig.(map[string]interface{}); ok && tlsEnabled(tlsMap) {
		obj["tls"] = "tls"
		var tlsParams []LinkParam
		getTlsParams(&tlsParams, tlsMap, "allowInsecure")
		populateVmessTLSObject(obj, tlsParams)
	} else {
		obj["tls"] = "none"
	}
}

func populateXrayVmessTlsParams(obj map[string]interface{}, tlsConfig interface{}) {
	if tlsMap, ok := tlsConfig.(map[string]interface{}); ok && tlsEnabled(tlsMap) {
		obj["tls"] = "tls"
		var tlsParams []LinkParam
		getXrayTlsParams(&tlsParams, tlsMap, "allowInsecure")
		populateVmessTLSObject(obj, tlsParams)
		if pcs, ok := obj["pcs"].(string); ok && pcs != "" {
			obj["pinSHA256"] = pcs
		}
	} else {
		obj["tls"] = "none"
	}
}

func populateVmessTLSObject(obj map[string]interface{}, tlsParams []LinkParam) {
	for _, p := range tlsParams {
		switch p.Key {
		case "security":
			// "tls" is already set
		case "allowInsecure":
			// Same shape as 3x-ui/v2rayN exports: a JSON boolean.
			obj["allowInsecure"] = true
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
}

func linkServer(addr map[string]interface{}) string {
	server, _ := addr["server"].(string)
	return server
}

func linkPort(addr map[string]interface{}) string {
	switch port := addr["server_port"].(type) {
	case float64:
		return fmt.Sprintf("%.0f", port)
	case int:
		return fmt.Sprintf("%d", port)
	case uint:
		return fmt.Sprintf("%d", port)
	case int64:
		return fmt.Sprintf("%d", port)
	case uint64:
		return fmt.Sprintf("%d", port)
	case json.Number:
		return port.String()
	case string:
		return port
	}
	return "0"
}

func toBase64(d []byte) string {
	return base64.StdEncoding.EncodeToString(d)
}

func addParams(uri string, params []LinkParam, remark string) string {
	URL, err := url.Parse(uri)
	if err != nil {
		return addRawParams(uri, params, remark)
	}
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
	return net.JoinHostPort(strings.Trim(linkServer(addr), "[]"), linkPort(addr))
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
			if host := firstHeaderValue(headers, "Host"); host != "" {
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
	transport := linkTransportType(params)
	return transport == "tcp" || transport == "raw"
}

// linkTransportType is the link's "type" parameter; links without one are
// raw TCP.
func linkTransportType(params []LinkParam) string {
	for _, p := range params {
		if p.Key == "type" {
			return p.Value
		}
	}
	return "tcp"
}

// XrayVlessVisionAllowed reports whether a user's XTLS Vision flow is kept on
// an Xray VLESS node, on the server and in its links alike. Xray-core runs
// Vision directly on raw TCP under TLS or REALITY, and on any transport
// underneath VLESS Encryption; elsewhere it fails with "XTLS only supports
// TLS and REALITY directly". Off raw TCP the panel keeps Vision only when
// VLESS Encryption comes with TLS or REALITY, as on the REALITY + XHTTP +
// Vision node: XHTTP nodes with VLESS Encryption alone never exported it, so
// the default Vision flow of their users must not start requiring it.
func XrayVlessVisionAllowed(transport string, vlessEncryption, tls bool) bool {
	switch transport {
	case "", "tcp", "raw":
		return vlessEncryption || tls
	}
	return vlessEncryption && tls
}

func getTlsParams(params *[]LinkParam, tls map[string]interface{}, insecureKey string) {
	reality, _ := tls["reality"].(map[string]interface{})
	if realityEnabled, _ := reality["enabled"].(bool); realityEnabled {
		*params = append(*params, LinkParam{"security", "reality"})
		if pbk, ok := reality["public_key"].(string); ok {
			*params = append(*params, LinkParam{"pbk", pbk})
		}
		if sid, ok := reality["short_id"].(string); ok {
			*params = append(*params, LinkParam{"sid", sid})
		}
	} else {
		*params = append(*params, LinkParam{"security", "tls"})
		if tlsNeedsInsecureCompat(tls) {
			*params = append(*params, LinkParam{insecureKey, "1"})
		}
		if disableSni, ok := tls["disable_sni"].(bool); ok && disableSni {
			*params = append(*params, LinkParam{"disable_sni", "1"})
		}
	}
	if utls, ok := tls["utls"].(map[string]interface{}); ok {
		if fingerprint, ok := utls["fingerprint"].(string); ok && fingerprint != "" {
			*params = append(*params, LinkParam{"fp", fingerprint})
		}
	}
	if sni, ok := tls["server_name"].(string); ok && sni != "" {
		*params = append(*params, LinkParam{"sni", sni})
	}
	if alpn := stringList(tls["alpn"]); len(alpn) > 0 {
		*params = append(*params, LinkParam{"alpn", strings.Join(alpn, ",")})
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

// adjustHTTPTransportParams maps sing-box's HTTP transport to the network that
// share-link clients expect: HTTP/2 ("http", i.e. h2) when TLS is enabled, and
// the TCP + HTTP header shape for plain connections.
func adjustHTTPTransportParams(params []LinkParam, tlsOn bool) []LinkParam {
	if !tlsOn {
		return params
	}
	isHTTP := false
	for _, param := range params {
		if param.Key == "headerType" && param.Value == "http" {
			isHTTP = true
		}
	}
	if !isHTTP {
		return params
	}
	result := make([]LinkParam, 0, len(params))
	for _, param := range params {
		switch {
		case param.Key == "type" && param.Value == "tcp":
			result = append(result, LinkParam{"type", "http"})
		case param.Key == "headerType":
		default:
			result = append(result, param)
		}
	}
	return result
}

func firstHeaderValue(headers map[string]interface{}, name string) string {
	for key, value := range headers {
		if !strings.EqualFold(key, name) {
			continue
		}
		if values := stringList(value); len(values) > 0 {
			return values[0]
		}
	}
	return ""
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
