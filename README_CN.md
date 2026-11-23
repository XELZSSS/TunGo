# TunGo

[English](README.md) | 中文

[![Go](https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![MyGo](https://img.shields.io/badge/MyGo-0891B2?style=flat-square)](https://github.com/egoist/mygo)
[![sing-box](https://img.shields.io/badge/sing--box-1677FF?style=flat-square)](https://github.com/SagerNet/sing-box)
[![Xray](https://img.shields.io/badge/Xray-6C5CE7?style=flat-square)](https://github.com/XTLS/Xray-core)

SOCKS5 与 HTTP 混合代理，流量交给 sing-box 或 xray，节点来自订阅或分享链接，用 [Go](https://github.com/golang/go) + [MyGo](https://github.com/egoist/mygo) 编写，参考 [NekoBox](https://github.com/qr243vbi/nekobox) 与 [v2rayN](https://github.com/2dust/v2rayN)

## 功能

- 订阅或分享链接能导入 vmess、vless、trojan、ss、hysteria2、tuic、socks、http 节点
- 可以测 TCP 延迟、真连接延迟和下载速度，并按延迟排序
- 策略组按最低延迟、故障转移、随机、轮询或最低负载自动挑节点
- 系统代理、PAC、TUN 三种方式都能接管流量，TUN 需要管理员权限
- sing-box 与 xray 双内核随时切换，内核和规则集可走下载镜像或离线安装
- 分流按域名、IP、端口、网络、协议进行，内置规则集拦截广告与追踪域名
- 规则的出口可选代理、直连、拦截，也能直接指向某个节点或策略组
- 右键节点能测真连接、测速和流媒体解锁，解锁按出口地区判断
- 托盘图标显示连接状态，可启动即连接、只留托盘，断网或唤醒后自动重连
- 备份只手动生成，可挑选还原，也能导入外部备份

## 开始使用

1. 「订阅」填地址，或「服务器 → 剪贴板导入」粘贴分享链接
2. 双击节点，按「启动」

## 项目结构

```text
TunGo/
├── main.go                程序入口
├── internal/
│   ├── app/               界面与交互
│   ├── core/              内核与配置生成
│   ├── proxy/             本地代理
│   ├── sub/               订阅与分享链接
│   ├── sysproxy/          系统代理
│   ├── appdir/            数据目录
│   ├── backup/            备份与恢复
│   ├── logs/              日志
│   └── i18n/              多语言文案
├── resources/
│   ├── bin/               内核与规则集
│   ├── icons/             界面图标
│   ├── icon.png           应用图标
│   ├── TunGo.svg          应用图标
│   └── resources.go       图标嵌入
├── mygo.json              打包配置
├── go.mod
├── LICENSE
└── README_CN.md
```

`app` 管界面与交互，其余包按职责分层，依赖单向、没有循环

## 数据目录

默认在 `%LOCALAPPDATA%\TunGo`，卸载会连同数据一起删除，设 `TUNGO_DATA` 可换位置

| 文件 | 内容 |
| --- | --- |
| `nodes.json` | 节点与策略组 |
| `subscription.json` | 订阅地址、更新间隔、上次更新时间 |
| `routing.json` | 路由方案与规则 |
| `dns.json` | DNS 设置 |
| `settings.json` | 主题、语言、端口、快捷键等 |
| `backups/` | 手动备份的 zip，不自动清理 |
| `logs/` | 按天切分的日志，保留 7 天 |

## 构建

```sh
go tool mygo build -platform windows/amd64
```

产物在 `build\windows-amd64\`，含 `TunGo.exe`、安装包和随包资源

目前只提供 Windows 版本，macOS 与 Linux 请自行构建测试

## 许可证

[MIT](LICENSE)