package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"tungo/internal/backup"
	"tungo/internal/i18n"
)

func (a *app) backupMenu(_ *ui.Context, m *ui.Menu) {
	if m.Item(i18n.T("立即备份")).Chosen() {
		a.backupNow()
	}
	if m.Item(i18n.T("还原最近一次备份")).Chosen() {
		a.restoreLatest()
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
func (a *app) restoreLatest() {
	if a.applied != "" {
		if err := a.clearSysProxy(); err != nil {
			a.logf(i18n.T("系统代理：%s"), err)
		}
	}
	paths, err := backup.List()
	if err != nil || len(paths) == 0 {
		a.hint(i18n.T("还没有备份"))
		return
	}
	restored, err := backup.Restore(paths[0])
	if err != nil {
		a.fail(err.Error())
		a.logf(i18n.T("还原失败：%s"), err)
		return
	}
	a.reloadAll()
	a.logf(i18n.T("已还原 %s（%s）"), paths[0], strings.Join(restored, ", "))
	a.say(i18n.T("已还原"))
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
	a.notice = ""
	a.know(a.kernelKind())
}
