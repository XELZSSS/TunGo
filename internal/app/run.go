package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tungo/internal/appdir"
	"tungo/internal/core"
	"tungo/internal/i18n"
	"tungo/internal/logs"
	"tungo/internal/sysproxy"
)

func mustDir() string {
	d, err := appdir.Dir()
	if err != nil {
		return "."
	}
	return d
}

func Run() {
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	mygo.App.SetName("TunGo")

	carry()

	if err := logs.Setup(filepath.Join(mustDir(), "logs")); err != nil {
		log.Print("no logs: ", err)
	}
	log.SetOutput(logs.Writer{})
	a := newApp()
	res, err := mygo.App.Path(mygo.PathResources)
	if err != nil {
		log.Fatal(err)
	}
	core.SetResources(res)
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.OnSecondInstance(func([]string, string) { a.show(); a.sync() })
	mygo.App.WhenReady(a.ready)
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { a.quit() })
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func carry() {
	d, err := appdir.Dir()
	if err != nil {
		log.Print(err)
		return
	}
	for _, place := range []struct {
		name mygo.PathName
		sub  string
	}{
		{mygo.PathUserData, "userdata"},
		{mygo.PathCache, "cache"},
		{mygo.PathLogs, "logs"},
	} {
		dir := filepath.Join(d, place.sub)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Print(err)
			continue
		}
		mygo.App.SetPath(place.name, dir)
	}
}

func (a *app) ready() {
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:     "TunGo",
		Width:     1300,
		Height:    800,
		MinWidth:  1280,
		MinHeight: 560,

		TitleBarStyle:     mygo.TitleBarHidden,
		HideWindowButtons: true,

		StateKey: "main",
		Content:  ui.View(a.view),
	})
	if icon, err := appIcon(256); err != nil {
		log.Print("no window icon: ", err)
	} else if err := a.win.SetIcon(icon); err != nil {
		log.Print("no window icon: ", err)
	}

	a.win.OnClose(func(e *mygo.CloseEvent) {
		if a.quitting {
			return
		}
		e.PreventDefault()
		a.win.Hide()
	})

	a.win.OnMaximize(func() { a.update(func() { a.maximized = true }) })
	a.win.OnResize(func() { a.update(func() { a.maximized = a.win.IsMaximized() }) })
	a.setUpTray()

	a.bindShortcuts()
	a.probe()
	a.checkApp(true)
	if item := a.subs.Running(); item != nil {
		a.refresh(item.Name)
	}
	go a.autoUpdate()

	go func() {
		time.Sleep(2 * time.Second)
		a.update(func() { a.know(a.kernelKind()) })
		a.update(a.check)
	}()
}

func (a *app) show() {
	a.win.Show()
	a.win.Focus()
}

func (a *app) quit() {
	a.tries++
	a.busy = false
	a.quitting = true
	if err := a.clearSysProxy(); err != nil {
		log.Print(err)
	}
	a.detach().close()
}

func (a *app) setUpTray() {
	icon, err := appIcon(32)
	if err != nil {
		log.Print("no tray icon: ", err)
		return
	}
	tray, err := mygo.NewTray(mygo.TrayOptions{
		Icon:    icon,
		ToolTip: "TunGo",
	})
	if err != nil {
		log.Print("no tray icon: ", err)
		return
	}
	a.tray = tray

	tray.OnClick(a.show)
	a.refreshTray()
}

func (a *app) refreshTray() {
	if a.tray == nil {
		return
	}
	nodes := radios(len(a.nodes), a.chosen, func(i int) string { return a.nodes[i].Name },
		func(i int) {
			a.choose(i)
			a.sync()
		})
	if len(nodes) == 0 {
		nodes = append(nodes, &mygo.MenuItem{Label: i18n.T("还没有节点"), Disabled: true})
	}
	routes := radios(len(a.routes.Items),
		slices.IndexFunc(a.routes.Items, func(item core.RoutingItem) bool { return item.Name == a.routes.Current }),
		func(i int) string { return routeLabel(a.routes.Items[i].Name) },
		func(i int) {
			a.selectRouting(a.routes.Items[i].Name)
			a.sync()
		})
	proxies := radios(len(sysproxy.Modes),
		slices.IndexFunc(sysproxy.Modes, func(mode sysproxy.Mode) bool { return string(mode) == a.proxy }),
		func(i int) string { return proxyName(sysproxy.Modes[i]) },
		func(i int) {
			a.setProxyMode(string(sysproxy.Modes[i]))
			a.sync()
		})
	a.tray.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Label: "TunGo — " + a.status(), Disabled: true},
		{Label: i18n.T("显示窗口"), Click: func(*mygo.MenuItem, *mygo.Window) { a.show() }},
		mygo.Separator(),
		{Label: i18n.T("服务器"), Submenu: nodes},
		{Label: i18n.T("连接"), Type: mygo.MenuItemCheckbox, Checked: a.connected(),
			Click: func(*mygo.MenuItem, *mygo.Window) {
				if a.connected() {
					a.disconnect()
				} else {
					a.connect()
				}
				a.sync()
			}},
		{Label: i18n.T("路由"), Submenu: routes},
		{Label: i18n.T("系统代理"), Submenu: proxies},
		mygo.Separator(),
		{Label: i18n.T("更新订阅"), Disabled: a.subs.Running() == nil, Click: func(*mygo.MenuItem, *mygo.Window) {
			if item := a.subs.Running(); item != nil {
				a.refresh(item.Name)
			}
		}},
		{Label: i18n.T("更新全部订阅"), Disabled: len(a.subs.Items) == 0, Click: func(*mygo.MenuItem, *mygo.Window) { a.refreshAll() }},
		{Label: i18n.T("复制终端代理命令"), Click: func(*mygo.MenuItem, *mygo.Window) { a.copyProxyCommand() }},
		mygo.Separator(),
		{Label: i18n.T("开机自启"), Type: mygo.MenuItemCheckbox, Checked: mygo.App.OpenAtLogin(),
			Click: func(it *mygo.MenuItem, _ *mygo.Window) {
				if err := mygo.App.SetOpenAtLogin(it.Checked); err != nil {
					log.Print(err)
				}
			}},
		{Label: i18n.T("打开日志目录"), Click: func(*mygo.MenuItem, *mygo.Window) { a.openLogs() }},
		mygo.Separator(),
		{Label: i18n.T("退出 TunGo"), Click: func(*mygo.MenuItem, *mygo.Window) {
			a.quitting = true
			mygo.App.Quit()
		}},
	}))
	a.tray.SetToolTip("TunGo — " + a.status())
}

func radios(count, selected int, label func(int) string, click func(int)) []*mygo.MenuItem {
	items := make([]*mygo.MenuItem, 0, count)
	for i := 0; i < count; i++ {
		at := i
		items = append(items, &mygo.MenuItem{
			Label:   label(at),
			Type:    mygo.MenuItemRadio,
			Checked: at == selected,
			Click: func(*mygo.MenuItem, *mygo.Window) {
				click(at)
			},
		})
	}
	return items
}

func (a *app) copyProxyCommand() {
	addr := a.localAddr()
	if a.local == nil {
		a.hint(i18n.T("先连接，终端才有代理可用"))
		return
	}
	a.writeNext = fmt.Sprintf("set http_proxy=http://%s\nset https_proxy=http://%s\nset all_proxy=socks5://%s\n\nexport http_proxy=http://%s\nexport https_proxy=http://%s\nexport all_proxy=socks5://%s",
		addr, addr, addr, addr, addr, addr)
	a.say(i18n.T("终端代理命令已复制"))
	a.update(func() {})
}
