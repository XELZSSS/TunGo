package app

import "tungo/internal/i18n"

func (a *app) hint(text string) { a.logf("%s", text) }

func (a *app) say(text string) { a.event("%s", text) }

func (a *app) fail(text string) {
	a.tab = 0
	a.logf(i18n.T("错误：%s"), text)
}
