package mc

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func MCPRotorKey() []byte {
	bt := string(rune(96))
	q := string(rune(34))
	dn := "j2h56ogodh3sk"
	dt := "=dziaq;"
	df := "|" + bt + "o=5v7!" + q + "-276"
	s := strings.Repeat(dn, 4) +
		strings.Repeat(dt+dn+df, 5) +
		"$" + "@" +
		strings.Repeat(dt, 7) +
		strings.Repeat(df, 2) +
		"&" + "]" + bt
	return []byte(s)
}

func mcpReverseData(d []byte) []byte {
	L := make([]byte, len(d))
	copy(L, d)
	n := 130
	if len(L) < n {
		n = len(L)
	}
	for i := 0; i < n; i++ {
		L[i] = L[i] ^ 156
	}
	for i, j := 0, len(L)-1; i < j; i, j = i+1, j-1 {
		L[i], L[j] = L[j], L[i]
	}
	return L
}

func mcpRotorDecrypt(mcp []byte) ([]byte, error) {
	raw, err := hex.DecodeString(strings.TrimSpace(string(mcp)))
	if err != nil {
		return nil, fmt.Errorf("mcp unhex 失败: %w", err)
	}
	rot := NewRotor(MCPRotorKey(), 6)
	dec := rot.Decrypt(raw)
	zr, err := zlib.NewReader(bytes.NewReader(dec))
	if err != nil {
		return nil, fmt.Errorf("mcp zlib 打开失败: %w", err)
	}
	defer zr.Close()
	plain, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("mcp zlib 解压失败: %w", err)
	}
	return mcpReverseData(plain), nil
}

func MCPCalc(mcpBlob []byte) (func(string) string, error) {
	mcp, err := mcpRotorDecrypt(mcpBlob)
	if err != nil {
		return nil, err
	}
	if len(mcp) < 145 {
		return nil, fmt.Errorf("mcp 明文太短: %d", len(mcp))
	}
	key, _ := hex.DecodeString("8D06E8C8B7D7B7284651AE04")
	offset := 141
	size := int(mcp[offset]) + 4
	if offset+size > len(mcp) {
		return nil, fmt.Errorf("str1 越界")
	}
	str1 := RC4Decrypt(key, mcp[offset+4:offset+size])
	offset += size + 1
	if offset >= len(mcp) {
		return nil, fmt.Errorf("str2 偏移越界")
	}
	size = int(mcp[offset]) + 4
	if offset+size > len(mcp) {
		return nil, fmt.Errorf("str2 越界")
	}
	str2 := RC4Decrypt(key, mcp[offset+4:offset+size])

	return func(salt string) string {
		h := md5.New()
		h.Write(str1)
		h.Write([]byte(salt))
		h.Write(str2)
		return hex.EncodeToString(h.Sum(nil))
	}, nil
}

func MCPCheckNum(mcpBlob []byte, salt string, playerID int, engineVersion, patchVersion, uid string) (any, error) {
	calc, err := MCPCalc(mcpBlob)
	if err != nil {
		return nil, err
	}
	valM := calc(salt + "0")

	rawParams := []string{"False", "[]", "", "", "3"}
	params := []any{false, []any{}, "", "", 3}

	uidInt, _ := strconv.ParseInt(uid, 10, 64)
	var tmps []any
	if uidInt > 2147483647 {
		tmps = []any{engineVersion, "android", patchVersion, "android", 2, 12, playerID}
	} else {
		tmps = []any{engineVersion, "windows", engineVersion, "win32", 0, 12, playerID}
	}
	var sb strings.Builder
	for _, c := range valM {
		sb.WriteString(strconv.Itoa((int(c)*2 + 5) ^ 255))
	}
	for _, t := range tmps {
		sb.WriteString(fmt.Sprintf("%v", t))
	}
	gameInfo := calc(sb.String())
	rawParams = append(rawParams, gameInfo)
	params = append(params, gameInfo)

	raw := valM[16:] + strings.Join(rawParams, "") + valM[:16]
	sign := calc(raw)

	inner := append([]any{valM, sign}, params...)
	return []any{"SetMCPCheckNum", []any{inner}, nil}, nil
}

func zlibCompress(d []byte) []byte {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	w.Write(d)
	w.Close()
	return buf.Bytes()
}

func BuildSyntheticMCP(str1, str2 []byte) []byte {
	key, _ := hex.DecodeString("8D06E8C8B7D7B7284651AE04")
	c1 := RC4Decrypt(key, str1)
	c2 := RC4Decrypt(key, str2)

	plain := make([]byte, 141)
	plain = append(plain, byte(len(str1)))
	plain = append(plain, 0, 0, 0, 0)
	plain = append(plain, c1...)
	plain = append(plain, 0)
	plain = append(plain, byte(len(str2)))
	plain = append(plain, 0, 0, 0, 0)
	plain = append(plain, c2...)
	plain = append(plain, make([]byte, 64)...)

	a := make([]byte, len(plain))
	copy(a, plain)
	for i, j := 0, len(a)-1; i < j; i, j = i+1, j-1 {
		a[i], a[j] = a[j], a[i]
	}
	for i := 0; i < 130 && i < len(a); i++ {
		a[i] ^= 156
	}
	comp := zlibCompress(a)
	rot := NewRotor(MCPRotorKey(), 6)
	enc := rot.Encrypt(comp)
	return []byte(hex.EncodeToString(enc))
}
