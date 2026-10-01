package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/proxyprobe"
	"github.com/Hhz0823/1s-ui/util"
	"github.com/Hhz0823/1s-ui/util/common"
)

// NodeLinkInfo is what a share link says about its node, without secrets.
type NodeLinkInfo struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
}

// nodeSchemes are the share links checked through a sing-box outbound;
// socks5:// and http:// links are checked directly as proxies instead.
var nodeSchemes = map[string]bool{
	"vmess": true, "vless": true, "trojan": true, "ss": true, "shadowsocks": true,
	"hy2": true, "hysteria2": true, "hy": true, "hysteria": true, "tuic": true,
	"anytls": true, "https": true, "naive+https": true, "naive+quic": true, "http2": true,
}

// IsNodeLink reports whether a link is a node share link (VLESS, VMess …).
func IsNodeLink(link string) bool {
	scheme, _, ok := strings.Cut(strings.TrimSpace(link), "://")
	return ok && nodeSchemes[strings.ToLower(scheme)]
}

// ParseNodeLink reads a node share link the way the check will use it.
func ParseNodeLink(link string) (map[string]interface{}, NodeLinkInfo, error) {
	link = strings.TrimSpace(link)
	if len(link) > proxyprobe.MaxLinkLength {
		return nil, NodeLinkInfo{}, common.NewError("share link is too long")
	}
	if !IsNodeLink(link) {
		return nil, NodeLinkInfo{}, common.NewError("unsupported share link; use vless://, vmess://, trojan://, ss://, hy2://, tuic://, anytls:// or a naive link")
	}
	parsed, name, err := util.GetOutbound(link, 0)
	if err != nil || parsed == nil {
		if err == nil {
			err = common.NewError("unsupported share link")
		}
		return nil, NodeLinkInfo{}, common.NewError("invalid share link: ", err.Error())
	}
	outbound := *parsed
	info := NodeLinkInfo{Name: strings.TrimSpace(name)}
	info.Protocol, _ = outbound["type"].(string)
	info.Host, _ = outbound["server"].(string)
	info.Port = linkPort(outbound["server_port"])
	if info.Host == "" {
		return nil, NodeLinkInfo{}, common.NewError("the share link has no server address")
	}
	return outbound, info, nil
}

// nodeFromOutbound copies a sing-box outbound object for a check.
func nodeFromOutbound(source map[string]interface{}) (map[string]interface{}, NodeLinkInfo, error) {
	raw, err := json.Marshal(source)
	if err != nil {
		return nil, NodeLinkInfo{}, err
	}
	var outbound map[string]interface{}
	if err := json.Unmarshal(raw, &outbound); err != nil {
		return nil, NodeLinkInfo{}, err
	}
	info := NodeLinkInfo{}
	info.Name, _ = outbound["tag"].(string)
	info.Protocol, _ = outbound["type"].(string)
	info.Host, _ = outbound["server"].(string)
	info.Port = linkPort(outbound["server_port"])
	switch info.Protocol {
	case "", "direct", "block", "dns", "selector", "urltest":
		return nil, NodeLinkInfo{}, common.NewError("not a proxy node")
	}
	if info.Host == "" {
		return nil, NodeLinkInfo{}, common.NewError("the node has no server address")
	}
	return outbound, info, nil
}

// linkPort reads a port number from a parsed outbound.
func linkPort(value interface{}) int {
	switch number := value.(type) {
	case int:
		return number
	case int64:
		return int(number)
	case uint16:
		return int(number)
	case float64:
		return int(number)
	case string:
		parsed, _ := strconv.Atoi(number)
		return parsed
	}
	return 0
}

// nodeUsesTCP reports whether a node is reached over TCP, so a plain TCP
// connection can tell a blocked address from a failing protocol. QUIC
// protocols (Hysteria, TUIC, naive over QUIC) are skipped.
func nodeUsesTCP(outbound map[string]interface{}) bool {
	switch outbound["type"] {
	case "hysteria", "hysteria2", "tuic":
		return false
	}
	if quic, _ := outbound["quic"].(bool); quic {
		return false
	}
	if transport, ok := outbound["transport"].(map[string]interface{}); ok && transport["type"] == "quic" {
		return false
	}
	return true
}

type nodeProbe struct {
	index    int
	tag      string
	outbound map[string]interface{}
	server   string
	target   string
}

// runNodeProbes checks the node specs at indexes through one throwaway
// sing-box instance; when sing-box rejects the set, each node gets its own
// instance so one bad link cannot fail the others.
func runNodeProbes(specs []proxyprobe.Spec, indexes []int, results []proxyprobe.Result, slots chan struct{}) {
	now := time.Now().Unix()
	var probes []nodeProbe
	for n, i := range indexes {
		spec := specs[i]
		if err := proxyprobe.Normalize(&spec); err != nil {
			results[i] = proxyprobe.Result{Time: now, Stage: proxyprobe.StageConfig, Error: err.Error()}
			continue
		}
		var outbound map[string]interface{}
		var info NodeLinkInfo
		var err error
		if len(spec.Outbound) > 0 {
			outbound, info, err = nodeFromOutbound(spec.Outbound)
		} else {
			outbound, info, err = ParseNodeLink(spec.Link)
		}
		if err == nil {
			err = core.ValidateOutbound(outbound)
		}
		if err == nil {
			err = proxyprobe.CheckNodeHost(context.Background(), info.Host)
		}
		if err != nil {
			results[i] = proxyprobe.Result{Time: now, Stage: proxyprobe.StageConfig, Error: truncate(err.Error(), 200)}
			continue
		}
		probe := nodeProbe{index: i, tag: fmt.Sprintf("probe-%d", n), outbound: outbound, target: spec.Target}
		outbound["tag"] = probe.tag
		delete(outbound, "detour")
		if nodeUsesTCP(outbound) && info.Port > 0 {
			probe.server = net.JoinHostPort(info.Host, strconv.Itoa(info.Port))
		}
		probes = append(probes, probe)
	}
	if len(probes) == 0 {
		return
	}
	outbounds := make([]map[string]interface{}, len(probes))
	for i, probe := range probes {
		outbounds[i] = probe.outbound
	}
	shared, sharedErr := core.StartProbeBox(outbounds)
	if shared != nil {
		defer shared.Close()
	}
	var wg sync.WaitGroup
	for _, probe := range probes {
		wg.Add(1)
		slots <- struct{}{}
		go func(probe nodeProbe) {
			defer func() { <-slots; wg.Done() }()
			box := shared
			if sharedErr != nil {
				own, err := core.StartProbeBox([]map[string]interface{}{probe.outbound})
				if err != nil {
					results[probe.index] = proxyprobe.Result{Time: time.Now().Unix(), Stage: proxyprobe.StageConfig, Error: truncate("sing-box rejected the node: "+err.Error(), 200)}
					return
				}
				defer own.Close()
				box = own
			}
			results[probe.index] = proxyprobe.RunDialer(context.Background(), probe.target, probe.server, func(ctx context.Context, host, port string) (net.Conn, error) {
				return box.Dial(ctx, probe.tag, host, port)
			})
		}(probe)
	}
	wg.Wait()
}
