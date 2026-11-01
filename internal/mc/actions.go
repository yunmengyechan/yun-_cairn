package mc

import (
	"fmt"
	"sync"
	"time"
)

type AuthState struct {
	X, Y, Z    float32
	Pitch, Yaw float32
	HeadYaw    float32
	OnGround   bool
	Tick       int64
	KeyMode    uint64
	MoveVecX   float32
	MoveVecY   float32
	Confirmed  bool
}

var _ = sync.Mutex{}

func (s *Session) MoveTo(x, y, z float32) error {
	if !s.LoginSuccess() {
		return fmt.Errorf("还没登录成功，不能传送")
	}
	s.authMu.Lock()
	s.auth.X, s.auth.Y, s.auth.Z = x, y, z
	s.authMu.Unlock()

	if err := s.PublishChunks(x, y, z, s.chunkRadius); err != nil {
		return err
	}

	s.authMu.Lock()
	tick := s.auth.Tick
	s.authMu.Unlock()
	if err := s.sendAuthInputLocked(tick, false); err != nil {
		return err
	}
	s.log("★ 已传送到 (%.1f, %.1f, %.1f)", x, y, z)
	return nil
}

func (s *Session) AddPos(dx, dy, dz float32) error {
	s.authMu.Lock()
	x, y, z := s.auth.X+dx, s.auth.Y+dy, s.auth.Z+dz
	s.authMu.Unlock()
	return s.MoveTo(x, y, z)
}

func (s *Session) Pos() (x, y, z float32, confirmed bool) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	return s.auth.X, s.auth.Y, s.auth.Z, s.auth.Confirmed
}

func (s *Session) Rot() (pitch, yaw float32) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	return s.auth.Pitch, s.auth.Yaw
}

func (s *Session) SetRot(pitch, yaw float32) error {
	s.authMu.Lock()
	s.auth.Pitch, s.auth.Yaw, s.auth.HeadYaw = pitch, yaw, yaw
	tick := s.auth.Tick
	s.authMu.Unlock()
	if !s.LoginSuccess() {
		return nil
	}
	return s.sendAuthInputLocked(tick, false)
}

func (s *Session) AuthInputRunning() bool {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	return s.authInputOn
}

func (s *Session) SetAuthInput(on bool) {
	s.authMu.Lock()
	was := s.authInputOn
	s.authInputOn = on
	s.authMu.Unlock()
	if on && !was {
		go s.authInputLoop()
		s.log("AuthInput 心跳已开启（50ms / 20Hz）")
	} else if !on && was {
		s.log("AuthInput 心跳已关闭")
	}
}

func (s *Session) authInputLoop() {
	s.authMu.Lock()
	iv := s.authInputEvery
	s.authMu.Unlock()
	if iv <= 0 {
		iv = 50 * time.Millisecond
	}
	t := time.NewTicker(iv)
	defer t.Stop()
	for range t.C {
		if s.closed {
			return
		}
		s.authMu.Lock()
		on := s.authInputOn
		tick := s.auth.Tick
		s.auth.Tick++
		s.authMu.Unlock()
		if !on {
			return
		}
		if !s.LoginSuccess() {
			continue
		}
		if err := s.sendAuthInputLocked(tick, false); err != nil {
			return
		}
	}
}

func newSessionAuthState() AuthState {
	return AuthState{OnGround: true}
}

func (s *Session) sendAuthInputLocked(tick int64, resetPos bool) error {
	s.authMu.Lock()
	pitch, yaw := s.auth.Pitch, s.auth.Yaw
	x, y, z := s.auth.X, s.auth.Y, s.auth.Z
	mvx, mvy := s.auth.MoveVecX, s.auth.MoveVecY
	keyMode := s.auth.KeyMode
	onGround := s.auth.OnGround
	s.authMu.Unlock()
	pkt := BuildPlayerAuthInputEx(pitch, yaw, x, y, z, mvx, mvy, yaw,
		keyMode, tick, 0, 0, 0, 0, 0, 0, onGround, resetPos)
	return s.SendPacket(pkt)
}

func (s *Session) Respawn() error {
	if !s.LoginSuccess() {
		return fmt.Errorf("还没登录成功")
	}
	s.authMu.Lock()
	tick := s.auth.Tick
	s.auth.Tick++
	s.authMu.Unlock()
	if err := s.sendAuthInputLocked(tick, true); err != nil {
		return err
	}
	s.log("★ 已发送重生请求（AuthInput resetPosition）")
	return nil
}

func (s *Session) PublishChunks(x, y, z float32, radius int) error {
	if radius <= 0 {
		radius = 8
	}
	pkt := BuildNetworkChunkPublisherUpdate(int32(x), int32(y), int32(z), radius)
	if err := s.SendPacket(pkt); err != nil {
		return err
	}
	s.log("视口更新 -> (%d, %d, %d) 半径 %d", int32(x), int32(y), int32(z), radius)
	return nil
}

func (s *Session) RequestSubChunk(cx, cy, cz int, offsets [][3]int8) error {
	if len(offsets) == 0 {
		return fmt.Errorf("offsets 为空")
	}
	pkt := BuildSubChunkRequest(0, cx, cy, cz, offsets)
	if err := s.SendPacket(pkt); err != nil {
		return err
	}
	s.log("子区块请求 -> (%d,%d,%d) 共 %d 个", cx, cy, cz, len(offsets))
	return nil
}

func BuildPlayerAuthInputFull(pitch, yaw, x, y, z float32, tick int64, resetPos bool) []byte {
	b := BuildPlayerAuthInput(pitch, yaw, x, y, z, tick)

	if resetPos && len(b) >= 2 {
		b[len(b)-1] = 1
	}
	return b
}

func BuildNetworkChunkPublisherUpdate(x, y, z int32, radius int) []byte {
	b := WriteVarint(IDNetworkChunkPublisherUpdate)
	b = append(b, WriteZigzag32(x)...)
	b = append(b, WriteZigzag32(y)...)
	b = append(b, WriteZigzag32(z)...)
	b = append(b, WriteVarint(uint32(radius))...)
	return b
}

func BuildSubChunkRequest(dimension int32, cx, cy, cz int, offsets [][3]int8) []byte {
	b := WriteVarint(IDSubChunkRequest)
	b = append(b, WriteZigzag32(dimension)...)
	b = append(b, WriteZigzag32(int32(cx))...)
	b = append(b, WriteZigzag32(int32(cy))...)
	b = append(b, WriteZigzag32(int32(cz))...)
	b = append(b, WriteVarint(uint32(len(offsets)))...)
	for _, o := range offsets {
		b = append(b, byte(o[0]), byte(o[1]), byte(o[2]))
	}
	return b
}
