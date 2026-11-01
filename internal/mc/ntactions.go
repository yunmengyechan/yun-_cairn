package mc

import (
	"fmt"
	"time"
)

func (s *Session) needLogin(what string) error {
	if !s.LoginSuccess() {
		return fmt.Errorf("还没登录成功，不能%s", what)
	}
	return nil
}

func (s *Session) RuntimeID() int64 {
	return s.playerID
}

func (s *Session) SendPlayerAction(actionType int) error {
	if err := s.needLogin("发玩家行为"); err != nil {
		return err
	}
	pkt := BuildPlayerAction(uint64(s.playerID), actionType, 0, 0, 0, 0, 0, 0, 0)
	return s.SendPacket(pkt)
}

func (s *Session) SendPlayerActionOnBlock(actionType int, bx, by, bz int32, face int32) error {
	if err := s.needLogin("发方块行为"); err != nil {
		return err
	}
	pkt := BuildPlayerAction(uint64(s.playerID), actionType, bx, by, bz, bx, by, bz, face)
	return s.SendPacket(pkt)
}

func (s *Session) SendMovePlayer(x, y, z float32, mode byte) error {
	if err := s.needLogin("发移动包"); err != nil {
		return err
	}
	s.authMu.Lock()
	pitch, yaw := s.auth.Pitch, s.auth.Yaw
	s.auth.X, s.auth.Y, s.auth.Z = x, y, z
	tick := s.auth.Tick
	s.auth.Tick++
	onGround := s.auth.OnGround
	s.authMu.Unlock()

	pkt := BuildMovePlayer(s.playerID, x, y, z, pitch, yaw, yaw, mode, onGround, 0, tick)
	if err := s.SendPacket(pkt); err != nil {
		return err
	}

	if mode == MoveModeTeleport {
		_ = s.PublishChunks(x, y, z, s.chunkRadius)
	}
	return nil
}

const (
	containerOpenWait  = 150 * time.Millisecond
	containerOpenRetry = 35
)

func (s *Session) OpenContainerAt(bx, by, bz int32) error {
	if err := s.needLogin("打开容器"); err != nil {
		return err
	}
	x, y, z, _ := s.Pos()

	pkt := BuildInventoryTransactionUseItem(bx, by, bz, 1,
		x, y, z,
		float32(bx)+0.5, float32(by)+1.0, float32(bz)+0.5)

	s.containerMu.Lock()
	before := s.containerOpenedAt
	s.containerMu.Unlock()

	for i := 0; i < containerOpenRetry; i++ {
		if err := s.SendPacket(pkt); err != nil {
			return err
		}
		time.Sleep(containerOpenWait)

		s.containerMu.Lock()
		opened := s.containerOpenedAt.After(before)
		wid := s.containerWindow
		typ := s.containerType
		s.containerMu.Unlock()

		if opened {
			s.log("✓ 容器已打开 windowID=%d type=%d（第 %d 次点击，(%d,%d,%d)）",
				wid, typ, i+1, bx, by, bz)
			return nil
		}
	}
	return fmt.Errorf("打开容器超时：点了 %d 次都没收到 ContainerOpen（(%d,%d,%d)，检查距离/朝向/方块是否已加载）",
		containerOpenRetry, bx, by, bz)
}

func (s *Session) OpenInventory() error {
	if err := s.needLogin("打开背包"); err != nil {
		return err
	}
	pkt := BuildInteractOpenInventory(s.RuntimeID())
	if err := s.SendPacket(pkt); err != nil {
		return err
	}
	s.log("打开背包")
	return nil
}

func (s *Session) CloseContainer(windowID byte) error {
	if err := s.needLogin("关闭容器"); err != nil {
		return err
	}
	return s.SendPacket(BuildContainerClose(windowID, false))
}

func (s *Session) UpdateCommandBlock(o CommandBlockOptions) error {
	if err := s.needLogin("写命令方块"); err != nil {
		return err
	}
	o.Block = true
	o.ShouldTrackOutput = false
	if err := s.SendPacket(BuildCommandBlockUpdate(o)); err != nil {
		return err
	}
	s.log("命令方块 (%d,%d,%d) 已更新: %s", o.X, o.Y, o.Z, o.Command)
	return nil
}

func (s *Session) SetInputFlags(flags ...int) {
	s.authMu.Lock()
	s.auth.KeyMode = AuthKeyMode(flags...)
	s.authMu.Unlock()
}

func (s *Session) ClearInputFlags() {
	s.authMu.Lock()
	s.auth.KeyMode = 0
	s.authMu.Unlock()
}

func (s *Session) SetMoveVector(dx, dz float32) {
	s.authMu.Lock()
	s.auth.MoveVecX, s.auth.MoveVecY = dx, dz
	s.authMu.Unlock()
}
