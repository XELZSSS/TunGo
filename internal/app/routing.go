package app

import (
	"reflect"
	"strings"

	"github.com/egoist/mygo/ui"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

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
	dns := core.LoadDNS()
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
	a.refreshTray()
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
