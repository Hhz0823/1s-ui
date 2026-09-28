package service

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/util"
	"github.com/Hhz0823/1s-ui/util/common"

	"gorm.io/gorm"
)

// Generated TLS configurations with a private CA (util.GenerateTLSAuthority).
// NaiveProxy clients run Chromium's certificate verifier, which refuses the
// long-lived self-signed certificates the panel used to generate, so quick-add
// Naive nodes get a CA whose certificate the share links carry and a
// short-lived server certificate the panel renews; clients never have to
// import the node again.

// naiveMaxLeafValidity is the longest server certificate Chromium accepts
// today (see util.GeneratedLeafValidity).
const naiveMaxLeafValidity = 200 * 24 * time.Hour

func pemLines(pemData []byte) []string {
	return strings.Split(strings.TrimSpace(string(pemData)), "\n")
}

// authorityTLSSides returns the server side (the leaf, then the CA) and the
// client side (trusting and pinning the CA) of a generated configuration.
func authorityTLSSides(server map[string]interface{}, serverName string, leafKey, chain, caCert []byte) (map[string]interface{}, map[string]interface{}) {
	if server == nil {
		server = map[string]interface{}{}
	}
	delete(server, "certificate_path")
	delete(server, "key_path")
	server["enabled"] = true
	server["server_name"] = serverName
	server["key"] = pemLines(leafKey)
	server["certificate"] = pemLines(chain)
	pin := util.CertSha256Base64(string(caCert))
	client := map[string]interface{}{
		"certificate":                    pemLines(caCert),
		"pinned_peer_certificate_sha256": []string{pin},
	}
	return server, client
}

// newTLSAuthority creates a CA and a server certificate for serverName.
func newTLSAuthority(serverName string, now time.Time) (caKey, caCert, leafKey, chain []byte, err error) {
	caKey, caCert, err = util.GenerateTLSAuthority("1S-UI "+strings.Trim(serverName, "[]"), now)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	leafKey, chain, err = util.IssueTLSLeaf(caKey, caCert, serverName, now)
	return caKey, caCert, leafKey, chain, err
}

// createAuthorityTLS saves a generated TLS configuration named name for
// serverName and keeps its CA for renewals.
func (s *LocalControlService) createAuthorityTLS(name, serverName string, revision uint64, changeActor, publicHost string) (uint, uint64, error) {
	caKey, caCert, leafKey, chain, err := newTLSAuthority(serverName, time.Now())
	if err != nil {
		return 0, revision, err
	}
	server, client := authorityTLSSides(nil, serverName, leafKey, chain, caCert)
	raw, err := json.Marshal(map[string]interface{}{"id": 0, "name": name, "server": server, "client": client})
	if err != nil {
		return 0, revision, err
	}
	if _, revision, err = s.ConfigService.SaveWithRevision(revision, "tls", "new", raw, "", changeActor, publicHost); err != nil {
		return 0, revision, err
	}
	var saved model.Tls
	if err = database.GetDB().Where("name = ?", name).First(&saved).Error; err != nil {
		return 0, revision, err
	}
	authority := model.TlsAuthority{TlsId: saved.Id, Certificate: string(caCert), Key: string(caKey)}
	if err = database.GetDB().Save(&authority).Error; err != nil {
		return 0, revision, err
	}
	return saved.Id, revision, nil
}

// renewAuthorityTLS issues a new server certificate for a configuration with
// a CA, inside the configuration save transaction.
func renewAuthorityTLS(tx *gorm.DB, tlsID uint, now time.Time) error {
	var authority model.TlsAuthority
	if err := tx.Where("tls_id = ?", tlsID).First(&authority).Error; err != nil {
		return common.NewError("TLS configuration has no private CA to renew with")
	}
	var tlsConfig model.Tls
	if err := tx.First(&tlsConfig, tlsID).Error; err != nil {
		return err
	}
	server := mapFromRaw(tlsConfig.Server)
	serverName, _ := server["server_name"].(string)
	if serverName == "" {
		serverName = util.LeafServerName(util.CertPEMFromTLS(server))
	}
	if serverName == "" {
		return common.NewError("TLS configuration has no server name")
	}
	leafKey, chain, err := util.IssueTLSLeaf([]byte(authority.Key), []byte(authority.Certificate), serverName, now)
	if err != nil {
		return err
	}
	server, client := authorityTLSSides(server, serverName, leafKey, chain, []byte(authority.Certificate))
	rawServer, err := json.MarshalIndent(server, "", "  ")
	if err != nil {
		return err
	}
	updates := map[string]interface{}{"server": json.RawMessage(rawServer)}
	// The client side trusts the CA and normally stays as it is; other client
	// options, such as a uTLS fingerprint, are kept.
	clientConfig := mapFromRaw(tlsConfig.Client)
	merged := mapFromRaw(tlsConfig.Client)
	for key, value := range client {
		merged[key] = value
	}
	if normalized, _ := json.Marshal(merged); string(normalized) != mustMarshalString(clientConfig) {
		rawClient, err := json.MarshalIndent(merged, "", "  ")
		if err != nil {
			return err
		}
		updates["client"] = json.RawMessage(rawClient)
	}
	return tx.Model(&model.Tls{}).Where("id = ?", tlsID).Updates(updates).Error
}

func mustMarshalString(value interface{}) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

// RenewGeneratedCertificates renews the server certificates that expire
// within util.GeneratedLeafRenewBefore. Links keep working: they carry the CA.
func (s *ConfigService) RenewGeneratedCertificates(now time.Time) (int, error) {
	db := database.GetDB()
	var authorities []model.TlsAuthority
	if err := db.Find(&authorities).Error; err != nil {
		return 0, err
	}
	renewed := 0
	for _, authority := range authorities {
		var tlsConfig model.Tls
		err := db.First(&tlsConfig, authority.TlsId).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			db.Where("tls_id = ?", authority.TlsId).Delete(&model.TlsAuthority{})
			continue
		}
		if err != nil {
			return renewed, err
		}
		if !util.LeafNeedsRenewal(util.CertPEMFromTLS(mapFromRaw(tlsConfig.Server)), authority.Certificate, now) {
			continue
		}
		id, _ := json.Marshal(authority.TlsId)
		if _, err = s.Save("tls", "renew", id, "", "system", ""); err != nil {
			return renewed, common.NewErrorf("renew TLS %q: %v", tlsConfig.Name, err)
		}
		renewed++
	}
	return renewed, nil
}

// MigrateGeneratedNaiveTLS moves the generated TLS configurations of Naive
// nodes created before the private CA, whose 12-month self-signed certificate
// Chromium-based clients refuse, to a CA. Only configurations used by nothing
// but Naive inbounds move, because their links carry the certificate; each of
// those nodes has to be imported once more.
func (s *ConfigService) MigrateGeneratedNaiveTLS(now time.Time) (int, error) {
	db := database.GetDB()
	var tlsConfigs []model.Tls
	if err := db.Where("name LIKE ?", generatedTLSNamePrefix+"%").Find(&tlsConfigs).Error; err != nil {
		return 0, err
	}
	migrated := 0
	for _, tlsConfig := range tlsConfigs {
		var authorities int64
		if err := db.Model(&model.TlsAuthority{}).Where("tls_id = ?", tlsConfig.Id).Count(&authorities).Error; err != nil {
			return migrated, err
		}
		server := mapFromRaw(tlsConfig.Server)
		chain := util.CertPEMFromTLS(server)
		if authorities > 0 || chain == "" || !util.CertIsSelfSigned(chain) || util.PrivateRootPEM(chain) != "" {
			continue
		}
		if onlyNaive, err := tlsUsedOnlyByNaive(db, tlsConfig.Id); err != nil {
			return migrated, err
		} else if !onlyNaive {
			if util.LeafValidity(chain) > naiveMaxLeafValidity && tlsUsedByNaive(db, tlsConfig.Id) {
				logger.Warningf("TLS %q is shared with other inbounds; its Naive nodes need a certificate valid for at most 200 days", tlsConfig.Name)
			}
			continue
		}
		serverName, _ := server["server_name"].(string)
		if serverName == "" {
			serverName = util.LeafServerName(chain)
		}
		caKey, caCert, leafKey, newChain, err := newTLSAuthority(serverName, now)
		if err != nil {
			return migrated, err
		}
		server, client := authorityTLSSides(server, serverName, leafKey, newChain, caCert)
		clientConfig := mapFromRaw(tlsConfig.Client)
		delete(clientConfig, "certificate_public_key_sha256")
		for key, value := range client {
			clientConfig[key] = value
		}
		raw, err := json.Marshal(map[string]interface{}{
			"id": tlsConfig.Id, "name": tlsConfig.Name, "server": server, "client": clientConfig,
		})
		if err != nil {
			return migrated, err
		}
		// The CA is kept first so a configuration never outlives it.
		authority := model.TlsAuthority{TlsId: tlsConfig.Id, Certificate: string(caCert), Key: string(caKey)}
		if err = db.Save(&authority).Error; err != nil {
			return migrated, err
		}
		if _, err = s.Save("tls", "edit", raw, "", "system", ""); err != nil {
			db.Where("tls_id = ?", tlsConfig.Id).Delete(&model.TlsAuthority{})
			return migrated, common.NewErrorf("move TLS %q to a private CA: %v", tlsConfig.Name, err)
		}
		migrated++
	}
	return migrated, nil
}

// tlsUsedOnlyByNaive reports whether only Naive inbounds with their own
// server address use a TLS configuration, so their links and outbounds can be
// rebuilt without a request's host name.
func tlsUsedOnlyByNaive(db *gorm.DB, tlsID uint) (bool, error) {
	var inbounds []model.Inbound
	if err := db.Where("tls_id = ?", tlsID).Find(&inbounds).Error; err != nil {
		return false, err
	}
	var services int64
	if err := db.Model(&model.Service{}).Where("tls_id = ?", tlsID).Count(&services).Error; err != nil {
		return false, err
	}
	if len(inbounds) == 0 || services > 0 {
		return false, nil
	}
	for _, inbound := range inbounds {
		var addrs []map[string]interface{}
		_ = json.Unmarshal(inbound.Addrs, &addrs)
		if inbound.Type != "naive" || len(addrs) == 0 {
			return false, nil
		}
		for _, addr := range addrs {
			if server, _ := addr["server"].(string); server == "" {
				return false, nil
			}
		}
	}
	return true, nil
}

func tlsUsedByNaive(db *gorm.DB, tlsID uint) bool {
	var count int64
	db.Model(&model.Inbound{}).Where("tls_id = ? AND type = ?", tlsID, "naive").Count(&count)
	return count > 0
}

// checkNaiveCertificate rejects a TLS configuration that NaiveProxy clients
// would refuse: a server certificate valid for more than 200 days.
func checkNaiveCertificate(tlsConfig model.Tls) error {
	chain := util.CertPEMFromTLS(mapFromRaw(tlsConfig.Server))
	if chain != "" && util.LeafValidity(chain) > naiveMaxLeafValidity {
		return common.NewErrorf("the certificate of TLS %q is valid for more than 200 days, which NaiveProxy clients (v2rayN, NaiveProxy) refuse; keep the automatic certificate", tlsConfig.Name)
	}
	return nil
}
