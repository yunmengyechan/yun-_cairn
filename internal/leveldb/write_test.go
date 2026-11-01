package leveldb

import (
	"encoding/binary"
	"fmt"
	"testing"
)

func TestWriteRoundTrip(t *testing.T) {

	pairs := map[string][]byte{}
	for x := 0; x < 3; x++ {
		for z := 0; z < 3; z++ {
			for y := 0; y < 2; y++ {
				k := chunkKey(int32(x), int32(z), TagSubChunk, int8(y))

				v := []byte{9, 1, byte(y), 0x22}
				pairs[string(k)] = v
			}
		}
	}

	for i := 0; i < 60; i++ {
		k := fmt.Sprintf("prefix/very/long/common/key/%03d", i)
		pairs[k] = []byte(fmt.Sprintf("value-%03d", i))
	}
	fmt.Printf("  写入 %d 条键值\n", len(pairs))

	data, err := WriteTable(pairs, nil)
	if err != nil {
		t.Fatalf("WriteTable: %v", err)
	}
	fmt.Printf("  表大小 %d 字节\n", len(data))

	tb, err := Open(data)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	fmt.Printf("  footer=%d 字节  index@%d+%d\n", tb.FooterSize, tb.indexHandle.Offset, tb.indexHandle.Size)

	got := map[string][]byte{}
	err = tb.Iterate(func(k, v []byte) bool {
		kc := append([]byte(nil), k...)
		vc := append([]byte(nil), v...)
		got[string(kc)] = vc
		return true
	})
	if err != nil {
		t.Fatalf("Iterate: %v", err)
	}
	fmt.Printf("  读回 %d 条\n", len(got))

	if len(got) != len(pairs) {
		t.Errorf("条数不符：写入 %d 读回 %d", len(pairs), len(got))
	}
	diff := 0
	for k, v := range pairs {
		g, ok := got[k]
		if !ok {
			if diff < 3 {
				fmt.Printf("    丢失键 %q\n", k[:min(20, len(k))])
			}
			diff++
			continue
		}
		if string(g) != string(v) {
			if diff < 3 {
				fmt.Printf("    值不符 %q: %q vs %q\n", k[:min(20, len(k))], g, v)
			}
			diff++
		}
	}
	if diff > 0 {
		t.Errorf("%d 条不一致", diff)
	} else {
		fmt.Printf("  ✅ 写入→读回 逐条一致\n")
	}

	ck, ok := ParseChunkKey(chunkKey(2, 1, TagSubChunk, 3))
	if !ok || ck.X != 2 || ck.Z != 1 || ck.SubY == nil || *ck.SubY != 3 {
		t.Errorf("ParseChunkKey 结果不对：%+v", ck)
	} else {
		fmt.Printf("  ✅ 区块键解析正确：(%d,%d) %s subY=%d\n", ck.X, ck.Z, TagName(ck.Tag), *ck.SubY)
	}
}

func chunkKey(x, z int32, tag byte, subY int8) []byte {
	b := make([]byte, 10)
	binary.LittleEndian.PutUint32(b[0:4], uint32(x))
	binary.LittleEndian.PutUint32(b[4:8], uint32(z))
	b[8] = tag
	b[9] = byte(subY)
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
