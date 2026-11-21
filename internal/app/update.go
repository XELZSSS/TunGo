package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/egoist/mygo"

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
		a.event("%s", i18n.T("正在检查应用更新…"))
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cleanup = append(a.cleanup, cancel)
	go func() {
		update, err := mygo.Updater.Check(ctx)
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
				a.releases.pending = update
				a.event(i18n.T("发现新版本 %s（当前 %s）"), update.Version, mygo.App.Version())
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
	a.releases.progress = ""
	if a.connected() {
		a.disconnect()
	}
	drain := a.drain
	a.event(i18n.T("正在下载 %s…"), update.Version)
	ctx, cancel := context.WithCancel(context.Background())
	a.cleanup = append(a.cleanup, cancel)
	go func() {
		defer cancel()
		if drain != nil {
			select {
			case <-drain:
			case <-ctx.Done():
				return
			}
		}
		err := update.Install(ctx, func(part, whole int64) {
			a.update(func() {
				a.releases.progress = size(part) + " / " + size(whole)
			})
		})
		a.update(func() {
			if ctx.Err() != nil {
				return
			}
			a.releases.installing = false
			a.releases.progress = ""
			if err != nil {
				a.logf(i18n.T("更新失败：%s"), err)
				a.fail(fmt.Sprintf(i18n.T("更新失败：%s"), err))
				return
			}
			a.releases.pending = nil
			a.event(i18n.T("已更新到 %s，正在重启…"), update.Version)
			mygo.App.Relaunch()
		})
	}()
}
