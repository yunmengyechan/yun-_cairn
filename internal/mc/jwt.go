package mc

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
)

func B64Url(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func B64UrlDecode(s string) ([]byte, error) {
	s = strings.TrimRight(s, "=")
	return base64.RawURLEncoding.DecodeString(s)
}

func ES384RawSign(priv *ecdsa.PrivateKey, data []byte) ([]byte, error) {
	h := sha512.Sum384(data)
	r, s, err := ecdsa.Sign(rand.Reader, priv, h[:])
	if err != nil {
		return nil, err
	}
	out := make([]byte, 96)
	r.FillBytes(out[:48])
	s.FillBytes(out[48:])
	return out, nil
}

func ES384RawVerify(pub *ecdsa.PublicKey, data []byte, sig []byte) bool {
	if len(sig) != 96 {
		return false
	}
	h := sha512.Sum384(data)
	r := new(big.Int).SetBytes(sig[:48])
	s := new(big.Int).SetBytes(sig[48:])
	return ecdsa.Verify(pub, h[:], r, s)
}

func SignJWT(priv *ecdsa.PrivateKey, header map[string]interface{}, payload interface{}) (string, error) {
	hj, err := NTJson(header)
	if err != nil {
		return "", err
	}
	pj, err := NTJson(payload)
	if err != nil {
		return "", err
	}
	signingInput := B64Url([]byte(hj)) + "." + B64Url([]byte(pj))
	sig, err := ES384RawSign(priv, []byte(signingInput))
	if err != nil {
		return "", err
	}
	return signingInput + "." + B64Url(sig), nil
}

func DecodeJWTHeader(token string) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("JWT 段数不对")
	}
	raw, err := B64UrlDecode(parts[0])
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}
