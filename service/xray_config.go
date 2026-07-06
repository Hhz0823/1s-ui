package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Hhz0823/1s-ui/config"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util/common"

	"gorm.io/gorm"
)

type XrayConfig struct {
	Log       map[string]interface{}   `json:"log,omitempty"`
	Inbounds  []map[string]interface{} `json:"inbounds"`
	Outbounds []map[string]interface{} `json:"outbounds"`
	Routing   map[string]interface{}   `json:"routing,omitempty"`
}

func (s *ConfigService) HasXrayInbounds() (bool, error) {
	var count int64
	err := database.GetDB().Model(model.Inbound{}).
		Where("core_type = ?", model.CoreTypeXray).
		Count(&count).Error
	return count > 0, err
}

func (s *ConfigService) GetXrayConfig() (*[]byte, error) {
	inbounds, err := s.InboundService.GetAllXrayConfig(database.GetDB())
	if err != nil {
		return nil, err
	}

	xrayConfig := XrayConfig{
		Log: map[string]interface{}{
			"loglevel": "warning",
		},
		Inbounds: inbounds,
		Outbounds: []map[string]interface{}{
			{
				"protocol": "freedom",
				"tag":      "direct",
			},
			{
				"protocol": "blackhole",
				"tag":      "block",
			},
		},
		Routing: map[string]interface{}{
			"rules": []interface{}{},
		},
	}

	rawConfig, err := json.MarshalIndent(xrayConfig, "", "  ")
	if err != nil {
		return nil, err
	}
	return &rawConfig, nil
}

func (s *InboundService) GetAllXrayConfig(db *gorm.DB) ([]map[string]interface{}, error) {
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Preload("Tls").
		Where("core_type = ?", model.CoreTypeXray).
		Find(&inbounds).Error
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(inbounds))
	for _, inbound := range inbounds {
		switch inbound.Type {
		case "vless":
			config, err := s.buildXrayVlessInbound(db, inbound)
			if err != nil {
				return nil, err
			}
			result = append(result, config)
		default:
			return nil, common.NewErrorf("xray inbound type <%s> is not supported yet", inbound.Type)
		}
	}
	return result, nil
}

func (s *InboundService) buildXrayVlessInbound(db *gorm.DB, inbound *model.Inbound) (map[string]interface{}, error) {
	full, err := inbound.MarshalFull()
	if err != nil {
		return nil, err
	}

	listen, _ := (*full)["listen"].(string)
	if listen == "" {
		listen = "0.0.0.0"
	}
	port := toInt((*full)["listen_port"])
	if port == 0 {
		return nil, common.NewErrorf("xray inbound <%s> missing listen_port", inbound.Tag)
	}

	transport, _ := (*full)["transport"].(map[string]interface{})
	network := "xhttp"
	if tp, ok := transport["type"].(string); ok && tp != "" {
		network = tp
	}

	clients, err := s.fetchXrayVlessClients(db, inbound.Id, network)
	if err != nil {
		return nil, err
	}

	streamSettings, err := buildXrayStreamSettings(inbound, transport, network)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tag":      inbound.Tag,
		"listen":   listen,
		"port":     port,
		"protocol": "vless",
		"settings": map[string]interface{}{
			"clients":    clients,
			"decryption": "none",
		},
		"streamSettings": streamSettings,
	}, nil
}

func (s *InboundService) fetchXrayVlessClients(db *gorm.DB, inboundId uint, network string) ([]map[string]interface{}, error) {
	var users []struct {
		Name   string
		Config string
	}
	err := db.Raw(`SELECT name, json_extract(config, "$.vless") AS config
		FROM clients
		WHERE enable = true
			AND json_extract(config, "$.vless") IS NOT NULL
			AND ? IN (SELECT json_each.value FROM json_each(clients.inbounds))`, inboundId).Scan(&users).Error
	if err != nil {
		return nil, err
	}

	clients := make([]map[string]interface{}, 0, len(users))
	for _, user := range users {
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(user.Config), &cfg); err != nil {
			return nil, err
		}
		uuid, _ := cfg["uuid"].(string)
		if uuid == "" {
			continue
		}
		client := map[string]interface{}{
			"id":    uuid,
			"email": user.Name,
		}
		if network == "tcp" {
			if flow, _ := cfg["flow"].(string); flow != "" {
				client["flow"] = flow
			}
		}
		clients = append(clients, client)
	}
	return clients, nil
}

func buildXrayStreamSettings(inbound *model.Inbound, transport map[string]interface{}, network string) (map[string]interface{}, error) {
	stream := map[string]interface{}{
		"network": network,
	}

	switch network {
	case "xhttp":
		stream["xhttpSettings"] = map[string]interface{}{
			"path": stringValue(transport["path"], "/xhttp"),
			"host": stringValue(transport["host"], ""),
			"mode": stringValue(transport["mode"], "auto"),
		}
	case "ws":
		stream["wsSettings"] = map[string]interface{}{
			"path": stringValue(transport["path"], "/"),
			"headers": map[string]interface{}{
				"Host": stringValue(transport["host"], ""),
			},
		}
	case "grpc":
		stream["grpcSettings"] = map[string]interface{}{
			"serviceName": stringValue(transport["service_name"], ""),
		}
	case "httpupgrade":
		stream["httpupgradeSettings"] = map[string]interface{}{
			"path": stringValue(transport["path"], "/"),
			"host": stringValue(transport["host"], ""),
		}
	case "tcp", "raw":
		stream["network"] = "tcp"
	default:
		return nil, common.NewErrorf("xray transport <%s> is not supported yet", network)
	}

	if inbound.Tls != nil && len(inbound.Tls.Server) > 2 {
		if err := addXraySecurity(stream, inbound.Tls); err != nil {
			return nil, err
		}
	}

	return stream, nil
}

func addXraySecurity(stream map[string]interface{}, tlsConfig *model.Tls) error {
	var server map[string]interface{}
	if err := json.Unmarshal(tlsConfig.Server, &server); err != nil {
		return err
	}
	if enabled, ok := server["enabled"].(bool); ok && !enabled {
		return nil
	}

	if reality, ok := server["reality"].(map[string]interface{}); ok {
		if enabled, _ := reality["enabled"].(bool); enabled {
			stream["security"] = "reality"
			stream["realitySettings"] = buildXrayRealitySettings(server, reality)
			return nil
		}
	}

	stream["security"] = "tls"
	tlsSettings, err := buildXrayTLSSettings(tlsConfig, server)
	if err != nil {
		return err
	}
	stream["tlsSettings"] = tlsSettings
	return nil
}

func buildXrayRealitySettings(server map[string]interface{}, reality map[string]interface{}) map[string]interface{} {
	settings := map[string]interface{}{
		"show": false,
	}
	if privateKey, _ := reality["private_key"].(string); privateKey != "" {
		settings["privateKey"] = privateKey
	}
	if shortIds := toStringSlice(reality["short_id"]); len(shortIds) > 0 {
		settings["shortIds"] = shortIds
	}
	if serverName, _ := server["server_name"].(string); serverName != "" {
		settings["serverNames"] = []string{serverName}
	}
	if handshake, ok := reality["handshake"].(map[string]interface{}); ok {
		host, _ := handshake["server"].(string)
		port := toInt(handshake["server_port"])
		if host != "" && port > 0 {
			settings["dest"] = fmt.Sprintf("%s:%d", host, port)
		}
	}
	return settings
}

func buildXrayTLSSettings(tlsConfig *model.Tls, server map[string]interface{}) (map[string]interface{}, error) {
	settings := map[string]interface{}{}
	if serverName, _ := server["server_name"].(string); serverName != "" {
		settings["serverName"] = serverName
	}
	if alpn := toStringSlice(server["alpn"]); len(alpn) > 0 {
		settings["alpn"] = alpn
	}
	if minVersion, _ := server["min_version"].(string); minVersion != "" {
		settings["minVersion"] = minVersion
	}
	if maxVersion, _ := server["max_version"].(string); maxVersion != "" {
		settings["maxVersion"] = maxVersion
	}

	cert := map[string]interface{}{}
	certFile, _ := server["certificate_path"].(string)
	keyFile, _ := server["key_path"].(string)
	if certFile == "" || keyFile == "" {
		var err error
		certFile, keyFile, err = writeXrayCertificateFiles(tlsConfig, server)
		if err != nil {
			return nil, err
		}
	}
	if certFile != "" {
		cert["certificateFile"] = certFile
	}
	if keyFile != "" {
		cert["keyFile"] = keyFile
	}
	if len(cert) > 0 {
		settings["certificates"] = []map[string]interface{}{cert}
	}
	return settings, nil
}

func writeXrayCertificateFiles(tlsConfig *model.Tls, server map[string]interface{}) (string, string, error) {
	certificate := strings.Join(toStringSlice(server["certificate"]), "\n")
	key := strings.Join(toStringSlice(server["key"]), "\n")
	if certificate == "" || key == "" {
		return "", "", nil
	}

	dir := filepath.Join(config.GetBinFolderPath(), "xray-certs")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", "", err
	}
	baseName := sanitizeFileName(fmt.Sprintf("%d-%s", tlsConfig.Id, tlsConfig.Name))
	certFile := filepath.Join(dir, baseName+".crt")
	keyFile := filepath.Join(dir, baseName+".key")
	if err := os.WriteFile(certFile, []byte(certificate+"\n"), 0600); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(keyFile, []byte(key+"\n"), 0600); err != nil {
		return "", "", err
	}
	return certFile, keyFile, nil
}

func toInt(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case uint:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	default:
		return 0
	}
}

func toStringSlice(value interface{}) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		return result
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	default:
		return nil
	}
}

func stringValue(value interface{}, fallback string) string {
	if s, ok := value.(string); ok && s != "" {
		return s
	}
	return fallback
}

func sanitizeFileName(value string) string {
	re := regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	value = re.ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}
