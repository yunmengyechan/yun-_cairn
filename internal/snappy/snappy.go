package snappy

import (
	"encoding/binary"
	"fmt"
)

type Error struct{ msg string }

func (e *Error) Error() string { return e.msg }

func errf(f string, a ...any) error { return &Error{fmt.Sprintf(f, a...)} }

func readVarint(data []byte, pos int) (uint64, int, error) {
	var value uint64
	var shift uint
	n := len(data)
	for pos < n {
		b := data[pos]
		pos++
		value |= uint64(b&0x7F) << shift
		if b&0x80 == 0 {
			return value, pos, nil
		}
		shift += 7
		if shift > 35 {
			return 0, pos, errf("snappy varint 过长")
		}
	}
	return 0, pos, errf("snappy 数据截断（长度前缀）")
}

func Decompress(data []byte) ([]byte, error) {
	if len(data) < 1 {
		return nil, errf("snappy 输入为空")
	}

	expected, pos, err := readVarint(data, 0)
	if err != nil {
		return nil, err
	}
	if expected > 1<<31 {
		return nil, errf("snappy 声明的原始长度过大：%d", expected)
	}

	out := make([]byte, 0, expected)
	n := len(data)

	for pos < n {
		tag := data[pos]
		pos++
		kind := tag & 0x03

		if kind == 0 {

			length := int(tag >> 2)
			if length < 60 {
				length++
			} else {
				extra := length - 59
				if pos+extra > n {
					return nil, errf("snappy literal 长度字段越界")
				}
				var raw uint32
				for i := 0; i < extra; i++ {
					raw |= uint32(data[pos+i]) << (8 * i)
				}
				length = int(raw) + 1
				pos += extra
			}
			if pos+length > n {
				return nil, errf("snappy literal 数据越界")
			}
			out = append(out, data[pos:pos+length]...)
			pos += length
			continue
		}

		var length, offset int
		switch kind {
		case 1:
			if pos >= n {
				return nil, errf("snappy copy1 越界")
			}

			length = int((tag>>2)&0x07) + 4
			offset = int(tag>>5)<<8 | int(data[pos])
			pos++
		case 2:
			if pos+2 > n {
				return nil, errf("snappy copy2 越界")
			}
			length = int(tag>>2) + 1
			offset = int(binary.LittleEndian.Uint16(data[pos : pos+2]))
			pos += 2
		default:
			if pos+4 > n {
				return nil, errf("snappy copy4 越界")
			}
			length = int(tag>>2) + 1
			offset = int(binary.LittleEndian.Uint32(data[pos : pos+4]))
			pos += 4
		}

		if offset == 0 || offset > len(out) {
			return nil, errf("snappy 复制偏移非法：%d", offset)
		}
		start := len(out) - offset

		if length <= offset {

			out = append(out, out[start:start+length]...)
		} else {

			for i := 0; i < length; i++ {
				out = append(out, out[start+i])
			}
		}
	}

	if uint64(len(out)) != expected {
		return nil, errf("snappy 解压长度不符：期望 %d 实际 %d", expected, len(out))
	}
	return out, nil
}

func writeVarint(dst []byte, v uint64) []byte {
	for v >= 0x80 {
		dst = append(dst, byte(v)|0x80)
		v >>= 7
	}
	return append(dst, byte(v))
}

func CompressLiteral(data []byte) []byte {
	out := writeVarint(nil, uint64(len(data)))
	pos := 0
	for pos < len(data) {

		chunk := len(data) - pos
		if chunk > 65536 {
			chunk = 65536
		}
		n := chunk - 1
		if n < 60 {
			out = append(out, byte(n<<2))
		} else if n < 256 {
			out = append(out, byte(60<<2), byte(n))
		} else if n < 65536 {
			out = append(out, byte(61<<2), byte(n), byte(n>>8))
		} else {
			out = append(out, byte(62<<2), byte(n), byte(n>>8), byte(n>>16))
		}
		out = append(out, data[pos:pos+chunk]...)
		pos += chunk
	}
	return out
}
