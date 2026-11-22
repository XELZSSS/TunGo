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

func xrayInbound(inbound string) (map[string]any, error) {
	host, port, err := net.SplitHostPort(inbound)
	if err != nil {
		return nil, err
	}
	number, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"tag":      "in",
		"listen":   host,
		"port":     number,
		"protocol": "socks",
		"settings": map[string]any{"auth": "noauth", "udp": true},
		"sniffing": map[string]any{"enabled": true, "destOverride": []any{"http", "tls", "quic"}, "routeOnly": false},
	}, nil
}

func xrayDNSInbound() map[string]any {
	return map[string]any{
		"tag":      "dns-in",
		"listen":   "127.0.0.1",
		"port":     1053,
		"protocol": "dokodemo-door",
		"settings": map[string]any{"address": "8.8.8.8", "port": 53, "network": "tcp,udp"},
	}
}

func xrayTun(name string) map[string]any {
	return map[string]any{
		"tag":      tunTag,
		"protocol": "tun",
		"settings": map[string]any{
			"name":                   name,
			"desc":                   name,
			"address":                []any{tunAddress},
			"mtu":                    tunMTU,
			"autoSystemRoutingTable": []any{"0.0.0.0/0", "::/0"},
			"autoSystemDnsToGateway": true,
			"autoOutboundsInterface": "auto",
		},
		"sniffing": map[string]any{"enabled": true, "destOverride": []any{"http", "tls", "quic"}},
	}
}

func xrayOutbound(n Node) (map[string]any, error) {

	if n.Raw != "" {
		var raw map[string]any
		if err := json.Unmarshal([]byte(n.Raw), &raw); err != nil {
			return nil, fmt.Errorf(i18n.T("自定义出站不是 JSON：%w"), err)
		}
		raw["tag"] = "proxy"
		return raw, nil
	}
	if !Xray.Supports(n.Protocol) || n.Direct() {
		return nil, fmt.Errorf(i18n.T("xray 不支持 %q"), n.Protocol)
	}
	out := map[string]any{"tag": "proxy", "protocol": n.Protocol}
	switch n.Protocol {
	case "socks", "http":
		server := map[string]any{"address": n.Server, "port": n.Port}
		if n.Username != "" {
			server["users"] = []any{map[string]any{"user": n.Username, "pass": n.Password}}
		}
		out["settings"] = map[string]any{"servers": []any{server}}
	case "shadowsocks":
		server := map[string]any{
			"address":  n.Server,
			"port":     n.Port,
			"method":   n.Method,
			"password": n.Password,
		}
		if n.Plugin != "" {
			server["plugin"] = n.Plugin
			server["pluginOpts"] = n.PluginOpts
		}
		out["settings"] = map[string]any{"servers": []any{server}}
	case "vmess":
		out["settings"] = map[string]any{"vnext": []any{map[string]any{
			"address": n.Server,
			"port":    n.Port,
			"users": []any{map[string]any{
				"id":       n.UUID,
				"alterId":  n.AlterID,
				"security": vmessSecurity(n),
			}},
		}}}
	case "vless":
		user := map[string]any{"id": n.UUID, "encryption": "none"}
		if n.Flow != "" {
			user["flow"] = n.Flow
		}
		out["settings"] = map[string]any{"vnext": []any{map[string]any{
			"address": n.Server,
			"port":    n.Port,
			"users":   []any{user},
		}}}
	case "trojan":
		out["settings"] = map[string]any{"servers": []any{map[string]any{
			"address":  n.Server,
			"port":     n.Port,
			"password": n.Password,
		}}}
	}
	if SecurityOf(n) != "" && strings.Contains(n.ECH, "://") {
		return nil, errors.New(i18n.T("ECH 要填配置内容，不能填链接"))
	}
	if carriesTLS(n.Protocol) || carriesTransport(n.Protocol) || ((n.Protocol == "socks" || n.Protocol == "http") && SecurityOf(n) != "") {
		if stream := xrayStream(n); len(stream) > 0 {
			out["streamSettings"] = stream
		}
	}
	if n.Mux > 0 && Muxable(n.Protocol) {
		out["mux"] = map[string]any{"enabled": true, "concurrency": n.Mux}
	}
	return out, nil
}
