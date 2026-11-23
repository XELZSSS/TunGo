package core

import "fmt"

const targetPrefix = "out:"

func TargetTag(name string) string { return targetPrefix + name }

func IsOut(out string) bool {
	for _, one := range Outs {
		if one == out {
			return true
		}
	}
	return false
}

func routed(r Routing, targets []Node) Routing {
	if len(targets) == 0 {
		return r
	}
	known := make(map[string]bool, len(targets))
	for _, target := range targets {
		known[target.Name] = true
	}
	rules := make([]Rule, len(r.Rules))
	for i, rule := range r.Rules {
		if known[rule.Out] {
			rule.Out = TargetTag(rule.Out)
		}
		rules[i] = rule
	}
	r.Rules = rules
	return r
}

func targetOutbounds(kind Kind, targets []Node) ([]any, error) {
	var out []any
	for _, target := range targets {
		var built []any
		var err error
		if kind == Xray {
			built, err = xrayTarget(target)
		} else {
			built, err = singBoxTarget(target)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, built...)
	}
	return out, nil
}

func singBoxTarget(n Node) ([]any, error) {
	tag := TargetTag(n.Name)
	if !n.Group() {
		built, err := buildNodeOutbounds(n, singBoxOutbound)
		if err != nil {
			return nil, err
		}
		built.main["tag"] = tag
		return append([]any{built.main}, built.extras...), nil
	}
	members, tags, err := targetMembers(n, singBoxOutbound, tag)
	if err != nil {
		return nil, err
	}
	auto := map[string]any{
		"type":                        "urltest",
		"tag":                         tag + "/auto",
		"outbounds":                   tags,
		"interrupt_exist_connections": false,
	}
	if n.StrategyOr() == "fallback" {
		auto["tolerance"] = 5000
	}
	selector := map[string]any{
		"type":                        "selector",
		"tag":                         tag,
		"outbounds":                   append([]any{tag + "/auto"}, tags...),
		"interrupt_exist_connections": false,
	}
	return append([]any{selector, auto}, members...), nil
}

func xrayTarget(n Node) ([]any, error) {
	tag := TargetTag(n.Name)
	if !n.Group() {
		built, err := buildNodeOutbounds(n, xrayOutbound)
		if err != nil {
			return nil, err
		}
		built.main["tag"] = tag
		return append([]any{built.main}, built.extras...), nil
	}
	members, _, err := targetMembers(n, xrayOutbound, tag)
	return members, err
}

func targetMembers(n Node, build outboundBuilder, tag string) ([]any, []any, error) {
	members := make([]any, 0, len(n.Members))
	tags := make([]any, 0, len(n.Members))
	for i, member := range n.Members {
		built, err := buildNodeOutbounds(member, build)
		if err != nil {
			return nil, nil, err
		}
		name := fmt.Sprintf("%s/%d", tag, i+1)
		built.main["tag"] = name
		members = append(members, built.main)
		members = append(members, built.extras...)
		tags = append(tags, name)
	}
	return members, tags, nil
}

func xrayTargetBalancer(n Node, tag string) map[string]any {
	selector := []any{tag + "/"}
	if n.StrategyOr() == "fallback" {
		return map[string]any{"tag": tag, "type": "fallback", "selector": selector}
	}
	out := map[string]any{
		"tag":      tag,
		"selector": selector,
		"strategy": map[string]any{"type": xrayBalancerType(n.StrategyOr())},
	}
	if n.StrategyOr() == "leastLoad" {
		out["strategy"].(map[string]any)["settings"] = map[string]any{"expected": 1}
	}
	return out
}
