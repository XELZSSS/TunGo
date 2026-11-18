# TunGo

本地代理客户端：一个 SOCKS5 + HTTP 混合代理，流量交给 sing-box 或 xray，节点来自订阅或分享链接，使用 [Go](https://github.com/golang/go) + [MyGo](https://github.com/egoist/mygo) 编写，实现方式参考 [NekoBox](https://github.com/qr243vbi/nekobox) 和 [v2rayN](https://github.com/2dust/v2rayN)

## 应用功能

- 导入订阅或分享链接，支持 vmess、vless、trojan、ss、hysteria2、tuic、socks、http
- 测 TCP 延迟、真连接延迟和下载速度，可按延迟排序
- 策略组按最低延迟、故障转移、随机、轮询、最低负载自动选节点
- 系统代理、PAC、TUN 三种接管方式，TUN 要管理员权限
- sing-box 与 xray 双内核，按协议自动切换，也可手动指定
- 路由与 DNS 可改，按域名、IP、端口、网络、协议分流，规则集随包附带

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
└── README.md
```

`app` 负责界面与交互，其余各包按职责分层，依赖单向、没有循环

## 开始使用

1. 「订阅」填地址，或「服务器 → 从剪贴板导入」粘贴分享链接
2. 双击节点，按「启动」

## 数据目录

默认在 `%LOCALAPPDATA%\TunGo`，卸载会连同数据一起删除；设环境变量 `TUNGO_DATA` 可换位置

| 文件 | 内容 |
| --- | --- |
| `nodes.json` | 节点与策略组 |
| `subscription.json` | 订阅地址、更新间隔、上次更新时间 |
| `routing.json` | 路由方案与规则 |
| `dns.json` | DNS 设置 |
| `settings.json` | 主题、语言、端口、快捷键等 |
| `backups/` | 备份 zip，保留最近 10 份 |
| `logs/` | 按天切分的日志，保留 7 天 |

## 构建

```sh
go tool mygo build -platform windows/amd64
```

产物在 `build\windows-amd64\`，含 `TunGo.exe` 与安装包 `tungo-setup-<version>-amd64.exe`，内核与规则集随包分发在 `resources\bin\`

目前只提供 Windows 版本，macOS 与 Linux 请自行构建测试

## 许可证

[MIT](LICENSE)
