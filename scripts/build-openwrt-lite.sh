#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-$(tr -d '\r\n' < "$ROOT_DIR/config/version")}"
PKG_RELEASE="${PKG_RELEASE:-1}"
OUT_DIR="${OUT_DIR:-$ROOT_DIR/dist/openwrt-lite}"
BUILD_DIR="${BUILD_DIR:-$ROOT_DIR/dist/build/openwrt-lite}"
SKIP_FRONTEND="${SKIP_FRONTEND:-0}"
TAGS="${SUI_LITE_TAGS:-openwrt_lite,with_quic,with_utls,badlinkname,tfogo_checklinkname0}"
LDFLAGS="${SUI_LITE_LDFLAGS:-}"
if [[ -z "$LDFLAGS" ]]; then
	LDFLAGS="-w -s -checklinkname=0 -linkmode external -extldflags '-static'"
fi
DEFAULT_TARGETS=(
	x86_64
	aarch64_generic
	aarch64_cortex-a53
	aarch64_cortex-a72
	arm_cortex-a7_neon-vfpv4
	arm_cortex-a9
	arm_cortex-a15_neon-vfpv4
	mips_24kc
	mipsel_24kc
	mipsel_74kc
	riscv64_generic
)

usage() {
	cat <<'EOF'
Usage:
  scripts/build-openwrt-lite.sh [--skip-frontend] [target ...]
  scripts/build-openwrt-lite.sh all

Targets:
  x86_64
  aarch64_generic, aarch64_cortex-a53, aarch64_cortex-a72
  arm_cortex-a7_neon-vfpv4, arm_cortex-a9, arm_cortex-a15_neon-vfpv4
  mips_24kc, mipsel_24kc, mipsel_74kc
  riscv64_generic

Notes:
  This build uses CGO because the panel currently uses go-sqlite3.
  For cross builds, set CC to a musl cross compiler before running.
EOF
}

targets=()
while (($#)); do
	case "$1" in
		--skip-frontend)
			SKIP_FRONTEND=1
			;;
		-h|--help)
			usage
			exit 0
			;;
		all)
			targets+=("${DEFAULT_TARGETS[@]}")
			;;
		*)
			targets+=("$1")
			;;
	esac
	shift
done

if ((${#targets[@]} == 0)); then
	targets=(x86_64)
fi

prepare_frontend() {
	if [[ "$SKIP_FRONTEND" == "1" ]]; then
		if [[ -f "$ROOT_DIR/web/html/index.html" ]]; then
			return
		fi
		if [[ -f "$ROOT_DIR/frontend/dist/index.html" ]]; then
			rm -rf "$ROOT_DIR/web/html"
			mkdir -p "$ROOT_DIR/web/html"
			cp -R "$ROOT_DIR/frontend/dist/." "$ROOT_DIR/web/html/"
			return
		fi
		echo "web/html is missing; run without --skip-frontend or provide frontend/dist" >&2
		exit 1
	fi

	(
		cd "$ROOT_DIR/frontend"
		npm install
		VITE_OPENWRT_LITE=true npm run build
	)
	rm -rf "$ROOT_DIR/web/html"
	mkdir -p "$ROOT_DIR/web/html"
	cp -R "$ROOT_DIR/frontend/dist/." "$ROOT_DIR/web/html/"
}

set_target_env() {
	OPENWRT_ARCH="$1"
	GOOS=linux
	unset GOARM GOMIPS GO386

	case "$OPENWRT_ARCH" in
		x86_64|amd64)
			OPENWRT_ARCH=x86_64
			GOARCH=amd64
			;;
		i386|i386_pentium4)
			OPENWRT_ARCH=i386_pentium4
			GOARCH=386
			GO386=sse2
			;;
		aarch64|aarch64_generic|aarch64_cortex-a53|aarch64_cortex-a72)
			[[ "$OPENWRT_ARCH" == "aarch64" ]] && OPENWRT_ARCH=aarch64_generic
			GOARCH=arm64
			;;
		armv7|arm_cortex-a7|arm_cortex-a7_neon-vfpv4|arm_cortex-a9|arm_cortex-a15_neon-vfpv4)
			[[ "$OPENWRT_ARCH" == "armv7" ]] && OPENWRT_ARCH=arm_cortex-a7_neon-vfpv4
			GOARCH=arm
			GOARM=7
			;;
		mips_24kc)
			GOARCH=mips
			GOMIPS=softfloat
			;;
		mipsel_24kc|mipsel_74kc)
			GOARCH=mipsle
			GOMIPS=softfloat
			;;
		riscv64|riscv64_generic)
			[[ "$OPENWRT_ARCH" == "riscv64" ]] && OPENWRT_ARCH=riscv64_generic
			GOARCH=riscv64
			;;
		*)
			echo "unsupported OpenWrt target: $OPENWRT_ARCH" >&2
			exit 1
			;;
	esac

	export GOOS GOARCH CGO_ENABLED=1
	[[ -n "${GOARM:-}" ]] && export GOARM || true
	[[ -n "${GOMIPS:-}" ]] && export GOMIPS || true
	[[ -n "${GO386:-}" ]] && export GO386 || true
}

host_target() {
	echo "$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
}

warn_missing_cross_cc() {
	local target="$GOOS/$GOARCH"
	if [[ "$target" != "$(host_target)" && -z "${CC:-}" ]]; then
		echo "warning: CC is not set for CGO cross build $target; CI sets a musl toolchain automatically" >&2
	fi
}

gnu_tar_args() {
	if tar --version 2>/dev/null | grep -qi "gnu tar"; then
		printf '%s\n' --format=gnu --owner=0 --group=0 --numeric-owner
	fi
}

build_binary() {
	local target="$1"
	local bin_dir="$BUILD_DIR/bin/$target"
	local bin_path="$bin_dir/sui"
	mkdir -p "$bin_dir"

	set_target_env "$target"
	warn_missing_cross_cc

	echo "==> Building $OPENWRT_ARCH ($GOOS/$GOARCH${GOARM:+/arm$GOARM}${GOMIPS:+/$GOMIPS})"
	(
		cd "$ROOT_DIR"
		go build -trimpath -buildvcs=false -tags "$TAGS" -ldflags "$LDFLAGS" -o "$bin_path" main.go
	)
	strip "$bin_path" 2>/dev/null || true
}

gzip_tar_dir() {
	local src_dir="$1"
	local out="$2"
	shift 2
	local tar_path="${out%.gz}"
	rm -f "$tar_path" "$out"
	(cd "$src_dir" && tar "$@" -cf "$tar_path" .)
	if ! gzip -9n "$tar_path" 2>/dev/null; then
		gzip -9 "$tar_path"
	fi
}

write_control_files() {
	local control_dir="$1"
	local installed_size="$2"

	cat > "$control_dir/control" <<EOF
Package: s-ui-lite
Version: ${VERSION}-${PKG_RELEASE}
Architecture: ${OPENWRT_ARCH}
Maintainer: Hhz0823
Section: net
Priority: optional
Depends: ca-bundle
Installed-Size: ${installed_size}
Description: 1S-UI OpenWrt Lite panel (sing-box only, Xray disabled)
EOF

	cat > "$control_dir/postinst" <<'EOF'
#!/bin/sh
[ -n "$IPKG_INSTROOT" ] && exit 0
/etc/init.d/s-ui-lite enable >/dev/null 2>&1 || true
exit 0
EOF

	cat > "$control_dir/prerm" <<'EOF'
#!/bin/sh
[ -n "$IPKG_INSTROOT" ] && exit 0
/etc/init.d/s-ui-lite stop >/dev/null 2>&1 || true
/etc/init.d/s-ui-lite disable >/dev/null 2>&1 || true
exit 0
EOF
	chmod 0755 "$control_dir/postinst" "$control_dir/prerm"
}

make_ipk() {
	local target="$1"
	local bin_path="$BUILD_DIR/bin/$target/sui"
	local pkg_dir="$BUILD_DIR/pkg/$OPENWRT_ARCH"
	local rootfs="$pkg_dir/data"
	local control_dir="$pkg_dir/control"
	local archive_dir="$pkg_dir/archive"
	local ipk="$OUT_DIR/s-ui-lite_${VERSION}-${PKG_RELEASE}_${OPENWRT_ARCH}.ipk"
	local tar_args=()
	while IFS= read -r arg; do
		tar_args+=("$arg")
	done < <(gnu_tar_args)

	rm -rf "$pkg_dir"
	mkdir -p "$rootfs/usr/bin" "$rootfs/etc/init.d" "$rootfs/etc/s-ui/db" "$rootfs/usr/lib/s-ui" "$control_dir" "$archive_dir" "$OUT_DIR"
	cp "$bin_path" "$rootfs/usr/bin/sui"
	cp "$ROOT_DIR/packaging/openwrt-lite/files/s-ui-lite.init" "$rootfs/etc/init.d/s-ui-lite"
	chmod 0755 "$rootfs/usr/bin/sui" "$rootfs/etc/init.d/s-ui-lite"

	local installed_size
	installed_size="$(du -sk "$rootfs" | awk '{print $1}')"
	write_control_files "$control_dir" "$installed_size"

	printf '2.0\n' > "$archive_dir/debian-binary"
	gzip_tar_dir "$control_dir" "$archive_dir/control.tar.gz" "${tar_args[@]}"
	gzip_tar_dir "$rootfs" "$archive_dir/data.tar.gz" "${tar_args[@]}"
	(
		cd "$archive_dir"
		rm -f "$ipk"
		ar -r "$ipk" debian-binary control.tar.gz data.tar.gz >/dev/null
	)
	echo "==> Wrote $ipk"
}

prepare_frontend
mkdir -p "$OUT_DIR" "$BUILD_DIR"

for target in "${targets[@]}"; do
	build_binary "$target"
	make_ipk "$target"
done
