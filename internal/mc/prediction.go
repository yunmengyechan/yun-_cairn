package mc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

type MovePrediction struct {
	Type       byte
	X, Y, Z    float32
	Yaw, Pitch float32
	OnGround   bool
	Tick       int64
	HasAngular bool
	AngularVel float32
}

func rdF32(b []byte, off int) (float32, int, error) {
	if off+4 > len(b) {
		return 0, off, errors.New("字节不够读 float32")
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(b[off : off+4])), off + 4, nil
}

func ParseCorrectPlayerMovePrediction(pkt []byte) (*MovePrediction, error) {
	id, n := ReadVarint(pkt, 0)
	off := int(n)
	if id != IDCorrectPlayerMovePrediction {
		return nil, fmt.Errorf("不是 CorrectPlayerMovePrediction: 0x%02x", id)
	}
	if off >= len(pkt) {
		return nil, errors.New("包体为空")
	}

	m := &MovePrediction{Type: pkt[off]}
	off++

	var err error
	if m.X, off, err = rdF32(pkt, off); err != nil {
		return nil, fmt.Errorf("position.x: %w", err)
	}
	if m.Y, off, err = rdF32(pkt, off); err != nil {
		return nil, fmt.Errorf("position.y: %w", err)
	}
	if m.Z, off, err = rdF32(pkt, off); err != nil {
		return nil, fmt.Errorf("position.z: %w", err)
	}

	for i := 0; i < 3; i++ {
		if _, off, err = rdF32(pkt, off); err != nil {
			return nil, fmt.Errorf("delta[%d]: %w", i, err)
		}
	}

	if m.Yaw, off, err = rdF32(pkt, off); err != nil {
		return nil, fmt.Errorf("rotation.yaw: %w", err)
	}
	if m.Pitch, off, err = rdF32(pkt, off); err != nil {
		return nil, fmt.Errorf("rotation.pitch: %w", err)
	}

	if off >= len(pkt) {
		return nil, errors.New("缺 angular 标志位")
	}
	if pkt[off] != 0 {
		m.HasAngular = true
		off++
		if m.AngularVel, off, err = rdF32(pkt, off); err != nil {
			return nil, fmt.Errorf("angular_velocity: %w", err)
		}
	} else {
		off++
	}

	if off >= len(pkt) {
		return nil, errors.New("缺 on_ground")
	}
	m.OnGround = pkt[off] != 0
	off++

	if off < len(pkt) {
		if tv, ok := readZigzag64(pkt, off); ok {
			m.Tick = tv
		}
	}

	return m, nil
}

func readZigzag64(b []byte, off int) (int64, bool) {
	var raw uint64
	var shift uint
	for i := off; i < len(b); i++ {
		c := b[i]
		raw |= uint64(c&0x7F) << shift
		if c&0x80 == 0 {
			return int64(raw>>1) ^ -int64(raw&1), true
		}
		shift += 7
		if shift > 63 {
			return 0, false
		}
	}
	return 0, false
}

func (s *Session) ApplyMovePrediction(m *MovePrediction) (bool, float32, [3]float32, [3]float32) {
	if m == nil || m.Type != 0 {
		return false, 0, [3]float32{}, [3]float32{}
	}
	s.authMu.Lock()
	old := [3]float32{s.auth.X, s.auth.Y, s.auth.Z}
	s.auth.X, s.auth.Y, s.auth.Z = m.X, m.Y, m.Z
	if m.HasAngular {
		s.auth.Yaw = m.Yaw
		s.auth.Pitch = m.Pitch
	}
	s.auth.OnGround = m.OnGround
	s.auth.Confirmed = true
	if m.Tick > 0 {
		s.auth.Tick = m.Tick
	}
	s.authMu.Unlock()
	newP := [3]float32{m.X, m.Y, m.Z}
	d := float32(math.Sqrt(float64(
		(m.X-old[0])*(m.X-old[0]) +
			(m.Y-old[1])*(m.Y-old[1]) +
			(m.Z-old[2])*(m.Z-old[2]))))
	return true, d, old, newP
}
