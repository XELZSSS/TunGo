package app

import (
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tungo/internal/backup"
	"tungo/internal/i18n"
)

func (a *app) backupMenu(_ *ui.Context, m *ui.Menu) {
	if m.Item(i18n.T("立即备份")).Chosen() {
		a.backupNow()
	}
	if m.Item(i18n.T("还原备份")).Chosen() {
		a.openRestores()
	}
	if m.Item(i18n.T("导入备份文件")).Chosen() {
		a.importBackup()
	}
	m.Separator()
	if m.Item(i18n.T("打开备份文件夹")).Chosen() {
		if dir, err := backup.Dir(); err == nil {
			a.reveal(dir)
		}
	}
}

func (a *app) backupNow() {
	path, err := backup.Make()
	if err != nil {
		a.fail(err.Error())
		a.logf(i18n.T("备份失败：%s"), err)
		return
	}
	a.logf(i18n.T("已备份到 %s"), path)
	a.say(i18n.T("已备份"))
}

func (a *app) openRestores() {
	paths, err := backup.List()
	if err != nil {
		a.fail(err.Error())
		a.logf("%s", err)
		return
	}
	if len(paths) == 0 {
		a.hint(i18n.T("还没有备份"))
		return
	}
	a.paths = paths
	a.listing = true
}

func (a *app) importBackup() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: a.win,
		Title:  i18n.T("选择备份文件"),
		Filters: []mygo.FileFilter{
			{Name: i18n.T("备份包"), Extensions: []string{"zip", "*"}},
		},
	})
	if err != nil {
		a.fail(err.Error())
		a.logf("%s", err)
		return
	}
	if len(paths) == 0 {
		return
	}
	a.restoreFrom(paths[0])
}

func (a *app) restoreFrom(path string) {
	a.listing = false
	if a.applied != "" {
		if err := a.clearSysProxy(); err != nil {
			a.logf(i18n.T("系统代理：%s"), err)
		}
	}
	restored, err := backup.Restore(path)
	if err != nil {
		a.fail(err.Error())
		a.logf(i18n.T("还原失败：%s"), err)
		return
	}
	a.reloadAll()
	a.logf(i18n.T("已还原 %s（%s）"), filepath.Base(path), strings.Join(restored, ", "))
	a.say(i18n.T("已还原"))
}

func (a *app) restoreForm(c *ui.Context) {
	if !a.listing {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.listing, func() {
		ui.Column(c).Gap(gapLoose).Width(dialogBase).Height(dialogBody).Children(func() {
			heading(c, i18n.T("还原备份"))
			muted(c, t, i18n.T("选一份备份，节点、订阅、路由、DNS 和设置都会被它覆盖，正在用的连接会断开。"))
			ui.Scroll(c).Grow(1).Padding(0, scrollGutter, 0, 0).Children(func() {
				ui.Column(c).Gap(gapTight).Children(func() {
					for _, path := range a.paths {
						one := path
						ui.Row(c).Gap(gapBase).AlignItems(ui.Center).Children(func() {
							code(c, t, filepath.Base(one))
							ui.Box(c).Grow(1)
							if ui.Button(c, i18n.T("还原")).Clicked() {
								a.restoreFrom(one)
							}
						})
					}
				})
			})
			ui.Row(c).Gap(gapBase).Justify(ui.End).Children(func() {
				if ui.Button(c, i18n.T("关闭")).Clicked() {
					a.listing = false
				}
			})
		})
	})
}

func (a *app) reloadAll() {
	if a.connected() {
		a.disconnect()
	}
	if err := a.loadAll(); err != nil {
		a.fail(err.Error())
		a.logf("%s", err)
		return
	}
	a.versionFor, a.version, a.latest = "", "", ""
	a.bindShortcuts()
	a.know(a.kernelKind())
}
