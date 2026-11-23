# TunGo

English | [中文](README_CN.md)

[![Go](https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![MyGo](https://img.shields.io/badge/MyGo-0891B2?style=flat-square)](https://github.com/egoist/mygo)
[![sing-box](https://img.shields.io/badge/sing--box-1677FF?style=flat-square)](https://github.com/SagerNet/sing-box)
[![Xray](https://img.shields.io/badge/Xray-6C5CE7?style=flat-square)](https://github.com/XTLS/Xray-core)

A SOCKS5 and HTTP mixed proxy that hands traffic to sing-box or xray, with nodes from subscriptions or share links, written in [Go](https://github.com/golang/go) and [MyGo](https://github.com/egoist/mygo), with references from [NekoBox](https://github.com/qr243vbi/nekobox) and [v2rayN](https://github.com/2dust/v2rayN)

## Features

- Import vmess, vless, trojan, ss, hysteria2, tuic, socks and http nodes from a subscription or a share link
- Measure TCP latency, real connection latency and download speed, and sort by latency
- Policy groups pick nodes by least ping, failover, random, round robin or least load
- System proxy, PAC and TUN all capture traffic, and TUN needs administrator privileges
- Switch between the sing-box and xray kernels at any time, and fetch kernels and rule sets through a mirror or from a file
- Split traffic by domain, IP, port, network and protocol, with built-in rule sets that block ads and trackers
- Send a rule to proxy, direct or block, or straight to a chosen node or policy group
- Right-click a node to test real latency, speed and streaming unblocking, which is judged by the exit region
- The tray icon shows the state, and the app can connect on launch, start in the tray and reconnect after the network drops or the system wakes
- Backups are made by hand, can be picked for restore and can be imported from a file

## Getting Started

1. Enter a subscription URL under **Subscription**, or choose **Server → Import from Clipboard** and paste a share link
2. Double-click a node, then click **Start**

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

`app` handles the interface and interaction, and the other packages are layered by responsibility with one-way dependencies and no cycles

## Data Directory

The default is `%LOCALAPPDATA%\TunGo`, uninstalling removes the data too, and `TUNGO_DATA` moves it

| File | Contents |
| --- | --- |
| `nodes.json` | Nodes and policy groups |
| `subscription.json` | Subscription URLs, update interval and last update time |
| `routing.json` | Routing profiles and rules |
| `dns.json` | DNS settings |
| `settings.json` | Theme, language, ports, shortcuts and other settings |
| `backups/` | Manually made backup ZIP files, never pruned |
| `logs/` | Daily log files, kept for 7 days |

## Build

```sh
go tool mygo build -platform windows/amd64
```

The output lands in `build\windows-amd64\`, with `TunGo.exe`, the installer and bundled resources

Windows is the only official build, and macOS and Linux can be built and tested manually

## License

[MIT](LICENSE)