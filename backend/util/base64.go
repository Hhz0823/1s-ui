package util

import (
	"encoding/base64"
	"strings"
)

// StrOrBase64Encoded returns the decoded text when str is base64 (standard or
// URL-safe, padded or not, optionally wrapped over several lines), otherwise
// str unchanged. Plain share-link lists are never decoded.
func StrOrBase64Encoded(str string) string {
	trimmed := strings.TrimSpace(str)
	if trimmed == "" || strings.Contains(trimmed, "://") {
		return str
	}
	compact := strings.NewReplacer("\r", "", "\n", "", " ", "", "\t", "").Replace(trimmed)
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding,
	} {
		if decoded, err := encoding.DecodeString(compact); err == nil {
			return string(decoded)
		}
	}
	return str
}

func B64StrToByte(str string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(str)
}

func ByteToB64Str(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
