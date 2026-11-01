package nbt

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"math"
)

type writer struct {
	buf bytes.Buffer
	le  bool
}

func (w *writer) ord() binary.ByteOrder {
	if w.le {
		return binary.LittleEndian
	}
	return binary.BigEndian
}

func (w *writer) u8(v byte) { w.buf.WriteByte(v) }

func (w *writer) i16(v int16) {
	var b [2]byte
	w.ord().PutUint16(b[:], uint16(v))
	w.buf.Write(b[:])
}

func (w *writer) i32(v int32) {
	var b [4]byte
	w.ord().PutUint32(b[:], uint32(v))
	w.buf.Write(b[:])
}

func (w *writer) i64(v int64) {
	var b [8]byte
	w.ord().PutUint64(b[:], uint64(v))
	w.buf.Write(b[:])
}

func (w *writer) f32(v float32) {
	var b [4]byte
	w.ord().PutUint32(b[:], math.Float32bits(v))
	w.buf.Write(b[:])
}

func (w *writer) f64(v float64) {
	var b [8]byte
	w.ord().PutUint64(b[:], math.Float64bits(v))
	w.buf.Write(b[:])
}

func (w *writer) str(s string) error {
	b := []byte(s)
	if len(b) > 65535 {
		return fmt.Errorf("NBT 字符串太长：%d 字节（上限 65535）", len(b))
	}
	w.i16(int16(len(b)))
	w.buf.Write(b)
	return nil
}

func (w *writer) payload(t Tag) error {
	switch t.Type {
	case TagByte:
		w.u8(byte(asI64(t.Val)))
	case TagShort:
		w.i16(int16(asI64(t.Val)))
	case TagInt:
		w.i32(int32(asI64(t.Val)))
	case TagLong:
		w.i64(asI64(t.Val))
	case TagFloat:
		w.f32(float32(asF64(t.Val)))
	case TagDouble:
		w.f64(asF64(t.Val))

	case TagByteArray:
		b, err := asBytes(t.Val)
		if err != nil {
			return err
		}
		w.i32(int32(len(b)))
		w.buf.Write(b)

	case TagString:
		s, ok := t.Val.(string)
		if !ok {
			return fmt.Errorf("TagString 的值不是 string，而是 %T", t.Val)
		}
		return w.str(s)

	case TagList:
		items, ok := t.Val.([]Tag)
		if !ok {
			return fmt.Errorf("TagList 的值不是 []Tag，而是 %T", t.Val)
		}
		et := t.Elem
		if et == 0 && len(items) > 0 {
			et = items[0].Type
		}
		w.u8(et)
		w.i32(int32(len(items)))
		for i, it := range items {

			if err := w.payload(Tag{Type: et, Val: it.Val, Elem: it.Elem}); err != nil {
				return fmt.Errorf("列表第 %d 项: %w", i, err)
			}
		}

	case TagCompound:
		c, err := asCompound(t.Val)
		if err != nil {
			return err
		}
		return w.compoundBody(c)

	case TagIntArray:
		a, ok := t.Val.([]int32)
		if !ok {
			return fmt.Errorf("TagIntArray 的值不是 []int32，而是 %T", t.Val)
		}
		w.i32(int32(len(a)))
		for _, v := range a {
			w.i32(v)
		}

	case TagLongArray:
		a, ok := t.Val.([]int64)
		if !ok {
			return fmt.Errorf("TagLongArray 的值不是 []int64，而是 %T", t.Val)
		}
		w.i32(int32(len(a)))
		for _, v := range a {
			w.i64(v)
		}

	default:
		return fmt.Errorf("未知 NBT tag：%d", t.Type)
	}
	return nil
}

func (w *writer) compoundBody(c Compound) error {

	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sortStrings(keys)

	for _, k := range keys {
		t := c[k]
		w.u8(t.Type)
		if err := w.str(k); err != nil {
			return err
		}
		if err := w.payload(t); err != nil {
			return fmt.Errorf("字段 %q: %w", k, err)
		}
	}
	w.u8(TagEnd)
	return nil
}

func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func Dump(root Compound, littleEndian bool, rootName string) ([]byte, error) {
	w := &writer{le: littleEndian}
	w.u8(TagCompound)
	if err := w.str(rootName); err != nil {
		return nil, err
	}
	if err := w.compoundBody(root); err != nil {
		return nil, err
	}
	return w.buf.Bytes(), nil
}

func DumpGzip(root Compound, littleEndian bool, rootName string) ([]byte, error) {
	raw, err := Dump(root, littleEndian, rootName)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func asI64(v any) int64 {
	switch x := v.(type) {
	case int8:
		return int64(x)
	case int16:
		return int64(x)
	case int32:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case uint8:
		return int64(x)
	case uint16:
		return int64(x)
	case uint32:
		return int64(x)
	case uint64:
		return int64(x)
	}
	return 0
}

func asF64(v any) float64 {
	switch x := v.(type) {
	case float32:
		return float64(x)
	case float64:
		return x
	case int8:
		return float64(x)
	case int16:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

func asBytes(v any) ([]byte, error) {
	switch x := v.(type) {
	case []byte:
		return x, nil
	case []int8:
		out := make([]byte, len(x))
		for i, b := range x {
			out[i] = byte(b)
		}
		return out, nil
	}
	return nil, fmt.Errorf("TagByteArray 的值不是 []byte，而是 %T", v)
}

func asCompound(v any) (Compound, error) {
	switch x := v.(type) {
	case Compound:
		return x, nil
	case map[string]Tag:
		return Compound(x), nil
	case map[string]any:

		out := make(Compound, len(x))
		for k, val := range x {
			t, err := Infer(val)
			if err != nil {
				return nil, fmt.Errorf("字段 %q: %w", k, err)
			}
			out[k] = t
		}
		return out, nil
	}
	return nil, fmt.Errorf("TagCompound 的值不是 Compound，而是 %T", v)
}

func Infer(v any) (Tag, error) {
	switch x := v.(type) {
	case Tag:
		return x, nil
	case bool:
		if x {
			return Byte(1), nil
		}
		return Byte(0), nil
	case int8:
		return Byte(x), nil
	case int16:
		return Short(x), nil
	case int32:
		return Int(x), nil
	case int64:
		return Long(x), nil
	case int:
		return Int(int32(x)), nil
	case float32:
		return Float(x), nil
	case float64:
		return Double(x), nil
	case string:
		return String(x), nil
	case []byte:
		return ByteArray(x), nil
	case []int8:
		b := make([]byte, len(x))
		for i, e := range x {
			b[i] = byte(e)
		}
		return ByteArray(b), nil
	case []int16:
		a := make([]int32, len(x))
		for i, e := range x {
			a[i] = int32(e)
		}
		return IntArray(a), nil
	case []int32:
		return IntArray(x), nil
	case []int64:
		return LongArray(x), nil
	case []Tag:
		return List(x), nil
	case Compound:
		return Tag{Type: TagCompound, Val: x}, nil
	case map[string]Tag:
		return Tag{Type: TagCompound, Val: Compound(x)}, nil
	case map[string]any:
		c, err := asCompound(x)
		if err != nil {
			return Tag{}, err
		}
		return Tag{Type: TagCompound, Val: c}, nil
	}
	return Tag{}, fmt.Errorf("推断不出 NBT 类型：%T", v)
}

func Build(m map[string]any) (Compound, error) {
	return asCompound(m)
}
