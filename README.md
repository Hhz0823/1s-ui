# 1S-UI

A feature-rich web panel for [Sing-Box](https://github.com/SagerNet/sing-box), forked from [alireza0/s-ui](https://github.com/alireza0/s-ui) v1.4.1.

> **Disclaimer:** This project is for personal learning and exchange purposes only. Please do not use it for illegal purposes.

---

## Quick Install

```
bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh)
```

Default login: **admin** / **admin** | Panel: `http://your-server-ip:2095/app/`

---

## Features

### Core

| Feature | Supported |
| -------------------------------------- | :-------: |
| Multi-protocol (VLESS, VMess, Trojan, SS, Hysteria2, TUIC, etc.) | :heavy_check_mark: |
| Multi-language (EN, ZH-CN, ZH-TW, FA, VI, RU) | :heavy_check_mark: |
| Multi-client / Multi-inbound management | :heavy_check_mark: |
| Advanced traffic routing interface | :heavy_check_mark: |
| Client, traffic and system status monitoring | :heavy_check_mark: |
| Subscription (link / json / clash + info) | :heavy_check_mark: |
| API interface | :heavy_check_mark: |

### UI and Themes

| Feature | Supported |
| -------------------------------------- | :-------: |
| 10 UI Themes (Light, Dark, Midnight, Ocean, Sunset, Forest, Sakura, Cyberpunk, Nord, Dracula) | :heavy_check_mark: |
| Responsive layout (Desktop and Mobile) | :heavy_check_mark: |
| Rounded corners and glassmorphism effects | :heavy_check_mark: |
| Real-time theme switching | :heavy_check_mark: |

### TLS and Security

| Feature | Supported |
| -------------------------------------- | :-------: |
| TLS pinnedPeerCertificateSha256 (v2rayN compatible) | :heavy_check_mark: |
| Auto self-signed certificate generation | :heavy_check_mark: |
| ACME / ECH / Reality support | :heavy_check_mark: |
| Client certificate verification | :heavy_check_mark: |

### Quick Add Node

| Protocol | Auto-config |
| -------- | ----------- |
| VMess | TLS + HTTP transport |
| VLESS | TLS + HTTP transport |
| Trojan | TLS + HTTP transport |
| Hysteria2 | TLS + obfs salamander |
| ShadowTLS | v3 + handshake server |
| TUIC | TLS + cubic congestion |
| Naive | TLS |
| AnyTls | TLS + padding scheme |
| Shadowsocks | Random password + method |

### Network and Kernel

| Feature | Options |
| ------- | ------- |
| BBR versions | v1, v2, v3, v2plus, bbrplus |
| Queue discipline | FQ, CAKE |
| Congestion algorithm | Cubic (default), BBR variants |

---

## Supported Platforms

| Platform | Architecture | Status |
|----------|--------------|--------|
| Linux | amd64, arm64, armv7, armv6, armv5, 386, s390x | Supported |
| Windows | amd64, 386, arm64 | Supported |
| macOS | amd64, arm64 | Experimental |

---

## Default Settings

| Setting | Value |
| ------- | ----- |
| Panel port | `2095` |
| Panel path | `/app/` |
| Subscription port | `2096` |
| Subscription path | `/sub/` |
| Username | `admin` |
| Password | `admin` |

---

## Install or Upgrade

### Linux/macOS

```
bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh)
```

### Windows

1. Download the latest Windows version from [GitHub Releases](https://github.com/Hhz0823/1s-ui/releases/latest)
2. Extract the ZIP file
3. Run `install-windows.bat` as administrator
4. Follow the installation guide

### Install specific version

Append the version tag with `v` at the end of the install command. For example, version `v1.4.4`:

```
bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh) v1.4.4
```

---

## Manual Installation

### Linux/macOS

1. Download the latest S-UI release from [GitHub Releases](https://github.com/Hhz0823/1s-ui/releases/latest)
2. (Optional) Download `s-ui.sh`: [s-ui.sh](https://raw.githubusercontent.com/Hhz0823/1s-ui/main/s-ui.sh)
3. (Optional) Copy `s-ui.sh` to `/usr/bin/` and run `chmod +x /usr/bin/s-ui`
4. Extract the s-ui tar.gz file to your chosen directory
5. Copy `*.service` files to `/etc/systemd/system/` and run `systemctl daemon-reload`
6. Use `systemctl enable s-ui --now` to enable and start the S-UI service
7. Use `systemctl enable sing-box --now` to start the sing-box service

### Windows

1. Download from [GitHub Releases](https://github.com/Hhz0823/1s-ui/releases/latest)
2. Extract the ZIP file
3. Run `install-windows.bat` as administrator
4. Follow the installation guide
5. Access panel at: `http://localhost:2095/app/`

---

## Uninstall S-UI

```
sudo -i
systemctl disable s-ui --now
rm -f /etc/systemd/system/sing-box.service
systemctl daemon-reload
rm -fr /usr/local/s-ui
rm /usr/bin/s-ui
```

---

## Docker Installation

<details>
<summary>Click to expand</summary>

### Docker Compose

```yaml
services:
  s-ui:
    image: ghcr.io/hhz0823/1s-ui
    container_name: s-ui
    hostname: "s-ui"
    network_mode: host
    volumes:
      - "./db:/app/db"
      - "./cert:/app/cert"
    tty: true
    restart: unless-stopped
    entrypoint: "./entrypoint.sh"
```

```
docker compose up -d
```

### Direct Docker

```
docker run -itd --network host -v $PWD/db/:/app/db/ -v $PWD/cert/:/root/cert/ --name s-ui --restart=unless-stopped ghcr.io/hhz0823/1s-ui
```

### Build from Source

```
git clone https://github.com/Hhz0823/1s-ui
docker build -t s-ui .
```

</details>

---

## Developer Build

<details>
<summary>Click to expand</summary>

### Build and run the full project

```
./runSUI.sh
```

### Clone the repository

```
git clone https://github.com/Hhz0823/1s-ui
```

### Frontend

See [frontend](frontend) directory.

### Backend

> Build the frontend at least once first.

```
rm -fr web/html/*
cp -R frontend/dist/ web/html/
go build -o sui main.go
```

Run the backend (from project root):

```
./sui
```

</details>

---

## Pages Overview

| Page | Description |
| ---- | ----------- |
| **Home** | System info cards, backup and restore, logs, usage statistics |
| **Inbound Management** | Add/edit/delete inbounds, quick add node with auto TLS |
| **Client Management** | User accounts, traffic limits, expiry settings |
| **Outbound Management** | Protocol config, link generation, clone |
| **Endpoint Management** | Node management, WireGuard/Tailscale |
| **Service Management** | CCM, OCM, DERP, SSMAPI |
| **TLS Settings** | Certificate management, ACME, ECH, Reality, Pinned SHA256 |
| **Basics** | Log level, routing defaults, experimental options |
| **Rules** | Route rules, rulesets, rule import |
| **DNS** | DNS servers, DNS rules, Fake-IP |
| **Admins** | Admin accounts, API tokens, login history |
| **Settings** | Panel config, subscription, network (BBR/FQ/CAKE) |

---

## Languages

- English
- Farsi (Persian)
- Vietnamese
- Simplified Chinese
- Traditional Chinese
- Russian

---

## Protocols

### Common

Mixed, SOCKS, HTTP, HTTPS, Direct, Redirect, TProxy

### V2Ray

VLESS, VMess, Trojan, Shadowsocks

### Other

ShadowTLS, Hysteria, Hysteria2, Naive, TUIC, AnyTls, WireGuard

---

## Environment Variables

<details>
<summary>Click to expand</summary>

| Variable | Type | Default |
| -------------- | :---: | :---: |
| SUI_LOG_LEVEL | `"debug"` or `"info"` or `"warn"` or `"error"` | `"info"` |
| SUI_DEBUG | `boolean` | `false` |
| SUI_BIN_FOLDER | `string` | `"bin"` |
| SUI_DB_FOLDER | `string` | `"db"` |
| SINGBOX_API | `string` | - |

</details>

---

## SSL Certificate

<details>
<summary>Click to expand</summary>

```
snap install core; snap refresh core
snap install --classic certbot
ln -s /snap/bin/certbot /usr/bin/certbot
certbot certonly --standalone --register-unsafely-without-email --non-interactive --agree-tos -d your-domain.com
```

</details>

---

Thanks to the original author: [alireza0/s-ui](https://github.com/alireza0/s-ui)