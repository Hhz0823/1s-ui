package sub

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/service"
	"github.com/Hhz0823/1s-ui/util"
)

const defaultJson = `
{
  "inbounds": [
    {
      "type": "tun",
      "address": [
				"172.19.0.1/30",
				"fdfe:dcba:9876::1/126"
			],
      "mtu": 9000,
      "auto_route": true,
      "strict_route": false,
      "endpoint_independent_nat": false,
      "platform": {
        "http_proxy": {
          "enabled": true,
          "server": "127.0.0.1",
          "server_port": 2080
        }
      }
    },
    {
      "type": "mixed",
      "listen": "127.0.0.1",
      "listen_port": 2080,
      "users": []
    }
  ]
}
`

type JsonService struct {
	service.SettingService
	LinkService
}

func (j *JsonService) GetJson(subId string, format string) (*string, []string, error) {
	var jsonConfig map[string]interface{}

	client, inDatas, err := j.getData(subId)
	if err != nil {
		return nil, nil, err
	}

	outbounds, outTags, err := j.getOutbounds(client.Config, inDatas, localLinks(client.Links))
	if err != nil {
		return nil, nil, err
	}

	links := j.LinkService.GetLinks(&client.Links, "external", "")
	tagNumEnable := 0
	if len(links) > 1 {
		tagNumEnable = 1
	}
	for index, link := range links {
		json, tag, err := util.GetOutbound(link, (index+1)*tagNumEnable)
		if err == nil && len(tag) > 0 {
			*outbounds = append(*outbounds, *json)
			*outTags = append(*outTags, tag)
		}
	}

	// sing-box rejects a config whose urltest group is empty ("missing
	// tags"), so a client with no usable node gets a message instead.
	if len(*outTags) == 0 {
		return nil, nil, ErrNoSingBoxNodes
	}

	j.addDefaultOutbounds(outbounds, outTags)

	err = json.Unmarshal([]byte(defaultJson), &jsonConfig)
	if err != nil {
		return nil, nil, err
	}

	jsonConfig["outbounds"] = outbounds

	// Add other objects from settings
	j.addOthers(&jsonConfig)

	result, _ := json.MarshalIndent(jsonConfig, "", "  ")
	resultStr := string(result)

	updateInterval, _ := j.SettingService.GetSubUpdates()
	headers := util.GetHeaders(client, updateInterval)

	return &resultStr, headers, nil
}

func (j *JsonService) getData(subId string) (*model.Client, []*model.Inbound, error) {
	db := database.GetDB()
	client := &model.Client{}
	err := db.Model(model.Client{}).Where("enable = true and name = ?", subId).First(client).Error
	if err != nil {
		return nil, nil, err
	}
	var clientInbounds []uint
	err = json.Unmarshal(client.Inbounds, &clientInbounds)
	if err != nil {
		return nil, nil, err
	}
	var inbounds []*model.Inbound
	err = db.Model(model.Inbound{}).Preload("Tls").Where("id in ?", clientInbounds).Find(&inbounds).Error
	if err != nil {
		return nil, nil, err
	}
	return client, inbounds, nil
}

// ErrNoSingBoxNodes means none of a client's nodes can run in sing-box.
var ErrNoSingBoxNodes = errors.New("这个用户没有 sing-box 能用的节点：XHTTP 和 VLESS Encryption 节点只有 Xray-core 客户端（v2rayN、v2rayNG 等）支持，请改用这些客户端导入订阅链接。\n" +
	"None of this client's nodes work in sing-box: XHTTP and VLESS Encryption nodes need an Xray-core client such as v2rayN or v2rayNG.")

// localLinks maps an inbound tag to the client's share links for it.
func localLinks(raw json.RawMessage) map[string][]string {
	var links []Link
	if json.Unmarshal(raw, &links) != nil {
		return nil
	}
	result := map[string][]string{}
	for _, link := range links {
		if link.Type == "local" && link.Uri != "" {
			result[link.Remark] = append(result[link.Remark], link.Uri)
		}
	}
	return result
}

func (j *JsonService) getOutbounds(clientConfig json.RawMessage, inbounds []*model.Inbound, links map[string][]string) (*[]map[string]interface{}, *[]string, error) {
	var outbounds []map[string]interface{}
	var configs map[string]interface{}
	var outTags []string

	err := json.Unmarshal(clientConfig, &configs)
	if err != nil {
		return nil, nil, err
	}
	var xrayLinks []string
	for _, inData := range inbounds {
		if inData.RuntimeCore() == model.CoreTypeXray {
			// An Xray inbound has no sing-box outbound of its own, but its
			// share links often do; they are converted below.
			xrayLinks = append(xrayLinks, links[inData.Tag]...)
			continue
		}
		if len(inData.OutJson) < 5 {
			continue
		}
		var outbound map[string]interface{}
		err = json.Unmarshal(inData.OutJson, &outbound)
		if err != nil {
			return nil, nil, err
		}
		protocol, _ := outbound["type"].(string)

		// Shadowsocks
		if protocol == "shadowsocks" {
			var userPass []string
			var inbOptions map[string]interface{}
			err = json.Unmarshal(inData.Options, &inbOptions)
			if err != nil {
				return nil, nil, err
			}
			method, _ := inbOptions["method"].(string)
			if strings.HasPrefix(method, "2022") {
				inbPass, _ := inbOptions["password"].(string)
				userPass = append(userPass, inbPass)
			}
			key := "shadowsocks"
			if method == "2022-blake3-aes-128-gcm" {
				key = "shadowsocks16"
			}
			userConfig, _ := configs[key].(map[string]interface{})
			pass, _ := userConfig["password"].(string)
			// Single-user 2022 inbounds take the server key alone.
			if pass != "" || len(userPass) == 0 {
				userPass = append(userPass, pass)
			}
			outbound["password"] = strings.Join(userPass, ":")
		} else { // Other protocols
			config, _ := configs[protocol].(map[string]interface{})
			for key, value := range config {
				if key == "name" || key == "alterId" || (key == "flow" && inData.TlsId == 0) {
					continue
				}
				outbound[key] = value
			}
		}

		var addrs []map[string]interface{}
		err = json.Unmarshal(inData.Addrs, &addrs)
		if err != nil {
			return nil, nil, err
		}
		util.NormalizeSingBoxOutbound(outbound)
		tag, _ := outbound["tag"].(string)
		if len(addrs) == 0 {
			// For mixed protocol, use separated socks and http
			if protocol == "mixed" {
				outbound["tag"] = tag
				j.pushMixed(&outbounds, &outTags, outbound)
			} else {
				outTags = append(outTags, tag)
				outbounds = append(outbounds, outbound)
			}
		} else {
			for index, addr := range addrs {
				// Copy original config
				newOut := make(map[string]interface{}, len(outbound))
				for key, value := range outbound {
					newOut[key] = value
				}
				// Change and push copied config
				newOut["server"], _ = addr["server"].(string)
				port, _ := addr["server_port"].(float64)
				newOut["server_port"] = int(port)

				// Override TLS
				if addrTls, ok := addr["tls"].(map[string]interface{}); ok {
					outTls, _ := newOut["tls"].(map[string]interface{})
					if outTls == nil {
						outTls = make(map[string]interface{})
					}
					for key, value := range addrTls {
						outTls[key] = value
					}
					newOut["tls"] = outTls
				}

				remark, _ := addr["remark"].(string)
				newTag := fmt.Sprintf("%d.%s%s", index+1, tag, remark)
				newOut["tag"] = newTag
				util.NormalizeSingBoxOutbound(newOut)
				// For mixed protocol, use separated socks and http
				if protocol == "mixed" {
					j.pushMixed(&outbounds, &outTags, newOut)
				} else {
					outTags = append(outTags, newTag)
					outbounds = append(outbounds, newOut)
				}
			}
		}
	}
	// Xray nodes sing-box can dial (REALITY or TLS over TCP, WebSocket or
	// gRPC); XHTTP and VLESS Encryption links fail to convert and are left out.
	used := map[string]bool{}
	for _, tag := range outTags {
		used[tag] = true
	}
	for _, uri := range xrayLinks {
		outbound, tag, err := util.GetOutbound(uri, 0)
		if err != nil || tag == "" {
			continue
		}
		tag = uniqueTag(tag, used)
		(*outbound)["tag"] = tag
		outTags = append(outTags, tag)
		outbounds = append(outbounds, *outbound)
	}
	return &outbounds, &outTags, nil
}

func uniqueTag(tag string, used map[string]bool) string {
	result := tag
	for n := 2; used[result]; n++ {
		result = fmt.Sprintf("%s-%d", tag, n)
	}
	used[result] = true
	return result
}

func (j *JsonService) addDefaultOutbounds(outbounds *[]map[string]interface{}, outTags *[]string) {
	outbound := []map[string]interface{}{
		{
			"outbounds": append([]string{"auto", "direct"}, *outTags...),
			"tag":       "proxy",
			"type":      "selector",
		},
		{
			"tag":       "auto",
			"type":      "urltest",
			"outbounds": outTags,
			"url":       "http://www.gstatic.com/generate_204",
			"interval":  "10m",
			"tolerance": 50,
		},
		{
			"type": "direct",
			"tag":  "direct",
		},
	}
	*outbounds = append(outbound, *outbounds...)
}

func (j *JsonService) addOthers(jsonConfig *map[string]interface{}) error {
	rules_start := []interface{}{
		map[string]interface{}{
			"action": "sniff",
		},
		map[string]interface{}{
			"clash_mode": "Direct",
			"action":     "route",
			"outbound":   "direct",
		},
	}
	rules_end := []interface{}{
		map[string]interface{}{
			"clash_mode": "Global",
			"action":     "route",
			"outbound":   "proxy",
		},
	}
	route := map[string]interface{}{
		"auto_detect_interface": true,
		"final":                 "proxy",
		"rules":                 rules_start,
	}

	othersStr, err := j.SettingService.GetSubJsonExt()
	if err != nil {
		return err
	}
	if len(othersStr) == 0 {
		(*jsonConfig)["route"] = route
		return nil
	}
	var othersJson map[string]interface{}
	err = json.Unmarshal([]byte(othersStr), &othersJson)
	if err != nil {
		return err
	}
	if _, ok := othersJson["log"]; ok {
		(*jsonConfig)["log"] = othersJson["log"]
	}
	if _, ok := othersJson["dns"]; ok {
		(*jsonConfig)["dns"] = othersJson["dns"]
	}
	if _, ok := othersJson["inbounds"]; ok {
		(*jsonConfig)["inbounds"] = othersJson["inbounds"]
	}
	if _, ok := othersJson["experimental"]; ok {
		(*jsonConfig)["experimental"] = othersJson["experimental"]
	}
	if _, ok := othersJson["rule_set"]; ok {
		route["rule_set"] = othersJson["rule_set"]
	}
	if settingRules, ok := othersJson["rules"].([]interface{}); ok {
		rules := append(rules_start, settingRules...)
		route["rules"] = append(rules, rules_end...)
	}
	if defaultDomainResolver, ok := othersJson["default_domain_resolver"].(string); ok {
		route["default_domain_resolver"] = defaultDomainResolver
	}
	(*jsonConfig)["route"] = route

	return nil
}

func (j *JsonService) pushMixed(outbounds *[]map[string]interface{}, outTags *[]string, out map[string]interface{}) {
	socksOut := make(map[string]interface{}, 1)
	httpOut := make(map[string]interface{}, 1)
	for key, value := range out {
		socksOut[key] = value
		httpOut[key] = value
	}
	socksTag := fmt.Sprintf("%s-socks", out["tag"])
	httpTag := fmt.Sprintf("%s-http", out["tag"])
	socksOut["type"] = "socks"
	httpOut["type"] = "http"
	socksOut["tag"] = socksTag
	httpOut["tag"] = httpTag
	*outbounds = append(*outbounds, socksOut, httpOut)
	*outTags = append(*outTags, socksTag, httpTag)
}
