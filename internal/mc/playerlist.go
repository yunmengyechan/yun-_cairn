package mc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf8"
)

func rdVarUInt(b []byte, off int) (uint32, int, error) {
	v, n := ReadVarint(b, off)
	if n == 0 {
		return 0, off, errors.New("varint 读失败")
	}
	return v, off + int(n), nil
}

func rdStringUTF(b []byte, off int) (string, int, error) {
	n, off2, err := rdVarUInt(b, off)
	if err != nil {
		return "", off, err
	}
	if off2+int(n) > len(b) {
		return "", off, fmt.Errorf("字符串越界：需 %d 字节，剩 %d", n, len(b)-off2)
	}
	s := string(b[off2 : off2+int(n)])
	if !utf8.ValidString(s) {
		s = string([]rune(s))
	}
	return s, off2 + int(n), nil
}

func rdByteSlice(b []byte, off int) ([]byte, int, error) {
	n, off2, err := rdVarUInt(b, off)
	if err != nil {
		return nil, off, err
	}
	if off2+int(n) > len(b) {
		return nil, off, fmt.Errorf("字节块越界：需 %d，剩 %d", n, len(b)-off2)
	}
	out := make([]byte, n)
	copy(out, b[off2:off2+int(n)])
	return out, off2 + int(n), nil
}

func rdU8(b []byte, off int) (byte, int, error) {
	if off >= len(b) {
		return 0, off, errors.New("读 u8 越界")
	}
	return b[off], off + 1, nil
}

func rdI32(b []byte, off int) (int32, int, error) {
	if off+4 > len(b) {
		return 0, off, errors.New("读 i32 越界")
	}
	return int32(binary.LittleEndian.Uint32(b[off : off+4])), off + 4, nil
}

func rdBool(b []byte, off int) (bool, int, error) {
	v, off2, err := rdU8(b, off)
	return v != 0, off2, err
}

func rdZigzag64(b []byte, off int) (int64, int, error) {
	v, n, err := rdVarUInt64(b, off)
	if err != nil {
		return 0, off, err
	}
	return int64(v>>1) ^ -int64(v&1), n, nil
}

func rdVarUInt64(b []byte, off int) (uint64, int, error) {
	var v uint64
	var shift uint
	for i := off; i < len(b); i++ {
		c := b[i]
		v |= uint64(c&0x7F) << shift
		if c&0x80 == 0 {
			return v, i + 1, nil
		}
		shift += 7
		if shift > 63 {
			return 0, off, errors.New("varint64 过长")
		}
	}
	return 0, off, errors.New("varint64 截断")
}

const (
	PlayerListAdd    = 0
	PlayerListRemove = 1
)

type PlayerEntry struct {
	UUID     [16]byte
	UniqueID int64
	Username string
	XUID     string
	ChatID   string
	Platform int32
	Verified bool

	SkinID     string
	SkinWidth  int32
	SkinHeight int32
}

type PlayerList struct {
	Action  byte
	Entries []PlayerEntry
}

func (p *PlayerList) IsAdd() bool { return p != nil && p.Action == PlayerListAdd }

func ParsePlayerList(pkt []byte) (*PlayerList, error) {

	_, n := ReadVarint(pkt, 0)
	off := int(n)

	action, off, err := rdU8(pkt, off)
	if err != nil {
		return nil, err
	}
	if action > PlayerListRemove {
		return nil, fmt.Errorf("非法的 PlayerList action：%d", action)
	}

	count, off, err := rdVarUInt(pkt, off)
	if err != nil {
		return nil, err
	}
	if count > 4096 {
		return nil, fmt.Errorf("PlayerList 条目数异常：%d", count)
	}

	pl := &PlayerList{Action: action}
	pl.Entries = make([]PlayerEntry, 0, count)

	for i := uint32(0); i < count; i++ {
		var e PlayerEntry
		if action == PlayerListRemove {

			if off+16 > len(pkt) {
				return nil, fmt.Errorf("Remove 第 %d 条 UUID 越界", i)
			}
			copy(e.UUID[:], pkt[off:off+16])
			off += 16
			pl.Entries = append(pl.Entries, e)
			continue
		}
		if e, off, err = readPlayerEntry(pkt, off); err != nil {
			return nil, fmt.Errorf("Add 第 %d 条: %w", i, err)
		}
		pl.Entries = append(pl.Entries, e)
	}

	if action == PlayerListAdd {
		for i := range pl.Entries {
			v, off2, err := rdBool(pkt, off)
			if err != nil {
				break
			}
			pl.Entries[i].Verified = v
			off = off2
		}
	}
	return pl, nil
}

func readPlayerEntry(b []byte, off int) (PlayerEntry, int, error) {
	var e PlayerEntry
	var err error

	if off+16 > len(b) {
		return e, off, errors.New("UUID 越界")
	}
	copy(e.UUID[:], b[off:off+16])
	off += 16

	if e.UniqueID, off, err = rdZigzag64(b, off); err != nil {
		return e, off, fmt.Errorf("uniqueID: %w", err)
	}
	if e.Username, off, err = rdStringUTF(b, off); err != nil {
		return e, off, fmt.Errorf("username: %w", err)
	}
	if e.XUID, off, err = rdStringUTF(b, off); err != nil {
		return e, off, fmt.Errorf("xuid: %w", err)
	}
	if e.ChatID, off, err = rdStringUTF(b, off); err != nil {
		return e, off, fmt.Errorf("chatID: %w", err)
	}
	if e.Platform, off, err = rdI32(b, off); err != nil {
		return e, off, fmt.Errorf("platform: %w", err)
	}

	skinID, off, err := rdStringUTF(b, off)
	if err != nil {
		return e, off, fmt.Errorf("skin.skinID: %w", err)
	}
	e.SkinID = skinID

	for k := 0; k < 2; k++ {
		if _, off, err = rdStringUTF(b, off); err != nil {
			return e, off, fmt.Errorf("skin 前置串 %d: %w", k, err)
		}
	}

	if e.SkinWidth, off, err = rdI32(b, off); err != nil {
		return e, off, err
	}
	if e.SkinHeight, off, err = rdI32(b, off); err != nil {
		return e, off, err
	}
	if _, off, err = rdByteSlice(b, off); err != nil {
		return e, off, fmt.Errorf("skin 图: %w", err)
	}

	animCount, off, err := rdVarUInt(b, off)
	if err != nil {
		return e, off, err
	}
	if animCount > 512 {
		return e, off, fmt.Errorf("动画数异常：%d", animCount)
	}
	for k := uint32(0); k < animCount; k++ {
		if off, err = skipSkinImage(b, off); err != nil {
			return e, off, err
		}
		if _, off, err = rdI32(b, off); err != nil {
			return e, off, err
		}
		if _, off, err = rdF32(b, off); err != nil {
			return e, off, err
		}
		if _, off, err = rdF32(b, off); err != nil {
			return e, off, err
		}
	}

	if off, err = skipSkinImage(b, off); err != nil {
		return e, off, err
	}

	for k := 0; k < 7; k++ {
		if _, off, err = rdStringUTF(b, off); err != nil {
			return e, off, fmt.Errorf("skin 串 %d: %w", k, err)
		}
	}

	pc, off, err := rdVarUInt(b, off)
	if err != nil {
		return e, off, err
	}
	if pc > 512 {
		return e, off, fmt.Errorf("persona 数异常：%d", pc)
	}
	for k := uint32(0); k < pc; k++ {
		for j := 0; j < 3; j++ {
			if _, off, err = rdStringUTF(b, off); err != nil {
				return e, off, err
			}
		}
		if _, off, err = rdBool(b, off); err != nil {
			return e, off, err
		}
		if _, off, err = rdStringUTF(b, off); err != nil {
			return e, off, err
		}
	}

	tc, off, err := rdVarUInt(b, off)
	if err != nil {
		return e, off, err
	}
	if tc > 512 {
		return e, off, fmt.Errorf("tint 组数异常：%d", tc)
	}
	for k := uint32(0); k < tc; k++ {
		if _, off, err = rdStringUTF(b, off); err != nil {
			return e, off, err
		}
		cc, off2, err := rdVarUInt(b, off)
		if err != nil {
			return e, off, err
		}
		off = off2
		if cc > 512 {
			return e, off, fmt.Errorf("tint 颜色数异常：%d", cc)
		}
		for j := uint32(0); j < cc; j++ {
			if _, off, err = rdStringUTF(b, off); err != nil {
				return e, off, err
			}
		}
	}

	for k := 0; k < 5; k++ {
		if _, off, err = rdBool(b, off); err != nil {
			return e, off, err
		}
	}

	return e, off, nil
}

func skipSkinImage(b []byte, off int) (int, error) {
	var err error
	if _, off, err = rdI32(b, off); err != nil {
		return off, err
	}
	if _, off, err = rdI32(b, off); err != nil {
		return off, err
	}
	if _, off, err = rdByteSlice(b, off); err != nil {
		return off, err
	}
	return off, nil
}
