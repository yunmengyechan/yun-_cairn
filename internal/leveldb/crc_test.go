package leveldb

import (
	"fmt"
	"testing"
)

func TestMaskCRC32C(t *testing.T) {

	cases := []struct {
		in   string
		want uint32
	}{

		{"", 0xa282ead8},
		{"a", 0x28e46e78},
		{"abc", 0x21f1576e},
		{"hello", 0x191c1fbb},
	}

	for _, c := range cases {
		got := maskedCRC32C([]byte(c.in))
		fmt.Printf("  maskedCRC32C(%q) = 0x%08x\n", c.in, got)
		if got != c.want {
			t.Errorf("  %q: 得到 0x%08x，期望 0x%08x", c.in, got, c.want)
		}
	}

	ieee := crc32IEEE([]byte("abc"))
	our := maskedCRC32C([]byte("abc"))
	fmt.Printf("  对比 abc：IEEE=0x%08x  maskedCRC32C=0x%08x（必须不同）\n", ieee, our)
}

func TestBlockSplitting(t *testing.T) {
	pairs := map[string][]byte{}
	for i := 0; i < 200; i++ {
		k := fmt.Sprintf("key/%04d", i)
		v := make([]byte, 40)
		for j := range v {
			v[j] = byte(i)
		}
		pairs[k] = v
	}

	data, err := WriteTable(pairs, &WriteOptions{BlockSize: 512})
	if err != nil {
		t.Fatalf("WriteTable: %v", err)
	}
	fmt.Printf("  200 条 × ~50 字节，BlockSize=512 → 表 %d 字节\n", len(data))

	tb, err := Open(data)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got := map[string][]byte{}
	tb.Iterate(func(k, v []byte) bool {
		got[string(k)] = append([]byte(nil), v...)
		return true
	})
	fmt.Printf("  读回 %d 条\n", len(got))
	if len(got) != len(pairs) {
		t.Errorf("条数不符：%d vs %d", len(got), len(pairs))
	}
	bad := 0
	for k, v := range pairs {
		if string(got[k]) != string(v) {
			bad++
		}
	}
	if bad > 0 {
		t.Errorf("%d 条值不符", bad)
	} else {
		fmt.Printf("  ✅ 分块后读出逐条一致\n")
	}

	idxRaw, err := tb.readRawBlock(tb.indexHandle)
	if err != nil {
		t.Fatalf("读 index: %v", err)
	}
	n := 0
	iterBlockEntries(idxRaw, func(k, v []byte) bool {
		n++
		return true
	})
	fmt.Printf("  index block 里 %d 个 data block 句柄\n", n)
	if n < 2 {
		t.Errorf("分块没生效：只有 %d 个 data block", n)
	} else {
		fmt.Printf("  ✅ 分块生效（%d 块）\n", n)
	}
}

func crc32IEEE(b []byte) uint32 {
	return crc32ChecksumIEEE(b)
}
