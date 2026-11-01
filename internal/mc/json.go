package mc

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode/utf16"
)

func NTJson(v interface{}) (string, error) {
	s, err := NTJsonNoNL(v)
	if err != nil {
		return "", err
	}
	return s + "\n", nil
}

func NTJsonNoNL(v interface{}) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}

	s := strings.TrimRight(buf.String(), "\n")
	return escapeNonASCII(s), nil
}

func MustNTJson(v interface{}) string {
	s, err := NTJson(v)
	if err != nil {
		return ""
	}
	return s
}

func escapeNonASCII(s string) string {
	if isASCII(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 32)
	for _, r := range s {
		if r < 0x80 {
			b.WriteRune(r)
			continue
		}
		if r > 0xFFFF {
			r1, r2 := utf16.EncodeRune(r)
			writeU(&b, uint16(r1))
			writeU(&b, uint16(r2))
		} else {
			writeU(&b, uint16(r))
		}
	}
	return b.String()
}

func writeU(b *strings.Builder, u uint16) {
	const hexd = "0123456789abcdef"
	b.WriteString("\\u")
	b.WriteByte(hexd[(u>>12)&0xF])
	b.WriteByte(hexd[(u>>8)&0xF])
	b.WriteByte(hexd[(u>>4)&0xF])
	b.WriteByte(hexd[u&0xF])
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
