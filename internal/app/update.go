package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tungo/internal/i18n"
)

const projectURL = "https://github.com/XELZSSS/TunGo"

func (a *app) about() {
	a.hint(fmt.Sprintf(i18n.T("TunGo %s · 数据在 %s"), mygo.App.Version(), mustDir()))
	if err := mygo.Shell.OpenExternal(projectURL); err != nil {
		a.logf(i18n.T("打不开项目主页：%s"), err)
	}
}

func (a *app) checkApp(quiet bool) {
	if a.releases.polling || a.releases.installing {
		return
	}
	a.releases.polling = true
	if !quiet {
		a.hint(i18n.T("正在检查应用更新…"))
	}
	go func() {
		update, err := mygo.Updater.Check(context.Background())
		a.update(func() {
			a.releases.polling = false
			switch {
			case errors.Is(err, mygo.ErrUpdatesDisabled):
				a.logf(i18n.T("应用更新不可用：%s"), err)
				if !quiet {
					a.fail(i18n.T("这个版本没带更新地址，请从项目主页下载新版"))
				}
			case err != nil:
				a.logf(i18n.T("检查应用更新失败：%s"), err)
				if !quiet {
					a.fail(fmt.Sprintf(i18n.T("检查应用更新失败：%s"), err))
				}
			case update == nil:
				a.logf(i18n.T("已是最新版本：%s"), mygo.App.Version())
				if !quiet {
					a.say(fmt.Sprintf(i18n.T("已是最新版本：%s"), mygo.App.Version()))
				}
			default:
				message := fmt.Sprintf(i18n.T("发现新版本 %s（当前 %s）"), update.Version, mygo.App.Version())
				a.releases.pending = update
				a.releases.dialog = true
				a.hint(message)
				a.logf("%s", message)
				if update.Notes != "" {
					a.logf(i18n.T("%s 的更新说明：%s"), update.Version, update.Notes)
				}
			}
		})
	}()
}

func (a *app) installApp() {
	update := a.releases.pending
	if update == nil || a.releases.installing {
		return
	}
	a.releases.installing = true
	a.releases.dialog = true
	a.releases.part, a.releases.whole = 0, 0
	drain := a.drain
	a.hint(fmt.Sprintf(i18n.T("正在下载 %s…"), update.Version))
	go func() {
		if drain != nil {
			<-drain
		}
		err := update.Install(context.Background(), func(part, whole int64) {
			a.update(func() {
				a.releases.part, a.releases.whole = part, whole
			})
		})
		a.update(func() {
			a.releases.installing = false
			a.releases.part, a.releases.whole = 0, 0
			if err != nil {
				a.logf(i18n.T("更新失败：%s"), err)
				a.fail(fmt.Sprintf(i18n.T("更新失败：%s"), err))
				return
			}
			a.releases.pending = nil
			a.releases.dialog = false
			a.say(fmt.Sprintf(i18n.T("已更新到 %s，正在重启…"), update.Version))
			mygo.App.Relaunch()
		})
	}()
}

func (a *app) updateDialog(c *ui.Context) {
	update := a.releases.pending
	if update == nil || !a.releases.dialog {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.releases.dialog, func() {
		ui.Column(c).Gap(gapBase).Width(480).Children(func() {
			heading(c, i18n.T("应用更新"))
			muted(c, t, fmt.Sprintf(i18n.T("发现新版本 %s（当前 %s）"), update.Version, mygo.App.Version()))
			if update.Notes != "" {
				ui.Scroll(c).Height(140).Children(func() {
					muted(c, t, update.Notes)
				})
			}
			if a.releases.installing {
				value := -1.0
				label := fmt.Sprintf(i18n.T("正在下载 %s…"), update.Version)
				if a.releases.whole > 0 {
					value = float64(a.releases.part) / float64(a.releases.whole)
					label += " " + size(a.releases.part) + " / " + size(a.releases.whole)
				}
				ui.Progress(c, value)
				code(c, t, label)
			}
			ui.Row(c).Gap(gapBase).Justify(ui.End).Children(func() {
				if ui.Button(c, i18n.T("查看发布说明")).Clicked() {
					if err := mygo.Shell.OpenExternal(projectURL + "/releases"); err != nil {
						a.logf(i18n.T("打不开项目主页：%s"), err)
					}
				}
				if ui.Button(c, i18n.T("稍后")).Disabled(a.releases.installing).Clicked() {
					a.releases.dialog = false
				}
				if ui.PrimaryButton(c, i18n.T("下载并安装")).Disabled(a.releases.installing).Clicked() {
					a.installApp()
				}
			})
		})
	})
}
