package sub

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestAppendLinkRemarkKeepsLinksParseable(t *testing.T) {
	const info = " 10.00GB 30 Days"
	tests := map[string]string{
		"socks://dTpw@198.51.100.1:1080":                   "",
		"trojan://pw@198.51.100.1:443?security=tls#节点%201": "节点 1",
		"ss://YWVzLTI1Ni1nY206cHc@198.51.100.1:8388#ss":    "ss",
	}
	for link, remark := range tests {
		got := AppendLinkRemark(link, info)
		if strings.Contains(got, " ") {
			t.Fatalf("remark suffix must be escaped: %q", got)
		}
		parsed, err := url.Parse(got)
		if err != nil {
			t.Fatalf("link became invalid: %q (%v)", got, err)
		}
		if parsed.Fragment != remark+info {
			t.Fatalf("fragment = %q, want %q", parsed.Fragment, remark+info)
		}
		if parsed.Port() == "" {
			t.Fatalf("port lost in %q", got)
		}
	}
}

func TestAppendLinkRemarkUpdatesEncodedProfiles(t *testing.T) {
	vmess, _ := json.Marshal(map[string]interface{}{"v": "2", "ps": "vm", "add": "198.51.100.1", "port": "443"})
	got := AppendLinkRemark("vmess://"+base64.StdEncoding.EncodeToString(vmess), " 1GB")
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, "vmess://"))
	if err != nil {
		t.Fatal(err)
	}
	var vmessProfile map[string]interface{}
	if err = json.Unmarshal(decoded, &vmessProfile); err != nil || vmessProfile["ps"] != "vm 1GB" {
		t.Fatalf("vmess remark = %#v (%v)", vmessProfile, err)
	}

	profile, _ := json.Marshal(map[string]interface{}{"Remarks": "naive", "Address": "198.51.100.1", "Port": 443})
	got = AppendLinkRemark("v2rayn://naive/"+base64.RawURLEncoding.EncodeToString(profile), " 1GB")
	if !strings.HasPrefix(got, "v2rayn://naive/") {
		t.Fatalf("v2rayN prefix changed: %q", got)
	}
	decoded, err = base64.RawURLEncoding.DecodeString(strings.TrimPrefix(got, "v2rayn://naive/"))
	if err != nil {
		t.Fatalf("v2rayN payload must stay valid base64url: %v", err)
	}
	var naiveProfile map[string]interface{}
	if err = json.Unmarshal(decoded, &naiveProfile); err != nil || naiveProfile["Remarks"] != "naive 1GB" {
		t.Fatalf("v2rayN remark = %#v (%v)", naiveProfile, err)
	}
}

func TestAppendLinkRemarkIgnoresEmptyInfo(t *testing.T) {
	const link = "socks://dTpw@198.51.100.1:1080#s"
	if got := AppendLinkRemark(link, " "); got != link {
		t.Fatalf("blank info changed the link: %q", got)
	}
}

func TestProfileTitleHeaderIsASCII(t *testing.T) {
	if got := profileTitleHeader("user-1"); got != "user-1" {
		t.Fatalf("ASCII title changed: %q", got)
	}
	got := profileTitleHeader("张三")
	if !strings.HasPrefix(got, "base64:") {
		t.Fatalf("non-ASCII title must be base64 encoded: %q", got)
	}
	decoded, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, "base64:"))
	if string(decoded) != "张三" {
		t.Fatalf("decoded title = %q", decoded)
	}
}
