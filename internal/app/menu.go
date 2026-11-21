package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tungo/internal/core"
	"tungo/internal/i18n"
	"tungo/internal/sub"
	"tungo/internal/sysproxy"
)

func (a *app) menuBar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(gapTight).AlignItems(ui.Center).PaddingX(padX).PaddingY(padYDense).Background(t.Background).Children(func() {
		ui.MenuButton(c, i18n.T("服务器"), func(m *ui.Menu) { a.serversMenu(c, m) })
		ui.MenuButton(c, i18n.T("订阅"), func(m *ui.Menu) { a.subscribeMenu(c, m) })
		ui.MenuButton(c, i18n.T("路由规则"), func(m *ui.Menu) { a.routingMenu(c, m) })
		ui.MenuButton(c, i18n.T("设置"), func(m *ui.Menu) { a.settingsMenu(c, m) })
		ui.MenuButton(c, i18n.T("帮助"), func(m *ui.Menu) { a.helpMenu(m) })
		ui.Box(c).Grow(1)
		ui.Row(c).Gap(gapLoose).AlignItems(ui.Center).Children(func() {
			current := routeLabel(a.routes.Current)
			if ui.Select(c, &current, a.routingNames()).Width(controlWide).Changed() {
				for _, item := range a.routes.Items {
					if routeLabel(item.Name) == current {
						a.selectRouting(item.Name)
						break
					}
				}
			}
			names := proxyNames()
			want := proxyName(sysproxy.Mode(a.proxy))
			if ui.Select(c, &want, names).Width(controlWider).Disabled(a.tun && a.connected()).Changed() {
				if at := slices.Index(names, want); at >= 0 {
					a.setProxyMode(string(sysproxy.Modes[at]))
				}
			}
			tun := ui.Toggle(c, &a.tun, "TUN").Tooltip(i18n.T("TUN 接管全部流量"))
			if !admin() {
				tun = tun.Tooltip(i18n.T("TUN 要管理员权限：从「设置」里重启为管理员")).Disabled(true)
			}
			if tun.Changed() {
				a.setTun(a.tun)
			}
		})

		ui.MenuButton(c, a.kernelLabel(), func(m *ui.Menu) { a.kernelMenu(c, m) }).
			Disabled(a.connected() || a.updating)
		if a.installNeeded() {
			if ui.Button(c, i18n.T("检查内核")).Clicked() {
				a.diagnose()
			}
		} else if a.latest != "" {
			if ui.Button(c, fmt.Sprintf(i18n.T("更新 %s"), strings.TrimPrefix(a.latest, "v"))).Clicked() {
				a.upgrade()
			}
			if ui.Button(c, i18n.T("忽略")).Clicked() {
				a.ignore()
			}
		}
	})
}

func (a *app) serversMenu(c *ui.Context, m *ui.Menu) {
	if m.Item(i18n.T("从剪贴板导入")).Chosen() {
		a.importClipboard(c)
	}
	m.Submenu(i18n.T("添加服务器"), func(p *ui.Menu) {
		for _, protocol := range core.Protocols {
			if protocol == "direct" {
				continue
			}
			label := protocol

			if kind := core.Kind(a.kind); kind != core.Auto && !kind.Supports(protocol) {
				label = fmt.Sprintf(i18n.T("%s（%s 不支持）"), protocol, kind)
			}
			if p.Item(label).Chosen() {
				a.addProtocol(protocol)
			}
		}
	})
	m.Separator()
	empty := len(a.nodes) == 0
	direct := a.node().Direct()

	if m.Item(i18n.T("编辑当前节点")).Disabled(empty || direct).Chosen() {
		a.after(func() { a.editAny(a.chosen) })
	}
	if m.Item(i18n.T("删除当前节点")).Disabled(empty || direct).Chosen() {
		a.after(func() { a.remove(a.chosen) })
	}
	if m.Item(i18n.T("复制分享链接")).Disabled(empty || direct || a.node().Group()).Chosen() {
		a.share(c)
	}
	m.Separator()
	if m.Item(i18n.T("上移")).Disabled(empty || direct || a.chosen == 0).Chosen() {
		a.after(func() { a.move(a.chosen, -1) })
	}
	if m.Item(i18n.T("下移")).Disabled(empty || direct || a.chosen >= len(a.nodes)-1).Chosen() {
		a.after(func() { a.move(a.chosen, 1) })
	}
	m.Submenu(i18n.T("测速"), func(p *ui.Menu) {

		if p.Item(i18n.T("TCP 延迟（全部）")).Chosen() {
			a.probe()
		}
		if p.Item(i18n.T("TCP 延迟（当前节点）")).Disabled(empty).Chosen() {
			a.after(func() { a.pingNode(a.chosen) })
		}
	})
	if m.Item(i18n.T("检测当前节点解锁")).Disabled(empty || direct || !a.connected()).Chosen() {
		a.checkUnlock()
	}
	m.Separator()
	if m.Item(i18n.T("添加策略组")).Chosen() {
		a.after(a.addGroup)
	}
	m.Separator()
	if m.Item(i18n.T("重新加载节点")).Chosen() {
		a.after(a.reload)
	}
	if m.Item(i18n.T("保存到文件")).Chosen() {
		a.save()
		a.logf("%s", i18n.T("已保存节点"))
	}
	if m.Item(i18n.T("打开节点文件")).Chosen() {
		a.reveal(a.nodesAt)
	}
}

func (a *app) nodeMenu(m *ui.Menu, at int) {
	if at < 0 || at >= len(a.nodes) {
		return
	}
	node := a.nodes[at]
	direct := node.Direct()
	if m.Item(i18n.T("使用这个节点")).Disabled(at == a.chosen || direct).Chosen() {
		a.after(func() {
			a.choose(at)
			if !a.connected() {
				a.connect()
			}
		})
	}
	m.Separator()
	if m.Item(i18n.T("编辑")).Chosen() {
		a.after(func() { a.editAny(at) })
	}
	if m.Item(i18n.T("删除")).Disabled(direct).Chosen() {
		a.after(func() { a.remove(at) })
	}
	if m.Item(i18n.T("复制分享链接")).Disabled(direct || node.Group()).Chosen() {
		a.after(func() { a.shareNode(node) })
	}
	m.Separator()
	if m.Item(i18n.T("上移")).Disabled(at == 0).Chosen() {
		a.after(func() { a.move(at, -1) })
	}
	if m.Item(i18n.T("下移")).Disabled(at == len(a.nodes)-1).Chosen() {
		a.after(func() { a.move(at, 1) })
	}
	m.Separator()
	if m.Item(i18n.T("真连接延迟")).Disabled(direct).Chosen() {
		a.after(func() { a.realPing(at) })
	}
	if m.Item(i18n.T("测速")).Disabled(direct).Chosen() {
		a.after(func() { a.speedTest(at) })
	}
}

func (a *app) subscribeMenu(_ *ui.Context, m *ui.Menu) {
	running := a.subs.Running()
	if m.Item(i18n.T("更新订阅")).Disabled(running == nil || a.subBusy).Chosen() {
		a.refresh(running.Name)
	}
	if m.Item(i18n.T("更新订阅（通过代理）")).Disabled(running == nil || a.subBusy || a.local == nil).Chosen() {
		a.refreshProxied(running.Name)

	}
	if m.Item(i18n.T("更新全部订阅")).Disabled(len(a.subs.Items) == 0 || a.subBusy).Chosen() {
		a.refreshAll()
	}
	m.Separator()
	m.Submenu(i18n.T("订阅分组"), func(p *ui.Menu) {
		for _, item := range a.subs.Items {
			name := item.Name
			if p.Item(i18n.T(name)).Radio(running != nil && running.Name == name).Chosen() {
				a.selectSub(name)
			}
		}
	})
	if m.Item(i18n.T("订阅设置")).Chosen() {
		a.editingSub = true
	}
}

func (a *app) routingMenu(c *ui.Context, m *ui.Menu) {
	if m.Item(i18n.T("编辑当前规则")).Chosen() {
		a.openRouting()
	}
	if m.Item(i18n.T("新建配置")).Chosen() {
		a.routeName = ""
		a.editingRouteName = true
	}
	if m.Item(i18n.T("删除当前配置")).Disabled(len(a.routes.Items) < 2).Chosen() {
		a.removeRouting()
	}
	m.Separator()
	if m.Item(i18n.T("DNS 设置")).Chosen() {
		a.openDNS()
	}
	m.Separator()
	m.Submenu(i18n.T("区域预置"), func(p *ui.Menu) { a.regionMenu(c, p) })
	m.Separator()
	m.Submenu(i18n.T("规则集"), func(p *ui.Menu) {
		if p.Item(i18n.T("镜像源设置")).Chosen() {
			a.openRuleBase()
		}
		if p.Item(i18n.T("更新全部规则集")).Chosen() {
			a.refreshRuleSets()
		}
	})
	if m.Item(i18n.T("忽略缺失的规则集并连接")).Chosen() {
		a.ignoreRules = true
		a.connect()
	}
}

func (a *app) settingsMenu(c *ui.Context, m *ui.Menu) {
	m.Submenu(i18n.T("内核"), func(p *ui.Menu) { a.kernelMenu(c, p) })
	m.Submenu(i18n.T("主题"), func(p *ui.Menu) {
		for _, name := range themes {
			if p.Item(themeLabel(name)).Radio(a.themeName == name).Chosen() {
				a.setTheme(name)
			}
		}
	})
	m.Submenu(i18n.T("语言"), func(p *ui.Menu) {
		if p.Item("中文").Radio(i18n.Lang() != "en").Chosen() {
			a.setLang("zh")
		}
		if p.Item("English").Radio(i18n.Lang() == "en").Chosen() {
			a.setLang("en")
		}
	})
	m.Separator()
	m.Submenu(i18n.T("系统代理"), func(p *ui.Menu) {
		for _, mode := range sysproxy.Modes {
			if p.Item(proxyName(mode)).Radio(a.proxy == string(mode)).Chosen() {
				a.setProxyMode(string(mode))
			}
		}
	})
	if sysproxy.LoopbackAvailable() {
		if m.Item(i18n.T("放行 Store 应用回环（UWP）")).Chosen() {
			a.exemptLoopback()
		}
	}
	if m.Item(i18n.T("开机自启")).Checked(mygo.App.OpenAtLogin()).Chosen() {
		if err := mygo.App.SetOpenAtLogin(!mygo.App.OpenAtLogin()); err != nil {
			a.fail(err.Error())
			a.logf("%s", err)
		}
	}
	m.Submenu(i18n.T("全局热键"), func(p *ui.Menu) {
		if p.Item(i18n.T("全局热键设置")).Chosen() {
			a.openShortcuts()
		}
	})
	if m.Item(fmt.Sprintf(i18n.T("本地端口（%s）"), a.portLabel())).Chosen() {
		a.openPort()
	}
	m.Separator()
	m.Submenu(i18n.T("备份与还原"), func(p *ui.Menu) { a.backupMenu(c, p) })
	if m.Item(i18n.T("打开数据目录")).Chosen() {
		a.reveal(mustDir())
	}
	if m.Item(i18n.T("打开日志目录")).Chosen() {
		a.openLogs()
	}
	m.Separator()
	if m.Item(i18n.T("清除流量统计")).Chosen() {
		a.upBase, a.downBase = a.upRaw, a.downRaw
		a.logf("%s", i18n.T("已清除流量统计"))
	}
	if !admin() {
		if m.Item(i18n.T("重启为管理员")).Chosen() {
			a.rebootAsAdmin()
		}
	}
	m.Separator()
	if m.Item(i18n.T("退出 TunGo")).Chosen() {
		a.requestQuit()
	}
}

func (a *app) kernelMenu(_ *ui.Context, m *ui.Menu) {
	for _, kind := range core.Choices {
		name := string(kind)
		label := name
		if kind == core.Auto {
			label = i18n.T("自动（默认 sing-box）")
		}
		if m.Item(label).Radio(a.kind == name).Disabled(a.connected()).Chosen() {
			a.kind = name
			a.saveSettings()
			a.know(a.kernelKind())
			a.check()
		}
	}
}

func (a *app) helpMenu(m *ui.Menu) {
	if m.Item(i18n.T("检查应用更新")).Disabled(a.releases.polling || a.releases.installing).Chosen() {
		a.checkApp(false)
	}
	if update := a.releases.pending; update != nil {
		if m.Item(fmt.Sprintf(i18n.T("更新应用到 %s"), update.Version)).Disabled(a.releases.installing).Chosen() {
			a.installApp()
		}
	}
	if a.releases.installing {
		label := i18n.T("正在更新应用")
		if a.releases.whole > 0 {
			label += fmt.Sprintf(" %d%%", 100*a.releases.part/a.releases.whole)
		}
		m.Item(label).Disabled(true)
	}
	m.Separator()
	if m.Item(i18n.T("检查内核更新")).Chosen() {
		a.check()
	}
	if m.Item(i18n.T("关于 TunGo")).Chosen() {
		a.about()
	}
}

func (a *app) importClipboard(c *ui.Context) {
	text := c.ReadClipboard()
	if strings.TrimSpace(text) == "" {
		a.hint(i18n.T("剪贴板是空的"))
		return
	}
	nodes, err := sub.Parse([]byte(text))
	if err != nil {
		a.fail(err.Error())
		a.logf(i18n.T("导入失败：%s"), err)
		return
	}
	a.appendNodes(nodes...)
	a.save()
	a.say(fmt.Sprintf(i18n.T("已导入 %d 个节点"), len(nodes)))
}

var protocolDefaults = map[string]core.Node{
	"socks":       {Port: 1080},
	"http":        {Port: 1080},
	"shadowsocks": {Method: "aes-256-gcm"},
	"trojan":      {TLS: "tls"},
}

func (a *app) addProtocol(protocol string) {
	a.draft = protocolDefaults[protocol]
	a.draft.Protocol = protocol
	if a.draft.Port == 0 {
		a.draft.Port = 443
	}
	a.draftPort = fmt.Sprint(a.draft.Port)
	a.alterID = "0"
	a.muxText = ""
	a.alpn = ""
	a.draftWas = core.Node{}
	a.detourName = ""
	a.rawText = ""
	a.editing = true
}

func (a *app) edit(i int) {
	if i < 0 || i >= len(a.nodes) {
		return
	}
	a.draft = a.nodes[i]
	a.draftPort = fmt.Sprint(a.draft.Port)
	a.alterID = fmt.Sprint(a.draft.AlterID)
	a.muxText = ""
	if a.draft.Mux > 0 {
		a.muxText = fmt.Sprint(a.draft.Mux)
	}
	a.alpn = strings.Join(a.draft.ALPN, ",")
	a.draftWas = a.nodes[i]
	a.detourName = ""
	if a.draft.Detour != nil {
		a.detourName = a.draft.Detour.Name
	}
	a.rawText = a.draft.Raw
	a.editing = true
}

func (a *app) remove(i int) {
	if i < 0 || i >= len(a.nodes) || a.nodes[i].Direct() {
		return
	}
	name := a.nodes[i].Name
	a.dropNode(i)
	a.save()
	a.logf(i18n.T("已删除节点：%s"), name)
	if a.connected() {
		a.connect()
	}
}

func (a *app) move(i, by int) {
	to := i + by

	if i < 0 || i >= len(a.nodes) || to < 0 || to >= len(a.nodes) {
		return
	}
	a.swapNodes(i, to)
	a.chosen = to
	a.save()
}

func (a *app) save() {
	a.prune()
	if err := core.SaveNodes(a.nodes); err != nil {
		a.fail(err.Error())
		a.logf(i18n.T("保存节点失败：%s"), err)
	}
	a.refreshTray()
}

func (a *app) prune() {
	real := make(map[string]bool, len(a.nodes))
	known := make(map[string]bool, len(a.nodes))
	for _, node := range a.nodes {
		known[node.Name] = true
		if !node.Group() {
			real[node.Name] = true
		}
	}
	for i := range a.nodes {
		if detour := a.nodes[i].Detour; detour != nil && !known[detour.Name] {
			a.nodes[i].Detour = nil
		}
		if !a.nodes[i].Group() {
			continue
		}
		kept := make([]string, 0, len(a.nodes[i].Children))
		for _, name := range a.nodes[i].Children {
			if real[name] {
				kept = append(kept, name)
			}
		}
		a.nodes[i].Children = kept
	}
}

func (a *app) reload() {
	nodes, _, err := core.LoadNodes()
	if err != nil {
		a.fail(err.Error())
		a.logf(i18n.T("重新加载失败：%s"), err)
		return
	}
	if a.connected() {
		a.disconnect()
	}
	a.setNodes(nodes)
	a.chosen = 0
	a.logf(i18n.T("已重新加载 %d 个节点"), len(nodes))
}

func (a *app) share(c *ui.Context) {
	if link := a.linkOf(a.node()); link != "" {
		c.WriteClipboard(link)
	}
}

func (a *app) shareNode(node core.Node) {
	if link := a.linkOf(node); link != "" {
		a.writeNext = link
	}
}

func (a *app) linkOf(node core.Node) string {
	link, err := sub.URI(node)
	if err != nil {
		a.fail(err.Error())
		a.logf("%s", err)
		return ""
	}
	a.say(i18n.T("分享链接已复制"))
	a.logf(i18n.T("分享链接已复制：%s"), node.Name)
	return link
}
