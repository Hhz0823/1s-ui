package util

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
)

const sha256DigestLength = 32

func pinnedPeerCertSha256ForLink(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if decoded, ok := decodePinnedPeerCertSha256Base64(value); ok {
		return hex.EncodeToString(decoded)
	}
	if _, normalized, ok := decodePinnedPeerCertSha256Hex(value); ok {
		return normalized
	}
	return value
}

func pinnedPeerCertSha256ForConfig(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if decoded, ok := decodePinnedPeerCertSha256Base64(value); ok {
		return base64.StdEncoding.EncodeToString(decoded)
	}
	if decoded, _, ok := decodePinnedPeerCertSha256Hex(value); ok {
		return base64.StdEncoding.EncodeToString(decoded)
	}
	return value
}

func decodePinnedPeerCertSha256Base64(value string) ([]byte, bool) {
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(value)
		if err == nil && len(decoded) == sha256DigestLength {
			return decoded, true
		}
	}
	return nil, false
}

func decodePinnedPeerCertSha256Hex(value string) ([]byte, string, bool) {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), ":", ""))
	decoded, err := hex.DecodeString(normalized)
	if err != nil || len(decoded) != sha256DigestLength {
		return nil, "", false
	}
	return decoded, normalized, true
}
