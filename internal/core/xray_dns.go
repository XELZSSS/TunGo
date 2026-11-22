package core

import (
	"net"
)

func xrayDNS(dns DNS, routing Routing, bootstrap string) map[string]any {
	servers := []any{}
	if bootstrap != "" {
		_, host, _ := splitDNS(bootstrap)
		if net.ParseIP(host) != nil {
			servers = append(servers, map[string]any{"address": host, "domains": []any{"dns.google", "cloudflare-dns.com", "doh.pub"}, "skipFallback": true})
		}
	}
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
	strategy := xrayStrategy(dns.Strategy)
	if dns.BlockAAAA {
		strategy = "UseIPv4"
	}
	out := map[string]any{
		"servers":       servers,
		"queryStrategy": strategy,
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
