package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/util/common"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// One-click VLESS nodes. Every variant imports from a plain vless:// link into
// v2rayN, v2rayNG, Shadowrocket, Anywhere and PassWall / PassWall 2 (ENC
// needs a client with VLESS Encryption: v2rayN, v2rayNG and PassWall with
// Xray-core, Anywhere on iOS 26+):
//
//   - reality-vision: VLESS + REALITY + XTLS Vision over TCP. No certificate to
//     trust and indistinguishable from the imitated TLS 1.3 site. On sing-box
//     its REALITY server also accepts clients whose fingerprint lacks the
//     X25519MLKEM768 key share (Shadowrocket, Anywhere before iOS 26), which
//     Xray-core 26.9.8+ servers reject.
//   - reality-xhttp: VLESS + REALITY + XHTTP (Xray-core only).
//   - reality-xhttp-vision: VLESS + REALITY + XHTTP with XTLS Vision (Xray-core
//     only). Xray-core runs Vision on XHTTP only underneath VLESS Encryption
//     ("XTLS only supports TLS and REALITY directly" otherwise), so the node
//     also carries a VLESS Encryption key.
//   - enc-vision: VLESS Encryption (post-quantum mlkem768x25519plus) with XTLS
//     Vision over raw TCP, without a TLS layer (Xray-core only).
//   - enc-xhttp: VLESS Encryption over XHTTP (Xray-core only).
//   - tls: the original self-signed TLS node (TCP + Vision on sing-box, XHTTP
//     on Xray-core); requests from older controllers use it.
const (
	VlessVariantRealityVision      = "reality-vision"
	VlessVariantRealityXHTTP       = "reality-xhttp"
	VlessVariantRealityXHTTPVision = "reality-xhttp-vision"
	VlessVariantEncVision          = "enc-vision"
	VlessVariantEncXHTTP           = "enc-xhttp"
	VlessVariantTLS                = "tls"

	vlessVisionFlow          = "xtls-rprx-vision"
	quickAddRealityPrefix    = "reality-"
	quickAddRealityProbeTime = 6 * time.Second
)

var vlessVariantXrayOnly = map[string]bool{
	VlessVariantRealityXHTTP: true, VlessVariantRealityXHTTPVision: true,
	VlessVariantEncVision: true, VlessVariantEncXHTTP: true,
}

func normalizeQuickAddVlessVariant(request *RemoteQuickAddRequest) error {
	variant := strings.ToLower(strings.TrimSpace(request.VlessVariant))
	switch variant {
	case "":
		variant = VlessVariantTLS
	case VlessVariantRealityVision, VlessVariantRealityXHTTP, VlessVariantRealityXHTTPVision,
		VlessVariantEncVision, VlessVariantEncXHTTP, VlessVariantTLS:
	default:
		return common.NewErrorf("unsupported VLESS variant %q", request.VlessVariant)
	}
	if vlessVariantXrayOnly[variant] && request.CoreType != model.CoreTypeXray {
		return common.NewErrorf("the VLESS %s variant needs Xray-core", variant)
	}
	if vlessVariantReality(variant) && request.CoreType != model.CoreTypeXray && !sdwanRealitySupported {
		return common.NewError("this build of sing-box has no REALITY support; choose Xray-core or the TLS variant")
	}
	realityServer, err := normalizeRealityServer(request.RealityServer)
	if err != nil {
		return err
	}
	request.VlessVariant = variant
	request.RealityServer = realityServer
	return nil
}

// effectiveVlessVariant treats an unset variant (older controllers) as the
// original self-signed TLS node.
func effectiveVlessVariant(request RemoteQuickAddRequest) string {
	if request.VlessVariant == "" {
		return VlessVariantTLS
	}
	return request.VlessVariant
}

func vlessVariantReality(variant string) bool {
	return variant == VlessVariantRealityVision || variant == VlessVariantRealityXHTTP || variant == VlessVariantRealityXHTTPVision
}

func vlessVariantEncryption(variant string) bool {
	return variant == VlessVariantEncVision || variant == VlessVariantEncXHTTP || variant == VlessVariantRealityXHTTPVision
}

func vlessVariantXHTTP(variant string) bool {
	return variant == VlessVariantRealityXHTTP || variant == VlessVariantEncXHTTP || variant == VlessVariantRealityXHTTPVision
}

// quickAddVlessFlow is the user flow of a variant: XTLS Vision runs on raw TCP
// under REALITY, TLS or VLESS Encryption, and on XHTTP only in the variant
// that puts VLESS Encryption underneath it.
func quickAddVlessFlow(request RemoteQuickAddRequest) string {
	switch effectiveVlessVariant(request) {
	case VlessVariantRealityVision, VlessVariantEncVision, VlessVariantRealityXHTTPVision:
		return vlessVisionFlow
	case VlessVariantTLS:
		if request.CoreType != model.CoreTypeXray {
			return vlessVisionFlow
		}
	}
	return ""
}

// quickAddVlessInbound fills the transport and, for VLESS Encryption, the
// server and client keys of a VLESS quick-add inbound.
func quickAddVlessInbound(inbound map[string]interface{}, request RemoteQuickAddRequest, publicHost string) error {
	isXray := request.CoreType == model.CoreTypeXray
	variant := effectiveVlessVariant(request)
	switch {
	case vlessVariantXHTTP(variant):
		// An empty host accepts any Host header, so clients may omit it.
		inbound["transport"] = map[string]interface{}{"type": "xhttp", "path": randomXHTTPPath(), "host": "", "mode": "auto"}
	case variant == VlessVariantTLS && isXray:
		inbound["transport"] = map[string]interface{}{"type": "xhttp", "path": "/xhttp", "host": publicHost, "mode": "auto"}
	case isXray:
		inbound["transport"] = map[string]interface{}{"type": "tcp"}
	default:
		inbound["transport"] = map[string]interface{}{}
	}
	if vlessVariantEncryption(variant) {
		decryption, encryption, err := generateVlessEncryption()
		if err != nil {
			return err
		}
		inbound["decryption"] = decryption
		// The client half is kept for share links only; Xray-core ignores it.
		inbound["encryption"] = encryption
	}
	return nil
}

func randomXHTTPPath() string {
	return "/" + strings.ToLower(common.Random(12))
}

// generateVlessEncryption returns a VLESS Encryption pair as `xray vlessenc`
// does, with X25519 client authentication. The key exchange itself is
// post-quantum (ML-KEM-768 + X25519) either way, and the 32-byte key keeps
// links short enough for QR codes; an ML-KEM-768 key would add 1.5 KB.
func generateVlessEncryption() (decryption, encryption string, err error) {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return "", "", err
	}
	publicKey := privateKey.PublicKey()
	decryption = "mlkem768x25519plus.native.600s." + base64.RawURLEncoding.EncodeToString(privateKey[:])
	encryption = "mlkem768x25519plus.native.0rtt." + base64.RawURLEncoding.EncodeToString(publicKey[:])
	return decryption, encryption, nil
}

// createQuickAddRealityTLS creates the REALITY keys shared by one quick-add
// batch. Without an explicit target, the server picks the fastest reachable
// TLS 1.3 + HTTP/2 site it can imitate.
func (s *LocalControlService) createQuickAddRealityTLS(request RemoteQuickAddRequest, revision uint64, changeActor, publicHost string) (uint, uint64, error) {
	serverName := request.RealityServer
	if serverName == "" {
		ctx, cancel := context.WithTimeout(context.Background(), quickAddRealityProbeTime)
		serverName = probeRealityServer(ctx)
		cancel()
	}
	name, err := availableTLSName(quickAddRealityPrefix + strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' {
			return r
		}
		return '-'
	}, serverName))
	if err != nil {
		return 0, revision, err
	}
	return s.createRealityTLS(name, serverName, revision, changeActor, publicHost)
}

func availableTLSName(base string) (string, error) {
	for copyIndex := 0; ; copyIndex++ {
		candidate := base
		if copyIndex > 0 {
			candidate = fmt.Sprintf("%s-copy%d", base, copyIndex)
		}
		var count int64
		if err := database.GetDB().Model(&model.Tls{}).Where("name = ?", candidate).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
	}
}

// createRealityTLS saves a REALITY server/client pair: an x25519 key, one
// short ID and the site to imitate, with the Chrome fingerprint for clients.
func (s *LocalControlService) createRealityTLS(name, serverName string, revision uint64, changeActor, publicHost string) (uint, uint64, error) {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return 0, revision, err
	}
	publicKey := privateKey.PublicKey()
	shortID := make([]byte, 8)
	if _, err = rand.Read(shortID); err != nil {
		return 0, revision, err
	}
	tlsConfig := map[string]interface{}{
		"id":   0,
		"name": name,
		"server": map[string]interface{}{
			"enabled":     true,
			"server_name": serverName,
			"reality": map[string]interface{}{
				"enabled":     true,
				"handshake":   map[string]interface{}{"server": serverName, "server_port": 443},
				"private_key": base64.RawURLEncoding.EncodeToString(privateKey[:]),
				"short_id":    []string{hex.EncodeToString(shortID)},
			},
		},
		"client": map[string]interface{}{
			"utls": map[string]interface{}{"enabled": true, "fingerprint": sdwanRealityFingerprint},
			"reality": map[string]interface{}{
				"enabled": true, "public_key": base64.RawURLEncoding.EncodeToString(publicKey[:]),
				"short_id": hex.EncodeToString(shortID),
			},
		},
	}
	raw, err := json.Marshal(tlsConfig)
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
	return saved.Id, revision, nil
}
