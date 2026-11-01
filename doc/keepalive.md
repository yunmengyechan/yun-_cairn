# 保活（keep-alive）

登录成功只是开始。这类长连接有三处会「静默掉线」，要分别保：

| 层 | 掉线表现 | 保活手段 | 代码 |
|---|---|---|---|
| 账号（大厅） | 大厅里房间消失 / 变成不在房间，游戏里还能动一会儿 | 定时查 `/online-lobby-room/query/user-room`，掉出就重进 | `internal/mc/heartbeat.go` |
| 游戏（协议） | 服务端判假死，不报错地不再下发任何包 | 定时发 `PlayerAuthInput` | `internal/mc/heartbeat.go` |
| 令牌 | 大厅接口回 `code=22 帐号在另一处登录` | 用 `sauth` 重新 `/pe-authentication` 换新令牌 | `cmd/cairn/main.go` 的 `OnAuthFail` |

---

## 1. 账号保活（大厅 HTTP）

`Session.AccountKeepAlive(stop, interval, autoReenter, onResult)`

- 间隔：`-keepalive`（默认 45 秒）。**别调太小**，大厅接口有频率限制。
- 每次做的事：
  1. 调 `/online-lobby-room/query/user-room`；
  2. 拿到的 `room_id` 为空 → 说明已不在房间；
  3. 若开了 `autoReenter` 且有 `ReenterRoomID` → 调 `/online-lobby-room-enter/` 重进。
- **不发任何游戏包**，所以和游戏心跳互不干扰。

> ★ 坑：账号在服务端还挂着旧房间会话时，直接进房即使 `code=0`，
> 游戏服 Login 阶段也会回 `reason=92`。重进前最好先 `/leave-room` 再进。

## 2. 游戏心跳（PlayerAuthInput）

`Session.StartGameHeartbeat(intervalMs)`

- 服务端期望 **20Hz**，即 `-heartbeat 50`。间隔太大服务端会认为你假死。
- 包内容是 `PlayerAuthInput`：位置 / 朝向 / 输入位 / 客户端 tick。
- 这是**唯一的存活证明**：只收不发，服务端不会主动踢你，但也不会再理你。

另外还有一层保护在会话里：**看门狗**（`session.go`）。10 秒采样一次收包计数，
连续 30 次（5 分钟）一个包都没收到 → 判定假死，主动断开重连。

## 3. 令牌续期

该渠道是**单会话**：同一个账号在别处登录，旧令牌立刻失效。

- 触发：大厅接口返回 `code=22`；
- 处理：`LobbyClient.OnAuthFail` 回调里用 `sauth` 重新走 `/pe-authentication`，
  拿到新 `token` / `sead` 后替换并重试这一次请求；
- 注意：`/pe-authentication` 本身也有频率限制，别每次请求都刷 —— 只在撞到 22 时刷。

## 4. 断线重连的节奏

如果要长期挂，重连必须**带抖动**，否则连接曲线非常像脚本：

```
第 N 次重连等待 = N² × 4 秒 × (0.7 ~ 1.3 随机)，最少 4 秒
```

（`daemon.go` 里的实现，OSS 版本保留在 `cmd/cairn` 的注释里，按需自行添加。）

---

## 常见现象对照

| 现象 | 原因 |
|---|---|
| 游戏里不报错，但所有动作都没反应 | 没发 `PlayerAuthInput`，服务端认为你不在输入流里 |
| 大厅查房间一会儿有、一会儿没有 | 账号保活间隔太短，被 `code=2311 查询过于频繁` 限流 |
| 一直 `code=22` | 多个进程共用同一账号令牌，互相顶号 |
| 能建房但连不上（`reason=92`） | 服务端还挂着旧房间会话，先 `/leave-room` 等 30 秒再进 |
