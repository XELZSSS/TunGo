# TunGo

Windows 上的代理客户端：本地跑一个 SOCKS5 + HTTP 混合代理，流量交给 sing-box 或 xray，节点来自订阅或分享链接。原生界面，打包出来是单文件 exe。

用 [Go](https://go.dev) 编写，界面是 [MyGo](https://github.com/egoist/mygo) 的原生控件（`go.mod` 里锁在 v0.9.2），没有 Web 前端；打包也用 MyGo 自带的 `build` 命令。

## 能做什么

- 导入订阅，或从剪贴板粘分享链接（vmess、vless、trojan、ss、hysteria2、tuic、socks、http）
- 节点列表带 TCP 延迟、真连接延迟和测速，可分组、可排序
- 策略组：一组节点按最低延迟、故障转移、随机、轮询或最低负载自动挑一个
- 三种接管方式：系统代理、PAC、TUN（TUN 需要管理员权限）
- 路由规则与 DNS 可改，规则集随包带着，缺了会自己补

## 开始用

1. 运行 `TunGo.exe`——安装版双击安装包，便携版解压即用
2. 「订阅」填地址，或「服务器 → 从剪贴板导入」分享链接
3. 双击节点，按「启动」

数据和配置在 `%LOCALAPPDATA%\TunGo`，卸载不会删掉。

## 自己构建

```sh
go tool mygo build -platform windows/amd64
```

产物在 `build\windows-amd64\`。sing-box、xray 与 `wintun.dll` 体积太大不入库，构建前先放进 `resources/bin\`。
