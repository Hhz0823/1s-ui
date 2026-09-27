package util

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"strings"
	"time"
)

// Chromium, and with it every NaiveProxy client (v2rayN runs the cronet-based
// Naive outbound of sing-box), rejects a server certificate valid for longer
// than the CA/Browser Forum limit in force on its issue date, even under a
// trust anchor the user added (net::ERR_CERT_VALIDITY_TOO_LONG): 200 days
// from 2026-03-15, 100 days from 2027-03-15 and 47 days from 2029-03-15.
// Generated certificates for such clients therefore come from a private CA
// that the share links carry, and the panel renews the server certificate
// before it expires; clients keep trusting the CA.
const (
	GeneratedLeafValidity    = 45 * 24 * time.Hour
	GeneratedLeafRenewBefore = 15 * 24 * time.Hour
	generatedCAYears         = 20
)

// GenerateTLSAuthority returns the key and certificate (PEM) of a new private
// root CA.
func GenerateTLSAuthority(commonName string, now time.Time) (keyPEM, certPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(generatedCAYears, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err = marshalECKeyPEM(key)
	if err != nil {
		return nil, nil, err
	}
	return keyPEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}

// IssueTLSLeaf signs a server certificate for serverName (a domain or an IP
// address) with the CA, valid for GeneratedLeafValidity, and returns its key
// and the chain served to clients: the leaf followed by the CA.
func IssueTLSLeaf(caKeyPEM, caCertPEM []byte, serverName string, now time.Time) (keyPEM, chainPEM []byte, err error) {
	caCert := parseLeafCert(string(caCertPEM))
	if caCert == nil || !caCert.IsCA {
		return nil, nil, errors.New("invalid CA certificate")
	}
	block, _ := pem.Decode(caKeyPEM)
	if block == nil {
		return nil, nil, errors.New("invalid CA key")
	}
	caKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}
	serverName = strings.Trim(strings.TrimSpace(serverName), "[]")
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: serverName},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(-time.Hour).Add(GeneratedLeafValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	if ip := net.ParseIP(serverName); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{serverName}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, caCert, key.Public(), caKey)
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err = marshalECKeyPEM(key)
	if err != nil {
		return nil, nil, err
	}
	chainPEM = append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw})...)
	return keyPEM, chainPEM, nil
}

// PrivateRootPEM returns the self-signed CA a server certificate chain ends
// in, as generated chains do, or "". Public chains leave their root out.
func PrivateRootPEM(chainPEM string) string {
	certs := parseCertChain(chainPEM)
	if len(certs) < 2 {
		return ""
	}
	if root := certs[len(certs)-1]; root.IsCA && root.CheckSignatureFrom(root) == nil {
		return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: root.Raw}))
	}
	return ""
}

// TrustAnchorPEM is the certificate a client pins or trusts for a server
// certificate chain: the private root CA of a generated chain, otherwise the
// leaf itself.
func TrustAnchorPEM(chainPEM string) string {
	if root := PrivateRootPEM(chainPEM); root != "" {
		return root
	}
	if leaf := parseLeafCert(chainPEM); leaf != nil {
		return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leaf.Raw}))
	}
	return ""
}

// LeafNeedsRenewal reports whether a chain's server certificate expires within
// GeneratedLeafRenewBefore, or is not issued by the CA in caCertPEM.
func LeafNeedsRenewal(chainPEM, caCertPEM string, now time.Time) bool {
	certs := parseCertChain(chainPEM)
	ca := parseLeafCert(caCertPEM)
	if len(certs) == 0 || ca == nil {
		return true
	}
	leaf := certs[0]
	return leaf.CheckSignatureFrom(ca) != nil || now.Add(GeneratedLeafRenewBefore).After(leaf.NotAfter)
}

// LeafValidity returns how long a chain's server certificate is valid.
func LeafValidity(chainPEM string) time.Duration {
	leaf := parseLeafCert(chainPEM)
	if leaf == nil {
		return 0
	}
	return leaf.NotAfter.Sub(leaf.NotBefore)
}

// LeafServerName is the first name a server certificate is issued for.
func LeafServerName(chainPEM string) string {
	leaf := parseLeafCert(chainPEM)
	if leaf == nil {
		return ""
	}
	if len(leaf.DNSNames) > 0 {
		return leaf.DNSNames[0]
	}
	if len(leaf.IPAddresses) > 0 {
		return leaf.IPAddresses[0].String()
	}
	return leaf.Subject.CommonName
}

func parseCertChain(pemData string) []*x509.Certificate {
	var certs []*x509.Certificate
	rest := []byte(pemData)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return certs
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return certs
		}
		certs = append(certs, cert)
	}
}

func randomSerial() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
}

func marshalECKeyPEM(key *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}
