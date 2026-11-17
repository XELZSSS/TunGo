package app

import (
	"fmt"
	"strings"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

func (a *app) kernelKind() core.Kind { return core.PickKind(core.Kind(a.kind), a.node()) }

func (a *app) kernelLabel() string {
	if a.installing != "" || a.updating {
		return i18n.T("下载中") + " " + a.installAt
	}
	kind := a.kernelKind()
	if a.kernel != nil {
		kind = a.kernel.Kind()
	}
	label := string(kind)
	if a.kind == string(core.Auto) {
		label = fmt.Sprintf(i18n.T("自动：%s"), kind)
	}
	if a.installNeeded() {
		return label + " " + i18n.T("未安装")
	}
	if a.latest != "" && a.versionFor == kind {
		if a.version != "" {
			label += " " + a.version
		}
		label += " → " + strings.TrimPrefix(a.latest, "v")
	}
	return label
}

func (a *app) know(kind core.Kind) {
	if a.versionFor == kind && a.version != "" {
		return
	}
	a.versionFor = kind
	a.version = ""
	a.latest = ""
	go func() {
		version := core.Version(kind)
		a.update(func() {
			if a.versionFor != kind {
				return
			}
			a.version = version
		})
	}()
}

func (a *app) check() {
	kind := a.kernelKind()
	if _, err := core.Binary(kind); err != nil {
		return
	}
	if a.checking || a.updating {
		return
	}
	a.checking = true
	go func() {
		latest, err := core.Latest(kind)
		installed := core.Version(kind)
		a.update(func() {
			a.checking = false
			a.version = installed

			if err != nil {
				a.latest = ""
				a.logf(i18n.T("检查 %s 更新失败：%s"), kind, err)
				return
			}
			if installed == "" {
				a.latest = ""
				a.logf(i18n.T("%s 的版本读不出来，跳过这次检查"), kind)
				return
			}
			if !core.Newer(latest, installed) {
				a.latest = ""
				return
			}
			if a.skipped[string(kind)] == latest {
				a.latest = ""
				a.logf(i18n.T("%s 有 %s，已按你的选择忽略"), kind, strings.TrimPrefix(latest, "v"))
				return
			}
			a.latest = latest
			a.event(i18n.T("%s 有新版本 %s（已装 %s），由你决定是否更新"), kind, strings.TrimPrefix(latest, "v"), installed)
		})
	}()
}

func (a *app) ignore() {
	if a.latest == "" {
		return
	}
	kind := string(a.kernelKind())
	tag := a.latest
	a.skipped[kind] = tag
	a.latest = ""
	a.logf(i18n.T("已忽略 %s %s，不再提示"), kind, strings.TrimPrefix(tag, "v"))
	a.saveSettings()
}

func (a *app) upgrade() {
	if a.updating || a.latest == "" {
		return
	}
	kind := a.kernelKind()
	tag := a.latest
	running := a.connected()
	system := a.applied

	if running {
		a.disconnect()
	}
	a.updating = true
	a.installAt = ""
	a.event(i18n.T("正在更新 %s 到 %s"), kind, strings.TrimPrefix(tag, "v"))

	back := func() {
		if !running {
			return
		}
		a.connect()
		if system && a.connected() {
			a.applySysProxy()
		}
	}
	go func() {
		_, err := core.Install(kind, tag, func(part, whole int64) {
			a.update(func() { a.installAt = size(part) + " / " + size(whole) })
		})
		a.update(func() {
			a.updating = false
			if err != nil {
				a.fail(err.Error())
				a.logf(i18n.T("更新失败：%s"), err)
				back()
				return
			}
			a.have = nil
			a.latest = ""
			a.versionFor = ""
			a.know(kind)
			a.event(i18n.T("%s 已更新"), kind)
			back()
		})
	}()
}

func (a *app) install() {
	if a.installing != "" {
		return
	}
	kind := a.kernelKind()
	name := string(kind)
	a.installing = name
	a.installAt = ""
	a.event(i18n.T("开始下载 %s"), name)
	go func() {
		_, err := core.Ensure(kind, func(part, whole int64) {
			a.update(func() { a.installAt = size(part) + " / " + size(whole) })
		})
		a.update(func() {
			a.installing = ""
			if err != nil {
				a.fail(err.Error())
				a.logf(i18n.T("%s 下载失败：%s"), name, err)
				return
			}
			a.say(fmt.Sprintf(i18n.T("%s 已装好，可以连接了"), name))
			a.have = nil
			a.versionFor = ""
			a.know(kind)
			a.event(i18n.T("%s 已装好"), name)
		})
	}()
}

func (a *app) installNeeded() bool {
	if len(a.nodes) == 0 || a.node().Direct() || a.connected() || a.installing != "" {
		return false
	}
	kind := a.kernelKind()
	if a.have == nil {
		a.have = map[core.Kind]bool{}
	}
	installed, known := a.have[kind]
	if !known {
		_, err := core.Binary(kind)
		installed = err == nil
		a.have[kind] = installed
	}
	return !installed
}
