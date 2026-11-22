package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

func (a *app) policyForms(c *ui.Context) {
	a.routeForm(c)
	a.ruleForm(c)
	a.routeNameForm(c)
	a.dnsForm(c)
}

func (a *app) routeForm(c *ui.Context) {
	if !a.editingRoute {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.editingRoute, func() {
		ui.Column(c).Gap(gapLoose).Width(660).Height(560).Children(func() {
			heading(c, fmt.Sprintf(i18n.T("路由规则 · %s"), routeLabel(a.routes.Current)))
			muted(c, t, i18n.T("从上往下第一条命中的规则决定走法，都没有命中就按最终走法。域名可写 geosite:cn、domain:x、full:x、keyword:x、regexp:x，地址可写 geoip:cn、网段或单个地址，端口可写 80,443 或 1000-2000。"))
			ui.Row(c).Gap(gapBase).AlignItems(ui.Center).Children(func() {
				muted(c, t, i18n.T("最终走法"))
				ui.Segmented(c, &a.routeFinalAt, i18n.T("代理"), i18n.T("直连")).Width(150)
				ui.Select(c, &a.routeStrategy, core.Strategies).Label(i18n.T("域名解析策略（只对 xray 生效）")).Width(240)
			})
			ui.Divider(c)
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(gapTight).Children(func() {
					if len(a.rules) == 0 {
						muted(c, t, i18n.T("还没有规则：所有流量都按最终走法"))
					}
					for i := range a.rules {
						at := i
						rule := a.rules[i]
						ui.Row(c).Gap(gapBase).AlignItems(ui.Center).Children(func() {
							ui.Text(c, ruleSummary(rule)).Font(fontMono).FontSize(fontS).Grow(1).TextColor(t.Text)
							if ui.Button(c, i18n.T("编辑")).Clicked() {
								a.editRule(at)
							}
							if ui.Button(c, "").Width(32).Children(func() {
								ui.Icon(c, iconUp).Size(iconSize, iconSize)
							}).Label(i18n.T("上移")).Disabled(at == 0).Clicked() {
								a.moveRule(at, -1)
							}
							if ui.Button(c, "").Width(32).Children(func() {
								ui.Icon(c, iconDown).Size(iconSize, iconSize)
							}).Label(i18n.T("下移")).Disabled(at == len(a.rules)-1).Clicked() {
								a.moveRule(at, 1)
							}
							if ui.Button(c, i18n.T("删除")).Clicked() {
								a.removeRule(at)
							}
						})
					}
					ui.Row(c).Gap(gapBase).Children(func() {
						if ui.Button(c, i18n.T("添加规则")).Clicked() {
							a.editRule(-1)
						}
					})
				})
			})
			ui.Row(c).Gap(gapBase).Justify(ui.End).Children(func() {
				if ui.Button(c, i18n.T("取消")).Clicked() {
					a.editingRoute = false
				}
				if ui.PrimaryButton(c, i18n.T("保存")).Clicked() {
					a.editingRoute = false
					a.saveRouting()
				}
			})
		})
	})
}

func (a *app) ruleForm(c *ui.Context) {
	if !a.editingRule {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.editingRule, func() {
		ui.Column(c).Gap(gapBase).Width(580).Height(520).Children(func() {
			heading(c, i18n.T("规则"))
			muted(c, t, i18n.T("填了哪几项就按哪几项匹配，任意一项命中即算命中。"))
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(gapBase).Children(func() {
					ui.Row(c).Gap(gapBase).Children(func() {
						ui.TextInput(c, &a.ruleDraft.Remark).Label(i18n.T("备注")).Grow(1)
						out := outName(a.ruleDraft.Out)
						if ui.Select(c, &out, outNames()).Label(i18n.T("出口")).Width(140).Changed() {
							a.ruleDraft.Out = outOf(out)
						}
					})
					ui.TextArea(c, &a.ruleDomains).Label(i18n.T("域名（geosite:cn、domain:x、full:x、keyword:x、regexp:x）")).Height(96)
					ui.TextArea(c, &a.ruleIPs).Label(i18n.T("地址（geoip:cn、10.0.0.0/8、1.1.1.1）")).Height(80)
					ui.Row(c).Gap(gapBase).Children(func() {
						ui.TextInput(c, &a.rulePort).Label(i18n.T("端口（80,443 或 1000-2000）")).Grow(1)
						ui.Select(c, &a.ruleNetwork, core.RuleNetworks).Label(i18n.T("网络")).Width(150)
					})
					ui.TextInput(c, &a.ruleProtocols).Label(i18n.T("协议（bittorrent、dns、http、tls、quic）")).Grow(1)
				})
			})
			ui.Row(c).Gap(gapBase).Justify(ui.End).Children(func() {
				if ui.Button(c, i18n.T("取消")).Clicked() {
					a.backToRules()
				}
				if ui.PrimaryButton(c, i18n.T("保存")).Clicked() {
					a.commitRule()
				}
			})
		})
	})
}

func (a *app) editRule(at int) {
	a.ruleAt = at
	a.ruleDraft = core.Rule{Out: core.Outs[0]}
	if at >= 0 && at < len(a.rules) {
		a.ruleDraft = a.rules[at]
	}
	a.ruleDomains = core.List(a.ruleDraft.Domain)
	a.ruleIPs = core.List(a.ruleDraft.IP)
	a.rulePort = a.ruleDraft.Port
	a.ruleNetwork = a.ruleDraft.Network
	a.ruleProtocols = strings.Join(a.ruleDraft.Protocol, ",")
	a.editingRoute = false
	a.editingRule = true
}

func (a *app) commitRule() {
	rule := a.ruleDraft
	rule.Domain = core.ParseList(a.ruleDomains)
	rule.IP = core.ParseList(a.ruleIPs)
	rule.Port = strings.TrimSpace(a.rulePort)
	rule.Network = a.ruleNetwork
	rule.Protocol = core.ParseList(a.ruleProtocols)
	if rule.Out == "" {
		rule.Out = core.Outs[0]
	}
	if !validPorts(rule.Port) {
		a.fail(i18n.T("端口只能写数字或范围（如 80,443 或 1000-2000），且都在 1 到 65535 之间"))
		return
	}
	if rule.Bare() {
		a.fail(i18n.T("这条规则没有匹配条件，没有保存"))
		a.backToRules()
		return
	}
	if a.ruleAt >= 0 && a.ruleAt < len(a.rules) {
		a.rules[a.ruleAt] = rule
	} else {
		a.rules = append(a.rules, rule)
	}
	a.backToRules()
}

func portNumber(text string) (int, bool) {
	number, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || number < 1 || number > 65535 {
		return 0, false
	}
	return number, true
}

func validPorts(text string) bool {
	if strings.TrimSpace(text) == "" {
		return true
	}
	for _, part := range strings.Split(text, ",") {
		low, high, ranged := strings.Cut(strings.TrimSpace(part), "-")
		lowAt, ok := portNumber(low)
		if !ok {
			return false
		}
		if !ranged {
			continue
		}
		highAt, ok := portNumber(high)
		if !ok || lowAt > highAt {
			return false
		}
	}
	return true
}

func (a *app) dnsPresets() []string {
	names := make([]string, 0, len(core.DNSPresets)+1)
	for _, one := range core.DNSPresets {
		names = append(names, i18n.T(one.Name))
	}
	return append(names, i18n.T("自定义"))
}

func (a *app) applyDNSPreset(name string) {
	for _, one := range core.DNSPresets {
		if i18n.T(one.Name) != name {
			continue
		}
		a.dnsRemote, a.dnsDirect, a.dnsBootstrap = one.Remote, one.Direct, one.Bootstrap
		return
	}
}

func (a *app) backToRules() {
	a.editingRule = false
	a.editingRoute = true
}

func (a *app) moveRule(at, by int) {
	to := at + by
	if at < 0 || at >= len(a.rules) || to < 0 || to >= len(a.rules) {
		return
	}
	a.rules[at], a.rules[to] = a.rules[to], a.rules[at]
}

func (a *app) removeRule(at int) {
	if at < 0 || at >= len(a.rules) {
		return
	}
	a.rules = append(a.rules[:at], a.rules[at+1:]...)
}

func ruleSummary(rule core.Rule) string {
	remark := rule.Remark
	if remark == "" {
		remark = i18n.T("未命名")
	}
	var matched []string
	if len(rule.Domain) > 0 {
		matched = append(matched, strings.Join(rule.Domain, ","))
	}
	if len(rule.IP) > 0 {
		matched = append(matched, strings.Join(rule.IP, ","))
	}
	if rule.Port != "" {
		matched = append(matched, ":"+rule.Port)
	}
	if rule.Network != "" {
		matched = append(matched, rule.Network)
	}
	if len(rule.Protocol) > 0 {
		matched = append(matched, strings.Join(rule.Protocol, ","))
	}
	return fmt.Sprintf("%s → %s · %s", remark, outName(rule.Out), strings.Join(matched, " "))
}

var outLabels = map[string]string{"proxy": "代理", "direct": "直连", "block": "拦截"}

func outNames() []string {
	names := make([]string, len(core.Outs))
	for i, out := range core.Outs {
		names[i] = outName(out)
	}
	return names
}

func outName(out string) string { return i18n.T(outLabels[out]) }

func outOf(name string) string {
	for _, out := range core.Outs {
		if outName(out) == name {
			return out
		}
	}
	return core.Outs[0]
}
