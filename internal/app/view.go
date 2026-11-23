package app

import (
	"fmt"
	"math"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

func (a *app) view(c *ui.Context) {
	a.runQueued()
	t := themeFor(a.themeName, a.themeName == "system" && systemDark())
	c.SetTheme(t)

	c.SetMenuPresentation(ui.PopoverMenus)

	if a.writeNext != "" {
		c.WriteClipboard(a.writeNext)
		a.writeNext = ""
	}
	ui.Column(c).Fill().Children(func() {
		a.titleBar(c, t)
		a.menuBar(c, t)
		a.toolBar(c, t)
		ui.SplitVerticalPercent(c, &a.panePct, func() {
			a.table(c, t)
		}, func() {
			a.panes(c, t)
		}).Grow(1)
		ui.Divider(c)
		a.statusBar(c, t)
	})
	a.overlays(c)
}

func (a *app) table(c *ui.Context, t *ui.Theme) {
	a.syncRows()
	if len(a.shown) == 0 {
		a.emptyPane(c, t, i18n.T("还没有节点：从「服务器」导入分享链接，或设置订阅"))
		return
	}
	if a.row < 0 || a.row >= len(a.shown) {
		a.row, a.rowAt = 0, 0
	}
	if a.rowAt < 0 || a.row == a.rowAt && a.shown[a.row] != a.chosen {
		if at := a.rowOf(a.chosen); at >= 0 {
			a.row = at
		}
		a.rowAt = a.row
	}
	ui.Themed(c, listTheme(t)).Grow(1).Padding(0, scrollGutter, 0, 0).Children(func() {
		ui.Table(c, &a.list, []ui.TableColumn{
			{ID: "name", Title: i18n.T("备注"), MinWidth: 150, Sortable: true},
			{ID: "addr", Title: i18n.T("地址"), Width: 150},
			{ID: "proto", Title: i18n.T("协议"), Width: 96},
			{ID: "net", Title: i18n.T("传输"), Width: 84},
			{ID: "cipher", Title: i18n.T("加密"), Width: 130},
			{ID: "tls", Title: i18n.T("安全"), Width: 104},
			{ID: "group", Title: i18n.T("分组"), Width: 88},
			{ID: "delay", Title: i18n.T("延迟"), Width: 72, Sortable: true},
			{ID: "speed", Title: i18n.T("速度"), Width: 84, Sortable: true},
		}, len(a.shown), func(row, col int) {

			if row < 0 || row >= len(a.shown) {
				return
			}
			at := a.shown[row]
			node := a.nodes[at]
			cell := ui.Box(c).FillWidth().Children(func() {
				if node.Group() {
					switch col {
					case 0:
						a.remark(c, t, node.Name, at)
					case 1:
						cellCode(c, t, strings.Join(node.Children, ", "))
					case 2:
						cellSoft(c, t, i18n.T("策略组"))
					case 4:
						cellSoft(c, t, "—")
					case 5:
						cellSoft(c, t, strategyName(node.StrategyOr()))
					case 6:
						cellSoft(c, t, node.Sub)
					case 7:
						a.delay(c, t, at)
					case 8:
						cellCode(c, t, a.speedLabel(at))
					}
					return
				}
				switch col {
				case 0:
					a.remark(c, t, node.Name, at)
				case 1:
					where := node.Server
					if node.Port != 0 {
						where = net.JoinHostPort(node.Server, fmt.Sprint(node.Port))
					}
					cellCode(c, t, where)
				case 2:
					cellCode(c, t, node.Protocol)
				case 3:
					cellCode(c, t, transport(node))
				case 4:
					cellCode(c, t, cipher(node))
				case 5:
					cellSoft(c, t, security(node))
				case 6:
					cellSoft(c, t, node.Sub)
				case 7:
					a.delay(c, t, at)
				case 8:
					cellCode(c, t, a.speedLabel(at))
				}
			})
			cell.ContextMenu(func(m *ui.Menu) { a.nodeMenu(m, at) })
			if cell.RowDoubleClicked() {
				a.row, a.rowAt = row, row
				a.choose(at)
				if !a.connected() {
					a.connect()
				}
			}
		})
	})
	if a.row != a.rowAt && a.row >= 0 && a.row < len(a.shown) {
		a.rowAt = a.row
		a.choose(a.shown[a.row])
	}
}

func (a *app) emptyPane(c *ui.Context, t *ui.Theme, text string) {
	ui.Box(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Padding(padX).Children(func() {
		muted(c, t, text)
	})
}

func (a *app) remark(c *ui.Context, t *ui.Theme, name string, at int) {
	if at != a.chosen {
		ui.Text(c, name).SingleLine().FontSize(fontM)
		return
	}
	ui.Row(c).Gap(gapTight).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, iconDot).Size(markSize, markSize).TextColor(t.Success)
		ui.Text(c, name).SingleLine().FontSize(fontM)
	})
}

func cellCode(c *ui.Context, t *ui.Theme, text string) {
	ui.Text(c, text).SingleLine().Font(fontMono).FontSize(fontS).TextColor(t.TextMuted)
}

func cellSoft(c *ui.Context, t *ui.Theme, text string) {
	ui.Text(c, text).SingleLine().FontSize(fontS).TextColor(t.TextMuted)
}

func (a *app) delay(c *ui.Context, t *ui.Theme, at int) {
	if at < 0 || at >= len(a.ms) || a.ms[at] <= 0 {
		cellCode(c, t, "—")
		return
	}
	color := t.Text
	if a.ms[at] > 500 {
		color = t.Warning
	}
	ui.Text(c, a.latency(at)).SingleLine().Font(fontMono).FontSize(fontS).TextColor(color)
}

func (a *app) panes(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Gap(gapBase).AlignItems(ui.Center).PaddingX(padX).PaddingY(padYDense).Children(func() {
			ui.Tabs(c, &a.tab, i18n.T("日志"), i18n.T("连接"))
			ui.Box(c).Grow(1)
			if a.tab == 0 {
				if ui.Button(c, i18n.T("复制")).Clicked() {
					a.copyLogs(c)
				}
				if ui.Button(c, i18n.T("清空")).Clicked() {
					a.clearLogs()
				}
			} else {
				muted(c, t, fmt.Sprintf(i18n.T("%d 个连接"), len(a.conns)))
				if ui.Button(c, i18n.T("断开")).Disabled(len(a.conns) == 0 || a.statless()).Clicked() {
					a.dropConns()
				}
			}
		})
		ui.Box(c).Grow(1).Children(func() {
			if a.tab == 0 {
				a.log(c, t)
				return
			}
			a.sessions(c, t)
		})
	})
}

func (a *app) log(c *ui.Context, t *ui.Theme) {
	if len(a.logs) == 0 {
		a.emptyPane(c, t, i18n.T("还没有日志"))
		return
	}
	if a.logScroll.Y >= a.logScroll.MaxY {
		a.logScroll.Y = math.MaxFloat32
	}
	ui.Scroll(c).Grow(1).Padding(0, scrollGutter, 0, 0).TrackScroll(&a.logScroll).Children(func() {
		ui.Column(c).Gap(gapTight).Padding(padX, 0, padX, padX).Children(func() {
			for _, line := range a.logs {
				code(c, t, line)
			}
		})
	})
}

func (a *app) sessions(c *ui.Context, t *ui.Theme) {
	if len(a.conns) == 0 {
		a.emptyPane(c, t, a.sessionsNote())
		return
	}
	ui.Themed(c, listTheme(t)).Grow(1).Padding(0, scrollGutter, 0, 0).Children(func() {
		ui.Table(c, &a.connList, []ui.TableColumn{
			{Title: i18n.T("主机"), MinWidth: 140},
			{Title: i18n.T("规则"), Width: 110},
			{Title: i18n.T("网络"), Width: 60},
			{Title: i18n.T("类型"), Width: 76},
			{Title: i18n.T("时间"), Width: 72},
		}, len(a.conns), func(row, col int) {
			if row < 0 || row >= len(a.conns) {
				return
			}
			one := a.conns[row]
			switch col {
			case 0:
				cellCode(c, t, one.Host)
			case 1:
				cellSoft(c, t, one.Chain)
			case 2:
				cellCode(c, t, one.Network)
			case 3:
				cellCode(c, t, one.Type)
			case 4:
				cellCode(c, t, elapsed(one.Started))
			}
		})
	})
}

func (a *app) dropConns() {
	if a.kernel == nil {
		return
	}
	if err := a.kernel.CloseConnections(); err != nil {
		a.fail(err.Error())
		a.logf("%s", err)
		return
	}
	a.conns = nil
	a.logf("%s", i18n.T("已断开全部连接"))
}

func elapsed(started time.Time) string {
	if started.IsZero() {
		return "—"
	}
	d := time.Since(started)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

func (a *app) sessionsNote() string {
	if !a.connected() {
		return i18n.T("连接后这里显示内核正在转发的连接")
	}
	if a.kernel == nil || a.kernel.Kind() != core.SingBox {
		return i18n.T("xray 不提供会话列表")
	}
	return i18n.T("没有正在转发的连接")
}

func (a *app) copyLogs(c *ui.Context) {
	if len(a.logs) == 0 {
		return
	}
	c.WriteClipboard(strings.Join(a.logs, "\n"))
	a.say(i18n.T("日志已复制"))
}

func (a *app) clearLogs() {
	a.logs = nil
	a.say(i18n.T("面板日志已清空"))
}

func (a *app) toolBar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(gapLoose).AlignItems(ui.Center).PaddingX(padX).PaddingY(padY).Background(t.Background).Children(func() {
		if a.busy {
			if ui.Button(c, i18n.T("取消")).Width(buttonBase).Clicked() {
				a.disconnect()
			}
		} else if a.connected() {
			if ui.Button(c, i18n.T("停止")).Width(buttonBase).Clicked() {
				a.disconnect()
			}
		} else if ui.PrimaryButton(c, i18n.T("启动")).Width(buttonBase).Clicked() {
			a.connect()
		}
		groups := a.groupNames()
		at := slices.Index(groups, a.group)
		if at < 0 {
			at = 0
			a.group = ""
		}
		if ui.Segmented(c, &at, groups...).Changed() {
			a.group = ""
			if at > 0 {
				a.group = groups[at]
			}
			a.rowAt = -1
			a.rowsDirty = true
		}
		ui.Box(c).Grow(1)
		if ui.SearchField(c, &a.search).Label(i18n.T("搜索节点")).Width(200).Changed() {
			a.rowAt = -1
			a.rowsDirty = true
		}
	})
}

func (a *app) inbound() string {
	if a.local != nil {
		return a.local.Addr()
	}
	if a.kernel != nil {
		return i18n.T("TUN")
	}
	return "—"
}

func (a *app) routingNames() []string {
	names := make([]string, len(a.routes.Items))
	for i, item := range a.routes.Items {
		names[i] = routeLabel(item.Name)
	}
	return names
}

func routeLabel(name string) string {
	if region, ok := strings.CutPrefix(name, "区域："); ok {
		return i18n.T("区域：") + i18n.T(region)
	}
	return i18n.T(name)
}

func (a *app) status() string {
	if !a.connected() {
		return i18n.T("未连接")
	}
	return i18n.T("已连接") + " · " + a.node().Name
}

func (a *app) statusColor(t *ui.Theme) ui.Color {
	if a.busy {
		return t.Warning
	}
	if !a.connected() {
		return t.TextMuted
	}
	return t.Success
}

func (a *app) statusBar(c *ui.Context, t *ui.Theme) {
	color := a.statusColor(t)
	ui.Row(c).Gap(gapBase).AlignItems(ui.Center).Height(barHeight).PaddingX(padX).Background(t.Background).Children(func() {
		label := ui.Text(c, a.statusLabel()).SingleLine().FontSize(fontS).FontWeight(weightSemibold).TextColor(color)
		if tip := a.barTip(); tip != "" {
			label.Tooltip(tip)
		}
		if a.connected() {
			ui.Text(c, a.inbound()).SingleLine().Font(fontMono).FontSize(fontS).TextColor(t.TextMuted)
		}
		ui.Box(c).Grow(1)
		a.barTraffic(c, t)
		a.barChart(c, t)
	})
}

func (a *app) statusLabel() string {
	switch {
	case a.busy:
		return i18n.T("正在启动内核…")
	case !a.connected():
		return i18n.T("未连接")
	}
	return i18n.T("已连接")
}

func (a *app) barTip() string {
	parts := []string{}
	if a.connected() {
		parts = append(parts, a.node().Name)
	}
	if info := a.subInfo(); info != "" {
		parts = append(parts, info)
	}
	return strings.Join(parts, "\n")
}

func (a *app) subInfo() string {
	parts := []string{}
	if traffic := a.subTraffic(); traffic != "" {
		parts = append(parts, traffic)
	}
	if expire := a.subExpire(); expire != "" {
		parts = append(parts, expire)
	}
	return strings.Join(parts, " · ")
}

func (a *app) barTraffic(c *ui.Context, t *ui.Theme) {
	up, down := a.upRaw-a.upBase, a.downRaw-a.downBase
	tip := ""
	if a.statless() && a.local == nil {
		up, down = 0, 0
		tip = i18n.T("xray 不统计流量")
	}
	for _, one := range []struct {
		icon *ui.SVG
		n    int64
	}{{iconUp, up}, {iconDown, down}} {
		cell := ui.Row(c).Width(cellTraffic).Gap(gapTight).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, one.icon).Size(iconSize, iconSize).TextColor(t.TextMuted)
			text := "—"
			if one.n > 0 {
				text = size(one.n)
			}
			ui.Text(c, text).Grow(1).SingleLine().Font(fontMono).FontSize(fontS).TextColor(t.TextMuted)
		})
		if tip != "" {
			cell.Tooltip(tip)
		}
	}
}

func (a *app) barChart(c *ui.Context, t *ui.Theme) {
	values := a.hist
	if a.statless() && a.local == nil {
		values = nil
	}
	if len(values) > sparkBars {
		values = values[len(values)-sparkBars:]
	}
	peak := int64(0)
	for _, value := range values {
		if value > peak {
			peak = value
		}
	}
	pad := sparkBars - len(values)
	chart := ui.Row(c).Width(sparkWidth).Height(sparkHeight).Gap(sparkGap).AlignItems(ui.End)
	chart.Tooltip(fmt.Sprintf(i18n.T("最近 %d 秒的合计吞吐"), sparkBars))
	chart.Children(func() {
		for i := 0; i < sparkBars; i++ {
			height, color := float32(1), t.Border
			if at := i - pad; at >= 0 && values[at] > 0 {
				color = t.TextMuted
				if peak > 0 {
					height = max(1, sparkHeight*float32(values[at])/float32(peak))
				}
			}
			ui.Box(c).Width(sparkBar).Height(height).Radius(sparkBar / 2).Background(color)
		}
	})
}

func (a *app) statless() bool {
	return a.kernel != nil && a.kernel.Kind() != core.SingBox
}

func (a *app) subTraffic() string {
	item := a.subs.Item(a.node().Sub)
	if item == nil || item.Total <= 0 {
		return ""
	}
	return fmt.Sprintf(i18n.T("订阅 %s / %s"), size(item.Used), size(item.Total))
}

func (a *app) subExpire() string {
	item := a.subs.Item(a.node().Sub)
	if item == nil || item.Expire <= 0 {
		return ""
	}
	return fmt.Sprintf(i18n.T("到期 %s"), time.Unix(item.Expire, 0).Format("2006-01-02"))
}

func (a *app) latency(i int) string {
	if a.ms[i] < 0 {
		return "—"
	}
	return fmt.Sprintf("%d ms", a.ms[i])
}

func heading(c *ui.Context, text string) {
	ui.Text(c, text).Font(fontDisplay).FontSize(fontL).FontWeight(weightSemibold)
}

func muted(c *ui.Context, t *ui.Theme, text string) {
	ui.Text(c, text).FontSize(fontS).TextColor(t.TextMuted)
}

func code(c *ui.Context, t *ui.Theme, text string) {
	ui.Text(c, text).Font(fontMono).FontSize(fontS).TextColor(t.TextMuted)
}

func size(n int64) string {
	switch {
	case n < 1<<10:
		return fmt.Sprintf("%d B", n)
	case n < 1<<20:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	case n < 1<<30:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	default:
		return fmt.Sprintf("%.2f GB", float64(n)/(1<<30))
	}
}
