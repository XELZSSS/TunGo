package core

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const (
	proxyTag    = "proxy"
	autoTag     = "proxy-auto"
	balancerTag = "proxy-balancer"
)

const detourTag = "detour"

func Config(kind Kind, n Node, inbound string, policy Policy, clash string, tun bool) ([]byte, error) {
	var config map[string]any
	switch kind {
	case SingBox:
		outbounds, err := singBoxOutbounds(n)
		if err != nil {
			return nil, err
		}
		in, err := singBoxInbound(inbound)
		if err != nil {
			return nil, err
		}
		route, err := singBoxRoute(policy.Routing)
		if err != nil {
			return nil, err
		}
		inbounds := []any{in}
		if tun {
			inbounds = append(inbounds, singBoxTun())
			rules := []any{
				map[string]any{"action": "sniff"},
				map[string]any{"protocol": "dns", "action": "hijack-dns"},
			}
			route["rules"] = append(rules, route["rules"].([]any)...)
		}
		config = map[string]any{
			"log":      map[string]any{"level": "warn"},
			"dns":      singBoxDNS(policy.DNS, policy.Routing),
			"inbounds": inbounds,
			"route":    route,
			"outbounds": append(outbounds,
				map[string]any{"type": "direct", "tag": "direct"},
				map[string]any{"type": "block", "tag": "block"},
			),

			"experimental": map[string]any{
				"clash_api": map[string]any{"external_controller": clash},
			},
		}
	case Xray:
		outbounds, err := xrayOutbounds(n)
		if err != nil {
			return nil, err
		}
		in, err := xrayInbound(inbound)
		if err != nil {
			return nil, err
		}
		inbounds := []any{in}
		if tun {
			inbounds = append(inbounds, xrayTun())
		}
		routing, err := xrayRouting(policy.Routing, n)
		if err != nil {
			return nil, err
		}
		config = map[string]any{
			"log":      map[string]any{"loglevel": "warning"},
			"dns":      xrayDNS(policy.DNS, policy.Routing),
			"inbounds": inbounds,
			"outbounds": append(outbounds,
				map[string]any{"tag": "direct", "protocol": "freedom"},
				map[string]any{"tag": "block", "protocol": "blackhole"},
			),
			"routing": routing,
		}

		for key, value := range xrayMeasure(n) {
			config[key] = value
		}
	default:
		return nil, fmt.Errorf("core: no config for %q", kind)
	}
	return json.MarshalIndent(config, "", "  ")
}

const (
	tunTag       = "tun"
	tunInterface = "TunGo"
	tunAddress   = "172.19.0.1/30"
	tunMTU       = 9000
)

func securityOf(n Node) string {
	if n.TLS != "" && n.TLS != "none" {
		return n.TLS
	}
	switch n.Protocol {
	case "trojan", "hysteria2", "tuic":

		return "tls"
	}
	return ""
}

func carriesTLS(protocol string) bool {
	switch protocol {
	case "vmess", "vless", "trojan", "hysteria2", "tuic":
		return true
	}
	return false
}

func carriesTransport(protocol string) bool {
	switch protocol {
	case "vmess", "vless", "trojan":
		return true
	}
	return false
}

func vmessSecurity(n Node) string {
	if n.Security != "" {
		return n.Security
	}
	return "auto"
}

func serverName(n Node) string {
	if n.ServerName != "" {
		return n.ServerName
	}
	return n.Server
}

func hosts(n Node) []any {
	if n.Host != "" {
		return []any{n.Host}
	}
	return []any{serverName(n)}
}

func hostName(n Node) string {
	if n.Host != "" {
		return n.Host
	}
	return serverName(n)
}

func wsPath(n Node) string {
	if strings.HasPrefix(n.Path, "/") {
		return n.Path
	}
	if n.Path == "" {
		return "/"
	}
	return "/" + n.Path
}

func earlyData(path string) (string, int) {
	i := strings.Index(path, "?ed=")
	if i < 0 {
		return path, 0
	}
	size, err := strconv.Atoi(path[i+len("?ed="):])
	if err != nil {
		return path, 0
	}
	return path[:i], size
}

func anyOf(values []string) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}

func dnsServer(tag, address, detour string) map[string]any {
	scheme, host, path := splitDNS(address)
	server := map[string]any{"tag": tag, "server": host}
	switch scheme {
	case "https", "h2":
		server["type"] = "https"
		if path != "" {
			server["path"] = path
		}
	case "tls", "quic", "h3", "udp", "tcp":
		server["type"] = scheme
	default:
		server["type"] = "udp"
	}
	if detour != "" {
		server["detour"] = detour
	}
	return server
}

func splitDNS(address string) (scheme, host, path string) {
	address = strings.TrimSpace(address)
	if address == "" || !strings.Contains(address, "://") {
		return "", address, ""
	}
	u, err := url.Parse(address)
	if err != nil {
		return "", address, ""
	}
	return u.Scheme, u.Host, u.Path
}

func anyOfInt(values []int) []any {
	out := make([]any, 0, len(values))
	for _, v := range values {
		out = append(out, v)
	}
	return out
}

func directSuffixes(routing Routing) []string {
	var out []string
	for _, rule := range routing.Rules {
		if rule.Out != "direct" {
			continue
		}
		for _, entry := range rule.Domain {
			if !strings.HasPrefix(entry, "geosite:") {
				out = append(out, entry)
			}
		}
	}
	return out
}

func domainsOf(entries []string) (suffix, full, keyword, regex []string) {
	for _, entry := range entries {
		switch {
		case entry == "":
		case strings.HasPrefix(entry, "full:"):
			full = append(full, strings.TrimPrefix(entry, "full:"))
		case strings.HasPrefix(entry, "keyword:"):
			keyword = append(keyword, strings.TrimPrefix(entry, "keyword:"))
		case strings.HasPrefix(entry, "regexp:"):
			regex = append(regex, strings.TrimPrefix(entry, "regexp:"))
		default:
			suffix = append(suffix, strings.TrimPrefix(entry, "domain:"))
		}
	}
	return suffix, full, keyword, regex
}

func domainRule(entries []string, server string) map[string]any {
	suffix, full, keyword, regex := domainsOf(entries)
	rule := map[string]any{"server": server}
	if len(suffix) > 0 {
		rule["domain_suffix"] = anyOf(suffix)
	}
	if len(full) > 0 {
		rule["domain"] = anyOf(full)
	}
	if len(keyword) > 0 {
		rule["domain_keyword"] = anyOf(keyword)
	}
	if len(regex) > 0 {
		rule["domain_regex"] = anyOf(regex)
	}
	if len(rule) == 1 {
		return nil
	}
	return rule
}

func suffixDomains(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v == "" {
			continue
		}
		if !strings.Contains(v, ":") {
			v = "domain:" + v
		}
		out = append(out, v)
	}
	return out
}

func alpnOf(n Node) []string {
	if len(n.ALPN) > 0 {
		return n.ALPN
	}
	switch n.Protocol {
	case "hysteria2", "tuic":

		return []string{"h3"}
	}
	switch n.Network {
	case "ws", "httpupgrade", "http":

		return []string{"http/1.1"}
	case "grpc":
		return []string{"h2"}
	}
	return nil
}
