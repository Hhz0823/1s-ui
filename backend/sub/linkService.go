package sub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/util"
)

type Link struct {
	Type   string `json:"type"`
	Remark string `json:"remark"`
	Uri    string `json:"uri"`
}

type LinkService struct {
}

func (s *LinkService) GetLinks(linkJson *json.RawMessage, types string, clientInfo string) []string {
	links := []Link{}
	var result []string
	err := json.Unmarshal(*linkJson, &links)
	if err != nil {
		return nil
	}
	for _, link := range links {
		switch link.Type {
		case "external":
			result = append(result, link.Uri)
		case "sub":
			subLinks := util.GetExternalLink(link.Uri)
			for _, subLink := range strings.Split(subLinks, "\n") {
				if subLink = strings.TrimSpace(subLink); subLink != "" {
					result = append(result, subLink)
				}
			}
		case "local":
			if types == "all" {
				result = append(result, s.addClientInfo(link.Uri, clientInfo))
			}
		}
	}
	return result
}

func (s *LinkService) GetExternalOutbounds(linkJson *json.RawMessage) ([]map[string]interface{}, []string) {
	links := []Link{}
	err := json.Unmarshal(*linkJson, &links)
	if err != nil {
		return nil, nil
	}

	var outbounds []map[string]interface{}
	var tags []string

	for _, link := range links {
		switch link.Type {
		case "external":
			outbound, tag, err := util.GetOutbound(link.Uri, 0)
			if err == nil && outbound != nil && len(tag) > 0 {
				outbounds = append(outbounds, *outbound)
				tags = append(tags, tag)
			}
		case "sub":
			subOutbounds, err := util.GetExternalSub(link.Uri)
			if err != nil {
				logger.Warning("sub: Error getting external sub:", err)
				continue
			}
			for _, outbound := range subOutbounds {
				if tag, _ := outbound["tag"].(string); len(tag) > 0 {
					outbounds = append(outbounds, outbound)
					tags = append(tags, tag)
				}
			}
		}
	}

	seen := make(map[string]int)
	for i, tag := range tags {
		if n := seen[tag]; n > 0 {
			newTag := fmt.Sprintf("%s-%d", tag, n)
			seen[tag] = n + 1
			tags[i] = newTag
			outbounds[i]["tag"] = newTag
		} else {
			seen[tag] = 1
		}
	}

	return outbounds, tags
}

func (s *LinkService) addClientInfo(uri string, clientInfo string) string {
	return AppendLinkRemark(uri, clientInfo)
}

// AppendLinkRemark appends text to the display name of a share link without
// breaking its encoding. Raw text after the URI (spaces, CJK) made v2rayN and
// Shadowrocket reject links such as socks:// that have no fragment.
func AppendLinkRemark(uri string, suffix string) string {
	if strings.TrimSpace(suffix) == "" {
		return uri
	}
	protocol := strings.SplitN(uri, "://", 2)
	if len(protocol) < 2 {
		return uri
	}
	switch protocol[0] {
	case "vmess":
		var vmessJson map[string]interface{}
		config, err := util.B64StrToByte(protocol[1])
		if err != nil {
			logger.Warning("sub: Error decoding vmess content:", err)
			return uri
		}
		err = json.Unmarshal(config, &vmessJson)
		if err != nil {
			logger.Warning("sub: Error decoding vmess content:", err)
			return uri
		}
		remark, _ := vmessJson["ps"].(string)
		vmessJson["ps"] = remark + suffix
		result, err := json.Marshal(vmessJson)
		if err != nil {
			logger.Warning("sub: Error decoding vmess + clientInfo content:", err)
			return uri
		}
		return "vmess://" + util.ByteToB64Str(result)
	case "v2rayn":
		return appendV2rayNProfileRemark(uri, suffix)
	default:
		base, fragment, _ := strings.Cut(uri, "#")
		remark, err := url.PathUnescape(fragment)
		if err != nil {
			remark = fragment
		}
		return base + "#" + url.PathEscape(remark+suffix)
	}
}

func appendV2rayNProfileRemark(uri string, suffix string) string {
	prefixEnd := strings.LastIndex(uri, "/")
	if prefixEnd < 0 {
		return uri
	}
	payload, err := base64.RawURLEncoding.DecodeString(uri[prefixEnd+1:])
	if err != nil {
		return uri
	}
	var profile map[string]interface{}
	if err = json.Unmarshal(payload, &profile); err != nil {
		return uri
	}
	remark, _ := profile["Remarks"].(string)
	profile["Remarks"] = remark + suffix
	updated, err := json.Marshal(profile)
	if err != nil {
		return uri
	}
	return uri[:prefixEnd+1] + base64.RawURLEncoding.EncodeToString(updated)
}
