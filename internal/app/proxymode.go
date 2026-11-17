package app

import (
	"tungo/internal/i18n"
	"tungo/internal/sysproxy"
)

func (a *app) applySysProxy() {
	if a.tun && a.connected() {
		a.hint(i18n.T("TUN 接管了全部流量，系统代理不用再设置"))
		return
	}
	if a.applied {
		a.stopPac()
		if err := sysproxy.Clear(); err != nil {
			a.fail(err.Error())
			a.logf(i18n.T("系统代理：%s"), err)
			return
		}
		a.applied = false
	}
	want := sysproxy.Mode(a.proxy)
	var err error
	switch want {
	case sysproxy.ModeSet:
		if a.local == nil {
			a.hint(i18n.T("先连接，系统代理才有指向"))
			return
		}
		err = sysproxy.Enable(a.localAddr())
	case sysproxy.ModePac:
		if a.local == nil {
			a.hint(i18n.T("先连接，PAC 才有指向"))
			return
		}
		var url string
		if url, err = a.startPac(); err == nil {
			err = sysproxy.EnablePac(url)
		}
		if err != nil {
			a.stopPac()
		}
	case sysproxy.ModeClear:
		err = sysproxy.Clear()
	default:
		return
	}
	if err != nil {
		a.applied = false
		a.fail(err.Error())
		a.logf(i18n.T("系统代理：%s"), err)
		return
	}
	a.applied = want != sysproxy.ModeClear
	a.notice = ""
	a.logf(i18n.T("系统代理：%s"), proxyName(want))
}

func (a *app) clearSysProxy() error {
	a.stopPac()
	if !a.applied {
		return nil
	}
	if err := sysproxy.Clear(); err != nil {
		return err
	}
	a.applied = false
	return nil
}

func proxyName(mode sysproxy.Mode) string {
	switch mode {
	case sysproxy.ModeClear:
		return i18n.T("清除系统代理")
	case sysproxy.ModeSet:
		return i18n.T("自动配置系统代理")
	case sysproxy.ModePac:
		return i18n.T("PAC 模式")
	}
	return i18n.T("不改变系统代理")
}

var (
	proxyNamesAt    string
	proxyNamesCache []string
)

func proxyNames() []string {
	if proxyNamesAt == i18n.Lang() && proxyNamesCache != nil {
		return proxyNamesCache
	}
	names := make([]string, len(sysproxy.Modes))
	for i, mode := range sysproxy.Modes {
		names[i] = proxyName(mode)
	}
	proxyNamesAt, proxyNamesCache = i18n.Lang(), names
	return names
}
