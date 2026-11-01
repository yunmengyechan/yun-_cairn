package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cairn/internal/mc"
)

var (
	flagSauth       = flag.String("sauth", "", "sauth_json（从网易登录态里取，字符串原样）")
	flagToken       = flag.String("token", "", "16 位登录令牌（/pe-authentication 返回）")
	flagSead        = flag.String("sead", "", "长令牌 sead（PE 取链/登录用）")
	flagUID         = flag.String("uid", "", "账号 UserID")
	flagNick        = flag.String("nick", "", "显示名（取链请求要带，空会被 400）")
	flagSID         = flag.String("sid", "", "netease_sid，格式 <房间实体ID>:LobbyGame")
	flagHost        = flag.String("host", "", "游戏服地址")
	flagPort        = flag.Int("port", 0, "游戏服端口")
	flagChain       = flag.Bool("chain", false, "只取证书链（不连游戏服）")
	flagProto       = flag.Int("protocol", 0, "覆盖协议版本（默认用包内常量）")
	flagKeepAlive   = flag.Int("keepalive", 45, "账号保活间隔（秒），0=关")
	flagHeartbeat   = flag.Int("heartbeat", 0, "游戏心跳间隔（毫秒），0=关；网易常用 50（20Hz）")
	flagAutoReenter = flag.Bool("auto-reenter", false, "掉出房间时自动重进（需要 -sid）")
	flagDuration    = flag.Int("duration", 0, "跑多久后退出（秒），0=一直跑")
)

func main() {
	flag.Parse()

	fmt.Printf("=== PE 登录演示 ===\n")
	fmt.Printf("协议版本 %d  engine %s  patch %s\n\n",
		protoVersion(), mc.EngineVersion, mc.PatchVersion)

	mc.LogHook = func(format string, a ...any) {
		fmt.Printf("  · "+format+"\n", a...)
	}

	token, sead, uid := *flagToken, *flagSead, *flagUID
	var sauth string = *flagSauth

	if sauth != "" {
		fmt.Println("[1] POST /pe-authentication（账号登录）")
		r, err := mc.PeLoginWithSauth(sauth)
		if err != nil {
			fmt.Printf("    ✗ 失败: %v\n", err)
			os.Exit(1)
		}
		token, sead, uid = r.Token, r.Sead, r.UserID
		if *flagNick == "" {
			*flagNick = r.Nickname
		}
		fmt.Printf("    ✓ uid=%s nick=%s token=%s…(len=%d) sead_len=%d\n",
			r.UserID, r.Nickname, safe(r.Token), len(r.Token), len(r.Sead))
		fmt.Println()
	}
	if token == "" || uid == "" {
		fmt.Println("需要 -sauth，或同时给 -token 与 -uid")
		flag.Usage()
		return
	}

	if *flagChain {
		fmt.Println("[2] POST /authentication-v2（取证书链）")
		key, err := mc.NewECDSAKey()
		if err != nil {
			fmt.Printf("    ✗ 生成 EC 密钥失败: %v\n", err)
			os.Exit(1)
		}
		authTok := token
		if sead != "" {
			authTok = sead
		}
		res, err := mc.FetchAuthChain(authTok, uid, *flagNick, *flagSID, key)
		if err != nil {
			fmt.Printf("    ✗ 失败: %v\n", err)
			fmt.Println("    （失败时客户端会降级为本地自签链，但那会被游戏服以 reason=92 拒绝）")
			return
		}
		fmt.Printf("    ✓ 链长度 %d\n", len(res.Chain))
		os.WriteFile("chain.json", []byte(res.Chain), 0o644)
		fmt.Println("    → 已写 chain.json")
		return
	}

	if *flagHost == "" || *flagPort == 0 {
		fmt.Println("需要 -host / -port 才能继续（或用 -chain 只取链）")
		return
	}
	if *flagProto > 0 {
		mc.SetProtocolVersion(uint32(*flagProto))
	}

	lobby := mc.NewLobbyClient("", uid, token)
	if sauth != "" {
		lobby.OnAuthFail = func() bool {
			fmt.Println("  · 令牌失效（code=22），用 sauth 续期…")
			r, err := mc.PeLoginWithSauth(sauth)
			if err != nil {
				fmt.Printf("  · 续期失败: %v\n", err)
				return false
			}
			token, sead, uid = r.Token, r.Sead, r.UserID
			lobby.UserID, lobby.Token = r.UserID, r.Token
			fmt.Printf("  · 续期成功：%s…\n", safe(r.Token))
			return true
		}
	}

	fmt.Printf("[3] 连接 %s:%d（sid=%s）\n", *flagHost, *flagPort, *flagSID)
	s := mc.NewSession(mc.SessionConfig{
		Host:              *flagHost,
		Port:              *flagPort,
		UserID:            uid,
		Token:             token,
		Sead:              sead,
		DisplayName:       *flagNick,
		NeteaseSID:        *flagSID,
		Lobby:             lobby,
		ReenterRoomID:     roomIDFromSID(*flagSID),
		AccountKeepAliveS: *flagKeepAlive,
		GameHeartbeatMs:   *flagHeartbeat,
	})
	s.OnLog = func(m string) { fmt.Printf("  · %s\n", m) }

	if err := s.Start(); err != nil {
		fmt.Printf("    ✗ 登录失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("    ✓ 登录成功")

	stop := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Println("\n收到退出信号，正在停止…")
		close(stop)
	}()

	if *flagKeepAlive > 0 {
		go s.AccountKeepAlive(stop, time.Duration(*flagKeepAlive)*time.Second,
			*flagAutoReenter, func(r mc.AccountKeepaliveResult) {
				if r.Err != nil {
					fmt.Printf("  · 账号保活失败：%v\n", r.Err)
				} else if r.Reentered {
					fmt.Printf("  · 账号保活：已自动重进房间 %s\n", r.RoomID)
				}
			})
	}
	if *flagHeartbeat > 0 {
		s.StartGameHeartbeat(*flagHeartbeat)
		fmt.Printf("  · 游戏心跳已开启（%d ms）\n", *flagHeartbeat)
	}

	if *flagDuration > 0 {
		select {
		case <-stop:
		case <-time.After(time.Duration(*flagDuration) * time.Second):
		}
	} else {
		<-stop
	}
	s.Close()
	fmt.Println("已退出")
}

func protoVersion() int {
	if *flagProto > 0 {
		return *flagProto
	}
	return int(mc.ProtocolVersion)
}

func safe(s string) string {
	if len(s) <= 6 {
		return s
	}
	return s[:6]
}

func roomIDFromSID(sid string) string {
	if i := strings.Index(sid, ":"); i > 0 {
		return sid[:i]
	}
	return ""
}
