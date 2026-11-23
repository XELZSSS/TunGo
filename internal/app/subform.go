package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"tungo/internal/core"
	"tungo/internal/i18n"
	"tungo/internal/sub"
)

func (a *app) subForm(c *ui.Context) {
	if !a.editingSub {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.editingSub, func() {
		ui.Column(c).Gap(gapLoose).Width(dialogWide).Height(dialogBody).Children(func() {
			heading(c, i18n.T("订阅"))
			ui.Scroll(c).Grow(1).Padding(0, scrollGutter, 0, 0).Children(func() {
				ui.Column(c).Gap(gapTight).Children(func() {
					if len(a.subs.Items) == 0 {
						muted(c, t, i18n.T("还没有订阅"))
					}
					for i := range a.subs.Items {
						at := i
						item := a.subs.Items[i]
						label := fmt.Sprintf(i18n.T("%s（%s）"), item.Name, intervalName(item.Interval))
						if traffic := trafficName(item); traffic != "" {
							label += " · " + traffic
						}
						ui.Row(c).Gap(gapBase).AlignItems(ui.Center).Children(func() {
							ui.Text(c, label).
								FontSize(fontS).Grow(1).TextColor(t.Text)
							if ui.Button(c, i18n.T("使用")).
								Disabled(a.subs.Running() != nil && a.subs.Running().Name == item.Name).
								Clicked() {
								a.selectSub(item.Name)
							}
							if ui.Button(c, i18n.T("编辑")).Clicked() {
								a.editSub(at)
							}
							if ui.Button(c, i18n.T("更新")).Disabled(a.subBusy).Clicked() {
								a.refresh(item.Name)
							}
							if ui.Button(c, i18n.T("删除")).Clicked() {
								a.after(func() { a.removeSub(item.Name) })
							}
						})
					}
					if ui.Button(c, i18n.T("新建订阅")).Clicked() {
						a.editSub(-1)
					}
				})
			})
			if a.subNote != "" {
				muted(c, t, a.subNote)
			}
			ui.Row(c).Gap(gapBase).Justify(ui.End).Children(func() {
				if ui.Button(c, i18n.T("关闭")).Clicked() {
					a.editingSub = false
				}
			})
		})
	})
}

func (a *app) subEditForm(c *ui.Context) {
	if !a.editingSubItem {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.editingSubItem, func() {
		ui.Column(c).Gap(gapLoose).Width(dialogBase).Children(func() {
			heading(c, i18n.T("订阅"))
			ui.TextInput(c, &a.subDraft.Name).Label(i18n.T("名称")).Placeholder(i18n.T("如：机场 A")).Grow(1)
			ui.TextInput(c, &a.subDraft.URL).Label(i18n.T("订阅地址")).Placeholder("https://…").Grow(1)
			ui.TextInput(c, &a.subInterval).Label(i18n.T("自动更新间隔（分钟，0 表示不自动更新）")).Width(300)
			ui.TextInput(c, &a.subDraft.UA).Label(i18n.T("更新时的 User-Agent（留空用 TunGo）")).Placeholder("ClashMetaForAndroid/2.11.1").Grow(1)
			muted(c, t, i18n.T("更新时只替换这个订阅带来的节点，自己添加的和别的订阅的不动。"))
			ui.Row(c).Gap(gapBase).Justify(ui.End).Children(func() {
				if ui.Button(c, i18n.T("取消")).Clicked() {
					a.editingSubItem = false
				}
				if ui.PrimaryButton(c, i18n.T("保存并更新")).Clicked() {
					a.commitSub()
				}
			})
		})
	})
}

func (a *app) editSub(at int) {
	a.subAt = at
	a.subDraft = sub.Item{}
	if at >= 0 && at < len(a.subs.Items) {
		a.subDraft = a.subs.Items[at]
	}
	a.subInterval = fmt.Sprint(a.subDraft.Interval)
	a.editingSub = false
	a.editingSubItem = true
}

func (a *app) commitSub() {
	name := strings.TrimSpace(a.subDraft.Name)
	url := strings.TrimSpace(a.subDraft.URL)
	if name == "" || url == "" {
		a.fail(i18n.T("订阅要有名称和地址"))
		return
	}
	interval, err := strconv.Atoi(strings.TrimSpace(a.subInterval))
	if err != nil || interval < 0 {
		interval = 0
	}
	item := a.subDraft
	item.Name, item.URL, item.Interval = name, url, interval
	item.UA = strings.TrimSpace(item.UA)
	if old := a.subName(a.subAt); old != "" && old != name {
		a.subs.Remove(old)
		for i := range a.nodes {
			if a.nodes[i].Sub == old {
				a.nodes[i].Sub = name
			}
		}
		a.nodesChanged()
		a.save()
	}
	a.subs.Put(item)
	a.subs.Current = name
	a.editingSubItem = false
	a.editingSub = true
	a.logf(i18n.T("订阅：%s"), name)
	a.saveSubs()
	a.refresh(name)
}

func (a *app) selectSub(name string) {
	if a.subs.Item(name) == nil {
		return
	}
	a.subs.Current = name
	a.logf(i18n.T("订阅：%s"), name)
	a.saveSubs()
}

func (a *app) removeSub(name string) {
	if a.subs.Item(name) == nil {
		return
	}
	was := a.node()
	dropped := was.Sub == name
	kept := make([]core.Node, 0, len(a.nodes))
	removed := 0
	for _, node := range a.nodes {
		if node.Sub != name {
			kept = append(kept, node)
			continue
		}
		removed++
	}
	a.setNodes(kept)
	a.chosen = 0
	if at := a.indexOfNode(was); at >= 0 {
		a.chosen = at
	}
	a.subs.Remove(name)
	a.save()
	a.saveSubs()
	a.logf(i18n.T("已删除订阅：%s（%d 个节点）"), name, removed)
	if dropped && a.connected() {
		a.connect()
	}
}

func intervalName(minutes int) string {
	if minutes <= 0 {
		return i18n.T("不自动更新")
	}
	return fmt.Sprintf(i18n.T("每 %d 分钟"), minutes)
}

func trafficName(item sub.Item) string {
	parts := []string{}
	if item.Total > 0 {
		parts = append(parts, fmt.Sprintf(i18n.T("已用 %s / %s"), sizeName(item.Used), sizeName(item.Total)))
	}
	if item.Expire > 0 {
		parts = append(parts, fmt.Sprintf(i18n.T("到期 %s"), time.Unix(item.Expire, 0).Format("2006-01-02")))
	}
	return strings.Join(parts, " · ")
}

func sizeName(bytes int64) string {
	switch {
	case bytes < 1<<30:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1<<20))
	case bytes < 1<<40:
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1<<30))
	}
	return fmt.Sprintf("%.2f TB", float64(bytes)/(1<<40))
}
