package util

import (
	"net"
	"testing"
	"time"
)

func TestGenerateSelfSignedTLSUsesCorrectSubjectAlternativeName(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	for _, test := range []struct {
		name   string
		server string
	}{
		{name: "domain", server: "node.example.com"},
		{name: "IPv4", server: "198.51.100.20"},
		{name: "IPv6", server: "2001:db8::20"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, certificate, err := GenerateSelfSignedTLS(test.server, now, now.AddDate(1, 0, 0))
			if err != nil {
				t.Fatal(err)
			}
			parsed := parseLeafCert(string(certificate))
			if parsed == nil {
				t.Fatal("generated certificate could not be parsed")
			}
			if err = parsed.VerifyHostname(test.server); err != nil {
				t.Fatalf("certificate does not match %q: %v", test.server, err)
			}
			if ip := net.ParseIP(test.server); ip != nil && len(parsed.IPAddresses) != 1 {
				t.Fatalf("IP certificate SANs = %#v", parsed.IPAddresses)
			}
		})
	}
}
