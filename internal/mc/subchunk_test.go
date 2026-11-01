package mc

import (
	"encoding/binary"
	"fmt"
	"os"
	"testing"
)

func TestDecodeSubChunkSynthetic(t *testing.T) {
	if os.Getenv("PH_REPO") != "" {
		os.Chdir(os.Getenv("PH_REPO"))
	}
	const hashBtn = 0x09129f50

	var b []byte
	b = append(b, 9)
	b = append(b, 1)
	b = append(b, 4)
	b = append(b, 4<<1)

	idx := make([]byte, 512*4)

	w0 := uint32(0)
	for i := 0; i < 8; i++ {
		w0 |= uint32(1) << uint(i*4)
	}
	binary.LittleEndian.PutUint32(idx[0:4], w0)
	w1 := uint32(0)
	for i := 0; i < 8; i++ {
		w1 |= uint32(1) << uint(i*4)
	}
	binary.LittleEndian.PutUint32(idx[4:8], w1)
	b = append(b, idx...)

	b = append(b, zigzagVarint(2)...)
	b = append(b, 0)

	b = append(b, zigzagVarint(uint64(hashBtn))...)

	bs, _, err := DecodeSubChunk(b)
	if err != nil {
		t.Fatalf("DecodeSubChunk: %v", err)
	}
	fmt.Printf("  解出 %d 个非空气方块\n", len(bs))
	if len(bs) != 16 {
		t.Fatalf("期望 16 个方块，得到 %d", len(bs))
	}
	for i, blk := range bs {
		want := BlockAt{X: 0, Y: i, Z: 0}
		if blk.X != want.X || blk.Y != want.Y || blk.Z != want.Z {
			t.Errorf("第 %d 个：得到 (%d,%d,%d)，期望 (%d,%d,%d)",
				i, blk.X, blk.Y, blk.Z, want.X, want.Y, want.Z)
		}
	}
	fmt.Printf("  坐标顺序 (x,z,y) 验证通过：前 16 个是 (0,0,0)..(0,15,0) 一列\n")
	fmt.Printf("  首个方块名: %s\n", bs[0].Name)
}

func zigzagVarint(v uint64) []byte {
	zz := (v << 1) ^ (v >> 63)
	var out []byte
	for {
		c := byte(zz & 0x7F)
		zz >>= 7
		if zz != 0 {
			c |= 0x80
		}
		out = append(out, c)
		if zz == 0 {
			break
		}
	}
	return out
}
