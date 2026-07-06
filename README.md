# 1S-UI

[![Release](https://img.shields.io/github/v/release/Hhz0823/1s-ui?label=release)](https://github.com/Hhz0823/1s-ui/releases/latest)
[![License](https://img.shields.io/github/license/Hhz0823/1s-ui)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8)](go.mod)
[![Vue](https://img.shields.io/badge/Vue-3-42b883)](frontend/package.json)
[![Sing-Box](https://img.shields.io/badge/core-sing--box-blue)](https://github.com/SagerNet/sing-box)

1S-UI 是一个基于 [Sing-Box](https://github.com/SagerNet/sing-box) 的多协议代理管理面板，面向个人服务器、订阅分发、节点管理和快速生成可用配置的场景。

本项目 fork 自 [alireza0/s-ui](https://github.com/alireza0/s-ui)，在原有 S-UI 面板能力上继续优化了界面布局、快速添加节点、TLS 配置、v2rayN 兼容链接和多平台发布流程。

> 本项目仅用于个人学习、研究和技术交流。请遵守所在地法律法规，不要将本项目用于任何非法用途。

---

## 亮点

- 多协议入站、出站、端点、服务和路由管理
- 支持 VLESS、VMess、Trojan、Shadowsocks、Hysteria2、TUIC、Naive、ShadowTLS、AnyTLS、WireGuard 等协议
- 一键添加节点，自动生成端口、标签、用户、TLS 和协议默认参数
- TLS 证书、ACME、ECH、Reality、Pinned SHA256 统一管理
- v2rayN 兼容链接，支持 `pcs` / `pinSHA256` 等 TLS 指纹字段
- Shadowsocks 默认使用 `2022-blake3-aes-256-gcm` 和 256 位密码强度
- 客户端流量、到期时间、在线状态、订阅信息和使用统计
- 响应式桌面/移动端 UI，多主题、侧边栏/顶部菜单布局
- Linux、Windows、Docker 和源码构建支持

---

## 快速安装

Linux 服务器推荐使用一键脚本：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/master/install.sh)
```

安装完成后，脚本会输出面板访问地址。默认配置通常为：

| 配置项 | 默认值 |
| --- | --- |
| 面板端口 | `2095` |
| 面板路径 | `/app/` |
| 订阅端口 | `2096` |
| 订阅路径 | `/sub/` |
| 数据目录 | `/usr/local/s-ui/db` |

安装脚本会引导设置面板端口、路径和管理员账号。跳过设置时，新安装会生成随机管理员账号密码；也可以后续通过 `s-ui` 管理脚本查看或重置。

```bash
s-ui
s-ui help
s-ui status
s-ui log
s-ui update
```

---

## 安装指定版本

在安装命令后追加版本号即可：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/master/install.sh) v1.4.5
```

版本号也可以省略开头的 `v`：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/master/install.sh) 1.4.5
```

---

## Docker

### Docker Compose

```yaml
services:
  s-ui:
    image: ghcr.io/Hhz0823/1s-ui
    container_name: s-ui
    hostname: "s-ui"
    network_mode: host
    volumes:
      - "./db:/app/db"
      - "./cert:/app/cert"
    tty: true
    restart: unless-stopped
    entrypoint: "./entrypoint.sh"
```

```bash
docker compose up -d
```

### Docker Run

```bash
docker run -itd \
  --network host \
  -v "$PWD/db:/app/db" \
  -v "$PWD/cert:/app/cert" \
  --name s-ui \
  --restart unless-stopped \
  ghcr.io/Hhz0823/1s-ui
```

---

## Windows

1. 打开 [Releases](https://github.com/Hhz0823/1s-ui/releases/latest)
2. 下载对应 Windows 压缩包
3. 解压后以管理员身份运行 `install-windows.bat`
4. 根据提示完成安装
5. 默认面板地址通常为 `http://localhost:2095/app/`

Windows 相关脚本在 [windows](windows) 目录中。

---

## 支持的平台

| 系统 | 架构 | 状态 |
| --- | --- | --- |
| Linux | amd64, arm64, armv7, armv6, armv5, 386, s390x | 支持 |
| Windows | amd64, 386, arm64 | 支持 |
| macOS | amd64, arm64 | 实验性 |
| Docker | linux/amd64, linux/arm64 | 支持 |

---

## 协议支持

### 入站协议

| 类型 | 协议 |
| --- | --- |
| 通用 | Mixed, SOCKS, HTTP, Direct |
| V2Ray 系 | VMess, VLESS, Trojan, Shadowsocks |
| 新协议 | Hysteria2, ShadowTLS, TUIC, Naive, AnyTLS |

### 出站与端点

| 类型 | 协议 |
| --- | --- |
| 常规出站 | Direct, Block, DNS, Selector, URLTest |
| 代理协议 | SOCKS, HTTP, Shadowsocks, VMess, VLESS, Trojan, Hysteria, Hysteria2, TUIC, ShadowTLS, Naive |
| 网络端点 | WireGuard, Tailscale, Warp |

---

## 快速添加节点

入站管理页面提供快速添加节点入口，用于创建常见可用配置。

| 协议 | 自动配置内容 |
| --- | --- |
| VMess | TLS + HTTP transport + 默认用户 |
| VLESS | TLS + HTTP transport + Vision flow + 默认用户 |
| Trojan | TLS + HTTP transport + 默认用户 |
| Hysteria2 | TLS + salamander obfs + 默认用户 |
| TUIC | TLS + cubic 拥塞控制 + 默认用户 |
| Naive | TLS + 默认用户 |
| AnyTLS | TLS + padding scheme + 默认用户 |
| ShadowTLS | v3 + handshake server + 默认用户 |
| Shadowsocks | `2022-blake3-aes-256-gcm` + 32 字节随机密码 |
| Mixed / SOCKS / HTTP / Direct | 端口、标签和基础入站配置 |

TLS 类型节点会自动生成自签名证书，并写入客户端侧 `pinned_peer_certificate_sha256`，用于生成更稳定的分享链接。

---

## TLS 与订阅

1S-UI 支持在面板中集中管理 TLS 入站和客户端配置：

- 服务端证书和私钥
- 客户端 SNI、ALPN、uTLS 指纹
- Pinned Peer Certificate SHA256
- ACME 证书申请
- ECH 配置
- Reality keypair
- sing-box、Clash、普通链接订阅

分享链接生成时会尽量保留 sing-box 和 v2rayN 常用字段，例如 Hysteria2 的 `pinSHA256` 和 TLS 的 `pcs`。

---

## 页面功能

| 页面 | 说明 |
| --- | --- |
| 主页 | 系统信息、运行状态、备份恢复、日志和统计入口 |
| 入站管理 | 创建、编辑、克隆、删除入站，快速添加节点 |
| 用户管理 | 用户、流量、到期时间、分组、在线状态和二维码 |
| 出站管理 | 出站协议、拨号参数、TLS、传输层和链路配置 |
| 节点管理 | WireGuard、Tailscale、Warp 等端点配置 |
| 服务管理 | CCM、OCM、DERP、SSMAPI |
| TLS 设置 | TLS、ACME、ECH、Reality、Pinned SHA256 |
| 基础信息 | 日志、实验项、全局 sing-box 基础配置 |
| 路由列表 | 路由规则、规则集、导入和规则动作 |
| DNS | DNS 服务器、DNS 规则、Fake-IP |
| 管理员 | 管理员账号、API Token、变更记录 |
| 设置 | 面板、订阅、网络、BBR/FQ/CAKE 等配置 |

---

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `SUI_LOG_LEVEL` | `info` | 日志等级，支持 `debug`、`info`、`warn`、`error` |
| `SUI_DEBUG` | `false` | 开启调试模式 |
| `SUI_DB_FOLDER` | 程序同级 `db` | 数据库目录 |
| `SUI_BIN_FOLDER` | 旧版本迁移使用 | 迁移旧版二进制目录时使用 |

Docker 镜像默认通过 `/app/db` 保存数据库，建议将该目录挂载到宿主机。

---

## 源码构建

### 前端

```bash
cd frontend
npm install
npm run build
```

### 后端

构建后端前，需要先把前端产物复制到 `web/html`：

```bash
rm -rf web/html/*
cp -R frontend/dist/* web/html/
go build -o sui main.go
```

本地开发可以使用：

```bash
./runSUI.sh
```

常用验证命令：

```bash
cd frontend && npm run build
go test ./...
```

---

## 目录结构

```text
.
├── api/          # HTTP API
├── app/          # 应用启动逻辑
├── cmd/          # CLI 命令和迁移
├── config/       # 版本、名称和环境配置
├── core/         # sing-box 运行和状态管理
├── database/     # 数据库和模型
├── frontend/     # Vue 3 + Vuetify 前端
├── service/      # 业务服务层
├── sub/          # 订阅生成
├── util/         # 链接、订阅和配置工具
├── web/          # Web 服务
└── windows/      # Windows 安装脚本
```

---

## 升级与卸载

升级到最新版本：

```bash
s-ui update
```

卸载：

```bash
systemctl disable s-ui --now
rm -f /etc/systemd/system/s-ui.service
rm -f /etc/systemd/system/sing-box.service
systemctl daemon-reload
rm -rf /usr/local/s-ui
rm -f /usr/bin/s-ui
```

卸载前请备份 `/usr/local/s-ui/db`。

---

## 证书示例

如果需要手动申请证书，可以使用 certbot：

```bash
snap install core
snap refresh core
snap install --classic certbot
ln -s /snap/bin/certbot /usr/bin/certbot
certbot certonly --standalone \
  --register-unsafely-without-email \
  --non-interactive \
  --agree-tos \
  -d example.com
```

也可以直接在面板的 TLS 设置中使用 ACME 相关功能。

---

## 安全说明

- 安装后请立即修改管理员账号和密码
- 面板路径、订阅路径和端口建议使用非默认值
- 不要将数据库、证书私钥、API Token 提交到公开仓库
- 公开访问面板时建议放在反向代理和 HTTPS 后面
- 使用订阅和分享链接前请检查其中是否包含敏感信息

---

## 致谢

- [SagerNet/sing-box](https://github.com/SagerNet/sing-box)
- [alireza0/s-ui](https://github.com/alireza0/s-ui)
- 所有提交、测试和反馈该项目的人

---

## License

本项目基于 [GPL-3.0](LICENSE) 许可发布。
