# cairn

Minecraft **Bedrock（基岩版）协议**登录实现，Go 编写，纯标准库、无第三方依赖。

覆盖从「账号登录态」到「进入游戏会话」的完整链路，并内置会话保活。

| 项 | 值 |
|---|---|
| Bedrock 协议版本 | **898** |
| 对应游戏版本 | `1.21.130` |
| RakNet 协议版本 | 8 |
| Go | 1.24+ |
| 依赖 | 仅标准库 |

---

## 链路

```
sauth / 登录态
   │
   ├─① 渠道认证        POST  <认证网关>         （登录请求加密 + 签名校验）
   │                   → { entity_id, token(16), sead(256) }
   │
   ├─② 取证书链        POST  <认证网关>/authentication-v2
   │                   → chain(JWT×N) → 用本机 EC 私钥重签（AuthenticationType=0）
   │
   └─③ 游戏服          UDP RakNet 握手
                       → RequestNetworkSettings(193) + 4B BE 协议版本
                       ← NetworkSettings(143) { threshold, algorithm }
                       → Login(1) { chain, client_data JWT }
                       ← ServerToClientHandshake(3) → ClientToServerHandshake(4)
                       ← PlayStatus(2) { 0 | 3 }
```

对应代码：

| 步骤 | 文件 |
|---|---|
| ① | `internal/mc/cookielogin.go`（`PeLoginWithSauth`）、`g79v12.go`、`x19.go` |
| ② | `internal/mc/auth.go`（`FetchAuthChain`）、`chain.go`（`ReissueChain` / `BuildLocalChain`） |
| ③ | `internal/mc/session.go`（`Start`）、`packet.go`、`internal/raknet/` |
| ③ 加密 | `internal/mc/crypto.go`（AES-256-CTR 会话）、`ecconv.go`（ECDH P-384） |
| ③ 登录包 | `internal/mc/skin.go`（client_data JWT） |

---

## 快速开始

```bash
go build -o cairn ./cmd/cairn

# cairn
./cairn -sauth '<sauth_json>'

# cairn
./cairn -token <t> -uid <u> -nick <昵称> -sid '<实体ID>:LobbyGame' -chain

# cairn
./cairn -sauth '<sauth_json>' -sid '<实体ID>:LobbyGame' \
          -host 1.2.3.4 -port 19132 \
          -keepalive 45 -heartbeat 50 -auto-reenter
```

### 保活参数

| 参数 | 默认 | 作用 |
|---|---|---|
| `-keepalive` | `45` | 账号保活间隔（秒），`0` 关闭 |
| `-heartbeat` | `0` | 游戏心跳间隔（毫秒），基岩版常用 `50`（20Hz） |
| `-auto-reenter` | 关 | 掉出房间时自动重进（用 `-sid` 的实体 ID） |
| `-duration` | `0` | 运行多少秒后退出，`0` = 常驻 |

保活分三层，互相独立，详见 [`doc/keepalive.md`](doc/keepalive.md)：

1. **账号保活**（HTTP）：定时确认账号仍在房间，掉出可自动重进；
2. **游戏心跳**（协议包）：定时发 `PlayerAuthInput`，不发会被判假死；
3. **令牌续期**：撞到 `code=22` 时用 sauth 重新登录换新令牌。

---

## 适用范围

本项目从 **`sauth_json`（账号登录态）** 开始，不包含账号密码 / 扫码登录。

- 认证与取链部分按本项目**实测的渠道**实现（网关地址、加密方式、签名字段都在对应文件里）；
- **不支持 Microsoft / Xbox 账号体系**，也没有走 Xbox Live 认证；
- 不同平台/渠道的认证流程不同，移植到别处需要替换 `cookielogin.go` 与 `auth.go` 里的认证实现。

`sauth_json` 属于账号凭据，不要提交进仓库、不要贴到公开渠道。

### client_data（皮肤 JWT）模板

`internal/mc/skin_data.json` 是**已脱敏的占位模板**：设备名 / 设备型号 / 皮肤贴图 /
随机 ID 都是合成的，不含任何真实设备指纹。签名前代码会改写其中
`ThirdPartyName` / `DeviceId` / `ClientRandomId` 三个字段。

要真正连服，请把它替换成**你自己抓的** client_data（可直接在运行目录放一份同名文件覆盖内嵌模板）。
沿用别人的设备指纹会得到 `reason=246`。

---

## 目录

```
cmd/cairn/          CLI 示例（三步登录 + 保活）
internal/mc/          协议层
  ├─ cookielogin.go   ① 渠道认证（sa_data / message / sign）
  ├─ auth.go          ② 取证书链
  ├─ chain.go         ② 证书链重签 / 本地自签
  ├─ session.go       ③ 登录主流程
  ├─ packet.go        ③ 包 ID / 协议版本 / Login 包
  ├─ crypto.go        ③ 会话加密（AES-256-CTR + SHA-256 签名）
  ├─ g79v12.go        登录请求加密
  ├─ x19.go           HTTP 载荷加密 / 动态令牌
  ├─ skin.go          client_data JWT 签名
  ├─ compress.go      压缩切换（算法 2 = 流式 deflate）
  ├─ heartbeat.go     保活（账号 + 游戏心跳）
  ├─ lobby.go         房间 HTTP 接口（建房/进房/取地址）
  └─ inventory.go / ntpackets.go / subchunk.go / netag.go …  游戏层包编解码
internal/raknet/      RakNet 握手 / 帧 / 分片 / ACK
internal/nbt/         网络 NBT（小端）
internal/blocks/      方块网络哈希 → 名字
internal/leveldb/     存档读写（与登录无关，可删）
doc/                  protocol.md · crypto.md · keepalive.md · pitfalls.md
```

只想读登录：看 `cookielogin.go` → `auth.go` → `chain.go` → `session.go` → `packet.go` 五个文件。

---

## 协议常量

```go
// internal/mc/packet.go
const ProtocolVersion = 898          // Bedrock 1.21.130

// internal/mc/auth.go
const EngineVersion = "3.10.5.301327"
const PatchVersion  = "3.10.6.301389"
```

---

## 已知限制

1. 取证书链接口**可能返回 401**。此时客户端会降级为本地自签链。
2. 自签链会被游戏服拒绝：`Disconnect reason=92`；换成服务端签发的链后若
   client_data 不匹配，会得到 `reason=246`。
3. `PlayStatus`：`0` 登录成功、`3` PlayerSpawn（正常进服）、`1/2` 版本不符、`4/5/6` 其它失败。
4. `reason=436`：指令下发超配额。

---

## 免责

- 协议常量（密钥表、签名常量、`sign` 变换）来自对公开客户端行为的观察，不含任何厂商源码。
- 本项目与任何游戏厂商无关，仅供协议研究与互操作性学习。
- 使用可能违反游戏服务条款并导致账号封禁，风险自负；请只在你自己的存档/环境中使用。

## License

MIT，见 [LICENSE](LICENSE)。
