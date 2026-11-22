package core

import (
	"net"
	"sort"
	"strconv"
)

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
