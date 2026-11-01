# 登录链路详解

> 目标：基岩版 1.21.130（Bedrock 协议 898），认证侧为渠道适配实现。
> 每一步都标注了对应代码位置，方便对照。

---

## 0. 总览

```
sauth_json
   │
   ├─① POST https://g79obtcore.minecraft.cn:8443/pe-authentication
   │      body  = { sa_data, sauth_json, engine_version, patch_version, message, seed, sign }
   │      cipher= G79V12
   │      →    { entity: { entity_id, token(16), sead(256), ... } }
   │
   ├─② POST https://g79authobt.minecraft.cn/authentication-v2
   │      body  = { bit:"64", clientKey(160), displayName, engineVersion,
   │               netease_sid:"<实体ID>:LobbyGame", os_name:"android",
   │               patchVersion, uid }
   │      cipher= AES-CBC + 随机尾字节（"HttpEncrypt"）
   │      header= user-token: HEX(动态令牌), user-id
   │      →    { chain: [jwt0, jwt1, ...] }
   │      然后用本机 EC 私钥重签： [新 CA JWT, chain[0], chain[1]]，AuthenticationType=0
   │
   └─③ UDP 游戏服
          OpenConnectionRequest1/2 → ConnectionRequest → NewIncomingConnection
          → RequestNetworkSettings(193) + 4 字节 BE 协议版本
          ← NetworkSettings(143) { threshold, algorithm }
          → Login(1) { chain, client_data JWT }
          ← ServerToClientHandshake(3) → ClientToServerHandshake(4)
          ← PlayStatus(2) { 0=登录成功, 3=PlayerSpawn }
```

---

## ① /pe-authentication

`internal/mc/cookielogin.go` `PeLoginWithSauth(sauth string)`。

请求体字段：

| 字段 | 说明 |
|---|---|
| `sa_data` | 设备指纹 JSON 字符串。**`os_name` 必须是 `android`**，填 `windows` 一律 `code=2001 版本过低` |
| `sauth_json` | 登录态原样透传 |
| `engine_version` / `patch_version` | `3.10.5.301327` / `3.10.6.301389` |
| `message` | `engine + k1 + patch + k2 + seed`（k1/k2 见 `cookielogin.go-306`） |
| `seed` | GUID |
| `sign` | `base64(PeSignCompute(message, 2, 8))` |

> ★ 两个最容易踩的点：
> 1. `sa_data.os_name` 必须是 `android`；
> 2. `sign` 的偏移参数是 **`(2, 8)`**，不是 `(4, 7)`（用真机抓包对验算得出）。

响应里有两个令牌，别混：

- `token`（16 位）：大厅 HTTP 接口用；
- `sead`（256 位）：**PE 模式取证书链用**（`launcher.py:224`：`auth_token = sead if is_pe`）。

---

## ② /authentication-v2（取证书链）

`internal/mc/auth.go` `FetchAuthChain`。

### 请求

- `clientKey` = P-384 公钥 DER(SPKI) 的 base64 去掉 PEM 头（26 字符）后的 **前 160 字符**
  （`chain.go` `PublicKeyB64`）。
- `bit` = `"64"`（PE）、`os_name` = `"android"`、`netease_sid` = `"<房间实体ID>:LobbyGame"`。
- body 用 `HttpEncrypt` 加密后 hex 编码发送（不是 JSON！）：
  `iv(16) + AES-CBC(key, body + 16 随机字符 + 零填充) + 选择字节`（`x19.go`）。
- header `user-token` = **大写 hex** 的动态令牌：

```
prefix = md5hex(token)                      // 32 个 ASCII 字符
raw    = prefix + body + "0eGsBkhl" + path
md5hex2= MD5(raw).hex
token  = 位旋转(6) → 逐字节 XOR → base64 → 取前 16 → '+'→'m','/'→'o' → 末尾加 '1'
```

（`x19.go` `ComputeDynamicToken` + `FinalizeToken`）

### 响应 → 重签

认证服返回的链**不是**能直接用的：要用**本机** EC 私钥重新签发一段 CA JWT 放在最前面。

`internal/mc/chain.go` `ReissueChain`：

```
新链 = [ 新签的 CA JWT(x5u=本机公钥, identityPublicKey=Mojang 公钥),
         原始 chain[0],
         原始 chain[1] ]
AuthenticationType = 0
```

### 取链失败时的降级

`chain.go` `BuildLocalChain`：单段自签 JWT，`AuthenticationType=2`。
实测这条路会被游戏服以 **`reason=92`** 拒绝（见 pitfalls）。

---

## ③ 游戏服登录

`internal/mc/session.go` `Start()`。

1. **RakNet 握手**（`internal/raknet/client.go`，协议版本 8）：
   `OCR1 → Reply1 → OCR2 → Reply2 → ConnectionRequest → Accepted → NewIncomingConnection`。
2. **RequestNetworkSettings(193)**：`varint(193) + 4 字节 BE 协议版本`。
   版本不对会直接收到 `PlayStatus(1/2)` 而不是 NetworkSettings。
3. **NetworkSettings(143)**：`{ compressionThreshold, compressionAlgorithm }`。
   实测服务端给的是 `threshold=1, algorithm=2` → **流式 deflate**（持久化 zlib 流、`Z_SYNC_FLUSH`、
   无 zlib 头），之后所有包都要压缩。见 `compress.go`。
4. **Login(1)**：

```
0x01
+ 4 字节 BE 协议版本
+ varint(bodyLen)
+ 4 字节 LE chainLen + chain
+ 4 字节 LE skinLen  + client_data JWT
```

   构造见 `packet.go` `BuildLogin`；`client_data` 的 JWT 由 `skin.go` `SignSkinData`
   用同一把 EC 密钥签名。
5. **握手 + 加密**：`ServerToClientHandshake(3)` 里带 JWT（含服务端 ECDH 公钥 + salt），
   客户端算共享密钥后回 `ClientToServerHandshake(4)`（**空包，只有一个字节 0x04**）。
   之后进入 AES-256-CTR 会话加密（见 `doc/crypto.md`）。
6. **PlayStatus(2)**：`0` = 登录成功，`3` = PlayerSpawn（正常进服）。

---

## 错误码

| 来源 | 码 | 含义 |
|---|---|---|
| HTTP | 2001 | 版本过低（多半是 `sa_data.os_name` 或 engine/patch 不对） |
| HTTP | 2100 | 操作过于频繁（`/pe-authentication` 有频率限制） |
| HTTP | 22 | 账号在另一处登录（令牌被顶） |
| HTTP | 401 | `/authentication-v2` 拒绝签发证书链 |
| Disconnect | **92** | 证书链不被接受（自签链） |
| Disconnect | **246** | client_data（皮肤 JWT）校验不过 |
| Disconnect | 436 | 指令超配额（用 `SettingsCommand`，别用 `CommandRequest`） |
| PlayStatus | 1 / 2 | 客户端太旧 / 服务端太旧（协议版本不匹配） |
