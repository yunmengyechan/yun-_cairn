package mc

import (
	"fmt"
	"strings"
	"time"
)

type HostOptions struct {
	ResID     string
	Slogan    string
	Password  string
	MinLevel  int
	MaxCount  int
	Version   string
	SaveID    any
	WaitReady time.Duration
	PollEvery time.Duration

	Settle time.Duration

	EntityID string

	AlwaysEnter bool
}

type HostResult struct {
	EntityID string
	RoomName string
	Host     string
	Port     int
	Server   *GameServer
	Reused   bool
}

func (r *HostResult) NeteaseSID() string {
	if r == nil || r.EntityID == "" {
		return ""
	}
	return r.EntityID + ":LobbyGame"
}

func (c *LobbyClient) HostOwnRoom(o HostOptions) (*HostResult, error) {
	if o.WaitReady <= 0 {
		o.WaitReady = 150 * time.Second
	}
	if o.PollEvery <= 0 {
		o.PollEvery = 3 * time.Second
	}
	if o.Settle <= 0 {
		o.Settle = 30 * time.Second
	}
	if o.Version == "" {
		o.Version = "1.21.130"
	}
	if o.ResID == "" {
		o.ResID = DefaultResID
	}

	res := &HostResult{EntityID: o.EntityID, Reused: o.EntityID != ""}

	if res.EntityID == "" {

		if my, err := c.MyRoom(); err == nil && my != nil {
			if cur := strOf(my["room_id"]); cur != "" {
				logf("[自建房] 当前在房间 %s，先退房…", cur)
				if ep, e2 := c.LeaveRoomHard(cur); e2 != nil {
					logf("[自建房] 退房失败（继续尝试建房）：%v", e2)
				} else {
					logf("[自建房] 已退房（端点 %s），等 %.0f 秒让服务端清会话再建…", ep, o.Settle.Seconds())
					time.Sleep(HumanJitter(o.Settle, 10))
				}
			}
		}

		ent, err := c.CreateRoomRetry(CreateRoomOptions{
			ResID:    o.ResID,
			SaveID:   o.SaveID,
			Slogan:   o.Slogan,
			Password: o.Password,
			MinLevel: o.MinLevel,
			MaxCount: o.MaxCount,
			Version:  o.Version,
		}, 5, 2*time.Second)
		if err != nil {
			return nil, fmt.Errorf("建房失败：%w", err)
		}
		res.EntityID = strOf(ent["entity_id"])
		res.RoomName = strOf(ent["room_name"])
		logf("[自建房] 建房成功：房间 %s（实体 %s）", res.RoomName, res.EntityID)

		if res.EntityID == "" {
			res.EntityID = strOf(ent["room_id"])
		}

		if res.EntityID == "" {
			if my, e := c.MyRoom(); e == nil && my != nil {
				res.EntityID = strOf(my["room_id"])
				if res.RoomName == "" {
					res.RoomName = strOf(my["room_name"])
				}
				logf("[自建房] 建房返回没有 entity_id，用 myroom 补：%s", res.EntityID)
			}
		}
		if res.EntityID == "" {
			return nil, fmt.Errorf("建房返回里没有 entity_id，无法确定性进房（拒绝猜测，避免进别人房）")
		}
	} else {
		logf("[自建房] 复用实体 %s（不新建房间）", res.EntityID)
	}

	already := false
	if my, err := c.MyRoom(); err == nil && my != nil {
		if strOf(my["room_id"]) == res.EntityID {
			already = true
			logf("[自建房] 已经在大厅的该房间内，跳过 room_enter")
		}
	}
	if !already || o.AlwaysEnter {
		if already {
			logf("[自建房] 已经在大厅房内，但按 always_enter 再进一次（验证 92 是否与此有关）")
		}
		logf("[自建房] room_enter(entity_id=%s)…", res.EntityID)
		if err := c.EnterRoom(res.EntityID, o.Password); err != nil {

			if strings.Contains(err.Error(), "12003") {
				logf("[自建房] 服务端说已在房内（12003），继续")
			} else {
				return res, fmt.Errorf("进房失败：%w", err)
			}
		} else {
			logf("[自建房] 进房 OK")
		}
	}

	if gs, err := c.GameEnter(); err == nil && gs != nil && gs.Host != "" {
		logf("[自建房] 服务器已在运行，直接拿到地址 %s:%d", gs.Host, gs.Port)
		fillHostServer(res, gs)
		return res, nil
	} else if err != nil {
		logf("[自建房] 首次 game_enter 未就绪：%v", err)
	}

	logf("[自建房] game-control start")
	if err := c.GameControl("start"); err != nil {
		logf("[自建房] 开局返回：%v（继续等地址）", err)
	} else {
		logf("[自建房] 开局 OK")
	}

	deadline := time.Now().Add(o.WaitReady)
	lastLog := time.Now()
	lastErr := ""
	for time.Now().Before(deadline) {
		gs, err := c.GameEnter()
		if err == nil && gs != nil && gs.Host != "" {
			logf("[自建房] 拿到地址 %s:%d", gs.Host, gs.Port)
			fillHostServer(res, gs)
			return res, nil
		}
		if err != nil {
			lastErr = err.Error()
		}
		if time.Since(lastLog) >= 15*time.Second {
			lastLog = time.Now()
			logf("[自建房] 还在等服务器就绪…（最后返回：%s）", lastErr)
		}
		time.Sleep(o.PollEvery)
	}
	if lastErr == "" {
		lastErr = "game_enter 一直没返回地址"
	}
	return res, fmt.Errorf("等了 %.0f 秒还没拿到游戏地址（最后返回：%s）", o.WaitReady.Seconds(), lastErr)
}

func fillHostServer(r *HostResult, gs *GameServer) {
	r.Server = gs
	r.Host = gs.Host
	r.Port = gs.Port
}
