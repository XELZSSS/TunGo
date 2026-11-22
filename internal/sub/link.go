package sub

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

func proxy(link string) (core.Node, error) {
	u, err := url.Parse(link)
	if err != nil {
		return core.Node{}, err
	}

	if u.Path != "" && u.Path != "/" {
		return core.Node{}, errors.New(i18n.T("不是节点"))
	}
	protocol := "socks"
	if strings.HasPrefix(u.Scheme, "http") {
		protocol = "http"
	}
	port, err := portText(protocol, u.Port())
	if err != nil {
		return core.Node{}, err
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
		PS       string          `json:"ps"`
		Add      string          `json:"add"`
		Port     json.RawMessage `json:"port"`
		ID       string          `json:"id"`
		Aid      json.RawMessage `json:"aid"`
		Scy      string          `json:"scy"`
		Net      string          `json:"net"`
		Path     string          `json:"path"`
		Host     string          `json:"host"`
		TLS      string          `json:"tls"`
		SNI      string          `json:"sni"`
		ALPN     string          `json:"alpn"`
		FP       string          `json:"fp"`
		ECH      string          `json:"ech"`
		Pcs      string          `json:"pcs"`
		Pbk      string          `json:"pbk"`
		Sid      string          `json:"sid"`
		Insecure string          `json:"allowInsecure"`
	}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return core.Node{}, err
	}
	port, err := number(v.Port)
	if err != nil {
		return core.Node{}, err
	}
	if port, err = portNumber("vmess", port); err != nil {
		return core.Node{}, err
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
		ECH:         ech(v.ECH),
		Pinned:      v.Pcs,
		PublicKey:   v.Pbk,
		ShortID:     v.Sid,
		Insecure:    truthy(v.Insecure),
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
	port, err := portText("vless", u.Port())
	if err != nil {
		return core.Node{}, err
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
		Insecure:    truthy(q.Get("allowInsecure")),
		PublicKey:   q.Get("pbk"),
		ShortID:     q.Get("sid"),
		ECH:         ech(q.Get("ech")),
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
	port, err := portText("trojan", u.Port())
	if err != nil {
		return core.Node{}, err
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
		Insecure:    truthy(q.Get("allowInsecure")),
		PublicKey:   q.Get("pbk"),
		ShortID:     q.Get("sid"),
		ECH:         ech(q.Get("ech")),
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
	port, err := portText("hysteria2", u.Port())
	if err != nil {
		return core.Node{}, err
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
		Insecure:     truthy(q.Get("insecure")),
		ALPN:         list(q.Get("alpn")),
		Obfs:         q.Get("obfs"),
		ObfsPassword: q.Get("obfs-password"),
		ECH:          ech(q.Get("ech")),
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
	port, err := portText("tuic", u.Port())
	if err != nil {
		return core.Node{}, err
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
		Insecure:   truthy(first(q.Get("allow_insecure"), q.Get("insecure"))),
		ALPN:       list(q.Get("alpn")),
		Congestion: q.Get("congestion_control"),
	}, nil
}

func anytls(link string) (core.Node, error) {
	u, err := url.Parse(link)
	if err != nil {
		return core.Node{}, err
	}
	if u.User == nil {
		return core.Node{}, errors.New(i18n.T("anytls: 缺少密码"))
	}
	port, err := portText("anytls", u.Port())
	if err != nil {
		return core.Node{}, err
	}
	password := u.User.Username()
	if given, ok := u.User.Password(); ok {
		password = given
	}
	q := u.Query()
	return core.Node{
		Name:       name(u.Fragment, u.Hostname(), port),
		Protocol:   "anytls",
		Server:     u.Hostname(),
		Port:       port,
		Password:   password,
		TLS:        "tls",
		ServerName: first(q.Get("sni"), q.Get("peer")),
		Insecure:   truthy(q.Get("insecure")),
		ALPN:       list(q.Get("alpn")),
	}, nil
}

func shadowsocks(link string) (core.Node, error) {
	body := strings.TrimPrefix(link, "ss://")
	given := ""
	query := ""
	if i := strings.LastIndex(body, "#"); i >= 0 {
		given, _ = url.QueryUnescape(body[i+1:])
		body = body[:i]
	}
	if i := strings.Index(body, "?"); i >= 0 {
		query = body[i+1:]
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
	address = strings.TrimSuffix(address, "/")
	host, portNumber, err := net.SplitHostPort(address)
	if err != nil {
		return core.Node{}, err
	}
	port, err := portText("ss", portNumber)
	if err != nil {
		return core.Node{}, err
	}
	node := core.Node{
		Name:     name(given, host, port),
		Protocol: "shadowsocks",
		Server:   host,
		Port:     port,
		Method:   method,
		Password: password,
	}
	if query != "" {
		values, _ := url.ParseQuery(query)
		if plugin := values.Get("plugin"); plugin != "" {
			parts := strings.Split(plugin, ";")
			node.Plugin = parts[0]
			node.PluginOpts = strings.Join(parts[1:], ";")
		}
	}
	return node, nil
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

func portNumber(protocol string, port int) (int, error) {
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf(i18n.T("%s: 端口不是 1 到 65535 的数字"), protocol)
	}
	return port, nil
}

func portText(protocol, text string) (int, error) {
	port, err := strconv.Atoi(text)
	if err != nil {
		port = 0
	}
	return portNumber(protocol, port)
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

func truthy(value string) bool { return value == "1" || strings.EqualFold(value, "true") }

func ech(value string) string {
	if _, ok := decode(value); ok {
		return value
	}
	return ""
}
