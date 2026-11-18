package core

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
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
	}
	if SecurityOf(n) != "" && carriesTLS(n.Protocol) {
		out["tls"] = singBoxTLS(n)
	}
	if carriesTransport(n.Protocol) && n.Network != "" && n.Network != "tcp" {
		if transport := singBoxTransport(n); transport != nil {
			out["transport"] = transport
		}
	}
	return out, nil
}

func singBoxTLS(n Node) map[string]any {
	tls := map[string]any{"enabled": true, "server_name": serverName(n)}
	if n.Insecure {
		tls["insecure"] = true
	}
	if alpn := alpnOf(n); len(alpn) > 0 {
		tls["alpn"] = alpn
	}
	if n.ECH != "" && !strings.Contains(n.ECH, "://") {
		tls["ech"] = singBoxECH(n.ECH)
	}
	fingerprint := n.Fingerprint
	if SecurityOf(n) == "reality" && fingerprint == "" {
		fingerprint = "chrome"
	}
	if fingerprint != "" {
		tls["utls"] = map[string]any{"enabled": true, "fingerprint": fingerprint}
	}
	if SecurityOf(n) == "reality" {
		tls["reality"] = map[string]any{
			"enabled":    true,
			"public_key": n.PublicKey,
			"short_id":   n.ShortID,
		}
	}
	return tls
}

func singBoxTransport(n Node) map[string]any {
	switch n.Network {
	case "ws":

		path, early := earlyData(wsPath(n))
		ws := map[string]any{"type": "ws", "path": path}
		if early > 0 {
			ws["max_early_data"] = early
			ws["early_data_header_name"] = "Sec-WebSocket-Protocol"
		}
		if n.Host != "" {
			ws["headers"] = map[string]any{"Host": n.Host}
		}
		return ws
	case "grpc":
		return map[string]any{"type": "grpc", "service_name": n.Path}
	case "http":
		return map[string]any{"type": "http", "host": []string{hostName(n)}, "path": wsPath(n)}
	case "httpupgrade":
		upgrade := map[string]any{"type": "httpupgrade", "path": wsPath(n)}
		if n.Host != "" {
			upgrade["host"] = n.Host
		}
		return upgrade
	}
	return nil
}

func singBoxDNS(dns DNS, routing Routing) map[string]any {
	servers := []any{
		dnsServer("remote", dns.Remote, "proxy"),

		dnsServer("direct", dns.Direct, ""),
	}

	if dns.Bootstrap != "" {
		_, host, _ := splitDNS(dns.Bootstrap)
		bootstrapIP := net.ParseIP(host) != nil
		servers = append(servers, dnsServer("bootstrap", dns.Bootstrap, ""))
		if bootstrapIP {
			for _, one := range servers[:2] {
				if server, ok := one.(map[string]any); ok {
					server["domain_resolver"] = "bootstrap"
				}
			}
		}
	}
	rules := []any{}

	for i, split := range dns.Split {
		if len(split.Domains) == 0 || split.Server == "" {
			continue
		}
		tag := "split-" + strconv.Itoa(i)
		servers = append(servers, dnsServer(tag, split.Server, ""))
		if rule := domainRule(split.Domains, tag); rule != nil {
			rules = append(rules, rule)
		}
	}
	if rule := domainRule(directSuffixes(routing), "direct"); rule != nil {
		rules = append(rules, rule)
	}
	if len(dns.Hosts) > 0 {

		predefined := map[string]any{}
		names := make([]string, 0, len(dns.Hosts))
		for name, address := range dns.Hosts {
			predefined[name] = address
			names = append(names, name)
		}
		sort.Strings(names)
		servers = append(servers, map[string]any{
			"type": "hosts", "tag": "hosts", "predefined": predefined,
		})
		rules = append(rules, map[string]any{"domain": names, "server": "hosts"})
	}
	if dns.BlockAAAA {

		rules = append(rules, map[string]any{"query_type": []any{"AAAA"}, "action": "reject"})
	}
	out := map[string]any{
		"servers":  servers,
		"final":    "remote",
		"strategy": dns.Strategy,
	}
	if len(rules) > 0 {
		out["rules"] = rules
	}
	return out
}

func singBoxRoute(routing Routing) (map[string]any, error) {
	rules := []any{}
	sets := []any{}
	seen := map[string]bool{}

	set := func(entry string) (tag string, domains, cidrs []string, err error) {
		kind, name := geoKindTag(entry)
		tag = kind + "-" + name
		if seen[tag] {
			return tag, nil, nil, nil
		}
		if path := ruleSetPath(tag + ".srs"); path != "" {
			seen[tag] = true
			sets = append(sets, map[string]any{"type": "local", "tag": tag, "format": "binary", "path": path})
			return tag, nil, nil, nil
		}
		if kind == "geoip" {
			cidrs = fileLines(ruleSetPath(tag + ".cidrs.txt"))
		} else {
			domains = fileLines(ruleSetPath(tag + ".domains.txt"))
		}
		if len(domains) == 0 && len(cidrs) == 0 {
			return "", nil, nil, fmt.Errorf(i18n.T("缺少规则集 %s：把 %s.srs 或 %s.domains.txt / .cidrs.txt 放进 %s"), entry, tag, tag, ruleSetDir())
		}
		return "", domains, cidrs, nil
	}
	for _, rule := range routing.Rules {
		if rule.Bare() {
			continue
		}
		base := map[string]any{"outbound": rule.Out}
		if ports, ranges := rule.Ports(); len(ports) > 0 || len(ranges) > 0 {
			if len(ports) > 0 {
				base["port"] = ports
			}
			if len(ranges) > 0 {
				base["port_range"] = ranges
			}
		}
		if network := rule.NetworkList(); len(network) > 0 {
			base["network"] = network
		}
		if len(rule.Protocol) > 0 {
			base["protocol"] = rule.Protocol
		}

		var entries, ruleSets, ips []string
		private := false
		for _, entry := range rule.Domain {
			if !strings.HasPrefix(entry, "geosite:") {
				entries = append(entries, entry)
				continue
			}
			tag, inline, _, err := set(entry)
			if err != nil {
				return nil, err
			}
			if tag != "" {
				ruleSets = append(ruleSets, tag)
			}
			entries = append(entries, inline...)
		}
		suffix, full, keyword, regex := domainsOf(entries)
		for _, entry := range rule.IP {
			if !strings.HasPrefix(entry, "geoip:") {
				ips = append(ips, entry)
				continue
			}
			if strings.TrimPrefix(entry, "geoip:") == "private" {
				private = true
				continue
			}
			tag, _, cidrs, err := set(entry)
			if err != nil {
				return nil, err
			}
			if tag != "" {
				ruleSets = append(ruleSets, tag)
			}
			ips = append(ips, cidrs...)
		}
		for _, one := range []struct {
			key   string
			value any
			on    bool
		}{
			{"domain_suffix", suffix, len(suffix) > 0},
			{"domain", full, len(full) > 0},
			{"domain_keyword", keyword, len(keyword) > 0},
			{"domain_regex", regex, len(regex) > 0},
			{"rule_set", ruleSets, len(ruleSets) > 0},
			{"ip_cidr", ips, len(ips) > 0},
			{"ip_is_private", true, private},
		} {
			if one.on {
				addRule(&rules, base, one.key, one.value)
			}
		}

		if len(rule.Domain) == 0 && len(rule.IP) == 0 {
			rules = append(rules, base)
		}
	}
	return map[string]any{
		"rules":                   rules,
		"rule_set":                sets,
		"final":                   routing.FinalTag(),
		"default_domain_resolver": "direct",
		"auto_detect_interface":   true,
	}, nil
}

func fileLines(path string) []string {
	return cachedList(path, func(lines []string) []string { return lines })
}
