package mc

import (
	"encoding/binary"
	"fmt"

	"cairn/internal/blocks"
)

type BlockAt struct {
	X, Y, Z int
	Hash    uint32
	Name    string
	States  string
}

func DecodeSubChunk(payload []byte) ([]BlockAt, int, error) {
	sp, err := ParseSubChunkPalette(payload)
	if err != nil {
		return nil, 0, err
	}
	if len(sp.Layers) == 0 || len(sp.Hashes) == 0 {
		return nil, sp.StoragesEnd, nil
	}

	off := 2
	if sp.Version >= 9 {
		off++
	}
	if off >= len(payload) {
		return nil, sp.StoragesEnd, nil
	}
	pt := payload[off]
	off++
	bits := int(pt >> 1)

	if bits == 0 {
		h := sp.Hashes[0]
		if h == 0 {
			return nil, sp.StoragesEnd, nil
		}
		name, states := lookupBlock(h)
		return []BlockAt{{X: -1, Y: -1, Z: -1, Hash: h, Name: name, States: states}},
			sp.StoragesEnd, nil
	}

	if bits <= 0 || bits > 16 {
		return nil, sp.StoragesEnd, fmt.Errorf("非法位宽：%d", bits)
	}
	perWord := 32 / bits
	words := (4096 + perWord - 1) / perWord
	if off+words*4 > len(payload) {
		return nil, sp.StoragesEnd, fmt.Errorf("索引数据不足：需 %d 字节，剩 %d", words*4, len(payload)-off)
	}
	base := off
	mask := uint32(1)<<uint(bits) - 1

	ws := make([]uint32, words)
	for i := 0; i < words; i++ {
		ws[i] = binary.LittleEndian.Uint32(payload[base+i*4 : base+i*4+4])
	}

	out := make([]BlockAt, 0, 256)
	for linear := 0; linear < 4096; linear++ {
		w := linear / perWord
		bitoff := uint((linear % perWord) * bits)
		pidx := (ws[w] >> bitoff) & mask
		if int(pidx) >= len(sp.Hashes) {
			continue
		}
		h := sp.Hashes[pidx]
		if h == 0 {
			continue
		}

		lx := (linear >> 8) & 0xF
		lz := (linear >> 4) & 0xF
		ly := linear & 0xF
		name, states := lookupBlock(h)
		out = append(out, BlockAt{X: lx, Y: ly, Z: lz, Hash: h, Name: name, States: states})
	}
	return out, sp.StoragesEnd, nil
}

func lookupBlock(h uint32) (string, string) {
	info := blocks.Lookup(h)
	if info.Name == "minecraft:unknown" {

		return fmt.Sprintf("unknown(0x%08x)", h), "{}"
	}
	return info.Name, info.States
}

type SubChunkSummary struct {
	Version     byte
	YIndex      int8
	HasYIndex   bool
	Total       int
	ByBlock     map[string]int
	PaletteSize int
}

func DecodeSubChunkSummary(payload []byte) (*SubChunkSummary, error) {
	bs, _, err := DecodeSubChunk(payload)
	if err != nil {
		return nil, err
	}
	sp, _ := ParseSubChunkPalette(payload)
	sum := &SubChunkSummary{ByBlock: make(map[string]int)}
	if sp != nil {
		sum.Version = sp.Version
		sum.YIndex = sp.YIndex
		sum.HasYIndex = sp.HasYIndex
		sum.PaletteSize = len(sp.Hashes)
	}
	sum.Total = len(bs)
	for _, b := range bs {
		sum.ByBlock[b.Name]++
	}
	return sum, nil
}
