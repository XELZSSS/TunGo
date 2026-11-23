package sub

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

type subscription struct {
	Proxies     []map[string]any `json:"proxies"`
	ProxyGroups []map[string]any `json:"proxy-groups"`
	Outbounds   []map[string]any `json:"outbounds"`
}

var clashProtocols = map[string]string{
	"ss": "shadowsocks", "shadowsocks": "shadowsocks",
	"socks": "socks", "socks5": "socks",
	"http":  "http",
	"vmess": "vmess", "vless": "vless", "trojan": "trojan",
	"hysteria2": "hysteria2", "hy2": "hysteria2",
	"tuic":   "tuic",
	"anytls": "anytls",
}

var singBoxSkips = map[string]bool{
	"direct": true, "block": true, "selector": true, "urltest": true, "dns": true,
}

func yamlValue(node *yaml.Node) any {
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		out := make([]any, 0, len(node.Content))
		for _, child := range node.Content {
			out = append(out, yamlValue(child))
		}
		if node.Kind == yaml.DocumentNode && len(out) == 1 {
			return out[0]
		}
		return out
	case yaml.MappingNode:
		out := make(map[string]any, len(node.Content)/2)
		for at := 0; at+1 < len(node.Content); at += 2 {
			out[node.Content[at].Value] = yamlValue(node.Content[at+1])
		}
		return out
	case yaml.ScalarNode:
		if node.Tag == "!!null" {
			return nil
		}
		return node.Value
	}
	return nil
}

func maps(value any) []map[string]any {
	items, _ := value.([]any)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if one, ok := item.(map[string]any); ok {
			out = append(out, one)
		}
	}
	return out
}

func parseConfig(text string) ([]core.Node, bool, error) {
	var doc subscription
	if json.Unmarshal([]byte(text), &doc) != nil {
		var root yaml.Node
		if yaml.Unmarshal([]byte(text), &root) != nil {
			return nil, false, nil
		}
		whole, _ := yamlValue(&root).(map[string]any)
		doc = subscription{Proxies: maps(whole["proxies"]), ProxyGroups: maps(whole["proxy-groups"]), Outbounds: maps(whole["outbounds"])}
	}
	if len(doc.Proxies) == 0 && len(doc.Outbounds) == 0 {
		return nil, false, nil
	}
	var nodes []core.Node
	failed, reason := 0, ""
	keep := func(node core.Node, err error) {
		if err != nil {
			failed++
			if reason == "" {
				reason = err.Error()
			}
			return
		}
		nodes = append(nodes, node)
	}
	for _, proxy := range doc.Proxies {
		keep(clashNode(proxy))
	}
	for _, outbound := range doc.Outbounds {
		if !singBoxSkips[field(outbound, "type")] {
			keep(singBoxNode(outbound))
		}
	}
	nodes = append(nodes, groups(doc, nodes)...)
	if len(nodes) == 0 {
		if failed == 0 {
			return nil, true, errors.New(i18n.T("订阅里没有认得的节点"))
		}
		return nil, true, fmt.Errorf(i18n.T("订阅里没有认得的节点：%d 条失败，首个原因：%s"), failed, reason)
	}
	return nodes, true, nil
}

func clashNode(one map[string]any) (core.Node, error) {
	kind := field(one, "type")
	protocol := clashProtocols[kind]
	if protocol == "" {
		return core.Node{}, fmt.Errorf(i18n.T("不认识的协议 %s"), kind)
	}
	server := field(one, "server")
	if server == "" {
		return core.Node{}, errors.New(i18n.T("缺少服务器或端口"))
	}
	port, err := portNumber(kind, num(one, "port"))
	if err != nil {
		return core.Node{}, err
	}
	transport, err := network(field(one, "network"))
	if err != nil {
		return core.Node{}, err
	}
	node := core.Node{
		Name:        first(field(one, "name"), name("", server, port)),
		Protocol:    protocol,
		Server:      server,
		Port:        port,
		Network:     transport,
		Flow:        field(one, "flow"),
		ServerName:  first(field(one, "sni"), field(one, "servername")),
		ALPN:        values(one, "alpn"),
		Fingerprint: first(field(one, "client-fingerprint"), field(one, "fingerprint")),
		Insecure:    flag(one, "skip-cert-verify"),
		ECH:         first(field(one, "ech-opts"), field(one, "ech")),
		Pinned:      field(one, "pinned-peer-cert-sha256"),
	}
	switch protocol {
	case "vmess":
		node.UUID = field(one, "uuid")
		node.AlterID = num(one, "alterId")
		node.Security = field(one, "cipher")
	case "vless":
		node.UUID = field(one, "uuid")
	case "trojan", "anytls":
		node.Password = field(one, "password")
	case "shadowsocks":
		node.Method = field(one, "cipher")
		node.Password = field(one, "password")
	case "hysteria2":
		node.Password = field(one, "password")
		node.Obfs = field(one, "obfs")
		node.ObfsPassword = field(one, "obfs-password")
	case "tuic":
		node.UUID = field(one, "uuid")
		node.Password = first(field(one, "password"), field(one, "token"))
		node.Congestion = field(one, "congestion-controller")
	case "socks", "http":
		node.Username = field(one, "username")
		node.Password = field(one, "password")
	}
	if flag(one, "tls") {
		node.TLS = "tls"
	}
	if reality := obj(one, "reality-opts"); reality != nil {
		node.TLS = "reality"
		node.PublicKey = field(reality, "public-key")
		node.ShortID = field(reality, "short-id")
	}
	switch node.Network {
	case "ws":
		ws := obj(one, "ws-opts")
		node.Path = field(ws, "path")
		node.Host = first(field(obj(ws, "headers"), "Host"), field(obj(ws, "headers"), "host"))
	case "grpc":
		node.Path = field(obj(one, "grpc-opts"), "grpc-service-name")
	case "http":
		if h2 := obj(one, "h2-opts"); h2 != nil {
			node.Path = field(h2, "path")
			node.Host = first(values(h2, "host")...)
			break
		}
		httpOpts := obj(one, "http-opts")
		node.Path = first(values(httpOpts, "path")...)
		node.Host = first(values(obj(httpOpts, "headers"), "Host")...)
	}
	return node, nil
}

func singBoxNode(one map[string]any) (core.Node, error) {
	kind := field(one, "type")
	protocol := clashProtocols[kind]
	if protocol == "" {
		return core.Node{}, fmt.Errorf(i18n.T("不认识的协议 %s"), kind)
	}
	server := field(one, "server")
	if server == "" {
		return core.Node{}, errors.New(i18n.T("缺少服务器或端口"))
	}
	port, err := portNumber(kind, num(one, "server_port"))
	if err != nil {
		return core.Node{}, err
	}
	transport := obj(one, "transport")
	protocolNetwork, err := network(field(transport, "type"))
	if err != nil {
		return core.Node{}, err
	}
	node := core.Node{
		Name:     first(field(one, "tag"), name("", server, port)),
		Protocol: protocol,
		Server:   server,
		Port:     port,
		Network:  protocolNetwork,
	}
	switch protocol {
	case "vmess":
		node.UUID = field(one, "uuid")
		node.AlterID = num(one, "alter_id")
		node.Security = field(one, "security")
		node.Flow = field(one, "flow")
	case "vless":
		node.UUID = field(one, "uuid")
		node.Flow = field(one, "flow")
	case "trojan", "anytls":
		node.Password = field(one, "password")
	case "shadowsocks":
		node.Method = field(one, "method")
		node.Password = field(one, "password")
		node.Plugin = field(one, "plugin")
		node.PluginOpts = field(one, "plugin_opts")
	case "hysteria2":
		node.Password = field(one, "password")
		node.Obfs = field(obj(one, "obfs"), "type")
		node.ObfsPassword = field(obj(one, "obfs"), "password")
	case "tuic":
		node.UUID = field(one, "uuid")
		node.Password = field(one, "password")
		node.Congestion = field(one, "congestion_control")
	case "socks", "http":
		node.Username = field(one, "username")
		node.Password = field(one, "password")
	}
	if tls := obj(one, "tls"); flag(tls, "enabled") {
		node.TLS = "tls"
		node.ServerName = field(tls, "server_name")
		node.Insecure = flag(tls, "insecure")
		node.ALPN = values(tls, "alpn")
		node.Fingerprint = field(obj(tls, "utls"), "fingerprint")
		node.ECH = echConfig(tls["ech"])
		node.Pinned = pinnedHash(tls["certificate"])
		if reality := obj(tls, "reality"); flag(reality, "enabled") {
			node.TLS = "reality"
			node.PublicKey = field(reality, "public_key")
			node.ShortID = field(reality, "short_id")
		}
	}
	switch node.Network {
	case "ws", "httpupgrade":
		node.Path = field(transport, "path")
		node.Host = field(obj(transport, "headers"), "Host")
	case "grpc":
		node.Path = field(transport, "service_name")
	case "http":
		node.Path = field(transport, "path")
		node.Host = first(values(transport, "host")...)
	}
	return node, nil
}

func echConfig(value any) string {
	switch one := value.(type) {
	case string:
		return one
	case map[string]any:
		return field(one, "config")
	case []any:
		if len(one) > 0 {
			return echConfig(one[0])
		}
	}
	return ""
}

func pinnedHash(value any) string {
	certificate, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	if hashes := values(certificate, "pinned_sha256"); len(hashes) > 0 {
		return hashes[0]
	}
	return ""
}

func field(one map[string]any, key string) string {
	switch value := one[key].(type) {
	case string:
		return value
	case int:
		return strconv.Itoa(value)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	return ""
}

func num(one map[string]any, key string) int {
	switch value := one[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	case string:
		n, _ := strconv.Atoi(value)
		return n
	}
	return 0
}

func obj(one map[string]any, key string) map[string]any {
	value, _ := one[key].(map[string]any)
	return value
}

func values(one map[string]any, key string) []string {
	switch value := one[key].(type) {
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if text := fmt.Sprint(item); text != "" {
				out = append(out, text)
			}
		}
		return out
	case string:
		return list(value)
	}
	return nil
}

func flag(one map[string]any, key string) bool {
	switch value := one[key].(type) {
	case bool:
		return value
	case string:
		return value == "true"
	}
	return false
}
func groups(doc subscription, nodes []core.Node) []core.Node {
	names := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		names[node.Name] = true
	}
	var out []core.Node
	seen := map[string]bool{}
	add := func(name, strategy string, children []string) {
		if name == "" || seen[name] || len(children) == 0 {
			return
		}
		seen[name] = true
		out = append(out, core.Node{Name: name, Role: core.GroupRole, Children: children, Strategy: strategy})
	}
	for _, group := range doc.ProxyGroups {
		add(field(group, "name"), groupStrategy(field(group, "type")), members(values(group, "proxies"), names))
	}
	for _, outbound := range doc.Outbounds {
		kind := field(outbound, "type")
		if kind != "selector" && kind != "urltest" {
			continue
		}
		add(field(outbound, "tag"), groupStrategy(kind), members(values(outbound, "outbounds"), names))
	}
	return out
}

func members(candidates []string, names map[string]bool) []string {
	out := make([]string, 0, len(candidates))
	for _, name := range candidates {
		if names[name] {
			out = append(out, name)
		}
	}
	return out
}

func groupStrategy(kind string) string {
	switch strings.ToLower(kind) {
	case "fallback":
		return "fallback"
	case "load-balance", "loadbalance", "leastload":
		return "leastLoad"
	case "random":
		return "random"
	case "round-robin", "roundrobin":
		return "roundRobin"
	}
	return "leastPing"
}
