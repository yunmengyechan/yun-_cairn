package snappy

import (
	"bytes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	cases := [][]byte{
		{},
		{'a'},
		[]byte("hello world"),
		bytes.Repeat([]byte("abc"), 100),
		bytes.Repeat([]byte{0}, 1000),
		bytes.Repeat([]byte("0123456789"), 20000),
	}
	for i, in := range cases {
		enc := CompressLiteral(in)
		out, err := Decompress(enc)
		if err != nil {
			t.Fatalf("用例 %d 解压失败: %v", i, err)
		}
		if !bytes.Equal(in, out) {
			t.Fatalf("用例 %d 往返不一致：输入 %d 字节，出来 %d 字节", i, len(in), len(out))
		}
		t.Logf("  用例 %d: %d -> 编码 %d -> 解码 %d  ✅", i, len(in), len(enc), len(out))
	}
}

func TestCopyTags(t *testing.T) {

	enc := []byte{0x08}
	enc = append(enc, 0x0C, 'a', 'b', 'c', 'd')
	enc = append(enc, 0x0E, 0x04, 0x00)
	out, err := Decompress(enc)
	if err != nil {
		t.Fatalf("copy2 不重叠: %v", err)
	}
	if string(out) != "abcdabcd" {
		t.Fatalf("copy2 不重叠: 得到 %q，期望 abcdabcd", out)
	}
	t.Logf("  copy2 不重叠 ✅ %q", out)

	enc2 := []byte{0x06}
	enc2 = append(enc2, 0x00, 'a')
	enc2 = append(enc2, 0x12, 0x01, 0x00)
	out2, err := Decompress(enc2)
	if err != nil {
		t.Fatalf("copy2 重叠: %v", err)
	}
	if string(out2) != "aaaaaa" {
		t.Fatalf("copy2 重叠: 得到 %q，期望 aaaaaa", out2)
	}
	t.Logf("  copy2 重叠(RLE) ✅ %q", out2)

	enc3 := []byte{0x06}
	enc3 = append(enc3, 0x04, 'x', 'y')
	enc3 = append(enc3, 0x01, 0x02)
	out3, err := Decompress(enc3)
	if err != nil {
		t.Fatalf("copy1: %v", err)
	}
	if string(out3) != "xyxyxy" {
		t.Fatalf("copy1: 得到 %q，期望 xyxyxy", out3)
	}
	t.Logf("  copy1 ✅ %q", out3)
}

func TestCorrupt(t *testing.T) {
	bad := [][]byte{
		{},
		{0xFF, 0xFF, 0xFF},
		{0x10, 0x0C, 0x00},
		{0x04, 0x00, 0x61},
	}
	for i, b := range bad {
		if _, err := Decompress(b); err == nil {
			t.Errorf("用例 %d 应该报错但没报", i)
		} else {
			t.Logf("  用例 %d 正确报错: %v", i, err)
		}
	}
}
