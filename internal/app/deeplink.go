package app

import (
	"fmt"
	"net/url"
	"strings"

	"tungo/internal/i18n"
	"tungo/internal/sub"
)

func (a *app) openLink(link string) {
	u, err := url.Parse(link)
	if err != nil || !strings.EqualFold(u.Scheme, "tungo") || u.Query().Get("url") == "" {
		return
	}
	a.after(func() {
		a.show()
		if u.Host == "import" {
			a.importLink(u.Query().Get("url"))
			return
		}
		a.subscribeLink(u.Query().Get("url"))
	})
}

func (a *app) importLink(target string) {
	nodes, err := sub.Parse([]byte(target))
	if err != nil {
		a.fail(err.Error())
		a.logf(i18n.T("导入失败：%s"), err)
		return
	}
	a.appendNodes(nodes...)
	a.save()
	a.say(fmt.Sprintf(i18n.T("已导入 %d 个节点"), len(nodes)))
}

func (a *app) subscribeLink(target string) {
	a.editSub(-1)
	a.subDraft.Name = i18n.T("导入的订阅")
	a.subDraft.URL = target
}
