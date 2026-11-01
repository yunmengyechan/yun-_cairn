package mc

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"testing"
)

func TestRollbackCTRMatchesStdlib(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}

	iv := make([]byte, 16)
	rand.Read(iv[:12])
	iv[15] = 0x02

	for _, n := range []int{0, 1, 7, 8, 15, 16, 17, 31, 32, 33, 100, 1000, 4096, 5000} {
		src := make([]byte, n)
		rand.Read(src)

		want := make([]byte, n)
		cipher.NewCTR(block, append([]byte(nil), iv...)).XORKeyStream(want, src)

		got := make([]byte, n)
		newRollbackCTR(block, append([]byte(nil), iv...)).XORKeyStream(got, src)

		if !bytes.Equal(want, got) {
			t.Fatalf("长度 %d 密钥流不一致", n)
		}
	}
}

func TestRollbackCTRStreaming(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	block, _ := aes.NewCipher(key)
	iv := make([]byte, 16)
	rand.Read(iv[:12])
	iv[15] = 0x02

	sizes := []int{3, 40, 16, 1, 200, 17, 8, 999}
	all := make([]byte, 0, 2048)
	for _, s := range sizes {
		b := make([]byte, s)
		rand.Read(b)
		all = append(all, b...)
	}

	want := make([]byte, len(all))
	cipher.NewCTR(block, append([]byte(nil), iv...)).XORKeyStream(want, all)

	my := newRollbackCTR(block, append([]byte(nil), iv...))
	got := make([]byte, 0, len(all))
	off := 0
	for _, s := range sizes {
		chunk := make([]byte, s)
		my.XORKeyStream(chunk, all[off:off+s])
		got = append(got, chunk...)
		off += s
	}

	if !bytes.Equal(want, got) {
		t.Fatal("分多次异或的结果和标准库不一致")
	}
}

func TestRollbackRestoresPosition(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	block, _ := aes.NewCipher(key)
	iv := make([]byte, 16)
	rand.Read(iv[:12])
	iv[15] = 0x02

	c := newRollbackCTR(block, append([]byte(nil), iv...))

	src := make([]byte, 300)
	rand.Read(src)

	base := make([]byte, 300)
	ref := newRollbackCTR(block, append([]byte(nil), iv...))
	ref.XORKeyStream(base, src)

	st := c.save()
	junk := make([]byte, 77)
	c.XORKeyStream(junk, src[:77])
	c.restore(st)

	got := make([]byte, 300)
	c.XORKeyStream(got, src)
	if !bytes.Equal(base, got) {
		t.Fatal("回滚后密钥流位置没恢复")
	}
	_ = st
}
