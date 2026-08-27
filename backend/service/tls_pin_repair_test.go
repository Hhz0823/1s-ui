package service

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util"

	boxtls "github.com/sagernet/sing-box/common/tls"
)

func TestRepairGeneratedTLSPins(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "tls-pin-repair.db")); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	privateKey, certificate, err := boxtls.GenerateCertificate(nil, nil, time.Now, "hy2.example", now.AddDate(1, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	serverConfig := map[string]interface{}{
		"enabled":     true,
		"key":         strings.Split(strings.TrimSpace(string(privateKey)), "\n"),
		"certificate": strings.Split(strings.TrimSpace(string(certificate)), "\n"),
	}
	wrongBase64 := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	wrongHex := strings.Repeat("0", 64)
	tlsConfig := model.Tls{
		Name:   "auto-hy2-test",
		Server: mustJSON(serverConfig),
		Client: mustJSON(map[string]interface{}{"pinned_peer_certificate_sha256": []string{wrongBase64}}),
	}
	if err = database.GetDB().Create(&tlsConfig).Error; err != nil {
		t.Fatal(err)
	}
	inbound := model.Inbound{
		Type: "hysteria2", Tag: "hy2-test", CoreType: model.CoreTypeSingBox, TlsId: tlsConfig.Id,
		Options: json.RawMessage(`{"listen":"0.0.0.0","listen_port":33305}`),
		Addrs:   json.RawMessage(`[]`),
		OutJson: mustJSON(map[string]interface{}{
			"server": "198.51.100.10", "server_port": 33305,
			"tls": map[string]interface{}{"enabled": true, "pinned_peer_certificate_sha256": []string{wrongBase64}},
		}),
	}
	if err = database.GetDB().Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	client := model.Client{
		Enable: true, Name: "hy2-user", Config: json.RawMessage(`{"hysteria2":{"password":"test"}}`),
		Inbounds: mustJSON([]uint{inbound.Id}),
		Links: mustJSON([]map[string]string{{
			"type": "local", "remark": inbound.Tag,
			"uri": "hysteria2://test@198.51.100.10:33305?security=tls&pinSHA256=" + wrongHex + "&fastopen=0#hy2-test",
		}}),
	}
	if err = database.GetDB().Create(&client).Error; err != nil {
		t.Fatal(err)
	}

	service := &ConfigService{}
	repaired, err := service.RepairGeneratedTLSPins()
	if err != nil {
		t.Fatal(err)
	}
	if repaired != 1 {
		t.Fatalf("repaired = %d, want 1", repaired)
	}
	wantBase64 := util.CertSha256Base64(string(certificate))
	wantHex := util.CertSha256Hex(string(certificate))

	if err = database.GetDB().First(&tlsConfig, tlsConfig.Id).Error; err != nil {
		t.Fatal(err)
	}
	assertStoredPin(t, tlsConfig.Client, wantBase64)
	if err = database.GetDB().First(&inbound, inbound.Id).Error; err != nil {
		t.Fatal(err)
	}
	var outbound map[string]interface{}
	if err = json.Unmarshal(inbound.OutJson, &outbound); err != nil {
		t.Fatal(err)
	}
	outboundTLS, _ := outbound["tls"].(map[string]interface{})
	encodedTLS, _ := json.Marshal(outboundTLS)
	assertStoredPin(t, encodedTLS, wantBase64)
	if err = database.GetDB().First(&client, client.Id).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(client.Links), "pinSHA256="+wantHex) || strings.Contains(string(client.Links), wrongHex) {
		t.Fatalf("client links were not repaired: %s", client.Links)
	}
	if repaired, err = service.RepairGeneratedTLSPins(); err != nil || repaired != 0 {
		t.Fatalf("second repair = %d, %v; want idempotent zero repair", repaired, err)
	}
}

func TestRepairGeneratedTLSRebuildsNaiveV2rayNLink(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "naive-link-repair.db")); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	privateKey, certificate, err := util.GenerateSelfSignedTLS("node.example.com", now, now.AddDate(1, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	pinBase64 := util.CertSha256Base64(string(certificate))
	tlsConfig := model.Tls{
		Name: "auto-naive-test",
		Server: mustJSON(map[string]interface{}{
			"enabled": true, "server_name": "node.example.com",
			"key":         strings.Split(strings.TrimSpace(string(privateKey)), "\n"),
			"certificate": strings.Split(strings.TrimSpace(string(certificate)), "\n"),
		}),
		Client: mustJSON(map[string]interface{}{
			"certificate":                    strings.Split(strings.TrimSpace(string(certificate)), "\n"),
			"pinned_peer_certificate_sha256": []string{pinBase64},
		}),
	}
	if err = database.GetDB().Create(&tlsConfig).Error; err != nil {
		t.Fatal(err)
	}
	inbound := model.Inbound{
		Type: "naive", Tag: "naive-test", CoreType: model.CoreTypeSingBox, TlsId: tlsConfig.Id,
		Options: json.RawMessage(`{"listen":"0.0.0.0","listen_port":443,"network":"udp","quic_congestion_control":"bbr"}`),
		Addrs:   json.RawMessage(`[{"server":"198.51.100.20","server_port":443,"remark":""}]`),
		OutJson: json.RawMessage(`{"quic":true,"quic_congestion_control":"bbr","udp_over_tcp":{"enabled":true}}`),
	}
	if err = database.GetDB().Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	client := model.Client{
		Enable: true, Name: "naive-user",
		Config:   json.RawMessage(`{"naive":{"username":"browser","password":"strong-secret"}}`),
		Inbounds: mustJSON([]uint{inbound.Id}),
		Links: mustJSON([]map[string]string{{
			"type": "local", "remark": inbound.Tag,
			"uri": "naive+quic://browser:strong-secret@198.51.100.20:443?security=tls#naive-test",
		}}),
	}
	if err = database.GetDB().Create(&client).Error; err != nil {
		t.Fatal(err)
	}

	service := &ConfigService{}
	repaired, err := service.RepairGeneratedTLSPins()
	if err != nil {
		t.Fatal(err)
	}
	if repaired != 1 {
		t.Fatalf("repaired = %d, want 1", repaired)
	}
	if err = database.GetDB().First(&client, client.Id).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(client.Links), "v2rayn://naive/") || strings.Contains(string(client.Links), "naive+quic://") {
		t.Fatalf("Naive links were not securely rebuilt: %s", client.Links)
	}
	if repaired, err = service.RepairGeneratedTLSPins(); err != nil || repaired != 0 {
		t.Fatalf("second repair = %d, %v; want idempotent zero repair", repaired, err)
	}
}

func assertStoredPin(t *testing.T, raw json.RawMessage, want string) {
	t.Helper()
	var config map[string]interface{}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	pins, _ := config["pinned_peer_certificate_sha256"].([]interface{})
	if len(pins) != 1 || pins[0] != want {
		t.Fatalf("stored pin = %#v, want %q", pins, want)
	}
}
