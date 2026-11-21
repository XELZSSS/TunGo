package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

func (a *app) strategyNames() []string {
	ids := a.kernelKind().Strategies()
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = core.StrategyName(id)
	}
	return names
}

func strategyName(id string) string { return core.StrategyName(id) }

func strategyOf(name string) string {
	for _, id := range core.GroupStrategies {
		if core.StrategyName(id) == name {
			return id
		}
	}
	return core.GroupStrategies[0]
}

func (a *app) addGroup() {
	a.groupWas = core.Node{}
	a.groupName = ""
	a.groupStrategy = strategyName(core.GroupStrategies[0])
	a.groupPick = map[string]bool{}
	a.editing = false
	a.editingGroup = true
}

func (a *app) editGroup(i int) {
	node := a.nodes[i]
	names := a.strategyNames()
	a.groupWas = node
	a.groupName = node.Name
	a.groupStrategy = names[0]
	for _, name := range names {
		if name == strategyName(node.StrategyOr()) {
			a.groupStrategy = name
		}
	}
	a.groupPick = map[string]bool{}
	for _, name := range node.Children {
		a.groupPick[name] = true
	}
	a.editing = false
	a.editingGroup = true
}

func (a *app) editAny(i int) {
	if i < 0 || i >= len(a.nodes) {
		return
	}
	if a.nodes[i].Group() {
		a.editGroup(i)
		return
	}
	a.edit(i)
}

func (a *app) groupForm(c *ui.Context) {
	if !a.editingGroup {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.editingGroup, func() {
		ui.Column(c).Gap(gapBase).Width(560).Height(560).Children(func() {
			heading(c, i18n.T("策略组"))
			ui.Row(c).Gap(gapBase).Children(func() {
				ui.TextInput(c, &a.groupName).Label(i18n.T("备注")).Placeholder(i18n.T("备注")).Grow(1)
				ui.Select(c, &a.groupStrategy, a.strategyNames()).Label(i18n.T("策略组类型")).Width(180)
			})
			muted(c, t, i18n.T("内核在这组成员里按类型挑一个：最低延迟挑答得最快的，故障转移只用它，随机与轮询轮流用，最低负载挑负载最轻的。"))
			ui.Divider(c)
			if a.kernelKind() == core.SingBox {
				ui.Text(c, i18n.T("sing-box 内核只能按延迟挑（最低延迟或故障转移），随机、轮询、最低负载要用 xray 内核。")).
					FontSize(fontS).TextColor(t.Danger)
			}
			muted(c, t, i18n.T("成员（按顺序）"))
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(gapTight).Children(func() {
					for i := range a.nodes {
						node := a.nodes[i]
						if node.Direct() || node.Group() {
							continue
						}
						chosen := a.groupPick[node.Name]
						if ui.Checkbox(c, &chosen, node.Name).Changed() {
							a.groupPick[node.Name] = chosen
						}
					}
				})
			})
			if a.groupNote != "" {
				ui.Text(c, a.groupNote).FontSize(fontS).TextColor(t.Danger)
			}
			ui.Row(c).Gap(gapBase).Justify(ui.End).Children(func() {
				if ui.Button(c, i18n.T("取消")).Clicked() {
					a.editingGroup = false
				}
				if ui.PrimaryButton(c, i18n.T("保存")).Clicked() {
					a.commitGroup()
				}
			})
		})
	})
}

func (a *app) commitGroup() {
	name := strings.TrimSpace(a.groupName)
	children := make([]string, 0, len(a.nodes))
	for _, node := range a.nodes {
		if node.Direct() || node.Group() || !a.groupPick[node.Name] {
			continue
		}
		children = append(children, node.Name)
	}
	if name == "" || len(children) == 0 {
		a.groupNote = i18n.T("策略组要有名称，还要至少一个节点")
		return
	}
	at := a.indexOfNode(a.groupWas)
	for i, node := range a.nodes {
		if i != at && node.Name == name {
			a.groupNote = i18n.T("已有同名节点")
			return
		}
	}
	group := core.Node{Name: name, Role: core.GroupRole, Children: children, Strategy: strategyOf(a.groupStrategy)}
	if at >= 0 {
		group.Sub = a.nodes[at].Sub
		a.nodes[at] = group
	} else {
		a.appendNodes(group)
		a.chosen = len(a.nodes) - 1
	}
	a.nodesChanged()
	a.groupNote = ""
	a.editingGroup = false
	a.save()
	a.logf(i18n.T("已保存节点：%s（%s）"), group.Name, group.Summary())
	a.notice = ""
	if a.connected() {
		a.connect()
	}
}
