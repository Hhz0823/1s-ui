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

INSTALL_KIND="minimal"
MEM_TOTAL_MB=1967
MEM_AVAIL_MB=1400
CPU_CORES=1
PROFILE="low"
FORCE_XRAY=0
FORCE_PROXY=0
FORCE_SKIP_CORE=0
FORCE_START_CORE=0
apply_kind_defaults >/dev/null
assert_eq 0 "$INSTALL_AGENT" "single-core minimal mode Agent exclusion"
assert_eq 0 "$SKIP_CORE" "single-core minimal mode core startup with sufficient memory"
assert_eq 1 "$DISABLE_XRAY" "single-core low profile Xray runtime guard"

INSTALL_KIND="minimal"
MEM_TOTAL_MB=1967
MEM_AVAIL_MB=1400
CPU_CORES=1
PROFILE="low"
FORCE_XRAY=1
FORCE_PROXY=0
FORCE_SKIP_CORE=0
FORCE_START_CORE=0
apply_kind_defaults >/dev/null
assert_eq 0 "$INSTALL_XRAY" "low profile ignores --with-xray"
assert_eq 1 "$DISABLE_XRAY" "low profile remains sing-box only"

INSTALL_KIND="minimal"
MEM_TOTAL_MB=1024
MEM_AVAIL_MB=700
CPU_CORES=1
PROFILE="low"
FORCE_XRAY=0
FORCE_PROXY=0
FORCE_SKIP_CORE=0
FORCE_START_CORE=0
apply_kind_defaults >/dev/null
assert_eq 0 "$SKIP_CORE" "1c1G minimal mode starts sing-box"
assert_eq 1 "$DISABLE_XRAY" "1c1G minimal mode keeps Xray disabled"
assert_eq 512 "$(core_start_budget_mb)" "1c1G sing-box startup budget"

FORCE_SKIP_CORE=1
apply_kind_defaults >/dev/null
assert_eq 1 "$SKIP_CORE" "explicit --skip-core remains panel-only"
assert_eq 384 "$(core_start_budget_mb)" "panel-only startup budget"
FORCE_SKIP_CORE=0

INSTALL_KIND="managed"
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
assert_eq 1 "$INSTALL_AGENT" "1c512 managed-client Agent enablement"
assert_eq 0 "$SKIP_CORE" "1c512 managed-client starts sing-box"
assert_eq 1 "$DISABLE_XRAY" "1c512 managed-client Xray runtime guard"
assert_eq 512 "$(core_start_budget_mb)" "1c512 managed-client startup budget"

INSTALL_KIND="managed"
CONTROLLER_URL=""
AGENT_TOKEN=""
if apply_kind_defaults >/dev/null; then
    fail "managed-client mode accepted missing controller credentials"
fi

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
grep -q 'setting -listen 127.0.0.1 -domain - -uri -' "$installer" \
    || fail "IP-only reverse proxy setup does not clear stale domain settings"

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
