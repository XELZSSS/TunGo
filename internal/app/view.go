package app

import (
	"fmt"
	"math"
	"net"
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
	if a.rowAt < 0 {
		if at := a.rowOf(a.chosen); at >= 0 {
			a.row = at
		}
		a.rowAt = a.row
	}
	if a.row < 0 {
		a.row = 0
	}
	if a.row >= len(a.shown) {
		a.row = len(a.shown) - 1
	}
	ui.Themed(c, listTheme(t)).Grow(1).Children(func() {
		ui.Table(c, &a.list, []ui.TableColumn{
			{ID: "name", Title: i18n.T("备注"), MinWidth: 140, Sortable: true},
			{ID: "addr", Title: i18n.T("地址"), Width: 170},
			{ID: "proto", Title: i18n.T("协议"), Width: 100},
			{ID: "net", Title: i18n.T("传输"), Width: 96},
			{ID: "cipher", Title: i18n.T("加密"), Width: 160},
			{ID: "tls", Title: i18n.T("安全"), Width: 124},
			{ID: "group", Title: i18n.T("分组"), Width: 96},
			{ID: "delay", Title: i18n.T("延迟"), Width: 76, Align: ui.End, Sortable: true},
			{ID: "speed", Title: i18n.T("速度"), Width: 72, Align: ui.End, Sortable: true},
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
						code(c, t, strings.Join(node.Children, "、"))
					case 2:
						muted(c, t, i18n.T("策略组"))
					case 4:
						muted(c, t, "—")
					case 5:
						muted(c, t, strategyName(node.StrategyOr()))
					case 6:
						muted(c, t, groupName(node))
					case 7:
						a.delay(c, t, at)
					case 8:
						code(c, t, a.speedLabel(at))
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
					code(c, t, where)
				case 2:
					code(c, t, node.Protocol)
				case 3:
					code(c, t, transport(node))
				case 4:
					code(c, t, cipher(node))
				case 5:
					muted(c, t, security(node))
				case 6:
					muted(c, t, groupName(node))
				case 7:
					a.delay(c, t, at)
				case 8:
					code(c, t, a.speedLabel(at))
				}
			})
			cell.ContextMenu(func(m *ui.Menu) { a.nodeMenu(m, at) })
			if cell.RowDoubleClicked() {
				a.rowAt = row
				a.choose(at)
				if !a.connected() {
					a.connect()
				}
			}
		})
	})
	if a.row != a.rowAt && a.row >= 0 && a.row < len(a.shown) {
		a.rowAt = a.row
		a.chosen = a.shown[a.row]
		a.know(a.kernelKind())
	}
}

func (a *app) emptyPane(c *ui.Context, t *ui.Theme, text string) {
	ui.Box(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Padding(padX).Children(func() {
		muted(c, t, text)
	})
}

func (a *app) remark(c *ui.Context, t *ui.Theme, name string, at int) {
	if at != a.chosen {
		ui.Text(c, name).FontSize(fontM)
		return
	}
	ui.Row(c).Gap(gapTight).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, iconDot).Size(markSize, markSize).TextColor(t.Success)
		ui.Text(c, name).FontSize(fontM)
	})
}

func (a *app) delay(c *ui.Context, t *ui.Theme, at int) {
	if at < 0 || at >= len(a.ms) || a.ms[at] <= 0 {
		code(c, t, "—")
		return
	}
	color := t.Text
	if a.ms[at] > 500 {
		color = t.Warning
	}
	ui.Text(c, a.latency(at)).Font(fontMono).FontSize(fontS).TextColor(color)
}

func traffic(c *ui.Context, t *ui.Theme, up, down int64) {
	if up == 0 && down == 0 {
		code(c, t, "—")
		return
	}
	for _, one := range []struct {
		icon *ui.SVG
		n    int64
	}{{iconUp, up}, {iconDown, down}} {
		ui.Row(c).Gap(gapTight).AlignItems(ui.Center).TextColor(t.TextMuted).Children(func() {
			ui.Icon(c, one.icon).Size(iconSize, iconSize)
			ui.Text(c, size(one.n)).Font(fontMono).FontSize(fontS)
		})
	}
}

func (a *app) panes(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).AlignItems(ui.Center).PaddingX(padX).PaddingY(padYDense).Children(func() {
			ui.Tabs(c, &a.tab, i18n.T("日志"), i18n.T("连接"))
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
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Gap(gapBase).AlignItems(ui.Center).PaddingX(padX).PaddingY(padYDense).Children(func() {
			if ui.Button(c, i18n.T("复制")).Clicked() {
				a.copyLogs()
			}
			if ui.Button(c, i18n.T("清空")).Clicked() {
				a.clearLogs()
			}
			ui.Box(c).Grow(1)
		})
		if len(a.logs) == 0 {
			a.emptyPane(c, t, i18n.T("还没有日志"))
			return
		}

		if a.logScroll.Y >= a.logScroll.MaxY {
			a.logScroll.Y = math.MaxFloat32
		}
		ui.Scroll(c).Grow(1).TrackScroll(&a.logScroll).Children(func() {
			ui.Column(c).Gap(gapTight).Padding(padX).Children(func() {
				for _, line := range a.logs {
					code(c, t, line)
				}
			})
		})
	})
}

func (a *app) sessions(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Gap(gapBase).AlignItems(ui.Center).PaddingX(padX).PaddingY(padYDense).Children(func() {
			muted(c, t, fmt.Sprintf(i18n.T("%d 个连接"), len(a.conns)))
			if ui.Button(c, i18n.T("断开")).Disabled(len(a.conns) == 0).Clicked() {
				a.dropConns()
			}
			ui.Box(c).Grow(1)
		})
		if len(a.conns) == 0 {
			a.emptyPane(c, t, a.sessionsNote())
			return
		}
		ui.Themed(c, listTheme(t)).Grow(1).Children(func() {
			ui.Table(c, &a.connList, []ui.TableColumn{
				{Title: i18n.T("主机"), MinWidth: 140},
				{Title: i18n.T("规则"), Width: 110},
				{Title: i18n.T("网络"), Width: 60},
				{Title: i18n.T("类型"), Width: 76},
				{Title: i18n.T("时间"), Width: 72, Align: ui.End},
			}, len(a.conns), func(row, col int) {

				if row < 0 || row >= len(a.conns) {
					return
				}
				one := a.conns[row]
				switch col {
				case 0:
					code(c, t, one.Host)
				case 1:
					muted(c, t, one.Chain)
				case 2:
					code(c, t, one.Network)
				case 3:
					code(c, t, one.Type)
				case 4:
					code(c, t, elapsed(one.Started))
				}
			})
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

func (a *app) copyLogs() {
	if len(a.logs) == 0 {
		return
	}
	a.writeNext = strings.Join(a.logs, "\n")
	a.say(i18n.T("日志已复制"))
	a.update(func() {})
}

func (a *app) clearLogs() {
	a.logs = nil
	a.say(i18n.T("日志已清空"))
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
		at := indexOf(groups, a.group)
		if ui.Segmented(c, &at, groups...).Changed() {
			a.group = ""
			if at > 0 {
				a.group = groups[at]
			}
			a.rowAt = -1
		}
	})
}

func (a *app) titleNode() string {
	if !a.connected() {
		return ""
	}
	return ellipsis(a.node().Name, 40)
}

func ellipsis(text string, n int) string {
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return string(runes[:n-1]) + "…"
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
		names[i] = i18n.T(item.Name)
	}
	return names
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
	ui.Row(c).Gap(gapBase).AlignItems(ui.Center).PaddingX(padX).PaddingY(padY).Background(t.Background).Children(func() {
		ui.Text(c, a.statusText()).FontSize(fontS).FontWeight(weightSemibold).TextColor(a.statusColor(t))
		code(c, t, a.inbound())
		ui.Box(c).Grow(1)
		if a.notice != "" {
			ui.Text(c, a.notice).FontSize(fontS).TextColor(a.noticeColor(t))
		} else if a.subNote != "" {
			muted(c, t, a.subNote)
		}
		traffic(c, t, a.up, a.down)
	})
}

func (a *app) statusText() string {
	if a.busy {
		return i18n.T("正在启动内核…")
	}
	if !a.connected() {
		return i18n.T("未连接")
	}
	return i18n.T("已连接")
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
