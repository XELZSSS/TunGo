package app

import (
	"net/http"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tungo/internal/core"
	"tungo/internal/proxy"
	"tungo/internal/sub"
)

// shell：窗口、托盘、主题，以及跨渲染周期传递的剪贴板与延迟任务。
type shell struct {
	win       *mygo.Window
	tray      *mygo.Tray
	quitting  bool
	maximized bool
	themeName string
	writeNext string
	queued    []func()
}

// lists：节点表与连接表的选择、滚动和分栏状态。
type lists struct {
	list     ui.ListState
	connList ui.ListState
	conns    []core.Conn
	panePct  float32
	tab      int
	group    string
	shown    []int
	row      int
	rowAt    int
}

// servers：节点清单与当前选中的内核、端口、延迟和测速结果。
type servers struct {
	nodes   []core.Node
	nodesAt string
	chosen  int
	kind    string
	port    string
	ms      []int
	speed   []float64
}

// feedback：提示条、忙碌标记与日志面板。
type feedback struct {
	notice     string
	noticeKind noticeKind
	busy       bool
	tries      int
	logs       []string
	logScroll  ui.ScrollState
}

// subscriptions：订阅清单与订阅表单。
type subscriptions struct {
	subs           sub.State
	subNote        string
	subBusy        bool
	subDraft       sub.Item
	subAt          int
	subFormAt      string
	editingSub     bool
	editingSubItem bool
	subInterval    string
}

// kernels：内核二进制的安装、版本与更新状态。
type kernels struct {
	installing string
	installAt  string
	version    string
	versionFor core.Kind
	latest     string
	skipped    map[string]string
	checking   bool
	have       map[core.Kind]bool
	updating   bool
}

// hotkeys：全局热键的绑定与编辑。
type hotkeys struct {
	shortcuts        map[string]string
	shortcutShowKey  string
	shortcutConnKey  string
	shortcutProxyKey string
	editingShortcuts bool
}

// forms：节点与策略组表单的草稿。
type forms struct {
	editing       bool
	draft         core.Node
	draftPort     string
	draftWas      core.Node
	detourName    string
	rawText       string
	alterID       string
	alpn          string
	editingGroup  bool
	groupWas      core.Node
	groupName     string
	groupStrategy string
	groupPick     map[string]bool
	groupNote     string
}

// routing：路由配置、规则与 DNS 的编辑状态。
type routing struct {
	routes           core.RoutingSet
	editingRoute     bool
	editingRouteName bool
	routeName        string
	rules            []core.Rule
	routeFinalAt     int
	routeStrategy    string
	editingRule      bool
	ruleAt           int
	ruleDraft        core.Rule
	ruleDomains      string
	ruleIPs          string
	rulePort         string
	ruleNetwork      string
	ruleProtocols    string
	editingDNS       bool
	dnsRemote        string
	dnsDirect        string
	dnsBootstrap     string
	dnsStrategy      string
	dnsBlockAAAA     bool
	dnsHosts         string
	dnsSplit         string
}

// link：连接本身——本地代理、内核进程、PAC 服务和流量统计。
type link struct {
	local   *proxy.Server
	kernel  *core.Kernel
	stop    chan struct{}
	pac     *http.Server
	proxy   string
	tun     bool
	applied bool
	up      int64
	down    int64
}
