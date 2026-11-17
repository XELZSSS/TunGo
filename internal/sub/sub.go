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

	"tungo/internal/core"
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
		return nil, fmt.Errorf("订阅返回 %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	return Parse(body)
}

func Parse(data []byte) ([]core.Node, error) {
	text := strings.TrimSpace(strings.TrimPrefix(string(data), "\ufeff"))
	if !strings.Contains(text, "://") {
		if decoded, ok := decode(text); ok {
			text = decoded
		}
	}
	var nodes []core.Node
	for _, line := range strings.Split(text, "\n") {
		link := strings.TrimSpace(line)
		if link == "" || strings.HasPrefix(link, "#") {
			continue
		}
		node, err := Link(link)
		if err != nil {

			continue
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, errors.New("订阅里没有认得的节点")
	}
	return nodes, nil
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
	return core.Node{}, errors.New("不认识的链接")
}

func proxy(link string) (core.Node, error) {
	u, err := url.Parse(link)
	if err != nil {
		return core.Node{}, err
	}

	if u.Path != "" && u.Path != "/" {
		return core.Node{}, errors.New("不是节点")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return core.Node{}, errors.New("缺少端口")
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
		return core.Node{}, errors.New("vmess: 不是 base64")
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

	aid, _ := number(v.Aid)
	return core.Node{
		Name:        name(v.PS, v.Add, port),
		Protocol:    "vmess",
		Server:      v.Add,
		Port:        port,
		AlterID:     aid,
		UUID:        v.ID,
		Security:    v.Scy,
		Network:     network(v.Net),
		Path:        v.Path,
		Host:        v.Host,
		TLS:         security(v.TLS),
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
		return core.Node{}, errors.New("vless: 缺少用户")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return core.Node{}, errors.New("vless: 缺少端口")
	}
	q := u.Query()
	return core.Node{
		Name:        name(u.Fragment, u.Hostname(), port),
		Protocol:    "vless",
		Server:      u.Hostname(),
		Port:        port,
		UUID:        u.User.Username(),
		Flow:        q.Get("flow"),
		Network:     network(q.Get("type")),
		Path:        first(q.Get("path"), q.Get("serviceName")),
		Host:        q.Get("host"),
		TLS:         security(q.Get("security")),
		ServerName:  first(q.Get("sni"), q.Get("host")),
		ALPN:        list(q.Get("alpn")),
		Fingerprint: q.Get("fp"),
		Insecure:    q.Get("allowInsecure") == "1",
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
		return core.Node{}, errors.New("trojan: 缺少密码")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return core.Node{}, errors.New("trojan: 缺少端口")
	}
	q := u.Query()
	return core.Node{
		Name:        name(u.Fragment, u.Hostname(), port),
		Protocol:    "trojan",
		Server:      u.Hostname(),
		Port:        port,
		Password:    u.User.Username(),
		Flow:        q.Get("flow"),
		Network:     network(q.Get("type")),
		Path:        first(q.Get("path"), q.Get("serviceName")),
		Host:        q.Get("host"),
		TLS:         security(first(q.Get("security"), "tls")),
		ServerName:  first(q.Get("sni"), q.Get("peer"), q.Get("host")),
		ALPN:        list(q.Get("alpn")),
		Fingerprint: q.Get("fp"),
		Insecure:    q.Get("allowInsecure") == "1",
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
		return core.Node{}, errors.New("hysteria2: 缺少密码")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return core.Node{}, errors.New("hysteria2: 缺少端口")
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
		return core.Node{}, errors.New("tuic: 缺少凭据")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return core.Node{}, errors.New("tuic: 缺少端口")
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
		decoded, ok := decode(body[:at])
		if !ok {
			return core.Node{}, errors.New("ss: 凭据不是 base64")
		}
		credentials, address = decoded, body[at+1:]
	} else {
		decoded, ok := decode(body)
		if !ok {
			return core.Node{}, errors.New("ss: 不是 base64")
		}
		at := strings.LastIndex(decoded, "@")
		if at < 0 {
			return core.Node{}, errors.New("ss: 缺少地址")
		}
		credentials, address = decoded[:at], decoded[at+1:]
	}

	method, password, ok := strings.Cut(credentials, ":")
	if !ok {
		return core.Node{}, errors.New("ss: 缺少密码")
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
		return 0, errors.New("端口不是数字")
	}
	return strconv.Atoi(text)
}

func network(kind string) string {
	switch kind {
	case "ws", "grpc", "http", "httpupgrade":
		return kind
	case "h2", "http2":
		return "http"
	}
	return ""
}

func security(kind string) string {
	switch kind {
	case "tls", "reality":
		return kind
	}
	return ""
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
