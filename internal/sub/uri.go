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

func URI(n core.Node) (string, error) {
	switch n.Protocol {
	case "vmess":
		return vmessURI(n)
	case "shadowsocks":
		return shadowsocksURI(n)
	case "hysteria2":
		return hysteria2URI(n)
	case "tuic":
		return tuicURI(n)
	case "vless", "trojan", "socks", "http":
		return linkURI(n)
	}
	return "", fmt.Errorf(i18n.T("不认识的协议 %s"), n.Protocol)
}

func linkURI(n core.Node) (string, error) {
	u := &url.URL{
		Scheme: n.Protocol,
		Host:   net.JoinHostPort(n.Server, strconv.Itoa(n.Port)),
	}
	switch n.Protocol {
	case "vless":
		u.User = url.User(n.UUID)
	case "trojan":
		u.User = url.User(n.Password)
	default:
		if n.Username != "" {
			u.User = url.UserPassword(n.Username, n.Password)
		}
	}
	q := url.Values{}
	if n.Protocol == "vless" {
		q.Set("encryption", "none")
	}
	if (n.Protocol == "vless" || n.Protocol == "trojan") && n.Flow != "" {
		q.Set("flow", n.Flow)
	}
	if n.Network != "" {
		q.Set("type", n.Network)
		if n.Network == "grpc" {
			q.Set("serviceName", n.Path)
		} else if n.Path != "" {
			q.Set("path", n.Path)
		}
	}
	if n.Host != "" {
		q.Set("host", n.Host)
	}
	sec := core.SecurityOf(n)
	if sec == "" {
		sec = "none"
	}
	q.Set("security", sec)
	if n.ServerName != "" {
		q.Set("sni", n.ServerName)
	}
	if len(n.ALPN) > 0 {
		q.Set("alpn", strings.Join(n.ALPN, ","))
	}
	if n.Fingerprint != "" {
		q.Set("fp", n.Fingerprint)
	}
	if n.Insecure {
		q.Set("allowInsecure", "1")
	}
	if n.PublicKey != "" {
		q.Set("pbk", n.PublicKey)
	}
	if n.ShortID != "" {
		q.Set("sid", n.ShortID)
	}
	if n.ECH != "" {
		q.Set("ech", n.ECH)
	}
	if n.Pinned != "" {
		q.Set("pcs", n.Pinned)
	}
	u.RawQuery = q.Encode()
	u.Fragment = n.Name
	return u.String(), nil
}

func shadowsocksURI(n core.Node) (string, error) {
	if n.Method == "" || n.Password == "" {
		return "", errors.New(i18n.T("ss：缺少加密方式或密码"))
	}
	credentials := base64.RawURLEncoding.EncodeToString([]byte(n.Method + ":" + n.Password))
	address := net.JoinHostPort(n.Server, strconv.Itoa(n.Port))
	link := "ss://" + credentials + "@" + address
	if n.Plugin != "" {
		plugin := n.Plugin
		if n.PluginOpts != "" {
			plugin += ";" + n.PluginOpts
		}
		link += "/?plugin=" + url.QueryEscape(plugin)
	}
	return link + "#" + url.QueryEscape(n.Name), nil
}

func hysteria2URI(n core.Node) (string, error) {
	u := &url.URL{
		Scheme: "hysteria2",
		Host:   net.JoinHostPort(n.Server, strconv.Itoa(n.Port)),
		User:   url.User(n.Password),
	}
	q := url.Values{}
	if n.ServerName != "" {
		q.Set("sni", n.ServerName)
	}
	if n.Insecure {
		q.Set("insecure", "1")
	}
	if n.Obfs != "" {
		q.Set("obfs", n.Obfs)
		q.Set("obfs-password", n.ObfsPassword)
	}
	if n.Pinned != "" {
		q.Set("pinSHA256", n.Pinned)
	}
	if len(n.ALPN) > 0 {
		q.Set("alpn", strings.Join(n.ALPN, ","))
	}
	if n.ECH != "" {
		q.Set("ech", n.ECH)
	}
	u.RawQuery = q.Encode()
	u.Fragment = n.Name
	return u.String(), nil
}

func tuicURI(n core.Node) (string, error) {
	u := &url.URL{
		Scheme: "tuic",
		Host:   net.JoinHostPort(n.Server, strconv.Itoa(n.Port)),
		User:   url.UserPassword(n.UUID, n.Password),
	}
	q := url.Values{}
	if n.ServerName != "" {
		q.Set("sni", n.ServerName)
	}
	if n.Congestion != "" {
		q.Set("congestion_control", n.Congestion)
	}
	if n.Insecure {
		q.Set("allow_insecure", "1")
	}
	if len(n.ALPN) > 0 {
		q.Set("alpn", strings.Join(n.ALPN, ","))
	}
	u.RawQuery = q.Encode()
	u.Fragment = n.Name
	return u.String(), nil
}

func vmessURI(n core.Node) (string, error) {
	transport := n.Network
	if transport == "" {
		transport = "tcp"
	}
	body := map[string]any{
		"v":    "2",
		"ps":   n.Name,
		"add":  n.Server,
		"port": strconv.Itoa(n.Port),
		"id":   n.UUID,
		"aid":  strconv.Itoa(n.AlterID),
		"scy":  vmessSecurity(n.Security),
		"net":  transport,
		"type": "none",
		"host": n.Host,
		"path": n.Path,
		"tls":  n.TLS,
		"sni":  n.ServerName,
	}
	if len(n.ALPN) > 0 {
		body["alpn"] = strings.Join(n.ALPN, ",")
	}
	if n.Fingerprint != "" {
		body["fp"] = n.Fingerprint
	}
	if n.Insecure {
		body["allowInsecure"] = "1"
	}
	if n.Pinned != "" {
		body["pcs"] = n.Pinned
	}
	if n.ECH != "" {
		body["ech"] = n.ECH
	}
	if n.PublicKey != "" {
		body["pbk"] = n.PublicKey
	}
	if n.ShortID != "" {
		body["sid"] = n.ShortID
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(data), nil
}

func vmessSecurity(given string) string {
	if given == "" {
		return "auto"
	}
	return given
}
