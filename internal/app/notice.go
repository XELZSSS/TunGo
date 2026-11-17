package app

import (
	"github.com/egoist/mygo/ui"
)

type noticeKind int

const (
	noticeHint noticeKind = iota
	noticeDone
	noticeFail
)

func (a *app) hint(text string) { a.notice, a.noticeKind = text, noticeHint }

func (a *app) say(text string) { a.notice, a.noticeKind = text, noticeDone }

func (a *app) fail(text string) { a.notice, a.noticeKind = text, noticeFail }

func (a *app) noticeColor(t *ui.Theme) ui.Color {
	switch a.noticeKind {
	case noticeDone:
		return t.Success
	case noticeFail:
		return t.Danger
	}
	return t.TextMuted
}
