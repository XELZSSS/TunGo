package app

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tungo/internal/core"
	"tungo/internal/i18n"
	"tungo/internal/logs"
	"tungo/internal/proxy"
	"tungo/internal/sub"
	"tungo/internal/sysproxy"
)

const (
	appPort   = "10809"
	coreSocks = "127.0.0.1:10810"
)

const probeAddr = "1.1.1.1:443"

const keepLogs = 200

var direct = core.Node{Name: "直连（不经节点）", Protocol: "direct"}

type app struct {
	win  *mygo.Window
	tray *mygo.Tray

	quitting bool

	maximized bool

	themeName string

	nodes      []core.Node
	nodesAt    string
	chosen     int
	list       ui.ListState
	connList   ui.ListState
	conns      []core.Conn
	panePct    float32
	tab        int
	group      string
	shown      []int
	row        int
	rowAt      int
	kind       string
	port       string
	ms         []int
	speed      []float64
	notice     string
	noticeKind noticeKind
	busy       bool
	tries      int
	logs       []string
	logScroll  ui.ScrollState

	writeNext string
	queued    []func()

	subs       sub.State
	subNote    string
	subBusy    bool
	subDraft   sub.Item
	subAt      int
	subFormAt  string
	installing string
	installAt  string

	version    string
	versionFor core.Kind
	latest     string
	skipped    map[string]string
	checking   bool
	have       map[core.Kind]bool
	updating   bool

	editing        bool
	editingGroup   bool
	groupWas       core.Node
	groupName      string
	groupStrategy  string
	groupPick      map[string]bool
	groupNote      string
	editingSub     bool
	editingSubItem bool
	subInterval    string

	shortcuts        map[string]string
	shortcutShowKey  string
	shortcutConnKey  string
	shortcutProxyKey string
	editingShortcuts bool

	routes           core.RoutingSet
	editingRoute     bool
	editingRouteName bool
	routeName        string
	rules            []core.Rule
	routeFinalAt     int
	routeStrategy    string

	editingRule   bool
	ruleAt        int
	ruleDraft     core.Rule
	ruleDomains   string
	ruleIPs       string
	rulePort      string
	ruleNetwork   string
	ruleProtocols string
	editingDNS    bool
	dnsRemote     string
	dnsDirect     string
	dnsBootstrap  string
	dnsStrategy   string
	dnsBlockAAAA  bool
	dnsHosts      string
	dnsSplit      string
	draft         core.Node
	draftPort     string
	draftWas      core.Node
	detourName    string
	rawText       string
	alterID       string
	alpn          string

	local  *proxy.Server
	kernel *core.Kernel
	stop   chan struct{}
	pac    *http.Server

	proxy   string
	tun     bool
	applied bool
	up      int64
	down    int64
}

func newApp() *app {
	core.WriteMirrors()
	nodes, at, err := core.LoadNodes()
	routes, routeErr := core.LoadRoutingSet()
	a := &app{
		nodes:     nodes,
		nodesAt:   at,
		kind:      string(core.Auto),
		routes:    routes,
		proxy:     string(sysproxy.ModeSet),
		themeName: "light",
		port:      appPort,
		panePct:   65,
		ms:        make([]int, len(nodes)),
		speed:     make([]float64, len(nodes)),
		subs:      sub.Load(),
	}
	for i := range a.ms {
		a.ms[i] = -1
	}
	a.list.Selected = &a.row
	a.list.RowHeight = tableRowHeight
	a.connList.RowHeight = tableRowHeight
	a.connList.Key = func(row int) any {
		if row < 0 || row >= len(a.conns) {
			return nil
		}
		return a.conns[row].ID
	}
	remembered, recallErr := loadSettings()
	i18n.Set(remembered.Lang)
	a.skipped, a.shortcuts = remembered.Skipped, remembered.Shortcuts
	if remembered.Kind != "" {
		a.kind = remembered.Kind
	}
	if remembered.Theme != "" {
		a.themeName = remembered.Theme
	}

	if remembered.Proxy != "" && remembered.Proxy != string(sysproxy.ModeClear) {
		a.proxy = remembered.Proxy
	}
	a.tun = remembered.Tun
	if err != nil {
		a.fail(err.Error())
		a.logf("%s", err)
	}
	if routeErr != nil {
		a.fail(routeErr.Error())
		a.logf("%s", routeErr)
	}
	if recallErr != nil {
		a.fail(recallErr.Error())
		a.logf("%s", recallErr)
	}
	a.event(i18n.T("TunGo 启动，%d 个节点"), len(nodes))
	return a
}

func (a *app) node() core.Node {
	if a.chosen < 0 || a.chosen >= len(a.nodes) {
		return core.Node{}
	}
	return a.resolved(a.nodes[a.chosen])
}

func (a *app) resolved(n core.Node) core.Node {
	if !n.Group() {
		return n
	}
	n.Members = nil
	for _, name := range n.Children {
		for _, node := range a.nodes {
			if node.Name != name || node.Group() || node.Direct() {
				continue
			}
			n.Members = append(n.Members, node)
			break
		}
	}
	return n
}

func (a *app) connected() bool { return a.local != nil || a.kernel != nil }

func (a *app) localAddr() string {
	if a.local != nil {
		return a.local.Addr()
	}
	return net.JoinHostPort("127.0.0.1", a.port)
}

func (a *app) event(format string, args ...any) { a.record(true, format, args...) }

func (a *app) logf(format string, args ...any) { a.record(false, format, args...) }

func (a *app) record(panel bool, format string, args ...any) {
	line := time.Now().Format("15:04:05") + "  " + fmt.Sprintf(format, args...)
	if panel {
		a.logs = append(a.logs, line)
		if len(a.logs) > keepLogs {
			a.logs = a.logs[len(a.logs)-keepLogs:]
		}
	}
	logs.Write(line)
}

func (a *app) openLogs() {
	dir := logs.Dir()
	if dir == "" {
		a.event("%s", i18n.T("还没有日志目录"))
		return
	}
	a.reveal(dir)
}

func (a *app) saveSettings() {
	saved := settings{
		Skipped: a.skipped, Kind: a.kind, Theme: a.themeName, Lang: i18n.Lang(),
		Proxy: a.proxy, Tun: a.tun, Shortcuts: a.shortcuts,
	}
	if err := saved.save(); err != nil {
		a.logf(i18n.T("保存设置失败：%s"), err)
	}
}

func (a *app) setTheme(name string) {
	a.themeName = name
	a.saveSettings()
}

func (a *app) setTun(on bool) {
	a.tun = on
	a.saveSettings()
	if a.connected() || a.busy {
		a.disconnect()
		a.connect()
	}
	a.sync()
}

func (a *app) setLang(lang string) {
	i18n.Set(lang)
	a.saveSettings()
	a.sync()
}

func (a *app) after(fn func()) {
	a.queued = append(a.queued, fn)
	a.update(func() {})
}

func (a *app) runQueued() {
	queued := a.queued
	if len(queued) == 0 {
		return
	}
	a.queued = nil
	for _, fn := range queued {
		fn()
	}
}

func (a *app) update(fn func()) {
	if a.win != nil {
		a.win.Update(fn)
		return
	}
	fn()
}

func (a *app) sync() {
	a.update(func() {})
	a.refreshTray()
}
