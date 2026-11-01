package mc

import (
	"bytes"
	"os"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const AuthServerURL = "https://g79authobt.minecraft.cn"

const AuthPathV2 = "/authentication-v2"

const (
	EngineVersion = "3.10.5.301327"
	PatchVersion  = "3.10.6.301389"
)

type ChainResult struct {
	Chain string
	Key   *ecdsa.PrivateKey
}

func FetchAuthChain(token, userID, displayName, neteaseSID string, key *ecdsa.PrivateKey) (*ChainResult, error) {
	if key == nil {
		k, err := NewECDSAKey()
		if err != nil {
			return nil, err
		}
		key = k
	}
	clientKey, err := PublicKeyB64(&key.PublicKey)
	if err != nil {
		return nil, err
	}

	uid := interface{}(userID)
	if n, e := strconv.ParseInt(userID, 10, 64); e == nil {
		uid = n
	}
	bodyObj := map[string]interface{}{
		"bit":           "64",
		"clientKey":     clientKey,
		"displayName":   displayName,
		"engineVersion": EngineVersion,
		"netease_sid":   neteaseSID,
		"os_name":       "android",
		"patchVersion":  PatchVersion,
		"uid":           uid,
	}

	bodyJSONStr, err := NTJsonNoNL(bodyObj)
	if err != nil {
		return nil, err
	}
	bodyJSON := []byte(bodyJSONStr)
	if err != nil {
		return nil, err
	}

	enc, err := HttpEncrypt(bodyJSON)
	if err != nil {
		return nil, err
	}
	hexBody := hex.EncodeToString(enc)

	if os.Getenv("PH_DEBUG") != "" {
		fmt.Printf("[DBG] body     = %s\n", string(bodyJSON))
		fmt.Printf("[DBG] body_len = %d\n", len(bodyJSON))
		fmt.Printf("[DBG] clientKey= %s\n", clientKey)
		fmt.Printf("[DBG] enc_len  = %d  head=%x\n", len(enc), enc[:min(24, len(enc))])
	}
	dynToken := ComputeDynamicToken(AuthPathV2, string(bodyJSON), token)
	if os.Getenv("PH_DEBUG") != "" {
		fmt.Printf("[DBG] token_raw= %s\n", dynToken)
		fmt.Printf("[DBG] token_hdr= %s\n", TokenToHeader(dynToken))
		fmt.Printf("[DBG] url      = %s\n", AuthServerURL+AuthPathV2)
	}

	req, err := http.NewRequest("POST", AuthServerURL+AuthPathV2,
		bytes.NewReader([]byte(hexBody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "libhttpclient/1.0.0.0")

	req.Header.Set("user-token", TokenToHeader(dynToken))
	req.Header.Set("user-id", userID)
	req.Header.Set("Content-Type", "application/json")

	cli := &http.Client{Timeout: 15 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("认证服 HTTP %d: %s", resp.StatusCode, string(raw[:min(200, len(raw))]))
	}

	text := string(bytes.TrimSpace(raw))
	if len(text) >= 2 && text[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			text = s
		}
	}
	ct, err := hex.DecodeString(text)
	if err != nil {
		return nil, fmt.Errorf("响应不是 hex: %w", err)
	}
	pt, err := HttpDecrypt(ct)
	if err != nil {
		return nil, fmt.Errorf("响应解密失败: %w", err)
	}

	reissued, err := ReissueChain(string(pt), key)
	if err != nil {
		return nil, fmt.Errorf("ReissueChain failed: %w", err)
	}
	return &ChainResult{Chain: reissued, Key: key}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
