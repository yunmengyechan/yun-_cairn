package mc

import (
	"errors"
	"fmt"
)

type BlockPos struct {
	X, Y, Z int32
}

func rdBlockPos(b []byte, off int) (BlockPos, int, error) {
	var p BlockPos
	var err error
	if p.X, off, err = rdZigzag32(b, off); err != nil {
		return p, off, fmt.Errorf("blockPos.x: %w", err)
	}
	y, off2, err := rdVarUInt(b, off)
	if err != nil {
		return p, off, fmt.Errorf("blockPos.y: %w", err)
	}
	p.Y = int32(y)
	off = off2
	if p.Z, off, err = rdZigzag32(b, off); err != nil {
		return p, off, fmt.Errorf("blockPos.z: %w", err)
	}
	return p, off, nil
}

func rdZigzag32(b []byte, off int) (int32, int, error) {
	v, n, err := rdVarUInt(b, off)
	if err != nil {
		return 0, off, err
	}
	return int32(v>>1) ^ -int32(v&1), n, nil
}

type CommandOutput struct {
	OriginType   uint32
	OriginUUID   [16]byte
	RequestID    string
	PlayerID     int64
	OutputType   byte
	SuccessCount uint32
	Messages     []CommandMessage
	DataSet      string
}

type CommandMessage struct {
	Success bool
	Message string
	Params  []string
}

func ParseCommandOutput(pkt []byte) (*CommandOutput, error) {
	_, n := ReadVarint(pkt, 0)
	off := int(n)
	co := &CommandOutput{}
	var err error

	if co.OriginType, off, err = rdVarUInt(pkt, off); err != nil {
		return nil, fmt.Errorf("origin: %w", err)
	}
	if off+16 > len(pkt) {
		return nil, errors.New("origin UUID 越界")
	}
	copy(co.OriginUUID[:], pkt[off:off+16])
	off += 16
	if co.RequestID, off, err = rdStringUTF(pkt, off); err != nil {
		return nil, fmt.Errorf("requestID: %w", err)
	}

	if co.OriginType == 3 || co.OriginType == 4 {
		if co.PlayerID, off, err = rdZigzag64(pkt, off); err != nil {
			return nil, fmt.Errorf("playerID: %w", err)
		}
	}

	if co.OutputType, off, err = rdU8(pkt, off); err != nil {
		return nil, fmt.Errorf("outputType: %w", err)
	}
	if co.SuccessCount, off, err = rdVarUInt(pkt, off); err != nil {
		return nil, fmt.Errorf("successCount: %w", err)
	}
	cnt, off, err := rdVarUInt(pkt, off)
	if err != nil {
		return nil, fmt.Errorf("messageCount: %w", err)
	}
	if cnt > 1024 {
		return nil, fmt.Errorf("输出条数异常：%d", cnt)
	}

	for i := uint32(0); i < cnt; i++ {
		var m CommandMessage
		if m.Success, off, err = rdBool(pkt, off); err != nil {
			return co, nil
		}
		if m.Message, off, err = rdStringUTF(pkt, off); err != nil {
			return co, nil
		}
		pc, off2, err := rdVarUInt(pkt, off)
		if err != nil {
			return co, nil
		}
		off = off2
		if pc > 256 {
			return co, nil
		}
		for j := uint32(0); j < pc; j++ {
			var pv string
			if pv, off, err = rdStringUTF(pkt, off); err != nil {
				return co, nil
			}
			m.Params = append(m.Params, pv)
		}
		co.Messages = append(co.Messages, m)
	}

	if co.OutputType == 4 {
		co.DataSet, _, _ = rdStringUTF(pkt, off)
	}
	return co, nil
}

func (c *CommandOutput) Text() string {
	if c == nil {
		return ""
	}
	var sb []byte
	for i, m := range c.Messages {
		if i > 0 {
			sb = append(sb, ';', ' ')
		}
		sb = append(sb, m.Message...)
	}
	return string(sb)
}

type ContainerOpenInfo struct {
	WindowID   int8
	WindowType int8
	Pos        BlockPos
	EntityID   int64
}

func ParseContainerOpen(pkt []byte) (*ContainerOpenInfo, error) {
	_, n := ReadVarint(pkt, 0)
	off := int(n)
	c := &ContainerOpenInfo{}
	var err error
	var b1, b2 byte
	if b1, off, err = rdU8(pkt, off); err != nil {
		return nil, err
	}
	c.WindowID = int8(b1)
	if b2, off, err = rdU8(pkt, off); err != nil {
		return nil, err
	}
	c.WindowType = int8(b2)
	if c.Pos, off, err = rdBlockPos(pkt, off); err != nil {
		return nil, err
	}
	if c.EntityID, _, err = rdZigzag64(pkt, off); err != nil {
		return nil, err
	}
	return c, nil
}

type RespawnInfo struct {
	X, Y, Z  float32
	State    byte
	EntityID uint64
}

const (
	RespawnSearching    = 0
	RespawnReadyToSpawn = 1
	RespawnClientReady  = 2
)

func ParseRespawn(pkt []byte) (*RespawnInfo, error) {
	_, n := ReadVarint(pkt, 0)
	off := int(n)
	r := &RespawnInfo{}
	var err error
	if r.X, off, err = rdF32(pkt, off); err != nil {
		return nil, err
	}
	if r.Y, off, err = rdF32(pkt, off); err != nil {
		return nil, err
	}
	if r.Z, off, err = rdF32(pkt, off); err != nil {
		return nil, err
	}
	if r.State, off, err = rdU8(pkt, off); err != nil {
		return nil, err
	}

	if r.EntityID, _, err = rdVarUInt64(pkt, off); err != nil {
		return nil, err
	}
	return r, nil
}

type HealthInfo struct {
	Health    int32
	MaxHealth int32
}

func ParseSetHealth(pkt []byte) (*HealthInfo, error) {
	_, n := ReadVarint(pkt, 0)
	off := int(n)
	h := &HealthInfo{}
	var err error

	if h.Health, off, err = rdZigzag32(pkt, off); err != nil {
		return nil, err
	}
	if h.MaxHealth, _, err = rdZigzag32(pkt, off); err != nil {

		h.MaxHealth = 20
	}
	return h, nil
}

type DeathInfo struct {
	Cause    string
	Messages []string
}

func ParseDeathInfo(pkt []byte) (*DeathInfo, error) {
	_, n := ReadVarint(pkt, 0)
	off := int(n)
	d := &DeathInfo{}
	var err error
	if d.Cause, off, err = rdStringUTF(pkt, off); err != nil {
		return nil, err
	}
	cnt, off, err := rdVarUInt(pkt, off)
	if err != nil {
		return d, nil
	}
	if cnt > 256 {
		return d, nil
	}
	for i := uint32(0); i < cnt; i++ {
		var s string
		if s, off, err = rdStringUTF(pkt, off); err != nil {
			break
		}
		d.Messages = append(d.Messages, s)
	}
	return d, nil
}
