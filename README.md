# TunGo

本地代理客户端：一个 SOCKS5 + HTTP 混合代理，流量交给 sing-box 或 xray，节点来自订阅或分享链接。使用 [Go](https://github.com/golang/go) + [MyGo](https://github.com/egoist/mygo) 编写，实现方式参考 [NekoBox](https://github.com/qr243vbi/nekobox) 和 [v2rayN](https://github.com/2dust/v2rayN)

## 能做什么

- 导入订阅，或从剪贴板粘分享链接（vmess、vless、trojan、ss、hysteria2、tuic、socks、http）
- 节点带 TCP 延迟、真连接延迟和测速，可分组、可排序
- 策略组按最低延迟、故障转移、随机、轮询或最低负载自动挑一个
- 系统代理、PAC、TUN 三种接管方式（TUN 要管理员权限）
- 路由规则与 DNS 可改，规则集随包带着

## 开始用

1. 「订阅」填地址，或「服务器 → 从剪贴板导入」分享链接
2. 双击节点，按「启动」

数据在 `%LOCALAPPDATA%\TunGo`，卸载不删

## 构建

```sh
go tool mygo build -platform windows/amd64
```

产物在 `build\windows-amd64\` sing-box、xray、`wintun.dll` 不入库，构建前放进 `resources/bin\`
