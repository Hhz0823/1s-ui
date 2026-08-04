# 1S-UI 代码架构与运行风险分析

> 审计基线：`main` / 后端 `1.5.7`，Linux（Ubuntu、Debian）为主要部署目标。

## 1. 项目规模与边界

- Go 后端约 2.0 万行，负责 Web/API、SQLite、订阅、双内核、Agent 和系统操作。
- Vue 3 + TypeScript 前端约 3.0 万行，使用 Vite、Vuetify 和 Pinia。
- Linux Release 把前端静态资源、面板、CLI 和完整 sing-box 能力链接进同一个 `sui`。
- v1.5.5 amd64 Release 实测：压缩包 37.7MB，`sui` 94.8MB，`sui-agent` 6.9MB。

## 2. 运行拓扑

```text
Browser
  -> Gin Web/API (session or API token)
     -> service layer
        -> SQLite/GORM
        -> embedded sing-box
        -> external Xray-core process
        -> Linux ip/sysctl/systemd operations

Remote VPS
  -> sui-agent (Bearer token + WebSocket/heartbeat)
     -> metrics, process state, command results, optional root terminal
```

主进程启动顺序位于 `app/app.go`：

1. 初始化日志和 SQLite。
2. 补齐默认设置。
3. 创建轻量 Core 句柄、定时任务、Web 服务、订阅服务。
4. 启动 Web、订阅和统计任务。
5. `SUI_SKIP_CORE=true` 时停止在面板层，不加载代理内核。
6. 正常模式恢复面板创建的 IPv6 地址，再启动 sing-box 和需要的 Xray。

## 3. 后端模块

| 目录 | 责任 |
| --- | --- |
| `app/` | 进程生命周期与启动顺序 |
| `web/` | Gin、前端资源、HTTP/HTTPS、Session |
| `api/` | 登录 API、Token API、Agent WebSocket、系统操作入口 |
| `service/` | 业务事务、配置生成、双内核编排、IPv6 中转、WARP、Agent |
| `database/` | SQLite/WAL、迁移、备份、GORM 模型 |
| `core/` | 嵌入式 sing-box、协议注册、Xray 子进程、出站检测 |
| `sub/` | 节点订阅与格式转换 |
| `cronjob/` | 流量统计、重置、在线状态、内核检查 |
| `agent/` | 节点指标采集、控制通道、PTY 终端 |
| `cmd/` | `sui` 管理命令、迁移、独立 Agent 入口 |

`ConfigService` 是运行编排中心。配置保存使用数据库事务，提交后按影响范围重启
sing-box 或 Xray。安全模式下保存现在只持久化配置，不再隐式启动内核。

## 4. 双内核能力

### sing-box

sing-box 以内嵌库运行，注册了：

- 入站：TUN、Redirect/TProxy、Direct、SOCKS、HTTP、Mixed、Shadowsocks、
  VMess、Trojan、Naive、ShadowTLS、VLESS、AnyTLS、Hysteria、TUIC、Hysteria2。
- 出站：Direct、Block、Selector、URLTest，以及主要代理协议。
- Endpoint：WireGuard、Tailscale（取决于构建标签）。
- DNS：TCP、UDP、TLS、HTTPS、Hosts、Local、FakeIP、QUIC/HTTP3、DHCP。
- Service：resolved、SSM API、DERP、CCM、OCM。

### Xray-core

Xray 作为独立二进制运行，面板为每个入站保存 `core_type` 并生成独立配置。
当前能力矩阵包括：

- 协议：VLESS、VMess、Trojan、Shadowsocks、SOCKS、HTTP、Mixed、
  Hysteria2、Dokodemo-door、WireGuard。
- 传输：XHTTP、RAW/TCP、mKCP、gRPC、WebSocket、HTTPUpgrade、
  Hysteria2 transport。
- Xray 自检会验证二进制版本、生成配置、入站数量和实际 `-test` 结果。

## 5. 业务功能

- 入站、出站、路由、DNS、TLS、服务和 Endpoint 管理。
- 客户端流量、到期、重置、订阅、二维码和批量操作。
- WARP 出站获取、地区参数和延迟检测；不会修改系统默认路由。
- IPv6 中转池：批量地址、端口、账号密码、出站绑定、系统地址恢复。
- SOCKS/多协议批量创建，以及 BitBrowser Excel 和纯文本导出。
- Agent 节点监控：CPU、内存、磁盘、网络、负载、地址、内核状态。
- Agent 控制：状态刷新、服务重启、批量命令和交互终端。
- 数据库备份/导入、变更审计、系统日志、拥塞控制配置。
- 六种语言：简中、繁中、英文、波斯语、俄语、越南语。

## 6. 数据模型

SQLite 默认位于 `/usr/local/s-ui/db/s-ui.db`，启用 WAL 和连接池限制。
主要表为：

- `settings`：面板、订阅、全局 sing-box 配置。
- `inbounds` / `outbounds` / `tls` / `services` / `endpoints`。
- `clients`：用户、配额、流量、到期和订阅信息。
- `stats` / `changes`：时间桶流量和操作审计。
- `relay_pools`：批量中转资源及面板创建的 IPv6。
- `agent_nodes`：Agent 元数据和 SHA-256 Token 哈希。
- `users` / `tokens`：管理员与 API Token。

旧明文管理员密码会在成功登录后自动升级为密码哈希。

## 7. 高权限边界

以下路径需要特别保护：

- `install.sh`：包管理、systemd、Swap、反向代理、Xray 下载。
- `service/relay.go`：`ip -6 addr add/del`。
- `api/apiService.go`：`sysctl` 和 `modprobe`。
- `sui-agent`：默认以 root 运行，并支持远程命令和 PTY。

Agent Token 只保存哈希，但控制面板一旦被接管，远程终端等同于节点 root。
生产环境必须使用 HTTPS、强管理员密码、最小暴露面和受控 Agent Token。
Agent/浏览器 WebSocket 已恢复同源校验，拒绝跨站终端劫持。

## 8. VPS 失联根因与修复

仓库没有主动调用 `shutdown`、`poweroff`、`halt` 或 `reboot`。严重问题来自 OOM
和磁盘耗尽：

1. 旧安装器会对已有小型 `/swapfile` 执行 `swapoff`，把换出页面压回 RAM。
2. 随后删除原 Swap，并用 64MB 缓冲写最多 2GB，且没有磁盘余量门闩。
3. 安装期历史上多次启动 94.8MB 的完整 `sui`。
4. `SUI_SKIP_CORE` 只保护进程启动，页面保存配置仍会隐式启动 sing-box。
5. 容器可能暴露宿主机内存，使 512MB LXC 被误判为大内存机器。

当前修复：

- 永不关闭、删除、覆盖或调整已有 Swap。
- 只创建 `/var/lib/s-ui/swapfile*` 独立补充文件。
- Swap 后保留至少 512MB 磁盘，安装前保留至少 384MB。
- 删除全局 `drop_caches` 和 64MB `dd` 缓冲。
- 识别 cgroup 内存与 Swap 上限；无法安全安装时在下载前退出。
- `--force` 不能绕过 2c2G 全面服务端门槛，也不能绕过 OOM、Swap、磁盘门闩。
- 低内存默认启动面板与 sing-box、禁用 Xray；仅显式 `--skip-core` 启用纯面板模式。
- 新安装不执行无意义的 `migrate/admin/uri` 完整进程。

## 9. 仍需长期处理

1. `sui` 仍是 94.8MB 的单体静态二进制；CLI、面板和 sing-box 应拆分。
2. 建议提供 Linux slim Release，去掉不常用的 gVisor、Tailscale、Naive/cronet。
3. Agent root 终端属于高风险运维能力，后续应增加独立开关和更细权限审计。
4. 全面模式安装 Xray/反代仍应只用于资源充足的控制面。

## 10. 验证结果

- `go test ./...`
- `go test -tags openwrt_lite ./...`
- `go vet ./...`
- `npm run build`
- `npm audit --audit-level=moderate`：0 漏洞
- `govulncheck ./...`：0 个可达漏洞
- `scripts/test-install-safety.sh`
- 本机安全模式烟测：登录、`/app/api/load` 正常，空载 RSS 约 38MB

Linux Release 的完整构建依赖 musl/cronet 工具链；本机验证覆盖代码、交叉编译
Agent 和安装器仿真，真实 512MB/1GB VPS 仍应在发布前做一次控制台监测安装。
