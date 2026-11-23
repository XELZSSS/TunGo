package app

import (
	"github.com/egoist/mygo"

	"tungo/internal/i18n"
)

func (a *app) notifyResult(ok bool, text string) {
	if !a.notify || !mygo.NotificationsSupported() {
		return
	}
	title := i18n.T("已连接")
	if !ok {
		title = i18n.T("连接失败")
	}
	mygo.NewNotification(mygo.NotificationOptions{
		Title:  "TunGo · " + title,
		Body:   text,
		Silent: ok,
	}).Show()
}
