package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

const (
	rulesNoConnect = iota
	rulesConnect
	rulesReconnect
)

func rulesIntent(connected bool) int {
	if connected {
		return rulesReconnect
	}
	return rulesNoConnect
}

func (a *app) regionMenu(_ *ui.Context, m *ui.Menu) {
	for _, region := range core.Regions {
		name := region
		if m.Item(i18n.T(name)).Chosen() {
			a.applyRegion(name)
		}
	}
}

func (a *app) applyRegion(region string) {
	label := region
	if region != core.Regions[0] {
		label = "区域：" + region
	}
	routing := core.RegionRouting(region)
	a.routes.Put(label, routing)
	a.routes.Current = label
	a.logf(i18n.T("区域预置：%s（%d 条规则）"), i18n.T(region), len(routing.Rules))
	if err := core.SaveRoutingSet(a.routes); err != nil {
		a.fail(err.Error())
		a.logf("%s", err)
		return
	}
	go a.ensureRules(routing, a.kernelKind(), rulesIntent(a.connected()))
}

func (a *app) ensureRules(routing core.Routing, kind core.Kind, want int) {
	failed := false
	for _, entry := range core.MissingRuleSets(routing, kind) {
		if err := core.EnsureRuleSet(entry, kind); err != nil {
			failed = true
			a.update(func() {
				a.fail(fmt.Sprintf(i18n.T("规则集 %s 取不到：%s"), entry, err))
				a.logf("%s", err)
			})
			continue
		}
		a.update(func() { a.logf(i18n.T("规则集已就绪：%s"), entry) })
	}
	if want == rulesNoConnect {
		return
	}
	a.update(func() {
		if failed {
			a.fail(i18n.T("规则集没补齐，先不连了（可在「路由规则」里选「忽略缺失的规则集并连接」）"))
			return
		}
		if want == rulesReconnect && !a.connected() {
			return
		}
		a.notice = ""
		a.logf("%s", i18n.T("规则集已就绪"))
		a.connect()
	})
}
