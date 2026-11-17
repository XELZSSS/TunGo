package core

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
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
	}, nil
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
			return nil, fmt.Errorf("自定义出站不是 JSON：%w", err)
		}
		raw["tag"] = "proxy"
		return raw, nil
	}
	if !Xray.Supports(n.Protocol) || n.Direct() {
		return nil, fmt.Errorf("xray 不支持 %q", n.Protocol)
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
		out["settings"] = map[string]any{"servers": []any{map[string]any{
			"address":  n.Server,
			"port":     n.Port,
			"method":   n.Method,
			"password": n.Password,
		}}}
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
	if carriesTLS(n.Protocol) || carriesTransport(n.Protocol) {
		if stream := xrayStream(n); len(stream) > 0 {
			out["streamSettings"] = stream
		}
	}
	return out, nil
}

func xrayStream(n Node) map[string]any {
	stream := map[string]any{}
	switch securityOf(n) {
	case "tls":
		tls := map[string]any{"serverName": serverName(n)}
		if alpn := alpnOf(n); len(alpn) > 0 {
			tls["alpn"] = alpn
		}
		if n.ECH != "" && !strings.Contains(n.ECH, "://") {
			tls["echConfigList"] = n.ECH
		}
		if n.Pinned != "" {
			tls["pinnedPeerCertSha256"] = n.Pinned
		}
		if n.Fingerprint != "" {
			tls["fingerprint"] = n.Fingerprint
		}

		if n.Insecure {
			tls["allowInsecure"] = true
		}
		stream["security"] = "tls"
		stream["tlsSettings"] = tls
	case "reality":
		reality := map[string]any{
			"serverName": serverName(n),
			"publicKey":  n.PublicKey,
			"shortId":    n.ShortID,
		}
		if n.Fingerprint != "" {
			reality["fingerprint"] = n.Fingerprint
		}
		stream["security"] = "reality"
		stream["realitySettings"] = reality
	}
	switch n.Network {
	case "ws":
		ws := map[string]any{"path": wsPath(n)}
		if n.Host != "" {
			ws["headers"] = map[string]any{"Host": n.Host}
		}
		stream["network"] = "ws"
		stream["wsSettings"] = ws
	case "grpc":
		stream["network"] = "grpc"
		stream["grpcSettings"] = map[string]any{"serviceName": n.Path}
	case "http":

		stream["network"] = "xhttp"
		stream["xhttpSettings"] = map[string]any{"host": hostName(n), "path": wsPath(n)}
	case "httpupgrade":
		stream["network"] = "httpupgrade"
		stream["httpupgradeSettings"] = map[string]any{"host": n.Host, "path": wsPath(n)}
	}
	return stream
}

func xrayDNS(dns DNS, routing Routing) map[string]any {
	servers := []any{}
	if values := suffixDomains(directSuffixes(routing)); len(values) > 0 {
		servers = append(servers, map[string]any{
			"address": dns.Direct, "domains": values, "skipFallback": true,
		})
	}
	for _, one := range dns.Split {
		if len(one.Domains) == 0 || one.Server == "" {
			continue
		}
		servers = append(servers, map[string]any{
			"address": one.Server, "domains": suffixDomains(one.Domains),
		})
	}
	servers = append(servers, map[string]any{"address": dns.Remote})
	out := map[string]any{

		"servers":       servers,
		"queryStrategy": xrayStrategy(dns.Strategy),
	}
	if dns.BlockAAAA && out["queryStrategy"] != "UseIPv6" {
		out["queryStrategy"] = "UseIPv4"
	}
	if len(dns.Hosts) > 0 {
		hosts := map[string]any{}
		for name, address := range dns.Hosts {
			hosts[name] = address
		}
		out["hosts"] = hosts
	}
	return out
}

func xrayStrategy(strategy string) string {
	switch strategy {
	case "prefer_ipv6", "ipv6_only":
		return "UseIPv6"
	}
	return "UseIPv4"
}

func xrayRouting(routing Routing, n Node) (map[string]any, error) {
	balancer := ""
	if n.Group() {
		balancer = balancerTag
	}
	out := func(rule string) map[string]any {
		if rule == proxyTag && balancer != "" {
			return map[string]any{"balancerTag": balancer}
		}
		return map[string]any{"outboundTag": rule}
	}
	rules := []any{}
	for _, rule := range routing.Rules {
		if rule.Bare() {
			continue
		}
		base := map[string]any{"type": "field"}
		for key, value := range out(rule.Out) {
			base[key] = value
		}
		if ports, ranges := rule.Ports(); len(ports) > 0 || len(ranges) > 0 {
			parts := make([]string, 0, len(ports)+len(ranges))
			for _, port := range ports {
				parts = append(parts, strconv.Itoa(port))
			}
			for _, one := range ranges {
				parts = append(parts, strings.Replace(one, ":", "-", 1))
			}
			base["port"] = strings.Join(parts, ",")
		}
		if network := rule.NetworkList(); len(network) > 0 {
			base["network"] = strings.Join(network, ",")
		}
		if len(rule.Protocol) > 0 {
			base["protocol"] = rule.Protocol
		}
		emit := func(key string, value any) {
			one := map[string]any{}
			for k, v := range base {
				one[k] = v
			}
			one[key] = value
			rules = append(rules, one)
		}
		var domains, ips []string
		for _, entry := range rule.Domain {
			switch {
			case strings.HasPrefix(entry, "geosite:"):
				domains = append(domains, xrayGeo([]string{entry})...)
			case strings.HasPrefix(entry, "full:"), strings.HasPrefix(entry, "keyword:"),
				strings.HasPrefix(entry, "regexp:"), strings.HasPrefix(entry, "domain:"):
				domains = append(domains, entry)
			default:
				domains = append(domains, "domain:"+entry)
			}
		}
		for _, entry := range rule.IP {
			if !strings.HasPrefix(entry, "geoip:") {
				ips = append(ips, entry)
				continue
			}
			ips = append(ips, xrayGeoIPs([]string{entry})...)
		}
		if len(rule.Domain) > 0 && len(domains) == 0 {
			return nil, fmt.Errorf("xray 的域名规则展开后为空：%s 缺少对应的 geosite 列表", strings.Join(rule.Domain, "、"))
		}
		if len(rule.IP) > 0 && len(ips) == 0 {
			return nil, fmt.Errorf("xray 的 IP 规则展开后为空：%s 缺少对应的 geoip 列表", strings.Join(rule.IP, "、"))
		}
		if len(domains) > 0 {
			emit("domain", domains)
		}
		if len(ips) > 0 {
			emit("ip", ips)
		}
		if len(rule.Domain) == 0 && len(rule.IP) == 0 {
			rules = append(rules, base)
		}
	}
	final := map[string]any{"type": "field", "port": "0-65535"}
	for key, value := range out(routing.FinalTag()) {
		final[key] = value
	}
	rules = append(rules, final)
	strategy := routing.Strategy
	if strategy == "" {
		strategy = "IPIfNonMatch"
	}
	route := map[string]any{"domainStrategy": strategy, "rules": rules}
	if balancer != "" {
		route["balancers"] = []any{xrayBalancer(n.StrategyOr())}
	}
	return route, nil
}
