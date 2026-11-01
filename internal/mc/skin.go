package mc

import (
	_ "embed"
	"crypto/ecdsa"
	"encoding/binary"
	"os"
	"strconv"
	"strings"
)

//go:embed skin_data.json
var embeddedSkinTemplate string

func SignSkinData(rawSkin string, key *ecdsa.PrivateKey) (string, error) {
	if rawSkin == "" {
		return "", nil
	}
	pubB64, err := PublicKeyB64(&key.PublicKey)
	if err != nil {
		return "", err
	}
	header := map[string]interface{}{
		"alg": "ES384",
		"x5u": pubB64,
	}
	hj, err := NTJson(header)
	if err != nil {
		return "", err
	}
	encHeader := B64Url([]byte(hj))
	encPayload := B64Url([]byte(rawSkin))
	signingInput := encHeader + "." + encPayload
	sig, err := ES384RawSign(key, []byte(signingInput))
	if err != nil {
		return "", err
	}
	return signingInput + "." + B64Url(sig), nil
}

func LoadSkinData(path, displayName string) string {
	if path == "" {
		path = "skin_data.json"
	}

	b, err := os.ReadFile(path)
	if err != nil {

		b = []byte(embeddedSkinTemplate)
	}
	s := string(b)
	if len(s) == 0 || displayName == "" {
		return s
	}
	return patchClientData(s, displayName)
}

func patchJSONStringValue(raw, key, val string) string {
	q := string(rune(34))
	k := q + key + q + ":" + q
	i := strings.Index(raw, k)
	if i < 0 {
		return raw
	}
	st := i + len(k)
	en := strings.Index(raw[st:], q)
	if en < 0 {
		return raw
	}
	return raw[:st] + escapeNonASCII(val) + raw[st+en:]
}

func patchClientData(raw, displayName string) string {
	out := patchJSONStringValue(raw, "ThirdPartyName", displayName)
	out = patchJSONStringValue(out, "DeviceId", displayName)
	var rb [8]byte
	for i := range rb {
		rb[i] = byte(randIntn(256))
	}
	rid := strconv.FormatUint(binary.BigEndian.Uint64(rb[:])&0x1FFFFFFFFFFFFF, 10)
	return patchJSONStringValue(out, "ClientRandomId", rid)
}
