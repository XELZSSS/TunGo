package app

import (
	"fmt"
	"reflect"
	"sort"
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
	remark := i18n.T(rule.Remark)
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

func (a *app) dnsForm(c *ui.Context) {
	if !a.editingDNS {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.editingDNS, func() {
		ui.Column(c).Gap(gapLoose).Width(560).Height(520).Children(func() {
			heading(c, i18n.T("DNS 设置"))
			muted(c, t, i18n.T("三个都填 DoH：远程 DNS 走节点，直连域名用直连 DNS，引导 DNS 只用来解析上面两个的域名。全程加密，运营商看不到你查了什么。"))
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(gapLoose).Children(func() {
					preset := i18n.T(core.DNSPresetOf(core.DNS{Remote: a.dnsRemote, Direct: a.dnsDirect, Bootstrap: a.dnsBootstrap}))
					if preset == "" {
						preset = i18n.T("自定义")
					}
					if ui.Select(c, &preset, a.dnsPresets()).Label(i18n.T("服务商（选了就填好下面三个）")).Grow(1).Changed() {
						a.applyDNSPreset(preset)
					}
					ui.TextInput(c, &a.dnsRemote).Label(i18n.T("远程 DNS")).Placeholder("https://1.1.1.1/dns-query").Grow(1)
					ui.TextInput(c, &a.dnsDirect).Label(i18n.T("直连 DNS")).Placeholder("https://223.5.5.5/dns-query").Grow(1)
					ui.TextInput(c, &a.dnsBootstrap).Label(i18n.T("引导 DNS（用来解析上面两个服务器的域名）")).Placeholder("https://8.8.8.8/dns-query").Grow(1)
					ui.Row(c).Gap(12).Children(func() {
						ui.Select(c, &a.dnsStrategy, core.DNSStrategies).Label(i18n.T("解析结果")).Width(200)
						ui.Toggle(c, &a.dnsBlockAAAA, i18n.T("屏蔽 IPv6 查询（AAAA）"))
					})
					ui.TextArea(c, &a.dnsHosts).Label(i18n.T("Hosts（一行一条，写 名称 地址）")).Height(80)
					ui.TextArea(c, &a.dnsSplit).Label(i18n.T("按域分流（一行一条，写 域名 服务器）")).Height(80)
				})
			})
			ui.Row(c).Gap(gapBase).Justify(ui.End).Children(func() {
				if ui.Button(c, i18n.T("取消")).Clicked() {
					a.editingDNS = false
				}
				if ui.PrimaryButton(c, i18n.T("保存")).Clicked() {
					a.editingDNS = false
					a.saveDNS()
				}
			})
		})
	})
}

func (a *app) openRouting() {
	routing := a.routes.Routing()
	a.routeWas = routing
	a.rules = append([]core.Rule{}, routing.Rules...)
	a.routeStrategy = routing.Strategy
	if a.routeStrategy == "" {
		a.routeStrategy = core.Strategies[0]
	}
	a.routeFinalAt = 0
	if routing.FinalTag() == "direct" {
		a.routeFinalAt = 1
	}
	a.editingRoute = true
}

func (a *app) openDNS() {
	dns, err := core.LoadDNS()
	if err != nil {

		a.fail(err.Error())
		a.logf("%s", err)
		dns = core.DefaultDNS()
	}
	a.dnsWas = dns
	a.dnsRemote = dns.Remote
	a.dnsDirect = dns.Direct
	a.dnsBootstrap = dns.Bootstrap
	a.dnsStrategy = dns.Strategy
	if a.dnsStrategy == "" {
		a.dnsStrategy = core.DNSStrategies[0]
	}
	a.dnsBlockAAAA = dns.BlockAAAA
	a.dnsHosts = core.List(hostLines(dns.Hosts))
	a.dnsSplit = core.List(splitLines(dns.Split))
	a.editingDNS = true
}

func (a *app) saveRouting() {
	routing := core.Routing{
		Strategy: a.routeStrategy,
		Final:    core.RoutingFinals[a.routeFinalAt],
		Rules:    a.rules,
	}
	changed := !reflect.DeepEqual(a.routeWas, routing)
	a.routes.Put(a.routes.Current, routing)
	a.routeWas = routing
	a.logf(i18n.T("路由规则已保存（%s）：%d 条规则，最终%s"),
		routeLabel(a.routes.Current), len(routing.Rules), finalName(routing.FinalTag()))
	a.saveRoutes(changed)
}

func (a *app) saveRoutes(reconnect bool) {
	if err := core.SaveRoutingSet(a.routes); err != nil {
		a.fail(err.Error())
		a.logf(i18n.T("保存路由失败：%s"), err)
		return
	}
	if reconnect && a.connected() {
		a.connect()
	}
}

func (a *app) selectRouting(name string) {
	if name == a.routes.Current || a.routes.Item(name) == nil {
		return
	}
	a.routes.Current = name
	a.logf(i18n.T("路由配置：%s"), routeLabel(name))
	a.saveRoutes(true)
}

func (a *app) addRouting() {
	name := strings.TrimSpace(a.routeName)
	if name == "" {
		return
	}
	a.routes.Add(name)
	a.routeName = ""
	a.editingRouteName = false
	a.logf(i18n.T("新建路由配置：%s"), name)
	a.saveRoutes(false)
}

func (a *app) removeRouting() {
	if len(a.routes.Items) < 2 {
		return
	}
	name := a.routes.Current
	a.routes.Remove(name)
	a.logf(i18n.T("已删除路由配置：%s"), i18n.T(name))
	a.saveRoutes(true)
}

func (a *app) routeNameForm(c *ui.Context) {
	if !a.editingRouteName {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.editingRouteName, func() {
		ui.Column(c).Gap(gapLoose).Width(420).Children(func() {
			heading(c, i18n.T("新建路由配置"))
			muted(c, t, i18n.T("新配置先照搬当前这套规则，再到「路由规则」里改它。"))
			ui.TextInput(c, &a.routeName).Label(i18n.T("名称")).Placeholder(i18n.T("如：国内直连")).Grow(1)
			ui.Row(c).Gap(gapBase).Justify(ui.End).Children(func() {
				if ui.Button(c, i18n.T("取消")).Clicked() {
					a.editingRouteName = false
				}
				if ui.PrimaryButton(c, i18n.T("创建")).Clicked() {
					a.addRouting()
				}
			})
		})
	})
}

func (a *app) saveDNS() {
	if !validHostLines(a.dnsHosts) {
		a.fail(i18n.T("Hosts 每行要写「名称 地址」"))
		return
	}
	if !validSplitLines(a.dnsSplit) {
		a.fail(i18n.T("按域分流每行要写「域名 服务器」"))
		return
	}
	dns := core.DNS{
		Remote:    a.dnsRemote,
		Direct:    a.dnsDirect,
		Bootstrap: a.dnsBootstrap,
		Strategy:  a.dnsStrategy,
		BlockAAAA: a.dnsBlockAAAA,
		Hosts:     parseHosts(a.dnsHosts),
		Split:     parseSplit(a.dnsSplit),
	}
	changed := !reflect.DeepEqual(a.dnsWas, dns)
	if err := core.SaveDNS(dns); err != nil {
		a.fail(err.Error())
		a.logf(i18n.T("保存 DNS 失败：%s"), err)
		return
	}
	a.dnsWas = dns
	a.logf(i18n.T("DNS 已保存：远程 %s，直连 %s，%s，屏蔽 AAAA %s，%d 条 Hosts"),
		dns.Remote, dns.Direct, dns.Strategy, onOff(dns.BlockAAAA), len(dns.Hosts))
	if changed && a.connected() {
		a.connect()
	}
}

func validHostLines(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if len(fields) != 2 {
			return false
		}
	}
	return true
}

func validSplitLines(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if len(fields) < 2 {
			return false
		}
	}
	return true
}

func onOff(on bool) string {
	if on {
		return i18n.T("开")
	}
	return i18n.T("关")
}

func finalName(tag string) string {
	if tag == "direct" {
		return i18n.T("直连")
	}
	return i18n.T("代理")
}

func parseHosts(text string) map[string]string {
	hosts := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		hosts[fields[0]] = fields[1]
	}
	if len(hosts) == 0 {
		return nil
	}
	return hosts
}

func hostLines(hosts map[string]string) []string {
	names := make([]string, 0, len(hosts))
	for name := range hosts {
		names = append(names, name+" "+hosts[name])
	}
	sort.Strings(names)
	return names
}

func parseSplit(text string) []core.SplitDNS {
	var out []core.SplitDNS
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		out = append(out, core.SplitDNS{
			Domains: core.ParseList(strings.Join(fields[:len(fields)-1], " ")),
			Server:  fields[len(fields)-1],
		})
	}
	return out
}

func splitLines(splits []core.SplitDNS) []string {
	out := make([]string, 0, len(splits))
	for _, one := range splits {
		out = append(out, strings.Join(one.Domains, ",")+" "+one.Server)
	}
	return out
}
