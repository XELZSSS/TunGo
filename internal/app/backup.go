package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"tungo/internal/backup"
	"tungo/internal/core"
	"tungo/internal/i18n"
	"tungo/internal/sub"
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
	a.logf(i18n.T("已还原 %s（%s）"), paths[0], strings.Join(restored, "、"))
	a.say(i18n.T("已还原"))
}

func (a *app) reloadAll() {
	if a.connected() {
		a.disconnect()
	}
	nodes, at, err := core.LoadNodes()
	if err != nil {
		a.fail(err.Error())
		a.logf("%s", err)
		return
	}
	routes, err := core.LoadRoutingSet()
	if err != nil {
		a.fail(err.Error())
		a.logf("%s", err)
		return
	}
	remembered, err := loadSettings()
	if err != nil {
		a.fail(err.Error())
		a.logf("%s", err)
		return
	}

	a.nodesAt = at
	a.setNodes(nodes)
	a.chosen = 0
	a.subs = sub.Load()
	a.routes = routes
	i18n.Set(remembered.Lang)
	a.skipped, a.shortcuts = remembered.Skipped, remembered.Shortcuts
	if remembered.Kind != "" {
		a.kind = remembered.Kind
	}
	if remembered.Theme != "" {
		a.themeName = remembered.Theme
	}
	if remembered.Proxy != "" {
		a.proxy = remembered.Proxy
	}
	a.tun = remembered.Tun
	a.notice = ""
}
