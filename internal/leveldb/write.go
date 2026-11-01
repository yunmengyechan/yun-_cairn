package leveldb

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"sort"
)

var castagnoliTable = crc32.MakeTable(crc32.Castagnoli)

func maskedCRC32C(b []byte) uint32 {
	crc := crc32.Checksum(b, castagnoliTable)
	return ((crc >> 15) | (crc << 17)) + 0xa282ead8
}

const defaultBlockSize = 4096

const restartInterval = 16

type WriteOptions struct {
	BlockRestartInterval int

	BlockSize int
}

func blockEntrySize(it kv) int {

	return 15 + len(it.k) + len(it.v)
}

type kv struct {
	k, v []byte
}

func WriteTable(pairs map[string][]byte, opt *WriteOptions) ([]byte, error) {
	if len(pairs) == 0 {
		return nil, fmt.Errorf("没有键值对")
	}
	rint := restartInterval
	if opt != nil && opt.BlockRestartInterval > 0 {
		rint = opt.BlockRestartInterval
	}

	items := make([]kv, 0, len(pairs))
	for k, v := range pairs {
		items = append(items, kv{[]byte(k), v})
	}
	sort.Slice(items, func(i, j int) bool {
		return string(items[i].k) < string(items[j].k)
	})

	blockSize := defaultBlockSize
	if opt != nil && opt.BlockSize > 0 {
		blockSize = opt.BlockSize
	}

	var out []byte
	var indexEntries []kv
	start := 0
	for start < len(items) {

		end := start + 1
		estimate := blockEntrySize(items[start])
		for end < len(items) && estimate < blockSize {
			estimate += blockEntrySize(items[end])
			end++
		}
		chunk := items[start:end]
		blk := buildBlock(chunk, rint)
		off := len(out)
		out = appendBlock(out, blk, 0)

		last := chunk[len(chunk)-1].k
		sep := append([]byte(nil), last...)
		if end >= len(items) {
			sep = append(sep, 0xff)
		}
		indexEntries = append(indexEntries, kv{sep, encodeHandle(off, len(blk))})
		start = end
	}

	indexBlock := buildBlock(indexEntries, rint)
	indexOff := len(out)
	out = appendBlock(out, indexBlock, 0)

	footer := make([]byte, 0, 48)
	footer = append(footer, encodeHandle(0, 0)...)
	footer = append(footer, encodeHandle(indexOff, len(indexBlock))...)

	for len(footer) < 40 {
		footer = append(footer, 0)
	}
	var magic [8]byte
	binary.LittleEndian.PutUint64(magic[:], MagicLegacy)
	footer = append(footer, magic[:]...)
	if len(footer) != 48 {
		return nil, fmt.Errorf("footer 长度异常：%d（应为 48）", len(footer))
	}
	out = append(out, footer...)
	return out, nil
}

func buildBlock(items []kv, rint int) []byte {
	var out []byte
	var restarts []uint32
	var prevKey []byte

	for i, it := range items {

		if i%rint == 0 {
			restarts = append(restarts, uint32(len(out)))
			prevKey = nil
		}

		shared := commonPrefixLen(prevKey, it.k)
		nonShared := len(it.k) - shared
		out = append(out, uvarint(uint64(shared))...)
		out = append(out, uvarint(uint64(nonShared))...)
		out = append(out, uvarint(uint64(len(it.v)))...)
		out = append(out, it.k[shared:]...)
		out = append(out, it.v...)
		prevKey = it.k
	}

	for _, r := range restarts {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], r)
		out = append(out, b[:]...)
	}
	var nb [4]byte
	binary.LittleEndian.PutUint32(nb[:], uint32(len(restarts)))
	out = append(out, nb[:]...)
	return out
}

func commonPrefixLen(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return i
}

func appendBlock(dst, block []byte, ctype byte) []byte {
	dst = append(dst, block...)
	dst = append(dst, ctype)
	crc := maskedCRC32C(block)
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], crc)
	dst = append(dst, b[:]...)
	return dst
}

func encodeHandle(off, size int) []byte {
	out := uvarint(uint64(off))
	return append(out, uvarint(uint64(size))...)
}

func uvarint(v uint64) []byte {
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

func crc32ChecksumIEEE(b []byte) uint32 { return crc32.ChecksumIEEE(b) }
