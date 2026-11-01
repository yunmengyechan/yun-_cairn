package nbt

import (
	"encoding/binary"
	"math"
)

type netReader struct {
	b   []byte
	off int
}

func (r *netReader) avail(k int) bool { return r.off+k <= len(r.b) }

func (r *netReader) u8() (byte, error) {
	if !r.avail(1) {
		return 0, errf("网络 NBT：读 u8 越界")
	}
	v := r.b[r.off]
	r.off++
	return v, nil
}

func (r *netReader) varint() (uint64, error) {
	var v uint64
	var sh uint
	for {
		if !r.avail(1) {
			return 0, errf("网络 NBT：varint 截断")
		}
		c := r.b[r.off]
		r.off++
		v |= uint64(c&0x7F) << sh
		if c&0x80 == 0 {
			return v, nil
		}
		sh += 7
		if sh > 63 {
			return 0, errf("网络 NBT：varint 过长")
		}
	}
}

func (r *netReader) zvarint() (int64, error) {
	v, err := r.varint()
	if err != nil {
		return 0, err
	}
	return int64(v>>1) ^ -int64(v&1), nil
}

func (r *netReader) u32le() (uint32, error) {
	if !r.avail(4) {
		return 0, errf("网络 NBT：读 u32 越界")
	}
	v := binary.LittleEndian.Uint32(r.b[r.off : r.off+4])
	r.off += 4
	return v, nil
}

func (r *netReader) bytes() ([]byte, error) {
	n, err := r.varint()
	if err != nil {
		return nil, err
	}
	if n > uint64(len(r.b)) {
		return nil, errf("网络 NBT：字节块长度非法 %d", n)
	}
	if !r.avail(int(n)) {
		return nil, errf("网络 NBT：字节块越界")
	}
	v := r.b[r.off : r.off+int(n)]
	r.off += int(n)
	return v, nil
}

func (r *netReader) str() (string, error) {
	b, err := r.bytes()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (r *netReader) value(tag byte) (any, error) {
	switch tag {
	case TagByte:
		v, err := r.u8()
		if err != nil {
			return nil, err
		}
		return int8(v), nil

	case TagShort:

		if !r.avail(2) {
			return nil, errf("网络 NBT：short 越界")
		}
		v := int16(binary.LittleEndian.Uint16(r.b[r.off : r.off+2]))
		r.off += 2
		return v, nil

	case TagInt:

		v, err := r.zvarint()
		return int32(v), err

	case TagLong:

		return r.zvarint()

	case TagFloat:
		if !r.avail(4) {
			return nil, errf("网络 NBT：float 越界")
		}
		v := math.Float32frombits(binary.LittleEndian.Uint32(r.b[r.off : r.off+4]))
		r.off += 4
		return v, nil

	case TagDouble:
		if !r.avail(8) {
			return nil, errf("网络 NBT：double 越界")
		}
		v := math.Float64frombits(binary.LittleEndian.Uint64(r.b[r.off : r.off+8]))
		r.off += 8
		return v, nil

	case TagByteArray:
		b, err := r.bytes()
		if err != nil {
			return nil, err
		}
		out := make([]byte, len(b))
		copy(out, b)
		return out, nil

	case TagString:
		return r.str()

	case TagList:
		et, err := r.u8()
		if err != nil {
			return nil, err
		}
		n, err := r.varint()
		if err != nil {
			return nil, err
		}
		if n > MaxListLen {
			return nil, errf("网络 NBT：列表过大 %d", n)
		}
		out := make([]Tag, 0, n)
		for i := uint64(0); i < n; i++ {
			v, err := r.value(et)
			if err != nil {
				return nil, err
			}
			out = append(out, Tag{Type: et, Val: v})
		}
		return out, nil

	case TagCompound:
		return r.compound()

	case TagIntArray:
		n, err := r.varint()
		if err != nil {
			return nil, err
		}
		if n > MaxListLen {
			return nil, errf("网络 NBT：整型数组过大 %d", n)
		}
		out := make([]int32, 0, n)
		for i := uint64(0); i < n; i++ {
			v, err := r.zvarint()
			if err != nil {
				return nil, err
			}
			out = append(out, int32(v))
		}
		return out, nil

	case TagLongArray:
		n, err := r.varint()
		if err != nil {
			return nil, err
		}
		if n > MaxListLen {
			return nil, errf("网络 NBT：长整型数组过大 %d", n)
		}
		out := make([]int64, 0, n)
		for i := uint64(0); i < n; i++ {
			v, err := r.zvarint()
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}
	return nil, errf("网络 NBT：未知 tag %d", tag)
}

func (r *netReader) compound() (Compound, error) {
	out := make(Compound)
	for {
		if !r.avail(1) {
			break
		}
		tag, err := r.u8()
		if err != nil {
			return nil, err
		}
		if tag == TagEnd {
			break
		}
		name, err := r.str()
		if err != nil {
			return nil, err
		}
		v, err := r.value(tag)
		if err != nil {
			return nil, err
		}
		out[name] = Tag{Type: tag, Val: v}
	}
	return out, nil
}

func ParseNetwork(b []byte) (string, Compound, int, error) {
	r := &netReader{b: b}
	t, err := r.u8()
	if err != nil {
		return "", nil, 0, err
	}
	if t != TagCompound {
		return "", nil, 0, errf("网络 NBT 根不是 Compound：tag=%d", t)
	}
	name, err := r.str()
	if err != nil {
		return "", nil, 0, err
	}
	c, err := r.compound()
	if err != nil {
		return "", nil, r.off, err
	}
	return name, c, r.off, nil
}

func ParseNetworkAt(b []byte, off int) (Compound, int, error) {
	if off < 0 || off > len(b) {
		return nil, off, errf("网络 NBT 偏移越界：%d", off)
	}
	r := &netReader{b: b, off: off}
	t, err := r.u8()
	if err != nil {
		return nil, off, err
	}
	if t != TagCompound {
		return nil, off, errf("网络 NBT 根不是 Compound：tag=%d", t)
	}
	if _, err := r.str(); err != nil {
		return nil, off, err
	}
	c, err := r.compound()
	return c, r.off, err
}
