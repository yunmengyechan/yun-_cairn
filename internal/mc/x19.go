package mc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
)

var X19Keys = [16]string{
	"MK6mipwmOUedplb6", "OtEylfId6dyhrfdn", "VNbhn5mvUaQaeOo9", "bIEoQGQYjKd02U0J",
	"fuaJrPwaH2cfXXLP", "LEkdyiroouKQ4XN1", "jM1h27H4UROu427W", "DhReQada7gZybTDk",
	"ZGXfpSTYUvcdKqdY", "AZwKf7MWZrJpGR5W", "amuvbcHw38TcSyPU", "SI4QotspbjhyFdT0",
	"VP4dhjKnDGlSJtbB", "UXDZx4KhZywQ2tcn", "NIK73ZNvNqzva4kd", "WeiW7qU766Q1YQZI",
}

const tokenSalt = "0eGsBkhl"

func PickX19Key(selector byte) []byte {
	return []byte(X19Keys[(selector>>4)&0x0F])
}

const randChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randASCII(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = randChars[rand.Intn(len(randChars))]
	}
	return b
}

func bytesRepeat(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

func aesCBCEncrypt(key, iv, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(plain)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("明文不是块整数倍: %d", len(plain))
	}
	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, plain)
	return out, nil
}

func aesCBCDecrypt(key, iv, ct []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ct)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("密文长度不是块整数倍: %d", len(ct))
	}
	out := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, ct)
	return out, nil
}

func HttpEncrypt(body []byte) ([]byte, error) {
	need := len(body) + 16
	paddedLen := ((need + 15) / 16) * 16
	padded := make([]byte, paddedLen)
	copy(padded, body)
	copy(padded[len(body):], randASCII(16))

	keyIdx := rand.Intn(15)
	selector := byte((keyIdx << 4) | 0x02)
	iv := randASCII(16)
	key := PickX19Key(selector)

	ct, err := aesCBCEncrypt(key, iv, padded)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, 16+len(ct)+1)
	out = append(out, iv...)
	out = append(out, ct...)
	out = append(out, selector)
	return out, nil
}

func HttpDecrypt(body []byte) ([]byte, error) {
	if len(body) < 18 {
		return nil, fmt.Errorf("密文太短: %d", len(body))
	}
	selector := body[len(body)-1]
	iv := body[:16]
	ct := body[16 : len(body)-1]
	key := PickX19Key(selector)
	pt, err := aesCBCDecrypt(key, iv, ct)
	if err != nil {
		return nil, err
	}
	count := 0
	idx := len(pt) - 1
	for idx >= 0 && count < 16 {
		if pt[idx] != 0 {
			count++
		}
		idx--
	}
	if idx+1 < 0 {
		return []byte{}, nil
	}
	return pt[:idx+1], nil
}

func ComputeDynamicToken(path, body, token string) string {
	tokMD5 := md5.Sum([]byte(token))
	raw := hex.EncodeToString(tokMD5[:]) + body + tokenSalt + strings.TrimRight(path, "?")
	sum := md5.Sum([]byte(raw))
	hexStr := hex.EncodeToString(sum[:])
	return FinalizeToken([]byte(hexStr))
}

func FinalizeToken(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	bits := make([]byte, 0, len(b)*8)
	for _, x := range b {
		for k := 7; k >= 0; k-- {
			if x&(1<<uint(k)) != 0 {
				bits = append(bits, 1)
			} else {
				bits = append(bits, 0)
			}
		}
	}

	rot := make([]byte, 0, len(bits))
	if len(bits) > 6 {
		rot = append(rot, bits[6:]...)
		rot = append(rot, bits[:6]...)
	} else {
		rot = append(rot, bits...)
	}

	shifted := make([]byte, len(b))
	for i := 0; i < len(b); i++ {
		var v byte
		for j := 0; j < 8; j++ {
			v = (v << 1) | rot[i*8+j]
		}
		shifted[i] = v
	}

	out := make([]byte, len(b))
	for i := range b {
		out[i] = b[i] ^ shifted[i]
	}

	s := base64.StdEncoding.EncodeToString(out)
	if len(s) < 16 {
		return ""
	}
	s = s[:16]
	s = strings.ReplaceAll(s, "+", "m")
	s = strings.ReplaceAll(s, "/", "o")
	return s + "1"
}

func TokenToHeader(raw string) string {
	return strings.ToUpper(hex.EncodeToString([]byte(raw)))
}

func randIntn(n int) int { return rand.Intn(n) }
