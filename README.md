# TunGo

English | [中文](README_CN.md)

[![Go](https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![MyGo](https://img.shields.io/badge/MyGo-0891B2?style=flat-square)](https://github.com/egoist/mygo)
[![sing-box](https://img.shields.io/badge/sing--box-1677FF?style=flat-square)](https://github.com/SagerNet/sing-box)
[![Xray](https://img.shields.io/badge/Xray-6C5CE7?style=flat-square)](https://github.com/XTLS/Xray-core)

A local proxy client that combines SOCKS5 and HTTP proxies. Traffic is handled by sing-box or xray, with nodes loaded from subscriptions or share links. TunGo is written in [Go](https://github.com/golang/go) and [MyGo](https://github.com/egoist/mygo), with implementation references from [NekoBox](https://github.com/qr243vbi/nekobox) and [v2rayN](https://github.com/2dust/v2rayN).

## Features

- Import subscriptions or share links, supporting vmess, vless, trojan, ss, hysteria2, tuic, socks and http
- Measure TCP latency, real connection latency and download speed, with sorting by latency
- Policy groups select nodes automatically by lowest latency, failover, random, round-robin or lowest load
- Three traffic capture modes: system proxy, PAC and TUN; TUN requires administrator privileges
- Dual cores with sing-box and xray; sing-box is the default and can be switched manually
- Configurable routing and DNS, with splitting by domain, IP, port, network and protocol; rule sets are bundled
- Built-in rule sets block advertising and tracking domains

## Getting Started

1. Under **Subscription**, enter a subscription URL, or choose **Server → Import from Clipboard** and paste a share link.
2. Double-click a node, then click **Start**.

## Project Structure

```text
TunGo/
├── main.go                  Application entry point
├── internal/
│   ├── app/                 Interface and interaction
│   ├── core/                Cores and configuration generation
│   ├── proxy/               Local proxy
│   ├── sub/                 Subscriptions and share links
│   ├── sysproxy/            System proxy
│   ├── appdir/              Data directory
│   ├── backup/              Backup and restore
│   ├── logs/                Logs
│   └── i18n/                Localized strings
├── resources/
│   ├── bin/                 Cores and rule sets
│   ├── icons/               Interface icons
│   ├── icon.png             Application icon
│   ├── TunGo.svg            Application icon
│   └── resources.go         Icon embedding
├── mygo.json                Packaging configuration
├── go.mod
├── LICENSE
└── README.md
```

`app` handles the interface and interaction. The other packages are separated by responsibility with single-direction dependencies and no cycles.

## Data Directory

The default location is `%LOCALAPPDATA%\TunGo`. Uninstalling the application also removes its data. Set the `TUNGO_DATA` environment variable to use a different location.

| File | Contents |
| --- | --- |
| `nodes.json` | Nodes and policy groups |
| `subscription.json` | Subscription URLs, update interval and last update time |
| `routing.json` | Routing profiles and rules |
| `dns.json` | DNS settings |
| `settings.json` | Theme, language, ports, shortcuts and other settings |
| `backups/` | Backup ZIP files; the latest 10 are kept |
| `logs/` | Daily log files; logs are kept for 7 days |

## Build

```sh
go tool mygo build -platform windows/amd64
```

The output is written to `build\windows-amd64\`, including `TunGo.exe`, the installer and bundled resources.

Windows is currently the only officially provided build. macOS and Linux can be built and tested manually.

## License

[MIT](LICENSE)
