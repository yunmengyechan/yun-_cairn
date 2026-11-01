package nbt

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf8"
)

const (
	TagEnd       = 0
	TagByte      = 1
	TagShort     = 2
	TagInt       = 3
	TagLong      = 4
	TagFloat     = 5
	TagDouble    = 6
	TagByteArray = 7
	TagString    = 8
	TagList      = 9
	TagCompound  = 10
	TagIntArray  = 11
	TagLongArray = 12
)

var TagNames = map[byte]string{
	0: "END", 1: "BYTE", 2: "SHORT", 3: "INT", 4: "LONG", 5: "FLOAT",
	6: "DOUBLE", 7: "BYTE_ARRAY", 8: "STRING", 9: "LIST", 10: "COMPOUND",
	11: "INT_ARRAY", 12: "LONG_ARRAY",
}

const MaxListLen = 1 << 24

type Tag struct {
	Type byte
	Val  any
	Elem byte
}

type Err struct{ msg string }

func (e *Err) Error() string { return e.msg }

func errf(f string, a ...any) error { return &Err{fmt.Sprintf(f, a...)} }

func Byte(v int8) Tag         { return Tag{Type: TagByte, Val: v} }
func Short(v int16) Tag       { return Tag{Type: TagShort, Val: v} }
func Int(v int32) Tag         { return Tag{Type: TagInt, Val: v} }
func Long(v int64) Tag        { return Tag{Type: TagLong, Val: v} }
func Float(v float32) Tag     { return Tag{Type: TagFloat, Val: v} }
func Double(v float64) Tag    { return Tag{Type: TagDouble, Val: v} }
func String(v string) Tag     { return Tag{Type: TagString, Val: v} }
func ByteArray(v []byte) Tag  { return Tag{Type: TagByteArray, Val: v} }
func IntArray(v []int32) Tag  { return Tag{Type: TagIntArray, Val: v} }
func LongArray(v []int64) Tag { return Tag{Type: TagLongArray, Val: v} }

func List(items []Tag) Tag {
	et := byte(TagEnd)
	if len(items) > 0 {
		et = items[0].Type
	}
	return Tag{Type: TagList, Val: items, Elem: et}
}

func ListOf(elem byte, items []Tag) Tag {
	return Tag{Type: TagList, Val: items, Elem: elem}
}

type Compound map[string]Tag

func NewCompound() Compound { return make(Compound) }

type reader struct {
	b  []byte
	p  int
	le bool
}

func (r *reader) need(n int) error {
	if r.p+n > len(r.b) {
		return errf("NBT 数据截断：偏移 %d 需要 %d 字节（共 %d）", r.p, n, len(r.b))
	}
	return nil
}

func (r *reader) u8() (byte, error) {
	if err := r.need(1); err != nil {
		return 0, err
	}
	v := r.b[r.p]
	r.p++
	return v, nil
}

func (r *reader) ord() binary.ByteOrder {
	if r.le {
		return binary.LittleEndian
	}
	return binary.BigEndian
}

func (r *reader) i16() (int16, error) {
	if err := r.need(2); err != nil {
		return 0, err
	}
	v := int16(r.ord().Uint16(r.b[r.p:]))
	r.p += 2
	return v, nil
}

func (r *reader) u16() (uint16, error) {
	if err := r.need(2); err != nil {
		return 0, err
	}
	v := r.ord().Uint16(r.b[r.p:])
	r.p += 2
	return v, nil
}

func (r *reader) i32() (int32, error) {
	if err := r.need(4); err != nil {
		return 0, err
	}
	v := int32(r.ord().Uint32(r.b[r.p:]))
	r.p += 4
	return v, nil
}

func (r *reader) i64() (int64, error) {
	if err := r.need(8); err != nil {
		return 0, err
	}
	v := int64(r.ord().Uint64(r.b[r.p:]))
	r.p += 8
	return v, nil
}

func (r *reader) f32() (float32, error) {
	if err := r.need(4); err != nil {
		return 0, err
	}
	v := math.Float32frombits(r.ord().Uint32(r.b[r.p:]))
	r.p += 4
	return v, nil
}

func (r *reader) f64() (float64, error) {
	if err := r.need(8); err != nil {
		return 0, err
	}
	v := math.Float64frombits(r.ord().Uint64(r.b[r.p:]))
	r.p += 8
	return v, nil
}

func (r *reader) str() (string, error) {
	n, err := r.u16()
	if err != nil {
		return "", err
	}
	if err := r.need(int(n)); err != nil {
		return "", err
	}

	s := string(r.b[r.p : r.p+int(n)])
	r.p += int(n)
	if !utf8.ValidString(s) {
		s = string(bytes.ToValidUTF8([]byte(s), []byte("\uFFFD")))
	}
	return s, nil
}
