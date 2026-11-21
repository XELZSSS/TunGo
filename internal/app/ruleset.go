package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

func (a *app) openRuleBase() {
	a.ruleBaseText = a.ruleBase
	a.editingRuleBase = true
}

func (a *app) saveRuleBase() {
	a.ruleBase = strings.TrimSpace(a.ruleBaseText)
	core.SetSourceBase(a.ruleBase)
	a.editingRuleBase = false
	a.saveSettings()
	a.logf(i18n.T("规则集源：%s"), core.SourceBase())
}

func (a *app) refreshRuleSets() {
	kind := core.PickKind(core.Kind(a.kind))
	go func() {
		count, err := core.RefreshRuleSets(kind)
		a.update(func() {
			if err != nil {
				a.fail(err.Error())
				a.logf(i18n.T("更新规则集：%s"), err)
				return
			}
			a.logf(i18n.T("已更新 %d 个规则集"), count)
		})
	}()
}

func (a *app) ruleBaseForm(c *ui.Context) {
	if !a.editingRuleBase {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.editingRuleBase, func() {
		ui.Column(c).Gap(gapLoose).Width(560).Children(func() {
			heading(c, i18n.T("规则集镜像"))
			ui.TextInput(c, &a.ruleBaseText).Label(i18n.T("镜像前缀")).Placeholder("https://ghfast.top/https://raw.githubusercontent.com/").Grow(1)
			muted(c, t, i18n.T("规则集默认从 GitHub 下载，连不上时换成镜像前缀，留空恢复默认。"))
			ui.Row(c).Gap(gapBase).Justify(ui.End).Children(func() {
				if ui.Button(c, i18n.T("取消")).Clicked() {
					a.editingRuleBase = false
				}
				if ui.PrimaryButton(c, i18n.T("保存")).Clicked() {
					a.saveRuleBase()
				}
			})
		})
	})
}
