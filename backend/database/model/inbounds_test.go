package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

func TestTUICMarshalJSONAddsDefaultH3ALPN(t *testing.T) {
	inbound := Inbound{
		Type:    "tuic",
		Tag:     "tuic-test",
		Options: json.RawMessage(`{"listen":"::","listen_port":443}`),
		Tls: &Tls{
			Server: json.RawMessage(`{"enabled":true}`),
		},
	}

	raw, err := inbound.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal TUIC inbound: %v", err)
	}
	var config map[string]interface{}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatalf("unmarshal TUIC config: %v", err)
	}
	tlsConfig, ok := config["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing TLS config in %s", raw)
	}
	alpn, ok := tlsConfig["alpn"].([]interface{})
	if !ok || len(alpn) != 1 || alpn[0] != "h3" {
		t.Fatalf("unexpected TUIC ALPN %#v", tlsConfig["alpn"])
	}
}

func TestInboundBandwidthLimitsStayOutOfCoreJSON(t *testing.T) {
	var inbound Inbound
	if err := json.Unmarshal([]byte(`{"type":"mixed","tag":"limited","listen":"::","listen_port":1080,"upload_limit":1024,"download_limit":2048}`), &inbound); err != nil {
		t.Fatal(err)
	}
	if inbound.UploadLimit != 1024 || inbound.DownloadLimit != 2048 {
		t.Fatalf("limits were not extracted: %#v", inbound)
	}
	if bytes.Contains(inbound.Options, []byte("upload_limit")) || bytes.Contains(inbound.Options, []byte("download_limit")) {
		t.Fatalf("limits leaked into stored core options: %s", inbound.Options)
	}
	coreJSON, err := inbound.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(coreJSON, []byte("upload_limit")) || bytes.Contains(coreJSON, []byte("download_limit")) {
		t.Fatalf("limits leaked into sing-box JSON: %s", coreJSON)
	}
	full, err := inbound.MarshalFull()
	if err != nil {
		t.Fatal(err)
	}
	if (*full)["upload_limit"] != int64(1024) || (*full)["download_limit"] != int64(2048) {
		t.Fatalf("full API model omitted limits: %#v", *full)
	}
	inbound.Options = json.RawMessage(`{"listen_port":1080,"upload_limit":9999,"download_limit":9999}`)
	coreJSON, err = inbound.MarshalJSON()
	if err != nil || bytes.Contains(coreJSON, []byte("upload_limit")) || bytes.Contains(coreJSON, []byte("download_limit")) {
		t.Fatalf("legacy option limits leaked into core JSON: %s err=%v", coreJSON, err)
	}
	full, err = inbound.MarshalFull()
	if err != nil || (*full)["upload_limit"] != int64(1024) || (*full)["download_limit"] != int64(2048) {
		t.Fatalf("legacy options overrode model limits: %#v err=%v", full, err)
	}
}

func TestInboundBandwidthLimitValidation(t *testing.T) {
	for _, value := range []string{"-1", "1.5", fmt.Sprint(MaxInboundBandwidthLimit + 1)} {
		var inbound Inbound
		raw := []byte(`{"type":"mixed","tag":"bad","upload_limit":` + value + `}`)
		if err := json.Unmarshal(raw, &inbound); err == nil {
			t.Fatalf("accepted invalid upload_limit %s", value)
		}
	}
}

func TestTUICMarshalJSONPreservesExplicitALPN(t *testing.T) {
	inbound := Inbound{
		Type:    "tuic",
		Tag:     "tuic-test",
		Options: json.RawMessage(`{"listen":"::","listen_port":443}`),
		Tls: &Tls{
			Server: json.RawMessage(`{"enabled":true,"alpn":["custom-tuic"]}`),
		},
	}

	raw, err := inbound.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal TUIC inbound: %v", err)
	}
	var config map[string]interface{}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatalf("unmarshal TUIC config: %v", err)
	}
	tlsConfig := config["tls"].(map[string]interface{})
	alpn := tlsConfig["alpn"].([]interface{})
	if len(alpn) != 1 || alpn[0] != "custom-tuic" {
		t.Fatalf("explicit TUIC ALPN was replaced: %#v", alpn)
	}
}
