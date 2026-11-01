package mc

import (
	"fmt"
	"time"
)

type AccountKeepaliveResult struct {
	At        time.Time
	InRoom    bool
	RoomID    string
	Err       error
	Reentered bool
}

func (s *Session) AccountKeepAlive(ch chan struct{}, interval time.Duration, autoReenter bool,
	onResult func(AccountKeepaliveResult)) {

	if s.cfg.Lobby == nil {
		s.log("账号保活：没配大厅客户端，跳过")
		return
	}
	if interval <= 0 {
		interval = time.Duration(s.cfg.AccountKeepAliveS) * time.Second
	}
	if interval <= 0 {
		interval = 45 * time.Second
	}

	s.log("账号保活已启动（间隔 %.0f 秒，大厅接口）", interval.Seconds())
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ch:
			s.log("账号保活已停止")
			return
		case <-t.C:
			r := s.accountKeepAliveOnce(autoReenter)
			if onResult != nil {
				onResult(r)
			}
		}
	}
}

func (s *Session) AccountKeepAliveCheck(autoReenter bool) AccountKeepaliveResult {
	return s.accountKeepAliveOnce(autoReenter)
}

func (s *Session) accountKeepAliveOnce(autoReenter bool) AccountKeepaliveResult {
	r := AccountKeepaliveResult{At: time.Now()}
	lobby := s.cfg.Lobby

	my, err := lobby.MyRoom()
	if err != nil {
		r.Err = err
		s.log("账号保活：查房间失败 %v", err)
		return r
	}

	if my != nil {
		r.RoomID = strOf(my["room_id"])
		if r.RoomID == "" {
			if ent, ok := my["entity"].(map[string]any); ok && ent != nil {
				r.RoomID = strOf(ent["entity_id"])
			}
		}
	}
	r.InRoom = r.RoomID != ""

	if r.InRoom {
		s.log("账号保活：仍在房间 %s", r.RoomID)
		return r
	}

	s.log("⚠ 账号保活：已不在房间里")
	if !autoReenter || s.cfg.ReenterRoomID == "" {
		return r
	}

	s.log("账号保活：尝试重进房间 %s", s.cfg.ReenterRoomID)
	if err := lobby.EnterRoom(s.cfg.ReenterRoomID, ""); err != nil {
		r.Err = fmt.Errorf("重进失败: %w", err)
		s.log("账号保活：重进失败 %v", err)
		return r
	}
	r.Reentered = true
	r.RoomID = s.cfg.ReenterRoomID
	r.InRoom = true
	s.log("账号保活：已重进房间 %s", r.RoomID)
	return r
}

func (s *Session) GameHeartbeatRunning() bool {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	return s.authInputOn
}

func (s *Session) StartGameHeartbeat(intervalMs int) {
	if intervalMs <= 0 {
		intervalMs = s.cfg.GameHeartbeatMs
	}
	if intervalMs <= 0 {
		intervalMs = 50
	}
	s.authMu.Lock()
	if s.authInputOn {
		s.authMu.Unlock()
		return
	}
	s.authInputEvery = time.Duration(intervalMs) * time.Millisecond
	s.authInputOn = true
	s.authMu.Unlock()
	s.log("游戏内心跳已启动（%d ms / %.0f Hz，PlayerAuthInput）", intervalMs, 1000.0/float64(intervalMs))
	go s.authInputLoop()
}

func (s *Session) StopGameHeartbeat() {
	s.authMu.Lock()
	s.authInputOn = false
	s.authMu.Unlock()
	s.log("游戏内心跳已停止")
}

func (s *Session) HeartbeatStatus() map[string]any {
	s.authMu.Lock()
	on := s.authInputOn
	iv := s.authInputEvery
	s.authMu.Unlock()
	ms := int(iv / time.Millisecond)
	if ms <= 0 {
		ms = 50
	}
	return map[string]any{
		"account_keepalive_s": s.cfg.AccountKeepAliveS,
		"game_heartbeat_on":   on,
		"game_heartbeat_ms":   ms,
		"game_heartbeat_hz":   fmt.Sprintf("%.0f", 1000.0/float64(ms)),
	}
}

func (s *Session) HeartbeatMs() int { return s.cfg.GameHeartbeatMs }
