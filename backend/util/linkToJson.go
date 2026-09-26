package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Hhz0823/1s-ui/util/common"
	"golang.org/x/net/http/httpguts"
)

// GetOutbound converts a share link into a sing-box outbound. The result is
// normalised so it can be saved and started by sing-box without edits.
func GetOutbound(uri string, i int) (*map[string]interface{}, string, error) {
	uri = strings.TrimSpace(uri)
	scheme, _, found := strings.Cut(uri, "://")
	if !found {
		return nil, "", common.NewError("Unsupported link format")
	}
	var (
		outbound *map[string]interface{}
		tag      string
		err      error
	)
	switch strings.ToLower(scheme) {
	case "vmess":
		outbound, tag, err = vmessFromLink(uri, i)
	case "ss", "shadowsocks":
		outbound, tag, err = ssFromLink(uri, i)
	case "socks", "socks5", "socks5h", "socks4", "socks4a":
		outbound, tag, err = socksFromLink(uri, i)
	case "hy2", "hysteria2":
		outbound, tag, err = hy2FromLink(uri, i)
	default:
		var u *url.URL
		u, err = url.Parse(uri)
		if err != nil {
			return nil, "", common.NewError("Unsupported link format")
		}
		switch strings.ToLower(u.Scheme) {
		case "vless":
			outbound, tag, err = vless(u, i)
		case "trojan":
			outbound, tag, err = trojan(u, i)
		case "hy", "hysteria":
			outbound, tag, err = hy(u, i)
		case "anytls":
			outbound, tag, err = anytls(u, i)
		case "tuic":
			outbound, tag, err = tuic(u, i)
		case "http", "https":
			outbound, tag, err = httpProxyFromLink(u, i)
		case "naive+https", "naive+quic", "http2":
			outbound, tag, err = parseNaiveLink(u, i)
		default:
			return nil, "", common.NewError("Unsupported link format")
		}
	}
	if err != nil {
		return nil, "", err
	}
	if outbound == nil {
		return nil, "", common.NewError("Unsupported link format")
	}
	NormalizeSingBoxOutbound(*outbound)
	return outbound, tag, nil
}

// linkTag returns the display name of a link, falling back to a readable
// type-host-port tag because every sing-box outbound needs a tag.
func linkTag(fragment string, query url.Values, outboundType, host string, port, index int) string {
	tag := strings.TrimSpace(fragment)
	if tag == "" && query != nil {
		tag = strings.TrimSpace(query.Get("remarks"))
		if tag == "" {
			tag = strings.TrimSpace(query.Get("remark"))
		}
	}
	if tag == "" {
		tag = fmt.Sprintf("%s-%s-%d", outboundType, strings.Trim(host, "[]"), port)
	}
	if index > 0 {
		tag = fmt.Sprintf("%d.%s", index, tag)
	}
	return tag
}

func decodeLinkBase64(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	value = strings.NewReplacer("\r", "", "\n", "", " ", "").Replace(value)
	if unescaped, err := url.PathUnescape(value); err == nil {
		value = unescaped
	}
	trimmed := strings.TrimRight(value, "=")
	var lastErr error
	for _, encoding := range []*base64.Encoding{base64.RawStdEncoding, base64.RawURLEncoding} {
		decoded, err := encoding.DecodeString(trimmed)
		if err == nil {
			return decoded, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func decodeLinkBase64Text(value string) (string, bool) {
	decoded, err := decodeLinkBase64(value)
	if err != nil || len(decoded) == 0 || !utf8.Valid(decoded) {
		return "", false
	}
	return string(decoded), true
}

func parseLinkPort(value string, fallback int) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, common.NewErrorf("invalid port %q", value)
	}
	return port, nil
}

func linkHostAndPort(u *url.URL, fallback int) (string, int, error) {
	host := u.Hostname()
	if host == "" {
		return "", 0, common.NewError("link has no server address")
	}
	port, err := parseLinkPort(u.Port(), fallback)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

func queryFlag(query url.Values, keys ...string) bool {
	for _, key := range keys {
		switch strings.ToLower(strings.TrimSpace(query.Get(key))) {
		case "1", "true", "yes":
			return true
		}
	}
	return false
}

func anyToInt(value interface{}) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		number, _ := strconv.Atoi(strings.TrimSpace(typed))
		return number
	case json.Number:
		number, _ := typed.Int64()
		return int(number)
	}
	return 0
}

func anyToString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		if typed {
			return "1"
		}
		return "0"
	case nil:
		return ""
	}
	return fmt.Sprint(value)
}

func truthy(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes":
			return true
		}
	}
	return false
}

func vmessFromLink(uri string, i int) (*map[string]interface{}, string, error) {
	payload := uri[len("vmess://"):]
	if cut := strings.IndexAny(payload, "?#"); cut >= 0 && !strings.Contains(payload[:cut], "@") {
		payload = payload[:cut]
	}
	if decoded, err := decodeLinkBase64(payload); err == nil {
		var dataJson map[string]interface{}
		if err = json.Unmarshal(decoded, &dataJson); err == nil {
			return vmessFromJSON(dataJson, i)
		}
	}
	// Xray-style "vmess://uuid@host:port?type=ws&security=tls#name" links.
	u, err := url.Parse(uri)
	if err != nil || u.User == nil {
		return nil, "", common.NewError("Invalid vmess link")
	}
	query := u.Query()
	host, port, err := linkHostAndPort(u, 443)
	if err != nil {
		return nil, "", err
	}
	transport, err := getTransport(query.Get("type"), &query)
	if err != nil {
		return nil, "", err
	}
	security := query.Get("encryption")
	if security == "" {
		security = "auto"
	}
	tag := linkTag(u.Fragment, query, "vmess", host, port, i)
	outbound := map[string]interface{}{
		"type":        "vmess",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"uuid":        u.User.Username(),
		"security":    security,
		"alter_id":    0,
		"tls":         getTls(query.Get("security"), &query),
	}
	if len(transport) > 0 {
		outbound["transport"] = transport
	}
	return &outbound, tag, nil
}

func vmessFromJSON(dataJson map[string]interface{}, i int) (*map[string]interface{}, string, error) {
	host := strings.Trim(anyToString(dataJson["add"]), "[] ")
	port := anyToInt(dataJson["port"])
	if host == "" || port < 1 || port > 65535 {
		return nil, "", common.NewError("Invalid vmess server address")
	}
	query := url.Values{}
	for _, key := range []string{"host", "path", "sni", "alpn", "fp"} {
		if value := anyToString(dataJson[key]); value != "" {
			query.Set(key, value)
		}
	}
	tpNet := strings.ToLower(anyToString(dataJson["net"]))
	if tpType := anyToString(dataJson["type"]); tpNet == "tcp" || tpNet == "" {
		query.Set("headerType", tpType)
	}
	if tpNet == "grpc" {
		query.Set("serviceName", anyToString(dataJson["path"]))
	}
	transport, err := getTransport(tpNet, &query)
	if err != nil {
		return nil, "", err
	}
	tlsConfig := map[string]interface{}{}
	if security := strings.ToLower(anyToString(dataJson["tls"])); security == "tls" {
		if truthy(dataJson["allowInsecure"]) || truthy(dataJson["insecure"]) || truthy(dataJson["skip-cert-verify"]) {
			query.Set("insecure", "1")
		}
		if pin := anyToString(dataJson["pcs"]); pin != "" {
			query.Set("pcs", pin)
		} else if pin := anyToString(dataJson["pinSHA256"]); pin != "" {
			query.Set("pcs", pin)
		}
		tlsConfig = getTls("tls", &query)
	}
	security := anyToString(dataJson["scy"])
	if security == "" {
		security = "auto"
	}
	alterID := anyToInt(dataJson["aid"])
	tag := linkTag(anyToString(dataJson["ps"]), nil, "vmess", host, port, i)
	vmess := map[string]interface{}{
		"type":        "vmess",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"uuid":        anyToString(dataJson["id"]),
		"security":    security,
		"alter_id":    alterID,
		"tls":         tlsConfig,
	}
	if len(transport) > 0 {
		vmess["transport"] = transport
	}
	return &vmess, tag, nil
}

func vless(u *url.URL, i int) (*map[string]interface{}, string, error) {
	query := u.Query()
	security := strings.ToLower(query.Get("security"))
	defaultPort := 80
	if security == "tls" || security == "reality" {
		defaultPort = 443
	}
	host, port, err := linkHostAndPort(u, defaultPort)
	if err != nil {
		return nil, "", err
	}
	transport, err := getTransport(query.Get("type"), &query)
	if err != nil {
		return nil, "", err
	}
	if u.User == nil || u.User.Username() == "" {
		return nil, "", common.NewError("vless link has no UUID")
	}
	tag := linkTag(u.Fragment, query, "vless", host, port, i)
	vless := map[string]interface{}{
		"type":        "vless",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"uuid":        u.User.Username(),
		"tls":         getTls(security, &query),
	}
	switch flow := query.Get("flow"); flow {
	case "xtls-rprx-vision", "xtls-rprx-vision-udp443":
		// sing-box only implements the base Vision flow.
		vless["flow"] = "xtls-rprx-vision"
	case "", "none":
	default:
		return nil, "", common.NewErrorf("vless flow %q is not supported by sing-box", flow)
	}
	if packetEncoding := query.Get("packetEncoding"); packetEncoding == "xudp" || packetEncoding == "packetaddr" {
		vless["packet_encoding"] = packetEncoding
	}
	if len(transport) > 0 {
		vless["transport"] = transport
	}
	return &vless, tag, nil
}

func trojan(u *url.URL, i int) (*map[string]interface{}, string, error) {
	query := u.Query()
	security := strings.ToLower(query.Get("security"))
	if security == "" {
		// Trojan is a TLS protocol; share links omit security=tls by default.
		security = "tls"
	}
	defaultPort := 443
	if security == "none" {
		defaultPort = 80
	}
	host, port, err := linkHostAndPort(u, defaultPort)
	if err != nil {
		return nil, "", err
	}
	transport, err := getTransport(query.Get("type"), &query)
	if err != nil {
		return nil, "", err
	}
	password := ""
	if u.User != nil {
		password = u.User.Username()
	}
	tag := linkTag(u.Fragment, query, "trojan", host, port, i)
	trojan := map[string]interface{}{
		"type":        "trojan",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"password":    password,
		"tls":         getTls(security, &query),
	}
	if len(transport) > 0 {
		trojan["transport"] = transport
	}
	return &trojan, tag, nil
}

func hy(u *url.URL, i int) (*map[string]interface{}, string, error) {
	query := u.Query()
	host, port, err := linkHostAndPort(u, 443)
	if err != nil {
		return nil, "", err
	}

	tag := linkTag(u.Fragment, query, "hysteria", host, port, i)
	hy := map[string]interface{}{
		"type":        "hysteria",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"tls":         getTls("tls", &query),
	}
	if obfs := query.Get("obfsParam"); obfs != "" {
		hy["obfs"] = obfs
	}
	if auth := query.Get("auth"); auth != "" {
		hy["auth_str"] = auth
	}
	down, _ := strconv.Atoi(query.Get("downmbps"))
	up, _ := strconv.Atoi(query.Get("upmbps"))
	recv_window_conn, _ := strconv.Atoi(query.Get("recv_window_conn"))
	recv_window, _ := strconv.Atoi(query.Get("recv_window"))
	// Hysteria v1 requires both bandwidth values; links frequently omit them.
	if down <= 0 {
		down = 100
	}
	if up <= 0 {
		up = 50
	}
	hy["down_mbps"] = down
	hy["up_mbps"] = up
	if recv_window_conn > 0 {
		hy["recv_window_conn"] = recv_window_conn
	}
	if recv_window > 0 {
		hy["recv_window"] = recv_window
	}
	if tlsConfig, ok := hy["tls"].(map[string]interface{}); ok && tlsConfig["alpn"] == nil {
		tlsConfig["alpn"] = []string{"hysteria"}
	}
	return &hy, tag, nil
}

// splitHysteriaPorts extracts Hysteria2 multi-port authorities such as
// "host:443,20000-30000". net/url rejects them, so the first single port is
// kept in the URL and every range becomes a sing-box server_ports entry.
func splitHysteriaPorts(uri string) (string, []string) {
	schemeEnd := strings.Index(uri, "://")
	if schemeEnd < 0 {
		return uri, nil
	}
	rest := uri[schemeEnd+3:]
	authorityEnd := strings.IndexAny(rest, "/?#")
	if authorityEnd < 0 {
		authorityEnd = len(rest)
	}
	authority := rest[:authorityEnd]
	userEnd := strings.LastIndex(authority, "@")
	hostPort := authority[userEnd+1:]
	portStart := strings.LastIndex(hostPort, ":")
	if strings.HasPrefix(hostPort, "[") {
		closing := strings.Index(hostPort, "]")
		if closing < 0 || portStart < closing {
			return uri, nil
		}
	}
	if portStart < 0 {
		return uri, nil
	}
	portSpec := hostPort[portStart+1:]
	if !strings.ContainsAny(portSpec, ",-") {
		return uri, nil
	}
	firstPort := ""
	var ranges []string
	for _, item := range strings.Split(portSpec, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if start, end, isRange := strings.Cut(item, "-"); isRange {
			ranges = append(ranges, strings.TrimSpace(start)+":"+strings.TrimSpace(end))
			if firstPort == "" {
				firstPort = strings.TrimSpace(start)
			}
			continue
		}
		if firstPort == "" {
			firstPort = item
		}
		ranges = append(ranges, item+":"+item)
	}
	newAuthority := authority[:userEnd+1] + hostPort[:portStart+1] + firstPort
	return uri[:schemeEnd+3] + newAuthority + rest[authorityEnd:], ranges
}

func hy2FromLink(uri string, i int) (*map[string]interface{}, string, error) {
	cleaned, hopPorts := splitHysteriaPorts(uri)
	u, err := url.Parse(cleaned)
	if err != nil {
		return nil, "", common.NewError("Invalid hysteria2 link")
	}
	query := u.Query()
	host, port, err := linkHostAndPort(u, 443)
	if err != nil {
		return nil, "", err
	}

	password := ""
	if u.User != nil {
		// Hysteria2 "userpass" auth is sent as the literal "user:pass" string.
		password = u.User.Username()
		if secret, hasSecret := u.User.Password(); hasSecret {
			password += ":" + secret
		}
	}
	if password == "" {
		password = query.Get("auth")
	}

	tag := linkTag(u.Fragment, query, "hysteria2", host, port, i)
	hy2 := map[string]interface{}{
		"type":        "hysteria2",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"password":    password,
		"tls":         getTls("tls", &query),
	}
	down, _ := strconv.Atoi(query.Get("downmbps"))
	up, _ := strconv.Atoi(query.Get("upmbps"))
	if down > 0 {
		hy2["down_mbps"] = down
	}
	if up > 0 {
		hy2["up_mbps"] = up
	}
	if obfs := strings.ToLower(query.Get("obfs")); obfs == "salamander" {
		hy2["obfs"] = map[string]interface{}{
			"type":     "salamander",
			"password": query.Get("obfs-password"),
		}
	} else if obfs != "" && obfs != "none" && obfs != "plain" {
		return nil, "", common.NewErrorf("unsupported hysteria2 obfs %q", obfs)
	}
	for _, portRange := range strings.Split(strings.NewReplacer(" ", "").Replace(query.Get("mport")), ",") {
		if portRange == "" {
			continue
		}
		if !strings.Contains(portRange, "-") {
			portRange += "-" + portRange
		}
		hopPorts = append(hopPorts, strings.Replace(portRange, "-", ":", 1))
	}
	if len(hopPorts) > 0 {
		hy2["server_ports"] = hopPorts
	}
	return &hy2, tag, nil
}

func anytls(u *url.URL, i int) (*map[string]interface{}, string, error) {
	query := u.Query()
	host, port, err := linkHostAndPort(u, 443)
	if err != nil {
		return nil, "", err
	}

	password := ""
	if u.User != nil {
		password = u.User.Username()
	}
	tag := linkTag(u.Fragment, query, "anytls", host, port, i)
	anytls := map[string]interface{}{
		"type":        "anytls",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"password":    password,
		"tls":         getTls("tls", &query),
	}
	return &anytls, tag, nil
}

func tuic(u *url.URL, i int) (*map[string]interface{}, string, error) {
	query := u.Query()
	host, port, err := linkHostAndPort(u, 443)
	if err != nil {
		return nil, "", err
	}

	if u.User == nil {
		return nil, "", common.NewError("tuic link has no UUID")
	}
	password, _ := u.User.Password()
	tag := linkTag(u.Fragment, query, "tuic", host, port, i)
	tlsConfig := getTls("tls", &query)
	if tlsConfig["alpn"] == nil {
		tlsConfig["alpn"] = []string{"h3"}
	}
	tuic := map[string]interface{}{
		"type":        "tuic",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"uuid":        u.User.Username(),
		"password":    password,
		"tls":         tlsConfig,
	}
	congestionControl := query.Get("congestion_control")
	if congestionControl == "" {
		congestionControl = query.Get("congestion-control")
	}
	switch congestionControl {
	case "cubic", "new_reno", "bbr":
		tuic["congestion_control"] = congestionControl
	}
	udpRelayMode := query.Get("udp_relay_mode")
	if udpRelayMode == "" {
		udpRelayMode = query.Get("udp-relay-mode")
	}
	switch udpRelayMode {
	case "native", "quic":
		tuic["udp_relay_mode"] = udpRelayMode
	}
	return &tuic, tag, nil
}

// ssFromLink parses SIP002 (base64url or percent-encoded userinfo) and the
// legacy fully base64 encoded form. It does not rely on net/url for the
// userinfo because standard base64 may contain "/" which url.Parse treats as
// the start of the path.
func ssFromLink(uri string, i int) (*map[string]interface{}, string, error) {
	rest := uri[strings.Index(uri, "://")+3:]
	fragment := ""
	if before, after, found := strings.Cut(rest, "#"); found {
		rest = before
		if unescaped, err := url.PathUnescape(after); err == nil {
			fragment = unescaped
		} else {
			fragment = after
		}
	}
	rawQuery := ""
	if before, after, found := strings.Cut(rest, "?"); found {
		rest = before
		rawQuery = after
	}
	query, _ := url.ParseQuery(rawQuery)
	rest = strings.TrimRight(rest, "/")

	var userInfo, hostPort string
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		userInfo, hostPort = rest[:at], rest[at+1:]
	} else {
		decoded, ok := decodeLinkBase64Text(rest)
		if !ok {
			return nil, "", common.NewError("Unsupported shadowsocks")
		}
		at := strings.LastIndex(decoded, "@")
		if at < 0 {
			return nil, "", common.NewError("Unsupported shadowsocks")
		}
		userInfo, hostPort = decoded[:at], decoded[at+1:]
	}

	method, password := "", ""
	if unescaped, err := url.PathUnescape(userInfo); err == nil && strings.Contains(unescaped, ":") {
		method, password, _ = strings.Cut(unescaped, ":")
	} else if decoded, ok := decodeLinkBase64Text(userInfo); ok && strings.Contains(decoded, ":") {
		method, password, _ = strings.Cut(decoded, ":")
	} else {
		return nil, "", common.NewError("Unsupported shadowsocks")
	}
	host, portText, err := net.SplitHostPort(hostPort)
	if err != nil {
		return nil, "", common.NewError("Invalid shadowsocks server address")
	}
	port, err := parseLinkPort(portText, 443)
	if err != nil {
		return nil, "", err
	}
	if query.Get("shadow-tls") != "" {
		return nil, "", common.NewError("Shadowsocks over ShadowTLS links need a separate ShadowTLS outbound; add it manually")
	}

	tag := linkTag(fragment, query, "shadowsocks", host, port, i)
	ss := map[string]interface{}{
		"type":        "shadowsocks",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"method":      strings.ToLower(method),
		"password":    password,
	}

	v2ray_type := query.Get("type")
	if len(v2ray_type) > 0 {
		pl_arr := []string{}
		host_header := query.Get("host")
		if query.Get("security") == "tls" {
			pl_arr = append(pl_arr, "tls")
		}
		if v2ray_type == "quic" {
			pl_arr = append(pl_arr, "mode=quic")
		}
		if len(host_header) > 0 {
			pl_arr = append(pl_arr, "host="+host_header)
		}
		ss["plugin"] = "v2ray-plugin"
		ss["plugin_opts"] = strings.Join(pl_arr, ";")
	}
	plugin := query.Get("plugin")
	if len(plugin) > 0 {
		pl_arr := strings.Split(plugin, ";")
		if len(pl_arr) > 0 {
			ss["plugin"] = pl_arr[0]
			ss["plugin_opts"] = strings.Join(pl_arr[1:], ";")
		}
	}
	if queryFlag(query, "uot", "udp-over-tcp") {
		ss["udp_over_tcp"] = map[string]interface{}{"enabled": true}
	}
	return &ss, tag, nil
}

// socksFromLink accepts socks5://user:pass@host:port, the v2rayN form
// socks://BASE64(user:pass)@host:port and the legacy socks://BASE64(all).
func socksFromLink(uri string, i int) (*map[string]interface{}, string, error) {
	scheme, rest, _ := strings.Cut(uri, "://")
	scheme = strings.ToLower(scheme)
	fragment := ""
	if before, after, found := strings.Cut(rest, "#"); found {
		rest = before
		if unescaped, err := url.PathUnescape(after); err == nil {
			fragment = unescaped
		} else {
			fragment = after
		}
	}
	rawQuery := ""
	if before, after, found := strings.Cut(rest, "?"); found {
		rest = before
		rawQuery = after
	}
	rest = strings.TrimRight(rest, "/")
	if !strings.Contains(rest, "@") && !strings.Contains(rest, ":") {
		if decoded, ok := decodeLinkBase64Text(rest); ok {
			rest = decoded
		}
	}
	u, err := url.Parse("socks://" + rest)
	if err != nil {
		// The credential part may contain unescaped reserved characters.
		at := strings.LastIndex(rest, "@")
		if at < 0 {
			return nil, "", common.NewError("Invalid socks link")
		}
		credentials := rest[:at]
		u, err = url.Parse("socks://" + rest[at+1:])
		if err != nil {
			return nil, "", common.NewError("Invalid socks link")
		}
		username, password, _ := strings.Cut(credentials, ":")
		u.User = url.UserPassword(username, password)
	}
	query, _ := url.ParseQuery(rawQuery)
	host, port, err := linkHostAndPort(u, 1080)
	if err != nil {
		return nil, "", err
	}
	username, password := "", ""
	if u.User != nil {
		username = u.User.Username()
		var hasPassword bool
		password, hasPassword = u.User.Password()
		if !hasPassword {
			if decoded, ok := decodeLinkBase64Text(username); ok && strings.Contains(decoded, ":") {
				username, password, _ = strings.Cut(decoded, ":")
			}
		}
	}
	version := "5"
	switch scheme {
	case "socks4":
		version = "4"
	case "socks4a":
		version = "4a"
	}
	tag := linkTag(fragment, query, "socks", host, port, i)
	socks := map[string]interface{}{
		"type":        "socks",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"version":     version,
	}
	if username != "" {
		socks["username"] = username
	}
	if password != "" {
		socks["password"] = password
	}
	return &socks, tag, nil
}

func httpProxyFromLink(u *url.URL, i int) (*map[string]interface{}, string, error) {
	query := u.Query()
	defaultPort := 80
	if u.Scheme == "https" {
		defaultPort = 443
	}
	host, port, err := linkHostAndPort(u, defaultPort)
	if err != nil {
		return nil, "", err
	}
	tag := linkTag(u.Fragment, query, "http", host, port, i)
	proxy := map[string]interface{}{
		"type":        "http",
		"tag":         tag,
		"server":      host,
		"server_port": port,
	}
	if u.User != nil {
		if username := u.User.Username(); username != "" {
			proxy["username"] = username
		}
		if password, ok := u.User.Password(); ok && password != "" {
			proxy["password"] = password
		}
	}
	if u.Scheme == "https" {
		tlsConfig := getTls("tls", &query)
		if tlsConfig["server_name"] == nil && net.ParseIP(host) == nil {
			tlsConfig["server_name"] = host
		}
		proxy["tls"] = tlsConfig
	}
	return &proxy, tag, nil
}

func parseNaiveLink(u *url.URL, i int) (*map[string]interface{}, string, error) {
	var host, portStr, username, password string
	var port int

	switch u.Scheme {
	case "http2":
		decoded := StrOrBase64Encoded(u.Hostname())
		if idx := strings.Index(decoded, "@"); idx != -1 {
			userInfo := decoded[:idx]
			hostPort := decoded[idx+1:]
			if idx2 := strings.Index(userInfo, ":"); idx2 != -1 {
				username = userInfo[:idx2]
				password = userInfo[idx2+1:]
			} else {
				username = userInfo
			}
			host, portStr, _ = net.SplitHostPort(hostPort)
			if portStr != "" {
				port, _ = strconv.Atoi(portStr)
			} else {
				port = 443
			}
		} else {
			return nil, "", common.NewError("Invalid naive link (http2)")
		}
	case "naive+https", "naive+quic":
		host, portStr, _ = net.SplitHostPort(u.Host)
		if portStr != "" {
			port, _ = strconv.Atoi(portStr)
		} else {
			port = 443
		}
		if u.User != nil {
			username = u.User.Username()
			password, _ = u.User.Password()
		}
	default:
		return nil, "", common.NewError("Unsupported naive scheme")
	}

	tag := u.Fragment
	if i > 0 {
		tag = fmt.Sprintf("%d.%s", i, u.Fragment)
	}
	if tag == "" {
		tag = fmt.Sprintf("naive-%d", i)
	}

	naive := map[string]interface{}{
		"type":        "naive",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"username":    username,
		"password":    password,
		"tls":         map[string]interface{}{"enabled": true},
	}

	query := u.Query()
	peer := query.Get("sni")
	if peer == "" {
		peer = query.Get("peer")
	}
	if peer != "" {
		if tls, ok := naive["tls"].(map[string]interface{}); ok {
			tls["server_name"] = peer
		}
	}
	if u.Scheme == "naive+quic" {
		naive["quic"] = true
	}
	if extraHeaders := query.Get("extra-headers"); extraHeaders != "" {
		headers := map[string]string{}
		for _, line := range strings.Split(strings.ReplaceAll(extraHeaders, "\r\n", "\n"), "\n") {
			separator := strings.Index(line, ":")
			if separator <= 0 {
				continue
			}
			name := strings.TrimSpace(line[:separator])
			value := strings.TrimSpace(line[separator+1:])
			if validNaiveExtraHeader(name, value) {
				headers[name] = value
			}
			if len(headers) >= 32 {
				break
			}
		}
		if len(headers) > 0 {
			naive["extra_headers"] = headers
		}
	}

	return &naive, tag, nil
}

func isReservedNaiveHeader(name string) bool {
	switch strings.ToLower(name) {
	case "proxy-authorization", "padding", "content-length", "transfer-encoding", "connection":
		return true
	default:
		return false
	}
}

func validNaiveExtraHeader(name, value string) bool {
	return httpguts.ValidHeaderFieldName(name) &&
		httpguts.ValidHeaderFieldValue(value) &&
		value != "" && len(value) <= 4096 &&
		!isReservedNaiveHeader(name)
}

func getTransport(tp_type string, q *url.Values) (map[string]interface{}, error) {
	transport := map[string]interface{}{}
	tp_host := q.Get("host")
	tp_path := q.Get("path")
	switch strings.ToLower(tp_type) {
	case "tcp", "raw", "", "none":
		if q.Get("headerType") == "http" {
			transport["type"] = "http"
			if len(tp_host) > 0 {
				transport["host"] = strings.Split(tp_host, ",")
			}
			if len(tp_path) > 0 {
				transport["path"] = tp_path
			}
		}
	case "http", "h2":
		transport["type"] = "http"
		if len(tp_host) > 0 {
			transport["host"] = strings.Split(tp_host, ",")
		}
		if len(tp_path) > 0 {
			transport["path"] = tp_path
		}
	case "ws", "websocket":
		transport["type"] = "ws"
		path, earlyData := splitWebSocketEarlyData(tp_path)
		if path != "" {
			transport["path"] = path
		}
		if earlyData > 0 {
			transport["max_early_data"] = earlyData
			transport["early_data_header_name"] = "Sec-WebSocket-Protocol"
		}
		if len(tp_host) > 0 {
			transport["headers"] = map[string]interface{}{
				"Host": tp_host,
			}
		}
	case "quic":
		transport["type"] = "quic"
	case "grpc", "gun":
		transport["type"] = "grpc"
		transport["service_name"] = q.Get("serviceName")
	case "httpupgrade":
		transport["type"] = "httpupgrade"
		if len(tp_path) > 0 {
			transport["path"] = tp_path
		}
		if len(tp_host) > 0 {
			transport["host"] = tp_host
		}
	default:
		// xhttp/splithttp/kcp only exist in Xray; silently falling back to TCP
		// produced outbounds that saved fine but never connected.
		return nil, common.NewErrorf("transport %q is not supported by sing-box outbounds", tp_type)
	}
	return transport, nil
}

// splitWebSocketEarlyData converts the Xray "/path?ed=2048" convention into
// sing-box max_early_data.
func splitWebSocketEarlyData(path string) (string, int) {
	base, rawQuery, found := strings.Cut(path, "?")
	if !found {
		return path, 0
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return path, 0
	}
	earlyData, err := strconv.Atoi(values.Get("ed"))
	if err != nil || earlyData <= 0 {
		return path, 0
	}
	values.Del("ed")
	if encoded := values.Encode(); encoded != "" {
		base += "?" + encoded
	}
	return base, earlyData
}

func getTls(security string, q *url.Values) map[string]interface{} {
	tls := map[string]interface{}{}
	tls_fp := q.Get("fp")
	tls_sni := q.Get("sni")
	if tls_sni == "" {
		tls_sni = q.Get("peer")
	}
	tls_alpn := q.Get("alpn")
	tls_ech := q.Get("ech")
	tls_pin := q.Get("pcs")
	if tls_pin == "" {
		tls_pin = q.Get("pinSHA256")
	}
	switch strings.ToLower(security) {
	case "tls", "xtls":
		tls["enabled"] = true
	case "reality":
		tls["enabled"] = true
		tls["reality"] = map[string]interface{}{
			"enabled":    true,
			"public_key": q.Get("pbk"),
			"short_id":   q.Get("sid"),
		}
		if tls_fp == "" {
			// sing-box requires uTLS for Reality clients.
			tls_fp = "chrome"
		}
	default:
		return tls
	}
	if len(tls_sni) > 0 {
		tls["server_name"] = tls_sni
	}
	if len(tls_alpn) > 0 {
		tls["alpn"] = strings.Split(tls_alpn, ",")
	}
	if queryFlag(*q, "insecure", "allowInsecure", "allow_insecure", "skip-cert-verify") {
		tls["insecure"] = true
	}
	if len(tls_fp) > 0 && tls_fp != "none" {
		tls["utls"] = map[string]interface{}{
			"enabled":     true,
			"fingerprint": tls_fp,
		}
	}
	if len(tls_ech) > 0 {
		tls["ech"] = map[string]interface{}{
			"enabled": true,
			"config": []string{
				tls_ech,
			},
		}
	}
	if queryFlag(*q, "disable_sni") {
		tls["disable_sni"] = true
	}
	if tlsPin := pinnedPeerCertSha256ForConfig(tls_pin); tlsPin != "" {
		// Kept only until NormalizeSingBoxOutbound decides how sing-box can
		// trust this certificate; sing-box has no certificate-hash pin option.
		tls["pinned_peer_certificate_sha256"] = []string{tlsPin}
	}
	return tls
}
