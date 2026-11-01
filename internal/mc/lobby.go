package mc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var LogHook func(format string, a ...any)

func logf(format string, a ...any) {
	if LogHook != nil {
		LogHook(format, a...)
	}
}

const (
	LobbyGateway = "https://g79apigatewayobt.minecraft.cn"
	MCGateway    = "https://g79mclobt.minecraft.cn"

	TransferGateway = "https://g79mcltransfer.minecraft.cn"
)

type LobbyClient struct {
	Base   string
	UserID string
	Token  string
	HTTP   *http.Client

	OnAuthFail func() bool
}

func NewLobbyClient(base, userID, token string) *LobbyClient {
	if base == "" {
		base = LobbyGateway
	}
	return &LobbyClient{
		Base:   base,
		UserID: userID,
		Token:  token,
		HTTP:   &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *LobbyClient) Post(path string, body any) (map[string]any, error) {
	out, err := c.postOnce(path, body)
	if err == nil && needsAuth(codeOf(out)) && c.OnAuthFail != nil && c.OnAuthFail() {
		return c.postOnce(path, body)
	}
	return out, err
}

func (c *LobbyClient) postOnce(path string, body any) (map[string]any, error) {
	var payload []byte
	if body == nil {
		payload = []byte("{}")
	} else {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = b
	}
	req, err := http.NewRequest("POST", c.Base+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "libhttpclient/1.0.0.0")

	req.Header.Set("x_trace_id", string(randASCII(32)))
	if c.UserID != "" {
		req.Header.Set("user-id", c.UserID)
	}

	if c.Token != "" {
		req.Header.Set("user-token", ComputeDynamicToken(path, string(payload), c.Token))
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]any{"code": 0}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		n := len(raw)
		if n > 120 {
			n = 120
		}
		return nil, fmt.Errorf("响应不是 JSON(%d 字节): %s", len(raw), string(raw[:n]))
	}
	return out, nil
}

func (c *LobbyClient) Get(path string) (map[string]any, error) {
	out, err := c.getOnce(path)
	if err == nil && needsAuth(codeOf(out)) && c.OnAuthFail != nil && c.OnAuthFail() {
		return c.getOnce(path)
	}
	return out, err
}

func (c *LobbyClient) getOnce(path string) (map[string]any, error) {
	req, err := http.NewRequest("GET", c.Base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "libhttpclient/1.0.0.0")
	req.Header.Set("x_trace_id", string(randASCII(32)))
	if c.UserID != "" {
		req.Header.Set("user-id", c.UserID)
	}
	if c.Token != "" {
		req.Header.Set("user-token", ComputeDynamicToken(path, "", c.Token))
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]any{"code": 0}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		n := len(raw)
		if n > 120 {
			n = 120
		}
		return nil, fmt.Errorf("响应不是 JSON(%d 字节): %s", len(raw), string(raw[:n]))
	}
	return out, nil
}

func codeOf(m map[string]any) int {
	if v, ok := m["code"].(float64); ok {
		return int(v)
	}
	return -1
}

func (c *LobbyClient) SearchRoomByName(keyword, version string) (map[string]any, error) {
	if version == "" {
		version = "1.21.130"
	}
	m, err := c.Post("/online-lobby-room/query/search-by-name-v2", map[string]any{
		"length": 10, "version": version, "res_id": "", "keyword": keyword, "offset": 0,
	})
	if err != nil {
		return nil, err
	}
	if codeOf(m) != 0 {
		return nil, fmt.Errorf("搜房失败 code=%d %v", codeOf(m), m["message"])
	}
	ents, _ := m["entities"].([]any)
	if len(ents) == 0 {
		return nil, fmt.Errorf("没找到房间 %s", keyword)
	}
	room, _ := ents[0].(map[string]any)
	return room, nil
}

func (c *LobbyClient) ListRooms(keyword, version string, length int) ([]map[string]any, error) {
	if version == "" {
		version = "1.21.130"
	}
	if length <= 0 {
		length = 20
	}
	m, err := c.Post("/online-lobby-room/query/search-by-name-v2", map[string]any{
		"length": length, "version": version, "res_id": "", "keyword": keyword, "offset": 0,
	})
	if err != nil {
		return nil, err
	}
	if codeOf(m) != 0 {
		return nil, fmt.Errorf("搜房失败 code=%d %v", codeOf(m), m["message"])
	}
	ents, _ := m["entities"].([]any)
	out := make([]map[string]any, 0, len(ents))
	for _, e := range ents {
		if r, ok := e.(map[string]any); ok {
			out = append(out, r)
		}
	}
	return out, nil
}

func (c *LobbyClient) GetRoom(roomID string) (map[string]any, error) {
	m, err := c.Post("/online-lobby-room/get", map[string]any{"room_id": roomID})
	if err != nil {
		return nil, err
	}
	if codeOf(m) != 0 {
		return nil, fmt.Errorf("拿房间失败 code=%d", codeOf(m))
	}
	ent, _ := m["entity"].(map[string]any)
	return ent, nil
}

func (c *LobbyClient) MyRoom() (map[string]any, error) {
	m, err := c.Post("/online-lobby-room/query/user-room", map[string]any{})
	if err != nil {
		return nil, err
	}
	ent, _ := m["entity"].(map[string]any)
	return ent, nil
}

func (c *LobbyClient) ListMembers(roomID string) ([]map[string]any, error) {
	m, err := c.Post("/online-lobby-member/query/list-by-room-id", map[string]any{"room_id": roomID})
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	raw, _ := m["entities"].([]any)
	for _, e := range raw {
		if d, ok := e.(map[string]any); ok {
			out = append(out, d)
		}
	}
	return out, nil
}

func (c *LobbyClient) GetSloganConfig() (map[string]any, error) {
	m, err := c.Post("/online-lobby-room/query/get-slogan-config", map[string]any{})
	if err != nil {
		return nil, err
	}
	if codeOf(m) != 0 {
		return nil, fmt.Errorf("拉口号配置失败 code=%d", codeOf(m))
	}
	ent, _ := m["entity"].(map[string]any)
	return ent, nil
}

func (c *LobbyClient) EnterRoom(roomID, password string) error {
	m, err := c.Post("/online-lobby-room-enter/", map[string]any{
		"password":               password,
		"room_id":                roomID,
		"lobby_manifest_version": "",
		"check_visibilily":       1,
	})
	if err != nil {
		return err
	}
	if code := codeOf(m); code != 0 {
		if code == 359 {
			return fmt.Errorf("进房失败(359)：房间满 / 不允许进入")
		}
		return fmt.Errorf("进房失败 code=%d %v", code, m["message"])
	}
	return nil
}

func (c *LobbyClient) UpdateRoom(roomID string, fields map[string]any) (map[string]any, error) {
	body := map[string]any{"room_id": roomID}
	for k, v := range fields {
		body[k] = v
	}
	m, err := c.Post("/online-lobby-room/update", body)
	if err != nil {
		return nil, err
	}
	if codeOf(m) != 0 {
		return nil, fmt.Errorf("改房间失败 code=%d %v", codeOf(m), m["message"])
	}
	ent, _ := m["entity"].(map[string]any)
	return ent, nil
}

func (c *LobbyClient) SetAllowSave(roomID string, allow bool) error {
	_, err := c.UpdateRoom(roomID, map[string]any{"allow_save": allow})
	return err
}

func (c *LobbyClient) SetVisibility(roomID string, vis int) error {
	_, err := c.UpdateRoom(roomID, map[string]any{"visibility": vis})
	return err
}

func (c *LobbyClient) SaveRoomSettings(roomID string, minLevel int, password, slogan string) error {
	_, err := c.UpdateRoom(roomID, map[string]any{
		"min_level": minLevel,
		"password":  password,
		"slogan":    slogan,
	})
	return err
}

func (c *LobbyClient) TransferOwner(roomID string, ownerID int64) error {
	_, err := c.UpdateRoom(roomID, map[string]any{"owner_id": ownerID})
	return err
}

func (c *LobbyClient) KickMember(roomID string, userID int64) error {
	m, err := c.Post("/online-lobby-member-kick", map[string]any{
		"user_id": userID, "room_id": roomID,
	})
	if err != nil {
		return err
	}
	if codeOf(m) != 0 {
		return fmt.Errorf("踢人失败 code=%d %v", codeOf(m), m["message"])
	}
	return nil
}

type CreateRoomOptions struct {
	ResID           string
	SaveID          any
	Slogan          string
	Password        string
	MinLevel        int
	MaxCount        int
	Version         string
	ManifestVersion string
}

const DefaultResID = "4674188872992911923"

const PEApiGateway = "https://g79apigatewayobt.minecraft.cn"

type ZoneItem struct {
	ResID     string `json:"res_id"`
	Name      string `json:"res_name"`
	ImageURL  string `json:"image_url"`
	OnlineNum int    `json:"online_num"`
	RoomNum   int    `json:"room_num"`
}

func (c *LobbyClient) ListZones() ([]ZoneItem, error) {
	body := map[string]any{
		"channel_id":         5,
		"length":             599,
		"filter_my_favorite": true,
		"first_type_id":      6,
		"order_flag":         1,
		"offset":             0,
		"asc_flag":           0,
		"filter_type":        []any{},
		"download_iids":      []any{},
	}

	c2 := *c
	c2.Base = PEApiGateway
	m, err := c2.Post("/pe-user-item-purchase", body)
	if err != nil {
		return nil, err
	}
	if code := codeOf(m); code != 0 {

		raw, _ := json.Marshal(m)
		if len(raw) > 400 {
			raw = raw[:400]
		}
		return nil, fmt.Errorf("拉专区列表失败 code=%d %v 原始=%s", code, m["message"], string(raw))
	}

	ents, _ := m["entities"].([]any)
	out := make([]ZoneItem, 0, len(ents))
	for _, e := range ents {
		d, ok := e.(map[string]any)
		if !ok {
			continue
		}
		iid := strOf(d["iid"])
		if iid == "" {
			iid = strOf(d["item_id"])
		}
		if iid == "" {
			continue
		}
		z := ZoneItem{
			ResID:    iid,
			Name:     strOf(d["res_name"]),
			ImageURL: strOf(d["image_url"]),
		}
		if z.ImageURL == "" {
			z.ImageURL = strOf(d["title_image_url"])
		}
		z.OnlineNum = intOf(d["online_num"])
		z.RoomNum = intOf(d["room_num"])
		if z.Name == "" {
			z.Name = "专区 " + iid
		}
		out = append(out, z)
	}
	return out, nil
}

type LobbyRes struct {
	ResID  string `json:"res_id"`
	Name   string `json:"res_name"`
	CurNum int    `json:"cur_num"`
}

func (c *LobbyClient) SearchLobbyRes(keyword string, length int) ([]LobbyRes, error) {
	if length <= 0 {
		length = 20
	}
	m, err := c.Post("/pe-item/query/search-lobby-res-by-keyword", map[string]any{
		"length":    length,
		"asc_flag":  1,
		"sort_type": 1,
		"keyword":   keyword,
		"offset":    0,
	})
	if err != nil {
		return nil, err
	}
	if code := codeOf(m); code != 0 {
		return nil, fmt.Errorf("搜地图失败 code=%d %v", code, m["message"])
	}
	ents, _ := m["entities"].([]any)
	out := make([]LobbyRes, 0, len(ents))
	for _, e := range ents {
		d, ok := e.(map[string]any)
		if !ok {
			continue
		}
		rid := strOf(d["res_id"])
		if rid == "" {
			continue
		}
		out = append(out, LobbyRes{
			ResID:  rid,
			Name:   strOf(d["res_name"]),
			CurNum: intOf(d["cur_num"]),
		})
	}
	return out, nil
}

func (c *LobbyClient) CheckGameBan() (banned bool, detail map[string]any, err error) {
	m, err := c.Post("/check-user-game-ban", map[string]any{"gameType": "LobbyGame"})
	if err != nil {
		return false, nil, err
	}
	if code := codeOf(m); code != 0 {

		return false, map[string]any{"code": code, "message": m["message"]}, nil
	}
	ent, _ := m["entity"].(map[string]any)
	if ent == nil {
		return false, nil, nil
	}
	return true, ent, nil
}

var leaveEndpoints = []string{
	"/online-lobby-room-enter/leave-room",
	"/online-lobby-room/leave",
	"/online-lobby-room-leave",
	"/online-lobby-room-quit",
	"/online-lobby-leave",
}

func (c *LobbyClient) LeaveRoomHard(roomID string) (string, error) {
	var lastErr error
	for _, ep := range leaveEndpoints {
		m, err := c.Post(ep, map[string]any{"room_id": roomID})
		if err != nil {
			lastErr = err
			continue
		}
		code := codeOf(m)

		if code == 0 || code == 52001 || code == 12004 {
			if code == 0 {
				return ep, nil
			}
			return ep, nil
		}
		lastErr = fmt.Errorf("%s code=%d %v", ep, code, m["message"])
	}
	return "", lastErr
}

func (c *LobbyClient) ListRoomsPaged(resID, version string, pageLen, maxPages int) ([]map[string]any, error) {
	if version == "" {
		version = "1.21.130"
	}
	if pageLen <= 0 {
		pageLen = 50
	}
	if maxPages <= 0 {
		maxPages = 20
	}
	c2 := *c
	c2.Base = MCGateway

	var all []map[string]any
	seen := map[string]bool{}
	for page := 0; page < maxPages; page++ {
		m, err := c2.Post("/online-lobby-room/query/list-room-by-res-id", map[string]any{
			"lobby_manifest_version": "",
			"res_id":                 resID,
			"version":                version,
			"with_friend":            true,
			"offset":                 page * pageLen,
			"length":                 pageLen,
		})
		if err != nil {
			if page == 0 {
				return nil, err
			}
			break
		}
		if code := codeOf(m); code != 0 {
			if page == 0 {
				return nil, fmt.Errorf("列房失败 code=%d %v", code, m["message"])
			}
			break
		}
		ents, _ := m["entities"].([]any)
		if len(ents) == 0 {
			break
		}
		newCount := 0
		for _, e := range ents {
			d, ok := e.(map[string]any)
			if !ok {
				continue
			}
			id := strOf(d["entity_id"])
			if id != "" {
				if seen[id] {
					continue
				}
				seen[id] = true
			}
			all = append(all, d)
			newCount++
		}
		if len(ents) < pageLen {
			break
		}
		if newCount == 0 {
			break
		}
	}
	return all, nil
}

func (c *LobbyClient) CreateRoomRetry(o CreateRoomOptions, tries int, gap time.Duration) (map[string]any, error) {
	if tries <= 0 {
		tries = 5
	}
	if gap <= 0 {
		gap = 2 * time.Second
	}
	var lastErr error
	for i := 1; i <= tries; i++ {
		m, err := c.CreateRoom(o)
		if err == nil {
			return m, nil
		}
		lastErr = err

		msg := err.Error()
		if !strings.Contains(msg, "12019") && !strings.Contains(msg, "12024") &&
			!strings.Contains(msg, "秒后可创建") && !strings.Contains(msg, "请稍等") {
			return nil, err
		}
		if i < tries {
			logf("[建房] 被限流（第 %d/%d 次），%v 后重试：%v", i, tries, gap, err)
			time.Sleep(gap)
		}
	}
	return nil, fmt.Errorf("建房重试 %d 次仍被限流: %w", tries, lastErr)
}

func (c *LobbyClient) WaitBackupVisible(slot int, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		bs, err := c.ListBackups()
		if err == nil {
			for _, b := range bs {
				if intOf(b["backup_id"]) == slot {
					return true
				}
			}
		}
		time.Sleep(2 * time.Second)
	}
	return false
}

func (c *LobbyClient) EnterByRoomNumber(roomNumber, password string, waitReady time.Duration) (string, *GameServer, error) {
	if roomNumber == "" {
		return "", nil, fmt.Errorf("房间号为空")
	}
	if waitReady <= 0 {
		waitReady = 150 * time.Second
	}

	logf("[进服] 按房间号 %s 搜房…", roomNumber)
	m, err := c.SearchRoomByName(roomNumber, "1.21.130")
	if err != nil {
		return "", nil, fmt.Errorf("搜房失败：%w", err)
	}

	eid := ""
	total := 0
	if ents, ok := m["entities"].([]any); ok {
		total = len(ents)
		for _, e := range ents {
			d, _ := e.(map[string]any)
			if d == nil {
				continue
			}
			if strOf(d["room_name"]) == roomNumber {
				eid = strOf(d["entity_id"])
				break
			}
		}
	}
	if eid == "" {

		if ent, ok := m["entity"].(map[string]any); ok && ent != nil {
			if strOf(ent["room_name"]) == roomNumber {
				eid = strOf(ent["entity_id"])
			}
		}
	}
	if eid == "" {
		if total > 0 {
			return "", nil, fmt.Errorf("没搜到房间 %s（搜到 %d 个模糊匹配但房间名都不等于它 —— 拒绝进别人的房间）", roomNumber, total)
		}
		return "", nil, fmt.Errorf("没搜到房间 %s（可能已关闭、是私密房间，或被 2311 限流）", roomNumber)
	}
	logf("[进服] 房间 %s -> 实体 %s", roomNumber, eid)

	if my, e := c.MyRoom(); e == nil && my != nil {
		if cur := strOf(my["room_id"]); cur != "" && cur != eid {
			logf("[进服] 先退掉旧房间 %s", cur)
			if _, e2 := c.LeaveRoomHard(cur); e2 != nil {
				logf("[进服] 退房失败（继续）：%v", e2)
			}
			time.Sleep(2 * time.Second)
		}
	}

	if my, e := c.MyRoom(); e == nil && my != nil {
		if strOf(my["room_id"]) != eid {
			logf("[进服] 进房…")
			if err := c.EnterRoom(eid, password); err != nil {
				return eid, nil, fmt.Errorf("进房失败：%w", err)
			}
		} else {
			logf("[进服] 已经在目标房间")
		}
	}

	if gs, err := c.GameEnter(); err == nil && gs != nil && gs.Host != "" {
		logf("[进服] 游戏已在运行，直接拿地址 %s:%d", gs.Host, gs.Port)
		return eid, gs, nil
	}

	logf("[进服] 发 game-control start")
	if err := c.GameControl("start"); err != nil {
		logf("[进服] 开局返回：%v（继续等地址）", err)
	}
	deadline := time.Now().Add(waitReady)
	lastLog := time.Now()
	for time.Now().Before(deadline) {
		gs, err := c.GameEnter()
		if err == nil && gs != nil && gs.Host != "" {
			logf("[进服] 拿到地址 %s:%d", gs.Host, gs.Port)
			return eid, gs, nil
		}
		if time.Since(lastLog) >= 15*time.Second {
			lastLog = time.Now()
			logf("[进服] 还在等服务器就绪…")
		}
		time.Sleep(3 * time.Second)
	}
	return eid, nil, fmt.Errorf("等了 %.0f 秒还没拿到地址", waitReady.Seconds())
}

type PlayerProfile struct {
	Uid       string `json:"id"`
	Nickname  string `json:"nickname"`
	Signature string `json:"signature"`
	HeadImage string `json:"headImage"`
	FrameID   string `json:"frame_id"`
	Level     int    `json:"level"`
	Exp       int    `json:"exp"`
	Online    bool   `json:"online"`
}

func (c *LobbyClient) Profiles(uids []string) ([]PlayerProfile, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	m, err := c.Post("/user-else-detail-many/", map[string]any{
		"is_need_recharge_benefit": 0,
		"uids":                     strings.Join(uids, ";"),
	})
	if err != nil {
		return nil, err
	}
	if code := codeOf(m); code != 0 {
		return nil, fmt.Errorf("查资料失败 code=%d %v", code, m["message"])
	}
	ents, _ := m["entities"].([]any)
	out := make([]PlayerProfile, 0, len(ents))
	for _, e := range ents {
		d, ok := e.(map[string]any)
		if !ok {
			continue
		}
		p := PlayerProfile{
			Uid:       strOf(d["id"]),
			Nickname:  strOf(d["nickname"]),
			Signature: strOf(d["signature"]),
			HeadImage: strOf(d["headImage"]),
			FrameID:   strOf(d["frame_id"]),
		}
		if pg, ok := d["pe_growth"].(map[string]any); ok {
			p.Level = intOf(pg["lv"])
			p.Exp = intOf(pg["exp"])
		}
		if os, ok := d["online_status"].(bool); ok {
			p.Online = os
		}
		out = append(out, p)
	}
	return out, nil
}

func (c *LobbyClient) RoomByTags(tags []int, uid string, total int) ([]map[string]any, error) {
	if total <= 0 {
		total = 30
	}
	if tags == nil {
		tags = []int{0}
	}
	c2 := *c
	c2.Base = TransferGateway
	m, err := c2.Post("/room-with-tags", map[string]any{
		"total":   total,
		"version": 48,
		"uid":     uidNumOrStr(uid),
		"tags":    tags,
	})
	if err != nil {
		return nil, err
	}
	if code := codeOf(m); code != 0 {
		return nil, fmt.Errorf("按标签列房失败 code=%d %v", code, m["message"])
	}
	return mapList(m["list"]), nil
}

func (c *LobbyClient) RoomRelated(uid string, total int) ([]map[string]any, error) {
	if total <= 0 {
		total = 60
	}
	c2 := *c
	c2.Base = TransferGateway
	m, err := c2.Post("/room-related", map[string]any{
		"total":   total,
		"version": 48,
		"uid":     uidNumOrStr(uid),
	})
	if err != nil {
		return nil, err
	}
	if code := codeOf(m); code != 0 {
		return nil, fmt.Errorf("列推荐房失败 code=%d %v", code, m["message"])
	}
	return mapList(m["list"]), nil
}

func (c *LobbyClient) RecentResList(length int) ([]string, error) {
	if length <= 0 {
		length = 10
	}
	m, err := c.Post("/online-lobby-room/query/recent-play-res-list", map[string]any{
		"length": length,
		"offset": 0,
	})
	if err != nil {
		return nil, err
	}
	if code := codeOf(m); code != 0 {
		return nil, fmt.Errorf("最近地图失败 code=%d %v", code, m["message"])
	}
	ents, _ := m["entities"].([]any)
	out := make([]string, 0, len(ents))
	for _, e := range ents {
		if s := strOf(e); s != "" {
			out = append(out, s)
		}
	}
	return out, nil
}

func mapList(v any) []map[string]any {
	list, _ := v.([]any)
	out := make([]map[string]any, 0, len(list))
	for _, e := range list {
		if d, ok := e.(map[string]any); ok {
			out = append(out, d)
		}
	}
	return out
}

func uidNumOrStr(uid string) any {
	if n, err := strconv.ParseInt(uid, 10, 64); err == nil {
		return n
	}
	return uid
}

func (c *LobbyClient) RoomMatch(resID, version string) (string, error) {
	if version == "" {
		version = "1.21.130"
	}
	m, err := c.Post("/online-lobby-room-match", map[string]any{
		"version":                version,
		"res_id":                 resID,
		"lobby_manifest_version": "",
	})
	if err != nil {
		return "", err
	}
	if code := codeOf(m); code != 0 {
		return "", fmt.Errorf("匹配房间失败 code=%d %v", code, m["message"])
	}
	ent, _ := m["entity"].(map[string]any)
	if ent == nil {
		return "", fmt.Errorf("匹配没返回 entity（可能这个地图暂时没房）")
	}
	rid := strOf(ent["room_id"])
	if rid == "" {
		return "", fmt.Errorf("匹配返回的 room_id 为空")
	}
	return rid, nil
}

func (c *LobbyClient) ListResByType(typeID, order, length int) ([]LobbyRes, error) {
	if length <= 0 {
		length = 20
	}
	m, err := c.Post("/online-lobby-room/query/list-res-id-by-type", map[string]any{
		"tag_id":      0,
		"length":      length,
		"with_friend": true,
		"offset":      0,
		"type":        typeID,
		"order":       order,
	})
	if err != nil {
		return nil, err
	}
	if code := codeOf(m); code != 0 {
		return nil, fmt.Errorf("列热门地图失败 code=%d %v", code, m["message"])
	}
	ents, _ := m["entities"].([]any)
	out := make([]LobbyRes, 0, len(ents))
	for _, e := range ents {
		d, ok := e.(map[string]any)
		if !ok {
			continue
		}
		rid := strOf(d["res_id"])
		if rid == "" {
			continue
		}
		out = append(out, LobbyRes{
			ResID:  rid,
			CurNum: intOf(d["cur_num"]),
		})
	}
	return out, nil
}

func (c *LobbyClient) ResNamesByIDs(ids []string) (map[string]string, error) {
	if len(ids) == 0 {
		return map[string]string{}, nil
	}
	m, err := c.Post("/pe-item/query/search-lobby-by-id-list", map[string]any{
		"res_id_list": ids,
	})
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	ents, _ := m["entities"].([]any)
	for _, e := range ents {
		d, ok := e.(map[string]any)
		if !ok {
			continue
		}
		id := strOf(d["res_id"])
		if id == "" {
			id = strOf(d["item_id"])
		}
		if id == "" {
			continue
		}
		out[id] = strOf(d["res_name"])
	}
	return out, nil
}

func (c *LobbyClient) RoomByName(name string, uid string) ([]map[string]any, error) {
	if name == "" {
		return nil, fmt.Errorf("房间名为空")
	}
	c2 := *c
	c2.Base = TransferGateway
	var uidVal any = uid
	if n, err := strconv.ParseInt(uid, 10, 64); err == nil {
		uidVal = n
	}
	m, err := c2.Post("/room-with-name", map[string]any{
		"name": name,
		"uid":  uidVal,
	})
	if err != nil {
		return nil, err
	}
	if code := codeOf(m); code != 0 {
		return nil, fmt.Errorf("按名查房间失败 code=%d %v", code, m["message"])
	}
	list, _ := m["list"].([]any)
	out := make([]map[string]any, 0, len(list))
	for _, e := range list {
		if d, ok := e.(map[string]any); ok {
			out = append(out, d)
		}
	}
	return out, nil
}

func (c *LobbyClient) FindEnterableRoom(resID, version string) (string, error) {
	if resID == "" {
		return "", fmt.Errorf("res_id 为空")
	}

	if rid, err := c.RoomMatch(resID, version); err == nil && rid != "" {
		logf("[进服] 一键匹配到房间 %s", rid)
		return rid, nil
	} else if err != nil {
		logf("[进服] 一键匹配没成（%v），改用列房挑一个", err)
	}

	rooms, err := c.ListRoomsPaged(resID, version, 50, 4)
	if err != nil {
		return "", fmt.Errorf("列房失败：%w", err)
	}
	if len(rooms) == 0 {
		return "", fmt.Errorf("这个地图现在一个房间都没有")
	}

	var best string
	bestNum := -1
	for _, r := range rooms {
		rid := strOf(r["entity_id"])
		if rid == "" {
			continue
		}

		if intOf(r["password"]) != 0 {
			if best == "" {
				best = rid
			}
			continue
		}
		cur := intOf(r["cur_num"])
		max := intOf(r["max_count"])
		if max > 0 && cur >= max {
			continue
		}
		if cur > bestNum {
			bestNum = cur
			best = rid
		}
	}
	if best == "" {
		return "", fmt.Errorf("%d 个房间都不满足条件（全满或都要密码）", len(rooms))
	}
	logf("[进服] 从 %d 个房间里挑了 %s（%d 人）", len(rooms), best, bestNum)
	return best, nil
}

func (c *LobbyClient) EnterByResID(resID, version, password string, waitReady time.Duration) (string, *GameServer, error) {
	if waitReady <= 0 {
		waitReady = 150 * time.Second
	}

	rid, err := c.FindEnterableRoom(resID, version)
	if err != nil {
		return "", nil, fmt.Errorf("找房失败：%w", err)
	}

	if my, e := c.MyRoom(); e == nil {
		if cur := strOf(my["room_id"]); cur != "" {
			if cur == rid {
				logf("[进服] 已经在目标房间 %s", rid)
			} else {
				logf("[进服] 先退掉旧房间 %s", cur)
				if _, e2 := c.LeaveRoomHard(cur); e2 != nil {
					logf("[进服] 退房失败（继续）：%v", e2)
				}
				time.Sleep(2 * time.Second)
			}
		}
	}
	if my, e := c.MyRoom(); e == nil {
		if strOf(my["room_id"]) != rid {
			logf("[进服] 进房 %s …", rid)
			if err := c.EnterRoom(rid, password); err != nil {
				return rid, nil, fmt.Errorf("进房失败：%w", err)
			}
		}
	}

	if gs, err := c.GameEnter(); err == nil && gs != nil && gs.Host != "" {
		logf("[进服] 游戏已在运行，直接拿地址 %s:%d", gs.Host, gs.Port)
		return rid, gs, nil
	}

	logf("[进服] 发 game-control start")
	if err := c.GameControl("start"); err != nil {
		logf("[进服] 开局返回：%v（继续等地址）", err)
	}

	deadline := time.Now().Add(waitReady)
	lastLog := time.Now()
	for time.Now().Before(deadline) {
		gs, err := c.GameEnter()
		if err == nil && gs != nil && gs.Host != "" {
			logf("[进服] 拿到地址 %s:%d", gs.Host, gs.Port)
			return rid, gs, nil
		}
		if err != nil && !strings.Contains(err.Error(), "12031") {
			logf("[进服] game-enter：%v", err)
		}
		if time.Since(lastLog) >= 15*time.Second {
			lastLog = time.Now()
			logf("[进服] 还在等服务器就绪…（已等 %.0f 秒）", waitReady.Seconds()-time.Until(deadline).Seconds())
		}
		time.Sleep(3 * time.Second)
	}
	return rid, nil, fmt.Errorf("等了 %.0f 秒还没拿到地址（服务器可能没起来）", waitReady.Seconds())
}

func (c *LobbyClient) ListRoomsByResID(resID, version string, length int) ([]map[string]any, error) {
	if version == "" {
		version = "1.21.130"
	}
	if length <= 0 {
		length = 20
	}

	c2 := *c
	c2.Base = MCGateway

	m, err := c2.Post("/online-lobby-room/query/list-room-by-res-id", map[string]any{
		"lobby_manifest_version": "",
		"res_id":                 resID,
		"version":                version,
		"with_friend":            true,
		"offset":                 0,
		"length":                 length,
	})
	if err != nil {
		return nil, err
	}
	if code := codeOf(m); code != 0 {
		return nil, fmt.Errorf("按专区列房间失败 code=%d %v", code, m["message"])
	}
	ents, _ := m["entities"].([]any)
	out := make([]map[string]any, 0, len(ents))
	for _, e := range ents {
		if d, ok := e.(map[string]any); ok {
			out = append(out, d)
		}
	}
	return out, nil
}

func needsAuth(code int) bool { return code == 10 || code == 22 }

func strOf(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%.0f", t)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

func (c *LobbyClient) CreateRoom(o CreateRoomOptions) (map[string]any, error) {
	if o.ResID == "" {
		o.ResID = DefaultResID
	}
	if o.Slogan == "" {
		o.Slogan = "来和我一起玩吧！"
	}
	if o.MaxCount <= 0 {
		o.MaxCount = 10
	}
	if o.Version == "" {
		o.Version = "1.21.130"
	}

	body := map[string]any{
		"min_level": o.MinLevel,
		"slogan":    o.Slogan,
		"max_count": o.MaxCount,
		"version":   o.Version,
		"password":  o.Password,
		"res_id":    o.ResID,
	}
	if o.SaveID != "" {
		body["save_id"] = o.SaveID
	}
	if o.ManifestVersion != "" {
		body["lobby_manifest_version"] = o.ManifestVersion
	}
	m, err := c.Post("/online-lobby-room", body)
	if err != nil {
		return nil, err
	}
	if codeOf(m) != 0 {
		return nil, fmt.Errorf("开服失败 code=%d %v", codeOf(m), m["message"])
	}
	ent, _ := m["entity"].(map[string]any)
	return ent, nil
}

func (c *LobbyClient) CreateBackup(backupID int, name string) (map[string]any, error) {
	m, err := c.Post("/online-lobby-backup/create", map[string]any{
		"backup_id": backupID, "name": name,
	})
	if err != nil {
		return nil, err
	}
	if codeOf(m) != 0 {
		return nil, fmt.Errorf("保存地图失败 code=%d %v", codeOf(m), m["message"])
	}
	ent, _ := m["entity"].(map[string]any)
	return ent, nil
}

func (c *LobbyClient) ListBackups() ([]map[string]any, error) {

	m, err := c.Get("/online-lobby-backup/query/list-by-user")
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	raw, _ := m["entities"].([]any)
	for _, e := range raw {
		if d, ok := e.(map[string]any); ok {
			out = append(out, d)
		}
	}
	return out, nil
}

func (c *LobbyClient) LeaveRoom(roomID string) error {
	m, err := c.Post("/online-lobby-room-enter/leave-room", map[string]any{
		"room_id": roomID, "team_quit": false,
	})
	if err != nil {
		return err
	}
	if codeOf(m) != 0 {
		return fmt.Errorf("退房 code=%d %v", codeOf(m), m["message"])
	}
	return nil
}

func (c *LobbyClient) GameControl(op string) error {
	m, err := c.Post("/online-lobby-game-control", map[string]any{"op": op})
	if err != nil {
		return err
	}
	if codeOf(m) != 0 {
		return fmt.Errorf("game-control(%s) 失败 code=%d %v", op, codeOf(m), m["message"])
	}
	return nil
}

type GameServer struct {
	Host     string
	Port     int
	ISP      bool
	CMCC     string
	CTCC     string
	CUCC     string
	CMCCPort int
	CTCCPort int
	CUCCPort int
}

func (c *LobbyClient) GameEnter() (*GameServer, error) {
	m, err := c.Post("/online-lobby-game-enter", map[string]any{})
	if err != nil {
		return nil, err
	}
	if codeOf(m) != 0 {
		return nil, fmt.Errorf("game-enter 失败 code=%d %v", codeOf(m), m["message"])
	}
	ent, _ := m["entity"].(map[string]any)
	if ent == nil {
		return nil, fmt.Errorf("game-enter 没返回 entity")
	}
	gs := &GameServer{}
	gs.Host, _ = ent["server_host"].(string)
	gs.Port = intOf(ent["server_port"])
	gs.ISP, _ = ent["isp_enable"].(bool)
	gs.CMCC, _ = ent["cmcc_server_host"].(string)
	gs.CTCC, _ = ent["ctcc_server_host"].(string)
	gs.CUCC, _ = ent["cucc_server_host"].(string)
	gs.CMCCPort = intOf(ent["cmcc_server_port"])
	gs.CTCCPort = intOf(ent["ctcc_server_port"])
	gs.CUCCPort = intOf(ent["cucc_server_port"])
	return gs, nil
}

func intOf(v any) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}
