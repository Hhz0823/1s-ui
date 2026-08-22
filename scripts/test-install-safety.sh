#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
installer="$repo_root/install.sh"
tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/s-ui-install-safety.XXXXXX")
trap 'rm -rf "$tmp_dir"' EXIT

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

assert_eq() {
    local want="$1" got="$2" label="$3"
    [[ "$got" == "$want" ]] || fail "$label: want=$want got=$got"
}

mkdir -p "$tmp_dir/bin" "$tmp_dir/state" "$tmp_dir/swap"
export TEST_COMMAND_LOG="$tmp_dir/commands.log"
export TEST_DF_FREE_MB=2048

cat >"$tmp_dir/bin/df" <<'EOF'
#!/bin/sh
printf 'Filesystem 1048576-blocks Used Available Capacity Mounted on\n'
printf 'testfs 4096 1024 %s 25%% /\n' "${TEST_DF_FREE_MB:?}"
EOF

cat >"$tmp_dir/bin/fallocate" <<'EOF'
#!/bin/sh
printf 'fallocate %s %s %s\n' "$1" "$2" "$3" >>"$TEST_COMMAND_LOG"
: >"$3"
EOF

cat >"$tmp_dir/bin/mkswap" <<'EOF'
#!/bin/sh
printf 'mkswap %s\n' "$1" >>"$TEST_COMMAND_LOG"
exit 0
EOF

cat >"$tmp_dir/bin/swapon" <<'EOF'
#!/bin/sh
printf 'swapon %s\n' "$*" >>"$TEST_COMMAND_LOG"
exit 0
EOF

cat >"$tmp_dir/bin/swapoff" <<'EOF'
#!/bin/sh
printf 'UNSAFE swapoff %s\n' "$*" >>"$TEST_COMMAND_LOG"
exit 99
EOF

cat >"$tmp_dir/bin/chattr" <<'EOF'
#!/bin/sh
exit 0
EOF

cat >"$tmp_dir/bin/systemctl" <<'EOF'
#!/bin/sh
last=""
for arg in "$@"; do
    last="$arg"
done
case "$1:$last" in
is-active:s-ui-agent)
    [ "${TEST_AGENT_ACTIVE:-0}" = 1 ]
    ;;
is-enabled:s-ui-agent)
    [ "${TEST_AGENT_ENABLED:-0}" = 1 ]
    ;;
is-active:* | is-enabled:* | list-unit-files:*)
    exit 1
    ;;
*)
    printf 'systemctl %s\n' "$*" >>"$TEST_COMMAND_LOG"
    exit 0
    ;;
esac
EOF

chmod +x "$tmp_dir/bin/"*
export PATH="$tmp_dir/bin:$PATH"

export SUI_INSTALL_SOURCE_ONLY=1
# shellcheck source=/dev/null
source "$installer"
unset SUI_INSTALL_SOURCE_ONLY

MEMINFO_FILE="$tmp_dir/meminfo"
PROC_SWAPS_FILE="$tmp_dir/swaps"
FSTAB_FILE="$tmp_dir/fstab"
MANAGED_SWAP_FILE="$tmp_dir/swap/swapfile"
CGROUP_V2_MEMORY_MAX_FILE="$tmp_dir/cgroup-memory.max"
CGROUP_V2_MEMORY_CURRENT_FILE="$tmp_dir/cgroup-memory.current"
CGROUP_V2_SWAP_MAX_FILE="$tmp_dir/cgroup-swap.max"
CGROUP_V2_SWAP_CURRENT_FILE="$tmp_dir/cgroup-swap.current"
CGROUP_V1_MEMORY_MAX_FILE="$tmp_dir/missing-v1-max"
CGROUP_V1_MEMORY_CURRENT_FILE="$tmp_dir/missing-v1-current"

cat >"$MEMINFO_FILE" <<'EOF'
MemTotal:        1048576 kB
MemAvailable:     524288 kB
SwapTotal:        262144 kB
SwapFree:         131072 kB
EOF
legacy_swap="$tmp_dir/legacy-swap"
printf 'must-not-change\n' >"$legacy_swap"
legacy_checksum=$(cksum "$legacy_swap")
{
    echo "Filename Type Size Used Priority"
    echo "$legacy_swap file 262140 131068 -2"
} >"$PROC_SWAPS_FILE"
: >"$FSTAB_FILE"
: >"$TEST_COMMAND_LOG"

MEM_TOTAL_MB=1024
MEM_AVAIL_MB=512
SWAP_MB=256
SWAP_PREPARED=0
CGROUP_SWAP_BLOCKED=0
ensure_swap_if_needed

[[ -f "$MANAGED_SWAP_FILE" ]] || fail "supplemental swap file was not created"
grep -q "fallocate -l 768M $MANAGED_SWAP_FILE.partial" "$TEST_COMMAND_LOG" \
    || fail "expected a 768MB supplemental swap"
grep -q "swapon $MANAGED_SWAP_FILE" "$TEST_COMMAND_LOG" \
    || fail "supplemental swap was not enabled"
if grep -q "UNSAFE swapoff" "$TEST_COMMAND_LOG"; then
    fail "installer attempted to disable existing swap"
fi
assert_eq "$legacy_checksum" "$(cksum "$legacy_swap")" "existing swap preservation"
grep -q "^$MANAGED_SWAP_FILE none swap sw 0 0$" "$FSTAB_FILE" \
    || fail "supplemental swap was not persisted"
assert_eq 1024 "$SWAP_MB" "total swap accounting"

rm -f "$MANAGED_SWAP_FILE"
: >"$FSTAB_FILE"
: >"$TEST_COMMAND_LOG"
export TEST_DF_FREE_MB=700
MEM_TOTAL_MB=512
SWAP_MB=0
SWAP_PREPARED=0
if ensure_swap_if_needed; then
    fail "low-disk swap creation should have been rejected"
fi
[[ ! -e "$MANAGED_SWAP_FILE" ]] || fail "low-disk path created a swap file"
if grep -q '^swapon ' "$TEST_COMMAND_LOG"; then
    fail "low-disk path attempted to enable swap"
fi

cat >"$MEMINFO_FILE" <<'EOF'
MemTotal:        8388608 kB
MemAvailable:    7340032 kB
SwapTotal:       2097152 kB
SwapFree:        2097152 kB
EOF
printf '536870912\n' >"$CGROUP_V2_MEMORY_MAX_FILE"
printf '134217728\n' >"$CGROUP_V2_MEMORY_CURRENT_FILE"
printf '0\n' >"$CGROUP_V2_SWAP_MAX_FILE"
export TEST_DF_FREE_MB=4096
CGROUP_MEMORY_LIMIT_MB=0
CGROUP_SWAP_BLOCKED=0
detect_resources
assert_eq 512 "$MEM_TOTAL_MB" "cgroup memory limit"
assert_eq 384 "$MEM_AVAIL_MB" "cgroup available memory"
assert_eq 0 "$SWAP_MB" "cgroup-disabled swap"
assert_eq 1 "$CGROUP_SWAP_BLOCKED" "cgroup swap guard"
FORCE_INSTALL=1
if require_mem_budget 400; then
    fail "--force must not bypass the OOM startup guard"
fi
FORCE_INSTALL=0
SWAP_PREPARED=0
if ensure_swap_if_needed; then
    fail "cgroup-disabled swap path should stop safely"
fi

INSTALL_KIND="full"
CONFIGURE_AGENT=0
CONNECT_URL=""
CONTROLLER_URL=""
AGENT_TOKEN=""
MEM_TOTAL_MB=1024
MEM_AVAIL_MB=384
SWAP_MB=1024
CPU_CORES=2
PROFILE="standard"
INSTALL_MODE="fresh"
PORT80_FREE=1
AUTO_YES=1
FORCE_INSTALL=1
FORCE_XRAY=""
FORCE_PROXY=""
FORCE_SKIP_CORE=0
FORCE_START_CORE=0
if apply_kind_defaults >/dev/null; then
    fail "--force bypassed the full-server 2c2G hard gate"
fi

INSTALL_KIND="full"
MEM_TOTAL_MB=2048
MEM_AVAIL_MB=1536
CPU_CORES=2
FORCE_INSTALL=0
FORCE_XRAY=0
FORCE_PROXY=0
FORCE_SKIP_CORE=0
FORCE_START_CORE=0
apply_kind_defaults >/dev/null
assert_eq 1 "$INSTALL_AGENT" "2c2G full mode Agent enablement"
assert_eq 0 "$SKIP_CORE" "2c2G full mode core startup"

INSTALL_KIND=""
MEM_TOTAL_MB=1967
MEM_AVAIL_MB=1400
CPU_CORES=1
PROFILE="low"
FORCE_XRAY=0
FORCE_PROXY=0
FORCE_SKIP_CORE=0
FORCE_START_CORE=0
resolve_install_profile
apply_kind_defaults >/dev/null
assert_eq "client" "$INSTALL_KIND" "default install profile"
assert_eq 1 "$INSTALL_AGENT" "default client ships dormant Agent"
assert_eq 0 "$CONFIGURE_AGENT" "default client does not bind Agent"
assert_eq 0 "$SKIP_CORE" "single-core client starts sing-box with sufficient memory"
assert_eq 1 "$DISABLE_XRAY" "single-core low profile Xray runtime guard"

INSTALL_KIND="client"
MEM_TOTAL_MB=1967
MEM_AVAIL_MB=1400
CPU_CORES=1
PROFILE="low"
FORCE_XRAY=1
FORCE_PROXY=0
FORCE_SKIP_CORE=0
FORCE_START_CORE=0
apply_kind_defaults >/dev/null
assert_eq 1 "$INSTALL_XRAY" "low profile honors --with-xray"
assert_eq 0 "$DISABLE_XRAY" "low profile enables optional Xray"
assert_eq 1 "$XRAY_ON_DEMAND" "low profile keeps optional Xray on-demand"
assert_eq 512 "$(core_start_budget_mb)" "low dual-core startup budget remains sing-box sized"

INSTALL_KIND="minimal"
MEM_TOTAL_MB=1024
MEM_AVAIL_MB=700
CPU_CORES=1
PROFILE="low"
FORCE_XRAY=0
FORCE_PROXY=0
FORCE_SKIP_CORE=0
FORCE_START_CORE=0
resolve_install_profile
apply_kind_defaults >/dev/null
assert_eq "client" "$INSTALL_KIND" "legacy minimal alias"
assert_eq 1 "$INSTALL_AGENT" "legacy minimal alias includes dormant Agent"
assert_eq 0 "$SKIP_CORE" "1c1G client starts sing-box"
assert_eq 1 "$DISABLE_XRAY" "1c1G client keeps Xray disabled"
assert_eq 512 "$(core_start_budget_mb)" "1c1G sing-box startup budget"

FORCE_SKIP_CORE=1
apply_kind_defaults >/dev/null
assert_eq 1 "$SKIP_CORE" "explicit --skip-core remains panel-only"
assert_eq 384 "$(core_start_budget_mb)" "panel-only startup budget"
FORCE_SKIP_CORE=0

INSTALL_KIND="client"
CONFIGURE_AGENT=1
MEM_TOTAL_MB=512
MEM_AVAIL_MB=320
CPU_CORES=1
PROFILE="low"
CONTROLLER_URL="https://panel.example.com/app/"
AGENT_TOKEN="abcdefghijklmnopqrstuvwxyzABCDEFGH12345678"
FORCE_XRAY=0
FORCE_PROXY=0
FORCE_SKIP_CORE=0
FORCE_START_CORE=0
apply_kind_defaults >/dev/null
assert_eq 1 "$INSTALL_AGENT" "1c512 connected client Agent package"
assert_eq 0 "$SKIP_CORE" "1c512 connected client starts sing-box"
assert_eq 1 "$DISABLE_XRAY" "1c512 connected client Xray runtime guard"
assert_eq 512 "$(core_start_budget_mb)" "1c512 connected client startup budget"

INSTALL_KIND="client"
CONFIGURE_AGENT=1
CONTROLLER_URL=""
AGENT_TOKEN=""
if apply_kind_defaults >/dev/null; then
    fail "automatic Agent binding accepted missing controller credentials"
fi

INSTALL_KIND=""
CONFIGURE_AGENT=0
CONNECT_URL=""
CONTROLLER_URL=""
AGENT_TOKEN=""
parse_args --managed-client --connect 'https://panel.example.com/app/agent/v1/enroll#abcdefghijklmnopqrstuvwxyzABCDEFGH12345678'
assert_eq "client" "$INSTALL_KIND" "legacy managed-client alias"
assert_eq 1 "$CONFIGURE_AGENT" "legacy managed-client enables automatic binding"

CONNECT_URL="https://panel.example.com/app/agent/v1/enroll#short"
if validate_agent_connection >/dev/null; then
    fail "automatic Agent binding accepted a malformed connection API"
fi
CONNECT_URL=""

AGENT_ENV_FILE="$tmp_dir/state/1s-ui-agent"
AGENT_BINARY="$tmp_dir/state/sui-agent"
AGENT_UNIT_FILE="$tmp_dir/state/s-ui-agent.service"
printf '#!/bin/sh\nexit 0\n' >"$AGENT_BINARY"
chmod +x "$AGENT_BINARY"
: >"$AGENT_UNIT_FILE"
: >"$AGENT_ENV_FILE"
: >"$TEST_COMMAND_LOG"
export TEST_AGENT_ACTIVE=1
export TEST_AGENT_ENABLED=1
capture_agent_service_state
assert_eq 1 "$AGENT_WAS_ACTIVE" "active Agent state capture"
assert_eq 1 "$AGENT_WAS_ENABLED" "enabled Agent state capture"
CONFIGURE_AGENT=0
START_SERVICE=1
restore_agent_service_state
assert_eq "restored" "$AGENT_STATE" "Agent state after upgrade restore"
grep -q '^systemctl enable s-ui-agent$' "$TEST_COMMAND_LOG" \
    || fail "upgrade did not restore Agent enablement"
grep -q '^systemctl start s-ui-agent$' "$TEST_COMMAND_LOG" \
    || fail "upgrade did not restart an active Agent"

rm -f "$AGENT_ENV_FILE"
AGENT_WAS_ACTIVE=0
AGENT_WAS_ENABLED=0
AGENT_STATE="configured"
restore_agent_service_state
assert_eq "installed" "$AGENT_STATE" "fresh Agent remains dormant"

if grep -Eq '请选择 \[1/2/3\]|确认按此方案安装|是否继续修改设置' "$installer"; then
    fail "default installer still contains SSH setup prompts"
fi
if grep -q 'config_after_install' "$installer"; then
    fail "default installer still calls the legacy SSH configuration flow"
fi
if grep -q '默认登录：admin / admin' "$installer"; then
    fail "installer still advertises a default Web password"
fi
grep -q '首次打开面板将在 Web 页面创建管理员账号和密码' "$installer" \
    || fail "installer does not direct fresh installs to first-run Web setup"
grep -q 'capture_agent_service_state' "$installer" \
    || fail "installer does not preserve the Agent service state before upgrade"
grep -q 'restore_agent_service_state' "$installer" \
    || fail "installer does not restore or configure Agent after upgrade"
grep -q '/usr/local/s-ui/db/.controller_mode' "$installer" \
    || fail "legacy full installs do not preserve controller mode"
grep -q 'web_xray_enabled' "$installer" \
    || fail "upgrades do not preserve the Web-managed Xray choice"
grep -q '99-xray-runtime.conf' "$installer" \
    || fail "installer does not respect the Web-managed Xray runtime override"

if grep -Eq '^[[:space:]]*swapoff([[:space:]]|$)' "$installer"; then
    fail "installer contains an executable swapoff command"
fi
if grep -q '/proc/sys/vm/drop_caches' "$installer"; then
    fail "installer still forces global page-cache drops"
fi
if grep -q 'bs=64M' "$installer"; then
    fail "installer still uses a 64MB dd buffer"
fi
if grep -q 'read -r -p "反代域名' "$installer"; then
    fail "interactive installer still asks for reverse proxy domain instead of using the server panel"
fi
grep -q '^Environment=SUI_SKIP_CORE=false$' "$repo_root/s-ui.service" \
    || fail "release service unit does not start sing-box by default"
grep -q '^Environment=SUI_API_LISTEN=127.0.0.1$' "$repo_root/s-ui.service" \
    || fail "release service unit does not bind the API backend to loopback"
grep -q '^Environment=SUI_API_PORT=2097$' "$repo_root/s-ui.service" \
    || fail "release service unit does not pin the internal API port"
grep -q 'setting -listen 127.0.0.1 -domain - -uri -' "$installer" \
    || fail "IP-only reverse proxy setup does not clear stale domain settings"
grep -q 's-ui-frontend.tar.gz' "$installer" \
    || fail "installer does not download the independent frontend artifact"
grep -q 's-ui-frontend.conf' "$installer" \
    || fail "installer does not keep the frontend gateway in its own nginx file"
grep -q 's-ui-public.conf' "$installer" \
    || fail "installer does not keep the public proxy in its own nginx file"
grep -q 'location = /.well-known/1s-ui/config.js' "$installer" \
    || fail "frontend gateway does not expose the fixed runtime config path"
grep -q 'add_header Cache-Control "no-store" always' "$installer" \
    || fail "frontend runtime config is cacheable"
grep -q 'rollback_frontend_gateway' "$installer" \
    || fail "frontend gateway changes have no rollback path"

FRONTEND_LISTEN="127.0.0.1"
FRONTEND_PORT=3095
FRONTEND_PATH="/panel/"
FRONTEND_DOMAIN="panel.example.com"
validate_frontend_entry || fail "valid persisted frontend entry was rejected"
FRONTEND_PATH='/bad path/'
if validate_frontend_entry; then
    fail "unsafe persisted frontend path was accepted"
fi
FRONTEND_PATH="/app/"
FRONTEND_LISTEN=""
FRONTEND_PORT=2095
FRONTEND_DOMAIN=""
SUBSCRIPTION_PORT=2096

port_in_use() { [[ "$1" -eq 2098 ]]; }
FRONTEND_PORT=2097
select_api_port || fail "API port selection failed"
assert_eq 2099 "$API_PORT" "API port skips frontend and occupied ports"
FRONTEND_PORT=2095
SUBSCRIPTION_PORT=2097
port_in_use() { return 1; }
select_api_port || fail "API port selection failed with subscription collision"
assert_eq 2098 "$API_PORT" "API port skips subscription port"
unset -f port_in_use
SUBSCRIPTION_PORT=2096

INSTALL_PROXY=0
PROXY_READY=0
PROXY_ENGINE=""
PROXY_DOMAIN=""
assert_eq "http://服务器IP:2095/app/" "$(panel_access_url)" "direct panel URL"
assert_eq "否" "$(proxy_summary_label)" "disabled proxy summary"

INSTALL_PROXY=1
PROXY_READY=1
PROXY_ENGINE="caddy"
PROXY_DOMAIN=""
assert_eq "http://服务器IP/app/" "$(panel_access_url)" "Caddy IP URL"
assert_eq "是(caddy，已启动)" "$(proxy_summary_label)" "active Caddy summary"

PROXY_DOMAIN="panel.example.com"
assert_eq "https://panel.example.com/app/" "$(panel_access_url)" "Caddy domain URL"

PROXY_ENGINE="nginx"
assert_eq "http://panel.example.com/app/" "$(panel_access_url)" "Nginx domain URL"

PROXY_READY=0
assert_eq "http://服务器IP:2095/app/" "$(panel_access_url)" "failed proxy fallback URL"
assert_eq "否（未启动）" "$(proxy_summary_label)" "failed proxy summary"

echo "PASS: installer swap, disk, and cgroup safety checks"
