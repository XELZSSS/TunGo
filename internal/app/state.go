package app

import (
	"net/http"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tungo/internal/core"
	"tungo/internal/proxy"
	"tungo/internal/sub"
)

type shell struct {
	win       *mygo.Window
	tray      *mygo.Tray
	quitting  bool
	maximized bool
	themeName string
	writeNext string
	queued    []func()
}

type lists struct {
	list     ui.ListState
	connList ui.ListState
	conns    []core.Conn
	panePct  float32
	tab      int
	group    string
	groups   []string
	groupsAt groupsKey
	shown    []int
	rowsAt   rowsKey
	row      int
	rowAt    int
}

type servers struct {
	nodes   []core.Node
	nodesAt string
	chosen  int
	kind    string
	port    string
	ms      []int
	speed   []float64
	rev     int
}

type feedback struct {
	notice     string
	noticeKind noticeKind
	busy       bool
	tries      int
	logs       []string
	logScroll  ui.ScrollState
}

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

type hotkeys struct {
	shortcuts        map[string]string
	shortcutShowKey  string
	shortcutConnKey  string
	shortcutProxyKey string
	editingShortcuts bool
}

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

type link struct {
	local   *proxy.Server
	kernel  *core.Kernel
	stop    chan struct{}
	drain   chan struct{}
	pac     *http.Server
	proxy   string
	tun     bool
	applied bool
	up      int64
	down    int64
}
