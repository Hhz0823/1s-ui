# 1S-UI 代码架构与运行风险分析

> 审计基线：`main` / 后端 `1.6.0-boost`，Linux（Ubuntu、Debian）为主要部署目标。

## 1. 项目规模与边界

- Go 后端约 2.0 万行，负责 Web/API、SQLite、订阅、双内核、Agent 和系统操作。
- Vue 3 + TypeScript 前端约 3.0 万行，使用 Vite、Vuetify 和 Pinia。
- 可选玻璃外观由统一 CSS 材质变量和单个按帧节流的指针高光监听器驱动；实色模式不运行折射更新，菜单、弹窗和页面动画支持 `prefers-reduced-motion`。
- Go 模块位于顶层 `backend/`，前端静态资源不再链接或嵌入 `sui`。
- 业务语言收敛为后端 Go 与前端 TypeScript：Shell 只负责安装/打包，Node 只参与前端构建，VPS 运行时不需要 Node.js、Python 或额外的应用服务器。
- v1.5.5 amd64 Release 实测：压缩包 37.7MB，`sui` 94.8MB，`sui-agent` 6.9MB。

## 2. 运行拓扑

```text
Browser
  -> nginx static frontend + API/WS gateway
     -> Gin API-only backend on 127.0.0.1:2097 (session or API token)
     -> service layer
        -> SQLite/GORM
        -> embedded sing-box
        -> external Xray-core process
        -> Linux ip/sysctl/systemd operations

Remote VPS
  -> sui-agent (Bearer token + WebSocket/heartbeat)
     -> metrics, process state, command results, optional root terminal
```

安装层下载架构相关后端包与独立 `s-ui-frontend.tar.gz`，由 nginx 托管前端，并安装
`sui`、sing-box、`sui-agent` 和两个 systemd unit。默认只启动后端、nginx 与 sing-box；Agent 没有主服务器连接配置时
保持禁用。`--connect` 只负责绑定并启动 Agent，Xray 和反代仍是显式可选组件。
旧 `--minimal`、`--managed-client`、`--full` 参数暂时保留为兼容入口。

安装组件状态和运行状态彼此独立：升级前记录 Agent 的 active/enabled 状态，替换
文件后再恢复原连接，避免普通升级把已受管的子服务器永久停掉。

面板运行角色也与安装包解耦：统一包提供 `client`、`full`、`monitor` 三种角色。
`client` 是默认值；`full` 提供完整远程控制；`monitor` 只允许 Agent 注册、心跳、
节点/历史指标和端口流量。两种服务端角色都强制校验 2 核 2 GiB。切回客户端时
立即断开 Agent、撤销配对密钥但保留节点记录；切到仅监控时关闭现有终端，并由
`AgentService` 拒绝命令、批量、终端、远程入站和中转 RPC。旧数据库存在 Agent
或连接密钥时自动继承完整主控制端，兼容已有部署。

主进程启动顺序位于 `backend/app/app.go`：

1. 初始化日志和 SQLite。
2. 补齐默认设置。
3. 创建轻量 Core 句柄、定时任务、Web 服务、订阅服务。
4. 启动 Web、订阅和统计任务。
5. `SUI_SKIP_CORE=true` 时停止在面板层，不加载代理内核。
6. 正常模式恢复面板创建的 IPv6 地址，再启动 sing-box 和需要的 Xray。

## 3. 后端模块

| 目录 | 责任 |
| --- | --- |
| `backend/app/` | 进程生命周期与启动顺序 |
| `backend/web/` | API 路由、Session、内部 HTTP 监听；不托管前端 |
| `backend/api/` | 登录 API、Token API、Agent WebSocket、系统操作入口 |
| `backend/service/` | 业务事务、配置生成、双内核编排、IPv6 中转、WARP、Agent |
| `backend/database/` | SQLite/WAL、迁移、备份、GORM 模型 |
| `backend/core/` | 嵌入式 sing-box、协议注册、Xray 子进程、出站检测 |
| `backend/sub/` | 节点订阅与格式转换 |
| `backend/cronjob/` | 流量统计、重置、在线状态、内核检查 |
| `backend/agent/` | 节点指标采集、控制通道、PTY 终端 |
| `backend/cmd/` | `sui` 管理命令、迁移、独立 Agent 入口 |

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
- Linux 设置页负责 Xray 组件生命周期：官方 Release 下载与校验、原子更新、启用/禁用、启动/停止和受保护卸载。禁用只改变运行策略，不删除入站；有 Xray 入站时卸载请求会被后端拒绝。
- 运行策略同时读取主机内存和 cgroup 限额。有效内存低于 1.5GiB 时 sing-box 与 Xray 共用一个运行槽：手动启动 Xray 先停止 sing-box，停止/禁用 Xray 或删除最后一个 Xray 入站会恢复 sing-box，启动失败也会回滚。
- Web 选择写入 DB 目录标记与 `99-xray-runtime.conf`，优先级高于安装器的 `optimize.conf`，升级不会重新禁用用户已开启的 Xray。

## 5. 业务功能

- 入站、出站、路由、DNS、TLS、服务和 Endpoint 管理。
- 客户端流量、到期、重置、订阅、二维码和批量操作。
- WARP 出站获取、地区参数和延迟检测；不会修改系统默认路由。
- IPv6 中转池：批量地址、端口、账号密码、出站绑定、系统地址恢复。
- SOCKS/多协议批量创建，以及 BitBrowser Excel 和纯文本导出。
- Agent 节点监控：CPU、内存、磁盘、网络、负载、地址、内核状态。
- Agent 控制：状态刷新、服务重启、批量命令和交互终端。
- 运行角色切换：客户端默认关闭 Agent 入站控制面；完整主控制端和仅监控按资源门槛显式启用。
- 子服务器绑定状态：显示主面板地址，支持纯地址换绑和不删除本机配置的安全解绑。
- 单字段绑定：主面板先打开 5 分钟单次连接窗口，子面板只输入公网面板地址；后端自动补全 enrollment 路径并完成独立 Token 签发。窗口首次成功后立即关闭，旧版带密钥地址继续兼容。
- 端口流量：本机和受管服务器按入站显示实时上下行、累计流量和限速，历史聚合使用缓存避免每 3 秒扫描统计表。
- sing-box 入站限速：共享 `StatsTracker` limiter 聚合约束 TCP/UDP 上传和下载；Xray 不支持时由模型/服务层拒绝非零值，不使用会影响 SSH 的系统级 `tc` 规则。
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

全新数据库不写入默认管理员，首次 Web 初始化使用原子条件插入创建唯一管理员并
立即保存 bcrypt 哈希。旧数据库和已有管理员保持不变，旧明文密码会在成功登录后
自动升级为密码哈希。

## 7. 高权限边界

以下路径需要特别保护：

- `install.sh`：包管理、systemd、Swap、反向代理、Xray 下载。
- `backend/service/relay.go`：`ip -6 addr add/del`。
- `backend/api/apiService.go`：`sysctl` 和 `modprobe`。
- `sui-agent`：默认以 root 运行，并支持远程命令和 PTY。

Agent Token 只保存哈希，但控制面板一旦被接管，远程终端等同于节点 root。
生产环境必须使用 HTTPS、强管理员密码、最小暴露面和受控 Agent Token。
浏览器变更 API 统一要求同源或精确 Origin 白名单以及 `X-Requested-With`；terminal WebSocket 复用同一白名单，Agent WebSocket 保持独立 Token/Origin 策略。

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
- 低内存默认启动面板与 sing-box；默认不下载 Xray。显式 `--with-xray` 或设置页安装后可保留双内核，但通过 `SUI_XRAY_ON_DEMAND` 只允许管理员手动启动，并在低于 1.5GiB 时强制两个内核运行态互斥。
- 新安装不执行无意义的 `migrate/admin/uri` 完整进程。
- 安装器不再询问安装类型、面板端口或管理员账号密码；默认路径可以直接用于自动化部署。
- Agent 文件随基础包安装但默认休眠，升级会恢复已有 Agent 的启用和运行状态。

## 9. 仍需长期处理

1. `sui` 仍是 94.8MB 的单体静态二进制；CLI、面板和 sing-box 应拆分。
2. 建议提供 Linux slim Release，去掉不常用的 gVisor、Tailscale、Naive/cronet。
3. Agent root 终端属于高风险运维能力，后续应增加独立开关和更细权限审计。
4. Xray 下载仍依赖 GitHub/Xray Release 可达性；失败只影响可选内核，不应中断 sing-box 与面板。

## 10. 验证结果

- `cd backend && go test ./...`
- `cd backend && go test -tags openwrt_lite ./...`
- `cd backend && go vet ./...`
- `npm run build`
- `npm audit --audit-level=moderate`：0 漏洞
- `govulncheck ./...`：0 个可达漏洞
- `scripts/test-install-safety.sh`
- 本机安全模式烟测：登录、`/app/api/load` 正常，空载 RSS 约 38MB

Linux Release 的完整构建依赖 musl/cronet 工具链；本机验证覆盖代码、交叉编译
Agent 和安装器仿真，真实 512MB/1GB VPS 仍应在发布前做一次控制台监测安装。
