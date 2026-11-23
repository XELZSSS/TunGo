package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

func (a *app) openMirror() {
	a.mirrorText = a.mirror
	a.editingMirror = true
}

func (a *app) saveMirror() {
	a.mirror = strings.TrimSpace(a.mirrorText)
	core.SetMirror(a.mirror)
	a.editingMirror = false
	a.saveSettings()
	a.logf(i18n.T("下载镜像：%s"), mirrorLabel())
}

func mirrorLabel() string {
	if prefix := core.Mirror(); prefix != "" {
		return prefix
	}
	return i18n.T("GitHub 直连")
}

func (a *app) refreshRuleSets() {
	kind := core.PickKind(core.Kind(a.kind))
	prefix := mirrorLabel()
	go func() {
		count, err := core.RefreshRuleSets(kind)
		a.update(func() {
			if count > 0 {
				a.logf(i18n.T("已更新 %d 个规则集（%s）"), count, prefix)
			}
			if err != nil {
				a.fail(err.Error())
				a.logf(i18n.T("更新规则集：%s"), err)
			}
		})
	}()
}

func (a *app) mirrorForm(c *ui.Context) {
	if !a.editingMirror {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.editingMirror, func() {
		ui.Column(c).Gap(gapLoose).Width(dialogBase).Children(func() {
			heading(c, i18n.T("下载镜像"))
			ui.TextInput(c, &a.mirrorText).Label(i18n.T("镜像前缀")).Placeholder("https://ghfast.top/").Grow(1)
			muted(c, t, i18n.T("内核、应用规则集都从 GitHub 下载，连不上时填一个镜像前缀，它直接拼在原始地址前面，留空就是直连。"))
			ui.Row(c).Gap(gapBase).Justify(ui.End).Children(func() {
				if ui.Button(c, i18n.T("取消")).Clicked() {
					a.editingMirror = false
				}
				if ui.PrimaryButton(c, i18n.T("保存")).Clicked() {
					a.saveMirror()
				}
			})
		})
	})
}
