package mc

import (
	"fmt"
	"math"
)

func MPPack(v any) []byte {
	return mpAppend(nil, v)
}

func mpAppend(b []byte, v any) []byte {
	switch t := v.(type) {
	case nil:
		return append(b, 0xC0)
	case bool:
		if t {
			return append(b, 0xC3)
		}
		return append(b, 0xC2)
	case int:
		return mpInt(b, int64(t))
	case int64:
		return mpInt(b, t)
	case uint64:
		return mpUint(b, t)
	case string:
		return mpStr(b, t)
	case []byte:
		return mpBin(b, t)
	case []any:
		return mpArray(b, t)
	case map[string]any:
		return mpMap(b, t)
	case float64:
		bits := math.Float64bits(t)
		b = append(b, 0xCB)
		for i := 7; i >= 0; i-- {
			b = append(b, byte(bits>>(uint(i)*8)))
		}
		return b
	default:
		return mpStr(b, fmt.Sprintf("%v", t))
	}
}

func mpInt(b []byte, v int64) []byte {
	if v >= 0 {
		return mpUint(b, uint64(v))
	}
	switch {
	case v >= -32:
		return append(b, byte(0xE0|(v+32)))
	case v >= -128:
		return append(b, 0xD0, byte(v))
	case v >= -32768:
		return append(b, 0xD1, byte(v>>8), byte(v))
	case v >= -2147483648:
		return append(b, 0xD2, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	default:
		b = append(b, 0xD3)
		for i := 7; i >= 0; i-- {
			b = append(b, byte(uint64(v)>>(uint(i)*8)))
		}
		return b
	}
}

func mpUint(b []byte, v uint64) []byte {
	switch {
	case v <= 127:
		return append(b, byte(v))
	case v <= 255:
		return append(b, 0xCC, byte(v))
	case v <= 65535:
		return append(b, 0xCD, byte(v>>8), byte(v))
	case v <= 4294967295:
		return append(b, 0xCE, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	default:
		b = append(b, 0xCF)
		for i := 7; i >= 0; i-- {
			b = append(b, byte(v>>(uint(i)*8)))
		}
		return b
	}
}

func mpStr(b []byte, s string) []byte {
	n := len(s)
	switch {
	case n <= 31:
		b = append(b, byte(0xA0|n))
	case n <= 255:
		b = append(b, 0xD9, byte(n))
	case n <= 65535:
		b = append(b, 0xDA, byte(n>>8), byte(n))
	default:
		b = append(b, 0xDB, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}
	return append(b, []byte(s)...)
}

func mpBin(b []byte, d []byte) []byte {
	n := len(d)
	switch {
	case n <= 255:
		b = append(b, 0xC4, byte(n))
	case n <= 65535:
		b = append(b, 0xC5, byte(n>>8), byte(n))
	default:
		b = append(b, 0xC6, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}
	return append(b, d...)
}

func mpArray(b []byte, a []any) []byte {
	n := len(a)
	switch {
	case n <= 15:
		b = append(b, byte(0x90|n))
	case n <= 65535:
		b = append(b, 0xDC, byte(n>>8), byte(n))
	default:
		b = append(b, 0xDD, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}
	for _, e := range a {
		b = mpAppend(b, e)
	}
	return b
}

func mpMap(b []byte, m map[string]any) []byte {
	n := len(m)
	if n <= 15 {
		b = append(b, byte(0x80|n))
	} else {
		b = append(b, 0xDE, byte(n>>8), byte(n))
	}
	for k, v := range m {
		b = mpStr(b, k)
		b = mpAppend(b, v)
	}
	return b
}

func MPUnpack(d []byte) (any, int, error) {
	if len(d) == 0 {
		return nil, 0, fmt.Errorf("msgpack 空")
	}
	c := d[0]
	switch {
	case c <= 0x7F:
		return int64(c), 1, nil
	case c >= 0xE0:
		return int64(int8(c)), 1, nil
	case c >= 0x80 && c <= 0x8F:
		return mpUnpackMap(d, int(c&0x0F))
	case c >= 0x90 && c <= 0x9F:
		return mpUnpackArray(d, int(c&0x0F))
	case c >= 0xA0 && c <= 0xBF:
		return mpUnpackStrN(d, int(c&0x1F), 1)
	}
	switch c {
	case 0xC0:
		return nil, 1, nil
	case 0xC2:
		return false, 1, nil
	case 0xC3:
		return true, 1, nil
	case 0xC4, 0xC5, 0xC6:
		return mpUnpackBin(d, c)
	case 0xCA:
		return nil, 5, nil
	case 0xCB:
		if len(d) < 9 {
			return nil, 0, fmt.Errorf("float64 不足")
		}
		var u uint64
		for i := 0; i < 8; i++ {
			u = u<<8 | uint64(d[1+i])
		}
		return math.Float64frombits(u), 9, nil
	case 0xCC:
		return int64(d[1]), 2, nil
	case 0xCD:
		return int64(uint16(d[1])<<8 | uint16(d[2])), 3, nil
	case 0xCE:
		return int64(uint32(d[1])<<24 | uint32(d[2])<<16 | uint32(d[3])<<8 | uint32(d[4])), 5, nil
	case 0xCF:
		var u uint64
		for i := 0; i < 8; i++ {
			u = u<<8 | uint64(d[1+i])
		}
		return int64(u), 9, nil
	case 0xD0:
		return int64(int8(d[1])), 2, nil
	case 0xD1:
		return int64(int16(uint16(d[1])<<8 | uint16(d[2]))), 3, nil
	case 0xD2:
		return int64(int32(uint32(d[1])<<24 | uint32(d[2])<<16 | uint32(d[3])<<8 | uint32(d[4]))), 5, nil
	case 0xD3:
		var u uint64
		for i := 0; i < 8; i++ {
			u = u<<8 | uint64(d[1+i])
		}
		return int64(u), 9, nil
	case 0xD9:
		return mpUnpackStrN(d, int(d[1]), 2)
	case 0xDA:
		return mpUnpackStrN(d, int(uint16(d[1])<<8|uint16(d[2])), 3)
	case 0xDC:
		return mpUnpackArray(d, int(uint16(d[1])<<8|uint16(d[2])))
	case 0xDE:
		return mpUnpackMap(d, int(uint16(d[1])<<8|uint16(d[2])))
	}
	return nil, 0, fmt.Errorf("未知 msgpack 前缀 0x%02x", c)
}

func mpUnpackStrN(d []byte, n, off int) (any, int, error) {
	if off+n > len(d) {
		return nil, 0, fmt.Errorf("str 越界")
	}
	return string(d[off : off+n]), off + n, nil
}

func mpUnpackBin(d []byte, c byte) (any, int, error) {
	var n, off int
	switch c {
	case 0xC4:
		n, off = int(d[1]), 2
	case 0xC5:
		n, off = int(uint16(d[1])<<8|uint16(d[2])), 3
	default:
		n, off = int(uint32(d[1])<<24|uint32(d[2])<<16|uint32(d[3])<<8|uint32(d[4])), 5
	}
	if off+n > len(d) {
		return nil, 0, fmt.Errorf("bin 越界")
	}
	return append([]byte(nil), d[off:off+n]...), off + n, nil
}

func mpUnpackArray(d []byte, n int) (any, int, error) {
	off := 1
	if d[0] == 0xDC {
		off = 3
	}
	out := make([]any, 0, n)
	for i := 0; i < n; i++ {
		v, used, err := MPUnpack(d[off:])
		if err != nil {
			return nil, 0, err
		}
		out = append(out, v)
		off += used
	}
	return out, off, nil
}

func mpUnpackMap(d []byte, n int) (any, int, error) {
	off := 1
	if d[0] == 0xDE {
		off = 3
	}
	out := map[string]any{}
	for i := 0; i < n; i++ {
		k, used, err := MPUnpack(d[off:])
		if err != nil {
			return nil, 0, err
		}
		off += used
		v, used2, err := MPUnpack(d[off:])
		if err != nil {
			return nil, 0, err
		}
		off += used2
		out[fmt.Sprintf("%v", k)] = v
	}
	return out, off, nil
}
