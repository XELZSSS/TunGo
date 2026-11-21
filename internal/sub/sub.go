package sub

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

func Fetch(from string) ([]core.Node, error) {
	return fetch(from, &http.Client{Timeout: time.Minute})
}

func FetchProxied(from, socksAddr string) ([]core.Node, error) {
	addr, err := url.Parse("socks5://" + socksAddr)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{Proxy: http.ProxyURL(addr), IdleConnTimeout: 30 * time.Second}
	defer transport.CloseIdleConnections()
	return fetch(from, &http.Client{Timeout: time.Minute, Transport: transport})
}

func fetch(from string, client *http.Client) ([]core.Node, error) {
	req, err := http.NewRequest("GET", from, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "TunGo")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(i18n.T("订阅返回 %s"), resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20+1))
	if err != nil {
		return nil, err
	}
	if len(body) > 8<<20 {
		return nil, errors.New(i18n.T("订阅内容超过 8MB，已停止导入"))
	}
	return Parse(body)
}

func Parse(data []byte) ([]core.Node, error) {
	text := strings.TrimSpace(strings.TrimPrefix(string(data), "\ufeff"))
	nodes, found, err := parseConfig(text)
	if !found && !strings.Contains(text, "://") {
		if decoded, ok := decode(text); ok {
			text = decoded
			nodes, found, err = parseConfig(text)
		}
	}
	if found {
		return nodes, err
	}
	var out []core.Node
	failed, reason := 0, ""
	for _, line := range strings.Split(text, "\n") {
		link := strings.TrimSpace(line)
		if link == "" || strings.HasPrefix(link, "#") {
			continue
		}
		node, err := Link(link)
		if err != nil {
			failed++
			if reason == "" {
				reason = err.Error()
			}
			continue
		}
		out = append(out, node)
	}
	if len(out) == 0 {
		if failed == 0 {
			return nil, errors.New(i18n.T("订阅里没有认得的节点"))
		}
		return nil, fmt.Errorf(i18n.T("订阅里没有认得的节点：%d 条失败，首个原因：%s"), failed, reason)
	}
	return out, nil
}

func Link(link string) (core.Node, error) {
	switch {
	case strings.HasPrefix(link, "vmess://"):
		return vmess(link)
	case strings.HasPrefix(link, "vless://"):
		return vless(link)
	case strings.HasPrefix(link, "trojan://"):
		return trojan(link)
	case strings.HasPrefix(link, "ss://"):
		return shadowsocks(link)
	case strings.HasPrefix(link, "hysteria2://"), strings.HasPrefix(link, "hy2://"):
		return hysteria2(link)
	case strings.HasPrefix(link, "tuic://"):
		return tuic(link)
	case strings.HasPrefix(link, "socks://"), strings.HasPrefix(link, "socks5://"),
		strings.HasPrefix(link, "http://"), strings.HasPrefix(link, "https://"):
		return proxy(link)
	}
	return core.Node{}, errors.New(i18n.T("不认识的链接"))
}

func proxy(link string) (core.Node, error) {
	u, err := url.Parse(link)
	if err != nil {
		return core.Node{}, err
	}

	if u.Path != "" && u.Path != "/" {
		return core.Node{}, errors.New(i18n.T("不是节点"))
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return core.Node{}, errors.New(i18n.T("缺少端口"))
	}
	protocol := "socks"
	if strings.HasPrefix(u.Scheme, "http") {
		protocol = "http"
	}
	node := core.Node{
		Name:     name(u.Fragment, u.Hostname(), port),
		Protocol: protocol,
		Server:   u.Hostname(),
		Port:     port,
	}
	if u.User != nil {
		node.Username = u.User.Username()
		node.Password, _ = u.User.Password()
	}
	return node, nil
}

func vmess(link string) (core.Node, error) {
	body, ok := decode(strings.TrimPrefix(link, "vmess://"))
	if !ok {
		return core.Node{}, errors.New(i18n.T("vmess: 不是 base64"))
	}
	var v struct {
		PS   string          `json:"ps"`
		Add  string          `json:"add"`
		Port json.RawMessage `json:"port"`
		ID   string          `json:"id"`
		Aid  json.RawMessage `json:"aid"`
		Scy  string          `json:"scy"`
		Net  string          `json:"net"`
		Path string          `json:"path"`
		Host string          `json:"host"`
		TLS  string          `json:"tls"`
		SNI  string          `json:"sni"`
		ALPN string          `json:"alpn"`
		FP   string          `json:"fp"`
	}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return core.Node{}, err
	}
	port, err := number(v.Port)
	if err != nil {
		return core.Node{}, err
	}
	if port <= 0 || port > 65535 {
		return core.Node{}, errors.New(i18n.T("端口不是数字"))
	}

	aid, _ := number(v.Aid)
	transport, err := network(v.Net)
	if err != nil {
		return core.Node{}, err
	}
	kind, err := security(v.TLS)
	if err != nil {
		return core.Node{}, err
	}
	return core.Node{
		Name:        name(v.PS, v.Add, port),
		Protocol:    "vmess",
		Server:      v.Add,
		Port:        port,
		AlterID:     aid,
		UUID:        v.ID,
		Security:    v.Scy,
		Network:     transport,
		Path:        v.Path,
		Host:        v.Host,
		TLS:         kind,
		ServerName:  v.SNI,
		ALPN:        list(v.ALPN),
		Fingerprint: v.FP,
	}, nil
}

func vless(link string) (core.Node, error) {
	u, err := url.Parse(link)
	if err != nil {
		return core.Node{}, err
	}
	if u.User == nil {
		return core.Node{}, errors.New(i18n.T("vless: 缺少用户"))
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return core.Node{}, errors.New(i18n.T("vless: 缺少端口"))
	}
	q := u.Query()
	transport, err := network(q.Get("type"))
	if err != nil {
		return core.Node{}, err
	}
	kind, err := security(q.Get("security"))
	if err != nil {
		return core.Node{}, err
	}
	return core.Node{
		Name:        name(u.Fragment, u.Hostname(), port),
		Protocol:    "vless",
		Server:      u.Hostname(),
		Port:        port,
		UUID:        u.User.Username(),
		Flow:        q.Get("flow"),
		Network:     transport,
		Path:        first(q.Get("path"), q.Get("serviceName")),
		Host:        q.Get("host"),
		TLS:         kind,
		ServerName:  first(q.Get("sni"), q.Get("host")),
		ALPN:        list(q.Get("alpn")),
		Fingerprint: q.Get("fp"),
		Insecure:    q.Get("allowInsecure") == "1" || strings.EqualFold(q.Get("allowInsecure"), "true"),
		PublicKey:   q.Get("pbk"),
		ShortID:     q.Get("sid"),
		ECH:         q.Get("ech"),
		Pinned:      q.Get("pcs"),
	}, nil
}

func trojan(link string) (core.Node, error) {
	u, err := url.Parse(link)
	if err != nil {
		return core.Node{}, err
	}
	if u.User == nil {
		return core.Node{}, errors.New(i18n.T("trojan: 缺少密码"))
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return core.Node{}, errors.New(i18n.T("trojan: 缺少端口"))
	}
	q := u.Query()
	transport, err := network(q.Get("type"))
	if err != nil {
		return core.Node{}, err
	}
	kind, err := security(first(q.Get("security"), "tls"))
	if err != nil {
		return core.Node{}, err
	}
	return core.Node{
		Name:        name(u.Fragment, u.Hostname(), port),
		Protocol:    "trojan",
		Server:      u.Hostname(),
		Port:        port,
		Password:    u.User.Username(),
		Flow:        q.Get("flow"),
		Network:     transport,
		Path:        first(q.Get("path"), q.Get("serviceName")),
		Host:        q.Get("host"),
		TLS:         kind,
		ServerName:  first(q.Get("sni"), q.Get("peer"), q.Get("host")),
		ALPN:        list(q.Get("alpn")),
		Fingerprint: q.Get("fp"),
		Insecure:    q.Get("allowInsecure") == "1" || strings.EqualFold(q.Get("allowInsecure"), "true"),
		PublicKey:   q.Get("pbk"),
		ShortID:     q.Get("sid"),
		ECH:         q.Get("ech"),
		Pinned:      q.Get("pcs"),
	}, nil
}

func hysteria2(link string) (core.Node, error) {
	u, err := url.Parse(link)
	if err != nil {
		return core.Node{}, err
	}
	if u.User == nil {
		return core.Node{}, errors.New(i18n.T("hysteria2: 缺少密码"))
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return core.Node{}, errors.New(i18n.T("hysteria2: 缺少端口"))
	}
	q := u.Query()
	return core.Node{
		Name:         name(u.Fragment, u.Hostname(), port),
		Protocol:     "hysteria2",
		Server:       u.Hostname(),
		Port:         port,
		Password:     u.User.Username(),
		TLS:          "tls",
		ServerName:   first(q.Get("sni"), q.Get("peer")),
		Insecure:     q.Get("insecure") == "1",
		ALPN:         list(q.Get("alpn")),
		Obfs:         q.Get("obfs"),
		ObfsPassword: q.Get("obfs-password"),
		ECH:          q.Get("ech"),
		Pinned:       q.Get("pinSHA256"),
	}, nil
}
func tuic(link string) (core.Node, error) {
	u, err := url.Parse(link)
	if err != nil {
		return core.Node{}, err
	}
	if u.User == nil {
		return core.Node{}, errors.New(i18n.T("tuic: 缺少凭据"))
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return core.Node{}, errors.New(i18n.T("tuic: 缺少端口"))
	}
	uuid := u.User.Username()
	password, _ := u.User.Password()
	q := u.Query()
	return core.Node{
		Name:       name(u.Fragment, u.Hostname(), port),
		Protocol:   "tuic",
		Server:     u.Hostname(),
		Port:       port,
		UUID:       uuid,
		Password:   password,
		TLS:        "tls",
		ServerName: first(q.Get("sni"), q.Get("peer")),
		Insecure:   q.Get("allow_insecure") == "1" || q.Get("insecure") == "1",
		ALPN:       list(q.Get("alpn")),
		Congestion: q.Get("congestion_control"),
	}, nil
}

func shadowsocks(link string) (core.Node, error) {
	body := strings.TrimPrefix(link, "ss://")
	given := ""
	if i := strings.LastIndex(body, "#"); i >= 0 {
		given, _ = url.QueryUnescape(body[i+1:])
		body = body[:i]
	}
	if i := strings.Index(body, "?"); i >= 0 {

		body = body[:i]
	}

	var credentials, address string
	if at := strings.LastIndex(body, "@"); at >= 0 {
		part := body[:at]
		if decoded, ok := decode(part); ok {
			credentials = decoded
		} else if strings.Contains(part, ":") {
			credentials = part
		} else {
			return core.Node{}, errors.New(i18n.T("ss: 凭据不是 base64"))
		}
		address = body[at+1:]
	} else {
		decoded, ok := decode(body)
		if !ok {
			return core.Node{}, errors.New(i18n.T("ss: 不是 base64"))
		}
		at := strings.LastIndex(decoded, "@")
		if at < 0 {
			return core.Node{}, errors.New(i18n.T("ss: 缺少地址"))
		}
		credentials, address = decoded[:at], decoded[at+1:]
	}

	method, password, ok := strings.Cut(credentials, ":")
	if !ok {
		return core.Node{}, errors.New(i18n.T("ss: 缺少密码"))
	}
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return core.Node{}, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return core.Node{}, err
	}
	return core.Node{
		Name:     name(given, host, port),
		Protocol: "shadowsocks",
		Server:   host,
		Port:     port,
		Method:   method,
		Password: password,
	}, nil
}

func decode(text string) (string, bool) {
	text = strings.TrimSpace(text)
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		decoded, err := encoding.DecodeString(text)
		if err == nil {
			return string(decoded), true
		}
	}
	return "", false
}

func number(raw json.RawMessage) (int, error) {
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0, errors.New(i18n.T("端口不是数字"))
	}
	return strconv.Atoi(text)
}

func network(kind string) (string, error) {
	switch kind {
	case "", "tcp", "raw":
		return "", nil
	case "ws", "grpc", "http", "httpupgrade":
		return kind, nil
	case "h2", "http2":
		return "http", nil
	}
	return "", fmt.Errorf(i18n.T("不支持的网络 %s"), kind)
}

func security(kind string) (string, error) {
	switch kind {
	case "", "none":
		return "", nil
	case "tls", "reality":
		return kind, nil
	}
	return "", fmt.Errorf(i18n.T("不支持的安全类型 %s"), kind)
}

func list(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func name(given, server string, port int) string {
	if given != "" {
		return given
	}
	return net.JoinHostPort(server, strconv.Itoa(port))
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

type subscription struct {
	Proxies   []map[string]any `json:"proxies"`
	Outbounds []map[string]any `json:"outbounds"`
}

var clashProtocols = map[string]string{
	"ss": "shadowsocks", "shadowsocks": "shadowsocks",
	"socks": "socks", "socks5": "socks",
	"http":  "http",
	"vmess": "vmess", "vless": "vless", "trojan": "trojan",
	"hysteria2": "hysteria2", "hy2": "hysteria2",
	"tuic": "tuic",
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
		doc = subscription{Proxies: maps(whole["proxies"]), Outbounds: maps(whole["outbounds"])}
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
	port := num(one, "port")
	if server == "" || port <= 0 || port > 65535 {
		return core.Node{}, errors.New(i18n.T("缺少服务器或端口"))
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
	}
	switch protocol {
	case "vmess":
		node.UUID = field(one, "uuid")
		node.AlterID = num(one, "alterId")
		node.Security = field(one, "cipher")
	case "vless":
		node.UUID = field(one, "uuid")
	case "trojan":
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
	port := num(one, "server_port")
	if server == "" || port <= 0 || port > 65535 {
		return core.Node{}, errors.New(i18n.T("缺少服务器或端口"))
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
	case "trojan":
		node.Password = field(one, "password")
	case "shadowsocks":
		node.Method = field(one, "method")
		node.Password = field(one, "password")
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

func field(one map[string]any, key string) string {
	if value, ok := one[key]; ok && value != nil {
		return fmt.Sprint(value)
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
