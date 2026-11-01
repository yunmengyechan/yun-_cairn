# 加密与签名

三层，互不相干，别混用。

---

## 1. HTTP 载荷加密

认证侧的 HTTP 接口不是明文，载荷要先加密再 hex 发送。**两套并存**：

### X19（`internal/mc/x19.go` `HttpEncrypt`）

```
明文 = body + 16 个随机可见字符，补零到 16 的倍数
密文 = iv(16) + AES-128-CBC(key, 明文, iv) + 选择字节(1)
选择字节 = (keyIndex << 4) | 0x02      // keyIndex ∈ [0, 14]
```

`/authentication-v2` 用这一套。响应反解：`x19.go` `HttpDecrypt`。

### G79V12（`internal/mc/g79v12.go`）

新协议（`/pe-authentication` 用）。同样布局，但：

- 16 个 AES-128 密钥的索引仍取选择字节高 4 位；
- 低 4 位是 `0x0C`（X19 是 `0x02`），用来区分算法家族；
- 明文侧还有一层 **RC4 流** 与尾部填充规则。

> 两者**不能互解**。判断用哪套：看接口 —— `/pe-authentication` → G79V12，
> `/authentication-v2` 与大厅明文接口 → X19/`HttpEncrypt`。

---

## 2. 动态令牌（HTTP header `user-token`）

```
prefix = md5hex(token)                 // 注意：是 16 字节 md5 的 hex（32 个 ASCII 字符）
raw    = prefix + body + "0eGsBkhl" + path
md5    = MD5(raw)                      // 32 个 ASCII 字符
bits   = 把 md5 的 32 个字节摊成 256 位
bits   = bits[6:] + bits[:6]           // 循环左移 6 位
for i in 0..31:                        // 每个字节与重排后的 8 位做 XOR
    md5[i] ^= reverse_bits(bits[i*8:(i+1)*8])
s = base64(md5)[:16]                   // '+'→'m', '/'→'o'
token = s + "1"                        // 共 17 字符
```

发出去时再做一次 **大写 hex**（`x19.go` `TokenToHeader`）。

两种 token 入口别混（Velox 手册原话）：

- `utility.get_encrypt_token` = 直接拿 **token 原始字节**参与哈希；
- `easy_utils.ComputeDynamicToken` = 先把 **token 前 16 字节转成十六进制**再参与哈希。

PE 客户端互相传的是 `Base64(md5(token))` 形态（VQ 的 `--MD5Token`、Velox 的 `setting.token()`）。

---

## 3. 游戏会话加密（`internal/mc/crypto.go`）

RakNet 握手本身不加密；游戏层在 `ServerToClientHandshake` 之后加密。

```
ECDH: P-384；客户端私钥 = 签证书链/client_data 用的那把
session_key = SHA256(salt + shared_secret)      // 48 字节
iv          = session_key[:12]
加密        = AES-256-CTR（当流密码用，不做 GCM 认证）
签名        = SHA256(seq_u64_LE + 明文 + session_key)[:8]
              先拼到明文尾部，再一起加密
计数器      收发各自独立，跨包连续
```

> ★ 实测：服务端用的是**自研 AES 实现**，不走 OpenSSL/BCrypt，
> 所以 hook 系统加密 API 抓不到明文。

---

## 4. JWT / 证书链签名

- 算法 `ES384`，签名必须是 **raw r||s（各 48 字节）**，不是 DER。
- header/payload 用 jsoncpp `FastWriter` 的字节形式：键按字典序、紧凑分隔符、
  **末尾有换行**、非 ASCII 转成 `\uXXXX`。用普通 `json.Marshal` 会验签失败。
- 本地自签链的 `identity` 用 `md5(userId)` 生成确定性 UUID（改 version/variant 位）。

见 `internal/mc/jwt.go`、`internal/mc/chain.go`、`internal/mc/skin.go`。
