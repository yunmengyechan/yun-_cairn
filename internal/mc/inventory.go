package mc

import "fmt"

type SlotItem struct {
	NetworkID      int32
	Count          uint16
	Metadata       uint32
	HasStackID     bool
	StackID        int32
	BlockRuntimeID int32
	Extra          []byte
}

func (it SlotItem) IsAir() bool { return it.NetworkID == 0 || it.NetworkID == -1 }

func rdItem(b []byte, off int) (SlotItem, int, error) {
	var it SlotItem
	var err error
	if it.NetworkID, off, err = rdZigzag32(b, off); err != nil {
		return it, off, fmt.Errorf("networkID: %w", err)
	}
	if it.IsAir() {
		return it, off, nil
	}
	if off+2 > len(b) {
		return it, off, fmt.Errorf("count 越界")
	}
	it.Count = uint16(b[off]) | uint16(b[off+1])<<8
	off += 2
	var md uint32
	if md, off, err = rdVarUInt(b, off); err != nil {
		return it, off, fmt.Errorf("metadata: %w", err)
	}
	it.Metadata = md
	var hs byte
	if hs, off, err = rdU8(b, off); err != nil {
		return it, off, fmt.Errorf("hasStackId: %w", err)
	}
	if hs != 0 {
		it.HasStackID = true
		if it.StackID, off, err = rdZigzag32(b, off); err != nil {
			return it, off, fmt.Errorf("stackID: %w", err)
		}
	}
	if it.BlockRuntimeID, off, err = rdZigzag32(b, off); err != nil {
		return it, off, fmt.Errorf("blockRuntimeID: %w", err)
	}
	var elen uint32
	if elen, off, err = rdVarUInt(b, off); err != nil {
		return it, off, fmt.Errorf("extraLen: %w", err)
	}
	if elen > 0 && elen < (1<<20) {
		if off+int(elen) > len(b) {
			return it, off, fmt.Errorf("extra 越界")
		}
		it.Extra = append([]byte(nil), b[off:off+int(elen)]...)
		off += int(elen)
	}
	return it, off, nil
}

type InventoryContent struct {
	WindowID           int32
	Slots              []SlotItem
	ContainerID        byte
	HasDynamicID       bool
	DynamicContainerID uint32
	StorageItem        SlotItem
}

func ParseInventoryContent(pkt []byte) (*InventoryContent, error) {
	_, n := ReadVarint(pkt, 0)
	off := int(n)
	ic := &InventoryContent{}
	var err error
	var wid uint32
	if wid, off, err = rdVarUInt(pkt, off); err != nil {
		return nil, fmt.Errorf("windowID: %w", err)
	}
	ic.WindowID = int32(wid)
	cnt, off, err := rdVarUInt(pkt, off)
	if err != nil {
		return nil, fmt.Errorf("count: %w", err)
	}
	if cnt > 512 {
		cnt = 512
	}
	ic.Slots = make([]SlotItem, 0, cnt)
	for i := uint32(0); i < cnt; i++ {
		var it SlotItem
		if it, off, err = rdItem(pkt, off); err != nil {
			return ic, nil
		}
		ic.Slots = append(ic.Slots, it)
	}
	if off >= len(pkt) {
		return ic, nil
	}
	ic.ContainerID = pkt[off]
	off++
	if off < len(pkt) {
		hd := pkt[off]
		off++
		if hd != 0 && off+4 <= len(pkt) {
			ic.HasDynamicID = true
			ic.DynamicContainerID = uint32(pkt[off]) | uint32(pkt[off+1])<<8 |
				uint32(pkt[off+2])<<16 | uint32(pkt[off+3])<<24
			off += 4
		}
	}
	if off < len(pkt) {
		if it, _, e := rdItem(pkt, off); e == nil {
			ic.StorageItem = it
		}
	}
	return ic, nil
}

func (ic *InventoryContent) NonAirCount() int {
	n := 0
	for _, s := range ic.Slots {
		if !s.IsAir() {
			n++
		}
	}
	return n
}

type SubChunkLayer struct {
	PaletteType byte
	Bits        int
	Words       int
	SingleHash  uint32
	IsSingle    bool
}

type SubChunkPalette struct {
	Version     byte
	LayerCount  byte
	YIndex      int8
	HasYIndex   bool
	Layers      []SubChunkLayer
	Hashes      []uint32
	StoragesEnd int
}

func ParseSubChunkPalette(payload []byte) (*SubChunkPalette, error) {
	if len(payload) < 3 {
		return nil, fmt.Errorf("子区块载荷太短：%d", len(payload))
	}
	sp := &SubChunkPalette{}
	off := 0
	sp.Version = payload[off]
	off++
	sp.LayerCount = payload[off]
	off++
	if sp.Version >= 9 {
		if off >= len(payload) {
			return nil, fmt.Errorf("缺 yIndex")
		}
		sp.YIndex = int8(payload[off])
		sp.HasYIndex = true
		off++
	}

	for i := 0; i < int(sp.LayerCount); i++ {
		if off >= len(payload) {
			sp.StoragesEnd = off
			return sp, nil
		}
		pt := payload[off]
		off++
		bits := int(pt >> 1)
		lay := SubChunkLayer{PaletteType: pt, Bits: bits}

		if bits == 0 {
			if off >= len(payload) {
				sp.StoragesEnd = off
				return sp, nil
			}
			h, n, err := rdZigzagVarint(payload, off)
			if err != nil {
				sp.StoragesEnd = off
				return sp, nil
			}
			off = n
			lay.IsSingle = true
			lay.SingleHash = uint32(h)
			sp.Hashes = append(sp.Hashes, uint32(h))
			sp.Layers = append(sp.Layers, lay)
			continue
		}

		perWord := 32 / bits
		words := (4096 + perWord - 1) / perWord
		lay.Words = words
		if off+words*4 > len(payload) {
			sp.StoragesEnd = off
			return sp, nil
		}
		off += words * 4

		psz64, n, err := rdZigzagVarint(payload, off)
		if err != nil {
			sp.StoragesEnd = off
			return sp, nil
		}
		off = n
		if psz64 > 100000 {
			return sp, fmt.Errorf("调色板过大：%d", psz64)
		}
		for j := uint64(0); j < uint64(psz64); j++ {
			h, n2, err := rdZigzagVarint(payload, off)
			if err != nil {
				sp.StoragesEnd = off
				return sp, nil
			}
			off = n2
			sp.Hashes = append(sp.Hashes, uint32(h))
		}
		sp.Layers = append(sp.Layers, lay)
	}
	sp.StoragesEnd = off
	return sp, nil
}

func rdZigzagVarint(b []byte, off int) (uint64, int, error) {
	v, n, err := rdVarUInt64(b, off)
	if err != nil {
		return 0, off, err
	}
	return uint64(int64(v>>1) ^ -int64(v&1)), n, nil
}
