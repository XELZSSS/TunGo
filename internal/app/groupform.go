package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

var groupStrategies = []struct {
	strategy string
	label    string
}{
	{"leastPing", "最低延迟"},
	{"fallback", "故障转移"},
	{"random", "随机"},
	{"roundRobin", "轮询"},
	{"leastLoad", "最低负载"},
}

func strategyNames() []string {
	names := make([]string, len(groupStrategies))
	for i, one := range groupStrategies {
		names[i] = i18n.T(one.label)
	}
	return names
}

func strategyName(strategy string) string {
	for _, one := range groupStrategies {
		if one.strategy == strategy {
			return i18n.T(one.label)
		}
	}
	return i18n.T(groupStrategies[0].label)
}

func strategyOf(name string) string {
	for _, one := range groupStrategies {
		if i18n.T(one.label) == name {
			return one.strategy
		}
	}
	return groupStrategies[0].strategy
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
	a.groupWas = node
	a.groupName = node.Name
	a.groupStrategy = strategyName(node.StrategyOr())
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
				ui.Select(c, &a.groupStrategy, strategyNames()).Label(i18n.T("策略组类型")).Width(180)
			})
			muted(c, t, i18n.T("内核在这组成员里按类型挑一个：最低延迟挑答得最快的，故障转移只用它，随机与轮询轮流用，最低负载挑负载最轻的。"))
			ui.Divider(c)
			if a.kernelKind() == core.SingBox {
				ui.Text(c, i18n.T("sing-box 内核只会按延迟挑（故障转移也只用最低延迟那个），随机、轮询、最低负载要换成 xray 内核才有效。")).
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
	group := core.Node{Name: name, Role: core.GroupRole, Children: children, Strategy: strategyOf(a.groupStrategy)}
	if at := a.indexOfNode(a.groupWas); at >= 0 {
		group.Sub = a.nodes[at].Sub
		a.nodes[at] = group
	} else {
		a.nodes = append(a.nodes, group)
		a.ms = append(a.ms, -1)
		a.speed = append(a.speed, 0)
		a.chosen = len(a.nodes) - 1
	}
	a.groupNote = ""
	a.editingGroup = false
	a.save()
	a.logf(i18n.T("已保存节点：%s（%s）"), group.Name, group.Summary())
	a.notice = ""
	if a.connected() {
		a.connect()
	}
}
