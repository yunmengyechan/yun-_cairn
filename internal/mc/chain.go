package mc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"
)

const MojangPublicKey = "MHYwEAYHKoZIzj0CAQYFK4EEACIDYgAEEsmU+IF/XeAF3yiqJ7Ko36btx6Jtd" +
	"B26wV9Eyw4AYR/nmesznkfXxwQ4B0NkSnGIZccbb2f3nFUYughKSoAcNHx+" +
	"lQm8F9h9RwhrNgeN907z06LUA2AqWcwqasxyaU0E"

func PublicKeyB64(pub *ecdsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", err
	}
	full := base64.StdEncoding.EncodeToString(der)
	if len(full) >= 186 {
		return full[26:186], nil
	}
	if len(full) > 160 {
		return full[:160], nil
	}
	return full, nil
}

func GenerateIdentityUUID(userID string) string {
	h := md5.Sum([]byte(userID))
	b := h[:]
	b[6] = (b[6] & 0x0F) | 0x30
	b[8] = (b[8] & 0x3F) | 0x80
	s := fmt.Sprintf("%x", b)
	return s[0:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:32]
}

func NewECDSAKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
}

func LoadECDSAKeyHex(h string) (*ecdsa.PrivateKey, error) {
	d, err := hex.DecodeString(strings.TrimSpace(h))
	if err != nil {
		return nil, err
	}
	if len(d) != 48 {
		return nil, fmt.Errorf("私钥应为 48 字节，实际 %d", len(d))
	}
	priv := new(ecdsa.PrivateKey)
	priv.PublicKey.Curve = elliptic.P384()
	priv.D = new(big.Int).SetBytes(d)
	priv.PublicKey.X, priv.PublicKey.Y = elliptic.P384().ScalarBaseMult(d)
	return priv, nil
}

func splitJWT(tok string) [][]byte {
	var segs [][]byte
	start := 0
	for i := 0; i <= len(tok); i++ {
		if i == len(tok) || tok[i] == '.' {
			raw, err := B64UrlDecode(tok[start:i])
			if err != nil {
				return nil
			}
			segs = append(segs, raw)
			start = i + 1
		}
	}
	if len(segs) != 3 {
		return nil
	}
	return segs
}

func ReissueChain(rawChain string, key *ecdsa.PrivateKey) (string, error) {
	var root struct {
		Chain []string `json:"chain"`
	}
	if err := json.Unmarshal([]byte(rawChain), &root); err != nil {
		return "", fmt.Errorf("解析原始链失败: %w", err)
	}
	if len(root.Chain) < 2 {
		return "", fmt.Errorf("chain 段数不足: %d", len(root.Chain))
	}

	parts := splitJWT(root.Chain[0])
	if parts == nil {
		return "", fmt.Errorf("chain[0] 不是合法 JWT")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(parts[1], &payload); err != nil {
		return "", fmt.Errorf("解析 chain[0] payload 失败: %w", err)
	}
	nbf := payload["nbf"]
	exp := payload["exp"]

	pubB64, err := PublicKeyB64(&key.PublicKey)
	if err != nil {
		return "", err
	}

	newPayload := map[string]interface{}{
		"certificateAuthority": true,
		"exp":                  exp,
		"identityPublicKey":    MojangPublicKey,
		"nbf":                  nbf,
	}
	newHeader := map[string]interface{}{
		"alg": "ES384",
		"x5u": pubB64,
	}
	newJWT, err := SignJWT(key, newHeader, newPayload)
	if err != nil {
		return "", fmt.Errorf("签新 CA JWT 失败: %w", err)
	}

	newChain := []string{newJWT, root.Chain[0], root.Chain[1]}

	chainJSON := MustNTJson(map[string]interface{}{"chain": newChain})
	cert := map[string]interface{}{
		"Certificate":        chainJSON,
		"AuthenticationType": 0,
		"Token":              "",
	}
	return MustNTJson(cert), nil
}

func BuildLocalChain(displayName, userID, neteaseSID string, key *ecdsa.PrivateKey) (string, error) {
	pubB64, err := PublicKeyB64(&key.PublicKey)
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	exp := now + 365*24*3600

	uidAny := interface{}(userID)
	if n, e := strconv.ParseInt(userID, 10, 64); e == nil {
		uidAny = n
	}

	sid := strings.TrimSuffix(neteaseSID, ":LobbyGame")
	if sid == ":LobbyGame" || sid == "" {
		sid = strings.TrimSuffix(neteaseSID, ":LobbyGame")
	}

	extra := map[string]interface{}{
		"XUID":          "",
		"identity":      GenerateIdentityUUID(userID),
		"displayName":   displayName,
		"uid":           uidAny,
		"version":       2,
		"env":           "obt",
		"platform":      "pe",
		"netease_sid":   sid,
		"game_type":     "LobbyGame",
		"engineVersion": EngineVersion,
		"patchVersion":  PatchVersion,
		"os_name":       "android",
		"bit":           "64",
	}
	payload := map[string]interface{}{
		"exp":               exp,
		"extraData":         extra,
		"identityPublicKey": pubB64,
		"iss":               "NetEase",
		"nbf":               now,
		"iat":               now,
	}
	header := map[string]interface{}{
		"alg": "ES384",
		"x5u": pubB64,
	}
	jwt, err := SignJWT(key, header, payload)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(os.Getenv("PH_CHAIN_MODE")) == "type0" {
		caPayload := map[string]interface{}{
			"certificateAuthority": true,
			"exp":                  exp,
			"identityPublicKey":    pubB64,
			"nbf":                  now,
		}
		caJWT, err := SignJWT(key, header, caPayload)
		if err != nil {
			return "", err
		}
		chainJSON := MustNTJson(map[string]interface{}{"chain": []string{caJWT, jwt}})
		cert := map[string]interface{}{
			"Certificate":        chainJSON,
			"AuthenticationType": 0,
			"Token":              "",
		}
		return MustNTJson(cert), nil
	}

	chainJSON := MustNTJson(map[string]interface{}{"chain": []string{jwt}})
	cert := map[string]interface{}{
		"Certificate":        chainJSON,
		"AuthenticationType": 2,
		"Token":              "",
	}
	return MustNTJson(cert), nil
}
