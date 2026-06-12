# 1S-UI

A feature-rich web panel for [Sing-Box](https://github.com/SagerNet/sing-box), forked from [alireza0/s-ui](https://github.com/alireza0/s-ui) v1.4.1.

> **Disclaimer:** This project is for personal learning and exchange purposes only. Please do not use it for illegal purposes.

## Features

| Feature | Supported |
| -------------------------------------- | :-------: |
| Multi-protocol | :heavy_check_mark: |
| Multi-language | :heavy_check_mark: |
| Multi-client/inbound | :heavy_check_mark: |
| Advanced traffic routing | :heavy_check_mark: |
| Client, traffic & system status | :heavy_check_mark: |
| Subscription (link/json/clash + info) | :heavy_check_mark: |
| 10 UI Themes (Dark/Light + 8 custom) | :heavy_check_mark: |
| API interface | :heavy_check_mark: |
| TLS pinnedPeerCertificateSha256 | :heavy_check_mark: |
| Quick add node (auto TLS + config) | :heavy_check_mark: |
| Congestion control (BBR/FQ/CAKE) | :heavy_check_mark: |

## Supported Platforms

| Platform | Architecture | Status |
|----------|--------------|--------|
| Linux | amd64, arm64, armv7, armv6, armv5, 386, s390x | Supported |
| Windows | amd64, 386, arm64 | Supported |
| macOS | amd64, arm64 | Experimental |

## Default Settings

- Panel port: 2095
- Panel path: /app/
- Subscription port: 2096
- Subscription path: /sub/
- Username: dmin
- Password: dmin

## Install or Upgrade

### Linux/macOS
`sh
bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh)
`

### Windows
1. Download the latest Windows version from [GitHub Releases](https://github.com/Hhz0823/1s-ui/releases/latest)
2. Extract the ZIP file
3. Run install-windows.bat as administrator
4. Follow the installation guide

## Manual Installation

### Linux/macOS
1. Download the latest S-UI release from [GitHub Releases](https://github.com/Hhz0823/1s-ui/releases/latest)
2. (Optional) Download the latest s-ui.sh: [s-ui.sh](https://raw.githubusercontent.com/Hhz0823/1s-ui/main/s-ui.sh)
3. (Optional) Copy s-ui.sh to /usr/bin/ and run chmod +x /usr/bin/s-ui
4. Extract the s-ui tar.gz file to your chosen directory
5. Copy *.service files to /etc/systemd/system/ and run systemctl daemon-reload
6. Use systemctl enable s-ui --now to enable and start the S-UI service
7. Use systemctl enable sing-box --now to start the sing-box service

## Uninstall S-UI

`sh
sudo -i
systemctl disable s-ui --now
rm -f /etc/systemd/system/sing-box.service
systemctl daemon-reload
rm -fr /usr/local/s-ui
rm /usr/bin/s-ui
`

## Docker Installation

<details>
<summary>Click to view details</summary>

### Docker Compose

`yaml
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
`

`shell
docker compose up -d
`

### Direct Docker

`shell
docker run -itd \
    --network host \
    -v E:\1S-ui/db/:/app/db/ \
    -v E:\1S-ui/cert/:/root/cert/ \
    --name s-ui \
    --restart=unless-stopped \
    ghcr.io/hhz0823/1s-ui
`

### Build from Source

`shell
git clone https://github.com/Hhz0823/1s-ui
docker build -t s-ui .
`

</details>

## Languages

- English
- Farsi
- Vietnamese
- Simplified Chinese
- Traditional Chinese
- Russian

## Protocols

- **Common protocols:** Mixed, SOCKS, HTTP, HTTPS, Direct, Redirect, TProxy
- **V2Ray protocols:** VLESS, VMess, Trojan, Shadowsocks
- **Other protocols:** ShadowTLS, Hysteria, Hysteria2, Naive, TUIC, AnyTls
- **XTLS** supported
- **Congestion control:** BBR v1/v2/v3/v2plus/plus, FQ, CAKE

## Environment Variables

<details>
<summary>Click to view details</summary>

| Variable | Type | Default |
| -------------- | :---: | :---: |
| SUI_LOG_LEVEL | "debug" \| "info" \| "warn" \| "error" | "info" |
| SUI_DEBUG | oolean | alse |
| SUI_BIN_FOLDER | string | "bin" |
| SUI_DB_FOLDER | string | "db" |
| SINGBOX_API | string | - |

</details>

## SSL Certificate

<details>
<summary>Click to view details</summary>

`ash
snap install core; snap refresh core
snap install --classic certbot
ln -s /snap/bin/certbot /usr/bin/certbot

certbot certonly --standalone --register-unsafely-without-email --non-interactive --agree-tos -d <your-domain>
`

</details>

---

Thanks to the original author: [alireza0/s-ui](https://github.com/alireza0/s-ui)
