#!/bin/sh
# 1S-UI for OpenWrt: the panel with sing-box, the PassWall-style proxy
# client, node checks and speed tests, optionally bound to a controller.
#
#   sh install-openwrt.sh [--panel URL --key KEY] [--name NAME]
#                         [--version VERSION] [--mirror auto|github|cn|URL]
#                         [--port 2095] [--no-web] [--insecure]
#   sh install-openwrt.sh --uninstall [--purge]
#
# Runs on OpenWrt 21.02 and later (opkg or apk) with BusyBox ash.

set -e

REPO="Hhz0823/1s-ui"
BUILTIN_MIRRORS="https://ghfast.top/ https://gh-proxy.com/ https://ghproxy.net/"
SERVICE=/etc/init.d/s-ui-lite
DATA_DIR=/etc/s-ui
WEB_DIR=/usr/share/s-ui/web

panel=""
key=""
name=""
version=""
line="${SUI_MIRROR:-auto}"
port=""
web=1
insecure=false
uninstall=0
purge=0

say() { printf '%s\n' "$*"; }
die() { printf 'Error: %s\n' "$*" >&2; exit 1; }

usage() {
	cat <<'EOF'
Usage: sh install-openwrt.sh [--panel URL --key KEY] [--name NAME] [--version VERSION]
                             [--mirror auto|github|cn|URL] [--port PORT] [--no-web] [--insecure]
       sh install-openwrt.sh --uninstall [--purge]

--panel URL --key KEY  bind this router to the 1S-UI controller at URL with the
                       enrollment key shown on the controller's server page
--name NAME            the name the router gets on the controller (default: hostname)
--port PORT            web and API port on the LAN (default 2095)
--no-web               skip the web UI (manage the router from the controller only)
--mirror               download line: auto (GitHub, mirrors when it is slow),
                       github, cn (mainland China mirrors first) or a mirror prefix
EOF
}

while [ $# -gt 0 ]; do
	case "$1" in
		--panel) panel="${2:-}"; shift 2 ;;
		--key) key="${2:-}"; shift 2 ;;
		--name) name="${2:-}"; shift 2 ;;
		--version) version="${2:-}"; shift 2 ;;
		--mirror) line="${2:-}"; shift 2 ;;
		--mirror=*) line="${1#--mirror=}"; shift ;;
		--port) port="${2:-}"; shift 2 ;;
		--no-web) web=0; shift ;;
		--insecure) insecure=true; shift ;;
		--uninstall) uninstall=1; shift ;;
		--purge) purge=1; shift ;;
		-h|--help) usage; exit 0 ;;
		*) usage >&2; die "unknown option $1" ;;
	esac
done

[ "$(id -u)" = 0 ] || die "run this installer as root"
[ -f /etc/openwrt_release ] || die "this installer is for OpenWrt; on Debian, Ubuntu or fnOS use install.sh"

if [ "$uninstall" = 1 ]; then
	[ -x "$SERVICE" ] && { "$SERVICE" stop >/dev/null 2>&1 || true; "$SERVICE" disable >/dev/null 2>&1 || true; }
	if command -v opkg >/dev/null 2>&1 && opkg list-installed 2>/dev/null | grep -q '^s-ui-lite '; then
		opkg remove s-ui-lite >/dev/null 2>&1 || true
	fi
	rm -f /usr/bin/sui "$SERVICE"
	rm -rf "$WEB_DIR" /usr/lib/s-ui /var/run/s-ui
	[ "$purge" = 1 ] && rm -rf "$DATA_DIR"
	say "1S-UI removed.$( [ "$purge" = 1 ] || printf ' Settings are kept in %s (use --purge to delete them).' "$DATA_DIR")"
	exit 0
fi

if [ -n "$panel$key" ]; then
	echo "$panel" | grep -Eq "^https?://[^[:space:]'\"\\\\#]+$" ||
		die "--key needs the controller address in --panel, for example https://panel.example.com:2095/app/"
	echo "$key" | grep -Eq '^[A-Za-z0-9_-]{32,128}$' || die "invalid enrollment key; copy it again from the controller"
fi
if [ -n "$port" ]; then
	echo "$port" | grep -Eq '^[0-9]+$' && [ "$port" -ge 1 ] && [ "$port" -le 65535 ] || die "invalid --port"
fi

# --- downloads ---------------------------------------------------------

have() { command -v "$1" >/dev/null 2>&1; }

# fetch URL FILE [SECONDS]
fetch() {
	if have curl; then
		curl -fsSL --connect-timeout 15 --max-time "${3:-300}" -o "$2" "$1"
	elif have uclient-fetch; then
		uclient-fetch -q -T "${3:-300}" -O "$2" "$1"
	else
		wget -q -T "${3:-300}" -O "$2" "$1"
	fi
}

# post URL BODY: prints the response
post() {
	skip=""
	if have curl; then
		[ "$insecure" = true ] && skip=--insecure
		curl -fsS --max-time 20 ${skip:+"$skip"} -H 'Content-Type: application/json' --data "$2" "$1"
	elif have uclient-fetch; then
		[ "$insecure" = true ] && skip=--no-check-certificate
		uclient-fetch -q -T 20 ${skip:+"$skip"} --post-data="$2" -O - "$1"
	else
		[ "$insecure" = true ] && skip=--no-check-certificate
		wget -q -T 20 ${skip:+"$skip"} --header='Content-Type: application/json' --post-data="$2" -O - "$1"
	fi
}

case "$line" in
	github) sources="-" ;;
	cn) sources="$BUILTIN_MIRRORS -" ;;
	http://*|https://*) case "$line" in */) ;; *) line="$line/" ;; esac; sources="$line - $BUILTIN_MIRRORS" ;;
	*)
		if fetch "https://github.com/$REPO/releases/latest" /dev/null 10 2>/dev/null; then
			sources="- $BUILTIN_MIRRORS"
		else
			say "GitHub is unreachable; using the mainland China mirrors."
			sources="$BUILTIN_MIRRORS -"
		fi
		;;
esac

# release_get NAME FILE: downloads a release asset, trying each source.
release_get() {
	for source in $sources; do
		[ "$source" = "-" ] && source=""
		if fetch "${source}https://github.com/$REPO/releases/download/$version/$1" "$2"; then
			return 0
		fi
	done
	return 1
}

tmp=$(mktemp -d /tmp/1s-ui.XXXXXX)
trap 'rm -rf "$tmp"' EXIT

if [ -z "$version" ]; then
	if fetch "https://api.github.com/repos/$REPO/releases/latest" "$tmp/latest.json" 20 2>/dev/null; then
		version=$(jsonfilter -i "$tmp/latest.json" -e '@.tag_name' 2>/dev/null || true)
	fi
	if [ -z "$version" ]; then
		for source in $sources; do
			[ "$source" = "-" ] && source=""
			if fetch "${source}https://github.com/$REPO/releases/latest" "$tmp/latest.html" 30 2>/dev/null; then
				version=$(grep -oE "/$REPO/releases/tag/[^\"'?#<> ]+" "$tmp/latest.html" | head -n1 | sed 's#.*/releases/tag/##')
				[ -n "$version" ] && break
			fi
		done
	fi
	[ -n "$version" ] || die "could not find the latest release; pass --version"
fi
case "$version" in v*) ;; *) version="v$version" ;; esac
echo "$version" | grep -Eq '^v[0-9A-Za-z._-]+$' || die "invalid version $version"

# --- architecture --------------------------------------------------------

arch=""
if have opkg; then
	arch=$(opkg print-architecture 2>/dev/null | awk '$1 == "arch" && $2 != "all" && $2 != "noarch" { a = $2 } END { print a }')
elif have apk; then
	arch=$(apk --print-arch 2>/dev/null || true)
fi
[ -n "$arch" ] || arch=$(uname -m)
case "$arch" in
	x86_64) target=x86_64 ;;
	aarch64_cortex-a53|aarch64_cortex-a72|aarch64_generic) target="$arch" ;;
	aarch64*|arm64) target=aarch64_generic ;;
	arm_cortex-a9*) target=arm_cortex-a9 ;;
	arm_cortex-a15*) target=arm_cortex-a15_neon-vfpv4 ;;
	arm_cortex-a*|armv7*) target=arm_cortex-a7_neon-vfpv4 ;;
	mips_*|mips) target=mips_24kc ;;
	mipsel_*|mipsel) target=mipsel_24kc ;;
	riscv64*) target=riscv64_generic ;;
	*) die "no 1S-UI build for architecture $arch" ;;
esac
say "Installing 1S-UI $version for $arch ($target)…"

memory=$(awk '/^MemTotal:/ { print int($2 / 1024) }' /proc/meminfo)
[ "${memory:-0}" -lt 200 ] && say "Note: this router has ${memory} MB of RAM; 256 MB or more is recommended."

# --- install -------------------------------------------------------------

sums=""
if release_get SHA256SUMS "$tmp/SHA256SUMS" 2>/dev/null; then
	sums="$tmp/SHA256SUMS"
fi
verify() {
	[ -n "$sums" ] && have sha256sum || return 0
	expected=$(awk -v n="$1" '$2 == n || $2 == "*" n { print $1; exit }' "$sums")
	[ -n "$expected" ] || return 0
	actual=$(sha256sum "$2" | awk '{ print $1 }')
	[ "$actual" = "$expected" ] || die "checksum mismatch for $1"
}

[ -x "$SERVICE" ] && "$SERVICE" stop >/dev/null 2>&1 || true
ipk="s-ui-lite_${version#v}-1_${arch}.ipk"
if have opkg && [ "$target" = "$arch" ] && release_get "$ipk" "$tmp/$ipk" 2>/dev/null; then
	verify "$ipk" "$tmp/$ipk"
	opkg install --force-reinstall --force-downgrade "$tmp/$ipk" >/dev/null || die "opkg could not install $ipk"
else
	bundle="s-ui-openwrt-${target}.tar.gz"
	release_get "$bundle" "$tmp/$bundle" || die "could not download $bundle; retry with --mirror URL"
	verify "$bundle" "$tmp/$bundle"
	tar -xzf "$tmp/$bundle" -C "$tmp"
	mkdir -p /usr/bin /etc/init.d "$DATA_DIR/db" /usr/lib/s-ui
	cp "$tmp/s-ui-lite/sui" /usr/bin/sui.new && chmod 0755 /usr/bin/sui.new && mv -f /usr/bin/sui.new /usr/bin/sui
	cp "$tmp/s-ui-lite/s-ui-lite.init" "$SERVICE" && chmod 0755 "$SERVICE"
fi

if [ "$web" = 1 ]; then
	if release_get s-ui-frontend.tar.gz "$tmp/web.tar.gz"; then
		verify s-ui-frontend.tar.gz "$tmp/web.tar.gz"
		rm -rf "$WEB_DIR.new" && mkdir -p "$WEB_DIR.new"
		tar -xzf "$tmp/web.tar.gz" -C "$WEB_DIR.new"
		rm -rf "$WEB_DIR" && mv "$WEB_DIR.new" "$WEB_DIR"
	else
		say "Could not download the web UI; the router can still be managed from the controller."
	fi
fi

# Kernel modules for the transparent proxy; the panel reports what is missing.
if [ ! -e /dev/net/tun ] || [ ! -d /sys/module/nft_queue ]; then
	if have opkg; then
		opkg update >/dev/null 2>&1 || true
		opkg install kmod-tun kmod-nft-queue kmod-inet-diag kmod-netlink-diag >/dev/null 2>&1 || true
	elif have apk; then
		apk add kmod-tun kmod-nft-queue kmod-inet-diag kmod-netlink-diag >/dev/null 2>&1 || true
	fi
	[ -e /dev/net/tun ] || say "Note: kmod-tun is not installed; the transparent proxy needs it (opkg install kmod-tun)."
fi

mkdir -p "$DATA_DIR"
if [ -n "$port" ]; then
	touch "$DATA_DIR/env"
	sed -i '/^SUI_API_PORT=/d' "$DATA_DIR/env"
	echo "SUI_API_PORT=$port" >> "$DATA_DIR/env"
fi

# --- controller ------------------------------------------------------------

if [ -n "$panel" ]; then
	[ -n "$name" ] || name=$(uci -q get system.@system[0].hostname 2>/dev/null || cat /proc/sys/kernel/hostname)
	name=$(printf '%s' "$name" | tr -d '\000-\037"\\' | cut -c1-80)
	[ -n "$name" ] || name=openwrt
	say "Connecting to the 1S-UI controller…"
	response=$(post "${panel%/}/agent/v1/enroll" "{\"code\":\"$key\",\"name\":\"$name\"}") ||
		die "the controller rejected the key; check the address and that the key was not regenerated"
	panel_url=$(printf '%s' "$response" | jsonfilter -e '@.obj.panel_url' 2>/dev/null || true)
	token=$(printf '%s' "$response" | jsonfilter -e '@.obj.token' 2>/dev/null || true)
	echo "$panel_url" | grep -Eq '^https?://[^[:space:]]+$' && echo "$token" | grep -Eq '^[A-Za-z0-9_-]{32,128}$' ||
		die "the controller returned an invalid answer"
	umask 077
	{
		echo "SUI_AGENT_PANEL=$panel_url"
		echo "SUI_AGENT_TOKEN=$token"
		echo "SUI_AGENT_INTERVAL=15s"
		echo "SUI_AGENT_INSECURE=$insecure"
	} > "$DATA_DIR/agent.env"
	umask 022
fi

"$SERVICE" enable >/dev/null 2>&1 || true
"$SERVICE" restart >/dev/null 2>&1 || "$SERVICE" start

lan=$(uci -q get network.lan.ipaddr 2>/dev/null | cut -d/ -f1)
[ -n "$lan" ] || lan=192.168.1.1
api_port="${port:-$(sed -n 's/^SUI_API_PORT=//p' "$DATA_DIR/env" 2>/dev/null | tail -n1)}"
api_port="${api_port:-2095}"
ok=0
i=0
while [ $i -lt 15 ]; do
	if fetch "http://127.0.0.1:$api_port/api/setup-status" "$tmp/status" 3 2>/dev/null; then
		ok=1
		break
	fi
	i=$((i + 1))
	sleep 1
done
[ "$ok" = 1 ] || die "1S-UI did not start; see: logread -e sui"

say ""
say "1S-UI $version is running."
[ -f "$WEB_DIR/index.html" ] && say "  Web UI: http://$lan:$api_port/app/  (set the admin account on first visit)"
[ -n "$panel" ] && say "  Bound to the controller: the router appears under its servers as \"$name\"."
say "  Proxy client: open 客户端代理 (Proxy client) on the router or on the controller."
