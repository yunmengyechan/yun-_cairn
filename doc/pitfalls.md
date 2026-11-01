# 踩坑清单

按「协议 / 大厅 / 工具链 / 运行时」分类。每条都是踩过的，不是猜的。

## 协议层

1. **`sa_data.os_name` 必须是 `android`** —— 填 `windows` 一律回 `code=2001 版本过低`。
2. **`sign` 参数是 `(2, 8)`** —— 不是 `(4, 7)`。用两份真机抓包的 `message`/`sign` 对验算过。
3. **协议版本要跟着房间版本走** —— 房间 `1.21.130` 对应 Bedrock **898**（不是文档里常见的 860）。
   版本低了服务端回 `PlayStatus=1`，高了回 `PlayStatus=2`，都不会给你 NetworkSettings。
4. **`BlockPos` 的 y 是普通 varint** —— x/z 才是 zigzag。
5. **网络 NBT 的 tag2(Short) 是定长 2 字节** —— 不是 varint。
6. **子区块调色板数量是 zigzag** —— 用普通 varint 会解出 0 个方块。
7. **压缩算法 2 = 流式 deflate** —— 持久化的 zlib 流、`Z_SYNC_FLUSH`、无 zlib 头；
   每包新建压缩器会直接掉线。
8. **`ClientToServerHandshake` 是空包**（只有一个字节 `0x04`），别塞公钥。

## 证书链 / 登录

9. **PE 模式取链要用 `sead`（长令牌），不是 16 位短 token** —— 混用会 401。
10. **取链请求体是加密后的 hex，不是 JSON** —— 直接发 JSON 会 400。
11. **`displayName` 不能为空** —— 空字符串会 400（不是 401，容易误判成令牌问题）。
12. **`clientKey` 是公钥 DER base64 去掉 PEM 头（26 字符）后的 160 字符**，长度错了服务端不认。
13. **自签链（`AuthenticationType=2`）会被游戏服 `reason=92` 拒** —— 必须用认证服签发、
    再用本机私钥重签的 `AuthenticationType=0` 链。
14. **client_data（皮肤 JWT）必须用同一把 EC 密钥签** —— 钥匙不配对会 `reason=246`。

## 大厅接口

15. **`check_visibilily` 官方就是拼错的**（少个 i）—— 必须照抄。
16. **建房不传 `lobby_manifest_version`** —— 传空字符串会 `code=12`。
17. **`max_count` 不能超过地图上限** —— 超了 `code=12`。
18. **搜房是模糊搜索** —— 搜「1234」会返回 11025、2008 一堆，**别拿第一个当目标**。
19. **`code=2311` 是查询限流** —— 等 30~60 秒。
20. **`code=501` 是没有该地图权限** —— 不是房间不存在。

## 保活 / 交互

21. **不发 `PlayerAuthInput` 就是假死** —— 服务端不踢你，但也不理你。
22. **开容器只发 `InventoryTransaction(30)` 不会打开** —— 同一笔事务还要塞进
    `PlayerAuthInput` 的 `item_interact` 输入位（服务端只处理后者）。
23. **`BlockActorData(56)` 随区块流式下发** —— 传送后先 `publish_chunks` 再等，别立刻读。
24. **指令用 `SettingsCommand`** —— `CommandRequest` 连发百余条会 `reason=436` 断开。

## 工具链

25. **`.ps1` 必须 UTF-8 带 BOM** —— PowerShell 5.1 按 ANSI 读会报语法错误。
26. **Go 交叉编译时把 `TMP`/`TEMP` 指到纯 ASCII 路径** —— 中文路径下 cgo/链接会失败。
27. **`$env:TEMP` 改过之后 `$env:TEMP\xxx` 就指向新位置** —— 拼路径要用绝对路径。
28. **JWT 的 header/payload 要按 jsoncpp `FastWriter` 的字节形式生成** ——
    键字典序、紧凑分隔符、末尾带换行、非 ASCII 转 `\uXXXX`；用 `json.Marshal` 会验签失败。
29. **ES384 签名要 raw `r||s`（各 48 字节）** —— 不是 DER。
