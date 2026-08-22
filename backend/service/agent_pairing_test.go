package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseAgentPairingLink(t *testing.T) {
	code := strings.Repeat("a", 43)
	endpoint, parsedCode, err := parseAgentPairingLink("https://panel.example/app/agent/v1/pair#" + code)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != "https://panel.example/app/agent/v1/pair" || parsedCode != code {
		t.Fatalf("unexpected parsed pairing link: endpoint=%q code=%q", endpoint, parsedCode)
	}
	enrollmentEndpoint, enrollmentCode, err := parseAgentPairingLink("https://panel.example/app/agent/v1/enroll#" + code)
	if err != nil {
		t.Fatal(err)
	}
	if enrollmentEndpoint != "https://panel.example/app/agent/v1/enroll" || enrollmentCode != code {
		t.Fatalf("unexpected parsed enrollment API: endpoint=%q code=%q", enrollmentEndpoint, enrollmentCode)
	}
	simpleEndpoint, simpleCode, err := parseAgentPairingLink("https://panel.example/app/#" + code)
	if err != nil {
		t.Fatal(err)
	}
	if simpleEndpoint != "https://panel.example/app/agent/v1/enroll" || simpleCode != code {
		t.Fatalf("unexpected simple enrollment address: endpoint=%q code=%q", simpleEndpoint, simpleCode)
	}
	addressEndpoint, addressCode, err := parseAgentPairingLink("https://panel.example/app/")
	if err != nil {
		t.Fatal(err)
	}
	if addressEndpoint != "https://panel.example/app/agent/v1/enroll" || addressCode != "" {
		t.Fatalf("unexpected address-only enrollment: endpoint=%q code=%q", addressEndpoint, addressCode)
	}

	invalid := []string{
		"",
		"ftp://panel.example/app/agent/v1/pair#" + code,
		"https://user@panel.example/app/agent/v1/pair#" + code,
		"https://panel.example/app/agent/v1/pair#short",
		"https://panel.example/app/agent/v1/pair",
	}
	for _, value := range invalid {
		if _, _, err := parseAgentPairingLink(value); err == nil {
			t.Fatalf("accepted invalid pairing link %q", value)
		}
	}
}

func TestAgentAddressPairingWindowIsSingleUse(t *testing.T) {
	settings := &SettingService{}
	t.Cleanup(settings.CloseAgentAddressPairing)
	expiresAt := settings.OpenAgentAddressPairing(42)
	if expiresAt <= 0 {
		t.Fatal("address pairing window has no expiry")
	}
	targetNodeID, err := settings.ConsumeAgentAddressPairing()
	if err != nil || targetNodeID != 42 {
		t.Fatalf("first address pairing claim failed: id=%d err=%v", targetNodeID, err)
	}
	if _, err := settings.ConsumeAgentAddressPairing(); err == nil {
		t.Fatal("address pairing window was accepted twice")
	}
}

func TestParseAgentEnvironmentDoesNotExposeUnrelatedValues(t *testing.T) {
	values := parseAgentEnvironment([]byte(`# managed connection
SUI_AGENT_PANEL='https://panel.example/app/'
SUI_AGENT_TOKEN=abcdefghijklmnopqrstuvwxyzABCDEFGH123456789
SUI_AGENT_INSECURE=true
UNRELATED=value
invalid-line
`))
	if values["SUI_AGENT_PANEL"] != "https://panel.example/app/" || values["SUI_AGENT_INSECURE"] != "true" {
		t.Fatalf("unexpected Agent environment: %#v", values)
	}
	if _, found := values["UNRELATED"]; found {
		t.Fatalf("unrelated environment value was accepted: %#v", values)
	}
}

func TestExchangeAgentPairingPostsFragmentCode(t *testing.T) {
	code := strings.Repeat("b", 43)
	token := strings.Repeat("c", 43)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/app/agent/v1/pair" {
			t.Errorf("unexpected pairing request: %s %s", r.Method, r.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		if r.URL.Fragment != "" || r.URL.RawQuery != "" {
			t.Errorf("pairing secret leaked into request URL: %s", r.URL.String())
			http.Error(w, "secret leaked", http.StatusBadRequest)
			return
		}
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		if payload["code"] != code {
			t.Errorf("unexpected pairing code: %q", payload["code"])
			http.Error(w, "invalid code", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(payload["name"]) == "" {
			t.Error("managed server hostname was not included")
			http.Error(w, "missing name", http.StatusBadRequest)
			return
		}
		if payload["panel_url"] != "https://child.example/app/" {
			t.Errorf("managed panel URL was not included: %#v", payload)
			http.Error(w, "missing panel URL", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"obj": map[string]string{
				"panel_url": serverURLFromRequest(r) + "/app/",
				"token":     token,
				"version":   "test",
			},
		})
	}))
	defer server.Close()

	connection, pairedToken, err := exchangeAgentPairing(
		context.Background(), server.URL+"/app/agent/v1/pair#"+code, false, "https://child.example/app/", server.Client(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if connection.PanelURL != server.URL+"/app/" || connection.Version != "test" || pairedToken != token {
		t.Fatalf("unexpected pairing result: connection=%#v token=%q", connection, pairedToken)
	}
}

func TestNormalizePanelURL(t *testing.T) {
	value, err := NormalizePanelURL("https://child.example:8443/app")
	if err != nil || value != "https://child.example:8443/app/" {
		t.Fatalf("unexpected normalized panel URL: %q err=%v", value, err)
	}
	for _, input := range []string{"", "ftp://child.example/app/", "https://user@child.example/app/", "https://child.example/app/?token=x"} {
		if _, err := NormalizePanelURL(input); err == nil {
			t.Fatalf("accepted invalid panel URL %q", input)
		}
	}
}

func TestExchangeAgentPairingAcceptsPublicAddressOnly(t *testing.T) {
	token := strings.Repeat("d", 43)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/app/agent/v1/enroll" {
			t.Errorf("unexpected enrollment request: %s %s", r.Method, r.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["code"] != "" || strings.TrimSpace(payload["name"]) == "" {
			t.Errorf("unexpected address-only payload: %#v", payload)
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"obj": map[string]string{
				"panel_url": serverURLFromRequest(r) + "/app/",
				"token":     token,
				"version":   "test",
			},
		})
	}))
	defer server.Close()

	connection, pairedToken, err := exchangeAgentPairing(context.Background(), server.URL+"/app/", false, "", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if connection.PanelURL != server.URL+"/app/" || pairedToken != token {
		t.Fatalf("unexpected address-only pairing result: connection=%#v token=%q", connection, pairedToken)
	}
}

func serverURLFromRequest(r *http.Request) string {
	return "http://" + r.Host
}
