package service

import (
	"encoding/json"
	"strings"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util"

	"gorm.io/gorm"
)

const generatedTLSNamePrefix = "auto-"

func normalizeGeneratedTLSPin(tlsConfig *model.Tls) (string, string, bool, error) {
	if tlsConfig == nil || !strings.HasPrefix(tlsConfig.Name, generatedTLSNamePrefix) {
		return "", "", false, nil
	}
	var serverConfig, clientConfig map[string]interface{}
	if err := json.Unmarshal(tlsConfig.Server, &serverConfig); err != nil {
		return "", "", false, nil
	}
	if err := json.Unmarshal(tlsConfig.Client, &clientConfig); err != nil {
		return "", "", false, nil
	}
	pemData := util.CertPEMFromTLS(serverConfig)
	pinBase64 := util.CertSha256Base64(pemData)
	pinHex := util.CertSha256Hex(pemData)
	if pinBase64 == "" || pinHex == "" {
		return "", "", false, nil
	}
	if pins, ok := clientConfig["pinned_peer_certificate_sha256"].([]interface{}); ok && len(pins) == 1 {
		if current, ok := pins[0].(string); ok && current == pinBase64 {
			return pinBase64, pinHex, false, nil
		}
	}
	clientConfig["pinned_peer_certificate_sha256"] = []string{pinBase64}
	updated, err := json.MarshalIndent(clientConfig, "", "  ")
	if err != nil {
		return "", "", false, err
	}
	tlsConfig.Client = updated
	return pinBase64, pinHex, true, nil
}

func setOutboundTLSPin(raw json.RawMessage, pinBase64 string) (json.RawMessage, bool, error) {
	var outbound map[string]interface{}
	if err := json.Unmarshal(raw, &outbound); err != nil {
		return raw, false, nil
	}
	tlsConfig, ok := outbound["tls"].(map[string]interface{})
	if !ok {
		return raw, false, nil
	}
	if pins, ok := tlsConfig["pinned_peer_certificate_sha256"].([]interface{}); ok && len(pins) == 1 {
		if current, ok := pins[0].(string); ok && current == pinBase64 {
			return raw, false, nil
		}
	}
	tlsConfig["pinned_peer_certificate_sha256"] = []string{pinBase64}
	updated, err := json.MarshalIndent(outbound, "", "  ")
	return updated, err == nil, err
}

func replaceLinkQueryValue(link, key, value string) (string, bool) {
	needle := key + "="
	index := strings.Index(link, needle)
	if index < 0 {
		return link, false
	}
	start := index + len(needle)
	end := len(link)
	if next := strings.IndexAny(link[start:], "&#"); next >= 0 {
		end = start + next
	}
	if link[start:end] == value {
		return link, false
	}
	return link[:start] + value + link[end:], true
}

func repairClientLinksForInboundTags(tx *gorm.DB, tags map[string]string) error {
	if len(tags) == 0 {
		return nil
	}
	var clients []model.Client
	if err := tx.Find(&clients).Error; err != nil {
		return err
	}
	for index := range clients {
		var links []map[string]string
		if err := json.Unmarshal(clients[index].Links, &links); err != nil {
			continue
		}
		changed := false
		for linkIndex := range links {
			pinHex, ok := tags[links[linkIndex]["remark"]]
			if !ok || links[linkIndex]["type"] != "local" {
				continue
			}
			updated, replaced := replaceLinkQueryValue(links[linkIndex]["uri"], "pinSHA256", pinHex)
			if replaced {
				links[linkIndex]["uri"] = updated
				changed = true
			}
		}
		if !changed {
			continue
		}
		updated, err := json.MarshalIndent(links, "", "  ")
		if err != nil {
			return err
		}
		if err = tx.Model(&model.Client{}).Where("id = ?", clients[index].Id).UpdateColumn("links", updated).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *ConfigService) RepairGeneratedTLSPins() (int, error) {
	repaired := 0
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		var tlsConfigs []model.Tls
		if err := tx.Where("name LIKE ?", generatedTLSNamePrefix+"%").Find(&tlsConfigs).Error; err != nil {
			return err
		}
		inboundTags := map[string]string{}
		for index := range tlsConfigs {
			pinBase64, pinHex, tlsChanged, err := normalizeGeneratedTLSPin(&tlsConfigs[index])
			if err != nil {
				return err
			}
			if pinBase64 == "" || pinHex == "" {
				continue
			}
			recordChanged := tlsChanged
			if tlsChanged {
				if err = tx.Model(&model.Tls{}).Where("id = ?", tlsConfigs[index].Id).UpdateColumn("client", tlsConfigs[index].Client).Error; err != nil {
					return err
				}
			}
			var inbounds []model.Inbound
			if err = tx.Where("tls_id = ?", tlsConfigs[index].Id).Find(&inbounds).Error; err != nil {
				return err
			}
			for inboundIndex := range inbounds {
				updated, didUpdate, updateErr := setOutboundTLSPin(inbounds[inboundIndex].OutJson, pinBase64)
				if updateErr != nil {
					return updateErr
				}
				if didUpdate {
					if err = tx.Model(&model.Inbound{}).Where("id = ?", inbounds[inboundIndex].Id).UpdateColumn("out_json", updated).Error; err != nil {
						return err
					}
					recordChanged = true
				}
				inboundTags[inbounds[inboundIndex].Tag] = pinHex
			}
			if recordChanged {
				repaired++
			}
		}
		return repairClientLinksForInboundTags(tx, inboundTags)
	})
	return repaired, err
}
