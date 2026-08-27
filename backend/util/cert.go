package util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"strings"
	"time"
)

func GenerateSelfSignedTLS(serverName string, now, expire time.Time) ([]byte, []byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}
	serverName = strings.Trim(strings.TrimSpace(serverName), "[]")
	template := &x509.Certificate{
		SerialNumber:          serial,
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              expire,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		Subject:               pkix.Name{CommonName: serverName},
	}
	if ip := net.ParseIP(serverName); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{serverName}
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		return nil, nil, err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	return privatePEM, certificatePEM, nil
}

func CertPEMFromTLS(tlsConfig map[string]interface{}) string {
	if tlsConfig == nil {
		return ""
	}
	switch c := tlsConfig["certificate"].(type) {
	case string:
		if c != "" {
			return c
		}
	case []interface{}:
		lines := make([]string, 0, len(c))
		for _, l := range c {
			if s, ok := l.(string); ok {
				lines = append(lines, s)
			}
		}
		if len(lines) > 0 {
			return strings.Join(lines, "\n")
		}
	}
	if path, ok := tlsConfig["certificate_path"].(string); ok && path != "" {
		if data, err := os.ReadFile(path); err == nil {
			return string(data)
		}
	}
	return ""
}

func parseLeafCert(pemData string) *x509.Certificate {
	rest := []byte(pemData)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil
		}
		if block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil
			}
			return cert
		}
	}
}

func CertIsSelfSigned(pemData string) bool {
	cert := parseLeafCert(pemData)
	if cert == nil {
		return false
	}
	return cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature) == nil
}

func CertPublicKeySha256(pemData string) string {
	cert := parseLeafCert(pemData)
	if cert == nil {
		return ""
	}
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return base64.StdEncoding.EncodeToString(sum[:])
}

func CertSha256Hex(pemData string) string {
	cert := parseLeafCert(pemData)
	if cert == nil {
		return ""
	}
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}

func CertSha256Base64(pemData string) string {
	cert := parseLeafCert(pemData)
	if cert == nil {
		return ""
	}
	sum := sha256.Sum256(cert.Raw)
	return base64.StdEncoding.EncodeToString(sum[:])
}
