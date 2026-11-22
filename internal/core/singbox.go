package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"tungo/internal/i18n"
)

func singBoxInbound(inbound string) (map[string]any, error) {
	host, port, err := net.SplitHostPort(inbound)
	if err != nil {
		return nil, err
	}
	number, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"type":        "socks",
		"tag":         "in",
		"listen":      host,
		"listen_port": number,
	}, nil
}

func singBoxTun(name string) map[string]any {
	return map[string]any{
		"type":           "tun",
		"tag":            tunTag,
		"interface_name": name,
		"address":        []any{tunAddress},
		"mtu":            tunMTU,
		"stack":          "system",
		"auto_route":     true,
		"strict_route":   true,
	}
}

func singBoxOutbound(n Node) (map[string]any, error) {

	if n.Raw != "" {
		var raw map[string]any
		if err := json.Unmarshal([]byte(n.Raw), &raw); err != nil {
			return nil, fmt.Errorf(i18n.T("自定义出站不是 JSON：%w"), err)
		}
		raw["tag"] = "proxy"
		return raw, nil
	}
	if !SingBox.Supports(n.Protocol) || n.Direct() {
		return nil, fmt.Errorf(i18n.T("sing-box 不支持 %q"), n.Protocol)
	}
	out := map[string]any{
		"type":        n.Protocol,
		"tag":         "proxy",
		"server":      n.Server,
		"server_port": n.Port,
	}
	switch n.Protocol {
	case "socks":
		out["version"] = "5"
		if n.Username != "" {
			out["username"] = n.Username
			out["password"] = n.Password
		}
	case "http":
		if n.Username != "" {
			out["username"] = n.Username
			out["password"] = n.Password
		}
	case "shadowsocks":
		out["method"] = n.Method
		out["password"] = n.Password
		if n.Plugin != "" {
			out["plugin"] = n.Plugin
			out["plugin_opts"] = n.PluginOpts
		}
	case "vmess":
		out["uuid"] = n.UUID
		out["security"] = vmessSecurity(n)
		out["alter_id"] = n.AlterID
	case "vless":
		out["uuid"] = n.UUID
		if n.Flow != "" {
			out["flow"] = n.Flow
		}
	case "trojan":
		out["password"] = n.Password
	case "hysteria2":
		out["password"] = n.Password
		if n.Obfs != "" {
			out["obfs"] = map[string]any{"type": n.Obfs, "password": n.ObfsPassword}
		}
	case "tuic":
		out["uuid"] = n.UUID
		out["password"] = n.Password
		if n.Congestion != "" {
			out["congestion_control"] = n.Congestion
		}
	case "anytls":
		out["password"] = n.Password
	}
	if security := SecurityOf(n); security != "" {
		if strings.Contains(n.ECH, "://") {
			return nil, errors.New(i18n.T("ECH 要填配置内容，不能填链接"))
		}
		if !carriesTLS(n.Protocol) {
			return nil, fmt.Errorf(i18n.T("sing-box 的 %s 出站不支持 TLS：去掉 TLS 或换用 xray"), n.Protocol)
		}
		out["tls"] = singBoxTLS(n)
	}
	if carriesTransport(n.Protocol) && n.Network != "" && n.Network != "tcp" {
		if transport := singBoxTransport(n); transport != nil {
			out["transport"] = transport
		}
	}
	if n.Mux > 0 && Muxable(n.Protocol) {
		out["multiplex"] = map[string]any{"enabled": true, "max_connections": n.Mux}
	}
	return out, nil
}
