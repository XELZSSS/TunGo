package app

import (
	"net"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"tungo/internal/i18n"
	"tungo/internal/sysproxy"
)

func (a *app) applySysProxy() {
	if a.tun && a.connected() {
		a.hint(i18n.T("TUN 接管了全部流量，系统代理不用再设置"))
		return
	}
	if a.applied != "" {
		a.stopPac()
		if err := sysproxy.Clear(); err != nil {
			a.fail(err.Error())
			a.logf(i18n.T("系统代理：%s"), err)
			return
		}
		a.applied = ""
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
		if url, err = a.startPac(a.localAddr()); err == nil {
			err = sysproxy.EnablePac(url)
		}
		if err != nil {
			a.stopPac()
		}
	case sysproxy.ModeClear:
		err = sysproxy.Clear()
	default:
		a.saveSettings()
		return
	}
	if err != nil {
		a.applied = ""
		a.fail(err.Error())
		a.logf(i18n.T("系统代理：%s"), err)
		a.saveSettings()
		return
	}
	a.applied = ""
	if want == sysproxy.ModeSet || want == sysproxy.ModePac {
		a.applied = string(want)
	}
	a.notice = ""
	a.logf(i18n.T("系统代理：%s"), proxyName(want))
	a.saveSettings()
}

func (a *app) clearSysProxy() error {
	a.stopPac()
	if a.applied == "" {
		return nil
	}
	if err := sysproxy.Clear(); err != nil {
		return err
	}
	a.applied = ""
	a.saveSettings()
	return nil
}

func (a *app) setProxyMode(mode string) {
	if m := sysproxy.Mode(mode); m == sysproxy.ModeSet || m == sysproxy.ModePac {
		a.proxyOn = mode
	}
	a.proxy = mode
	a.applySysProxy()
	a.saveSettings()
}

func (a *app) toggleProxy() {
	if a.proxy == string(sysproxy.ModeClear) {
		a.setProxyMode(a.proxyOn)
		return
	}
	a.setProxyMode(string(sysproxy.ModeClear))
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

func proxyNames() []string {
	names := make([]string, len(sysproxy.Modes))
	for i, mode := range sysproxy.Modes {
		names[i] = proxyName(mode)
	}
	return names
}

func listenLabel(port int) string {
	if port <= 0 {
		return i18n.T("每次启动随机")
	}
	return strconv.Itoa(port)
}

func (a *app) openPort() {
	a.portText = ""
	if a.port > 0 {
		a.portText = strconv.Itoa(a.port)
	}
	a.editingPort = true
}

func (a *app) portForm(c *ui.Context) {
	if !a.editingPort {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.editingPort, func() {
		ui.Column(c).Gap(gapLoose).Width(420).Children(func() {
			heading(c, i18n.T("本地端口"))
			muted(c, t, i18n.T("终端和其它软件连这个端口；留空表示每次启动随机。改完正在用的连接会重连。"))
			ui.TextInput(c, &a.portText).Label(i18n.T("端口")).Placeholder("1080").Grow(1)
			ui.Row(c).Gap(gapBase).Justify(ui.End).Children(func() {
				if ui.Button(c, i18n.T("取消")).Clicked() {
					a.editingPort = false
				}
				if ui.PrimaryButton(c, i18n.T("保存")).Clicked() {
					a.savePort()
				}
			})
		})
	})
}

func (a *app) portLabel() string {
	if a.local != nil {
		if _, port, err := net.SplitHostPort(a.local.Addr()); err == nil {
			return port
		}
	}
	return listenLabel(a.port)
}

func (a *app) savePort() {
	port := 0
	if text := strings.TrimSpace(a.portText); text != "" {
		value, err := strconv.Atoi(text)
		if err != nil || value < 1 || value > 65535 {
			a.fail(i18n.T("端口要是一个 1 到 65535 的数字，留空表示每次启动随机"))
			return
		}
		port = value
	}
	a.editingPort = false
	if port == a.port {
		return
	}
	a.port = port
	a.saveSettings()
	a.logf(i18n.T("本地端口：%s"), listenLabel(port))
	if a.connected() {
		a.connect()
	}
}
