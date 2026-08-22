# VPS「安装后重启」根因分析（OOM）

## 1. 现象

安装 1S-UI 过程中或刚装完后，VPS **突然断连 / 像断电重启**。  
用户侧常描述为「关机」「重启」，多数情况下并不是 `reboot` 命令，而是：

1. **宿主机/虚拟化层 OOM** 直接杀掉整台虚拟机（最像断电）
2. **访客内核 OOM killer** 杀掉关键进程后系统不可用，或触发云厂商看门狗重启
3. 少数情况：磁盘写满、内核 panic、厂商滥用检测

## 2. 关键证据（仓库与产物）

| 事实 | 数值 / 位置 |
| --- | --- |
| 面板主二进制 `sui` | **~90MB** 静态链接（v1.5.5 amd64） |
| 压缩包 | **~36MB** `s-ui-linux-amd64.tar.gz` |
| Agent 二进制 | ~6.6MB（相对小） |
| 发布构建 tags | `with_gvisor,with_tailscale,with_naive_outbound,with_musl,with_quic,...`（见 `.github/workflows/release.yml`） |
| OpenWrt Lite | ~20–22MB 包，tags 更精简（无 tailscale/naive/gvisor 全家桶） |
| CLI 与后端同二进制 | `backend/main.go` 同时 import `app` + `cmd`，`sui migrate` 也会加载全量代码 |

## 3. 根因排序（按影响）

### R0 — 安装器关闭已有小型 Swap（可直接触发整机 OOM）

旧逻辑在低内存机器上发现 `/swapfile` 小于目标值时，会执行：

1. `swapoff /swapfile`
2. `rm -f /swapfile`
3. 用 `fallocate` 或 `dd bs=64M` 重建 1–2GB 文件

这条路径有三个致命问题：

- `swapoff` 会把已经换出的页面强行搬回 RAM；VPS 本来就在低内存状态，可能当场触发 OOM。
- 无磁盘安全余量检查，2GB 写入可能填满根分区，造成 SSH、systemd 和数据库同时失效。
- `dd bs=64M` 会额外申请大块用户态缓冲区，并在低配磁盘上制造长时间 IO 阻塞。

修复原则是：**任何已有 Swap 都不可关闭、删除、覆盖或改尺寸**。安装器只能创建自己的独立补充 Swap，并且必须先保留至少 512MB 根分区空间；若 cgroup 禁止 Swap 或空间不足，应在下载前安全退出。

### R1 — 安装期多次拉起 90MB Go 进程

安装脚本历史上会连续执行：

1. `sui migrate`
2. `sui admin ...`（随机管理员）
3. `systemctl start s-ui`（再一次全量进程）
4. 有时还有 `sui uri` / `sui setting` / `sui admin -show`

每次都是 **完整链接了 sing-box + gvisor + tailscale + naive/cronet + CGO sqlite** 的进程。

更关键的是：`backend/cmd/migration.MigrateDb()` 在 **全新安装（无数据库文件）时直接 return**，却仍要：

- 启动 90MB 进程
- 完成 Go runtime / 包初始化
- 再发现「Database not found」退出

→ **新鲜安装跑 migrate = 纯内存浪费，几乎零收益。**

在约 **1GB / 0 Swap** 机器上，单次 RSS 常达数百 MB，与系统占用叠加后即可触发 OOM。

### R1b — 页面保存配置绕过安全模式并自动启动内核

`SUI_SKIP_CORE=true` 原本只保护 `APP.Start()` 和定时检查任务，但
`ConfigService.Save()` 在发现 sing-box 未运行时仍会调用 `StartCore()`。
因此低内存 VPS 可能完成安装并正常打开面板，却在第一次保存节点、出站或基础配置时加载完整协议注册表并 OOM。

修复后，安全模式下保存只写入 SQLite；只有管理员明确点击启动/重启内核时才加载 sing-box/Xray。

### R2 — 产物体积与功能耦合过重（结构因）

默认 Linux 发布开启：

- `with_tailscale`
- `with_gvisor`
- `with_naive_outbound` + cronet/musl 工具链

字符串与体积均显示这些代码进了 `sui`。  
这是 **长期 RAM/磁盘压力** 的来源；`SUI_SKIP_CORE` 只能避免 *启动代理内核*，**不能缩小已链接的代码体积，也不能避免包级 import 带来的初始化成本**。

`core` 包的 `register.go` 在编译期就 import 了几乎全部协议实现；  
「懒加载 NewCore」只推迟 `InboundRegistry()` 调用，**无法把 90MB 变成 20MB**。

### R3 — 解压与双份落盘的峰值

完整解压会写出：

- `sui` 90MB
- `sui-agent` 6.6MB
- 脚本/service

再 `cp` 到 `/usr/local/s-ui`，短时间存在 **双份文件 + page cache**。  
page cache 可回收，但与 Go RSS 重叠时仍会推高瞬时压力。

### R4 — 可选组件（历史加重项）

安装期若再叠加：

- Xray zip + geoip/geosite 解压
- `apt install caddy/nginx`

会在已经吃紧的峰值上再叠加一次，旧版脚本曾默认或半默认走到这条路径。

### R5 — Swap 缺失、受 cgroup 限制或创建失败

无 Swap 时，OOM 更倾向 **直接杀 VM**（云厂商常见）。  
自动建 Swap 有用，但在下列情况失败：

- 磁盘空间不足
- 厂商禁用 swap
- LXC/Docker 的 `memory.swap.max=0` 或只允许很小的 Swap
- `fallocate` 稀疏文件在压力下表现差
- 用 `dd` 写 2G 时本身又制造 IO/内存峰值

### R6 — 易被误判的因素

| 误判 | 说明 |
| --- | --- |
| 「必须 2 核 2G 才能装普通面板」 | 产品策略问题，**不是**本次重启的直接机制 |
| 「只是 SSH 断了」 | 若是 hypervisor OOM，整机重启，SSH 必然断 |
| 「SUI_SKIP_CORE 后一定安全」 | 不启内核仍可能因 **单次 90MB 进程 + 多次启动** 而 OOM |

## 4. 安装峰值模型（约 961MB 机）

粗模型（MB，可回收与不可回收混计）：

| 阶段 | 额外压力 | 说明 |
| --- | --- | --- |
| OS + SSH | ~150–250 | 基线 |
| 下载 36MB | 中 | 多可回收 |
| 解压 90MB+ | 高 | 双份文件时更高 |
| **每次** `sui *` | **很高** | 不可回收 RSS 主体 |
| systemctl start | 很高 | 常驻进程 |
| Xray/Caddy | 叠加 | 应完全移出安装关键路径 |

结论：**关键路径上只能允许「一次」全量 `sui` 进程（最终的服务启动）。**

## 5. 已有缓解措施评价

| 措施 | 有效性 | 局限 |
| --- | --- | --- |
| `SUI_SKIP_CORE` | 高（运行期） | 不降低二进制基线体积 |
| `GOMEMLIMIT` | 中 | 限制堆增长，不限制代码段/CGO/映射 |
| 自动 Swap | 高（若成功） | 可能失败或自身有代价 |
| 保留已有 Swap + 独立补充文件 | 高 | 必须同时检查磁盘余量与 cgroup 限额 |
| 默认不装 Xray/反代 | 高 | 低配显式 `--with-xray` 或设置页安装后必须保持 Xray 按需启动 |
| 低配单内核运行 | 高 | 物理/cgroup 有效内存低于 1.5GiB 时，sing-box 与 Xray 只能二选一运行 |
| 懒加载 NewCore | 低–中 | 结构上 import 仍在 |
| 去掉 MemoryMax | 正确 | 硬上限曾导致颠簸更像死机 |

## 6. 正确修复方向

### 立即（安装脚本，不改功能面）

1. **新鲜安装：禁止调用 `sui migrate` / 尽量不调用 `sui admin`**
   - 无 DB 时 migrate 本就是空操作
   - 首次管理员改由浏览器一次性初始化，不创建默认账号
2. **统一基础包**：面板和 Agent 文件一起落盘，Agent 未绑定时保持禁用、不产生常驻进程
3. **升级安装**：仅在已有 DB 时 migrate **一次**
4. **双内核运行保护**：低于 1.5GiB 时，启动 Xray 前停止 sing-box；停止、禁用 Xray 或删除最后一个 Xray 入站后恢复 sing-box；启动失败自动回滚
4. **启动前内存门闩**：`MemAvailable + SwapFree` 过低则中止并提示，避免硬刚
5. 安装结束 **不要** 再跑 `sui uri`（又一次 90MB）
6. **永不执行 `swapoff`**：只新增 `/var/lib/s-ui/swapfile*`，失败时不改系统原有 Swap
7. **磁盘门闩**：Swap 创建后至少保留 512MB，安装包下载前至少保留 384MB
8. **识别 cgroup 限额**：以容器真实内存/Swap 上限替代宿主机 `/proc/meminfo` 假象
9. **不强制 drop_caches**：由内核回收 page cache，避免全局 `sync/drop_caches` 造成 IO 冻结
10. **安全模式贯穿保存链**：Web 保存配置不得隐式启动内核，显式启动操作除外

### 中期（产物结构）

1. 默认发布 **slim** 构建（去掉 naive/tailscale 或拆可选插件），目标二进制明显小于 90MB  
2. 或提供 `s-ui-linux-*-slim.tar.gz`，低内存机器默认拉 slim  
3. CLI 与面板拆分：`sui-cli` 只链 database，migrate/admin 不再加载 sing-box

### 产品策略（已澄清）

- **普通面板**：无硬性 2c2G  
- **集群控制面（服务器监控）**：默认关闭，需在 Web 设置中显式开启；硬性要求 2c2G，安装脚本与 Agent API 均不可绕过
- **受管客户端**：只主动连接主面板，不开放 Agent 控制入口；解绑不会删除本机 Web、入站或代理配置

## 7. 复现与验证清单

安装前后在 VPS 执行：

```bash
free -h
swapon --show
df -h /
# 若再次发生：
dmesg -T | grep -iE 'oom|kill|out of memory' | tail -50
journalctl -u s-ui -n 100 --no-pager
ps aux --sort=-%mem | head
```

成功标准（1G 级机器）：

- 安装过程 **只出现一次** 长时间 `sui` 进程（systemd 拉起的服务）
- Swap 已启用或明确失败并中止
- 原有 Swap 路径、大小和启用状态均未改变
- 根分区不会因自动 Swap 被写满
- 无 Xray/Caddy 安装期步骤
- `dmesg` 无 OOM 记录，机器不重启
