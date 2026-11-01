package mc

import (
	"encoding/binary"
	"fmt"

	"cairn/internal/blocks"
)

type DiskSubChunkInput struct {
	YIndex int

	Blocks []uint32

	Names map[uint32]string
}

func EncodeDiskSubChunk(in *DiskSubChunkInput) ([]byte, error) {
	if in == nil {
		return nil, fmt.Errorf("输入为空")
	}
	if len(in.Blocks) != 4096 {
		return nil, fmt.Errorf("方块数必须是 4096，得到 %d", len(in.Blocks))
	}

	var pal []uint32
	seen := map[uint32]int{}
	for _, rid := range in.Blocks {
		if rid == 0 {
			continue
		}
		if _, ok := seen[rid]; !ok {
			seen[rid] = len(pal)
			pal = append(pal, rid)
		}
	}

	if len(pal) == 0 {
		out := []byte{9, 1, byte(in.YIndex)}
		out = append(out, 0<<1)
		out = append(out, 0)
		return out, nil
	}

	if len(pal) == 1 {
		out := []byte{9, 1, byte(in.YIndex)}
		out = append(out, 0<<1)
		out = append(out, writeZigzagVarintU64(uint64(pal[0]))...)
		return out, nil
	}

	bits := 4
	for {
		maxIdx := (1 << bits) - 1
		if len(pal)+1 <= maxIdx {
			break
		}
		bits++
		if bits > 16 {
			return nil, fmt.Errorf("调色板过大：%d 种", len(pal))
		}
	}

	perWord := 32 / bits
	words := (4096 + perWord - 1) / perWord
	storage := make([]byte, words*4)
	mask := uint32(1)<<uint(bits) - 1

	for linear := 0; linear < 4096; linear++ {
		rid := in.Blocks[linear]
		pidx := uint32(0)
		if rid != 0 {
			pidx = uint32(seen[rid] + 1)
		}
		w := linear / perWord
		bitOff := uint((linear % perWord) * bits)
		cur := binary.LittleEndian.Uint32(storage[w*4 : w*4+4])
		cur |= (pidx & mask) << bitOff
		binary.LittleEndian.PutUint32(storage[w*4:w*4+4], cur)
	}

	out := []byte{9, 1, byte(in.YIndex)}
	out = append(out, byte(bits<<1))
	out = append(out, storage...)

	palCount := len(pal) + 1
	out = append(out, writeUVarintU64(uint64(palCount))...)
	out = append(out, writeZigzagVarintU64(0)...)
	for _, rid := range pal {
		out = append(out, writeZigzagVarintU64(uint64(rid))...)
	}
	return out, nil
}

func writeUVarintU64(v uint64) []byte {
	var out []byte
	for {
		c := byte(v & 0x7F)
		v >>= 7
		if v != 0 {
			c |= 0x80
		}
		out = append(out, c)
		if v == 0 {
			break
		}
	}
	return out
}

func writeZigzagVarintU64(v uint64) []byte {
	zz := (v << 1) ^ (v >> 63)
	return writeUVarintU64(zz)
}

var _ = blocks.RuntimeLookup
