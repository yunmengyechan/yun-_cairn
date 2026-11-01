package mc

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"cairn/internal/raknet"
)

type SessionConfig struct {
	Host        string
	Port        int
	UserID      string
	Token       string
	DisplayName string
	NeteaseSID  string

	Sead string

	AccountKeepAliveS int

	GameHeartbeatMs int

	Lobby *LobbyClient

	ReenterRoomID string

	KeepAliveS int

	Candidates []string
}

type Session struct {
	cfg SessionConfig

	rk  *raknet.Client
	key *ecdsa.PrivateKey

	enc       *GameEncryption
	encrypted bool

	compressed           bool
	zlib                 *ZlibStreamCompressor
	compressionAlgorithm int
	compressionThreshold int

	loginSuccess bool
	playerID     int64

	mu      sync.Mutex
	pending [][]byte
	OnLog   func(string)

	OnEvent func(Event)
	tracker *PlayerTracker

	containerMu       sync.Mutex
	containerOpenedAt time.Time
	containerWindow   int8
	containerType     int8

	authMu         sync.Mutex
	auth           AuthState
	authInputOn    bool
	authInputEvery time.Duration
	chunkRadius    int
	closed         bool

	decryptFail      int
	decryptFailTotal int
	cryptoDbg        int

	uhMu       sync.Mutex
	unknownPkt map[byte]uint64
	unknownRpc map[string]uint64
}

func NewSession(cfg SessionConfig) *Session {
	if cfg.KeepAliveS <= 0 {
		cfg.KeepAliveS = 45
	}

	if cfg.AccountKeepAliveS <= 0 {
		cfg.AccountKeepAliveS = 45
	}
	return &Session{
		cfg:     cfg,
		tracker: NewPlayerTracker(),

		auth: newSessionAuthState(),
	}
}

func (s *Session) log(f string, a ...any) {
	if s.OnLog != nil {
		s.OnLog(fmt.Sprintf(f, a...))
	}
}

func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func (s *Session) Start() error {

	var k *ecdsa.PrivateKey
	if b, err := os.ReadFile("ec_key.txt"); err == nil {
		if kk, err := LoadECDSAKeyHex(strings.TrimSpace(string(b))); err == nil {
			k = kk
			s.log("★ 使用 ec_key.txt 里的私钥（与证书链配对）")
		}
	}
	if k == nil {
		kk, err := NewECDSAKey()
		if err != nil {
			return fmt.Errorf("生成 EC 密钥失败: %w", err)
		}
		k = kk
		_ = os.WriteFile("ec_key.txt", []byte(fmt.Sprintf("%096x", k.D)), 0644)
	}
	s.key = k

	chain := ""
	if os.Getenv("PH_USE_LOCAL_CHAIN") != "" {
		if b, err := os.ReadFile("chain.json"); err == nil && len(b) > 0 {
			chain = string(b)
			s.log("使用本地 chain.json（PH_USE_LOCAL_CHAIN）")
		}
	}
	if chain == "" {

		authTok := s.cfg.Token
		authKind := "短token"
		if s.cfg.Sead != "" {
			authTok = s.cfg.Sead
			authKind = "sead"
		}
		s.log("正在取证书链…（用 %s，len=%d）", authKind, len(authTok))
		s.log("[取链参数] token=%s…(len=%d) uid=%s sid=%s nick=%s",
			safePrefix(authTok, 6), len(authTok), s.cfg.UserID, s.cfg.NeteaseSID, s.cfg.DisplayName)
		res, err := FetchAuthChain(authTok, s.cfg.UserID, s.cfg.DisplayName, s.cfg.NeteaseSID, k)
		if err != nil {
			s.log("取链失败(%v)，降级用本地自签链", err)
			c, e2 := BuildLocalChain(s.cfg.DisplayName, s.cfg.UserID, s.cfg.NeteaseSID, k)
			if e2 != nil {
				return fmt.Errorf("取链失败且自签失败: %v / %v", err, e2)
			}
			chain = c
		} else {
			chain = res.Chain
			if res.Key != nil {
				s.key = res.Key
				k = res.Key
			}
		}
	}
	s.log("证书链长度 %d", len(chain))

	cands := s.candidates()
	var lastErr error
	connected := false
	for i, c := range cands {
		rk := raknet.New()
		rk.OnLog = func(m string) { s.log("%s", m) }
		if i > 0 {
			s.log("切换候选地址 %d/%d：%s", i+1, len(cands), c.label())
		} else {
			s.log("RakNet 连接 %s …", c.label())
		}
		if _, err := rk.Handshake(c.host, c.port, 10*time.Second); err != nil {
			lastErr = err
			s.log("× %s 握手失败：%v", c.label(), err)
			tryCloseRk(rk)
			continue
		}
		s.rk = rk
		s.cfg.Host, s.cfg.Port = c.host, c.port
		connected = true
		s.log("RakNet 已连接（%s）", c.label())
		break
	}
	if !connected {
		return fmt.Errorf("所有候选地址都连不上（试了 %d 个）：%w", len(cands), lastErr)
	}

	s.log("发送 RequestNetworkSettings")
	if err := s.SendPacket(BuildRequestNetworkSettings()); err != nil {
		return err
	}
	nsRaw, err := s.recvGamePacket(6 * time.Second)
	if err != nil {
		return fmt.Errorf("等 NetworkSettings 失败: %w", err)
	}

	if len(nsRaw) > 0 && nsRaw[0] == IDPlayStatus {
		st, _ := ParsePlayStatus(nsRaw)
		s.log("⚠ RequestNetworkSettings 阶段就被回 PlayStatus=%d（我们发的协议版本 %d），原始包 %x",
			st, curProtocolVersion(), nsRaw)
		for i := 0; i < 8; i++ {
			p2, e := s.recvGamePacket(1 * time.Second)
			if e != nil {
				continue
			}
			s.log("  ↳ 后续包 0x%02x（%d 字节）：%x", p2[0], len(p2), p2[:min(96, len(p2))])
			if p2[0] == IDDisconnect {
				s.log("  ↳ 拒绝原因：%s", ParseDisconnect(p2))
				break
			}
		}
		return fmt.Errorf("服务端在拿网络设置前就拒绝：PlayStatus=%d（我们发的协议版本 %d）", st, ProtocolVersion)
	}
	ns, err := ParseNetworkSettings(nsRaw)
	if err != nil {
		return err
	}
	s.log("NetworkSettings: threshold=%d algorithm=%d", ns.CompressionThreshold, ns.CompressionAlgorithm)
	s.compressionThreshold = ns.CompressionThreshold
	s.compressionAlgorithm = ns.CompressionAlgorithm
	s.compressed = true
	if ns.CompressionAlgorithm == 2 {
		z, err := NewZlibStream()
		if err != nil {
			return fmt.Errorf("初始化流式压缩失败: %w", err)
		}
		s.zlib = z
		s.log("启用流式 deflate（算法 2）")
	}

	skinRaw := LoadSkinData("", s.cfg.DisplayName)
	skinJWT := ""
	if skinRaw != "" {
		j, e := SignSkinData(skinRaw, k)
		if e == nil {
			skinJWT = j
			s.log("已用 Go 签皮肤 JWT（%d 字节）", len(j))
		}
	}
	if skinJWT == "" {
		s.log("没有 client_data，Login 无皮肤（服务器会拒 reason=246）")
	}
	loginPkt := BuildLogin(chain, skinJWT)
	s.log("发送 Login 包（%d 字节）", len(loginPkt))
	if err := s.SendPacket(loginPkt); err != nil {
		return err
	}

	var pkt []byte
	dl := time.Now().Add(25 * time.Second)
	for time.Now().Before(dl) {
		p2, e := s.recvGamePacket(2 * time.Second)
		if e != nil {
			continue
		}
		s.log("登录阶段收到 0x%02x（%d 字节）", p2[0], len(p2))
		if p2[0] == IDServerToClientHandshake || p2[0] == IDPlayStatus || p2[0] == IDDisconnect {
			pkt = p2
			break
		}
		s.pending = append(s.pending, p2)
	}
	if pkt == nil {
		return fmt.Errorf("登录超时：没等到握手/PlayStatus")
	}

	switch pkt[0] {
	case IDDisconnect:
		return fmt.Errorf("服务器拒绝（Disconnect）：%s", ParseDisconnect(pkt))
	case IDPlayStatus:
		st, _ := ParsePlayStatus(pkt)
		if st == 0 {
			s.loginSuccess = true
			s.log("登录成功（无加密模式）")
		} else {
			return fmt.Errorf("登录失败，PlayStatus=%d", st)
		}
	case IDServerToClientHandshake:
		if err := s.handleHandshake(pkt); err != nil {
			return fmt.Errorf("密钥交换失败: %w", err)
		}
		s.log("ECDH 密钥交换完成，AES-GCM 加密已启用")

		s.encrypted = true
		hs := BuildClientToServerHandshake(nil)
		s.log("ClientToServerHandshake 包体 %d 字节: %x  (encrypted=%v)", len(hs), hs, s.encrypted)
		if err := s.SendPacket(hs); err != nil {
			return err
		}
		s.log("已发 ClientToServerHandshake，开始等 PlayStatus")
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			p2, err := s.recvGamePacket(2 * time.Second)
			if err != nil {
				continue
			}
			s.log("等待阶段收到 0x%02x", p2[0])
			if p2[0] == IDPlayStatus {
				st, _ := ParsePlayStatus(p2)
				if st == 0 {
					s.loginSuccess = true
					s.log("★ 登录成功！")
				} else {
					return fmt.Errorf("登录失败 PlayStatus=%d", st)
				}
				break
			}
			if p2[0] == IDDisconnect {
				return fmt.Errorf("被断开：%s", ParseDisconnect(p2))
			}
			s.pending = append(s.pending, p2)
		}
		if !s.loginSuccess {
			return fmt.Errorf("等 PlayStatus 超时")
		}
	default:
		return fmt.Errorf("意外的响应 0x%02x: %x", pkt[0], pkt[:min(40, len(pkt))])
	}

	HumanDelay(700, 2600)
	if err := s.SendPacket(BuildClientCacheStatus(true)); err != nil {
		s.log("发 ClientCacheStatus 失败: %v", err)
	} else {
		s.log("已发 ClientCacheStatus")
	}
	return nil
}

func (s *Session) handleHandshake(pkt []byte) error {
	id, n := ReadVarint(pkt, 0)
	off := int(n)
	if id != IDServerToClientHandshake {
		return fmt.Errorf("不是握手包: 0x%02x", id)
	}
	jwtLen, n2 := ReadVarint(pkt, off)
	off += int(n2)
	if off+int(jwtLen) > len(pkt) {
		return fmt.Errorf("JWT 数据不完整")
	}
	jwt := string(pkt[off : off+int(jwtLen)])
	_ = os.WriteFile("s2c_handshake.jwt", []byte(jwt), 0644)
	s.log("★ 握手 JWT 已 dump（%d 字节）", len(jwt))
	pubDER, salt, err := ParseServerHandshakeJWT(jwt)
	if err != nil {
		return err
	}
	_ = os.WriteFile("s2c_pub.der", pubDER, 0644)
	_ = os.WriteFile("s2c_salt.bin", salt, 0644)
	s.log("★ 服务器公钥 DER=%d 字节  salt=%d 字节  %x", len(pubDER), len(salt), salt)
	pubRaw, err := DERtoRawPoint(pubDER)
	if err != nil {
		return err
	}
	kp, err := keyPairFromECDSA(s.key)
	if err != nil {
		return err
	}
	sessionKey, err := DeriveSessionKey(kp, pubRaw, salt)
	if err != nil {
		return err
	}
	_ = os.WriteFile("s2c_session_key.bin", sessionKey, 0644)
	s.log("★ session_key = %x", sessionKey)
	ge, err := NewGameEncryption(sessionKey)
	if err != nil {
		return err
	}

	ge.OnDebug = func(msg string) {
		if s.loginSuccess {
			return
		}

		s.cryptoDbg++
		if s.cryptoDbg <= 5 || s.cryptoDbg%100 == 0 {
			s.log("CRYPTO %s（累计 %d 条）", msg, s.cryptoDbg)
		}
	}
	s.enc = ge
	return nil
}

func ParseDisconnect(pkt []byte) string {
	if len(pkt) < 2 {
		return fmt.Sprintf("raw=%x", pkt)
	}
	reason, n := ReadVarint(pkt, 1)
	off := 1 + int(n)
	hide := 0
	if off < len(pkt) {
		hide = int(pkt[off])
	}
	return fmt.Sprintf("reason=%d hide=%d raw=%x", reason, hide, pkt)
}

func (s *Session) SendPacket(pkt []byte) error {
	dumpPacket("TX", pkt)
	batch := append(WriteVarint(uint32(len(pkt))), pkt...)
	return s.SendBatch(batch)
}

func (s *Session) SendPackets(pkts ...[]byte) error {
	var buf []byte
	for _, p := range pkts {
		dumpPacket("TX", p)
		buf = append(buf, WriteVarint(uint32(len(p)))...)
		buf = append(buf, p...)
	}
	return s.SendBatch(buf)
}

var dumpOn = strings.EqualFold(os.Getenv("PH_DUMP"), "1") ||
	strings.EqualFold(os.Getenv("PH_DUMP"), "true")

func dumpPacket(dir string, pkt []byte) {
	if !dumpOn || len(pkt) == 0 {
		return
	}

	id, _, _ := rdVarUInt(pkt, 0)
	name := NTIDByNumber[int(id)]
	if name == "" {
		name = "?"
	}

	limit := len(pkt)
	cut := ""
	if limit > 256 {
		limit = 256
		cut = fmt.Sprintf(" …(共 %d 字节)", len(pkt))
	}
	hex := make([]string, 0, limit)
	for _, b := range pkt[:limit] {
		hex = append(hex, fmt.Sprintf("%02x", b))
	}
	fmt.Printf("[%s] 0x%02x %-24s %dB%s\n     %s\n",
		dir, id, name, len(pkt), cut, strings.Join(hex, " "))
}

func (s *Session) SendBatch(batch []byte) error {
	if s.rk == nil {
		return errors.New("未连接")
	}
	if s.compressed {
		switch {
		case s.compressionAlgorithm == 2 && s.zlib != nil:
			out, err := s.zlib.Compress(batch)
			if err != nil {
				return err
			}
			batch = out
		case s.compressionAlgorithm == 0 && len(batch) >= s.compressionThreshold:
			c, err := ZlibCompress(batch)
			if err != nil {
				return err
			}
			batch = append([]byte{0x00}, c...)
		default:
			batch = append([]byte{0xff}, batch...)
		}
	}
	if s.encrypted && s.enc != nil {
		batch = s.enc.Encrypt(batch)
	}
	return s.rk.SendGame(batch)
}

func (s *Session) recvGamePacket(timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		raw, err := s.rk.RecvGame(200 * time.Millisecond)
		if err != nil {
			continue
		}
		body, err := s.decodeBatch(raw)
		if err != nil {

			if errors.Is(err, errPlainResend) {
				continue
			}
			s.log("解包失败: %v", err)
			continue
		}
		if len(body) == 0 {
			continue
		}
		return body, nil
	}
	return nil, fmt.Errorf("超时")
}

func (s *Session) decodeBatch(raw []byte) ([]byte, error) {
	data := raw
	if os.Getenv("PH_DEBUG") != "" {
		s.log("DBG raw=%d  %x", len(raw), raw[:min(48, len(raw))])
	}
	if s.encrypted && s.enc != nil {
		pt, err := s.enc.Decrypt(data)
		if err != nil {

			if s.looksLikePlainBatch(data) {
				s.decryptFailTotal++
				if s.decryptFailTotal <= 3 {
					s.log("忽略服务器重发的明文包（%d 字节，累计 %d 次）", len(data), s.decryptFailTotal)
				}
				return nil, errPlainResend
			}

			s.decryptFail++
			if s.decryptFail <= 3 || s.decryptFail%50 == 0 {
				s.log("解密失败（第 %d 次）: %v", s.decryptFail, err)
			}
			return nil, errPlainResend
		}
		data = pt
	}
	if s.compressed && len(data) > 0 {
		if s.zlib != nil {
			out, err := s.zlib.Decompress(data)
			if err != nil || out == nil {

				if len(data) > 0 && data[0] >= 0x01 && data[0] <= 0xcb {

				} else if err != nil {
					return nil, fmt.Errorf("流式解压: %w", err)
				} else {
					return nil, errPlainResend
				}
			} else {
				data = out
			}
		} else if len(data) > 0 && data[0] == 0x00 {
			out, err := ZlibDecompress(data[1:])
			if err != nil {
				return nil, err
			}
			data = out
		} else if len(data) > 0 && data[0] == 0xff {
			data = data[1:]
		}
	}

	ln, n := ReadVarint(data, 0)
	off := int(n)
	if off+int(ln) > len(data) || ln == 0 {
		return nil, errPlainResend
	}
	sub := data[off : off+int(ln)]
	if len(sub) > 0 && sub[0] == IDCommandOutput {
		s.log("← CommandOutput(%d 字节) %x", len(sub), sub[:min(40, len(sub))])
	} else if len(sub) > 0 && sub[0] == IDText {
		s.log("← Text(%d 字节)", len(sub))
	}
	return sub, nil
}

var errPlainResend = errors.New("跳过（重发的明文包）")

func (s *Session) looksLikePlainBatch(b []byte) bool {
	if len(b) < 2 {
		return false
	}

	if b[0] == 0x00 || b[0] == 0xff {
		return true
	}

	ln, n := ReadVarint(b, 0)
	if ln == 0 || int(n) >= len(b) {
		return false
	}

	rest := len(b) - int(n)
	if rest <= 0 {
		return false
	}
	if int(ln) != rest && int(ln) > rest {
		return false
	}
	id := b[int(n)]

	switch id {
	case 0x01,
		0x02,
		0x03,
		0x06,
		0x8f,
		0x09:
		return true
	}
	return false
}

const (
	SessionEngineVersion = "3.10.5.301327"
	SessionPatchVersion  = "3.10.6.301389"
)

func uuidString(u [16]byte) string {
	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		u[0], u[1], u[2], u[3], u[4], u[5], u[6], u[7],
		u[8], u[9], u[10], u[11], u[12], u[13], u[14], u[15])
}

func (s *Session) emitEvent(name string, data map[string]any) {
	if s.OnEvent == nil {
		return
	}
	if data == nil {
		data = map[string]any{}
	}
	s.OnEvent(Event{Name: name, Data: data})
}

func (s *Session) Close() {
	s.authMu.Lock()
	s.closed = true
	s.authInputOn = false
	s.authMu.Unlock()
	if s.rk != nil {
		s.rk.Close()
	}
}

func (s *Session) LoginSuccess() bool { return s.loginSuccess }

func (s *Session) SendSettingsCommand(txt string) error {
	if !s.loginSuccess {
		return errors.New("尚未登录")
	}
	return s.SendPacket(BuildSettingsCommand(txt, false))
}

func (s *Session) SendChat(text string) error {
	if !s.loginSuccess {
		return errors.New("尚未登录")
	}
	return s.SendPacket(BuildTextPacket(1, s.cfg.DisplayName, text))
}

func (s *Session) SendCommand(cmd string) error {
	if !s.loginSuccess {
		return errors.New("尚未登录")
	}
	cmd = trimLeadingSlash(cmd)
	if cmd == "" {
		return errors.New("指令为空")
	}
	return s.SendPacket(BuildCommandRequest(cmd, cmd))
}

func trimLeadingSlash(s string) string {
	s = strings.TrimSpace(s)
	for len(s) > 0 && s[0] == '/' {
		s = strings.TrimSpace(s[1:])
	}
	return s
}

func (s *Session) ServeLoop(ch chan struct{}) {

	s.mu.Lock()
	pend := s.pending
	s.pending = nil
	s.mu.Unlock()
	for _, p := range pend {
		s.handleGamePacket(p)
	}

	for {
		select {
		case <-ch:
			return
		default:
		}
		pkt, err := s.recvGamePacket(1 * time.Second)
		if err != nil {
			continue
		}
		if !s.handleGamePacket(pkt) {
			return
		}
	}
}

func (s *Session) handleGamePacket(pkt []byte) bool {
	if len(pkt) == 0 {
		return true
	}
	dumpPacket("RX", pkt)
	switch pkt[0] {
	case IDDisconnect:
		s.log("被服务器断开：%s", ParseDisconnect(pkt))
		return false
	case IDPlayStatus:
		st, _ := ParsePlayStatus(pkt)
		s.log("PlayStatus=%d", st)
	case IDResourcePacksInfo:
		s.log("收到 ResourcePacksInfo，回 ResourcePackResponse")
		_ = s.SendPacket(BuildResourcePackResponse(3))
	case IDStartGame:
		s.log("收到 StartGame（%d 字节）", len(pkt))
		for _, p := range BuildPostStartGameHandshake() {
			_ = s.SendPacket(p)
		}
		s.log("已发 PostStartGameHandshake")
	case IDCorrectPlayerMovePrediction:

		if mp, err := ParseCorrectPlayerMovePrediction(pkt); err != nil {
			s.log("⚠ 位置纠正包解析失败：%v", err)
		} else if ok, d, old, nw := s.ApplyMovePrediction(mp); ok {
			s.log("← 位置纠正 type=%d  位移 %.2f格  (%.1f,%.1f,%.1f) → (%.1f,%.1f,%.1f)  onGround=%v",
				mp.Type, d, old[0], old[1], old[2], nw[0], nw[1], nw[2], mp.OnGround)
		} else if mp != nil {
			s.log("← 位置纠正 type=%d（载具，忽略）", mp.Type)
		}
	case IDPyRpc:
		s.handlePyRpc(pkt)
	case IDPlayerList:

		if pl, err := ParsePlayerList(pkt); err != nil {
			s.log("⚠ PlayerList 解析失败：%v", err)
		} else if s.tracker != nil {
			d := s.tracker.Apply(pl)
			for _, e := range d.Joined {
				s.log("← ★ 玩家加入：%s（uniqueID=%d）", e.Username, e.UniqueID)
				s.emitEvent("player_join", map[string]any{
					"uuid": uuidString(e.UUID), "unique_id": e.UniqueID,
					"name": e.Username, "xuid": e.XUID, "count": d.Total})
			}
			for _, e := range d.Left {
				s.log("← ★ 玩家离开：%s", e.Username)
				s.emitEvent("player_leave", map[string]any{
					"uuid": uuidString(e.UUID), "name": e.Username, "count": d.Total})
			}
		}
	case IDText:
		if tm, err := ParseText(pkt); err == nil && tm.Message != "" {
			s.log("← 聊天 %s: %s", tm.Source, tm.Message)
			s.emitEvent("chat", map[string]any{
				"type": tm.Type, "source": tm.Source,
				"message": tm.Message, "sys": tm.SysMsg})
		}
		s.log("← Text(%d 字节)", len(pkt))
	case IDCommandOutput:
		s.log("← CommandOutput: %x", pkt[:min(60, len(pkt))])
	case IDNetworkSettings:
		if ns, err := ParseNetworkSettings(pkt); err == nil {
			s.compressionThreshold = ns.CompressionThreshold
			s.compressionAlgorithm = ns.CompressionAlgorithm
			s.log("NetworkSettings 更新: threshold=%d algorithm=%d", ns.CompressionThreshold, ns.CompressionAlgorithm)
		}
	default:

		if s.handleExtraEvent(pkt[0], pkt) {
			break
		}

		n := s.countUnknownPkt(pkt[0])
		if n <= 3 || (os.Getenv("PH_DEBUG") != "" && n%50 == 0) {
			s.log("← 未处理包 0x%02x（%d 字节，累计 %d 次）", pkt[0], len(pkt), n)
		}
	}
	return true
}

func (s *Session) handlePyRpc(pkt []byte) {
	ctx := RpcContext{
		UID:           s.cfg.UserID,
		PlayerID:      s.playerID,
		EngineVersion: SessionEngineVersion,
		PatchVersion:  SessionPatchVersion,
	}
	replies, name, err := HandlePyRpc(pkt, ctx)
	if err != nil {
		s.log("PyRpc 处理失败: %v", err)
		return
	}
	if name != "" && len(replies) == 0 {

		n := s.countUnknownRpc(name)
		if n <= 3 {
			s.log("PyRpc【未处理】%s（累计 %d 次）—— 没答可能会被判定异常", name, n)
		}
		return
	}
	if name != "" {
		s.log("PyRpc: %s（回 %d 个包）", name, len(replies))
	}
	for _, r := range replies {
		_ = s.SendPacket(r)
	}
}

func (s *Session) PassiveKeepAlive(ch chan struct{}) {
	interval := time.Duration(s.cfg.KeepAliveS) * time.Second
	if interval <= 0 {
		interval = 45 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ch:
			return
		case <-t.C:
			if !s.loginSuccess {
				continue
			}

			_ = s.SendPacket(BuildTickSync())
		}
	}
}

func (s *Session) countUnknownPkt(id byte) uint64 {
	s.uhMu.Lock()
	defer s.uhMu.Unlock()
	if s.unknownPkt == nil {
		s.unknownPkt = map[byte]uint64{}
	}
	s.unknownPkt[id]++
	return s.unknownPkt[id]
}

func (s *Session) countUnknownRpc(name string) uint64 {
	s.uhMu.Lock()
	defer s.uhMu.Unlock()
	if s.unknownRpc == nil {
		s.unknownRpc = map[string]uint64{}
	}
	s.unknownRpc[name]++
	return s.unknownRpc[name]
}

func (s *Session) UnhandledSummary() (uint64, uint64, string) {
	s.uhMu.Lock()
	defer s.uhMu.Unlock()
	var pktTotal, rpcTotal uint64
	var parts []string
	for id, n := range s.unknownPkt {
		pktTotal += n
		parts = append(parts, fmt.Sprintf("包0x%02x×%d", id, n))
	}
	for name, n := range s.unknownRpc {
		rpcTotal += n
		parts = append(parts, fmt.Sprintf("RPC[%s]×%d", name, n))
	}
	sort.Strings(parts)
	return pktTotal, rpcTotal, strings.Join(parts, " ")
}

func (s *Session) Stats() (sent, recv, sentB, recvB uint64) {
	if s.rk == nil {
		return 0, 0, 0, 0
	}
	return s.rk.Stats()
}

func (s *Session) Online() bool { return s.loginSuccess }
