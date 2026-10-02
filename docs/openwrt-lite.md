# 1S-UI for OpenWrt

The OpenWrt build is 1S-UI for routers and other low-memory devices. It
runs as one process: the panel, sing-box, the controller agent and the web UI
together use about 50 MB of memory, and Go's heap is capped at a quarter of
the router's RAM (between 48 and 256 MiB).

What it does on a router:

- **Proxy client, like PassWall or v2rayN**: subscriptions and share links,
  latency tests, a chosen node or the fastest of a group, routing modes
  (mainland China direct, GFW list only, global), a transparent proxy for the
  whole LAN through TUN, a SOCKS5 + HTTP port (7890), clean DNS through
  dnsmasq, and devices or domains that always or never use the proxy.
- **Relay for a controller**: node checks for every protocol and speed tests
  run from the router, so the controller and the Android app see what the
  home network sees.
- **Remote control**: bound to a controller, the router's mode, node,
  subscriptions and proxy switch can be changed from the controller's web UI
  and from the Android app.

It uses sing-box only (no Xray), with the `with_quic` and `with_utls` build
tags; standard gRPC, ACME, the Naive outbound, gVisor and Tailscale are left
out to keep the binary small.

## Install

Log in to the router over SSH as root and run:

```sh
wget -O /tmp/1s-ui-openwrt.sh https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install-openwrt.sh && sh /tmp/1s-ui-openwrt.sh
```

To bind the router to a controller in the same step, open **Server Monitor →
Add child server** on the controller, expand **fnOS NAS / OpenWrt / panel address +
key**, generate a key, choose **OpenWrt router** and paste the command it
shows. It looks like this:

```sh
wget -O /tmp/1s-ui-openwrt.sh https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install-openwrt.sh \
  && sh /tmp/1s-ui-openwrt.sh --panel 'https://controller.example.com/app/' --key 'KEY' --version '1.7.1'
```

In mainland China, use the controller's mainland China command, which
downloads the script and the release through a GitHub mirror
(`--mirror cn`). Without `--mirror`, the installer uses GitHub and switches to
the mirrors when GitHub is unreachable. Downloads are checked against the
release's `SHA256SUMS`. The OpenWrt packages are published with every release
from v1.7.1 on.

Then open `http://router-ip:2095/` and create the administrator.

Options:

| Option | Meaning |
| --- | --- |
| `--panel URL --key KEY` | Bind to the controller at `URL` with its enrollment key |
| `--name NAME` | Name shown on the controller (default: the hostname) |
| `--version vX.Y.Z` | Release to install (default: the latest) |
| `--mirror auto\|github\|cn\|URL` | Download line |
| `--port PORT` | Web and API port (default 2095) |
| `--no-web` | Skip the web UI and manage the router from the controller only |
| `--insecure` | Accept a self-signed certificate on the controller |
| `--uninstall [--purge]` | Remove 1S-UI; `--purge` also deletes `/etc/s-ui` |

The installer needs OpenWrt 21.02 or later (iStoreOS and ImmortalWrt work the
same way). With opkg it installs the `.ipk` matching
`opkg print-architecture`; otherwise (apk-based OpenWrt after 24.10, or an
architecture without its own `.ipk`) it unpacks
`s-ui-openwrt-<arch>.tar.gz`. It also installs the kernel modules the
transparent proxy uses (`kmod-tun`, `kmod-nft-queue`, `kmod-inet-diag`,
`kmod-netlink-diag`) when they are missing.

## Transparent proxy

Turn on **Transparent proxy** under **Proxy client**. sing-box creates the TUN
device with `auto_route` and `auto_redirect`, which installs its own nftables
rules; OpenWrt's fw4 needs no extra configuration. Mainland China addresses
are excluded from the TUN routes in the "bypass mainland China" mode, so that
traffic never enters the proxy.

With **Take over LAN DNS** on, the client writes `1s-ui-client.conf` into
dnsmasq's runtime directory (`/tmp/dnsmasq.d`, or `/tmp/dnsmasq.cfg*.d` on
newer releases), pointing dnsmasq at its local DNS server
(127.0.0.1:5335), which resolves mainland China domains through the direct DNS
server and everything else through the remote one. Stopping the service
removes the file and restarts dnsmasq, so the LAN keeps working.

## Files

| Path | Contents |
| --- | --- |
| `/usr/bin/sui` | Program |
| `/etc/init.d/s-ui-lite` | procd service (`start`, `stop`, `restart`, `enable`) |
| `/etc/s-ui/db/s-ui.db` | Database |
| `/etc/s-ui/db/rulesets` | Routing rule files (mainland China and GFW lists, refreshed weekly) |
| `/etc/s-ui/agent.env` | Controller connection (mode 600) |
| `/etc/s-ui/env` | Optional overrides, such as `SUI_API_PORT=8080` or `GOMEMLIMIT=96MiB` |
| `/usr/share/s-ui/web` | Web UI |
| `/usr/lib/s-ui` | Runtime files |

Logs go to the system log: `logread -e sui`.

## Build

Build one target:

```sh
scripts/build-openwrt-lite.sh x86_64
```

Build all predefined targets:

```sh
scripts/build-openwrt-lite.sh all
```

Each target produces `dist/openwrt-lite/s-ui-lite_<version>-1_<arch>.ipk` and
`dist/openwrt-lite/s-ui-openwrt-<arch>.tar.gz`. The web UI is the release's
`s-ui-frontend.tar.gz`, which the installer unpacks into
`/usr/share/s-ui/web`.

The panel uses SQLite through CGO, so cross builds need a musl C toolchain in
`CC`. The release workflow builds every target with Bootlin musl toolchains
(`.github/workflows/openwrt-lite.yml`, which `release.yml` calls) and publishes
the packages with each release.

To build with more sing-box features, override `SUI_LITE_TAGS`:

```sh
SUI_LITE_TAGS='openwrt_lite,with_quic,with_grpc,with_utls,with_acme,badlinkname,tfogo_checklinkname0' scripts/build-openwrt-lite.sh x86_64
```

## Targets

- `x86_64`
- `aarch64_generic`, `aarch64_cortex-a53`, `aarch64_cortex-a72`
- `arm_cortex-a7_neon-vfpv4`, `arm_cortex-a9`, `arm_cortex-a15_neon-vfpv4`
- `mips_24kc`, `mipsel_24kc`, `mipsel_74kc`
- `riscv64_generic`

If `opkg print-architecture` reports another architecture, the installer
uses the tarball of the generic target with the same instruction set:
`aarch64_generic` for other ARM64 cores, `arm_cortex-a7_neon-vfpv4` for other
ARMv7 cores, `mips_24kc` and `mipsel_24kc` for other MIPS cores. ARMv6 and
older are not supported.
