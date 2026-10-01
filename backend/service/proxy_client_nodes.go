package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/proxyprobe"
	"github.com/Hhz0823/1s-ui/util"
	"github.com/Hhz0823/1s-ui/util/common"

	M "github.com/sagernet/sing/common/metadata"
	"gorm.io/gorm"
)

const (
	proxyClientMaxSubscriptions = 50
	proxyClientSubscriptionMax  = 16 << 20
	// Airports serve share links to v2rayN; other agents may get Clash YAML.
	proxyClientDefaultUserAgent = "v2rayN/7.13.8"
)

// ProxyClientSubscriptionInput saves a subscription; an empty URL keeps the
// stored one (the page never receives it in full on remote devices).
type ProxyClientSubscriptionInput struct {
	Id          uint   `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	UserAgent   string `json:"user_agent"`
	AutoUpdate  int    `json:"auto_update"`
	Enabled     *bool  `json:"enabled"`
	ProxyUpdate bool   `json:"proxy_update"`
}

// SaveSubscription adds or changes a subscription and fetches its nodes.
func (s *ProxyClientService) SaveSubscription(input ProxyClientSubscriptionInput) (*ProxyClientState, error) {
	db := database.GetDB()
	subscription := model.ProxyClientSubscription{Enabled: true, CreatedAt: time.Now().Unix()}
	if input.Id != 0 {
		if err := db.Where("id = ?", input.Id).First(&subscription).Error; err != nil {
			return nil, common.NewError("subscription not found")
		}
	} else {
		var count int64
		db.Model(&model.ProxyClientSubscription{}).Count(&count)
		if count >= proxyClientMaxSubscriptions {
			return nil, common.NewErrorf("at most %d subscriptions", proxyClientMaxSubscriptions)
		}
	}
	if link := strings.TrimSpace(input.URL); link != "" {
		parsed, err := url.Parse(link)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || len(link) > 2048 {
			return nil, common.NewError("subscription URL must be an http:// or https:// address")
		}
		subscription.URL = link
	}
	if subscription.URL == "" {
		return nil, common.NewError("subscription URL is required")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		if parsed, err := url.Parse(subscription.URL); err == nil {
			name = parsed.Hostname()
		}
	}
	if len([]rune(name)) > 80 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return nil, common.NewError("subscription name must be at most 80 characters")
	}
	if input.AutoUpdate < 0 || input.AutoUpdate > 24*30 {
		return nil, common.NewError("auto update must be 0-720 hours")
	}
	userAgent := strings.TrimSpace(input.UserAgent)
	if len(userAgent) > 128 || strings.IndexFunc(userAgent, unicode.IsControl) >= 0 {
		return nil, common.NewError("invalid user agent")
	}
	subscription.Name, subscription.UserAgent, subscription.AutoUpdate = name, userAgent, input.AutoUpdate
	subscription.ProxyUpdate = input.ProxyUpdate
	if input.Enabled != nil {
		subscription.Enabled = *input.Enabled
	}
	if err := db.Save(&subscription).Error; err != nil {
		return nil, err
	}
	if subscription.Enabled {
		if err := s.updateSubscription(subscription.Id); err != nil {
			state, stateErr := s.State()
			if stateErr != nil {
				return nil, stateErr
			}
			// Saved; the error is on the subscription for the page to show.
			return state, nil
		}
	}
	return s.State()
}

// DeleteSubscription removes a subscription and its nodes.
func (s *ProxyClientService) DeleteSubscription(id uint) (*ProxyClientState, error) {
	db := database.GetDB()
	proxyClientMu.Lock()
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("subscription_id = ?", id).Delete(&model.ProxyClientNode{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&model.ProxyClientSubscription{}).Error
	})
	proxyClientMu.Unlock()
	if err != nil {
		return nil, err
	}
	s.reapplyIfNodesChanged()
	return s.State()
}

// UpdateSubscriptions fetches one subscription (id) or every enabled one (0).
func (s *ProxyClientService) UpdateSubscriptions(id uint) (*ProxyClientState, error) {
	db := database.GetDB()
	var subscriptions []model.ProxyClientSubscription
	query := db.Order("id ASC")
	if id != 0 {
		query = query.Where("id = ?", id)
	} else {
		query = query.Where("enabled = ?", true)
	}
	if err := query.Find(&subscriptions).Error; err != nil {
		return nil, err
	}
	if id != 0 && len(subscriptions) == 0 {
		return nil, common.NewError("subscription not found")
	}
	var failures []string
	for _, subscription := range subscriptions {
		if err := s.updateSubscription(subscription.Id); err != nil {
			failures = append(failures, subscription.Name+": "+err.Error())
		}
	}
	if id != 0 && len(failures) > 0 {
		return nil, common.NewError(failures[0])
	}
	return s.State()
}

// UpdateDueSubscriptions refreshes subscriptions whose auto-update time
// passed; the cron job calls it.
func (s *ProxyClientService) UpdateDueSubscriptions(now time.Time) {
	var subscriptions []model.ProxyClientSubscription
	if err := database.GetDB().Where("enabled = ? AND auto_update > 0", true).Find(&subscriptions).Error; err != nil {
		return
	}
	for _, subscription := range subscriptions {
		if now.Unix()-subscription.UpdatedAt < int64(subscription.AutoUpdate)*3600 {
			continue
		}
		if err := s.updateSubscription(subscription.Id); err != nil {
			logger.Warning("update subscription ", subscription.Name, ": ", err)
		}
	}
}

func (s *ProxyClientService) updateSubscription(id uint) error {
	db := database.GetDB()
	var subscription model.ProxyClientSubscription
	if err := db.Where("id = ?", id).First(&subscription).Error; err != nil {
		return err
	}
	body, header, err := fetchProxyClientSubscription(subscription)
	var nodes []parsedClientNode
	if err == nil {
		nodes, err = parseProxyClientSubscription(body)
	}
	now := time.Now().Unix()
	if err != nil {
		db.Model(&model.ProxyClientSubscription{}).Where("id = ?", id).Updates(map[string]interface{}{
			"last_error": truncate(err.Error(), 255), "updated_at": now,
		})
		return err
	}
	updates := map[string]interface{}{"last_error": "", "updated_at": now, "node_count": len(nodes)}
	for key, value := range parseSubscriptionUserinfo(header.Get("Subscription-Userinfo")) {
		updates[key] = value
	}
	proxyClientMu.Lock()
	err = replaceProxyClientNodes(id, nodes)
	if err == nil {
		err = db.Model(&model.ProxyClientSubscription{}).Where("id = ?", id).Updates(updates).Error
	}
	proxyClientMu.Unlock()
	if err != nil {
		return err
	}
	s.reapplyIfNodesChanged()
	return nil
}

// fetchProxyClientSubscription downloads a subscription, through the client
// proxy when the subscription asks for it and the proxy is running.
func fetchProxyClientSubscription(subscription model.ProxyClientSubscription) ([]byte, http.Header, error) {
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSHandshakeTimeout: 15 * time.Second,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
	}
	if subscription.ProxyUpdate && corePtr != nil && corePtr.HasOutbound(proxyClientProxyTag) {
		dialer, err := core.OutboundDialer(proxyClientProxyTag)
		if err == nil {
			transport.Proxy = nil
			transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, M.ParseSocksaddr(address))
			}
		}
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 45 * time.Second}
	request, err := http.NewRequest(http.MethodGet, subscription.URL, nil)
	if err != nil {
		return nil, nil, err
	}
	userAgent := subscription.UserAgent
	if userAgent == "" {
		userAgent = proxyClientDefaultUserAgent
	}
	request.Header.Set("User-Agent", userAgent)
	response, err := client.Do(request)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, nil, fmt.Errorf("download failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("the subscription answered HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, proxyClientSubscriptionMax+1))
	if err != nil {
		return nil, nil, err
	}
	if len(body) > proxyClientSubscriptionMax {
		return nil, nil, errors.New("the subscription is too large")
	}
	return body, response.Header, nil
}

// parseSubscriptionUserinfo reads "upload=…; download=…; total=…; expire=…".
func parseSubscriptionUserinfo(value string) map[string]interface{} {
	result := map[string]interface{}{}
	for _, part := range strings.Split(value, ";") {
		key, number, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		parsed, err := strconv.ParseInt(strings.TrimSpace(number), 10, 64)
		if err != nil || parsed < 0 {
			continue
		}
		switch strings.ToLower(key) {
		case "upload", "download", "total", "expire":
			result[strings.ToLower(key)] = parsed
		}
	}
	return result
}

type parsedClientNode struct {
	name     string
	protocol string
	host     string
	port     int
	link     string
	outbound string
}

// parseProxyClientSubscription reads share links (plain or base64) or a
// sing-box JSON config.
func parseProxyClientSubscription(body []byte) ([]parsedClientNode, error) {
	text := strings.TrimSpace(strings.TrimPrefix(string(body), "\ufeff"))
	if text == "" {
		return nil, errors.New("the subscription is empty")
	}
	if strings.HasPrefix(text, "{") {
		return parseSingboxSubscription([]byte(text))
	}
	decoded := strings.TrimSpace(util.StrOrBase64Encoded(text))
	if strings.HasPrefix(decoded, "proxies:") || strings.Contains(decoded, "\nproxies:") {
		return nil, errors.New("this is a Clash subscription; use the v2rayN (share link) or sing-box subscription address")
	}
	nodes := parseClientLinks(decoded)
	if len(nodes) == 0 {
		return nil, errors.New("no supported nodes in the subscription")
	}
	return nodes, nil
}

func parseClientLinks(text string) []parsedClientNode {
	var nodes []parsedClientNode
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "://") {
			continue
		}
		scheme, _, _ := strings.Cut(line, "://")
		switch strings.ToLower(scheme) {
		case "socks", "socks5", "socks5h", "http":
		default:
			if !IsNodeLink(line) {
				continue
			}
		}
		parsed, name, err := util.GetOutbound(line, 0)
		if err != nil || parsed == nil {
			continue
		}
		outbound := *parsed
		node := parsedClientNode{name: strings.TrimSpace(name), link: line}
		node.protocol, _ = outbound["type"].(string)
		node.host, _ = outbound["server"].(string)
		node.port = linkPort(outbound["server_port"])
		if node.host == "" || isSubscriptionInfoNode(node.name) {
			continue
		}
		nodes = append(nodes, node)
		if len(nodes) >= proxyClientMaxNodes {
			break
		}
	}
	return nodes
}

// isSubscriptionInfoNode skips the fake nodes airports use to show the
// remaining traffic or the expiry date.
func isSubscriptionInfoNode(name string) bool {
	lower := strings.ToLower(name)
	for _, marker := range []string{"剩余流量", "套餐到期", "过期时间", "到期时间", "官网", "expire", "traffic left", "remaining"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func parseSingboxSubscription(body []byte) ([]parsedClientNode, error) {
	var config struct {
		Outbounds []map[string]interface{} `json:"outbounds"`
	}
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, errors.New("invalid sing-box subscription: " + err.Error())
	}
	var nodes []parsedClientNode
	for _, outbound := range config.Outbounds {
		copied, info, err := nodeFromOutbound(outbound)
		if err != nil || core.ValidateOutbound(copied) != nil {
			continue
		}
		delete(copied, "detour")
		raw, _ := json.Marshal(copied)
		nodes = append(nodes, parsedClientNode{name: info.Name, protocol: info.Protocol, host: info.Host, port: info.Port, outbound: string(raw)})
		if len(nodes) >= proxyClientMaxNodes {
			break
		}
	}
	if len(nodes) == 0 {
		return nil, errors.New("no supported nodes in the subscription")
	}
	return nodes, nil
}

func clientNodeKey(name, protocol, host string, port int) string {
	return name + "\x00" + protocol + "\x00" + strings.ToLower(host) + "\x00" + strconv.Itoa(port)
}

// replaceProxyClientNodes stores a subscription's nodes. Nodes that did not
// change keep their ids and test results, so a refresh keeps the selection.
func replaceProxyClientNodes(subscriptionID uint, nodes []parsedClientNode) error {
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		var existing []model.ProxyClientNode
		if err := tx.Where("subscription_id = ?", subscriptionID).Find(&existing).Error; err != nil {
			return err
		}
		byKey := map[string]model.ProxyClientNode{}
		for _, node := range existing {
			byKey[clientNodeKey(node.Name, node.Protocol, node.Host, node.Port)] = node
		}
		keep := map[uint]bool{}
		for i, parsed := range nodes {
			name := parsed.name
			if name == "" {
				name = fmt.Sprintf("%s %s:%d", parsed.protocol, parsed.host, parsed.port)
			}
			if len([]rune(name)) > 128 {
				name = string([]rune(name)[:128])
			}
			key := clientNodeKey(name, parsed.protocol, parsed.host, parsed.port)
			node, found := byKey[key]
			if found && !keep[node.Id] {
				keep[node.Id] = true
				if node.Link != parsed.link || node.Outbound != parsed.outbound {
					node.TcpMs, node.DelayMs, node.TestedAt, node.TestError = 0, 0, 0, ""
				}
			} else {
				node = model.ProxyClientNode{SubscriptionId: subscriptionID}
			}
			node.Name, node.Protocol, node.Host, node.Port = name, parsed.protocol, parsed.host, parsed.port
			node.Link, node.Outbound, node.SortOrder = parsed.link, parsed.outbound, i
			if err := tx.Save(&node).Error; err != nil {
				return err
			}
			keep[node.Id] = true
		}
		var stale []uint
		for _, node := range existing {
			if !keep[node.Id] {
				stale = append(stale, node.Id)
			}
		}
		if len(stale) > 0 {
			return tx.Where("id IN ?", stale).Delete(&model.ProxyClientNode{}).Error
		}
		return nil
	})
}

// AddNodes adds share links by hand (one per line).
func (s *ProxyClientService) AddNodes(links string) (*ProxyClientState, error) {
	nodes := parseClientLinks(util.StrOrBase64Encoded(strings.TrimSpace(links)))
	if len(nodes) == 0 {
		return nil, common.NewError("no supported share links; paste vless://, vmess://, trojan://, ss://, hy2://, tuic://, anytls://, socks5:// or http:// links")
	}
	db := database.GetDB()
	var count int64
	db.Model(&model.ProxyClientNode{}).Count(&count)
	if int(count)+len(nodes) > proxyClientMaxNodes {
		return nil, common.NewErrorf("at most %d nodes", proxyClientMaxNodes)
	}
	proxyClientMu.Lock()
	err := db.Transaction(func(tx *gorm.DB) error {
		for _, parsed := range nodes {
			name := parsed.name
			if name == "" {
				name = fmt.Sprintf("%s %s:%d", parsed.protocol, parsed.host, parsed.port)
			}
			node := model.ProxyClientNode{Name: name, Protocol: parsed.protocol, Host: parsed.host, Port: parsed.port, Link: parsed.link, SortOrder: int(count)}
			if err := tx.Create(&node).Error; err != nil {
				return err
			}
			count++
		}
		return nil
	})
	proxyClientMu.Unlock()
	if err != nil {
		return nil, err
	}
	return s.State()
}

// DeleteNodes removes nodes added by hand or from a subscription.
func (s *ProxyClientService) DeleteNodes(ids []uint) (*ProxyClientState, error) {
	if len(ids) == 0 {
		return s.State()
	}
	proxyClientMu.Lock()
	err := database.GetDB().Where("id IN ?", ids).Delete(&model.ProxyClientNode{}).Error
	proxyClientMu.Unlock()
	if err != nil {
		return nil, err
	}
	s.reapplyIfNodesChanged()
	return s.State()
}

// reapplyIfNodesChanged restarts sing-box when the nodes in use changed.
func (s *ProxyClientService) reapplyIfNodesChanged() {
	settings := loadProxyClientSettings(database.GetDB())
	if !settings.Enabled || corePtr == nil || !corePtr.IsRunning() {
		return
	}
	nodes := proxyClientSelectedNodes(database.GetDB(), settings)
	running := corePtr.HasOutbound(proxyClientProxyTag)
	if len(nodes) == 0 && !running {
		return
	}
	if running && len(nodes) > 0 && proxyClientRunningMatches(nodes) {
		return
	}
	if err := s.ConfigService.RestartCore(); err != nil {
		logger.Warning("apply proxy client nodes: ", err)
	}
}

// proxyClientRunning remembers what the running config was built from.
var proxyClientRunning atomic.Value // string

func proxyClientNodesFingerprint(nodes []model.ProxyClientNode) string {
	var builder strings.Builder
	for _, node := range nodes {
		builder.WriteString(strconv.FormatUint(uint64(node.Id), 10))
		builder.WriteString(":")
		builder.WriteString(node.Link)
		builder.WriteString(node.Outbound)
		builder.WriteString("\n")
	}
	return builder.String()
}

func proxyClientRunningMatches(nodes []model.ProxyClientNode) bool {
	running, _ := proxyClientRunning.Load().(string)
	return running == proxyClientNodesFingerprint(nodes)
}

// proxyClientTests counts latency tests in progress.
var proxyClientTests atomic.Int32

func proxyClientTesting() bool { return proxyClientTests.Load() > 0 }

// TestNodes measures nodes (all with no ids): a TCP connection to the
// server and a real request through the node, like v2rayN's real delay.
func (s *ProxyClientService) TestNodes(ids []uint) (*ProxyClientState, error) {
	db := database.GetDB()
	var nodes []model.ProxyClientNode
	query := db.Order("id ASC")
	if len(ids) > 0 {
		query = query.Where("id IN ?", ids)
	}
	if err := query.Find(&nodes).Error; err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return s.State()
	}
	if !proxyClientTests.CompareAndSwap(0, 1) {
		return nil, common.NewError("a latency test is already running")
	}
	defer proxyClientTests.Store(0)
	settings := loadProxyClientSettings(db)
	for start := 0; start < len(nodes); start += maxProxyProbes {
		batch := nodes[start:min(start+maxProxyProbes, len(nodes))]
		request := ProxyProbeRequest{Probes: make([]proxyprobe.Spec, len(batch))}
		for i, node := range batch {
			request.Probes[i] = clientNodeSpec(node, settings.TestURL)
		}
		response, err := RunProxyProbes(request)
		if err != nil {
			return nil, err
		}
		now := time.Now().Unix()
		for i, result := range response.Results {
			updates := map[string]interface{}{"tcp_ms": result.ConnectMs, "delay_ms": int64(0), "tested_at": now, "test_error": ""}
			if result.OK {
				updates["delay_ms"] = result.LatencyMs
			} else {
				updates["test_error"] = truncate(result.Error, 255)
			}
			db.Model(&model.ProxyClientNode{}).Where("id = ?", batch[i].Id).Updates(updates)
		}
	}
	return s.State()
}

// clientNodeSpec checks a client node through the same outbound the client
// would use; SOCKS5 and HTTP nodes included.
func clientNodeSpec(node model.ProxyClientNode, target string) proxyprobe.Spec {
	spec := proxyprobe.Spec{Type: proxyprobe.TypeNode, Link: node.Link, Host: node.Host, Port: node.Port, Target: target}
	if outbound, err := clientNodeOutbound(node); err == nil {
		delete(outbound, "domain_resolver")
		spec.Outbound = outbound
	}
	return spec
}

// clientNodeOutbound builds the sing-box outbound of a node.
func clientNodeOutbound(node model.ProxyClientNode) (map[string]interface{}, error) {
	var outbound map[string]interface{}
	if node.Outbound != "" {
		if err := json.Unmarshal([]byte(node.Outbound), &outbound); err != nil {
			return nil, err
		}
	} else {
		parsed, _, err := util.GetOutbound(node.Link, 0)
		if err != nil {
			return nil, err
		}
		outbound = *parsed
	}
	outbound["tag"] = proxyClientNodePrefix + strconv.FormatUint(uint64(node.Id), 10)
	delete(outbound, "detour")
	outbound["domain_resolver"] = proxyClientDNSDirectTag
	return outbound, nil
}
