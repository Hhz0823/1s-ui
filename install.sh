#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
blue='\033[0;34m'
plain='\033[0m'

cur_dir=$(pwd)

# Decision flags
INSTALL_KIND=""              # minimal | managed | full
INSTALL_PANEL=1
INSTALL_XRAY=0
DISABLE_XRAY=0              # 1 = low-resource runtime is sing-box only
INSTALL_PROXY=0
INSTALL_AGENT=0              # copy sui-agent binary + unit
PROXY_ENGINE=""              # caddy | nginx | ""
PROXY_DOMAIN=""
PROXY_EMAIL=""
PROXY_READY=0                # 1 = reverse proxy was configured and is active
INSTALL_MODE="fresh"         # fresh | upgrade
PROFILE="standard"           # low | standard | high
MEM_TOTAL_MB=0
MEM_AVAIL_MB=0
SWAP_MB=0
CPU_CORES=1
DISK_FREE_MB=0
FORCE_INSTALL=0
AUTO_YES=0
FORCE_XRAY=""                # "" | 1 | 0
FORCE_PROXY=""               # "" | 1 | 0
SKIP_CORE=0                  # 1 = SUI_SKIP_CORE (panel web only)
START_SERVICE=1
REQUESTED_VERSION=""
CONTROLLER_URL=""
AGENT_TOKEN=""
CONNECT_URL=""
AGENT_INSECURE=0
PORT80_FREE=1
PORT443_FREE=1
# Full/cluster recommendation
CLUSTER_CPU_CORES=2
CLUSTER_MEM_MB=2048

# Runtime files are variables so the safety helpers can be tested without
# touching the host. Production execution keeps these Linux defaults.
MEMINFO_FILE="/proc/meminfo"
PROC_SWAPS_FILE="/proc/swaps"
CGROUP_V2_MEMORY_MAX_FILE="/sys/fs/cgroup/memory.max"
CGROUP_V2_MEMORY_CURRENT_FILE="/sys/fs/cgroup/memory.current"
CGROUP_V2_SWAP_MAX_FILE="/sys/fs/cgroup/memory.swap.max"
CGROUP_V2_SWAP_CURRENT_FILE="/sys/fs/cgroup/memory.swap.current"
CGROUP_V1_MEMORY_MAX_FILE="/sys/fs/cgroup/memory/memory.limit_in_bytes"
CGROUP_V1_MEMORY_CURRENT_FILE="/sys/fs/cgroup/memory/memory.usage_in_bytes"
FSTAB_FILE="/etc/fstab"
MANAGED_SWAP_FILE="/var/lib/s-ui/swapfile"
SWAP_DISK_RESERVE_MB=512
SWAP_PREPARED=0
CGROUP_MEMORY_LIMIT_MB=0
CGROUP_SWAP_BLOCKED=0
CGROUP_SWAP_LIMIT_MB=0

usage() {
    cat <<EOF
用法: install.sh [版本号] [选项]

安装模式（三选一，推荐显式指定）:
  --minimal, --simple, -m   轻量 Web 面板（完整 Web UI + sing-box，无 Xray/反代/Agent）
  --managed-client          受管客户端（完整 Web UI + sing-box + Agent，低配 VPS）
  --full, --complete, --server
                            全面服务端（面板 + Xray + 反代 + Agent + 自动启内核）

通用选项:
  -y, --yes             自动确认安装（未指定模式时默认 --minimal）
  --with-xray           额外安装 Xray-core（低配档位会忽略此项并只用 sing-box）
  --no-xray             跳过 Xray-core
  --with-proxy          安装反代（Caddy/Nginx）
  --no-proxy            不安装反代
  --domain DOMAIN       反代域名（HTTPS，多用于全面安装）
  --email EMAIL         ACME 邮箱（Caddy 可选）
  --controller URL      受管客户端连接的中心面板 URL（包含面板路径）
  --agent-token TOKEN   中心面板生成的 Agent 注册密钥
  --connect URL         主面板生成的连接 API 或一次性连接地址（推荐）
  --agent-insecure      Agent 连接中心时跳过 TLS 证书校验
  --start-core          安装后自动启动代理内核
  --skip-core           仅面板 Web，不自动启内核（更安全）
  --no-start            只装文件，不 systemctl start
  --force               兼容旧命令；不能绕过全面服务端的 2核2G 硬门槛
  -h, --help            显示帮助

对比:
  轻量  = 完整 Web UI + sing-box（低配友好）
  全面  = 面板 + Xray + 反代 + Agent 二进制 + 自动启内核
          要求 ≥${CLUSTER_CPU_CORES} 核 / ≥${CLUSTER_MEM_MB}MB（硬门槛）

示例:
  # 轻量 Web 面板（推荐日常/小机器）
  bash install.sh -y --minimal
  bash install.sh v1.5.8 -y -m

  # 全面服务端（生产/多节点控制面）
  bash install.sh -y --full --domain panel.example.com --email a@b.com

  # 受管客户端（完整 Web 面板，可从中心管理入站）
  bash install.sh -y --managed-client --connect 'https://panel.example.com/app/agent/v1/enroll#KEY'
  bash install.sh -y --managed-client --controller https://panel.example.com/app/ --agent-token TOKEN

  # 交互选择模式
  bash install.sh
EOF
}

parse_args() {
    local args=()
    while [[ $# -gt 0 ]]; do
        case "$1" in
        -y | --yes)
            AUTO_YES=1
            shift
            ;;
        --minimal | --simple | -m)
            INSTALL_KIND="minimal"
            shift
            ;;
        --full | --complete | --server)
            INSTALL_KIND="full"
            shift
            ;;
        --managed-client | --managed | --client)
            INSTALL_KIND="managed"
            shift
            ;;
        --with-xray)
            FORCE_XRAY=1
            shift
            ;;
        --no-xray)
            FORCE_XRAY=0
            shift
            ;;
        --with-proxy)
            FORCE_PROXY=1
            shift
            ;;
        --no-proxy)
            FORCE_PROXY=0
            shift
            ;;
        --domain)
            PROXY_DOMAIN="${2:-}"
            shift 2
            ;;
        --email)
            PROXY_EMAIL="${2:-}"
            shift 2
            ;;
        --controller)
            CONTROLLER_URL="${2:-}"
            shift 2
            ;;
        --agent-token)
            AGENT_TOKEN="${2:-}"
            shift 2
            ;;
        --connect)
            CONNECT_URL="${2:-}"
            shift 2
            ;;
        --agent-insecure)
            AGENT_INSECURE=1
            shift
            ;;
        --start-core)
            SKIP_CORE=0
            FORCE_START_CORE=1
            FORCE_SKIP_CORE=0
            shift
            ;;
        --skip-core)
            SKIP_CORE=1
            FORCE_SKIP_CORE=1
            FORCE_START_CORE=0
            shift
            ;;
        --no-start)
            START_SERVICE=0
            shift
            ;;
        --force)
            FORCE_INSTALL=1
            shift
            ;;
        -h | --help)
            usage
            exit 0
            ;;
        -*)
            echo -e "${red}未知参数: $1${plain}"
            usage
            exit 1
            ;;
        *)
            args+=("$1")
            shift
            ;;
        esac
    done
    if [[ ${#args[@]} -gt 0 ]]; then
        REQUESTED_VERSION="${args[0]}"
    fi
}

FORCE_SKIP_CORE=0
FORCE_START_CORE=0

release=""

detect_os() {
    if [[ -f /etc/os-release ]]; then
        # shellcheck source=/dev/null
        source /etc/os-release
        release=$ID
    elif [[ -f /usr/lib/os-release ]]; then
        # shellcheck source=/dev/null
        source /usr/lib/os-release
        release=$ID
    else
        echo "检测系统失败，请联系作者！" >&2
        exit 1
    fi
}

arch() {
    case "$(uname -m)" in
    x86_64 | x64 | amd64) echo 'amd64' ;;
    i*86 | x86) echo '386' ;;
    armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
    armv7* | armv7 | arm) echo 'armv7' ;;
    armv6* | armv6) echo 'armv6' ;;
    armv5* | armv5) echo 'armv5' ;;
    s390x) echo 's390x' ;;
    *) echo -e "${red}不支持的 CPU 架构！${plain}" && exit 1 ;;
    esac
}

port_in_use() {
    local port="$1"
    if command -v ss >/dev/null 2>&1; then
        ss -lntu 2>/dev/null | awk '{print $5}' | grep -Eq "[:.]${port}$" && return 0
    elif command -v netstat >/dev/null 2>&1; then
        netstat -lntu 2>/dev/null | awk '{print $4}' | grep -Eq "[:.]${port}$" && return 0
    fi
    return 1
}

meminfo_kb() {
    local key="$1"
    awk -v key="${key}:" '$1 == key { print $2; exit }' "$MEMINFO_FILE" 2>/dev/null
}

cgroup_value_kb() {
    local file raw
    for file in "$@"; do
        [[ -r "$file" ]] || continue
        raw=$(tr -d '[:space:]' <"$file" 2>/dev/null || true)
        [[ "$raw" =~ ^[0-9]+$ ]] || continue
        awk -v bytes="$raw" 'BEGIN { printf "%.0f\n", bytes / 1024 }'
        return 0
    done
    echo 0
}

effective_available_kb() {
    local available_kb limit_kb current_kb cgroup_available_kb
    available_kb=$(meminfo_kb MemAvailable)
    available_kb=${available_kb:-0}
    limit_kb=$(cgroup_value_kb "$CGROUP_V2_MEMORY_MAX_FILE" "$CGROUP_V1_MEMORY_MAX_FILE")
    current_kb=$(cgroup_value_kb "$CGROUP_V2_MEMORY_CURRENT_FILE" "$CGROUP_V1_MEMORY_CURRENT_FILE")
    if [[ "$limit_kb" -gt 0 ]]; then
        if [[ "$current_kb" -ge "$limit_kb" ]]; then
            cgroup_available_kb=0
        else
            cgroup_available_kb=$((limit_kb - current_kb))
        fi
        if [[ "$available_kb" -eq 0 || "$cgroup_available_kb" -lt "$available_kb" ]]; then
            available_kb="$cgroup_available_kb"
        fi
    fi
    echo "$available_kb"
}

detect_resources() {
    local mem_kb avail_kb swap_kb cgroup_limit_kb swap_limit swap_limit_kb
    CGROUP_MEMORY_LIMIT_MB=0
    CGROUP_SWAP_BLOCKED=0
    CGROUP_SWAP_LIMIT_MB=0
    mem_kb=$(meminfo_kb MemTotal)
    avail_kb=$(effective_available_kb)
    swap_kb=$(meminfo_kb SwapTotal)
    mem_kb=${mem_kb:-0}
    avail_kb=${avail_kb:-0}
    swap_kb=${swap_kb:-0}

    # Containers may expose host /proc/meminfo while enforcing a much smaller
    # cgroup limit. Use the lower value or the installer can misclassify a
    # 512MB LXC container as a large server and trigger its OOM killer.
    cgroup_limit_kb=$(cgroup_value_kb "$CGROUP_V2_MEMORY_MAX_FILE" "$CGROUP_V1_MEMORY_MAX_FILE")
    if [[ "$cgroup_limit_kb" -gt 0 && ( "$mem_kb" -eq 0 || "$cgroup_limit_kb" -lt "$mem_kb" ) ]]; then
        mem_kb="$cgroup_limit_kb"
        CGROUP_MEMORY_LIMIT_MB=$((cgroup_limit_kb / 1024))
    fi

    swap_limit=""
    [[ -r "$CGROUP_V2_SWAP_MAX_FILE" ]] && swap_limit=$(tr -d '[:space:]' <"$CGROUP_V2_SWAP_MAX_FILE" 2>/dev/null || true)
    if [[ "$swap_limit" == "0" ]]; then
        CGROUP_SWAP_BLOCKED=1
        swap_kb=0
    elif [[ "$swap_limit" =~ ^[0-9]+$ ]]; then
        swap_limit_kb=$(awk -v bytes="$swap_limit" 'BEGIN { printf "%.0f\n", bytes / 1024 }')
        CGROUP_SWAP_LIMIT_MB=$((swap_limit_kb / 1024))
        if [[ "$swap_kb" -gt "$swap_limit_kb" ]]; then
            swap_kb="$swap_limit_kb"
        fi
    fi

    MEM_TOTAL_MB=$((mem_kb / 1024))
    MEM_AVAIL_MB=$((avail_kb / 1024))
    SWAP_MB=$((swap_kb / 1024))
    CPU_CORES=$(nproc 2>/dev/null || grep -c ^processor /proc/cpuinfo 2>/dev/null || echo 1)
    DISK_FREE_MB=$(df -Pm /usr/local 2>/dev/null | awk 'NR==2{print $4}')
    [[ -z "$DISK_FREE_MB" ]] && DISK_FREE_MB=$(df -Pm / 2>/dev/null | awk 'NR==2{print $4}')
    [[ -z "$DISK_FREE_MB" ]] && DISK_FREE_MB=0

    if [[ "$MEM_TOTAL_MB" -lt 900 || ( "$MEM_TOTAL_MB" -lt 1200 && "$SWAP_MB" -eq 0 ) || "$CPU_CORES" -lt 2 ]]; then
        PROFILE="low"
    elif [[ "$MEM_TOTAL_MB" -lt 2800 || "$CPU_CORES" -lt 4 ]]; then
        PROFILE="standard"
    else
        PROFILE="high"
    fi

    if [[ -x /usr/local/s-ui/sui ]] || systemctl list-unit-files 2>/dev/null | grep -q '^s-ui\.service'; then
        INSTALL_MODE="upgrade"
    else
        INSTALL_MODE="fresh"
    fi

    for p in 80 443; do
        if port_in_use "$p"; then
            [[ "$p" == "80" ]] && PORT80_FREE=0
            [[ "$p" == "443" ]] && PORT443_FREE=0
        else
            [[ "$p" == "80" ]] && PORT80_FREE=1
            [[ "$p" == "443" ]] && PORT443_FREE=1
        fi
    done
}

choose_install_kind() {
    if [[ -n "$INSTALL_KIND" ]]; then
        return 0
    fi
    if [[ "$AUTO_YES" -eq 1 ]]; then
        INSTALL_KIND="minimal"
        echo -e "${yellow}未指定模式且 -y：默认 ${green}轻量 Web 面板 (--minimal)${plain}"
        return 0
    fi
    echo -e "${blue}========== 选择安装模式 ==========${plain}"
    echo -e "  ${green}1) 轻量 Web 面板${plain}  —— 完整 Web UI + sing-box"
    echo -e "                 不装 Xray / 反代 / Agent，流程短、低配友好"
    echo -e "  ${green}2) 全面服务端${plain} —— 面板 + Xray + 反代 + Agent + 自动启内核"
    echo -e "                 适合生产 / 多节点集群控制面（要求 ≥2核2G）"
    echo -e "  ${green}3) 受管客户端${plain} —— 完整 Web UI + sing-box + Agent"
    echo -e "                 适合由中心面板管理的 1核512MB VPS"
    echo -e "${blue}=================================${plain}"
    read -r -p "请选择 [1/2/3]，默认 1: " kind_ans
    case "${kind_ans}" in
    2 | full | Full | FULL | f | F)
        INSTALL_KIND="full"
        ;;
    3 | managed | Managed | MANAGED | c | C)
        INSTALL_KIND="managed"
        ;;
    *)
        INSTALL_KIND="minimal"
        ;;
    esac
}

# Apply component defaults from INSTALL_KIND, then honor FORCE_* overrides.
apply_kind_defaults() {
    local xray_reason="" proxy_reason="" core_reason=""

    DISABLE_XRAY=0
    if [[ "$PROFILE" == "low" ]]; then
        DISABLE_XRAY=1
    fi

    if [[ "$INSTALL_KIND" == "full" ]]; then
        INSTALL_XRAY=1
        INSTALL_PROXY=1
        INSTALL_AGENT=1
        SKIP_CORE=0
        xray_reason="全面服务端：安装 Xray-core"
        proxy_reason="全面服务端：安装反代"
        core_reason="全面服务端：自动启动代理内核"
        # Full server/Agent control plane is a hard 2c2G gate.
        if [[ "$CPU_CORES" -lt "$CLUSTER_CPU_CORES" || "$MEM_TOTAL_MB" -lt "$CLUSTER_MEM_MB" ]]; then
            echo -e "${red}全面服务端要求至少 ${CLUSTER_CPU_CORES} 核 / ${CLUSTER_MEM_MB}MB，当前 ${CPU_CORES} 核 / ${MEM_TOTAL_MB}MB。${plain}"
            echo -e "${yellow}该配置只能安装轻量 Web 面板：bash install.sh -y --minimal${plain}"
            if [[ "$FORCE_INSTALL" -eq 1 ]]; then
                echo -e "${yellow}--force 不会绕过服务器监控的 2核2G 门槛。${plain}"
            fi
            return 1
        fi
    elif [[ "$INSTALL_KIND" == "managed" ]]; then
        INSTALL_XRAY=0
        INSTALL_PROXY=0
        INSTALL_AGENT=1
        SKIP_CORE=0
        core_reason="受管客户端：启动面板与轻量 sing-box（低配档位禁用 Xray）"
        xray_reason="受管客户端默认 sing-box（可用 --with-xray）"
        proxy_reason="受管客户端默认不安装反代（可按需启用）"
        if [[ -z "$CONNECT_URL" && -z "$CONTROLLER_URL" && "$AUTO_YES" -ne 1 ]]; then
            read -r -p "粘贴主服务器连接 API 或一次性地址（推荐，直接回车可用旧方式）: " CONNECT_URL
        fi
        if [[ -z "$CONNECT_URL" ]]; then
            if [[ -z "$CONTROLLER_URL" && "$AUTO_YES" -ne 1 ]]; then
                read -r -p "中心面板 URL（例如 https://panel.example.com/app/）: " CONTROLLER_URL
            fi
            if [[ -z "$AGENT_TOKEN" && "$AUTO_YES" -ne 1 ]]; then
                read -r -s -p "Agent 注册密钥: " AGENT_TOKEN
                echo
            fi
            if [[ ! "$CONTROLLER_URL" =~ ^https?://[^[:space:]\'\"\\]+$ ]]; then
                echo -e "${red}受管客户端需要 --connect，或有效的 --controller URL。${plain}"
                return 1
            fi
            if [[ ! "$AGENT_TOKEN" =~ ^[A-Za-z0-9_-]{32,128}$ ]]; then
                echo -e "${red}受管客户端需要 --connect，或有效的 --agent-token。${plain}"
                return 1
            fi
        elif [[ "$CONNECT_URL" != http://* && "$CONNECT_URL" != https://* ]] || [[ "$CONNECT_URL" != *"#"* ]]; then
            echo -e "${red}主服务器连接 API 或一次性地址格式无效。${plain}"
            return 1
        fi
    else
        # minimal — 1.4.10-style: panel + sing-box only
        INSTALL_KIND="minimal"
        INSTALL_XRAY=0
        INSTALL_PROXY=0
        INSTALL_AGENT=0
        SKIP_CORE=0
        core_reason="轻量模式：启动面板时加载轻量 sing-box"
        xray_reason="轻量模式：不装 Xray（可用 --with-xray）"
        proxy_reason="轻量模式：不装反代（可用 --with-proxy --domain ...）"
    fi

    # Explicit core flags win over kind defaults
    if [[ "$FORCE_SKIP_CORE" -eq 1 ]]; then
        SKIP_CORE=1
        core_reason="用户指定 --skip-core"
    elif [[ "$FORCE_START_CORE" -eq 1 ]]; then
        SKIP_CORE=0
        core_reason="用户指定 --start-core"
    fi

    # Explicit component overrides
    if [[ "$FORCE_XRAY" == "1" ]]; then
        INSTALL_XRAY=1
        xray_reason="用户指定 --with-xray"
    elif [[ "$FORCE_XRAY" == "0" ]]; then
        INSTALL_XRAY=0
        xray_reason="用户指定 --no-xray"
    fi
    if [[ -z "$(xray_asset)" && "$INSTALL_XRAY" -eq 1 ]]; then
        INSTALL_XRAY=0
        xray_reason="当前架构无自动 Xray 包"
    fi
    if [[ "$DISABLE_XRAY" -eq 1 ]]; then
        INSTALL_XRAY=0
        xray_reason="低配档位：仅使用 sing-box，禁止下载或启动 Xray-core"
    fi

    if [[ "$FORCE_PROXY" == "1" || ( "$INSTALL_KIND" == "full" && "$FORCE_PROXY" != "0" ) || -n "$PROXY_DOMAIN" ]]; then
        if [[ "$FORCE_PROXY" == "0" ]]; then
            INSTALL_PROXY=0
            PROXY_ENGINE=""
            proxy_reason="用户指定 --no-proxy"
        else
            local has_nginx=0 has_caddy=0
            systemctl is-active --quiet nginx 2>/dev/null && has_nginx=1
            systemctl is-active --quiet caddy 2>/dev/null && has_caddy=1
            if [[ "$PORT80_FREE" -ne 1 && "$has_nginx" -ne 1 && "$has_caddy" -ne 1 && -z "$PROXY_DOMAIN" ]]; then
                if [[ "$INSTALL_KIND" == "full" ]]; then
                    INSTALL_PROXY=1
                    PROXY_ENGINE="nginx"
                    proxy_reason="全面安装：80 占用仍尝试配置 Nginx（可能需手工改端口）"
                else
                    INSTALL_PROXY=0
                    proxy_reason="80 端口占用且无现成反代，跳过"
                fi
            else
                INSTALL_PROXY=1
                PROXY_ENGINE="caddy"
                [[ "$MEM_TOTAL_MB" -lt 3000 ]] && PROXY_ENGINE="nginx"
                [[ "$has_nginx" -eq 1 ]] && PROXY_ENGINE="nginx"
                [[ "$has_caddy" -eq 1 ]] && PROXY_ENGINE="caddy"
                proxy_reason="启用反代（${PROXY_ENGINE}）"
            fi
        fi
    fi
    if [[ "$FORCE_PROXY" == "0" ]]; then
        INSTALL_PROXY=0
        PROXY_ENGINE=""
        proxy_reason="用户指定 --no-proxy"
    fi
    if [[ "$MEM_TOTAL_MB" -lt 1500 ]]; then
        if [[ "$INSTALL_XRAY" -eq 1 ]]; then
            INSTALL_XRAY=0
            xray_reason="内存 <1.5G：安装期强制延后 Xray，避免 OOM"
        fi
        if [[ "$INSTALL_PROXY" -eq 1 ]]; then
            INSTALL_PROXY=0
            PROXY_ENGINE=""
            proxy_reason="内存 <1.5G：安装期强制延后反代，避免与面板叠加"
        fi
    fi

    # --start-core / --skip-core already applied via SKIP_CORE / FORCE_SKIP_CORE
    if [[ "$FORCE_SKIP_CORE" -eq 0 ]]; then
        # allow --start-core parsed as SKIP_CORE=0 before apply
        :
    fi

    echo -e "${blue}========== 安装方案 ==========${plain}"
    echo -e "系统：${green}${PRETTY_NAME:-$release}${plain} | 架构：$(arch) | 内核：$(uname -r)"
    echo -e "资源：${CPU_CORES} 核 / 内存 ${MEM_TOTAL_MB}MB（可用 ${MEM_AVAIL_MB}MB）/ Swap ${SWAP_MB}MB / 磁盘约 ${DISK_FREE_MB}MB"
    if [[ "$CGROUP_MEMORY_LIMIT_MB" -gt 0 ]]; then
        echo -e "容器限制：内存上限 ${CGROUP_MEMORY_LIMIT_MB}MB$([ "$CGROUP_SWAP_BLOCKED" -eq 1 ] && echo ' / 禁止 Swap' || true)$([ "$CGROUP_SWAP_LIMIT_MB" -gt 0 ] && echo " / Swap 上限 ${CGROUP_SWAP_LIMIT_MB}MB" || true)"
    fi
    echo -e "档位：${PROFILE} | 面板：${INSTALL_MODE}"
    if [[ "$INSTALL_KIND" == "full" ]]; then
        echo -e "模式：${green}全面服务端 (--full)${plain}"
    elif [[ "$INSTALL_KIND" == "managed" ]]; then
        echo -e "模式：${green}受管客户端 (--managed-client)${plain}"
    else
        echo -e "模式：${green}轻量 Web 面板 (--minimal)${plain}"
    fi
    echo -e "组件：Xray=$(xray_summary_label)  反代=$([ "$INSTALL_PROXY" -eq 1 ] && echo "是(${PROXY_ENGINE:-?})" || echo 否)  Agent=$([ "$INSTALL_AGENT" -eq 1 ] && echo 是 || echo 否)  自动启内核=$([ "$SKIP_CORE" -eq 1 ] && echo 否 || echo 是)"
    echo -e "  Xray：${xray_reason}"
    echo -e "  反代：${proxy_reason}"
    echo -e "  内核：${core_reason}"
    echo -e "${blue}==============================${plain}"

    if [[ "$AUTO_YES" -ne 1 ]]; then
        read -r -p "确认按此方案安装？[Y/n]: " confirm
        if [[ "${confirm}" == "n" || "${confirm}" == "N" ]]; then
            echo "已取消。"
            exit 0
        fi
        if [[ "$INSTALL_KIND" == "full" && -z "$PROXY_DOMAIN" && "$INSTALL_PROXY" -eq 1 ]]; then
            echo -e "${yellow}反代将先使用服务器 IP 的 HTTP:80。安装后请在「面板设置 → 服务端面板」配置域名。${plain}"
        fi
    fi
}

analyze_vps() {
    detect_resources
    choose_install_kind
    apply_kind_defaults || return 1
}

apply_systemd_optimize() {
    local unit="/etc/systemd/system/s-ui.service"
    [[ -f "$unit" ]] || return 0

    # Strip dangerous hard memory caps if present (from older installs).
    # Hard MemoryMax can thrash small VPS into a reboot-like freeze.
    sed -i '/^MemoryMax=/d;/^MemoryHigh=/d' "$unit" 2>/dev/null || true

    mkdir -p /etc/systemd/system/s-ui.service.d
    local skip_line="Environment=SUI_SKIP_CORE=false"
    local xray_line="Environment=SUI_DISABLE_XRAY=false"
    local go_mem_lines=""
    if [[ "$SKIP_CORE" -eq 1 ]]; then
        skip_line="Environment=SUI_SKIP_CORE=true"
        mkdir -p /usr/local/s-ui/db
        touch /usr/local/s-ui/db/.skip_core
        chmod 644 /usr/local/s-ui/db/.skip_core
    else
        rm -f /usr/local/s-ui/db/.skip_core
    fi
    if [[ "$DISABLE_XRAY" -eq 1 ]]; then
        xray_line="Environment=SUI_DISABLE_XRAY=true"
        mkdir -p /usr/local/s-ui/db
        touch /usr/local/s-ui/db/.disable_xray
        chmod 644 /usr/local/s-ui/db/.disable_xray
    else
        rm -f /usr/local/s-ui/db/.disable_xray
    fi

    # Cap Go heap so panel web UI does not balloon toward total RAM.
    # Cores are separate processes (Xray) or started later (sing-box).
    if [[ "$MEM_TOTAL_MB" -lt 1200 ]]; then
        go_mem_lines=$'Environment=GOMEMLIMIT=180MiB\nEnvironment=GOGC=40'
    elif [[ "$MEM_TOTAL_MB" -lt 2048 ]]; then
        go_mem_lines=$'Environment=GOMEMLIMIT=280MiB\nEnvironment=GOGC=50'
    elif [[ "$MEM_TOTAL_MB" -lt 4096 ]]; then
        go_mem_lines=$'Environment=GOMEMLIMIT=512MiB\nEnvironment=GOGC=75'
    fi

    cat >/etc/systemd/system/s-ui.service.d/optimize.conf <<EOF
[Service]
# Prefer killing the panel, not the whole VPS, under memory pressure.
OOMScoreAdjust=800
Nice=10
${skip_line}
${xray_line}
${go_mem_lines}
EOF

    if [[ "$SKIP_CORE" -eq 1 ]]; then
        echo -e "${green}已启用安全模式：面板启动时不自动加载 sing-box/Xray（防 OOM 关机）${plain}"
        echo -e "${yellow}需要代理时在面板配置入站后点「重启内核」${plain}"
    else
        echo -e "${yellow}已配置为自动启动 sing-box 代理内核${plain}"
    fi
    if [[ "$DISABLE_XRAY" -eq 1 ]]; then
        echo -e "${green}低配档位仅启用 sing-box；Xray-core 不下载、不启动${plain}"
    fi
    [[ -n "$go_mem_lines" ]] && echo -e "${green}已限制面板 Go 内存（GOMEMLIMIT），降低 OOM 风险${plain}"
}

install_base() {
    # Only install missing tools. Never full upgrade. Skip apt update when possible
    # (apt update alone can OOM tiny VPS during install).
    local need=0
    for bin in curl tar; do
        command -v "$bin" >/dev/null 2>&1 || need=1
    done
    if [[ "$INSTALL_XRAY" -eq 1 ]]; then
        command -v unzip >/dev/null 2>&1 || need=1
        command -v wget >/dev/null 2>&1 || need=1
    fi
    command -v wget >/dev/null 2>&1 || command -v curl >/dev/null 2>&1 || need=1
    if [[ "$need" -eq 0 ]]; then
        echo -e "${green}基础工具已就绪，跳过包管理器安装（省内存）${plain}"
        return 0
    fi
    local packages=(wget curl tar ca-certificates)
    [[ "$INSTALL_XRAY" -eq 1 ]] && packages+=(unzip)
    case "${release}" in
    centos | almalinux | rocky | oracle)
        yum install -y -q "${packages[@]}"
        ;;
    fedora)
        dnf install -y -q "${packages[@]}"
        ;;
    arch | manjaro | parch)
        pacman -Sy --noconfirm "${packages[@]}"
        ;;
    opensuse-tumbleweed)
        zypper -q install -y "${packages[@]}"
        ;;
    *)
        # Avoid apt-get update on low RAM unless packages missing
        if [[ "$MEM_TOTAL_MB" -ge 1500 ]]; then
            apt-get update -qq 2>/dev/null || true
        fi
        DEBIAN_FRONTEND=noninteractive apt-get install -y -q --no-install-recommends "${packages[@]}" \
            || DEBIAN_FRONTEND=noninteractive apt-get install -y -q "${packages[@]}"
        ;;
    esac
}

xray_asset() {
    case "$(arch)" in
    amd64) echo 'Xray-linux-64.zip' ;;
    386) echo 'Xray-linux-32.zip' ;;
    arm64) echo 'Xray-linux-arm64-v8a.zip' ;;
    armv7) echo 'Xray-linux-arm32-v7a.zip' ;;
    armv6) echo 'Xray-linux-arm32-v6.zip' ;;
    armv5) echo 'Xray-linux-arm32-v5.zip' ;;
    s390x) echo 'Xray-linux-s390x.zip' ;;
    *) echo '' ;;
    esac
}

install_package() {
    local pkg="$1"
    case "${release}" in
    centos | almalinux | rocky | oracle)
        yum install -y -q "$pkg"
        ;;
    fedora)
        dnf install -y -q "$pkg"
        ;;
    arch | manjaro | parch)
        pacman -Sy --noconfirm "$pkg"
        ;;
    opensuse-tumbleweed)
        zypper -q install -y "$pkg"
        ;;
    *)
        DEBIAN_FRONTEND=noninteractive apt-get install -y -q "$pkg"
        ;;
    esac
}

install_caddy_pkg() {
    if command -v caddy >/dev/null 2>&1; then
        return 0
    fi
    case "${release}" in
    ubuntu | debian | armbian)
        apt-get update -qq
        # Official Caddy repo when available; fall back to distro package.
        if ! DEBIAN_FRONTEND=noninteractive apt-get install -y -q caddy 2>/dev/null; then
            apt-get install -y -q debian-keyring debian-archive-keyring apt-transport-https 2>/dev/null || true
            curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' 2>/dev/null | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg 2>/dev/null || true
            curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' 2>/dev/null | tee /etc/apt/sources.list.d/caddy-stable.list >/dev/null 2>&1 || true
            apt-get update -qq 2>/dev/null || true
            DEBIAN_FRONTEND=noninteractive apt-get install -y -q caddy || return 1
        fi
        ;;
    *)
        install_package caddy || return 1
        ;;
    esac
    command -v caddy >/dev/null 2>&1
}

write_caddy_config() {
    local domain="$1"
    local email="$2"
    local panel_port="${3:-2095}"
    mkdir -p /etc/caddy
    {
        echo "# BEGIN 1S-UI MANAGED REVERSE PROXY"
        if [[ -n "$domain" && -n "$email" ]]; then
            echo "{"
            echo "	email ${email}"
            echo "}"
        fi
        if [[ -n "$domain" ]]; then
            echo "${domain} {"
        else
            echo ":80 {"
        fi
        cat <<EOF
	encode gzip
	reverse_proxy 127.0.0.1:${panel_port} {
		header_up X-Real-IP {remote_host}
		header_up X-Forwarded-For {remote_host}
		header_up X-Forwarded-Proto {scheme}
	}
}
EOF
        echo "# END 1S-UI MANAGED REVERSE PROXY"
    } >/etc/caddy/Caddyfile
}

write_nginx_config() {
    local domain="$1"
    local panel_port="${2:-2095}"
    local conf_dir="/etc/nginx/conf.d"
    mkdir -p "$conf_dir" /etc/nginx/sites-available /etc/nginx/sites-enabled 2>/dev/null || true
    local conf_file="${conf_dir}/s-ui.conf"
    # Prefer sites-available when present (Debian style)
    if [[ -d /etc/nginx/sites-available ]]; then
        conf_file="/etc/nginx/sites-available/s-ui.conf"
    fi
    local server_name="_"
    [[ -n "$domain" ]] && server_name="$domain"

    cat >"$conf_file" <<EOF
# BEGIN 1S-UI MANAGED REVERSE PROXY
server {
    listen 80;
    listen [::]:80;
    server_name ${server_name};

    client_max_body_size 32m;

    location / {
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_pass http://127.0.0.1:${panel_port};
    }
}
# END 1S-UI MANAGED REVERSE PROXY
EOF
    if [[ -d /etc/nginx/sites-enabled ]]; then
        ln -sfn "$conf_file" /etc/nginx/sites-enabled/s-ui.conf
        # Disable default site if it would catch all traffic
        rm -f /etc/nginx/sites-enabled/default 2>/dev/null || true
    fi
}

bind_panel_localhost() {
    local domain="$1"
    local uri=""
    if [[ -n "$domain" ]]; then
        local scheme="http"
        [[ "$PROXY_ENGINE" == "caddy" ]] && scheme="https"
        uri="${scheme}://${domain}/app/"
        /usr/local/s-ui/sui setting -listen 127.0.0.1 -domain "$domain" -uri "$uri" >/dev/null 2>&1 || true
    else
        /usr/local/s-ui/sui setting -listen 127.0.0.1 -domain - -uri - >/dev/null 2>&1 || true
    fi
    # Avoid restart storm; caller restarts once if needed.
}

panel_access_url() {
    if [[ "$PROXY_READY" -eq 1 ]]; then
        if [[ -n "$PROXY_DOMAIN" ]]; then
            if [[ "$PROXY_ENGINE" == "caddy" ]]; then
                printf 'https://%s/app/' "$PROXY_DOMAIN"
            else
                printf 'http://%s/app/' "$PROXY_DOMAIN"
            fi
        else
            printf 'http://服务器IP/app/'
        fi
        return
    fi
    printf 'http://服务器IP:2095/app/'
}

proxy_summary_label() {
    if [[ "$INSTALL_PROXY" -ne 1 ]]; then
        printf '否'
    elif [[ "$PROXY_READY" -eq 1 ]]; then
        printf '是(%s，已启动)' "$PROXY_ENGINE"
    else
        printf '否（未启动）'
    fi
}

xray_summary_label() {
    if [[ "$INSTALL_XRAY" -eq 1 ]]; then
        echo "是"
    elif [[ "$DISABLE_XRAY" -eq 1 ]]; then
        echo "否（低配仅 sing-box）"
    else
        echo "否"
    fi
}

install_reverse_proxy() {
    PROXY_READY=0
    if [[ "$INSTALL_PROXY" -ne 1 ]]; then
        echo -e "${yellow}按预检结果跳过反代安装。${plain}"
        return 0
    fi

    local panel_port=2095
    if [[ -x /usr/local/s-ui/sui ]]; then
        local p
        p=$(/usr/local/s-ui/sui setting -show 2>/dev/null | awk -F'\t' '/Panel port/ {print $NF}' | tr -d ' ')
        [[ "$p" =~ ^[0-9]+$ ]] && panel_port="$p"
    fi

    echo -e "${yellow}正在配置反向代理（引擎: ${PROXY_ENGINE}）...${plain}"
    bind_panel_localhost "$PROXY_DOMAIN"
    systemctl try-restart s-ui 2>/dev/null || true
    sleep 1

    if [[ "$PROXY_ENGINE" == "caddy" ]]; then
        if ! install_caddy_pkg; then
            echo -e "${yellow}Caddy 安装失败，回退 Nginx${plain}"
            PROXY_ENGINE="nginx"
        else
            write_caddy_config "$PROXY_DOMAIN" "$PROXY_EMAIL" "$panel_port"
            systemctl enable caddy >/dev/null 2>&1 || true
            if systemctl restart caddy && systemctl is-active --quiet caddy; then
                PROXY_READY=1
                echo -e "${green}Caddy 反代已启动${plain}"
                if [[ -n "$PROXY_DOMAIN" ]]; then
                    echo -e "面板地址：${green}https://${PROXY_DOMAIN}/app/${plain}"
                else
                    echo -e "面板地址：${green}http://服务器IP/app/${plain}（80 端口）"
                fi
                return 0
            fi
            echo -e "${yellow}Caddy 启动失败，回退 Nginx${plain}"
            PROXY_ENGINE="nginx"
        fi
    fi

    if [[ "$PROXY_ENGINE" == "nginx" ]]; then
        if ! install_package nginx; then
            echo -e "${red}Nginx 安装失败，跳过反代${plain}"
            /usr/local/s-ui/sui setting -listen - >/dev/null 2>&1 || true
            systemctl try-restart s-ui 2>/dev/null || true
            return 1
        fi
        write_nginx_config "$PROXY_DOMAIN" "$panel_port"
        nginx -t 2>/dev/null || true
        systemctl enable nginx >/dev/null 2>&1 || true
        if systemctl restart nginx && systemctl is-active --quiet nginx; then
            PROXY_READY=1
            # Caddy fallback changes the public scheme from HTTPS to HTTP.
            bind_panel_localhost "$PROXY_DOMAIN"
            echo -e "${green}Nginx 反代已启动${plain}"
            if [[ -n "$PROXY_DOMAIN" ]]; then
                echo -e "HTTP：${green}http://${PROXY_DOMAIN}/app/${plain}"
                echo -e "${yellow}提示：可用 certbot --nginx -d ${PROXY_DOMAIN} 配置 HTTPS${plain}"
            else
                echo -e "面板地址：${green}http://服务器IP/app/${plain}（80 端口）"
            fi
            return 0
        fi
        echo -e "${red}Nginx 启动失败，已恢复面板监听全部网卡${plain}"
        /usr/local/s-ui/sui setting -listen - >/dev/null 2>&1 || true
        systemctl try-restart s-ui 2>/dev/null || true
        return 1
    fi
}

install_xray() {
    if [[ "$INSTALL_XRAY" -ne 1 ]]; then
        echo -e "${yellow}按预检结果跳过 Xray-core 安装（面板默认使用 sing-box）。${plain}"
        if [[ "$DISABLE_XRAY" -eq 1 ]]; then
            echo -e "${yellow}当前低配档位已禁用 Xray；升级 CPU/内存并重新检测为非低配后才可安装。${plain}"
        else
            echo -e "${yellow}之后可在资源充足时重新执行安装脚本并加 --with-xray。${plain}"
        fi
        return 0
    fi

    local asset
    asset="$(xray_asset)"
    if [[ -z "$asset" ]]; then
        echo -e "${yellow}当前架构暂未配置 Xray-core 自动下载，跳过${plain}"
        return 0
    fi

    local xray_version
    xray_version=$(curl -Ls "https://api.github.com/repos/XTLS/Xray-core/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [[ ! -n "$xray_version" ]]; then
        echo -e "${yellow}获取 Xray-core 最新版本失败，跳过 Xray 准备${plain}"
        return 1
    fi

    echo -e "${yellow}正在安装 Xray-core ${xray_version}...${plain}"
    local tmp_dir="/tmp/s-ui-xray"
    rm -rf "$tmp_dir"
    mkdir -p "$tmp_dir" /usr/local/s-ui/bin

    local zip_path="/tmp/${asset}"
    local url="https://github.com/XTLS/Xray-core/releases/download/${xray_version}/${asset}"
    wget -N --no-check-certificate -O "$zip_path" "$url"
    if [[ $? -ne 0 ]]; then
        echo -e "${yellow}下载 Xray-core 失败，可稍后手动放置到 /usr/local/s-ui/bin/xray${plain}"
        rm -rf "$tmp_dir" "$zip_path"
        return 1
    fi

    unzip -qo "$zip_path" -d "$tmp_dir"
    if [[ ! -f "$tmp_dir/xray" ]]; then
        echo -e "${yellow}Xray-core 压缩包中未找到 xray 二进制，跳过${plain}"
        rm -rf "$tmp_dir" "$zip_path"
        return 1
    fi

    install -m 755 "$tmp_dir/xray" /usr/local/s-ui/bin/xray
    [[ -f "$tmp_dir/geoip.dat" ]] && install -m 644 "$tmp_dir/geoip.dat" /usr/local/s-ui/bin/geoip.dat
    [[ -f "$tmp_dir/geosite.dat" ]] && install -m 644 "$tmp_dir/geosite.dat" /usr/local/s-ui/bin/geosite.dat
    rm -rf "$tmp_dir" "$zip_path"
    echo -e "${green}Xray-core 已安装到 /usr/local/s-ui/bin/xray${plain}"

    if systemctl is-active --quiet s-ui; then
        echo -e "${yellow}正在重启 1S-UI 以加载新的 Xray-core...${plain}"
        if ! systemctl try-restart s-ui; then
            echo -e "${red}Xray-core 已更新，但 1S-UI 重启失败，请检查：journalctl -u s-ui -n 80${plain}"
            return 1
        fi
        sleep 2
        if ! systemctl is-active --quiet s-ui; then
            echo -e "${red}1S-UI 未能在 Xray-core 更新后保持运行，请检查服务日志${plain}"
            return 1
        fi
    fi
}

config_after_install() {
    # migrate is done once by install_s-ui before this; avoid double-loading 90MB binary.
    echo -e "${yellow}安装/更新完成！出于安全考虑，建议修改面板设置 ${plain}"
    read -r -p "是否继续修改设置 [y/n]？: " config_confirm
    if [[ "${config_confirm}" == "y" || "${config_confirm}" == "Y" ]]; then
        echo -e "请输入${yellow}面板端口${plain}（留空则使用现有/默认值）："
        read -r config_port
        echo -e "请输入${yellow}面板路径${plain}（留空则使用现有/默认值）："
        read -r config_path
        echo -e "请输入${yellow}订阅端口${plain}（留空则使用现有/默认值）："
        read -r config_subPort
        echo -e "请输入${yellow}订阅路径${plain}（留空则使用现有/默认值）："
        read -r config_subPath

        echo -e "${yellow}正在初始化，请稍候...${plain}"
        params=""
        [ -z "$config_port" ] || params="$params -port $config_port"
        [ -z "$config_path" ] || params="$params -path $config_path"
        [ -z "$config_subPort" ] || params="$params -subPort $config_subPort"
        [ -z "$config_subPath" ] || params="$params -subPath $config_subPath"
        # shellcheck disable=SC2086
        GOMEMLIMIT=200MiB GOGC=40 /usr/local/s-ui/sui setting ${params}

        read -r -p "是否修改管理员账号密码 [y/n]？: " admin_confirm
        if [[ "${admin_confirm}" == "y" || "${admin_confirm}" == "Y" ]]; then
            read -r -p "请设置用户名：" config_account
            read -r -p "请设置密码：" config_password
            echo -e "${yellow}正在初始化，请稍候...${plain}"
            GOMEMLIMIT=200MiB GOGC=40 /usr/local/s-ui/sui admin -username "${config_account}" -password "${config_password}"
        else
            echo -e "${yellow}当前管理员账号密码：${plain}"
            GOMEMLIMIT=200MiB GOGC=40 /usr/local/s-ui/sui admin -show
        fi
    else
        echo -e "${yellow}跳过自定义。默认管理员：admin / admin（请尽快修改）${plain}"
        echo -e "${yellow}忘记密码可执行： s-ui → 重置管理员${plain}"
    fi
}

prepare_services() {
    if [[ -f "/etc/systemd/system/sing-box.service" ]]; then
        echo -e "${yellow}正在停止 sing-box 服务... ${plain}"
        systemctl stop sing-box 2>/dev/null || true
        rm -f /usr/local/s-ui/bin/sing-box /usr/local/s-ui/bin/runSingbox.sh /usr/local/s-ui/bin/signal
    fi
    if [[ -e "/usr/local/s-ui/bin" ]]; then
        echo -e "###############################################################"
        echo -e "${green}/usr/local/s-ui/bin${yellow} 目录已存在，将保留其中自定义二进制${plain}"
        echo -e "###############################################################"
    fi
    systemctl daemon-reload
}

resolve_managed_connection() {
    [[ -n "$CONNECT_URL" ]] || return 0
    local endpoint code response node_name
    endpoint="${CONNECT_URL%%#*}"
    code="${CONNECT_URL#*#}"
    if [[ ! "$endpoint" =~ ^https?://[^[:space:]\'\"\\]+/agent/v1/(pair|enroll)$ ]] || [[ ! "$code" =~ ^[A-Za-z0-9_-]{32,128}$ ]]; then
        echo -e "${red}主服务器连接 API 或一次性地址格式无效。${plain}"
        return 1
    fi
    node_name=$(hostname 2>/dev/null | tr -d '\r\n' | sed 's/["\\]//g' | cut -c1-80)
    [[ -n "$node_name" ]] || node_name="managed-server"
    local curl_args=(--fail --silent --show-error --max-time 20)
    [[ "$AGENT_INSECURE" -eq 1 ]] && curl_args+=(--insecure)
    echo -e "${yellow}正在连接主服务器...${plain}"
    response=$(curl "${curl_args[@]}" -H 'Content-Type: application/json' --data "{\"code\":\"${code}\",\"name\":\"${node_name}\"}" "$endpoint") || {
        echo -e "${red}主服务器拒绝连接 API；请检查地址、密钥或重新生成。${plain}"
        return 1
    }
    CONTROLLER_URL=$(printf '%s' "$response" | sed -nE 's/.*"panel_url"[[:space:]]*:[[:space:]]*"([^\"]+)".*/\1/p' | head -n1)
    AGENT_TOKEN=$(printf '%s' "$response" | sed -nE 's/.*"token"[[:space:]]*:[[:space:]]*"([^\"]+)".*/\1/p' | head -n1)
    if [[ ! "$CONTROLLER_URL" =~ ^https?://[^[:space:]\'\"\\]+$ ]] || [[ ! "$AGENT_TOKEN" =~ ^[A-Za-z0-9_-]{32,128}$ ]]; then
        echo -e "${red}主面板返回的连接信息无效。${plain}"
        return 1
    fi
}

configure_managed_agent() {
    [[ "$INSTALL_AGENT" -eq 1 ]] || return 0
    resolve_managed_connection || return 1
    [[ -n "$CONTROLLER_URL" && -n "$AGENT_TOKEN" ]] || return 0
    if [[ ! -x /usr/local/s-ui/sui-agent || ! -f /etc/systemd/system/s-ui-agent.service ]]; then
        echo -e "${red}受管客户端缺少 Agent 二进制或 systemd 服务。${plain}"
        return 1
    fi
    mkdir -p /etc/default
    umask 077
    {
        printf 'SUI_AGENT_PANEL=%s\n' "$CONTROLLER_URL"
        printf 'SUI_AGENT_TOKEN=%s\n' "$AGENT_TOKEN"
        printf 'SUI_AGENT_INTERVAL=15s\n'
        printf 'SUI_AGENT_INSECURE=%s\n' "$([ "$AGENT_INSECURE" -eq 1 ] && echo true || echo false)"
        printf 'SUI_AGENT_LOCAL_SOCKET=/run/s-ui/control.sock\n'
    } > /etc/default/1s-ui-agent
    chmod 0600 /etc/default/1s-ui-agent
    systemctl daemon-reload
    if [[ "$START_SERVICE" -ne 1 ]]; then
        echo -e "${yellow}Agent 已配置但未启动；稍后执行 systemctl enable --now s-ui-agent${plain}"
        return 0
    fi
    local check=(/usr/local/s-ui/sui-agent --panel "$CONTROLLER_URL" --token "$AGENT_TOKEN" --local-socket /run/s-ui/control.sock --interval 15s --once)
    [[ "$AGENT_INSECURE" -eq 1 ]] && check+=(--insecure)
    echo -e "${yellow}验证中心面板连接和本机 Web 面板控制通道...${plain}"
    if [[ ! -S /run/s-ui/control.sock ]]; then
        echo -e "${red}本机 1S-UI 控制通道不存在，请检查 s-ui 服务版本和日志。${plain}"
        return 1
    fi
    if ! "${check[@]}"; then
        echo -e "${red}Agent 无法连接中心面板，请检查 URL、Token、防火墙和证书。${plain}"
        return 1
    fi
    systemctl enable --now s-ui-agent
    systemctl is-active --quiet s-ui-agent || return 1
    echo -e "${green}受管客户端已连接中心面板，本地 Web 面板保持可独立使用。${plain}"
}

# Reclaimable + swap free, in MB (best-effort).
mem_budget_mb() {
    local avail_kb swap_free_kb swap_limit_kb swap_current_kb cgroup_swap_free_kb
    avail_kb=$(effective_available_kb)
    swap_free_kb=$(meminfo_kb SwapFree)
    avail_kb=${avail_kb:-0}
    swap_free_kb=${swap_free_kb:-0}
    [[ "$CGROUP_SWAP_BLOCKED" -eq 1 ]] && swap_free_kb=0
    if [[ "$CGROUP_SWAP_LIMIT_MB" -gt 0 ]]; then
        swap_limit_kb=$((CGROUP_SWAP_LIMIT_MB * 1024))
        swap_current_kb=$(cgroup_value_kb "$CGROUP_V2_SWAP_CURRENT_FILE")
        if [[ "$swap_current_kb" -ge "$swap_limit_kb" ]]; then
            cgroup_swap_free_kb=0
        else
            cgroup_swap_free_kb=$((swap_limit_kb - swap_current_kb))
        fi
        [[ "$swap_free_kb" -gt "$cgroup_swap_free_kb" ]] && swap_free_kb="$cgroup_swap_free_kb"
    fi
    echo $(((avail_kb + swap_free_kb) / 1024))
}

# Refuse to launch the large static binary if remaining budget is too low.
require_mem_budget() {
    local need="${1:-280}"
    local have
    have=$(mem_budget_mb)
    echo -e "可用内存预算（MemAvailable+SwapFree）：${have}MB（启动面板建议 ≥${need}MB）"
    if [[ "$have" -lt "$need" ]]; then
        echo -e "${red}内存预算不足，强行启动面板进程极易触发 OOM 并造成整机失联。${plain}"
        echo -e "${yellow}请：1) 确认 Swap 已启用  2) 关闭其它占内存进程  3) 或换 ≥2G 机器${plain}"
        echo -e "${yellow}排查： free -h; swapon --show; dmesg | grep -i oom | tail${plain}"
        return 1
    fi
    return 0
}

core_start_budget_mb() {
    if [[ "$SKIP_CORE" -eq 1 ]]; then
        echo 384
    elif [[ "$DISABLE_XRAY" -eq 1 || "$INSTALL_XRAY" -eq 0 ]]; then
        echo 512
    else
        echo 768
    fi
}

swap_path_is_active() {
    local path="$1"
    awk -v path="$path" 'NR > 1 && $1 == path { found=1 } END { exit found ? 0 : 1 }' "$PROC_SWAPS_FILE" 2>/dev/null
}

disk_free_mb_at() {
    local path="$1"
    df -Pm "$path" 2>/dev/null | awk 'NR == 2 { print $4; exit }'
}

require_install_disk_budget() {
    local need="${1:-384}"
    local have
    have=$(disk_free_mb_at /usr/local)
    [[ -z "$have" ]] && have=$(disk_free_mb_at /)
    have=${have:-0}
    if [[ "$have" -le 0 ]]; then
        echo -e "${red}无法读取根分区剩余空间，已停止安装以避免写满磁盘。${plain}"
        return 1
    fi
    if [[ "$have" -gt 0 && "$have" -lt "$need" ]]; then
        echo -e "${red}磁盘空间不足：仅剩 ${have}MB，安全安装至少需要 ${need}MB。${plain}"
        echo -e "${yellow}已在下载和解压前停止，避免写满根分区导致 VPS 失联。${plain}"
        return 1
    fi
    return 0
}

# Pick a new path owned by this installer. Existing swap files are never
# disabled, resized, removed, or overwritten.
next_managed_swap_path() {
    local candidate suffix
    for suffix in "" ".supplemental" ".supplemental.2" ".supplemental.3"; do
        candidate="${MANAGED_SWAP_FILE}${suffix}"
        if [[ ! -e "$candidate" ]] && ! swap_path_is_active "$candidate"; then
            echo "$candidate"
            return 0
        fi
    done
    return 1
}

ensure_swap_if_needed() {
    [[ "$SWAP_PREPARED" -eq 1 ]] && return 0

    local desired_total_mb=1024
    local minimum_total_mb=512
    [[ "$MEM_TOTAL_MB" -lt 700 ]] && minimum_total_mb=768

    # Only low-memory hosts need automatic swap. A healthy existing swap of at
    # least the minimum below is enough for the panel + sing-box startup path.
    if [[ "$MEM_TOTAL_MB" -ge 1500 || "$SWAP_MB" -ge "$minimum_total_mb" ]]; then
        SWAP_PREPARED=1
        return 0
    fi

    if [[ "$CGROUP_SWAP_BLOCKED" -eq 1 ]]; then
        echo -e "${red}当前容器的 cgroup 禁止 Swap，无法安全补充内存。${plain}"
        echo -e "${yellow}请在 VPS 管理面板提高内存/Swap 限额，脚本不会冒险继续。${plain}"
        return 1
    fi
    if [[ "$CGROUP_SWAP_LIMIT_MB" -gt 0 && "$CGROUP_SWAP_LIMIT_MB" -lt "$minimum_total_mb" ]]; then
        echo -e "${red}当前容器的 Swap 上限仅 ${CGROUP_SWAP_LIMIT_MB}MB，创建宿主机 Swap 也无法突破该限制。${plain}"
        echo -e "${yellow}请先在 VPS 管理面板提高 cgroup Swap 限额。${plain}"
        return 1
    fi

    local desired_add_mb=$((desired_total_mb - SWAP_MB))
    local minimum_add_mb=$((minimum_total_mb - SWAP_MB))
    [[ "$minimum_add_mb" -lt 1 ]] && {
        SWAP_PREPARED=1
        return 0
    }

    local swap_path swap_dir free_mb max_add_mb size_mb partial
    swap_path=$(next_managed_swap_path) || {
        echo -e "${red}找不到可安全创建的补充 Swap 路径；现有文件均保持不变。${plain}"
        return 1
    }
    swap_dir=$(dirname "$swap_path")
    mkdir -p "$swap_dir"

    free_mb=$(disk_free_mb_at "$swap_dir")
    free_mb=${free_mb:-0}
    if [[ "$free_mb" -le "$SWAP_DISK_RESERVE_MB" ]]; then
        echo -e "${red}磁盘仅剩 ${free_mb}MB，必须保留至少 ${SWAP_DISK_RESERVE_MB}MB；不创建 Swap。${plain}"
        return 1
    fi
    max_add_mb=$((free_mb - SWAP_DISK_RESERVE_MB))
    if [[ "$max_add_mb" -lt "$minimum_add_mb" ]]; then
        echo -e "${red}磁盘不足以安全补充 Swap：需至少 ${minimum_add_mb}MB，保留系统空间后仅可用 ${max_add_mb}MB。${plain}"
        return 1
    fi
    size_mb="$desired_add_mb"
    [[ "$size_mb" -gt "$max_add_mb" ]] && size_mb="$max_add_mb"

    echo -e "${yellow}低内存机器：新增 ${size_mb}MB 独立 Swap（保留已有 Swap，不执行 swapoff）${plain}"
    partial="${swap_path}.partial"
    rm -f "$partial"

    # Btrfs swap files need NOCOW before allocation. Harmless elsewhere.
    : >"$partial"
    chattr +C "$partial" 2>/dev/null || true

    local ok=0
    if command -v fallocate >/dev/null 2>&1 && fallocate -l "${size_mb}M" "$partial" 2>/dev/null; then
        ok=1
    else
        rm -f "$partial"
        # 1MB blocks avoid the old 64MB userspace buffer spike on tiny VPS.
        if dd if=/dev/zero of="$partial" bs=1M count="$size_mb" status=none conv=fsync 2>/dev/null; then
            ok=1
        fi
    fi
    if [[ "$ok" -eq 1 ]]; then
        chmod 600 "$partial"
        if mkswap "$partial" >/dev/null 2>&1; then
            mv "$partial" "$swap_path"
            if swapon "$swap_path" >/dev/null 2>&1; then
                ok=2
            fi
        fi
    fi
    if [[ "$ok" -ne 2 ]]; then
        rm -f "$partial" "$swap_path"
        echo -e "${red}补充 Swap 创建或启用失败；未改动任何已有 Swap。${plain}"
        return 1
    fi

    if ! awk -v path="$swap_path" '$1 == path && $3 == "swap" { found=1 } END { exit found ? 0 : 1 }' "$FSTAB_FILE" 2>/dev/null; then
        printf '%s none swap sw 0 0\n' "$swap_path" >>"$FSTAB_FILE"
    fi
    SWAP_MB=$((SWAP_MB + size_mb))
    SWAP_PREPARED=1
    echo -e "${green}补充 Swap 已启用：${swap_path}（${size_mb}MB，总计约 ${SWAP_MB}MB）${plain}"
    return 0
}

# Sets DOWNLOAD_TARBALL path on success.
download_release() {
    local version="$1"
    local arch_name
    arch_name="$(arch)"
    local url="https://github.com/Hhz0823/1s-ui/releases/download/${version}/s-ui-linux-${arch_name}.tar.gz"
    local out="/tmp/s-ui-linux-${arch_name}.tar.gz"
    DOWNLOAD_TARBALL=""
    rm -f "$out"
    echo -e "下载：${url}"
    if command -v curl >/dev/null 2>&1; then
        curl -fL --retry 3 --retry-delay 2 --connect-timeout 20 -o "$out" "$url" || return 1
    else
        wget -q --no-check-certificate -O "$out" "$url" || return 1
    fi
    local sz
    sz=$(stat -c%s "$out" 2>/dev/null || stat -f%z "$out" 2>/dev/null || echo 0)
    if [[ "${sz:-0}" -lt 5000000 ]]; then
        echo -e "${red}下载文件过小（${sz} bytes），可能 404 或截断${plain}"
        head -c 200 "$out" 2>/dev/null || true
        rm -f "$out"
        return 1
    fi
    DOWNLOAD_TARBALL="$out"
}

install_s-ui() {
    cd /tmp/ || exit 1

    # ------------------------------------------------------------------
    # OOM root-cause controls (see docs/oom-reboot-analysis.md):
    #  - Panel binary ~90MB; each `sui` CLI invocation is a full process.
    #  - Fresh install: migrate is a no-op (no DB) but still costs full RSS.
    #  - Critical path must launch the 90MB binary AT MOST ONCE (systemd start).
    # ------------------------------------------------------------------

    echo -e "${yellow}安装前停止旧服务并准备 Swap（防 OOM 重启）...${plain}"
    systemctl stop s-ui 2>/dev/null || true
    systemctl stop s-ui-agent 2>/dev/null || true
    free -h 2>/dev/null || true
    swapon --show 2>/dev/null || true
    require_install_disk_budget 384 || exit 1

    local last_version
    if [[ -z "$REQUESTED_VERSION" ]]; then
        last_version=$(curl -Ls "https://api.github.com/repos/Hhz0823/1s-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$last_version" ]]; then
            last_version=$(curl -Ls "https://api.github.com/repos/Hhz0823/1s-ui/releases?per_page=5" | grep '"tag_name":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
        fi
        if [[ ! -n "$last_version" ]]; then
            echo -e "${red}获取 s-ui 版本失败，可能是 Github API 限制导致，请稍后重试${plain}"
            exit 1
        fi
        echo -e "已获取 s-ui 版本：${last_version}，开始安装..."
    else
        last_version="$REQUESTED_VERSION"
        [[ "${last_version}" != v* ]] && last_version="v${last_version}"
        echo -e "开始安装 s-ui ${last_version}"
    fi

    if ! download_release "$last_version"; then
        echo -e "${red}下载 s-ui ${last_version} 失败，请确认可访问 Github${plain}"
        exit 1
    fi
    local tarball="$DOWNLOAD_TARBALL"

    systemctl stop s-ui 2>/dev/null || true
    rm -rf /tmp/s-ui-extract
    mkdir -p /tmp/s-ui-extract

    # Selective extract: panel files always; agent only for full server.
    local members=(s-ui/sui s-ui/s-ui.service s-ui/s-ui.sh)
    if [[ "$INSTALL_AGENT" -eq 1 ]]; then
        members+=(s-ui/sui-agent s-ui/s-ui-agent.service)
        echo -e "${yellow}解压面板 + Agent...${plain}"
    else
        echo -e "${yellow}选择性解压（轻量 Web 面板：sui + Web UI + service）...${plain}"
    fi
    local extract_ok=0
    if tar xzf "$tarball" -C /tmp/s-ui-extract "${members[@]}" 2>/dev/null; then
        extract_ok=1
    else
        echo -e "${yellow}选择性解压失败，回退完整解压${plain}"
        tar xzf "$tarball" -C /tmp/s-ui-extract || extract_ok=0
        [[ -f /tmp/s-ui-extract/s-ui/sui || -f /tmp/s-ui-extract/sui ]] && extract_ok=1
    fi
    rm -f "$tarball"
    if [[ "$extract_ok" -ne 1 ]]; then
        echo -e "${red}解压失败${plain}"
        rm -rf /tmp/s-ui-extract
        exit 1
    fi

    local src_dir="/tmp/s-ui-extract/s-ui"
    [[ -d "$src_dir" ]] || src_dir="/tmp/s-ui-extract"
    if [[ ! -f "$src_dir/sui" ]]; then
        echo -e "${red}包内未找到 sui 二进制${plain}"
        ls -la "$src_dir" /tmp/s-ui-extract 2>/dev/null || true
        rm -rf /tmp/s-ui-extract
        exit 1
    fi

    mkdir -p /usr/local/s-ui /usr/local/s-ui/db /usr/local/s-ui/bin
    cp -f "$src_dir/sui" /usr/local/s-ui/sui
    chmod +x /usr/local/s-ui/sui
    if [[ -f "$src_dir/s-ui.sh" ]]; then
        cp -f "$src_dir/s-ui.sh" /usr/local/s-ui/s-ui.sh
        cp -f "$src_dir/s-ui.sh" /usr/bin/s-ui
        chmod +x /usr/bin/s-ui /usr/local/s-ui/s-ui.sh
    fi
    if [[ -f "$src_dir/s-ui.service" ]]; then
        cp -f "$src_dir/s-ui.service" /etc/systemd/system/s-ui.service
    elif [[ -f /tmp/s-ui-extract/s-ui.service ]]; then
        cp -f /tmp/s-ui-extract/s-ui.service /etc/systemd/system/s-ui.service
    fi
    if [[ "$INSTALL_AGENT" -eq 1 && -f "$src_dir/sui-agent" ]]; then
        cp -f "$src_dir/sui-agent" /usr/local/s-ui/sui-agent
        chmod +x /usr/local/s-ui/sui-agent
        [[ -f "$src_dir/s-ui-agent.service" ]] && cp -f "$src_dir/s-ui-agent.service" /etc/systemd/system/
        echo -e "${green}已安装 sui-agent 二进制${plain}"
    fi
    rm -rf /tmp/s-ui-extract

    # systemd: SUI_SKIP_CORE + GOMEMLIMIT before first start
    apply_systemd_optimize
    prepare_services

    local has_db=0
    [[ -f /usr/local/s-ui/db/s-ui.db ]] && has_db=1

    # ---- CLI policy (critical) ----
    # Fresh install: DO NOT run `sui migrate` or `sui admin`.
    #   migrate() exits immediately when DB missing, but still loads ~90MB RSS.
    #   InitDB on first service start creates admin/admin.
    # Upgrade: run migrate ONCE only if DB already exists.
    if [[ "$has_db" -eq 1 ]]; then
        echo -e "${yellow}检测到已有数据库：升级路径，执行一次 migrate...${plain}"
        if require_mem_budget 300; then
            GOMEMLIMIT=200MiB GOGC=40 /usr/local/s-ui/sui migrate || \
                echo -e "${yellow}migrate 非零退出，继续启动服务${plain}"
        else
            echo -e "${red}内存不足，跳过 migrate；请空闲时手动： /usr/local/s-ui/sui migrate${plain}"
        fi
    else
        echo -e "${green}全新安装：跳过 migrate/admin CLI（避免无意义的 90MB 进程峰值）${plain}"
        echo -e "${yellow}默认登录：admin / admin（登录后请立即修改）${plain}"
    fi

    # Interactive settings only when user is present AND memory allows an extra CLI.
    # Skip on -y / low RAM to keep critical path = single service start.
    if [[ "$AUTO_YES" -ne 1 && "$MEM_TOTAL_MB" -ge 1500 ]]; then
        config_after_install
    elif [[ "$AUTO_YES" -ne 1 ]]; then
        echo -e "${yellow}低内存：跳过交互式 setting CLI；可用面板 Web 或稍后 s-ui 菜单修改${plain}"
    fi

    systemctl daemon-reload
    systemctl enable s-ui >/dev/null 2>&1 || true

    if [[ "$START_SERVICE" -eq 1 ]]; then
        # This should be the ONLY full binary launch on a fresh install.
        local start_budget
        start_budget=$(core_start_budget_mb)
        require_mem_budget "$start_budget" || exit 1
        if [[ "$SKIP_CORE" -eq 1 ]]; then
            echo -e "${yellow}启动面板（唯一一次加载主进程；安全模式不启内核）...${plain}"
        else
            echo -e "${yellow}启动面板与代理内核（已通过 ${start_budget}MB 启动预算检查）...${plain}"
        fi
        if ! systemctl start s-ui; then
            echo -e "${red}s-ui 服务启动失败，最近日志：${plain}"
            journalctl -u s-ui -n 80 --no-pager || true
            echo -e "${yellow}请查 OOM: dmesg | grep -iE 'oom|killed' | tail${plain}"
            exit 1
        fi
        sleep 3
        if ! systemctl is-active --quiet s-ui; then
            echo -e "${red}s-ui 未能保持运行，最近日志：${plain}"
            journalctl -u s-ui -n 80 --no-pager || true
            dmesg 2>/dev/null | grep -iE 'oom|kill' | tail -20 || true
            exit 1
        fi
        if [[ "$SKIP_CORE" -eq 1 ]]; then
            echo -e "${green}面板已运行（未自动加载代理内核）${plain}"
        else
            echo -e "${green}面板与代理内核已启动${plain}"
        fi

        # Never install Xray/proxy in the same critical window on tiny hosts.
        if [[ "$INSTALL_XRAY" -eq 1 ]]; then
            if [[ "$MEM_TOTAL_MB" -lt 1500 ]]; then
                echo -e "${yellow}内存 <1.5G：拒绝安装期装 Xray（面板稳定后用 --with-xray）${plain}"
            else
                install_xray || echo -e "${yellow}Xray-core 未安装${plain}"
            fi
        fi
        if [[ "$INSTALL_PROXY" -eq 1 ]]; then
            if [[ "$MEM_TOTAL_MB" -lt 1500 ]]; then
                echo -e "${yellow}内存 <1.5G：跳过安装期反代${plain}"
            else
                install_reverse_proxy || true
            fi
        fi
    else
        echo -e "${yellow}已按 --no-start 跳过启动。稍后：systemctl start s-ui${plain}"
    fi

    if [[ "$INSTALL_KIND" == "managed" ]]; then
        configure_managed_agent || exit 1
    fi

    echo -e "${green}s-ui ${last_version}${plain} 安装完成"
    local kind_label="轻量 Web 面板"
    [[ "$INSTALL_KIND" == "full" ]] && kind_label="全面服务端"
    [[ "$INSTALL_KIND" == "managed" ]] && kind_label="受管客户端"
    echo -e "安装摘要：方案=${green}${kind_label}${plain} 升级=${INSTALL_MODE} 档位=${PROFILE}"
    echo -e "  Xray=$(xray_summary_label) 反代=$(proxy_summary_label) Agent=$([ "$INSTALL_AGENT" -eq 1 ] && echo 是 || echo 否) 自动启内核=$([ "$SKIP_CORE" -eq 1 ] && echo 否 || echo 是) Swap=${SWAP_MB}MB"
    echo -e "访问：浏览器打开 ${green}$(panel_access_url)${plain}（默认 admin/admin）"
    if [[ "$PROXY_READY" -eq 1 ]]; then
        echo -e "${yellow}安全说明：2095 仅监听本机，请勿使用公网 IP:2095；公网入口由 ${PROXY_ENGINE} 提供。${plain}"
    fi
    if [[ "$SKIP_CORE" -eq 1 ]]; then
        echo -e "${yellow}安全模式：配置入站后在面板内重启内核再启用代理。${plain}"
    fi
    if [[ "$INSTALL_KIND" == "minimal" ]]; then
        echo -e "${yellow}当前为轻量 Web 面板，已包含完整可视化界面。若需全面服务端可重新执行：${plain}"
        echo -e "  bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh) -y --full --domain 你的域名"
    fi
    if [[ "$INSTALL_KIND" == "managed" ]]; then
        echo -e "${yellow}当前服务器既可从中心管理，也可直接登录本机 Web 面板操作。${plain}"
    fi
    echo -e ""
    echo -e "${yellow}若仍关机： free -h; swapon --show; dmesg | grep -iE 'oom|kill' | tail -30; journalctl -u s-ui -n 80 --no-pager${plain}"
}

# ---- main ----
if [[ "${SUI_INSTALL_SOURCE_ONLY:-0}" == "1" ]]; then
    return 0 2>/dev/null || exit 0
fi

parse_args "$@"
# root required for install (--help exits earlier in parse_args)
[[ $EUID -ne 0 ]] && echo -e "${red}致命错误：${plain}请使用 root 权限运行此脚本 \n " && exit 1
detect_os
echo -e "${green}1S-UI 安装程序${plain}"
echo -e "当前系统发行版为：${release}"
echo -e "架构：$(arch)"
if ! analyze_vps; then
    echo -e "${red}安装预检未通过，尚未下载或修改服务。${plain}"
    exit 1
fi

# Single safety preparation; it is a no-op when memory or existing Swap is sufficient.
if ! ensure_swap_if_needed; then
    echo -e "${red}低内存安全准备失败，已在下载/解压前停止。${plain}"
    echo -e "${yellow}--force 不会绕过 2核2G、OOM、Swap 或磁盘保护。${plain}"
    exit 1
fi

if ! install_base; then
    echo -e "${red}基础工具安装失败，已停止；不会继续进入下载和启动阶段。${plain}"
    exit 1
fi
install_s-ui
