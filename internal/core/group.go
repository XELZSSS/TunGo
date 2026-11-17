package core

import "fmt"

const probeURL = "https://www.google.com/generate_204"

func singBoxOutbounds(n Node) ([]any, error) {
	if n.Group() {
		return singBoxGroup(n)
	}
	outbound, err := singBoxOutbound(n)
	if err != nil {
		return nil, err
	}
	outbounds := []any{outbound}

	if n.Detour != nil && !n.Detour.Direct() {
		detour, err := singBoxOutbound(*n.Detour)
		if err != nil {
			return nil, err
		}
		detour["tag"] = detourTag
		outbound["detour"] = detourTag
		outbounds = append(outbounds, detour)
	}
	return outbounds, nil
}

func singBoxGroup(n Node) ([]any, error) {
	members := make([]any, 0, len(n.Members))
	tags := make([]any, 0, len(n.Members))
	for i, member := range n.Members {
		outbound, err := singBoxOutbound(member)
		if err != nil {
			return nil, err
		}
		tag := memberTag(i, member)
		outbound["tag"] = tag
		members = append(members, outbound)
		tags = append(tags, tag)
	}
	auto := map[string]any{
		"type":                        "urltest",
		"tag":                         autoTag,
		"outbounds":                   tags,
		"interrupt_exist_connections": false,
	}

	if n.StrategyOr() == "fallback" {
		auto["tolerance"] = 5000
	}
	out := map[string]any{
		"type":                        "selector",
		"tag":                         proxyTag,
		"outbounds":                   append([]any{autoTag}, tags...),
		"interrupt_exist_connections": false,
	}
	return append([]any{out, auto}, members...), nil
}

func xrayOutbounds(n Node) ([]any, error) {
	if n.Group() {
		members := make([]any, 0, len(n.Members))
		for i, member := range n.Members {
			outbound, err := xrayOutbound(member)
			if err != nil {
				return nil, err
			}
			outbound["tag"] = memberTag(i, member)
			members = append(members, outbound)
		}
		return members, nil
	}
	outbound, err := xrayOutbound(n)
	if err != nil {
		return nil, err
	}
	outbounds := []any{outbound}

	if n.Detour != nil && !n.Detour.Direct() {
		detour, err := xrayOutbound(*n.Detour)
		if err != nil {
			return nil, err
		}
		detour["tag"] = detourTag
		outbound["sockopt"] = map[string]any{"dialerProxy": detourTag}
		outbounds = append(outbounds, detour)
	}
	return outbounds, nil
}

func xrayMeasure(n Node) map[string]any {
	if !n.Group() {
		return nil
	}
	selector := []any{proxyTag + "-"}
	switch n.StrategyOr() {
	case "leastLoad", "fallback":
		return map[string]any{"burstObservatory": map[string]any{
			"subjectSelector": selector,
			"pingConfig": map[string]any{
				"destination": probeURL,
				"interval":    "5m",
				"timeout":     "30s",
				"sampling":    2,
			},
		}}
	case "leastPing":
		return map[string]any{"observatory": map[string]any{
			"subjectSelector":   selector,
			"probeUrl":          probeURL,
			"probeInterval":     "3m",
			"enableConcurrency": true,
		}}
	}
	return nil
}

func xrayBalancer(strategy string) map[string]any {
	out := map[string]any{
		"tag":      balancerTag,
		"selector": []any{proxyTag + "-"},
		"strategy": map[string]any{"type": xrayBalancerType(strategy)},
	}
	if strategy == "leastLoad" || strategy == "fallback" {
		settings := map[string]any{"expected": 1}
		if strategy == "fallback" {

			settings["tolerance"] = 0.2
			settings["maxRTT"] = "5000ms"
		}
		out["strategy"].(map[string]any)["settings"] = settings
	}
	return out
}

func xrayBalancerType(strategy string) string {
	switch strategy {
	case "random":
		return "random"
	case "roundRobin":
		return "roundRobin"
	case "leastPing":
		return "leastPing"
	}
	return "leastLoad"
}

func memberTag(i int, n Node) string {
	return fmt.Sprintf("%s-%d-%s", proxyTag, i+1, n.Name)
}
