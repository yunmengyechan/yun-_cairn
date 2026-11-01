package leveldb

import (
	"encoding/binary"
	"fmt"

	"cairn/internal/snappy"
)

const (
	MagicLegacy = 0xdb4775248b80fb57
	MagicNew    = 0x88e241b785f4cff7
)

const (
	TagSubChunk    = 0x2B
	TagSubChunkV2  = 0x2E
	TagBlockEntity = 0x2D
	TagEntity      = 0x2C
)

type BlockHandle struct {
	Offset int
	Size   int
}

type Table struct {
	data        []byte
	indexHandle BlockHandle
	MetaHandle  BlockHandle
	FooterSize  int
}

func readVarint(b []byte, off int) (uint64, int, error) {
	var v uint64
	var sh uint
	for i := off; i < len(b); i++ {
		c := b[i]
		v |= uint64(c&0x7F) << sh
		if c&0x80 == 0 {
			return v, i + 1, nil
		}
		sh += 7
		if sh > 63 {
			return 0, off, fmt.Errorf("varint 过长")
		}
	}
	return 0, off, fmt.Errorf("varint 截断")
}

func readBlockHandle(b []byte, off int) (BlockHandle, int, error) {
	var h BlockHandle
	o, n, err := readVarint(b, off)
	if err != nil {
		return h, off, err
	}
	sz, n2, err := readVarint(b, n)
	if err != nil {
		return h, off, err
	}

	if o > uint64(1)<<31 || sz > uint64(1)<<31 {
		return h, off, fmt.Errorf("块句柄数值异常：offset=%d size=%d", o, sz)
	}
	h.Offset = int(o)
	h.Size = int(sz)
	return h, n2, nil
}

func Open(data []byte) (*Table, error) {
	n := len(data)

	if n < 48 {
		return nil, fmt.Errorf("LevelDB 表文件过短：%d 字节", n)
	}
	magic := binary.LittleEndian.Uint64(data[n-8 : n])
	var footerSize int
	switch magic {
	case MagicLegacy:
		footerSize = 48
	case MagicNew:
		footerSize = 53
	default:
		return nil, fmt.Errorf("LevelDB footer magic 不符：0x%016X", magic)
	}

	footer := data[n-footerSize:]
	meta, off, err := readBlockHandle(footer, 0)
	if err != nil {
		return nil, fmt.Errorf("meta 句柄: %w", err)
	}
	idx, _, err := readBlockHandle(footer, off)
	if err != nil {
		return nil, fmt.Errorf("index 句柄: %w", err)
	}
	return &Table{data: data, indexHandle: idx, MetaHandle: meta, FooterSize: footerSize}, nil
}

func (t *Table) readRawBlock(h BlockHandle) ([]byte, error) {
	if h.Offset < 0 || h.Size < 0 || h.Offset+h.Size+5 > len(t.data) {
		return nil, fmt.Errorf("块越界：offset=%d size=%d（共 %d）", h.Offset, h.Size, len(t.data))
	}
	raw := t.data[h.Offset : h.Offset+h.Size]
	ctype := t.data[h.Offset+h.Size]

	switch ctype {
	case 0:
		return raw, nil
	case 1:
		out, err := snappy.Decompress(raw)
		if err != nil {
			return nil, fmt.Errorf("snappy 解压块失败: %w", err)
		}
		return out, nil
	case 2:
		return nil, fmt.Errorf("zlib 压缩块暂不支持（真实数据里极少见）")
	}
	return nil, fmt.Errorf("未知的块压缩类型：%d", ctype)
}

func iterBlockEntries(block []byte, fn func(k, v []byte) bool) {
	n := len(block)
	if n < 4 {
		return
	}
	numRestarts := binary.LittleEndian.Uint32(block[n-4:])

	if numRestarts == 0 || int(numRestarts) > n/4 {
		return
	}
	entriesEnd := n - 4 - int(numRestarts)*4
	if entriesEnd < 0 {
		return
	}

	pos := 0
	var prevKey []byte
	for pos < entriesEnd {
		shared64, p1, err := readVarint(block, pos)
		if err != nil {
			return
		}
		nonShared64, p2, err := readVarint(block, p1)
		if err != nil {
			return
		}
		vlen64, p3, err := readVarint(block, p2)
		if err != nil {
			return
		}
		pos = p3
		shared, nonShared, vlen := int(shared64), int(nonShared64), int(vlen64)
		if pos+nonShared+vlen > entriesEnd {
			return
		}

		if shared > len(prevKey) {
			return
		}
		key := make([]byte, 0, shared+nonShared)
		key = append(key, prevKey[:shared]...)
		key = append(key, block[pos:pos+nonShared]...)
		pos += nonShared
		val := block[pos : pos+vlen]
		pos += vlen
		prevKey = key
		if !fn(key, val) {
			return
		}
	}
}

func (t *Table) Iterate(fn func(k, v []byte) bool) error {
	indexRaw, err := t.readRawBlock(t.indexHandle)
	if err != nil {
		return fmt.Errorf("读 index block: %w", err)
	}
	var failed error
	iterBlockEntries(indexRaw, func(_sep, value []byte) bool {
		h, _, e := readBlockHandle(value, 0)
		if e != nil {
			return true
		}
		raw, e := t.readRawBlock(h)
		if e != nil {
			failed = e
			return true
		}
		cont := true
		iterBlockEntries(raw, func(k, v []byte) bool {
			cont = fn(k, v)
			return cont
		})
		return cont
	})
	return failed
}

type ChunkKey struct {
	X    int32
	Z    int32
	Tag  byte
	SubY *int8
}

func ParseChunkKey(key []byte) (*ChunkKey, bool) {
	if len(key) < 9 {
		return nil, false
	}
	ck := &ChunkKey{
		X:   int32(binary.LittleEndian.Uint32(key[0:4])),
		Z:   int32(binary.LittleEndian.Uint32(key[4:8])),
		Tag: key[8],
	}
	if len(key) > 9 {
		y := int8(key[9])
		ck.SubY = &y
	}
	return ck, true
}

func TagName(tag byte) string {
	switch tag {
	case TagSubChunk:
		return "SubChunk"
	case TagSubChunkV2:
		return "SubChunkV2"
	case TagBlockEntity:
		return "BlockEntity"
	case TagEntity:
		return "Entity"
	}
	return fmt.Sprintf("Unknown(0x%02x)", tag)
}
