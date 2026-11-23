package core

import (
	"fmt"
	"strconv"
	"strings"

	"tungo/internal/i18n"
)

func xrayRouting(routing Routing, n Node, targets []Node) (map[string]any, error) {
	balancer := ""
	if n.Group() {
		balancer = balancerTag
	}
	grouped := map[string]bool{}
	extraBalancers := []any{}
	for _, target := range targets {
		if !target.Group() {
			continue
		}
		tag := TargetTag(target.Name)
		grouped[tag] = true
		extraBalancers = append(extraBalancers, xrayTargetBalancer(target, tag))
	}
	out := func(rule string) map[string]any {
		if rule == proxyTag && balancer != "" {
			return map[string]any{"balancerTag": balancer}
		}
		if grouped[rule] {
			return map[string]any{"balancerTag": rule}
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
		var domains, ips []string
		var missing []string
		for _, entry := range rule.Domain {
			switch {
			case strings.HasPrefix(entry, "geosite:"):
				found := xrayGeo([]string{entry})
				if len(found) == 0 {
					missing = append(missing, entry)
				}
				domains = append(domains, found...)
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
			found := xrayGeoIPs([]string{entry})
			if len(found) == 0 {
				missing = append(missing, entry)
			}
			ips = append(ips, found...)
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf(i18n.T("xray 的地理规则展开后为空：%s 缺少对应的 geosite/geoip 列表"), strings.Join(missing, ", "))
		}
		if len(rule.Domain) > 0 && len(domains) == 0 {
			return nil, fmt.Errorf(i18n.T("xray 的域名规则展开后为空：%s 缺少对应的 geosite 列表"), strings.Join(rule.Domain, ", "))
		}
		if len(rule.IP) > 0 && len(ips) == 0 {
			return nil, fmt.Errorf(i18n.T("xray 的 IP 规则展开后为空：%s 缺少对应的 geoip 列表"), strings.Join(rule.IP, ", "))
		}
		if len(domains) > 0 {
			addRule(&rules, base, "domain", domains)
		}
		if len(ips) > 0 {
			addRule(&rules, base, "ip", ips)
		}
		if len(rule.Domain) == 0 && len(rule.IP) == 0 {
			rules = append(rules, base)
		}
	}
	final := map[string]any{"type": "field", "network": "tcp,udp"}
	for key, value := range out(routing.FinalTag()) {
		final[key] = value
	}
	rules = append(rules, final)
	strategy := routing.Strategy
	if strategy == "" {
		strategy = "IPIfNonMatch"
	}
	route := map[string]any{"domainStrategy": strategy, "rules": rules}
	balancers := []any{}
	if balancer != "" {
		balancers = append(balancers, xrayBalancer(n.StrategyOr(), n))
	}
	balancers = append(balancers, extraBalancers...)
	if len(balancers) > 0 {
		route["balancers"] = balancers
	}
	return route, nil
}
