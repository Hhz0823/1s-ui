# 一键创建节点与客户端兼容性

**入站管理 → 一键添加节点** 一次创建 1–100 条节点：端口、标签、用户、证书、REALITY 密钥和分享链接一次生成，已占用的端口自动跳过。主控在 **服务器监控 → 服务器详情 → 管理入站** 里为子服务器创建节点时，用的是同一套后端逻辑。

## VLESS 的六种模式

VLESS 默认 **REALITY + Vision**。选择只有 Xray-core 支持的模式时，入站自动改用 Xray-core。

| 模式 | 内核 | v2rayN | v2rayNG | 小火箭（Shadowrocket） | Anywhere | PassWall / PassWall 2 |
| --- | --- | --- | --- | --- | --- | --- |
| REALITY + Vision（默认，推荐） | sing-box / Xray-core | ✅ | ✅ | ✅ | ✅ | ✅ |
| REALITY + XHTTP | Xray-core | ✅ | ✅ | ✅ | ✅ | ✅ |
| REALITY + XHTTP + Vision（含 VLESS Encryption） | Xray-core | ✅ | ✅ | 未确认 | iOS 26+ | ✅ |
| VLESS Encryption + Vision | Xray-core | ✅ | ✅ | 未确认 | iOS 26+ | ✅ 订阅导入；在节点页粘贴链接会丢失 flow，需手动选 `xtls-rprx-vision` |
| VLESS Encryption + XHTTP | Xray-core | ✅ | ✅ | 未确认 | iOS 26+ | ✅ |
| TLS + 自签名证书 | sing-box / Xray-core | ✅ | ✅ | ✅ | 需先在 Trusted Certificates 添加链接中的 `pcs` 指纹 | ✅ |
| NaiveProxy（HTTPS / HTTP/2） | sing-box | ✅（分享链接，已用 v2rayN 所用的 sing-box 实测） | ❌ 客户端未实现 | 手动添加：NaiveProxy 类型，旧版为 HTTPS/HTTP2 并开启 Padding | ❌ 客户端未实现 | 可解析 `naive+https` 链接（未实测） |

- **REALITY**：每批节点生成独立的 x25519 密钥和 Short ID，客户端使用 Chrome 指纹；伪装目标留空时服务器自动探测最快可达的 TLS 1.3 + HTTP/2 网站。自动候选已排除 Xray-core 警告会增加 IP 被封概率的 Apple、iCloud、Microsoft 及 `.cn/.ru/.ir` 网站；REALITY 在 443 端口（未被占用时）最自然。
- **Xray-core 版本**：Xray-core 26.9.8 起 REALITY 服务端会拒绝不带 X25519MLKEM768 密钥交换的 ClientHello（小火箭 2.2.92、iOS 26 以下的 Anywhere）。面板安装的是最新稳定版 26.3.27，不受影响；sing-box 的 REALITY 服务端也不受影响，需要最广兼容时选择 sing-box。
- **VLESS Encryption**：抗量子的 `mlkem768x25519plus`（ML-KEM-768 + X25519），不需要 TLS 层，可叠加 XTLS Vision；服务端私钥只保存在面板，链接中只有客户端公钥。
- **XHTTP**：随机路径、`mode=auto`。REALITY + XHTTP 和 VLESS Encryption + XHTTP 不下发 `flow`。
- **REALITY + XHTTP + Vision**：Xray-core 只在 VLESS Encryption 之上允许 XTLS Vision 走 XHTTP（否则报错 “XTLS only supports TLS and REALITY directly”，已用 26.3.27 与 26.9.9 实测），因此该模式同时启用抗量子 VLESS Encryption，链接同时带 `encryption`、`flow=xtls-rprx-vision`、`security=reality` 与 `type=xhttp`。只有这种组合会在 XHTTP 上保留用户的 Vision flow；只开 VLESS Encryption 的 XHTTP 节点仍不下发，老链接不受影响。
- **PassWall / PassWall 2**：订阅导入和在节点页粘贴链接都已按其解析代码逐项校验，并按它生成的 Xray 出站配置实测连通。XHTTP 节点会自动使用 Xray-core；当前 PassWall 生成的配置需要 Xray-core 26.7.11 及以上。

## NaiveProxy

- 一键创建默认使用 HTTPS / HTTP/2（小火箭只支持这一种），QUIC / HTTP/3 仍可选。sing-box 的 Naive 入站只接受带 NaiveProxy Padding 的请求，普通 HTTPS 代理客户端无法连接；v2rayNG 与 Anywhere 请使用 VLESS REALITY 节点。
- **证书**：v2rayN 用 sing-box 的 Naive 出站（Chromium 内核）连接，Chromium 自 2026-03-15 起拒绝有效期超过 200 天的服务器证书（2027-03-15 起 100 天，2029-03-15 起 47 天），即使证书是用户自己信任的也一样（`ERR_CERT_VALIDITY_TOO_LONG`）。因此一键创建的 Naive 节点使用面板私有 CA：分享链接携带并锁定 CA 证书，服务器证书有效期 45 天，面板在到期前 15 天自动续期，客户端无需重新导入。v1.6.3 及更早版本创建的 Naive 节点升级后会自动换成私有 CA，**需要在客户端重新导入一次**。手动选择的 TLS 配置若证书有效期超过 200 天，一键创建会直接提示。

## 下行分离（下载走 CDN）

REALITY + XHTTP、REALITY + XHTTP + Vision 与 VLESS Encryption + XHTTP 可以打开「下行分离」并填写 CDN 域名：上传仍直连服务器 IP，下载经 CDN 回源到本机，适合回程线路差的服务器。

- **域名要求**：在 CDN 开启代理（Cloudflare 橙色云朵）并解析到本服务器 IP；Cloudflare 的 SSL/TLS 模式设为「完全（Full）」；防火墙与云服务器安全组放行 CDN 端口；不要对该域名启用会质询非浏览器客户端的 Bot Fight Mode 或 WAF 规则。CDN 端口默认从 Cloudflare 的 HTTPS 端口 443、2053、2083、2087、2096、8443 中选空闲的，也可手动指定。
- **创建前检查**：面板会在 CDN 端口临时提供一个随机令牌，并经 `https://域名:端口` 取回：域名未开启代理、没有指向本机、端口被防火墙拦截或 SSL 模式不对时直接给出原因，不会创建任何节点。节点地址固定为服务器本身（即使用 CDN 域名打开面板也不会写成 CDN 域名）。
- **实现**：XHTTP 只在同一个 XHTTP 入站内配对上下行，所以该节点在 Xray 中是一个不带安全层的 XHTTP 入站（抽象 Unix 套接字）、节点端口上带 REALITY 的入口，以及 CDN 端口上的 TLS 入口，两个入口都用 VLESS 回落交给同一个 XHTTP 入站。
- **客户端**：分享链接在 XHTTP 的 `extra` 中携带 `downloadSettings`，v2rayN、v2rayNG、PassWall / PassWall 2 与 Anywhere 会按它走 CDN 下载；不支持的客户端（如小火箭）上下行都直连，同样可用。已用真实 Xray-core 在两端、中间模拟 CDN 实测三种模式。

## 其他协议的默认值

- SOCKS / HTTP / Mixed 自动生成用户，不会创建无认证的开放代理，并可生成导入链接。
- Shadowsocks 默认 `2022-blake3-aes-256-gcm`。
- sing-box 的 VMess 使用 WebSocket 传输。
- ShadowTLS 需要配合独立的 Shadowsocks 入站（detour），且 v2rayN 无法导入，所以不在一键创建中，可在完整入站编辑器里手动配置。

## 分享链接与订阅

- 分享链接按 v2rayN 与小火箭的解析方式生成：Shadowsocks 使用 SIP002 URL 安全 Base64，SOCKS 使用 `socks://BASE64(用户:密码)@主机:端口`（v2rayN 不识别 `socks5://`），IPv6 地址自动加方括号，备注统一 URL 编码。
- 使用自动生成的自签证书时，链接同时携带证书指纹（`pcs` / `pinSHA256`）和 `allowInsecure=1` / `insecure=1` 兼容标记：支持指纹的客户端仍会校验证书，不支持指纹的客户端（小火箭、v2rayN 内置 sing-box）也能正常连接。使用受信任的 CA 证书时不会添加该标记。
- VLESS 仅在 TLS / REALITY + TCP 传输时下发 `xtls-rprx-vision`，WebSocket / gRPC 节点不会出现 flow 不匹配。
- 订阅附加的流量 / 到期信息写入节点备注，不会拼接在链接末尾导致无法导入。
- 订阅支持 Clash、JSON 和标准 URI 三种格式。

## 出站导入

出站可以直接粘贴 SOCKS5 / SOCKS4、HTTP(S)、VMess、VLESS、Trojan、Shadowsocks、Hysteria2、TUIC、AnyTLS 和 Naive 链接。Hysteria2 支持 `主机:443,20000-30000` 端口跳跃和 `user:pass` 认证，并去掉 sing-box 不支持的 `fastopen` 与证书指纹字段，导入后可直接保存运行。

## 测试

所有模式都有回归测试：链接按 v2rayN、v2rayNG、Anywhere 与 PassWall / PassWall 2 的真实解析规则逐项校验，并用真实 Xray-core（26.3.27 与 26.9.9）客户端仅凭分享链接（以及按 PassWall 的方式）连接面板生成的服务端、实际转发流量。
