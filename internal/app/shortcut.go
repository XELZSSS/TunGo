package app

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tungo/internal/i18n"
)

const (
	shortcutShow  = "show"
	shortcutConn  = "connect"
	shortcutProxy = "proxy"
)

var shortcutNames = map[string]string{
	shortcutShow:  "显示窗口",
	shortcutConn:  "连接 / 断开",
	shortcutProxy: "系统代理开关",
}

func (a *app) shortcutForm(c *ui.Context) {
	if !a.editingShortcuts {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.editingShortcuts, func() {
		ui.Column(c).Gap(gapLoose).Width(520).Children(func() {
			heading(c, i18n.T("全局热键"))
			muted(c, t, i18n.T("写一个组合键，如 CommandOrControl+Alt+T；留空表示不绑定。它由系统截获，别的窗口有焦点时也生效。"))
			for _, one := range []struct {
				action string
				key    *string
			}{
				{shortcutShow, &a.shortcutShowKey},
				{shortcutConn, &a.shortcutConnKey},
				{shortcutProxy, &a.shortcutProxyKey},
			} {
				ui.TextInput(c, one.key).Label(i18n.T(shortcutNames[one.action])).Placeholder("CommandOrControl+Alt+T").Grow(1)
			}
			ui.Row(c).Gap(gapBase).Justify(ui.End).Children(func() {
				if ui.Button(c, i18n.T("取消")).Clicked() {
					a.editingShortcuts = false
				}
				if ui.PrimaryButton(c, i18n.T("保存")).Clicked() {
					a.editingShortcuts = false
					a.saveShortcuts()
				}
			})
		})
	})
}

func (a *app) openShortcuts() {
	saved, err := loadSettings()
	if err != nil {
		a.fail(err.Error())
		a.logf("%s", err)
		return
	}
	shortcuts := saved.Shortcuts
	if shortcuts == nil {
		shortcuts = map[string]string{}
	}
	a.shortcuts = shortcuts
	a.shortcutShowKey = shortcuts[shortcutShow]
	a.shortcutConnKey = shortcuts[shortcutConn]
	a.shortcutProxyKey = shortcuts[shortcutProxy]
	a.editingShortcuts = true
}

func (a *app) saveShortcuts() {
	a.shortcuts = map[string]string{}
	for action, key := range map[string]string{
		shortcutShow:  a.shortcutShowKey,
		shortcutConn:  a.shortcutConnKey,
		shortcutProxy: a.shortcutProxyKey,
	} {
		if key = strings.TrimSpace(key); key != "" {
			a.shortcuts[action] = key
		}
	}
	a.saveSettings()
	a.bindShortcuts()
}

func (a *app) bindShortcuts() {
	mygo.GlobalShortcut.UnregisterAll()
	for action, key := range a.shortcuts {
		name := action
		err := mygo.GlobalShortcut.Register(key, func() { a.shortcut(name) })
		if err != nil {
			a.logf(i18n.T("热键 %s 没绑上：%s"), key, err)
			continue
		}
		a.logf(i18n.T("热键 %s：%s"), key, i18n.T(shortcutNames[name]))
	}
}

func (a *app) shortcut(action string) {
	a.update(func() {
		switch action {
		case shortcutShow:
			a.show()
		case shortcutConn:
			if a.connected() {
				a.disconnect()
			} else {
				a.connect()
			}
			a.refreshTray()
		case shortcutProxy:
			a.toggleProxy()
		}
	})
}
