package app

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"net"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"tungo/internal/core"
	"tungo/internal/i18n"
	"tungo/internal/sub"
)

func (a *app) choose(i int) {
	if i == a.chosen || i < 0 || i >= len(a.nodes) {
		return
	}
	a.chosen = i

	a.know(a.kernelKind())
	if a.connected() {
		a.connect()
	}
}

func (a *app) refresh(name string) { a.pullSubs([]string{name}, false) }

func (a *app) refreshProxied(name string) { a.pullSubs([]string{name}, true) }

func (a *app) saveSubs() {
	if err := sub.Save(a.subs); err != nil {
		a.fail(err.Error())
		a.logf("%s", err)
	}
}

func (a *app) autoUpdate() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	time.Sleep(time.Duration(rand.IntN(10)) * time.Second)
	for range ticker.C {
		a.update(func() {
			if a.subBusy {
				return
			}
			var due []string
			for _, item := range a.subs.Items {
				if item.Interval > 0 && time.Since(item.Updated) >= time.Duration(item.Interval)*time.Minute {
					due = append(due, item.Name)
				}
			}
			a.pullSubs(due, false)
		})
	}
}

func (a *app) probe() {
	if len(a.nodes) == 0 {
		return
	}
	a.logf("%s", i18n.T("开始测速"))
	nodes := slices.Clone(a.nodes)
	jobs := make(chan core.Node)
	for range probeWorkers {
		go func() {
			for node := range jobs {
				a.pingAt(node)
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, node := range nodes {
			jobs <- node
		}
	}()
}

func (a *app) pingNode(i int) {
	if i < 0 || i >= len(a.nodes) {
		return
	}
	go a.pingAt(a.nodes[i])
}

func nodeAddr(node core.Node) string {
	if node.Direct() {
		return probeAddr
	}
	return net.JoinHostPort(node.Server, fmt.Sprint(node.Port))
}

func (a *app) pingAt(node core.Node) {
	start := time.Now()
	ms := -1
	if conn, err := net.DialTimeout("tcp", nodeAddr(node), 3*time.Second); err == nil {
		conn.Close()
		ms = int(time.Since(start).Milliseconds())
	}
	a.update(func() {
		if at := a.indexOfNode(node); at >= 0 {
			a.ms[at] = ms
			a.rowsDirty = true
		}
	})
}

func (a *app) groupNames() []string {
	if a.groupsAt == a.nodesVersion && a.groups != nil {
		return a.groups
	}
	names := []string{i18n.T("全部")}
	seen := map[string]bool{}
	for _, node := range a.nodes {
		name := node.Sub
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	a.groups, a.groupsAt = names, a.nodesVersion
	return names
}

func (a *app) syncRows() {
	column, desc := "", false
	if order := a.list.Sort; order != nil {
		column, desc = order.Column, order.Descending
	}
	if !a.rowsDirty && a.group == a.rowsGroup && column == a.rowsColumn && desc == a.rowsDesc {
		return
	}
	a.rowsDirty = false
	a.rowsGroup, a.rowsColumn, a.rowsDesc = a.group, column, desc
	shown := a.shown[:0]
	for i, node := range a.nodes {
		if a.group != "" && node.Sub != a.group {
			continue
		}
		shown = append(shown, i)
	}
	a.shown = shown
	a.sortRows()
}

func (a *app) rowOf(node int) int {
	for row, at := range a.shown {
		if at == node {
			return row
		}
	}
	return -1
}

func (a *app) sortRows() {
	order := a.list.Sort
	if order == nil || order.Column == "" || len(a.shown) < 2 {
		return
	}
	slices.SortStableFunc(a.shown, func(one, other int) int {
		var by int
		if order.Column == "name" {
			by = strings.Compare(strings.ToLower(a.nodes[one].Name), strings.ToLower(a.nodes[other].Name))
		} else {
			x, known := a.sortKey(one, order.Column)
			y, otherKnown := a.sortKey(other, order.Column)
			if !known || !otherKnown {
				switch {
				case known:
					return -1
				case otherKnown:
					return 1
				}
				return 0
			}
			by = cmp.Compare(x, y)
		}
		if order.Descending {
			return -by
		}
		return by
	})
}

func (a *app) sortKey(at int, column string) (int64, bool) {
	switch column {
	case "delay":
		if a.ms[at] > 0 {
			return int64(a.ms[at]), true
		}
	case "speed":
		if a.speed[at] > 0 {
			return int64(a.speed[at] * 1000), true
		}
	}
	return 0, false
}
func (a *app) indexOfNode(was core.Node) int {
	if was.Name == "" {
		return -1
	}
	for i, node := range a.nodes {
		if sameNode(node, was) {
			return i
		}
	}
	return -1
}
func transport(node core.Node) string {
	parts := []string{}
	if node.Network != "" {
		parts = append(parts, node.Network)
	}
	if node.TLS != "" {
		parts = append(parts, node.TLS)
	}
	if len(parts) == 0 {
		return "tcp"
	}
	return strings.Join(parts, " · ")
}

func cipher(node core.Node) string {
	switch node.Protocol {
	case "shadowsocks":
		return node.Method
	case "vmess":
		if node.Security == "" || node.Security == "auto" {
			return "auto"
		}
		return node.Security
	case "vless":
		if node.Flow == "" {
			return "—"
		}
		return node.Flow
	case "hysteria2", "tuic":
		if node.Congestion == "" {
			return "bbr"
		}
		return node.Congestion
	}
	return "—"
}

func security(node core.Node) string {
	if node.Direct() {
		return "—"
	}
	var parts []string
	switch node.TLS {
	case "":
		parts = append(parts, i18n.T("无"))
	case "tls":
		parts = append(parts, "TLS")
		if node.Fingerprint != "" {
			parts = append(parts, node.Fingerprint)
		}
	case "reality":
		parts = append(parts, "REALITY")
	}
	if node.Detour != nil {
		parts = append(parts, "→ "+node.Detour.Name)
	}
	return strings.Join(parts, " · ")
}

func sameNode(one, other core.Node) bool {
	if one.Name != other.Name || one.Protocol != other.Protocol || one.Server != other.Server || one.Port != other.Port {
		return false
	}
	return reflect.DeepEqual(plainNode(one), plainNode(other))
}

func plainNode(node core.Node) core.Node {
	node.Members = nil
	children := slices.Clone(node.Children)
	slices.Sort(children)
	node.Children = children
	if node.Detour != nil {
		detour := plainNode(*node.Detour)
		node.Detour = &detour
	}
	return node
}

func (a *app) refreshAll() {
	names := make([]string, 0, len(a.subs.Items))
	for _, item := range a.subs.Items {
		names = append(names, item.Name)
	}
	a.pullSubs(names, false)
}

func (a *app) pullSubs(names []string, proxied bool) {
	if a.subBusy {
		return
	}
	type job struct{ name, url string }
	jobs := make([]job, 0, len(names))
	for _, name := range names {
		if item := a.subs.Item(name); item != nil {
			jobs = append(jobs, job{item.Name, item.URL})
		}
	}
	if len(jobs) == 0 {
		return
	}
	addr := a.localAddr()
	if proxied && addr == "" {
		return
	}
	a.subBusy = true
	if proxied {
		a.subNote = i18n.T("正在通过代理更新订阅")
		a.logf("%s", a.subNote)
	} else {
		a.subNote = i18n.T("正在更新订阅")
		if len(jobs) == 1 {
			a.event(i18n.T("正在更新订阅：%s"), jobs[0].name)
		} else {
			a.event("%s", a.subNote)
		}
	}
	go func() {
		results := make([]subResult, len(jobs))
		var wg sync.WaitGroup
		for i, one := range jobs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[i].name = one.name
				if proxied {
					results[i].nodes, results[i].err = sub.FetchProxied(one.url, addr)
					return
				}
				results[i].nodes, results[i].err = sub.Fetch(one.url)
			}()
		}
		wg.Wait()
		a.update(func() { a.applySubs(results, proxied) })
	}()
}

type subResult struct {
	name  string
	nodes []core.Node
	err   error
}

func (a *app) applySubs(results []subResult, proxied bool) {
	was := a.node()
	fresh := make(map[string][]core.Node, len(results))
	ok, failed := 0, 0
	var lastErr error
	for _, one := range results {
		if one.err != nil {
			failed++
			lastErr = one.err
			a.logf(i18n.T("订阅失败（%s）：%s"), one.name, one.err)
			continue
		}
		ok++
		for i := range one.nodes {
			one.nodes[i].Sub = one.name
		}
		fresh[one.name] = one.nodes
		if kept := a.subs.Item(one.name); kept != nil {
			kept.Updated = time.Now()
		}
		if proxied {
			a.logf(i18n.T("订阅更新（%s，经代理）：%d 个节点"), one.name, len(one.nodes))
		} else {
			a.logf(i18n.T("订阅更新（%s）：%d 个节点"), one.name, len(one.nodes))
		}
	}
	if ok > 0 {
		merged := make([]core.Node, 0, len(a.nodes))
		for _, node := range a.nodes {
			if _, replaced := fresh[node.Sub]; replaced {
				continue
			}
			merged = append(merged, node)
		}
		for _, one := range results {
			if one.err == nil {
				merged = append(merged, one.nodes...)
			}
		}
		a.setNodes(merged)
		a.chosen = 0
		if at := a.indexOfNode(was); at >= 0 {
			a.chosen = at
		}
		if a.connected() && !sameNode(a.node(), was) {
			a.connect()
		}
		a.save()
	}
	a.subBusy = false
	switch {
	case lastErr != nil && len(results) == 1:
		a.subNote = lastErr.Error()
	case len(results) == 1:
		a.subNote = fmt.Sprintf(i18n.T("已更新：%d 个节点"), len(results[0].nodes))
	default:
		a.subNote = fmt.Sprintf(i18n.T("订阅已更新：%d 个成功，%d 个失败"), ok, failed)
	}
	a.saveSubs()
}

const probeWorkers = 12
