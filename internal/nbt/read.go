package nbt

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
)

type listVal struct {
	items []Tag
	elem  byte
}

func (r *reader) payload(tag byte) (any, error) {
	switch tag {
	case TagByte:
		v, err := r.u8()
		return int8(v), err
	case TagShort:
		return r.i16()
	case TagInt:
		return r.i32()
	case TagLong:
		return r.i64()
	case TagFloat:
		return r.f32()
	case TagDouble:
		return r.f64()
	case TagByteArray:
		n, err := r.i32()
		if err != nil {
			return nil, err
		}

		if n < 0 || int(n) > len(r.b) {
			return nil, errf("NBT 字节数组长度非法：%d", n)
		}
		if err := r.need(int(n)); err != nil {
			return nil, err
		}
		v := make([]byte, n)
		copy(v, r.b[r.p:r.p+int(n)])
		r.p += int(n)
		return v, nil
	case TagString:
		return r.str()
	case TagList:
		et, err := r.u8()
		if err != nil {
			return nil, err
		}
		n, err := r.i32()
		if err != nil {
			return nil, err
		}
		if n < 0 || int(n) > MaxListLen {
			return nil, errf("NBT 列表长度非法：%d", n)
		}
		out := make([]Tag, 0, n)
		for i := int32(0); i < n; i++ {
			v, err := r.payload(et)
			if err != nil {
				return nil, err
			}

			if lv, ok := v.(listVal); ok {
				out = append(out, Tag{Type: TagList, Val: lv.items, Elem: lv.elem})
				continue
			}
			out = append(out, Tag{Type: et, Val: v})
		}

		return listVal{items: out, elem: et}, nil
	case TagCompound:
		c, err := r.compound()
		if err != nil {
			return nil, err
		}
		return c, nil
	case TagIntArray:
		n, err := r.i32()
		if err != nil {
			return nil, err
		}
		if n < 0 || int(n) > MaxListLen {
			return nil, errf("NBT 整型数组长度非法：%d", n)
		}
		out := make([]int32, 0, n)
		for i := int32(0); i < n; i++ {
			v, err := r.i32()
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	case TagLongArray:
		n, err := r.i32()
		if err != nil {
			return nil, err
		}
		if n < 0 || int(n) > MaxListLen {
			return nil, errf("NBT 长整型数组长度非法：%d", n)
		}
		out := make([]int64, 0, n)
		for i := int32(0); i < n; i++ {
			v, err := r.i64()
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}
	return nil, errf("未知 NBT tag：%d", tag)
}

func (r *reader) compound() (Compound, error) {
	out := make(Compound)
	for {
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
		v, err := r.payload(tag)
		if err != nil {
			return nil, err
		}

		if lv, ok := v.(listVal); ok {
			out[name] = Tag{Type: TagList, Val: lv.items, Elem: lv.elem}
			continue
		}
		out[name] = Tag{Type: tag, Val: v}
	}
	return out, nil
}

func ParseNamed(data []byte, littleEndian bool) (string, Compound, error) {
	r := &reader{b: data, le: littleEndian}
	t, err := r.u8()
	if err != nil {
		return "", nil, err
	}
	if t == TagEnd {
		return "", make(Compound), nil
	}
	if t != TagCompound {
		return "", nil, errf("NBT 根不是 Compound，而是 tag=%d(%s)", t, TagNames[t])
	}
	name, err := r.str()
	if err != nil {
		return "", nil, err
	}
	c, err := r.compound()
	return name, c, err
}

func Parse(data []byte, littleEndian bool) (Compound, error) {
	_, c, err := ParseNamed(data, littleEndian)
	return c, err
}

func ParseAt(data []byte, off int, littleEndian bool) (Compound, int, error) {
	if off < 0 || off > len(data) {
		return nil, off, errf("NBT 偏移越界：%d（共 %d）", off, len(data))
	}
	r := &reader{b: data[off:], le: littleEndian}
	t, err := r.u8()
	if err != nil {
		return nil, off, err
	}
	if t != TagCompound {
		return nil, off, errf("NBT 根不是 Compound：tag=%d", t)
	}
	if _, err := r.str(); err != nil {
		return nil, off, err
	}
	c, err := r.compound()
	if err != nil {
		return nil, off, err
	}
	return c, off + r.p, nil
}

func Decompress(data []byte) ([]byte, error) {
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("gzip 头解析失败: %w", err)
		}
		defer zr.Close()
		out, err := io.ReadAll(zr)
		if err != nil {
			return nil, fmt.Errorf("gzip 解压失败: %w", err)
		}
		return out, nil
	}
	if len(data) >= 1 && data[0] == 0x78 {
		zr, err := zlib.NewReader(bytes.NewReader(data))
		if err == nil {
			defer zr.Close()
			if out, err := io.ReadAll(zr); err == nil {
				return out, nil
			}
		}

	}
	return data, nil
}

func LoadFile(data []byte, littleEndian bool) (Compound, error) {
	raw, err := Decompress(data)
	if err != nil {
		return nil, err
	}
	return Parse(raw, littleEndian)
}

func (c Compound) GetInt(key string, def int64) int64 {
	t, ok := c[key]
	if !ok {
		return def
	}
	switch v := t.Val.(type) {
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	}
	return def
}

func (c Compound) GetStr(key string, def string) string {
	if t, ok := c[key]; ok {
		if s, ok := t.Val.(string); ok {
			return s
		}
	}
	return def
}

func (c Compound) GetList(key string) []Tag {
	if t, ok := c[key]; ok && t.Type == TagList {
		if l, ok := t.Val.([]Tag); ok {
			return l
		}
	}
	return nil
}

func (c Compound) GetCompound(key string) Compound {
	if t, ok := c[key]; ok && t.Type == TagCompound {
		if sub, ok := t.Val.(Compound); ok {
			return sub
		}
	}
	return nil
}

func (c Compound) GetBytes(key string) []byte {
	if t, ok := c[key]; ok && t.Type == TagByteArray {
		if b, ok := t.Val.([]byte); ok {
			return b
		}
	}
	return nil
}

func (c Compound) GetLongs(key string) []int64 {
	if t, ok := c[key]; ok && t.Type == TagLongArray {
		if a, ok := t.Val.([]int64); ok {
			return a
		}
	}
	return nil
}

func (c Compound) GetInts(key string) []int32 {
	if t, ok := c[key]; ok && t.Type == TagIntArray {
		if a, ok := t.Val.([]int32); ok {
			return a
		}
	}
	return nil
}
