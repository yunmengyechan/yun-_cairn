package raknet

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type Frame struct {
	Reliability    int
	Body           []byte
	ReliableIndex  uint32
	SequencedIndex uint32
	OrderedIndex   uint32
	OrderChannel   byte
	Fragmented     bool
	CompoundSize   uint32
	CompoundID     uint16
	FragmentIndex  uint32
}

func (f *Frame) Size() int {
	s := 3
	if IsReliable(f.Reliability) {
		s += 3
	}
	if IsSequenced(f.Reliability) {
		s += 4
	} else if IsOrdered(f.Reliability) {
		s += 4
	}
	if f.Fragmented {
		s += 10
	}
	s += len(f.Body)
	return s
}

func PackFrame(f *Frame) []byte {
	r := make([]byte, 0, f.Size())

	flags := byte((f.Reliability & 0x07) << 5)
	if f.Fragmented {
		flags |= 0x10
	}
	r = append(r, flags)

	lengthInBits := len(f.Body) * 8
	var lb [2]byte
	binary.BigEndian.PutUint16(lb[:], uint16(lengthInBits))
	r = append(r, lb[:]...)

	if IsReliable(f.Reliability) {
		r = appendUint24LE(r, f.ReliableIndex)
	}

	if IsSequenced(f.Reliability) {
		r = appendUint24LE(r, f.SequencedIndex)
		r = append(r, f.OrderChannel)
	} else if IsOrdered(f.Reliability) {
		r = appendUint24LE(r, f.OrderedIndex)
		r = append(r, f.OrderChannel)
	}

	if f.Fragmented {
		var cs [4]byte
		binary.BigEndian.PutUint32(cs[:], f.CompoundSize)
		r = append(r, cs[:]...)
		var ci [2]byte
		binary.BigEndian.PutUint16(ci[:], f.CompoundID)
		r = append(r, ci[:]...)
		var fi [4]byte
		binary.BigEndian.PutUint32(fi[:], f.FragmentIndex)
		r = append(r, fi[:]...)
	}

	r = append(r, f.Body...)
	return r
}

func PackFrameSet(seq uint32, frames []*Frame) []byte {
	if len(frames) == 0 {
		return nil
	}
	count := len(frames)
	if count > 14 {
		count = 14
	}

	r := make([]byte, 0, 64)
	r = append(r, byte(0x80+count-1))
	r = appendUint24LE(r, seq)
	for i := 0; i < count; i++ {
		r = append(r, PackFrame(frames[i])...)
	}
	return r
}

func UnpackFrameSet(data []byte) (uint32, []*Frame, error) {
	if len(data) < 4 {
		return 0, nil, errors.New("数据太短")
	}
	pid := data[0]
	if pid < 0x80 || pid > 0x8d {
		return 0, nil, fmt.Errorf("不是 FrameSet: 0x%02x", pid)
	}
	seq, off := readUint24LE(data, 1)
	frames := []*Frame{}
	for off < len(data) {
		f, next, err := UnpackFrame(data, off)
		if err != nil {
			break
		}
		frames = append(frames, f)
		off = next
	}
	return seq, frames, nil
}

func UnpackFrame(data []byte, off int) (*Frame, int, error) {
	if off+3 > len(data) {
		return nil, off, errors.New("帧头不足")
	}
	flags := data[off]
	off++
	rel := int((flags >> 5) & 0x07)
	frag := (flags & 0x10) != 0

	lengthInBits := int(binary.BigEndian.Uint16(data[off : off+2]))
	off += 2
	bodyLen := (lengthInBits + 7) / 8

	f := &Frame{Reliability: rel, Fragmented: frag}

	if IsReliable(rel) {
		v, n := readUint24LE(data, off)
		f.ReliableIndex = v
		off = n
	}
	if IsSequenced(rel) {
		v, n := readUint24LE(data, off)
		f.SequencedIndex = v
		off = n
		if off < len(data) {
			f.OrderChannel = data[off]
			off++
		}
	} else if IsOrdered(rel) {
		v, n := readUint24LE(data, off)
		f.OrderedIndex = v
		off = n
		if off < len(data) {
			f.OrderChannel = data[off]
			off++
		}
	}
	if frag {
		if off+10 > len(data) {
			return nil, off, errors.New("分片信息不足")
		}
		f.CompoundSize = binary.BigEndian.Uint32(data[off : off+4])
		off += 4
		f.CompoundID = binary.BigEndian.Uint16(data[off : off+2])
		off += 2
		f.FragmentIndex = binary.BigEndian.Uint32(data[off : off+4])
		off += 4
	}

	if off+bodyLen > len(data) {
		bodyLen = len(data) - off
	}
	if bodyLen < 0 {
		bodyLen = 0
	}
	f.Body = append([]byte(nil), data[off:off+bodyLen]...)
	off += bodyLen
	return f, off, nil
}
