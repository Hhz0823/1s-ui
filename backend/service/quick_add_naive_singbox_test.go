//go:build !openwrt_lite

package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util"
)

// These tests connect to quick-add NaiveProxy nodes the way v2rayN does: it
// imports the v2rayn://naive/ link and runs the Naive outbound of the official
// sing-box build (with libcronet), named here by SINGBOX_TEST_BINARY.

func requireSingBoxClient(t *testing.T) string {
	t.Helper()
	binary := os.Getenv("SINGBOX_TEST_BINARY")
	if binary == "" {
		t.Skip("SINGBOX_TEST_BINARY is not set")
	}
	return binary
}

// v2rayNNaiveOutbound builds the sing-box outbound v2rayN generates for an
// imported v2rayn://naive/ link (SingboxOutboundService.FillOutbound and
// FillOutboundTls).
func v2rayNNaiveOutbound(t *testing.T, link string) map[string]interface{} {
	t.Helper()
	encoded, ok := strings.CutPrefix(link, "v2rayn://naive/")
	if !ok {
		t.Fatalf("not a v2rayN Naive link: %q", link)
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var profile struct {
		Address, Username, Password, Sni, Cert string
		Port                                   int
		ProtoExtraObj                          struct {
			NaiveQuic, Uot      bool
			CongestionControl   string
			InsecureConcurrency int
		}
	}
	if err = json.Unmarshal(raw, &profile); err != nil {
		t.Fatal(err)
	}
	tls := map[string]interface{}{"enabled": true, "server_name": profile.Sni}
	var certificates []string
	for rest := []byte(profile.Cert); ; {
		var block *pem.Block
		if block, rest = pem.Decode(rest); block == nil {
			break
		}
		certificates = append(certificates, string(pem.EncodeToMemory(block)))
	}
	if len(certificates) > 0 {
		tls["certificate"] = certificates
	}
	outbound := map[string]interface{}{
		"type": "naive", "tag": "proxy", "server": profile.Address, "server_port": profile.Port,
		"username": profile.Username, "password": profile.Password, "tls": tls,
	}
	extra := profile.ProtoExtraObj
	if extra.NaiveQuic {
		outbound["quic"] = true
		if extra.CongestionControl != "" {
			outbound["quic_congestion_control"] = extra.CongestionControl
		}
	}
	if extra.InsecureConcurrency > 0 {
		outbound["insecure_concurrency"] = extra.InsecureConcurrency
	}
	if extra.Uot {
		outbound["udp_over_tcp"] = true
	}
	return outbound
}

// startSingBoxClient runs the sing-box client with a mixed (SOCKS and HTTP)
// entry in front of the outbound and returns the entry's port.
func startSingBoxClient(t *testing.T, binary string, outbound map[string]interface{}) int {
	t.Helper()
	port := freeLocalPort(t)
	config := map[string]interface{}{
		"log": map[string]interface{}{"level": "warn"},
		"inbounds": []interface{}{map[string]interface{}{
			"type": "mixed", "tag": "mixed-in", "listen": "127.0.0.1", "listen_port": port,
		}},
		"outbounds": []interface{}{outbound},
	}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "client.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	cmd := exec.Command(binary, "run", "-c", path)
	// The purego build loads libcronet.so from its own directory.
	cmd.Dir = filepath.Dir(binary)
	cmd.Stdout, cmd.Stderr = &output, &output
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if t.Failed() {
			t.Logf("sing-box client config: %s\noutput:\n%s", raw, output.String())
		}
	})
	waitTCP(t, fmt.Sprintf("127.0.0.1:%d", port))
	return port
}

func clientLinks(t *testing.T, clientID uint) []string {
	t.Helper()
	var client model.Client
	if err := database.GetDB().First(&client, clientID).Error; err != nil {
		t.Fatal(err)
	}
	var links []map[string]string
	if err := json.Unmarshal(client.Links, &links); err != nil {
		t.Fatal(err)
	}
	uris := make([]string, 0, len(links))
	for _, link := range links {
		uris = append(uris, link["uri"])
	}
	return uris
}

// startQuickAddSingBoxServer runs the panel's sing-box configuration and
// returns a function that stops it.
func startQuickAddSingBoxServer(t *testing.T, control *LocalControlService) func() {
	t.Helper()
	config, err := control.ConfigService.GetConfigWithDB("", database.GetDB())
	if err != nil {
		t.Fatal(err)
	}
	instance := core.NewCore()
	if err = instance.Start(*config); err != nil {
		t.Fatalf("sing-box: %v\n%s", err, *config)
	}
	stopped := false
	stop := func() {
		if !stopped {
			stopped = true
			_ = instance.Stop()
		}
	}
	t.Cleanup(stop)
	return stop
}

func v2rayNNaiveLink(t *testing.T, clientID uint) string {
	t.Helper()
	links := clientLinks(t, clientID)
	for _, link := range links {
		if strings.HasPrefix(link, "v2rayn://naive/") {
			return link
		}
	}
	t.Fatalf("no v2rayN link among %q", links)
	return ""
}

func quickAddNaive(t *testing.T, control *LocalControlService, mode string) (*RemoteQuickAddResponse, int) {
	t.Helper()
	port := freeLocalPort(t)
	response, err := control.QuickAddLocalInbounds(RemoteQuickAddRequest{
		CoreType: model.CoreTypeSingBox, Protocol: "naive", Count: 1, Port: port,
		PublicHost: "127.0.0.1", NaiveUsername: "naive-e2e", Password: "naive-e2e-secret",
		NaiveMode: mode, NaiveQUICCongestionControl: "bbr",
	}, "admin")
	if err != nil {
		t.Fatal(err)
	}
	return response, port
}

func naiveInboundTLS(t *testing.T, inboundID uint) model.Tls {
	t.Helper()
	var inbound model.Inbound
	if err := database.GetDB().Preload("Tls").First(&inbound, inboundID).Error; err != nil || inbound.Tls == nil {
		t.Fatalf("inbound %d has no TLS: %v", inboundID, err)
	}
	return *inbound.Tls
}

func TestQuickAddNaiveCarriesTrafficWithV2rayNSingBox(t *testing.T) {
	binary := requireSingBoxClient(t)
	for _, mode := range []string{"https", "quic"} {
		t.Run(mode, func(t *testing.T) {
			control := setupQuickAddTest(t)
			response, port := quickAddNaive(t, control, mode)
			stop := startQuickAddSingBoxServer(t, control)
			if mode == "https" {
				waitTCP(t, fmt.Sprintf("127.0.0.1:%d", port))
			}
			link := v2rayNNaiveLink(t, response.Created[0].ClientID)
			probe := quickAddProbe(t)
			socks := startSingBoxClient(t, binary, v2rayNNaiveOutbound(t, link))
			if err := fetchThroughSocks(socks, probe); err != nil {
				t.Fatalf("v2rayN (sing-box Naive) has no traffic through the %s node: %v\nlink: %s", mode, err, link)
			}

			// A renewal replaces the server certificate; the imported node,
			// which trusts the CA, keeps working and its link is unchanged.
			before := naiveInboundTLS(t, response.Created[0].ID)
			renewed, err := control.ConfigService.RenewGeneratedCertificates(time.Now().Add(40 * 24 * time.Hour))
			if err != nil || renewed != 1 {
				t.Fatalf("renewed %d, %v", renewed, err)
			}
			after := naiveInboundTLS(t, response.Created[0].ID)
			if string(before.Server) == string(after.Server) || string(before.Client) != string(after.Client) {
				t.Fatalf("a renewal must replace only the server certificate:\nbefore %s %s\nafter %s %s", before.Server, before.Client, after.Server, after.Client)
			}
			if again := v2rayNNaiveLink(t, response.Created[0].ClientID); again != link {
				t.Fatalf("the link changed on renewal:\n%s\n%s", link, again)
			}
			stop()
			startQuickAddSingBoxServer(t, control)
			if mode == "https" {
				waitTCP(t, fmt.Sprintf("127.0.0.1:%d", port))
			}
			// The client notices the restarted server on its next connection.
			for attempt := 0; attempt < 10; attempt++ {
				if err = fetchThroughSocks(socks, probe); err == nil {
					break
				}
				t.Logf("attempt %d after the restart: %v", attempt+1, err)
				time.Sleep(2 * time.Second)
			}
			if err != nil {
				t.Fatalf("the imported %s node stopped working after a renewal: %v", mode, err)
			}
		})
	}
}

// TestGeneratedNaiveTLSMovesToPrivateCA covers nodes created before the CA:
// their 12-month self-signed certificate moves to a CA at startup, and the
// rebuilt link works in v2rayN.
func TestGeneratedNaiveTLSMovesToPrivateCA(t *testing.T) {
	binary := requireSingBoxClient(t)
	control := setupQuickAddTest(t)
	response, port := quickAddNaive(t, control, "https")
	legacy := naiveInboundTLS(t, response.Created[0].ID)
	now := time.Now()
	key, certificate, err := util.GenerateSelfSignedTLS("127.0.0.1", now, now.AddDate(0, 12, 0))
	if err != nil {
		t.Fatal(err)
	}
	server := mapFromRaw(legacy.Server)
	server["key"], server["certificate"] = pemLines(key), pemLines(certificate)
	client := map[string]interface{}{"certificate": pemLines(certificate)}
	rawServer, _ := json.Marshal(server)
	rawClient, _ := json.Marshal(client)
	db := database.GetDB()
	if err = db.Model(&model.Tls{}).Where("id = ?", legacy.Id).Updates(map[string]interface{}{"server": rawServer, "client": rawClient}).Error; err != nil {
		t.Fatal(err)
	}
	db.Where("tls_id = ?", legacy.Id).Delete(&model.TlsAuthority{})

	migrated, err := control.ConfigService.MigrateGeneratedNaiveTLS(now)
	if err != nil || migrated != 1 {
		t.Fatalf("migrated %d, %v", migrated, err)
	}
	if again, _ := control.ConfigService.MigrateGeneratedNaiveTLS(now); again != 0 {
		t.Fatalf("migrated a configuration twice")
	}
	moved := naiveInboundTLS(t, response.Created[0].ID)
	chain := util.CertPEMFromTLS(mapFromRaw(moved.Server))
	if util.PrivateRootPEM(chain) == "" || util.LeafValidity(chain) > util.GeneratedLeafValidity {
		t.Fatalf("not moved to a CA: %s", chain)
	}
	startQuickAddSingBoxServer(t, control)
	waitTCP(t, fmt.Sprintf("127.0.0.1:%d", port))
	link := v2rayNNaiveLink(t, response.Created[0].ClientID)
	socks := startSingBoxClient(t, binary, v2rayNNaiveOutbound(t, link))
	if err = fetchThroughSocks(socks, quickAddProbe(t)); err != nil {
		t.Fatalf("the moved node has no traffic: %v\nlink: %s", err, link)
	}
}
