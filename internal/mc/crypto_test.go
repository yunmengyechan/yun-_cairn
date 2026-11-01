package mc

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestECDH相等(t *testing.T) {
	a, err := NewKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	salt := make([]byte, 16)
	rand.Read(salt)

	k1, err := DeriveSessionKey(a, b.PublicKeyRaw(), salt)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := DeriveSessionKey(b, a.PublicKeyRaw(), salt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatal("双方会话密钥不一致")
	}
	if len(k1) != 32 {
		t.Fatalf("会话密钥长度应为 32，实际 %d", len(k1))
	}
	t.Logf("✓ ECDH P-384 一致，session_key=%x...", k1[:8])
}

func Test加密往返(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	enc, err := NewGameEncryption(key)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := NewGameEncryption(key)
	if err != nil {
		t.Fatal(err)
	}

	msgs := [][]byte{
		[]byte("hello"),
		[]byte("world!!"),
		[]byte("0123456789abcdef"),
	}
	for i, m := range msgs {
		ct := enc.Encrypt(m)
		pt, err := dec.Decrypt(ct)
		if err != nil {
			t.Fatalf("第 %d 包解密失败: %v", i, err)
		}
		if !bytes.Equal(pt, m) {
			t.Fatalf("第 %d 包内容不一致: %q != %q", i, pt, m)
		}
	}
	t.Log("✓ 流式 AES-256-GCM 往返正确（3 个连续包）")
}

func Test签名检测(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	enc, _ := NewGameEncryption(key)
	dec, _ := NewGameEncryption(key)
	ct := enc.Encrypt([]byte("test data"))
	ct[0] ^= 0x01
	if _, err := dec.Decrypt(ct); err == nil {
		t.Fatal("篡改后竟然验签通过")
	}
	t.Log("✓ 篡改能被检出")
}

func TestVarint(t *testing.T) {
	for _, v := range []uint32{0, 1, 127, 128, 255, 300, 16383, 16384, 0xffffff} {
		b := WriteVarint(v)
		got, n := ReadVarint(b, 0)
		if got != v || n != len(b) {
			t.Fatalf("varint %d 往返失败: got=%d n=%d len=%d", v, got, n, len(b))
		}
	}
	t.Log("✓ varint 往返正确")
}
