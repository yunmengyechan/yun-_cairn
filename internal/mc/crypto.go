package mc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type KeyPair struct {
	Priv *ecdh.PrivateKey
}

func NewKeyPair() (*KeyPair, error) {
	p, err := ecdh.P384().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &KeyPair{Priv: p}, nil
}

func (k *KeyPair) PublicKeyRaw() []byte {
	return k.Priv.PublicKey().Bytes()
}

func DeriveSessionKey(k *KeyPair, serverPubRaw []byte, salt []byte) ([]byte, error) {
	serverPub, err := ecdh.P384().NewPublicKey(serverPubRaw)
	if err != nil {
		return nil, fmt.Errorf("服务器公钥无效: %w", err)
	}
	shared, err := k.Priv.ECDH(serverPub)
	if err != nil {
		return nil, fmt.Errorf("ECDH 失败: %w", err)
	}
	h := sha256.New()
	h.Write(salt)
	h.Write(shared)
	return h.Sum(nil), nil
}

type rollbackCTR struct {
	block cipher.Block
	ctr   [16]byte
	pad   [16]byte
	used  int
}

type ctrState struct {
	ctr  [16]byte
	pad  [16]byte
	used int
}

func newRollbackCTR(b cipher.Block, iv []byte) *rollbackCTR {
	c := &rollbackCTR{block: b}
	copy(c.ctr[:], iv)
	c.next()
	return c
}

func (c *rollbackCTR) next() {
	c.block.Encrypt(c.pad[:], c.ctr[:])
	for i := 15; i >= 0; i-- {
		c.ctr[i]++
		if c.ctr[i] != 0 {
			break
		}
	}
}

func (c *rollbackCTR) XORKeyStream(dst, src []byte) {
	for len(src) > 0 {
		if c.used == 16 {
			c.next()
			c.used = 0
		}
		n := 16 - c.used
		if n > len(src) {
			n = len(src)
		}
		for i := 0; i < n; i++ {
			dst[i] = src[i] ^ c.pad[c.used+i]
		}
		c.used += n
		dst, src = dst[n:], src[n:]
	}
}

func (c *rollbackCTR) save() ctrState {
	return ctrState{ctr: c.ctr, pad: c.pad, used: c.used}
}

func (c *rollbackCTR) restore(s ctrState) {
	c.ctr, c.pad, c.used = s.ctr, s.pad, s.used
}

type GameEncryption struct {
	key     []byte
	sendNum uint64
	recvNum uint64
	sendCt  *rollbackCTR
	recvCt  *rollbackCTR
	OnDebug func(string)
}

func NewGameEncryption(sessionKey []byte) (*GameEncryption, error) {
	if len(sessionKey) != 32 {
		return nil, fmt.Errorf("session_key 必须 32 字节，实际 %d", len(sessionKey))
	}
	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return nil, err
	}

	iv := sessionKey[:12]

	makeCounter := func() []byte {
		c := make([]byte, 16)
		copy(c, iv)
		c[15] = 0x02
		return c
	}
	return &GameEncryption{
		key:    append([]byte(nil), sessionKey...),
		sendCt: newRollbackCTR(block, makeCounter()),
		recvCt: newRollbackCTR(block, makeCounter()),
	}, nil
}

func (g *GameEncryption) computeSign(data []byte, num uint64) []byte {
	h := sha256.New()
	var nb [8]byte
	binary.LittleEndian.PutUint64(nb[:], num)
	h.Write(nb[:])
	h.Write(data)
	h.Write(g.key)
	return h.Sum(nil)[:8]
}

func (g *GameEncryption) Encrypt(plaintext []byte) []byte {
	sign := g.computeSign(plaintext, g.sendNum)
	g.sendNum++
	buf := make([]byte, len(plaintext)+8)
	copy(buf, plaintext)
	copy(buf[len(plaintext):], sign)
	out := make([]byte, len(buf))
	g.sendCt.XORKeyStream(out, buf)
	return out
}

func (g *GameEncryption) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < 8 {
		return nil, errors.New("密文不足 8 字节")
	}

	st := g.recvCt.save()

	plain := make([]byte, len(ciphertext))
	g.recvCt.XORKeyStream(plain, ciphertext)
	body := plain[:len(plain)-8]
	sign := plain[len(plain)-8:]
	expect := g.computeSign(body, g.recvNum)

	if equal8(sign, expect) {
		g.recvNum++
		return body, nil
	}

	g.recvCt.restore(st)

	hit := int64(-1)
	for d := int64(-4); d <= 8; d++ {
		n := int64(g.recvNum) + d
		if n < 0 {
			continue
		}
		if equal8(sign, g.computeSign(body, uint64(n))) {
			hit = n
			break
		}
	}
	if g.OnDebug != nil {
		g.OnDebug(fmt.Sprintf(
			"解密失败: recvNum=%d 期望=%x 实到=%x 命中序号=%d 明文头=%x",
			g.recvNum, expect, sign, hit, body[:min(16, len(body))]))
	}

	if pt, ok := g.recover(ciphertext, st); ok {
		return pt, nil
	}
	g.recvNum++
	return nil, fmt.Errorf("签名不匹配（recvNum=%d 命中=%d）", g.recvNum, hit)
}

var (
	RecoverWindow  = 4096
	RecoverMaxSkip = 16
)

func (g *GameEncryption) recover(ciphertext []byte, base ctrState) ([]byte, bool) {
	if len(ciphertext) < 8 {
		return nil, false
	}
	n := len(ciphertext)
	out := make([]byte, n)
	pt := make([]byte, n-8)
	var zero [1]byte

	g.recvCt.restore(base)
	for skip := 1; skip <= RecoverWindow; skip++ {

		g.recvCt.XORKeyStream(out[:1], zero[:])
		st := g.recvCt.save()

		out[0] ^= ciphertext[0]
		g.recvCt.XORKeyStream(out[1:], ciphertext[1:])

		sign := out[n-8:]
		body := out[:n-8]
		for numSkip := 0; numSkip <= RecoverMaxSkip; numSkip++ {
			if equal8(sign, g.computeSign(body, g.recvNum+uint64(numSkip))) {
				g.recvNum += uint64(numSkip) + 1
				copy(pt, body)
				if g.OnDebug != nil {
					g.OnDebug(fmt.Sprintf(
						"★ 密钥流已恢复：跳过 %d 字节 / 跳过 %d 个序号", skip, numSkip))
				}
				return pt, true
			}
		}

		g.recvCt.restore(st)
	}
	return nil, false
}

func equal8(a, b []byte) bool {
	if len(a) < 8 || len(b) < 8 {
		return false
	}
	for i := 0; i < 8; i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (g *GameEncryption) SendNum() uint64 { return g.sendNum }

func ParseServerHandshakeJWT(token string) (pubDER []byte, salt []byte, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, nil, fmt.Errorf("JWT 段数不是 3: %d", len(parts))
	}
	hdrRaw, err := b64urlDecode(parts[0])
	if err != nil {
		return nil, nil, err
	}
	payRaw, err := b64urlDecode(parts[1])
	if err != nil {
		return nil, nil, err
	}
	var hdr struct {
		X5u string `json:"x5u"`
	}
	if err := json.Unmarshal(hdrRaw, &hdr); err != nil {
		return nil, nil, err
	}
	if hdr.X5u == "" {
		return nil, nil, errors.New("JWT header 缺 x5u")
	}
	pubDER, err = base64.StdEncoding.DecodeString(hdr.X5u)
	if err != nil {
		return nil, nil, fmt.Errorf("x5u base64 解码失败: %w", err)
	}
	var pay struct {
		Salt string `json:"salt"`
	}
	if err := json.Unmarshal(payRaw, &pay); err != nil {
		return nil, nil, err
	}
	if pay.Salt == "" {
		return nil, nil, errors.New("JWT payload 缺 salt")
	}
	salt, err = base64.StdEncoding.DecodeString(pay.Salt)
	if err != nil {
		return nil, nil, fmt.Errorf("salt base64 解码失败: %w", err)
	}
	return pubDER, salt, nil
}

func b64urlDecode(s string) ([]byte, error) {

	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}
	return base64.URLEncoding.DecodeString(s)
}

func WriteVarint(v uint32) []byte {
	out := make([]byte, 0, 5)
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		out = append(out, b)
		if v == 0 {
			break
		}
	}
	return out
}

func ReadVarint(b []byte, off int) (uint32, int) {
	var result uint32
	var shift uint
	start := off
	for off < len(b) {
		x := b[off]
		off++
		result |= uint32(x&0x7f) << shift
		if x&0x80 == 0 {
			break
		}
		shift += 7
		if shift >= 35 {
			break
		}
	}
	return result, off - start
}

func WriteString(s string) []byte {
	b := WriteVarint(uint32(len(s)))
	return append(b, []byte(s)...)
}

func ReadString(b []byte, off int) (string, int, error) {
	ln, n := ReadVarint(b, off)
	off += n
	if off+int(ln) > len(b) {
		return "", off, errors.New("字符串越界")
	}
	return string(b[off : off+int(ln)]), off + int(ln), nil
}

func WriteZigzag32(v int32) []byte {
	return WriteVarint(uint32((v << 1) ^ (v >> 31)))
}

func WriteZigzag64(v uint64) []byte {
	zz := (v << 1) ^ (v >> 63)
	return WriteVarint64(zz)
}

func WriteVarint64(v uint64) []byte {
	out := make([]byte, 0, 10)
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		out = append(out, b)
		if v == 0 {
			break
		}
	}
	return out
}
