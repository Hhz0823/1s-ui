package proxyprobe

import (
	"encoding/base64"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ParseLink reads the proxy formats people paste: share links such as
// socks5://user:pass@host:port#name (v2rayN puts base64 "user:pass" in a
// socks:// link), http://user:pass@host:port, and the seller format
// host:port:user:pass. The returned name is the link's #fragment, if any.
func ParseLink(text string) (Spec, string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Spec{}, "", errors.New("empty proxy link")
	}
	if scheme, rest, ok := strings.Cut(text, "://"); ok {
		return parseURL(strings.ToLower(scheme), rest, text)
	}
	return parsePlain(text)
}

func parseURL(scheme, rest, original string) (Spec, string, error) {
	var spec Spec
	switch scheme {
	case "socks", "socks5", "socks5h":
		spec.Type = TypeSOCKS5
	case "http":
		spec.Type = TypeHTTP
	case "https":
		return Spec{}, "", errors.New("HTTPS proxies (TLS to the proxy) are not supported; use http:// or socks5://")
	default:
		return Spec{}, "", errors.New("unsupported proxy link; use socks5://, http:// or host:port:user:pass")
	}
	parsed, err := url.Parse(spec.Type + "://" + rest)
	if err != nil || parsed.Hostname() == "" {
		return Spec{}, "", errors.New("invalid proxy link: " + original)
	}
	spec.Host = parsed.Hostname()
	spec.Port, err = strconv.Atoi(parsed.Port())
	if err != nil {
		return Spec{}, "", errors.New("the proxy link has no port")
	}
	if parsed.User != nil {
		spec.Username = parsed.User.Username()
		spec.Password, _ = parsed.User.Password()
		if _, hasPassword := parsed.User.Password(); !hasPassword {
			if user, password, ok := decodeUserinfo(spec.Username); ok {
				spec.Username, spec.Password = user, password
			}
		}
	}
	name, _ := url.PathUnescape(parsed.Fragment)
	if err := Normalize(&spec); err != nil {
		return Spec{}, "", err
	}
	return spec, strings.TrimSpace(name), nil
}

// decodeUserinfo reads v2rayN's base64("user:pass") userinfo.
func decodeUserinfo(value string) (string, string, bool) {
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		decoded, err := encoding.DecodeString(value)
		if err != nil {
			continue
		}
		user, password, ok := strings.Cut(string(decoded), ":")
		if ok && user != "" {
			return user, password, true
		}
	}
	return "", "", false
}

// parsePlain reads host:port, user:pass@host:port and host:port:user:pass.
func parsePlain(text string) (Spec, string, error) {
	spec := Spec{Type: TypeSOCKS5}
	if userinfo, address, ok := strings.Cut(text, "@"); ok {
		user, password, _ := strings.Cut(userinfo, ":")
		spec.Username, spec.Password = user, password
		text = address
	}
	if host, port, err := net.SplitHostPort(text); err == nil {
		spec.Host = host
		spec.Port, err = strconv.Atoi(port)
		if err != nil {
			return Spec{}, "", errors.New("invalid proxy port")
		}
	} else {
		parts := strings.Split(text, ":")
		if len(parts) != 4 || spec.Username != "" {
			return Spec{}, "", errors.New("unsupported proxy format; use socks5://, http:// or host:port:user:pass")
		}
		spec.Host = parts[0]
		spec.Port, err = strconv.Atoi(parts[1])
		if err != nil {
			return Spec{}, "", errors.New("invalid proxy port")
		}
		spec.Username, spec.Password = parts[2], parts[3]
	}
	if err := Normalize(&spec); err != nil {
		return Spec{}, "", err
	}
	return spec, "", nil
}
