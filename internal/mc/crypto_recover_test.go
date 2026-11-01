package mc

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestRecoverAfterPacketLoss(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	srv, _ := NewGameEncryption(key)
	cli, _ := NewGameEncryption(key)

	pkts := make([][]byte, 5)
	enc := make([][]byte, 5)
	for i := range pkts {
		pkts[i] = make([]byte, 40)
		rand.Read(pkts[i])
		enc[i] = srv.Encrypt(pkts[i])
	}

	got, err := cli.Decrypt(enc[0])
	if err != nil {
		t.Fatalf("0 号包就失败了: %v", err)
	}
	if !bytes.Equal(got, pkts[0]) {
		t.Fatal("0 号包内容不对")
	}

	got, err = cli.Decrypt(enc[4])
	if err != nil {
		t.Fatalf("丢包后没能恢复: %v", err)
	}
	if !bytes.Equal(got, pkts[4]) {
		t.Fatal("恢复出来的内容不对")
	}

	more := make([]byte, 30)
	rand.Read(more)
	got, err = cli.Decrypt(srv.Encrypt(more))
	if err != nil {
		t.Fatalf("恢复后下一个包又失败了: %v", err)
	}
	if !bytes.Equal(got, more) {
		t.Fatal("恢复后的内容不对")
	}
}

func TestRecoverSingleLoss(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	srv, _ := NewGameEncryption(key)
	cli, _ := NewGameEncryption(key)

	a := make([]byte, 60)
	rand.Read(a)
	b := make([]byte, 60)
	rand.Read(b)

	ea := srv.Encrypt(a)
	_ = srv.Encrypt(b)

	c := make([]byte, 60)
	rand.Read(c)
	ec := srv.Encrypt(c)

	if _, err := cli.Decrypt(ea); err != nil {
		t.Fatal("a 失败")
	}
	got, err := cli.Decrypt(ec)
	if err != nil {
		t.Fatalf("丢一个包后没能恢复: %v", err)
	}
	if !bytes.Equal(got, c) {
		t.Fatal("恢复内容不对")
	}
}

func TestNoLossNoRecover(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	srv, _ := NewGameEncryption(key)
	cli, _ := NewGameEncryption(key)

	for i := 0; i < 20; i++ {
		m := make([]byte, 50+i*7)
		rand.Read(m)
		got, err := cli.Decrypt(srv.Encrypt(m))
		if err != nil {
			t.Fatalf("第 %d 个包失败: %v", i, err)
		}
		if !bytes.Equal(got, m) {
			t.Fatalf("第 %d 个包内容不对", i)
		}
	}
}
