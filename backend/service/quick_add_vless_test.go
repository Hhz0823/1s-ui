package service

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// checkVlessLinkForClients applies the vless:// parsing rules of the clients
// the quick-add nodes are made for: v2rayNG (VlessFmt), v2rayN (VLESSFmt) and
// Anywhere (ProxyConfiguration+URLParsing, RealityConfiguration,
// VLESSEncryption, XHTTPConfiguration). Shadowrocket takes the same fields;
// checkVlessLinkForPassWall adds PassWall's two parsers.
func checkVlessLinkForClients(t *testing.T, link string) url.Values {
	t.Helper()
	parsed, err := url.Parse(link)
	if err != nil || parsed.Scheme != "vless" {
		t.Fatalf("not a vless link: %q (%v)", link, err)
	}
	// v2rayNG returns nothing for a link without a query.
	if parsed.RawQuery == "" {
		t.Fatalf("v2rayNG rejects links without parameters: %s", link)
	}
	if !uuidPattern.MatchString(parsed.User.Username()) {
		t.Fatalf("Anywhere needs a UUID user: %s", link)
	}
	if port, err := strconv.Atoi(parsed.Port()); err != nil || port < 1 || port > 65535 {
		t.Fatalf("bad port in %s", link)
	}
	if parsed.Fragment == "" {
		t.Fatalf("link has no name: %s", link)
	}
	query := parsed.Query()
	for key, values := range query {
		if len(values) != 1 {
			t.Fatalf("parameter %s repeats; clients disagree on which copy wins: %s", key, link)
		}
	}

	encryption := query.Get("encryption")
	switch {
	case encryption == "none":
	case encryption == "":
		t.Fatalf("encryption must be explicit (Xray-based clients require it): %s", link)
	default:
		checkVlessEncryptionForAnywhere(t, encryption)
	}

	security := query.Get("security")
	switch security {
	case "reality":
		if query.Get("sni") == "" {
			t.Fatalf("REALITY needs sni: %s", link)
		}
		if key, err := base64.RawURLEncoding.DecodeString(query.Get("pbk")); err != nil || len(key) != 32 {
			t.Fatalf("REALITY pbk must be a base64url x25519 key: %s", link)
		}
		if sid := query.Get("sid"); len(sid) > 16 || len(sid)%2 != 0 {
			t.Fatalf("REALITY sid must be at most 8 hex bytes: %s", link)
		} else if _, err := hex.DecodeString(sid); err != nil {
			t.Fatalf("REALITY sid is not hex: %s", link)
		}
		if query.Get("fp") == "" {
			t.Fatalf("REALITY needs a uTLS fingerprint: %s", link)
		}
	case "tls", "none":
	default:
		t.Fatalf("unexpected security %q: %s", security, link)
	}

	network := query.Get("type")
	switch network {
	case "tcp":
	case "xhttp":
		// Anywhere sends an explicit empty host verbatim instead of
		// falling back to the SNI or the address.
		if values, ok := query["host"]; ok && values[0] == "" {
			t.Fatalf("XHTTP link carries an empty host: %s", link)
		}
		switch query.Get("mode") {
		case "auto", "packet-up", "stream-up", "stream-one":
		default:
			t.Fatalf("XHTTP mode %q is unknown to clients: %s", query.Get("mode"), link)
		}
		if !strings.HasPrefix(query.Get("path"), "/") {
			t.Fatalf("XHTTP path must be absolute: %s", link)
		}
	default:
		t.Fatalf("unexpected transport %q: %s", network, link)
	}

	if flow := query.Get("flow"); flow != "" {
		if flow != vlessVisionFlow {
			t.Fatalf("clients only know the %s flow: %s", vlessVisionFlow, link)
		}
		// Xray-core (v2rayN, v2rayNG, PassWall) and Anywhere run Vision
		// directly on raw TCP under TLS 1.3 or REALITY, and on other
		// transports only underneath VLESS Encryption.
		if network != "tcp" && encryption == "none" {
			t.Fatalf("Vision on %s needs VLESS Encryption: %s", network, link)
		}
		// Anywhere refuses Vision without outer TLS 1.3/REALITY or VLESS
		// Encryption underneath.
		if security == "none" && encryption == "none" {
			t.Fatalf("Vision without TLS, REALITY or VLESS Encryption: %s", link)
		}
	}
	return query
}

// checkVlessEncryptionForAnywhere follows Anywhere's VLESSEncryption.parse:
// mlkem768x25519plus.<native|xorpub|random>.<1rtt|0rtt>[.padding].<keys>,
// keys being base64url X25519 (32 bytes) or ML-KEM-768 (1184 bytes).
func checkVlessEncryptionForAnywhere(t *testing.T, encryption string) {
	t.Helper()
	segments := strings.Split(encryption, ".")
	if len(segments) < 4 || segments[0] != "mlkem768x25519plus" {
		t.Fatalf("unsupported VLESS Encryption %q", encryption)
	}
	if segments[1] != "native" && segments[1] != "xorpub" && segments[1] != "random" {
		t.Fatalf("unsupported VLESS Encryption mode %q", encryption)
	}
	if segments[2] != "0rtt" && segments[2] != "1rtt" {
		t.Fatalf("unsupported VLESS Encryption RTT %q", encryption)
	}
	keys := 0
	for _, segment := range segments[3:] {
		if len(segment) < 20 {
			continue // padding
		}
		key, err := base64.RawURLEncoding.DecodeString(segment)
		if err != nil || (len(key) != 32 && len(key) != 1184) {
			t.Fatalf("VLESS Encryption key %q is not a client key", segment)
		}
		keys++
	}
	if keys == 0 {
		t.Fatalf("VLESS Encryption %q has no key", encryption)
	}
}

// quickAddClientLink returns the first share link of the user created with a
// quick-add node.
func quickAddClientLink(t *testing.T, clientID uint) string {
	t.Helper()
	var client model.Client
	if err := database.GetDB().First(&client, clientID).Error; err != nil {
		t.Fatal(err)
	}
	var links []map[string]string
	if err := json.Unmarshal(client.Links, &links); err != nil || len(links) == 0 {
		t.Fatalf("client has no links: %s (%v)", client.Links, err)
	}
	return links[0]["uri"]
}

func TestQuickAddVlessVariantValidation(t *testing.T) {
	for _, test := range []struct {
		core, variant, server string
		ok                    bool
	}{
		{model.CoreTypeSingBox, "", "", true},
		{model.CoreTypeSingBox, VlessVariantRealityVision, "", sdwanRealitySupported},
		{model.CoreTypeSingBox, " Reality-Vision ", "www.example.com", sdwanRealitySupported},
		{model.CoreTypeXray, VlessVariantRealityVision, "", true},
		{model.CoreTypeSingBox, VlessVariantRealityXHTTP, "", false},
		{model.CoreTypeSingBox, VlessVariantEncVision, "", false},
		{model.CoreTypeSingBox, VlessVariantEncXHTTP, "", false},
		{model.CoreTypeXray, VlessVariantEncXHTTP, "", true},
		{model.CoreTypeSingBox, VlessVariantRealityXHTTPVision, "", false},
		{model.CoreTypeXray, VlessVariantRealityXHTTPVision, "www.example.com", true},
		{model.CoreTypeXray, "vision", "", false},
		{model.CoreTypeSingBox, VlessVariantRealityVision, "https://www.example.com/", false},
	} {
		request := RemoteQuickAddRequest{CoreType: test.core, Protocol: "vless", Count: 1, Port: 443, VlessVariant: test.variant, RealityServer: test.server}
		err := validateRemoteQuickAddRequest(&request)
		if (err == nil) != test.ok {
			t.Fatalf("%s/%q/%q: err = %v", test.core, test.variant, test.server, err)
		}
		if err == nil && test.variant == "" && request.VlessVariant != VlessVariantTLS {
			t.Fatalf("an unset variant (older controllers) must keep the TLS node, got %q", request.VlessVariant)
		}
	}
}

// xrayDiscouragesRealityTarget mirrors the warning in Xray-core's
// infra/conf/transport_security.go: imitating these targets makes the GFW
// more likely to block the server's IP.
func xrayDiscouragesRealityTarget(name string) bool {
	name = strings.ToLower(name)
	for _, suffix := range []string{".ru", ".ir", ".cn"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	for _, brand := range []string{"apple", "icloud", "microsoft"} {
		if strings.Contains(name, brand) {
			return true
		}
	}
	return false
}

func TestAutomaticRealityTargetsAvoidDiscouragedSites(t *testing.T) {
	for _, host := range append([]string{defaultSdwanRealitySNI}, sdwanRealityCandidates...) {
		if xrayDiscouragesRealityTarget(host) {
			t.Fatalf("automatic REALITY target %q is one Xray-core warns against", host)
		}
		if normalized, err := normalizeRealityServer(host); err != nil || normalized != host {
			t.Fatalf("automatic REALITY target %q is not a valid target: %v", host, err)
		}
	}
}

func TestGenerateVlessEncryptionPairs(t *testing.T) {
	decryption, encryption, err := generateVlessEncryption()
	if err != nil {
		t.Fatal(err)
	}
	checkVlessEncryptionForAnywhere(t, encryption)
	server := strings.Split(decryption, ".")
	client := strings.Split(encryption, ".")
	if len(server) != 4 || server[0] != "mlkem768x25519plus" || server[1] != "native" || server[2] != "600s" {
		t.Fatalf("decryption = %q", decryption)
	}
	if server[3] == client[3] {
		t.Fatal("the link must carry the public key, not the server's private key")
	}
	_, other, _ := generateVlessEncryption()
	if other == encryption {
		t.Fatal("every node needs its own key")
	}
}

// TestQuickAddSingBoxVlessLinksImport covers the sing-box VLESS nodes, which
// need no Xray binary: REALITY + Vision (the default) and the TLS node.
func TestQuickAddSingBoxVlessLinksImport(t *testing.T) {
	for _, test := range []struct {
		variant string
		want    []string
	}{
		{VlessVariantRealityVision, []string{"security=reality", "flow=xtls-rprx-vision", "type=tcp", "sni=reality.test", "fp=chrome"}},
		{VlessVariantTLS, []string{"security=tls", "flow=xtls-rprx-vision", "type=tcp", "pcs="}},
	} {
		t.Run(test.variant, func(t *testing.T) {
			if vlessVariantReality(test.variant) && !sdwanRealitySupported {
				t.Skip("sing-box REALITY needs the with_utls build tag")
			}
			control := setupQuickAddTest(t)
			stubRealityProbe(t, "reality.test")
			response, err := control.QuickAddLocalInbounds(RemoteQuickAddRequest{
				CoreType: model.CoreTypeSingBox, Protocol: "vless", VlessVariant: test.variant,
				Count: 2, Port: 42100, PublicHost: "198.51.100.60",
			}, "admin")
			if err != nil {
				t.Fatal(err)
			}
			if len(response.Created) != 2 {
				t.Fatalf("created = %#v", response.Created)
			}
			for _, created := range response.Created {
				link := quickAddClientLink(t, created.ClientID)
				if nodes := checkVlessLinkForPassWall(t, link); len(nodes) != 2 {
					t.Fatalf("PassWall imports %s only by %v: %s", test.variant, nodes, link)
				}
				for _, fragment := range test.want {
					if !strings.Contains(link, fragment) {
						t.Fatalf("%s link lacks %q: %s", test.variant, fragment, link)
					}
				}
			}
		})
	}
}
