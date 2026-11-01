package mc

import (
	"encoding/binary"
	"fmt"
	"strings"

	"cairn/internal/blocks"
	"cairn/internal/nbt"
)

type DiskBlock struct {
	X, Y, Z   int
	Name      string
	States    string
	RuntimeID int
}

func DecodeDiskSubChunk(value []byte, subY int, maxBlocks int) ([]DiskBlock, error) {
	if len(value) < 2 {
		return nil, nil
	}
	if maxBlocks <= 0 {
		maxBlocks = 8192
	}

	version := value[0]

	if version != 8 && version != 9 {
		return nil, fmt.Errorf("不支持的磁盘子区块版本：%d", version)
	}
	pos := 1
	numStorages := int(value[pos])
	pos++
	if version >= 9 {
		if pos >= len(value) {
			return nil, nil
		}
		pos++
	}

	var out []DiskBlock
	for layer := 0; layer < numStorages; layer++ {
		if pos >= len(value) {
			break
		}
		header := value[pos]
		pos++
		bits := int(header >> 1)

		if bits == 0 {
			rid, n, err := rdZigzagVarintInt(value, pos)
			if err != nil {
				break
			}
			pos = n
			info := blocks.RuntimeLookup(rid)
			if isAirName(info.Name) {
				continue
			}
			for ly := 0; ly < 16; ly++ {
				for lz := 0; lz < 16; lz++ {
					for lx := 0; lx < 16; lx++ {
						out = append(out, DiskBlock{
							X: lx, Y: ly, Z: lz,
							Name: info.Name, States: info.States, RuntimeID: rid,
						})
						if len(out) >= maxBlocks {
							return out, nil
						}
					}
				}
			}
			continue
		}

		perWord := 32 / bits
		words := (4096 + perWord - 1) / perWord
		nbytes := words * 4
		if pos+nbytes > len(value) {
			break
		}
		storage := value[pos : pos+nbytes]
		pos += nbytes

		palette, np, err := readDiskPalette(value, pos)
		if err != nil || len(palette) == 0 {
			break
		}
		pos = np

		mask := uint32(1)<<uint(bits) - 1
		ws := make([]uint32, words)
		for i := 0; i < words; i++ {
			ws[i] = binary.LittleEndian.Uint32(storage[i*4 : i*4+4])
		}

		for linear := 0; linear < 4096; linear++ {
			w := linear / perWord
			bitOff := uint((linear % perWord) * bits)
			pidx := (ws[w] >> bitOff) & mask
			if int(pidx) >= len(palette) {
				continue
			}
			blk := palette[pidx]
			if isAirName(blk.Name) {
				continue
			}

			out = append(out, DiskBlock{
				X: (linear >> 8) & 0xF, Y: linear & 0xF, Z: (linear >> 4) & 0xF,
				Name: blk.Name, States: blk.States,
			})
			if len(out) >= maxBlocks {
				return out, nil
			}
		}
	}
	return out, nil
}

func readDiskPalette(value []byte, pos int) ([]blocks.Info, int, error) {

	size, p, err := rdVarUInt(value, pos)
	if err == nil && size > 0 && size < 65536 {
		if pal, np, ok := tryReadNbtPalette(value, p, int(size)); ok {
			return pal, np, nil
		}
	}

	zz, p2, err := rdZigzagVarint(value, pos)
	if err == nil && zz > 0 && zz < 65536 {
		if pal, np, ok := tryReadNbtPalette(value, p2, int(zz)); ok {
			return pal, np, nil
		}
	}
	return nil, pos, fmt.Errorf("调色板读取失败")
}

func tryReadNbtPalette(value []byte, pos int, count int) ([]blocks.Info, int, bool) {
	out := make([]blocks.Info, 0, count)
	for i := 0; i < count; i++ {
		if pos >= len(value) {
			return nil, pos, false
		}
		c, np, err := nbt.ParseAt(value, pos, true)
		if err != nil {
			return nil, pos, false
		}
		name := c.GetStr("name", "minecraft:air")
		states := "{}"
		if st := c.GetCompound("states"); st != nil {
			states = nbtStatesToJSON(st)
		}
		out = append(out, blocks.Info{Name: name, States: states})
		pos = np
	}
	return out, pos, true
}

func nbtStatesToJSON(c nbt.Compound) string {
	if len(c) == 0 {
		return "{}"
	}
	var sb strings.Builder
	sb.WriteByte('{')
	first := true
	for k, t := range c {
		if !first {
			sb.WriteByte(',')
		}
		first = false
		fmt.Fprintf(&sb, "%q:", k)
		switch v := t.Val.(type) {
		case string:
			fmt.Fprintf(&sb, "%q", v)
		default:
			fmt.Fprintf(&sb, "%v", v)
		}
	}
	sb.WriteByte('}')
	return sb.String()
}

func isAirName(n string) bool {
	return strings.Contains(strings.ToLower(n), "air")
}

func rdZigzagVarintInt(b []byte, off int) (int, int, error) {
	v, n, err := rdZigzagVarint(b, off)
	if err != nil {
		return 0, off, err
	}
	return int(int64(v)), n, nil
}
