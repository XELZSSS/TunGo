package sub

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

func Fetch(from, ua string) ([]core.Node, Info, error) {
	return fetch(from, ua, &http.Client{Timeout: time.Minute})
}

func FetchProxied(from, socksAddr, ua string) ([]core.Node, Info, error) {
	addr, err := url.Parse("socks5://" + socksAddr)
	if err != nil {
		return nil, Info{}, err
	}
	transport := &http.Transport{Proxy: http.ProxyURL(addr), IdleConnTimeout: 30 * time.Second}
	defer transport.CloseIdleConnections()
	return fetch(from, ua, &http.Client{Timeout: time.Minute, Transport: transport})
}

func fetch(from, ua string, client *http.Client) ([]core.Node, Info, error) {
	req, err := http.NewRequest("GET", from, nil)
	if err != nil {
		return nil, Info{}, err
	}
	if ua == "" {
		ua = "TunGo"
	}
	req.Header.Set("User-Agent", ua)
	resp, err := client.Do(req)
	if err != nil {
		return nil, Info{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, Info{}, fmt.Errorf(i18n.T("订阅返回 %s"), resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if err != nil {
		return nil, Info{}, err
	}
	if len(body) > 8<<20 {
		return nil, Info{}, errors.New(i18n.T("订阅内容超过 8MB，已停止导入"))
	}
	nodes, err := Parse(body)
	return nodes, userInfo(resp.Header.Get("subscription-userinfo")), err
}

func userInfo(header string) Info {
	var info Info
	for _, field := range strings.Split(header, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(field), "=")
		if !ok {
			continue
		}
		number, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "upload", "download":
			info.Used += number
		case "total":
			info.Total = number
		case "expire":
			info.Expire = number
		}
	}
	return info
}

func Parse(data []byte) ([]core.Node, error) {
	text := strings.TrimSpace(strings.TrimPrefix(string(data), "\ufeff"))
	nodes, found, err := parseConfig(text)
	if !found {
		if decoded, ok := decode(text); ok {
			if decodedNodes, decodedFound, decodedErr := parseConfig(decoded); decodedFound {
				nodes, found, err = decodedNodes, true, decodedErr
				text = decoded
			} else if strings.Contains(text, "://") {
				text = decoded
			}
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
	case strings.HasPrefix(link, "anytls://"):
		return anytls(link)
	case strings.HasPrefix(link, "socks://"), strings.HasPrefix(link, "socks5://"),
		strings.HasPrefix(link, "http://"), strings.HasPrefix(link, "https://"):
		return proxy(link)
	}
	return core.Node{}, errors.New(i18n.T("不认识的链接"))
}
