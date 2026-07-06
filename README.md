# 1S-UI

[![Release](https://img.shields.io/github/v/release/Hhz0823/1s-ui?label=release)](https://github.com/Hhz0823/1s-ui/releases/latest)
[![License](https://img.shields.io/github/license/Hhz0823/1s-ui)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8)](go.mod)
[![Vue](https://img.shields.io/badge/Vue-3-42b883)](frontend/package.json)
[![sing-box](https://img.shields.io/badge/core-sing--box-blue)](https://github.com/SagerNet/sing-box)
[![Xray-core](https://img.shields.io/badge/core-Xray--core-green)](https://github.com/XTLS/Xray-core)

1S-UI is a modern proxy management panel based on S-UI, focused on sing-box, multi-protocol inbound management, TLS automation, v2rayN compatible sharing links, and optional Xray-core inbound support.

1S-UI 是基于 S-UI 二次开发的现代代理管理面板，主打 sing-box、多协议入站管理、TLS 自动化、v2rayN 兼容分享链接，以及可选的 Xray-core 入站支持。

> This project is for personal learning, research, and technical communication only. Please comply with local laws and regulations.
>
> 本项目仅用于个人学习、研究和技术交流。请遵守所在地法律法规。

**Languages:** [简体中文](#简体中文) | [English](#english) | [日本語](#日本語)

---

## Screenshots / 页面截图

The screenshots below use local demo data and do not contain real server addresses.

以下截图使用本地演示数据，不包含真实服务器地址。

| Dashboard / 首页仪表 | Inbounds / 入站管理 |
| --- | --- |
| ![Dashboard](docs/screenshots/dashboard.png) | ![Inbounds](docs/screenshots/inbounds.png) |

| Login / 登录 |
| --- |
| ![Login](docs/screenshots/login.png) |

---

## 简体中文

### 项目简介

1S-UI fork 自 [alireza0/s-ui](https://github.com/alireza0/s-ui)，在原 S-UI 的基础上继续优化了界面布局、快速添加节点、TLS 配置、v2rayN 链接兼容、多内核运行和多平台发布流程。

默认核心是 [sing-box](https://github.com/SagerNet/sing-box)。在需要 Xray 特性的场景中，例如 VLESS XHTTP / Reality / TLS，可以为入站选择 [Xray-core](https://github.com/XTLS/Xray-core)。

### 主要特性

- 多协议入站、出站、端点、服务、DNS 和路由管理
- 入站级核心选择：默认 sing-box，可选 Xray-core
- 支持 VMess、VLESS、Trojan、Shadowsocks、Hysteria2、TUIC、Naive、ShadowTLS、AnyTLS、WireGuard 等协议
- 一键添加节点，自动生成端口、标签、用户、TLS 和协议默认参数
- TLS、ACME、ECH、Reality、Pinned Peer Certificate SHA256 集中管理
- Hysteria2 / TLS 分享链接兼容 v2rayN，`pinSHA256` 会按 Xray 需要输出 hex 指纹
- Shadowsocks 默认使用 `2022-blake3-aes-256-gcm` 和 256 位密码强度
- 首页仪表卡、运行状态、日志、备份恢复、使用量统计
- 响应式 UI，支持侧边栏、顶部菜单、主题切换和背景设置
- Linux、Windows、Docker、OpenWrt Lite 多平台发布

### 快速安装

Linux 服务器推荐使用一键脚本：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh)
```

安装指定版本：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh) v1.4.7
```

默认配置通常为：

| 配置项 | 默认值 |
| --- | --- |
| 面板端口 | `2095` |
| 面板路径 | `/app/` |
| 订阅端口 | `2096` |
| 订阅路径 | `/sub/` |
| 数据目录 | `/usr/local/s-ui/db` |

常用命令：

```bash
s-ui
s-ui status
s-ui log
s-ui update
```

### Docker

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

或者：

```bash
docker run -itd \
  --network host \
  -v "$PWD/db:/app/db" \
  -v "$PWD/cert:/app/cert" \
  --name s-ui \
  --restart unless-stopped \
  ghcr.io/Hhz0823/1s-ui
```

### Windows

1. 打开 [Releases](https://github.com/Hhz0823/1s-ui/releases/latest)
2. 下载 Windows 压缩包
3. 解压后以管理员身份运行 `install-windows.bat`
4. 默认面板地址通常为 `http://localhost:2095/app/`

### OpenWrt Lite

OpenWrt Lite 面向路由器和低内存设备，只保留 sing-box 核心，不包含 Xray-core 运行时。

从 [Releases](https://github.com/Hhz0823/1s-ui/releases/latest) 下载对应架构的 `s-ui-lite_*.ipk` 后安装：

```bash
opkg install ./s-ui-lite_1.4.7-1_x86_64.ipk
/etc/init.d/s-ui-lite enable
/etc/init.d/s-ui-lite start
```

更多说明见 [docs/openwrt-lite.md](docs/openwrt-lite.md)。

### 支持平台

| 系统 | 架构 | 状态 |
| --- | --- | --- |
| Linux | amd64, arm64, armv7, armv6, armv5, 386, s390x | 支持 |
| Windows | amd64, arm64 | 支持 |
| Docker | linux/amd64, linux/arm64 | 支持 |
| OpenWrt Lite | x86_64, aarch64, armv7, mips, mipsel, riscv64 | sing-box only |

### 页面功能

| 页面 | 说明 |
| --- | --- |
| 首页 | 系统仪表、运行状态、备份恢复、日志、使用量统计 |
| 入站管理 | 创建、编辑、克隆、删除入站，快速添加节点 |
| 用户管理 | 用户、流量、到期时间、分组、在线状态、二维码 |
| 出站管理 | 出站协议、拨号参数、TLS、传输层配置 |
| 节点管理 | WireGuard、Tailscale、Warp 等端点 |
| 服务管理 | CCM、OCM、DERP、SSMAPI |
| TLS 设置 | TLS、ACME、ECH、Reality、Pinned SHA256 |
| 基础信息 | 日志、实验项、全局 sing-box 配置 |
| 路由列表 | 路由规则、规则集、导入和规则动作 |
| DNS | DNS 服务器、DNS 规则、Fake-IP |
| 管理员 | 管理员账号、API Token、变更记录 |
| 设置 | 面板、订阅、网络、BBR/FQ/CAKE 等配置 |

### 多内核运行

1S-UI 默认使用 sing-box 运行所有入站。对于 Xray-core 独有能力，可以在创建或编辑入站时切换核心。目前重点支持 Xray VLESS，并提供 XHTTP、TCP、WebSocket、gRPC、HTTPUpgrade 等传输选项。

Linux 一键安装脚本和 Docker 镜像会自动准备 Xray-core。手动部署时请将 Xray 二进制放到程序同级 `bin/xray`，Windows 放到 `bin/xray.exe`；也可以通过 `SUI_XRAY_PATH` 指定路径。

### 源码构建

前端：

```bash
cd frontend
npm install
npm run build
```

后端：

```bash
rm -rf web/html/*
cp -R frontend/dist/* web/html/
go build -o sui main.go
```

验证：

```bash
cd frontend && npm run build
go test ./...
```

### 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `SUI_LOG_LEVEL` | `info` | 日志等级 |
| `SUI_DEBUG` | `false` | 调试模式 |
| `SUI_DB_FOLDER` | 程序同级 `db` | 数据库目录 |
| `SUI_BIN_FOLDER` | 程序同级 `bin` | 外部运行时目录 |
| `SUI_XRAY_PATH` | `SUI_BIN_FOLDER/xray` | Xray-core 路径 |
| `SUI_XRAY_CONFIG` | `SUI_BIN_FOLDER/xray.json` | Xray 配置输出路径 |

### 安全建议

- 安装后请立即修改管理员账号和密码
- 建议修改默认端口和面板路径
- 不要公开数据库、证书私钥、API Token
- 公开访问面板时建议放在 HTTPS 反向代理后面
- 分享链接和订阅链接发出前请确认不包含敏感信息

---

## English

### Overview

1S-UI is a proxy management panel based on [S-UI](https://github.com/alireza0/s-ui). It keeps sing-box as the default runtime and adds a cleaner UI, quick node creation, TLS automation, v2rayN-compatible links, optional Xray-core inbound support, and multi-platform releases.

### Highlights

- Manage inbounds, outbounds, endpoints, services, DNS, and routes
- Per-inbound core selection: sing-box by default, Xray-core when needed
- Protocols: VMess, VLESS, Trojan, Shadowsocks, Hysteria2, TUIC, Naive, ShadowTLS, AnyTLS, WireGuard, and more
- Quick node creation with generated port, tag, user, TLS, and protocol defaults
- Centralized TLS, ACME, ECH, Reality, and pinned SHA256 management
- v2rayN-compatible Hysteria2 sharing links, including Xray-ready hex `pinSHA256`
- Dashboard cards, runtime status, logs, backup and restore, usage statistics
- Responsive Vue 3 + Vuetify frontend
- Linux, Windows, Docker, and OpenWrt Lite releases

### Install

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh)
```

Install a specific version:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh) v1.4.7
```

Common commands:

```bash
s-ui
s-ui status
s-ui log
s-ui update
```

### Docker

```bash
docker run -itd \
  --network host \
  -v "$PWD/db:/app/db" \
  -v "$PWD/cert:/app/cert" \
  --name s-ui \
  --restart unless-stopped \
  ghcr.io/Hhz0823/1s-ui
```

### OpenWrt Lite

OpenWrt Lite is designed for routers and low-memory devices. It only includes sing-box and disables Xray-core runtime features.

```bash
opkg install ./s-ui-lite_1.4.7-1_x86_64.ipk
/etc/init.d/s-ui-lite enable
/etc/init.d/s-ui-lite start
```

See [docs/openwrt-lite.md](docs/openwrt-lite.md) for architecture and packaging details.

### Build From Source

```bash
cd frontend
npm install
npm run build
cd ..
rm -rf web/html/*
cp -R frontend/dist/* web/html/
go build -o sui main.go
```

### Security Notes

- Change the administrator username and password after installation
- Use non-default panel paths and ports
- Keep databases, private keys, and API tokens out of public repositories
- Put public panels behind HTTPS reverse proxies
- Review subscription and sharing links before sending them to others

---

## 日本語

### 概要

1S-UI は [S-UI](https://github.com/alireza0/s-ui) をベースにしたプロキシ管理パネルです。標準ランタイムは sing-box で、必要に応じて Xray-core 入站も利用できます。UI、クイックノード作成、TLS 管理、v2rayN 互換リンク、多平台リリースを強化しています。

### 主な機能

- 入站、出站、エンドポイント、サービス、DNS、ルーティングの管理
- 入站ごとのコア選択：標準は sing-box、必要時に Xray-core
- VMess、VLESS、Trojan、Shadowsocks、Hysteria2、TUIC、Naive、ShadowTLS、AnyTLS、WireGuard などをサポート
- クイックノード作成：ポート、タグ、ユーザー、TLS、プロトコル既定値を自動生成
- TLS、ACME、ECH、Reality、Pinned SHA256 を集中管理
- v2rayN 互換の Hysteria2 共有リンクに対応し、Xray 用 `pinSHA256` は hex 形式で出力
- ダッシュボード、実行状態、ログ、バックアップ、使用量統計
- Linux、Windows、Docker、OpenWrt Lite に対応

### インストール

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh)
```

バージョン指定：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh) v1.4.7
```

### Docker

```bash
docker run -itd \
  --network host \
  -v "$PWD/db:/app/db" \
  -v "$PWD/cert:/app/cert" \
  --name s-ui \
  --restart unless-stopped \
  ghcr.io/Hhz0823/1s-ui
```

### OpenWrt Lite

OpenWrt Lite はルーターや低メモリ環境向けの軽量版です。sing-box のみを含み、Xray-core ランタイムは含まれません。

```bash
opkg install ./s-ui-lite_1.4.7-1_x86_64.ipk
/etc/init.d/s-ui-lite enable
/etc/init.d/s-ui-lite start
```

詳細は [docs/openwrt-lite.md](docs/openwrt-lite.md) を参照してください。

### ソースからビルド

```bash
cd frontend
npm install
npm run build
cd ..
rm -rf web/html/*
cp -R frontend/dist/* web/html/
go build -o sui main.go
```

---

## Directory Structure

```text
.
├── api/          # HTTP API
├── app/          # Application bootstrap
├── cmd/          # CLI commands and migrations
├── config/       # Version, name, and environment config
├── core/         # sing-box / Xray-core runtime
├── database/     # Database and models
├── docs/         # Documentation and screenshots
├── frontend/     # Vue 3 + Vuetify frontend
├── service/      # Business services
├── sub/          # Subscription generation
├── util/         # Link, subscription, and config utilities
├── web/          # Web server
└── windows/      # Windows installation scripts
```

## Credits

- [SagerNet/sing-box](https://github.com/SagerNet/sing-box)
- [XTLS/Xray-core](https://github.com/XTLS/Xray-core)
- [alireza0/s-ui](https://github.com/alireza0/s-ui)
- Everyone who tests, reports issues, and contributes feedback

## License

This project is released under the [GPL-3.0](LICENSE) license.
