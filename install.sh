#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
blue='\033[0;34m'
plain='\033[0m'

cur_dir=$(pwd)

# Decision flags
INSTALL_KIND=""              # client | full (minimal/managed remain aliases)
INSTALL_PANEL=1
INSTALL_XRAY=0
DISABLE_XRAY=0              # 1 = low-resource runtime cannot use Xray
XRAY_ON_DEMAND=0            # 1 = Xray is installed but never auto-started
INSTALL_PROXY=0
INSTALL_AGENT=1              # Agent files ship with every panel installation
CONFIGURE_AGENT=0            # bind/start Agent only when connection data is supplied
AGENT_WAS_ACTIVE=0
AGENT_WAS_ENABLED=0
AGENT_STATE="installed"
AGENT_ENV_FILE="/etc/default/1s-ui-agent"
AGENT_BINARY="/usr/local/s-ui/sui-agent"
AGENT_UNIT_FILE="/etc/systemd/system/s-ui-agent.service"
PROXY_ENGINE=""              # caddy | nginx | ""
PROXY_DOMAIN=""
PROXY_EMAIL=""
PROXY_READY=0                # 1 = reverse proxy was configured and is active
FRONTEND_ROOT="/usr/local/s-ui/frontend"
FRONTEND_RUNTIME_CONFIG="/usr/local/s-ui/frontend-runtime/config.js"
NGINX_FRONTEND_CONFIG="/etc/nginx/sites-available/s-ui-frontend.conf"
NGINX_PUBLIC_CONFIG="/etc/nginx/sites-available/s-ui-public.conf"
FRONTEND_LISTEN=""
FRONTEND_PORT=2095
FRONTEND_PATH="/app/"
FRONTEND_DOMAIN=""
SUBSCRIPTION_PORT=2096
API_LISTEN="127.0.0.1"
API_PORT=2097
FRONTEND_TARBALL=""
FRONTEND_INSTALL_PENDING=0
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
PUBLIC_IP=""
PUBLIC_IP_SOURCE=""          # override | external | local | ""
# Full/cluster recommendation
CLUSTER_CPU_CORES=2
CLUSTER_MEM_MB=2048
# Below the cluster recommendation the controller runs in lite mode, which
# uses sing-box only. Matches MinLiteControllerMemBytes in the panel.
LITE_CONTROLLER_CPU_CORES=1
LITE_CONTROLLER_MEM_MB=400
LITE_CONTROLLER=0            # 1 = full install on a host below 2c2G
SINGBOX_ONLY=0               # 1 = never download or start Xray-core

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

唯一推荐安装指令:
  bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh)

安装结果:
  完整 Web 面板 + sing-box + 休眠 Agent。首次进入 Web 后，向导会设置
  管理员、运行角色与可选主服务器连接；Agent 绑定主服务器后才启动。

以下参数仅为旧版自动化兼容，新安装无需使用:
  --minimal, --simple, -m   等同默认客户端安装
  --managed-client          等同默认安装，但要求同时提供 --connect 或旧式连接参数
  --full, --complete, --server
                            主控制端安装（面板 + Xray + 反代）；低于 2核2G 时
                            自动改为精简主控（仅 sing-box，代理与路由照常可用）

通用选项:
  -y, --yes             兼容旧命令；默认安装本身不再询问安装类型
  --with-xray           额外安装 Xray-core（低配也支持，但只按需启动）
  --no-xray             跳过 Xray-core
  --singbox-only        仅使用 sing-box 内核：不下载、不启动 Xray-core
  --with-proxy          安装反代（Caddy/Nginx）
  --no-proxy            不安装反代
  --domain DOMAIN       反代域名（HTTPS，多用于全面安装）
  --email EMAIL         ACME 邮箱（Caddy 可选）
  --controller URL      旧式中心面板 URL（需同时提供 --agent-token）
  --agent-token TOKEN   旧式 Agent 注册密钥（需同时提供 --controller）
  --connect URL         主面板生成的连接 API 或一次性连接地址（推荐）
  --agent-insecure      Agent 连接中心时跳过 TLS 证书校验
  --start-core          安装后自动启动代理内核
  --skip-core           仅面板 Web，不自动启内核（更安全）
  --no-start            只装文件，不 systemctl start
  --force               兼容旧命令；不能绕过主控制端 1核/400MB 的最低要求
  -h, --help            显示帮助

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
            INSTALL_KIND="client"
            shift
            ;;
        --full | --complete | --server)
            INSTALL_KIND="full"
            shift
            ;;
        --managed-client | --managed)
            INSTALL_KIND="client"
            CONFIGURE_AGENT=1
            shift
            ;;
        --client)
            INSTALL_KIND="client"
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
        --singbox-only | --sing-box-only)
            SINGBOX_ONLY=1
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
            CONFIGURE_AGENT=1
            shift 2
            ;;
        --agent-token)
            AGENT_TOKEN="${2:-}"
            CONFIGURE_AGENT=1
            shift 2
            ;;
        --connect)
            CONNECT_URL="${2:-}"
            CONFIGURE_AGENT=1
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

resolve_install_profile() {
    [[ -n "$INSTALL_KIND" ]] || INSTALL_KIND="client"
    case "$INSTALL_KIND" in
    minimal | managed)
        INSTALL_KIND="client"
        ;;
    client | full)
        ;;
    *)
        echo -e "${red}未知安装方案: ${INSTALL_KIND}${plain}"
        return 1
        ;;
    esac
}

validate_agent_connection() {
    [[ "$CONFIGURE_AGENT" -eq 1 ]] || return 0
    if [[ -n "$CONNECT_URL" ]]; then
        local endpoint code
        endpoint="${CONNECT_URL%%#*}"
        code="${CONNECT_URL#*#}"
        if [[ ! "$endpoint" =~ ^https?://[^[:space:]\'\"\\]+/agent/v1/(pair|enroll)$ ]] || [[ ! "$code" =~ ^[A-Za-z0-9_-]{32,128}$ ]]; then
            echo -e "${red}主服务器连接 API 或一次性地址格式无效。${plain}"
            return 1
        fi
        return 0
    fi
    if [[ ! "$CONTROLLER_URL" =~ ^https?://[^[:space:]\'\"\\]+$ ]]; then
        echo -e "${red}自动绑定需要 --connect，或有效的 --controller URL。${plain}"
        return 1
    fi
    if [[ ! "$AGENT_TOKEN" =~ ^[A-Za-z0-9_-]{32,128}$ ]]; then
        echo -e "${red}自动绑定需要 --connect，或有效的 --agent-token。${plain}"
        return 1
    fi
}

# Apply component defaults from INSTALL_KIND, then honor FORCE_* overrides.
apply_kind_defaults() {
    local xray_reason="" proxy_reason="" core_reason=""

    # Preserve a component choice made later in the Web panel. Upgrades must
    # not silently disable or remove an installed on-demand Xray runtime.
    local web_xray_enabled=0
    if [[ "$INSTALL_MODE" == "upgrade" && -f /usr/local/s-ui/db/.xray_enabled ]]; then
        web_xray_enabled=1
    fi

    DISABLE_XRAY=0
    XRAY_ON_DEMAND=0
    LITE_CONTROLLER=0
    if [[ "$PROFILE" == "low" || "$SINGBOX_ONLY" -eq 1 ]]; then
        DISABLE_XRAY=1
    fi

    INSTALL_AGENT=1
    validate_agent_connection || return 1

    if [[ "$INSTALL_KIND" == "full" ]]; then
        INSTALL_XRAY=1
        INSTALL_PROXY=1
        INSTALL_AGENT=1
        SKIP_CORE=0
        xray_reason="全面服务端：安装 Xray-core"
        proxy_reason="全面服务端：安装反代"
        core_reason="全面服务端：自动启动代理内核"
        # Below 2c2G the controller runs in lite mode with sing-box only.
        if [[ "$CPU_CORES" -lt "$CLUSTER_CPU_CORES" || "$MEM_TOTAL_MB" -lt "$CLUSTER_MEM_MB" ]]; then
            if [[ "$CPU_CORES" -lt "$LITE_CONTROLLER_CPU_CORES" || "$MEM_TOTAL_MB" -lt "$LITE_CONTROLLER_MEM_MB" ]]; then
                echo -e "${red}主控制端至少需要 ${LITE_CONTROLLER_CPU_CORES} 核 / ${LITE_CONTROLLER_MEM_MB}MB，当前 ${CPU_CORES} 核 / ${MEM_TOTAL_MB}MB。${plain}"
                echo -e "${yellow}该配置请直接使用默认客户端安装：bash install.sh${plain}"
                return 1
            fi
            LITE_CONTROLLER=1
            core_reason="精简主控：只启动 sing-box 内核"
        fi
    else
        INSTALL_KIND="client"
        INSTALL_XRAY=0
        INSTALL_PROXY=0
        SKIP_CORE=0
        core_reason="默认客户端：启动 Web 面板与轻量 sing-box"
        xray_reason="默认使用 sing-box（可用 --with-xray 安装按需 Xray）"
        proxy_reason="默认不安装反代，可稍后在面板设置中启用"
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
        # Preserve the old dual-core installation option without making a
        # 512MB VPS start two proxy processes at boot.
        if [[ "$PROFILE" == "low" || "$MEM_TOTAL_MB" -lt 1500 ]]; then
            DISABLE_XRAY=0
            XRAY_ON_DEMAND=1
            xray_reason="用户指定 --with-xray；低配仅按需启动，默认使用 sing-box"
        fi
    elif [[ "$FORCE_XRAY" == "0" ]]; then
        INSTALL_XRAY=0
        xray_reason="用户指定 --no-xray"
    fi

    if [[ "$web_xray_enabled" -eq 1 && "$FORCE_XRAY" != "0" ]]; then
        INSTALL_XRAY=0
        DISABLE_XRAY=0
        XRAY_ON_DEMAND=1
        xray_reason="保留面板中启用的现有 Xray-core（按需启动）"
    fi
    if [[ -z "$(xray_asset)" && "$INSTALL_XRAY" -eq 1 ]]; then
        INSTALL_XRAY=0
        xray_reason="当前架构无自动 Xray 包"
    fi
    if [[ "$LITE_CONTROLLER" -eq 1 || "$SINGBOX_ONLY" -eq 1 ]]; then
        DISABLE_XRAY=1
        XRAY_ON_DEMAND=0
    fi
    if [[ "$DISABLE_XRAY" -eq 1 ]]; then
        INSTALL_XRAY=0
        xray_reason="低配档位：仅使用 sing-box，禁止下载或启动 Xray-core"
        [[ "$SINGBOX_ONLY" -eq 1 ]] && xray_reason="用户指定 --singbox-only：仅使用 sing-box"
        [[ "$LITE_CONTROLLER" -eq 1 ]] && xray_reason="精简主控（低于 ${CLUSTER_CPU_CORES}核${CLUSTER_MEM_MB}MB）：仅使用 sing-box"
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
        if [[ "$INSTALL_XRAY" -eq 1 && "$FORCE_XRAY" != "1" ]]; then
            INSTALL_XRAY=0
            xray_reason="内存 <1.5G：默认不下载 Xray；如需双内核请显式 --with-xray"
        elif [[ "$INSTALL_XRAY" -eq 1 ]]; then
            XRAY_ON_DEMAND=1
            xray_reason="内存 <1.5G：已安装 Xray，但仅允许面板手动启动"
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
    if [[ "$LITE_CONTROLLER" -eq 1 ]]; then
        echo -e "模式：${green}精简主控制端 (--full，仅 sing-box；代理与路由功能照常可用)${plain}"
    elif [[ "$INSTALL_KIND" == "full" ]]; then
        echo -e "模式：${green}全面服务端 (--full)${plain}"
    else
        echo -e "模式：${green}统一客户端（默认）${plain}"
    fi
    echo -e "组件：Xray=$(xray_summary_label)  反代=$([ "$INSTALL_PROXY" -eq 1 ] && echo "是(${PROXY_ENGINE:-?})" || echo 否)  Agent=$([ "$CONFIGURE_AGENT" -eq 1 ] && echo 安装并绑定 || echo 安装但休眠)  自动启内核=$([ "$SKIP_CORE" -eq 1 ] && echo 否 || echo 是)"
    echo -e "  Xray：${xray_reason}"
    echo -e "  反代：${proxy_reason}"
    echo -e "  内核：${core_reason}"
    echo -e "${blue}==============================${plain}"

    if [[ "$INSTALL_KIND" == "full" && -z "$PROXY_DOMAIN" && "$INSTALL_PROXY" -eq 1 ]]; then
        echo -e "${yellow}反代将先使用服务器 IP 的 HTTP:80。安装后请在「面板设置 → 服务端面板」配置域名。${plain}"
    fi
}

analyze_vps() {
    detect_resources
    resolve_install_profile || return 1
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
    local xray_mode_line="Environment=SUI_XRAY_ON_DEMAND=false"
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
    if [[ "$XRAY_ON_DEMAND" -eq 1 ]]; then
        xray_mode_line="Environment=SUI_XRAY_ON_DEMAND=true"
        mkdir -p /usr/local/s-ui/db
        touch /usr/local/s-ui/db/.xray_on_demand
        chmod 644 /usr/local/s-ui/db/.xray_on_demand
    else
        rm -f /usr/local/s-ui/db/.xray_on_demand
    fi

    # Installer owns optimize.conf. The later 99-xray-runtime.conf is owned by
    # the Web component manager and intentionally takes precedence.
    if [[ "$INSTALL_MODE" == "fresh" ]]; then
        rm -f /etc/systemd/system/s-ui.service.d/99-xray-runtime.conf
        rm -f /usr/local/s-ui/db/.xray_enabled
    fi
    if [[ "$INSTALL_KIND" == "full" ]]; then
        mkdir -p /usr/local/s-ui/db
        touch /usr/local/s-ui/db/.controller_mode
        chmod 600 /usr/local/s-ui/db/.controller_mode
    elif [[ "$INSTALL_MODE" == "fresh" ]]; then
        rm -f /usr/local/s-ui/db/.controller_mode
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
${xray_mode_line}
Environment=SUI_API_LISTEN=${API_LISTEN}
Environment=SUI_API_PORT=${API_PORT}
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
    elif [[ "$XRAY_ON_DEMAND" -eq 1 ]]; then
        echo -e "${yellow}低配双内核：默认使用 sing-box；Xray-core 仅在面板中手动启动${plain}"
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

# Xray-core release installed when GitHub cannot say which one is the latest;
# the panel installs the same one in that case.
XRAY_FALLBACK_VERSION="v26.3.27"

# github_latest_tag prints the latest stable tag of a GitHub repository: from
# the API, else from the /releases/latest redirect on github.com, which still
# answers where api.github.com is blocked or rate limited.
github_latest_tag() {
    local repo="$1" tag=""
    tag=$(curl -Ls --connect-timeout 10 --max-time 20 "https://api.github.com/repos/${repo}/releases/latest" 2>/dev/null | grep '"tag_name":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
    if [[ ! "$tag" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+ ]]; then
        tag=$(curl -sI --connect-timeout 10 --max-time 20 "https://github.com/${repo}/releases/latest" 2>/dev/null | tr -d '\r' | grep -i '^location:' | head -1 | sed -nE 's#.*/releases/tag/([^/?#[:space:]]+).*#\1#p')
    fi
    [[ "$tag" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+ ]] && echo "$tag"
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
    local tmp
    tmp=$(mktemp "/etc/caddy/.Caddyfile.s-ui.XXXXXX") || return 1
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
    } >"$tmp"
    chmod 0644 "$tmp"
    mv -f "$tmp" /etc/caddy/Caddyfile
}

write_nginx_config() {
    local domain="$1"
    local panel_port="${2:-2095}"
    mkdir -p /etc/nginx/sites-available /etc/nginx/sites-enabled 2>/dev/null || true
    local conf_file="$NGINX_PUBLIC_CONFIG"
    local server_name="_"
    [[ -n "$domain" ]] && server_name="$domain"

    local tmp
    tmp=$(mktemp "/etc/nginx/sites-available/.s-ui-public.XXXXXX") || return 1
    cat >"$tmp" <<EOF
# BEGIN 1S-UI MANAGED REVERSE PROXY
server {
    listen 80;
    listen [::]:80;
    server_name ${server_name};

    client_max_body_size 32m;

    location / {
        proxy_http_version 1.1;
        proxy_set_header Host \$http_host;
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
    chmod 0644 "$tmp"
    mv -f "$tmp" "$conf_file"
    if [[ -d /etc/nginx/sites-enabled ]]; then
        ln -sfn "$conf_file" /etc/nginx/sites-enabled/s-ui-public.conf
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
        uri="${scheme}://${domain}${FRONTEND_PATH}"
        /usr/local/s-ui/sui setting -listen 127.0.0.1 -domain "$domain" -uri "$uri" >/dev/null 2>&1 || true
    else
        /usr/local/s-ui/sui setting -listen 127.0.0.1 -domain - -uri - >/dev/null 2>&1 || true
    fi
    # Avoid restart storm; caller restarts once if needed.
}

valid_ipv4() {
    local ip="${1:-}"
    local IFS=.
    local octets=()
    read -r -a octets <<<"$ip"
    [[ "${#octets[@]}" -eq 4 ]] || return 1

    local octet
    for octet in "${octets[@]}"; do
        [[ "$octet" =~ ^[0-9]{1,3}$ ]] || return 1
        ((10#$octet <= 255)) || return 1
    done

    ((10#${octets[0]} < 224)) || return 1
    case "$ip" in
    0.* | 127.* | 169.254.*) return 1 ;;
    esac
    return 0
}

fetch_public_ipv4() {
    local endpoint raw candidate
    local endpoints=(
        "https://api.ipify.org"
        "https://api.ip.sb/ip"
    )
    for endpoint in "${endpoints[@]}"; do
        raw=""
        if command -v curl >/dev/null 2>&1; then
            raw=$(curl -4 -fsS --connect-timeout 2 --max-time 3 "$endpoint" 2>/dev/null || true)
        elif command -v wget >/dev/null 2>&1; then
            raw=$(wget -qO- --timeout=3 "$endpoint" 2>/dev/null || true)
        else
            return 1
        fi
        candidate=$(printf '%s' "$raw" | tr -d '[:space:]')
        if valid_ipv4 "$candidate"; then
            printf '%s' "$candidate"
            return 0
        fi
    done
    return 1
}

find_local_ipv4() {
    local candidate addresses
    candidate=""
    if command -v ip >/dev/null 2>&1; then
        candidate=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{ for (i = 1; i <= NF; i++) if ($i == "src") { print $(i + 1); exit } }' || true)
        if valid_ipv4 "$candidate"; then
            printf '%s' "$candidate"
            return 0
        fi
    fi

    addresses=""
    if command -v hostname >/dev/null 2>&1; then
        addresses=$(hostname -I 2>/dev/null || true)
    fi
    for candidate in $addresses; do
        if valid_ipv4 "$candidate"; then
            printf '%s' "$candidate"
            return 0
        fi
    done
    return 1
}

detect_public_ip() {
    PUBLIC_IP=""
    PUBLIC_IP_SOURCE=""

    local candidate="${SUI_PUBLIC_IP:-}"
    if valid_ipv4 "$candidate"; then
        PUBLIC_IP="$candidate"
        PUBLIC_IP_SOURCE="override"
        return 0
    fi

    candidate=$(fetch_public_ipv4 || true)
    if valid_ipv4 "$candidate"; then
        PUBLIC_IP="$candidate"
        PUBLIC_IP_SOURCE="external"
        return 0
    fi

    candidate=$(find_local_ipv4 || true)
    if valid_ipv4 "$candidate"; then
        PUBLIC_IP="$candidate"
        PUBLIC_IP_SOURCE="local"
        return 0
    fi
    return 1
}

panel_access_host() {
    if [[ -n "$PUBLIC_IP" ]]; then
        printf '%s' "$PUBLIC_IP"
    else
        printf '<服务器公网IP>'
    fi
}

panel_access_url() {
    if [[ "$PROXY_READY" -eq 1 ]]; then
        if [[ -n "$PROXY_DOMAIN" ]]; then
            if [[ "$PROXY_ENGINE" == "caddy" ]]; then
                printf 'https://%s%s' "$PROXY_DOMAIN" "$FRONTEND_PATH"
            else
                printf 'http://%s%s' "$PROXY_DOMAIN" "$FRONTEND_PATH"
            fi
        else
            printf 'http://%s%s' "$(panel_access_host)" "$FRONTEND_PATH"
        fi
        return
    fi
    printf 'http://%s:%s%s' "$(panel_access_host)" "$FRONTEND_PORT" "$FRONTEND_PATH"
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
        [[ "$XRAY_ON_DEMAND" -eq 1 ]] && echo "是（按需）" || echo "是"
    elif [[ "$DISABLE_XRAY" -eq 1 ]]; then
        echo "否（低配仅 sing-box）"
    else
        echo "否"
    fi
}

agent_summary_label() {
    case "$AGENT_STATE" in
    connected)
        printf '已连接主服务器'
        ;;
    restored)
        printf '已恢复原连接'
        ;;
    configured)
        printf '已配置（未启动）'
        ;;
    *)
        printf '已安装（未绑定休眠）'
        ;;
    esac
}

install_reverse_proxy() {
    PROXY_READY=0
    if [[ "$INSTALL_PROXY" -ne 1 ]]; then
        echo -e "${yellow}按预检结果跳过反代安装。${plain}"
        return 0
    fi

    local panel_port="$FRONTEND_PORT"

    echo -e "${yellow}正在配置反向代理（引擎: ${PROXY_ENGINE}）...${plain}"
    bind_panel_localhost "$PROXY_DOMAIN"
    FRONTEND_LISTEN="127.0.0.1"
    FRONTEND_DOMAIN="$PROXY_DOMAIN"
    if ! configure_frontend_gateway; then
        echo -e "${red}前端网关切换本机监听失败，已停止公网反代配置${plain}"
        return 1
    fi

    if [[ "$PROXY_ENGINE" == "caddy" ]]; then
        if ! install_caddy_pkg; then
            echo -e "${yellow}Caddy 安装失败，回退 Nginx${plain}"
            PROXY_ENGINE="nginx"
        else
            local caddy_backup="/tmp/s-ui-caddy.public.backup"
            rm -f "$caddy_backup"
            [[ -f /etc/caddy/Caddyfile ]] && cp -f /etc/caddy/Caddyfile "$caddy_backup"
            write_caddy_config "$PROXY_DOMAIN" "$PROXY_EMAIL" "$panel_port"
            if ! caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile >/dev/null 2>&1; then
                [[ -f "$caddy_backup" ]] && cp -f "$caddy_backup" /etc/caddy/Caddyfile || rm -f /etc/caddy/Caddyfile
                echo -e "${yellow}Caddy 配置校验失败，回退 Nginx${plain}"
                PROXY_ENGINE="nginx"
            else
            systemctl enable caddy >/dev/null 2>&1 || true
            if systemctl restart caddy && systemctl is-active --quiet caddy; then
                rm -f "$caddy_backup"
                PROXY_READY=1
                echo -e "${green}Caddy 反代已启动${plain}"
                if [[ -n "$PROXY_DOMAIN" ]]; then
                    echo -e "面板地址：${green}https://${PROXY_DOMAIN}${FRONTEND_PATH}${plain}"
                else
                    echo -e "面板地址：${green}$(panel_access_url)${plain}（80 端口）"
                fi
                return 0
            fi
            [[ -f "$caddy_backup" ]] && cp -f "$caddy_backup" /etc/caddy/Caddyfile || rm -f /etc/caddy/Caddyfile
            systemctl try-restart caddy >/dev/null 2>&1 || true
            echo -e "${yellow}Caddy 启动失败，回退 Nginx${plain}"
            PROXY_ENGINE="nginx"
            fi
        fi
    fi

    if [[ "$PROXY_ENGINE" == "nginx" ]]; then
        if ! install_package nginx; then
            echo -e "${red}Nginx 安装失败，跳过反代${plain}"
            /usr/local/s-ui/sui setting -listen - >/dev/null 2>&1 || true
            FRONTEND_LISTEN=""
            FRONTEND_DOMAIN=""
            configure_frontend_gateway || true
            return 1
        fi
        local nginx_public_backup="/tmp/s-ui-nginx.public.backup"
        rm -f "$nginx_public_backup"
        [[ -f "$NGINX_PUBLIC_CONFIG" ]] && cp -f "$NGINX_PUBLIC_CONFIG" "$nginx_public_backup"
        write_nginx_config "$PROXY_DOMAIN" "$panel_port"
        if ! nginx -t; then
            [[ -f "$nginx_public_backup" ]] && cp -f "$nginx_public_backup" "$NGINX_PUBLIC_CONFIG" || {
                rm -f "$NGINX_PUBLIC_CONFIG" /etc/nginx/sites-enabled/s-ui-public.conf
            }
            nginx -t >/dev/null 2>&1 && systemctl reload nginx >/dev/null 2>&1 || true
            echo -e "${red}Nginx 公网入口校验失败，已回滚${plain}"
            return 1
        fi
        systemctl enable nginx >/dev/null 2>&1 || true
        if systemctl restart nginx && systemctl is-active --quiet nginx; then
            rm -f "$nginx_public_backup"
            PROXY_READY=1
            # Caddy fallback changes the public scheme from HTTPS to HTTP.
            bind_panel_localhost "$PROXY_DOMAIN"
            echo -e "${green}Nginx 反代已启动${plain}"
            if [[ -n "$PROXY_DOMAIN" ]]; then
                echo -e "HTTP：${green}http://${PROXY_DOMAIN}${FRONTEND_PATH}${plain}"
                echo -e "${yellow}提示：可用 certbot --nginx -d ${PROXY_DOMAIN} 配置 HTTPS${plain}"
            else
                echo -e "面板地址：${green}$(panel_access_url)${plain}（80 端口）"
            fi
            return 0
        fi
        [[ -f "$nginx_public_backup" ]] && cp -f "$nginx_public_backup" "$NGINX_PUBLIC_CONFIG" || {
            rm -f "$NGINX_PUBLIC_CONFIG" /etc/nginx/sites-enabled/s-ui-public.conf
        }
        nginx -t >/dev/null 2>&1 && systemctl reload nginx >/dev/null 2>&1 || true
        echo -e "${red}Nginx 启动失败，已恢复面板监听全部网卡${plain}"
        /usr/local/s-ui/sui setting -listen - >/dev/null 2>&1 || true
        FRONTEND_LISTEN=""
        FRONTEND_DOMAIN=""
        configure_frontend_gateway || true
        return 1
    fi
}

install_xray() {
    if [[ "$INSTALL_XRAY" -ne 1 ]]; then
        echo -e "${yellow}按预检结果跳过 Xray-core 安装（面板默认使用 sing-box）。${plain}"
        if [[ "$DISABLE_XRAY" -eq 1 ]]; then
            echo -e "${yellow}当前低配默认不下载 Xray；如需保留历史双内核方案，请重新执行并加 --with-xray。${plain}"
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
    xray_version=$(github_latest_tag "XTLS/Xray-core")
    if [[ ! -n "$xray_version" ]]; then
        xray_version="$XRAY_FALLBACK_VERSION"
        echo -e "${yellow}无法从 GitHub 获取 Xray-core 最新版本，改为安装经过测试的 ${xray_version}${plain}"
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

capture_agent_service_state() {
    AGENT_WAS_ACTIVE=0
    AGENT_WAS_ENABLED=0
    if systemctl is-active --quiet s-ui-agent 2>/dev/null; then
        AGENT_WAS_ACTIVE=1
    fi
    if systemctl is-enabled --quiet s-ui-agent 2>/dev/null; then
        AGENT_WAS_ENABLED=1
    fi
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
    if [[ ! -x "$AGENT_BINARY" || ! -f "$AGENT_UNIT_FILE" ]]; then
        echo -e "${red}受管客户端缺少 Agent 二进制或 systemd 服务。${plain}"
        return 1
    fi
    mkdir -p "$(dirname "$AGENT_ENV_FILE")"
    umask 077
    {
        printf 'SUI_AGENT_PANEL=%s\n' "$CONTROLLER_URL"
        printf 'SUI_AGENT_TOKEN=%s\n' "$AGENT_TOKEN"
        printf 'SUI_AGENT_INTERVAL=15s\n'
        printf 'SUI_AGENT_INSECURE=%s\n' "$([ "$AGENT_INSECURE" -eq 1 ] && echo true || echo false)"
        printf 'SUI_AGENT_LOCAL_SOCKET=/run/s-ui/control.sock\n'
    } >"$AGENT_ENV_FILE"
    chmod 0600 "$AGENT_ENV_FILE"
    systemctl daemon-reload
    if [[ "$START_SERVICE" -ne 1 ]]; then
        AGENT_STATE="configured"
        echo -e "${yellow}Agent 已配置但未启动；稍后执行 systemctl enable --now s-ui-agent${plain}"
        return 0
    fi
    local check=("$AGENT_BINARY" --panel "$CONTROLLER_URL" --token "$AGENT_TOKEN" --local-socket /run/s-ui/control.sock --interval 15s --once)
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
    AGENT_STATE="connected"
    echo -e "${green}受管客户端已连接中心面板，本地 Web 面板保持可独立使用。${plain}"
}

restore_agent_service_state() {
    if [[ "$CONFIGURE_AGENT" -eq 1 ]]; then
        configure_managed_agent
        return $?
    fi
    if [[ ! -f "$AGENT_ENV_FILE" ]]; then
        AGENT_STATE="installed"
        return 0
    fi
    if [[ ! -x "$AGENT_BINARY" || ! -f "$AGENT_UNIT_FILE" ]]; then
        echo -e "${yellow}检测到旧 Agent 配置，但当前发布包缺少 Agent 文件，跳过恢复。${plain}"
        AGENT_STATE="configured"
        return 0
    fi
    if [[ "$AGENT_WAS_ENABLED" -eq 1 ]]; then
        systemctl enable s-ui-agent >/dev/null 2>&1 || true
    fi
    if [[ "$START_SERVICE" -eq 1 && "$AGENT_WAS_ACTIVE" -eq 1 ]]; then
        if ! systemctl start s-ui-agent; then
            echo -e "${red}原有 Agent 连接恢复失败，请检查：journalctl -u s-ui-agent -n 80${plain}"
            return 1
        fi
        AGENT_STATE="restored"
        echo -e "${green}已恢复升级前的 Agent 连接。${plain}"
        return 0
    fi
    AGENT_STATE="configured"
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
    elif [[ "$DISABLE_XRAY" -eq 1 || "$INSTALL_XRAY" -eq 0 || "$XRAY_ON_DEMAND" -eq 1 ]]; then
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

download_frontend_release() {
    local version="$1"
    local url="https://github.com/Hhz0823/1s-ui/releases/download/${version}/s-ui-frontend.tar.gz"
    local out="/tmp/s-ui-frontend.tar.gz"
    FRONTEND_TARBALL=""
    rm -f "$out"
    echo -e "下载独立前端：${url}"
    if command -v curl >/dev/null 2>&1; then
        curl -fL --retry 3 --retry-delay 2 --connect-timeout 20 -o "$out" "$url" || return 1
    else
        wget -q --no-check-certificate -O "$out" "$url" || return 1
    fi
    local sz
    sz=$(stat -c%s "$out" 2>/dev/null || stat -f%z "$out" 2>/dev/null || echo 0)
    if [[ "${sz:-0}" -lt 10000 ]] || ! tar tzf "$out" >/dev/null 2>&1; then
        echo -e "${red}独立前端压缩包无效或下载不完整${plain}"
        rm -f "$out"
        return 1
    fi
    if tar tzf "$out" | awk '/(^|\/)\.\.($|\/)|^\// { bad=1 } END { exit bad ? 0 : 1 }'; then
        echo -e "${red}独立前端压缩包包含不安全路径${plain}"
        rm -f "$out"
        return 1
    fi
    FRONTEND_TARBALL="$out"
}

capture_frontend_entry() {
    FRONTEND_LISTEN=""
    FRONTEND_PORT=2095
	FRONTEND_PATH="/app/"
	FRONTEND_DOMAIN=""
	SUBSCRIPTION_PORT=2096
	[[ -x /usr/local/s-ui/sui && -f /usr/local/s-ui/db/s-ui.db ]] || return 0

    local output value
    output=$(GOMEMLIMIT=200MiB GOGC=40 /usr/local/s-ui/sui setting -show 2>/dev/null || true)
    value=$(awk -F'\t' '/Panel port/ {gsub(/[[:space:]]/, "", $NF); print $NF; exit}' <<<"$output")
    [[ "$value" =~ ^[0-9]+$ && "$value" -ge 1 && "$value" -le 65535 ]] && FRONTEND_PORT="$value"
    value=$(awk -F'\t' '/Panel path/ {gsub(/^[[:space:]]+|[[:space:]]+$/, "", $NF); print $NF; exit}' <<<"$output")
    [[ -n "$value" ]] && FRONTEND_PATH="$value"
    value=$(awk -F'\t' '/Panel IP/ {gsub(/^[[:space:]]+|[[:space:]]+$/, "", $NF); print $NF; exit}' <<<"$output")
    [[ -n "$value" ]] && FRONTEND_LISTEN="$value"
	value=$(awk -F'\t' '/Panel Domain/ {gsub(/^[[:space:]]+|[[:space:]]+$/, "", $NF); print $NF; exit}' <<<"$output")
	[[ -n "$value" ]] && FRONTEND_DOMAIN="$value"
	value=$(awk -F'\t' '/Sub port/ {gsub(/[[:space:]]/, "", $NF); print $NF; exit}' <<<"$output")
	[[ "$value" =~ ^[0-9]+$ && "$value" -ge 1 && "$value" -le 65535 ]] && SUBSCRIPTION_PORT="$value"
}

validate_frontend_entry() {
    [[ "$FRONTEND_PORT" =~ ^[0-9]+$ && "$FRONTEND_PORT" -ge 1 && "$FRONTEND_PORT" -le 65535 ]] || return 1
    [[ "$FRONTEND_PATH" == /* ]] || FRONTEND_PATH="/${FRONTEND_PATH}"
    [[ "$FRONTEND_PATH" == */ ]] || FRONTEND_PATH="${FRONTEND_PATH}/"
    [[ "$FRONTEND_PATH" =~ ^/[A-Za-z0-9._~%/-]*/$ && "$FRONTEND_PATH" != *//* ]] || return 1
	[[ "$FRONTEND_LISTEN" =~ ^[A-Fa-f0-9:.]*$ ]] || return 1
	[[ "$FRONTEND_DOMAIN" =~ ^[A-Za-z0-9.-]*$ ]] || return 1
	[[ "$SUBSCRIPTION_PORT" =~ ^[0-9]+$ && "$SUBSCRIPTION_PORT" -ge 1 && "$SUBSCRIPTION_PORT" -le 65535 ]] || return 1
}

select_api_port() {
	API_PORT=2097
	while [[ "$API_PORT" -le 2197 ]] && { [[ "$FRONTEND_PORT" -eq "$API_PORT" ]] || [[ "$SUBSCRIPTION_PORT" -eq "$API_PORT" ]] || port_in_use "$API_PORT"; }; do
		API_PORT=$((API_PORT + 1))
	done
	[[ "$API_PORT" -le 2197 ]] || return 1
}

install_frontend_files() {
    local stage="/usr/local/s-ui/frontend.new"
    local previous="/usr/local/s-ui/frontend.previous"
    rm -rf "$stage" "$previous"
    mkdir -p "$stage"
    tar xzf "$FRONTEND_TARBALL" -C "$stage"
    [[ -f "$stage/index.html" ]] || {
        echo -e "${red}独立前端产物缺少 index.html${plain}"
        rm -rf "$stage"
        return 1
    }
    [[ -d "$FRONTEND_ROOT" ]] && mv "$FRONTEND_ROOT" "$previous"
    mv "$stage" "$FRONTEND_ROOT"
    FRONTEND_INSTALL_PENDING=1
    rm -f "$FRONTEND_TARBALL"
}

write_frontend_runtime_config() {
    local runtime_dir
    runtime_dir=$(dirname "$FRONTEND_RUNTIME_CONFIG")
    mkdir -p "$runtime_dir"
    local tmp
    tmp=$(mktemp "${runtime_dir}/.config.js.XXXXXX") || return 1
    printf 'window.__SUI_CONFIG__ = Object.freeze({"basePath":"%s","backendUrl":""});\n' "$FRONTEND_PATH" >"$tmp"
    chmod 0644 "$tmp"
    mv -f "$tmp" "$FRONTEND_RUNTIME_CONFIG"
}

write_frontend_api_location() {
    local prefix="$1"
    cat <<EOF
    location ^~ ${prefix} {
        proxy_http_version 1.1;
        proxy_set_header Host \$http_host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$http_x_forwarded_proto;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 3600s;
        proxy_pass http://${API_LISTEN}:${API_PORT};
    }
EOF
}

write_frontend_gateway_config() {
    mkdir -p /etc/nginx/sites-available /etc/nginx/sites-enabled
    local tmp
    tmp=$(mktemp "/etc/nginx/sites-available/.s-ui-frontend.XXXXXX") || return 1
    local listen_lines server_name="_"
    [[ -n "$FRONTEND_DOMAIN" ]] && server_name="$FRONTEND_DOMAIN"
    if [[ -z "$FRONTEND_LISTEN" || "$FRONTEND_LISTEN" == "0.0.0.0" || "$FRONTEND_LISTEN" == "::" ]]; then
        listen_lines=$'    listen '"${FRONTEND_PORT}"$';\n    listen [::]:'"${FRONTEND_PORT}"';'
    elif [[ "$FRONTEND_LISTEN" == *:* ]]; then
        listen_lines="    listen [${FRONTEND_LISTEN//[\[\]]/}]:${FRONTEND_PORT};"
    else
        listen_lines="    listen ${FRONTEND_LISTEN}:${FRONTEND_PORT};"
    fi

    {
        echo "# BEGIN 1S-UI MANAGED FRONTEND GATEWAY"
        echo "server {"
        printf '%s\n' "$listen_lines"
        echo "    server_name ${server_name};"
        echo "    client_max_body_size 32m;"
        cat <<EOF

    location = /.well-known/1s-ui/config.js {
        alias ${FRONTEND_RUNTIME_CONFIG};
        default_type application/javascript;
        add_header Cache-Control "no-store" always;
    }

EOF
        write_frontend_api_location "/api/"
        write_frontend_api_location "/apiv2/"
        write_frontend_api_location "/agent/v1/"
        if [[ "$FRONTEND_PATH" != "/" ]]; then
            write_frontend_api_location "${FRONTEND_PATH}api/"
            write_frontend_api_location "${FRONTEND_PATH}apiv2/"
            write_frontend_api_location "${FRONTEND_PATH}agent/v1/"
            echo "    location = ${FRONTEND_PATH%/} { return 308 ${FRONTEND_PATH}; }"
        fi
        cat <<EOF
    location ^~ ${FRONTEND_PATH} {
        alias ${FRONTEND_ROOT}/;
        try_files \$uri \$uri/ ${FRONTEND_PATH}index.html;
        # Browsers re-check the UI after a panel update instead of showing a cached one.
        add_header Cache-Control "no-cache" always;
    }
}
# END 1S-UI MANAGED FRONTEND GATEWAY
EOF
    } >"$tmp"
    chmod 0644 "$tmp"
    mv -f "$tmp" "$NGINX_FRONTEND_CONFIG"
    ln -sfn "$NGINX_FRONTEND_CONFIG" /etc/nginx/sites-enabled/s-ui-frontend.conf
}

rollback_frontend_gateway() {
    local config_backup="$1"
    local runtime_backup="$2"
    local legacy_public_moved="${3:-}"
    [[ -f "$config_backup" ]] && cp -f "$config_backup" "$NGINX_FRONTEND_CONFIG" || {
        rm -f "$NGINX_FRONTEND_CONFIG" /etc/nginx/sites-enabled/s-ui-frontend.conf
    }
    [[ -f "$runtime_backup" ]] && cp -f "$runtime_backup" "$FRONTEND_RUNTIME_CONFIG" || rm -f "$FRONTEND_RUNTIME_CONFIG"
    if [[ -n "$legacy_public_moved" && -f "$NGINX_PUBLIC_CONFIG" ]]; then
        mv "$NGINX_PUBLIC_CONFIG" "$legacy_public_moved"
        rm -f /etc/nginx/sites-enabled/s-ui-public.conf
        [[ "$legacy_public_moved" == /etc/nginx/sites-available/* ]] &&
            ln -sfn "$legacy_public_moved" /etc/nginx/sites-enabled/s-ui.conf
    fi
    if [[ -d /usr/local/s-ui/frontend.previous ]]; then
        rm -rf "$FRONTEND_ROOT"
        mv /usr/local/s-ui/frontend.previous "$FRONTEND_ROOT"
    elif [[ "$FRONTEND_INSTALL_PENDING" -eq 1 ]]; then
        rm -rf "$FRONTEND_ROOT"
    fi
    nginx -t >/dev/null 2>&1 && systemctl reload nginx >/dev/null 2>&1 || true
}

configure_frontend_gateway() {
    if ! command -v nginx >/dev/null 2>&1 && ! install_package nginx; then
        echo -e "${red}nginx 安装失败，无法提供独立前端入口${plain}"
        if [[ -d /usr/local/s-ui/frontend.previous ]]; then
            rm -rf "$FRONTEND_ROOT"
            mv /usr/local/s-ui/frontend.previous "$FRONTEND_ROOT"
        elif [[ "$FRONTEND_INSTALL_PENDING" -eq 1 ]]; then
            rm -rf "$FRONTEND_ROOT"
        fi
        return 1
    fi
    local config_backup="/tmp/s-ui-frontend.conf.backup"
    local runtime_backup="/tmp/s-ui-config.js.backup"
    local legacy_public_moved=""
    rm -f "$config_backup" "$runtime_backup"
    [[ -f "$NGINX_FRONTEND_CONFIG" ]] && cp -f "$NGINX_FRONTEND_CONFIG" "$config_backup"
    [[ -f "$FRONTEND_RUNTIME_CONFIG" ]] && cp -f "$FRONTEND_RUNTIME_CONFIG" "$runtime_backup"
    if [[ -f "$NGINX_FRONTEND_CONFIG" ]] && ! grep -q '^# BEGIN 1S-UI MANAGED FRONTEND GATEWAY$' "$NGINX_FRONTEND_CONFIG"; then
        echo -e "${red}${NGINX_FRONTEND_CONFIG} 是自定义配置，安装器不会覆盖${plain}"
        rollback_frontend_gateway "$config_backup" "$runtime_backup"
        return 1
    fi
    if [[ ! -f "$NGINX_PUBLIC_CONFIG" ]]; then
        local legacy_public
        for legacy_public in /etc/nginx/sites-available/s-ui.conf /etc/nginx/conf.d/s-ui.conf; do
            [[ -f "$legacy_public" ]] || continue
            if grep -q '^# BEGIN 1S-UI MANAGED REVERSE PROXY$' "$legacy_public" &&
                grep -q 'proxy_pass http://127.0.0.1:' "$legacy_public"; then
                mv "$legacy_public" "$NGINX_PUBLIC_CONFIG"
                legacy_public_moved="$legacy_public"
                rm -f /etc/nginx/sites-enabled/s-ui.conf
                ln -sfn "$NGINX_PUBLIC_CONFIG" /etc/nginx/sites-enabled/s-ui-public.conf
                break
            fi
        done
    fi
    if ! write_frontend_runtime_config || ! write_frontend_gateway_config; then
        rollback_frontend_gateway "$config_backup" "$runtime_backup" "$legacy_public_moved"
        return 1
    fi
    if ! nginx -t; then
        rollback_frontend_gateway "$config_backup" "$runtime_backup" "$legacy_public_moved"
        echo -e "${red}nginx 前端网关校验失败，已回滚${plain}"
        return 1
    fi
    systemctl enable nginx >/dev/null 2>&1 || true
    if systemctl is-active --quiet nginx; then
        if ! systemctl reload nginx; then
            rollback_frontend_gateway "$config_backup" "$runtime_backup" "$legacy_public_moved"
            return 1
        fi
    else
        if ! systemctl start nginx; then
            rollback_frontend_gateway "$config_backup" "$runtime_backup" "$legacy_public_moved"
            return 1
        fi
    fi
    rm -rf /usr/local/s-ui/frontend.previous
    FRONTEND_INSTALL_PENDING=0
    rm -f "$config_backup" "$runtime_backup" /usr/local/s-ui/db/.frontend_apply_required
    return 0
}

install_s-ui() {
    cd /tmp/ || exit 1

	capture_frontend_entry
	if ! validate_frontend_entry; then
		echo -e "${red}现有前端 listen/port/webPath/webDomain 无法安全迁移${plain}"
		exit 1
    fi

    # ------------------------------------------------------------------
    # OOM root-cause controls (see docs/oom-reboot-analysis.md):
    #  - Panel binary ~90MB; each `sui` CLI invocation is a full process.
    #  - Fresh install: migrate is a no-op (no DB) but still costs full RSS.
    #  - Critical path must launch the 90MB binary AT MOST ONCE (systemd start).
    # ------------------------------------------------------------------

    echo -e "${yellow}安装前停止旧服务并准备 Swap（防 OOM 重启）...${plain}"
	capture_agent_service_state
	systemctl stop s-ui 2>/dev/null || true
	systemctl stop s-ui-agent 2>/dev/null || true
	if ! select_api_port; then
		echo -e "${red}未找到与面板、订阅及现有服务隔离的内部 API 端口${plain}"
		systemctl start s-ui 2>/dev/null || true
		restore_agent_service_state
		exit 1
	fi
    free -h 2>/dev/null || true
    swapon --show 2>/dev/null || true
    require_install_disk_budget 384 || exit 1

    local last_version
    if [[ -z "$REQUESTED_VERSION" ]]; then
        last_version=$(github_latest_tag "Hhz0823/1s-ui")
        if [[ ! -n "$last_version" ]]; then
            last_version=$(curl -Ls "https://api.github.com/repos/Hhz0823/1s-ui/releases?per_page=5" | grep '"tag_name":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
        fi
        if [[ ! -n "$last_version" ]]; then
            echo -e "${red}获取 s-ui 版本失败：api.github.com 和 github.com 都无法访问，请稍后重试，或在命令末尾加上版本号（例如 v1.6.3）${plain}"
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
    if ! download_frontend_release "$last_version"; then
        echo -e "${red}下载独立前端 s-ui-frontend.tar.gz 失败，未继续升级${plain}"
        rm -f "$tarball"
        exit 1
    fi

    systemctl stop s-ui 2>/dev/null || true
    rm -rf /tmp/s-ui-extract
    mkdir -p /tmp/s-ui-extract

    # Every panel receives the same base package. Agent stays dormant until a
    # controller connection is configured, so idle installs add no process.
    local members=(s-ui/sui s-ui/s-ui.service s-ui/s-ui.sh s-ui/sui-agent s-ui/s-ui-agent.service)
    echo -e "${yellow}解压统一客户端基础包（Web + sing-box + 休眠 Agent）...${plain}"
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

    if ! install_frontend_files; then
        echo -e "${red}独立前端安装失败${plain}"
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
    if [[ -f "$src_dir/sui-agent" ]]; then
        cp -f "$src_dir/sui-agent" "$AGENT_BINARY"
        chmod +x "$AGENT_BINARY"
        [[ -f "$src_dir/s-ui-agent.service" ]] && cp -f "$src_dir/s-ui-agent.service" "$AGENT_UNIT_FILE"
        echo -e "${green}已安装 sui-agent；未绑定主服务器时不会启动${plain}"
    else
        echo -e "${yellow}当前发布包不含 sui-agent，面板仍可独立运行${plain}"
    fi
    rm -rf /tmp/s-ui-extract

    if ! configure_frontend_gateway; then
        echo -e "${red}独立前端网关配置失败，后端未启动${plain}"
        exit 1
    fi

    # systemd: SUI_SKIP_CORE + GOMEMLIMIT before first start
    apply_systemd_optimize
    prepare_services

    local has_db=0
    [[ -f /usr/local/s-ui/db/s-ui.db ]] && has_db=1

    # ---- CLI policy (critical) ----
    # Fresh install: DO NOT run `sui migrate` or `sui admin`.
    #   migrate() exits immediately when DB missing, but still loads ~90MB RSS.
    #   The first browser visit creates the administrator through Web setup.
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
        echo -e "${yellow}首次打开面板将在 Web 页面创建管理员账号和密码。${plain}"
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

        # Optional components are prepared only after the panel has started;
        # low-resource Xray remains on-demand and is never started here.
        if [[ "$INSTALL_XRAY" -eq 1 ]]; then
            if [[ "$MEM_TOTAL_MB" -lt 1500 && "$FORCE_XRAY" != "1" ]]; then
                echo -e "${yellow}内存 <1.5G：默认跳过 Xray；需要双内核请显式 --with-xray${plain}"
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

    restore_agent_service_state || exit 1

    echo -e "${green}s-ui ${last_version}${plain} 安装完成"
    local kind_label="统一客户端"
    [[ "$INSTALL_KIND" == "full" ]] && kind_label="全面服务端"
    echo -e "安装摘要：方案=${green}${kind_label}${plain} 升级=${INSTALL_MODE} 档位=${PROFILE}"
    echo -e "  Xray=$(xray_summary_label) 反代=$(proxy_summary_label) Agent=$(agent_summary_label) 自动启内核=$([ "$SKIP_CORE" -eq 1 ] && echo 否 || echo 是) Swap=${SWAP_MB}MB"
    echo -e "访问：浏览器打开 ${green}$(panel_access_url)${plain}（首次访问创建管理员）"
    if [[ "$PROXY_READY" -eq 1 ]]; then
        echo -e "${yellow}安全说明：${FRONTEND_PORT} 仅监听本机，请勿使用公网 IP:${FRONTEND_PORT}；公网入口由 ${PROXY_ENGINE} 提供。${plain}"
    fi
    if [[ "$SKIP_CORE" -eq 1 ]]; then
        echo -e "${yellow}安全模式：配置入站后在面板内重启内核再启用代理。${plain}"
    fi
    if [[ "$AGENT_STATE" == "connected" || "$AGENT_STATE" == "restored" ]]; then
        echo -e "${yellow}当前服务器既可从中心管理，也可直接登录本机 Web 面板操作。${plain}"
    else
        echo -e "${yellow}稍后可在「服务器监控 → 连接主服务器」粘贴连接 API 完成绑定。${plain}"
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

detect_public_ip || true
case "$PUBLIC_IP_SOURCE" in
override)
    echo -e "${green}面板访问地址使用指定公网 IPv4：${PUBLIC_IP}${plain}"
    ;;
external)
    echo -e "${green}已自动识别服务器公网 IPv4：${PUBLIC_IP}${plain}"
    ;;
local)
    echo -e "${yellow}公网 IPv4 查询不可用，暂用本机地址：${PUBLIC_IP}；若服务器经过 NAT，请改用实际公网 IP。${plain}"
    ;;
*)
    echo -e "${yellow}未能自动识别公网 IPv4，安装完成后访问地址会保留 <服务器公网IP> 提示。${plain}"
    ;;
esac
install_s-ui
