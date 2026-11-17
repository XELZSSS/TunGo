package app

import (
	"fmt"
	"net"
	"strings"
	"time"

	"tungo/internal/core"
	"tungo/internal/i18n"
	"tungo/internal/proxy"
	"tungo/internal/sysproxy"
)

func (a *app) connect() {
	if a.busy {
		return
	}
	if len(a.nodes) == 0 {
		a.hint(i18n.T("还没有节点：从「服务器」导入分享链接，或设置订阅"))
		return
	}
	node := a.node()
	if node.Group() && len(node.Members) == 0 {
		a.hint(i18n.T("策略组里没有可用的节点"))
		return
	}
	tun := a.tun && !node.Direct()
	if tun && !admin() {
		a.hint(i18n.T("TUN 模式要管理员权限：从「设置」里重启为管理员"))
		return
	}

	kind := a.kernelKind()
	if !core.Carries(kind, node) {
		what := node.Protocol
		if what == "" {
			what = i18n.T("策略组")
		}
		a.hint(fmt.Sprintf(i18n.T("%s 带不了 %s，请在菜单里换一个内核"), kind, what))
		return
	}
	if !node.Direct() {
		if _, err := core.Binary(kind); err != nil {
			a.hint(fmt.Sprintf(i18n.T("包里没有 %s，正在下载"), kind))
			a.install()
			return
		}

		routing, err := core.LoadRouting()
		if err != nil {
			a.fail(err.Error())
			return
		}
		if missing := core.MissingRuleSets(routing, kind); len(missing) > 0 {
			a.hint(fmt.Sprintf(i18n.T("正在下载规则集：%s"), strings.Join(missing, "、")))
			go a.ensureRules(routing, true)
			return
		}
		a.know(kind)
	}
	a.stopLocal()

	a.tries++
	a.busy = true
	a.hint(i18n.T("正在启动内核…"))
	a.event(i18n.T("启动 %s：%s"), kind, node.Name)
	go a.launch(kind, node, tun, a.tries)
}

func (a *app) launch(kind core.Kind, node core.Node, tun bool, ticket int) {
	var kernel *core.Kernel
	if !node.Direct() {
		started, err := core.Start(kind, node, coreSocks, tun)
		if err != nil {
			a.update(func() {
				if ticket != a.tries {
					return
				}
				a.busy = false
				a.fail(err.Error())
				a.logf(i18n.T("内核起不来：%s"), err)
			})
			return
		}
		kernel = started
	}
	var local *proxy.Server
	if !tun {
		var dialer proxy.Dialer = &net.Dialer{Timeout: 10 * time.Second}
		if kernel != nil {
			dialer = &proxy.Upstream{Addr: kernel.SocksAddr()}
		}
		opened, err := proxy.Listen(net.JoinHostPort("127.0.0.1", a.port), dialer)
		if err != nil {
			if kernel != nil {
				kernel.Stop()
			}
			a.update(func() {
				if ticket != a.tries {
					return
				}
				a.busy = false
				a.fail(fmt.Sprintf(i18n.T("本地端口 %s 起不来：%s"), a.port, err))
				a.logf("%s", err)
			})
			return
		}
		local = opened
	}
	a.update(func() {
		if ticket != a.tries {

			if local != nil {
				local.Close()
			}
			if kernel != nil {
				kernel.Stop()
			}
			return
		}
		a.busy = false
		a.kernel = kernel
		a.local = local
		a.stop = make(chan struct{})
		a.notice = ""
		if tun {
			a.event(i18n.T("已连接：%s 接管全部流量"), node.Name)
			if err := a.clearSysProxy(); err != nil {
				a.logf(i18n.T("系统代理：%s"), err)
			}
		} else {
			a.event(i18n.T("已连接：%s 在 %s"), node.Name, local.Addr())
			if a.proxy == string(sysproxy.ModeSet) || a.proxy == string(sysproxy.ModePac) {
				a.applySysProxy()
			}
		}
		go a.watch(a.stop, local, kernel)
	})
}

func (a *app) stopLocal() {
	if a.local != nil {
		a.local.Close()
		a.local = nil
	}
	if a.stop != nil {
		close(a.stop)
		a.stop = nil
	}
	if a.kernel != nil {
		a.kernel.Stop()
		a.kernel = nil
	}
}

func (a *app) disconnect() {
	was := a.connected()

	a.tries++
	a.busy = false
	if err := a.clearSysProxy(); err != nil {
		a.fail(err.Error())
		a.logf(i18n.T("系统代理：%s"), err)
	}
	a.stopLocal()
	a.up, a.down = 0, 0
	a.conns = nil
	if was {
		a.event("%s", i18n.T("已断开"))
	}
}

func (a *app) watch(stop <-chan struct{}, local *proxy.Server, kernel *core.Kernel) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		var gone <-chan struct{}
		if kernel != nil {
			gone = kernel.Done()
		}
		select {
		case <-stop:
			return
		case <-gone:

			a.update(func() {
				if a.kernel != kernel {
					return
				}
				a.disconnect()
				a.fail(i18n.T("内核已退出，连接已断开"))
				a.event(i18n.T("内核已退出：%s"), kernel.Output())
			})
			return
		case <-ticker.C:
			var up, down int64
			var conns []core.Conn
			if local != nil {
				up, down = local.Stats()
			}
			if kernel != nil {
				atUp, atDown, list, err := kernel.Sessions()
				if err == nil {
					conns = list
					if local == nil {
						up, down = atUp, atDown
					}
				}
			}
			a.update(func() {
				a.up, a.down = up, down
				a.conns = conns
			})
		}
	}
}
