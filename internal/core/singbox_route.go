package core

import (
	"fmt"
	"strings"

	"tungo/internal/i18n"
)

func singBoxRoute(routing Routing) (map[string]any, error) {
	rules := []any{}
	sets := []any{}
	seen := map[string]bool{}

	set := func(entry string) (tag string, domains, cidrs []string, err error) {
		kind, name := geoKindTag(entry)
		tag = kind + "-" + name
		if kind == "geoip" && name == "private" {
			return "", nil, nil, nil
		}
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
