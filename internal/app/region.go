package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

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
		label = i18n.T("区域：") + i18n.T(region)
	}
	routing := core.RegionRouting(region)
	a.routes.Put(label, routing)
	a.routes.Current = label
	a.logf(i18n.T("区域预置：%s（%d 条规则）"), i18n.T(region), len(routing.Rules))
	a.saveRoutes()
	go a.ensureRules(routing, false)
}

func (a *app) ensureRules(routing core.Routing, resume bool) {
	kind := a.kernelKind()
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
	if !resume {
		return
	}
	a.update(func() {
		if failed {
			a.fail(i18n.T("规则集没补齐，先不连了"))
			return
		}
		a.notice = ""
		a.logf("%s", i18n.T("规则集已就绪"))
		a.connect()
	})
}
