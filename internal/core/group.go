package core

import "fmt"

const probeURL = "https://www.google.com/generate_204"

func singBoxOutbounds(n Node) ([]any, error) {
	if n.Group() {
		return singBoxGroup(n)
	}
	return nodeOutbounds(n, singBoxOutbound)
}

func singBoxGroup(n Node) ([]any, error) {
	members := make([]any, 0, len(n.Members))
	tags := make([]any, 0, len(n.Members))
	for i, member := range n.Members {
		outbound, err := buildNodeOutbounds(member, singBoxOutbound)
		if err != nil {
			return nil, err
		}
		tag := memberTag(i, member)
		outbound.main["tag"] = tag
		members = append(members, outbound.main)
		members = append(members, outbound.extras...)
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
			outbound, err := buildNodeOutbounds(member, xrayOutbound)
			if err != nil {
				return nil, err
			}
			outbound.main["tag"] = memberTag(i, member)
			members = append(members, outbound.main)
			members = append(members, outbound.extras...)
		}
		return members, nil
	}
	return nodeOutbounds(n, xrayOutbound)
}

type outboundBuilder func(Node) (map[string]any, error)

type nodeOutboundsResult struct {
	main   map[string]any
	extras []any
}

func buildNodeOutbounds(n Node, build outboundBuilder) (nodeOutboundsResult, error) {
	outbound, err := build(n)
	if err != nil {
		return nodeOutboundsResult{}, err
	}
	result := nodeOutboundsResult{main: outbound}
	if n.Detour == nil || n.Detour.Direct() {
		return result, nil
	}
	detour, err := build(*n.Detour)
	if err != nil {
		return nodeOutboundsResult{}, err
	}
	tag := detourTag + "-" + n.Name + "-" + outboundTag(outbound)
	detour["tag"] = tag
	if _, ok := outbound["type"]; ok {
		outbound["detour"] = tag
	} else {
		outbound["sockopt"] = map[string]any{"dialerProxy": tag}
	}
	result.extras = append(result.extras, detour)
	return result, nil
}

func nodeOutbounds(n Node, build outboundBuilder) ([]any, error) {
	result, err := buildNodeOutbounds(n, build)
	if err != nil {
		return nil, err
	}
	return append([]any{result.main}, result.extras...), nil
}

func outboundTag(outbound map[string]any) string {
	if tag, ok := outbound["tag"].(string); ok {
		return tag
	}
	return ""
}

func xrayMeasure(n Node, targets []Node) map[string]any {
	subject := []any{}
	observatory, burst := false, false
	watch := func(node Node, prefix string) {
		switch node.StrategyOr() {
		case "fallback", "leastLoad":
			burst = true
		case "leastPing":
			observatory = true
		default:
			return
		}
		subject = append(subject, prefix)
	}
	if n.Group() {
		watch(n, proxyTag+"-")
	}
	for _, target := range targets {
		if target.Group() {
			watch(target, TargetTag(target.Name)+"/")
		}
	}
	out := map[string]any{}
	if burst {
		out["burstObservatory"] = map[string]any{
			"subjectSelector": subject,
			"pingConfig": map[string]any{
				"destination": probeURL,
				"interval":    "5m",
				"timeout":     "30s",
				"sampling":    2,
			},
		}
	}
	if observatory {
		out["observatory"] = map[string]any{
			"subjectSelector":   subject,
			"probeUrl":          probeURL,
			"probeInterval":     "3m",
			"enableConcurrency": true,
		}
	}
	return out
}

func xrayBalancer(strategy string, n Node) map[string]any {
	selector := []any{proxyTag + "-"}
	fallback := strategy == "fallback"
	for _, member := range n.Members {
		if member.NeedsPin() {
			fallback = true
			break
		}
	}
	if fallback {
		return map[string]any{
			"tag":      balancerTag,
			"type":     "fallback",
			"selector": selector,
		}
	}
	out := map[string]any{
		"tag":      balancerTag,
		"selector": []any{proxyTag + "-"},
		"strategy": map[string]any{"type": xrayBalancerType(strategy)},
	}
	if strategy == "leastLoad" {
		out["strategy"].(map[string]any)["settings"] = map[string]any{"expected": 1}
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
