# OpenWrt Lite

OpenWrt Lite is a small package variant of 1S-UI for routers and low-memory devices.

- Core: embedded sing-box only
- Disabled: Xray runtime, Xray config generation, Xray inbound creation
- Default sing-box build tags: `with_quic`, `with_utls`
- Omitted heavy build tags: standard gRPC, ACME, naive outbound, gVisor, Tailscale
- Package format: `.ipk` with a procd init script

## Build

Build one target:

```sh
scripts/build-openwrt-lite.sh x86_64
```

Build all predefined OpenWrt targets:

```sh
scripts/build-openwrt-lite.sh all
```

The panel uses SQLite through CGO, so cross builds need a musl C toolchain in `CC`.
The GitHub Actions workflow `.github/workflows/openwrt-lite.yml` sets this automatically with Bootlin musl toolchains.

The default Lite profile keeps common proxy features that need QUIC or uTLS, such as Hysteria/TUIC-style transports and TLS fingerprint support, while removing ACME and the heavier standard gRPC implementation. The gRPC transport still uses sing-box's lite gRPC implementation. To build a custom package with extra sing-box features, override `SUI_LITE_TAGS`:

```sh
SUI_LITE_TAGS='openwrt_lite,with_quic,with_grpc,with_utls,with_acme,badlinkname,tfogo_checklinkname0' scripts/build-openwrt-lite.sh x86_64
```

## Install

Copy the matching package to the router, then run:

```sh
opkg install ./s-ui-lite_1.4.7-1_x86_64.ipk
/etc/init.d/s-ui-lite enable
/etc/init.d/s-ui-lite start
```

Useful paths:

- Binary: `/usr/bin/sui`
- Service: `/etc/init.d/s-ui-lite`
- Database: `/etc/s-ui/db/s-ui.db`
- Runtime bin folder: `/usr/lib/s-ui`

## Targets

Predefined package targets:

- `x86_64`
- `aarch64_generic`
- `aarch64_cortex-a53`
- `aarch64_cortex-a72`
- `arm_cortex-a7_neon-vfpv4`
- `arm_cortex-a9`
- `arm_cortex-a15_neon-vfpv4`
- `mips_24kc`
- `mipsel_24kc`
- `mipsel_74kc`
- `riscv64_generic`

If your router reports a different architecture with `opkg print-architecture`, use the matching release package or add a mapping to `scripts/build-openwrt-lite.sh`.
