package app

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"tungo/internal/core"
	"tungo/internal/i18n"
	"tungo/internal/proxy"
	"tungo/internal/sysproxy"
)

func (a *app) connect() {
	ignore := a.ignoreRules
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
			a.fail(fmt.Sprintf(i18n.T("安装包里没有 %s，请重新下载完整安装包"), kind))
			a.logf("%s", err)
			return
		}

		routing := core.LoadRouting()
		if missing := core.MissingRuleSets(routing, kind); len(missing) > 0 {
			if !ignore {
				a.hint(fmt.Sprintf(i18n.T("正在下载规则集：%s"), strings.Join(missing, ", ")))
				go a.ensureRules(routing, kind, rulesConnect)
				return
			}
			a.logf(i18n.T("已忽略缺失的规则集：%s"), strings.Join(missing, ", "))
		}
		a.know(kind)
		if kind == core.SingBox && (node.NeedsPin() || node.SkipsVerify()) {
			a.logf(i18n.T("sing-box 不支持证书 pin，已忽略：%s"), node.Name)
		}
	}

	wait, prev := a.drainState(nil), a.detach()
	a.ignoreRules = false
	a.tries++
	ticket := a.tries
	a.busy = true
	a.hint(i18n.T("正在启动内核…"))
	a.event(i18n.T("启动 %s：%s"), kind, node.Name)
	go a.launch(wait, prev, kind, node, a.port, tun, ticket)
}

func (a *app) launch(wait <-chan struct{}, prev running, kind core.Kind, node core.Node, port int, tun bool, ticket int) {
	if wait != nil {
		<-wait
	}
	prev.close()

	var kernel *core.Kernel
	var local *proxy.Server
	killOnce := sync.Once{}
	kill := func() { killOnce.Do(func() { a.finishLaunch(kernel, local) }) }
	a.registerLaunch(ticket, kill)
	defer a.dropLaunch(ticket)

	if !node.Direct() {
		started, err := startKernel(kind, node, tun)
		if err != nil {
			a.settle(ticket, func() {
				a.busy = false
				a.fail(err.Error())
				a.logf(i18n.T("内核起不来：%s"), err)
				if busyPort(err) {
					a.logf("%s", i18n.T("内核的内部端口被占用，请稍后重试或重启 TunGo"))
				}
			})
			return
		}
		kernel = started
	}

	note := ""
	if !tun {
		var dialer proxy.Dialer = &net.Dialer{Timeout: 10 * time.Second}
		if kernel != nil {
			dialer = &proxy.Upstream{Addr: kernel.SocksAddr()}
		}
		opened, err := proxy.Listen(listenAddr(port), dialer)
		if err != nil && port > 0 {
			if opened, err = proxy.Listen("127.0.0.1:0", dialer); err == nil {
				note = fmt.Sprintf(i18n.T("本地端口 %d 已被占用，这次改用 %s"), port, opened.Addr())
			}
		}
		if err != nil {
			kill()
			if kernel != nil {
				kernel.Stop()
			}
			a.settle(ticket, func() {
				a.busy = false
				a.fail(err.Error())
				a.logf("%s", err)
			})
			return
		}
		local = opened
	}
	a.update(func() {
		if ticket != a.tries {
			kill()
			return
		}
		a.busy = false
		a.kernel = kernel
		a.local = local
		a.stop = make(chan struct{})
		a.notice = ""
		if note != "" {
			a.logf("%s", note)
		}
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

type running struct {
	local  *proxy.Server
	kernel *core.Kernel
	stop   chan struct{}
}

func (r running) empty() bool { return r.local == nil && r.kernel == nil && r.stop == nil }

func (r running) close() {
	if r.stop != nil {
		close(r.stop)
	}
	if r.local != nil {
		r.local.Close()
	}
	if r.kernel != nil {
		r.kernel.Stop()
	}
}

func (a *app) detach() running {
	held := running{local: a.local, kernel: a.kernel, stop: a.stop}
	a.local, a.kernel, a.stop = nil, nil, nil
	return held
}

func (a *app) detachLaunch(held running) {
	if held.empty() {
		return
	}
	done := make(chan struct{})
	wait := a.drainState(done)
	go func() {
		if wait != nil {
			<-wait
		}
		held.close()
		close(done)
	}()
}

func (a *app) finishLaunch(kernel *core.Kernel, local *proxy.Server) {
	a.detachLaunch(running{kernel: kernel, local: local})
}

func (a *app) killLaunches() {
	a.launchMu.Lock()
	kills := a.launchKill
	a.launchKill = map[int]func(){}
	a.launchMu.Unlock()
	for _, kill := range kills {
		kill()
	}
}

func (a *app) registerLaunch(ticket int, kill func()) {
	a.launchMu.Lock()
	old := a.launchKill[ticket]
	if a.launchKill == nil {
		a.launchKill = map[int]func(){}
	}
	a.launchKill[ticket] = kill
	a.launchMu.Unlock()
	if old != nil {
		old()
	}
}

func (a *app) dropLaunch(ticket int) {
	a.launchMu.Lock()
	delete(a.launchKill, ticket)
	a.launchMu.Unlock()
}

func (a *app) restartMeasures() {
	a.cancelMeas()
	a.measureCtx, a.cancelMeas = context.WithCancel(context.Background())
}

func (a *app) disconnect() {
	was := a.connected()
	a.killLaunches()
	a.restartMeasures()
	a.tries++
	a.busy = false
	if err := a.clearSysProxy(); err != nil {
		a.fail(err.Error())
		a.logf(i18n.T("系统代理：%s"), err)
	}
	held := a.detach()
	a.detachLaunch(held)
	a.upRaw, a.downRaw, a.upBase, a.downBase = 0, 0, 0, 0
	a.sampleUp, a.sampleDown, a.hist = 0, 0, nil
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
		var localDone <-chan struct{}
		if local != nil {
			localDone = local.Done()
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
		case <-localDone:
			a.update(func() {
				if a.local != local {
					return
				}
				a.disconnect()
				a.fail(i18n.T("本地代理已停止，连接已断开"))
			})
			return
		case <-ticker.C:
			var up, down int64
			var conns []core.Conn
			if local != nil {
				up, down = local.Stats()
			}
			if kernel != nil && (local == nil || kernel.Kind() == core.SingBox) {
				atUp, atDown, list, err := kernel.Sessions()
				if err == nil {
					conns = list
					if local == nil {
						up, down = atUp, atDown
					}
				}
			}
			a.update(func() {
				a.upRaw, a.downRaw = up, down
				a.conns = conns
				a.pushTraffic(up, down)
			})
		}
	}
}

const trafficSpan = 30

func (a *app) pushTraffic(up, down int64) {
	delta := up + down - a.sampleUp - a.sampleDown
	a.sampleUp, a.sampleDown = up, down
	if delta < 0 {
		delta = 0
	}
	a.hist = append(a.hist, delta)
	if len(a.hist) > trafficSpan {
		a.hist = a.hist[len(a.hist)-trafficSpan:]
	}
}

func (a *app) drainState(next chan struct{}) chan struct{} {
	a.drainMu.Lock()
	defer a.drainMu.Unlock()
	held := a.drain
	a.drain = next
	return held
}

func (a *app) currentDrain() <-chan struct{} {
	a.drainMu.Lock()
	defer a.drainMu.Unlock()
	return a.drain
}

func startKernel(kind core.Kind, node core.Node, tun bool) (*core.Kernel, error) {
	kernel, err := startOnce(kind, node, tun)
	if err != nil && !tun && busyPort(err) {
		kernel, err = startOnce(kind, node, tun)
	}
	return kernel, err
}

func startOnce(kind core.Kind, node core.Node, tun bool) (*core.Kernel, error) {
	socks, err := kernelInbound()
	if err != nil {
		return nil, err
	}
	return core.Start(kind, node, socks, tun)
}

func busyPort(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, text := range []string{"address already in use", "only one usage of each socket address"} {
		if strings.Contains(msg, text) {
			return true
		}
	}
	return false
}

func kernelInbound() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	return l.Addr().String(), nil
}

func listenAddr(port int) string {
	if port <= 0 {
		return "127.0.0.1:0"
	}
	return net.JoinHostPort("127.0.0.1", fmt.Sprint(port))
}

func (a *app) settle(ticket int, fn func()) {
	a.update(func() {
		if ticket == a.tries {
			fn()
		}
	})
}
