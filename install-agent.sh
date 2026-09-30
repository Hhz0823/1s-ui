#!/usr/bin/env bash

set -euo pipefail

repo="Hhz0823/1s-ui"
panel_url=""
token=""
version=""
insecure="false"
connect_url=""
# Download line: auto (GitHub, or mainland China mirrors when GitHub is
# unreachable or slow), github, cn (mirrors first) or a mirror prefix URL.
download_line="${SUI_MIRROR:-auto}"
builtin_mirrors=("https://ghfast.top/" "https://gh-proxy.com/" "https://ghproxy.net/")
enroll_key=""
node_name=""

usage() {
    echo "Usage: install-agent.sh --connect CONTROLLER_API [--version VERSION] [--insecure] [--mirror auto|github|cn|URL]"
    echo "   or: install-agent.sh --panel URL --key ENROLLMENT_KEY [--name NAME] [--version VERSION] [--insecure] [--mirror auto|github|cn|URL]"
    echo "   or: install-agent.sh --panel URL --token TOKEN [--version VERSION] [--insecure] [--mirror auto|github|cn|URL]"
    echo
    echo "--panel URL --key KEY binds this server to the controller panel at URL using the"
    echo "reusable enrollment key shown in the panel. Works on fnOS (Feiniu NAS) and other"
    echo "Linux systems with systemd; run it with sudo from a normal admin login."
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --panel)
            panel_url="${2:-}"
            shift 2
            ;;
        --token)
            token="${2:-}"
            shift 2
            ;;
        --key)
            enroll_key="${2:-}"
            shift 2
            ;;
        --name)
            node_name="${2:-}"
            shift 2
            ;;
        --connect)
            connect_url="${2:-}"
            shift 2
            ;;
        --version)
            version="${2:-}"
            shift 2
            ;;
        --insecure)
            insecure="true"
            shift
            ;;
        --mirror)
            download_line="${2:-}"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

if [[ ${EUID} -ne 0 ]]; then
    echo "Run this installer as root (for example: sudo bash install-agent.sh ...)." >&2
    exit 1
fi
if [[ "$(uname -s)" != "Linux" ]] || ! command -v systemctl >/dev/null 2>&1; then
    echo "The 1S-UI Agent installer requires Linux with systemd." >&2
    exit 1
fi
if [[ -n "$enroll_key" ]]; then
    if [[ -n "$connect_url" || -n "$token" ]]; then
        echo "--key cannot be combined with --connect or --token." >&2
        exit 2
    fi
    if [[ ! "$panel_url" =~ ^https?://[^[:space:]\'\"\\#]+$ ]]; then
        echo "--key needs the controller panel address in --panel, for example https://panel.example.com:2095/app/" >&2
        exit 1
    fi
    if [[ ! "$enroll_key" =~ ^[A-Za-z0-9_-]{32,128}$ ]]; then
        echo "Invalid enrollment key. Copy it again from the controller panel." >&2
        exit 1
    fi
    connect_url="${panel_url%/}/agent/v1/enroll#${enroll_key}"
fi
if [[ -n "$connect_url" ]]; then
    if [[ "$connect_url" != http://* && "$connect_url" != https://* ]] || [[ "$connect_url" != *"#"* ]]; then
        echo "Invalid controller connection API or one-time address." >&2
        exit 1
    fi
else
    if [[ ! "$panel_url" =~ ^https?://[^[:space:]\'\"\\]+$ ]]; then
        echo "Invalid panel URL." >&2
        exit 1
    fi
    if [[ ! "$token" =~ ^[A-Za-z0-9_-]{32,128}$ ]]; then
        echo "Invalid enrollment token." >&2
        exit 1
    fi
fi

resolve_connection() {
    [[ -n "$connect_url" ]] || return 0
    local endpoint code response
    endpoint="${connect_url%%#*}"
    code="${connect_url#*#}"
    if [[ ! "$endpoint" =~ ^https?://[^[:space:]\'\"\\]+/agent/v1/(pair|enroll)$ ]] || [[ ! "$code" =~ ^[A-Za-z0-9_-]{32,128}$ ]]; then
        echo "Invalid controller connection API or one-time address." >&2
        return 1
    fi
    [[ -n "$node_name" ]] || node_name=$(hostname 2>/dev/null)
    node_name=$(printf '%s' "$node_name" | tr -d '\r\n' | sed 's/["\\]//g' | cut -c1-80)
    [[ -n "$node_name" ]] || node_name="managed-server"
    local curl_args=(--fail --silent --show-error --max-time 20)
    [[ "$insecure" == "true" ]] && curl_args+=(--insecure)
    echo "Connecting to the 1S-UI controller..."
    response=$(curl "${curl_args[@]}" -H 'Content-Type: application/json' --data "{\"code\":\"${code}\",\"name\":\"${node_name}\"}" "$endpoint") || {
        echo "The controller rejected or could not process the connection API." >&2
        [[ -n "$enroll_key" ]] && echo "Check the panel address and that the enrollment key was not regenerated or revoked." >&2
        return 1
    }
    panel_url=$(printf '%s' "$response" | sed -nE 's/.*"panel_url"[[:space:]]*:[[:space:]]*"([^\"]+)".*/\1/p' | head -n1)
    token=$(printf '%s' "$response" | sed -nE 's/.*"token"[[:space:]]*:[[:space:]]*"([^\"]+)".*/\1/p' | head -n1)
    if [[ ! "$panel_url" =~ ^https?://[^[:space:]\'\"\\]+$ ]] || [[ ! "$token" =~ ^[A-Za-z0-9_-]{32,128}$ ]]; then
        echo "The controller returned an invalid pairing response." >&2
        return 1
    fi
}

case "$(uname -m)" in
    x86_64|amd64) arch="amd64" ;;
    i386|i486|i586|i686) arch="386" ;;
    aarch64|arm64) arch="arm64" ;;
    armv7*|armhf) arch="armv7" ;;
    armv6*) arch="armv6" ;;
    armv5*) arch="armv5" ;;
    s390x) arch="s390x" ;;
    *)
        echo "Unsupported architecture: $(uname -m)" >&2
        exit 1
        ;;
esac

sources=()
case "$download_line" in
    github) sources=("") ;;
    cn) sources=("${builtin_mirrors[@]}" "") ;;
    http://*|https://*)
        [[ "$download_line" == */ ]] || download_line="${download_line}/"
        sources=("$download_line" "" "${builtin_mirrors[@]}")
        ;;
    *)
        if curl -sI -o /dev/null --connect-timeout 5 --max-time 8 "https://github.com/${repo}/releases/latest"; then
            sources=("" "${builtin_mirrors[@]}")
        else
            echo "GitHub is unreachable; using the mainland China mirrors."
            sources=("${builtin_mirrors[@]}" "")
        fi
        ;;
esac

if [[ -z "$version" ]]; then
    version=$(curl -fsSL --connect-timeout 10 --max-time 20 "https://api.github.com/repos/${repo}/releases/latest" 2>/dev/null | sed -nE 's/.*"tag_name":[[:space:]]*"([^"]+)".*/\1/p' | head -n1 || true)
    if [[ -z "$version" ]]; then
        for source in "${sources[@]}"; do
            version=$(curl -sL -D - --connect-timeout 10 --max-time 20 "${source}https://github.com/${repo}/releases/latest" 2>/dev/null | tr -d '\r' | grep -oE "/${repo}/releases/tag/[^/?#\"'<> ]+" | head -n1 | sed -E 's#.*/releases/tag/##' || true)
            [[ -n "$version" ]] && break
        done
    fi
fi
[[ "$version" == v* ]] || version="v${version}"
if [[ ! "$version" =~ ^v[0-9A-Za-z._-]+$ ]]; then
    echo "Invalid release version." >&2
    exit 1
fi

tmp_dir=$(mktemp -d /tmp/1s-ui-agent.XXXXXX)
trap 'rm -rf "$tmp_dir"' EXIT
archive="$tmp_dir/s-ui.tar.gz"
asset="s-ui-linux-${arch}.tar.gz"
url="https://github.com/${repo}/releases/download/${version}/${asset}"

# Each source but the last is dropped when it stays under 64KB/s for 20s.
echo "Downloading 1S-UI Agent ${version} for ${arch}..."
downloaded="false"
index=0
for source in "${sources[@]}"; do
    index=$((index + 1))
    curl_args=(--fail --location --connect-timeout 15 --output "$archive")
    if [[ $index -lt ${#sources[@]} ]]; then
        curl_args+=(--speed-limit 65536 --speed-time 20)
    else
        curl_args+=(--retry 3)
    fi
    echo "Source: ${source:-https://github.com/}"
    if curl "${curl_args[@]}" "${source}${url}"; then
        downloaded="true"
        break
    fi
done
if [[ "$downloaded" != "true" ]]; then
    echo "Download failed from GitHub and every mirror. Retry with --mirror URL." >&2
    exit 1
fi

# Releases list their SHA-256 in SHA256SUMS; older ones have none.
if command -v sha256sum >/dev/null 2>&1; then
    for source in "${sources[@]}"; do
        if curl -fsSL --connect-timeout 10 --max-time 20 -o "$tmp_dir/SHA256SUMS" "${source}https://github.com/${repo}/releases/download/${version}/SHA256SUMS" 2>/dev/null; then
            expected=$(awk -v n="$asset" '$2 == n || $2 == "*" n { print $1; exit }' "$tmp_dir/SHA256SUMS")
            if [[ -n "$expected" ]]; then
                actual=$(sha256sum "$archive" | awk '{ print $1 }')
                if [[ "${actual,,}" != "${expected,,}" ]]; then
                    echo "Checksum mismatch for ${asset}." >&2
                    exit 1
                fi
                echo "SHA-256 verified."
            fi
            break
        fi
    done
fi
tar -xzf "$archive" -C "$tmp_dir" s-ui/sui-agent s-ui/s-ui-agent.service

install -d -m 0755 /usr/local/s-ui /etc/default /etc/systemd/system
install -m 0755 "$tmp_dir/s-ui/sui-agent" /usr/local/s-ui/sui-agent
install -m 0644 "$tmp_dir/s-ui/s-ui-agent.service" /etc/systemd/system/s-ui-agent.service
resolve_connection
previous_panel=$(sed -nE 's/^SUI_AGENT_PANEL=(.*)$/\1/p' /etc/default/1s-ui-agent 2>/dev/null | head -n1 || true)
if [[ -n "$previous_panel" && "$previous_panel" != "$panel_url" ]]; then
    echo "Replacing the existing binding to ${previous_panel}."
fi
umask 077
{
    printf 'SUI_AGENT_PANEL=%s\n' "$panel_url"
    printf 'SUI_AGENT_TOKEN=%s\n' "$token"
    printf 'SUI_AGENT_INTERVAL=15s\n'
    printf 'SUI_AGENT_INSECURE=%s\n' "$insecure"
    printf 'SUI_AGENT_LOCAL_SOCKET=/run/s-ui/control.sock\n'
} > /etc/default/1s-ui-agent

echo "Validating the panel connection..."
agent_check=(
    /usr/local/s-ui/sui-agent
    --panel "$panel_url"
    --token "$token"
    --interval 15s
    --local-socket /run/s-ui/control.sock
    --once
)
if [[ "$insecure" == "true" ]]; then
    agent_check+=(--insecure)
fi
"${agent_check[@]}"
systemctl daemon-reload
systemctl enable --now s-ui-agent
systemctl is-active --quiet s-ui-agent
echo "1S-UI Agent is connected and running."
