package app

import (
	"fmt"
	"net"
	"sort"
	"strings"
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

func (a *app) refresh(name string) {
	item := a.subs.Item(name)
	if item == nil || a.subBusy {
		return
	}
	from := item.URL
	a.subBusy = true
	a.subNote = i18n.T("正在更新订阅")
	a.event(i18n.T("正在更新订阅：%s"), name)
	go func() {
		nodes, err := sub.Fetch(from)
		a.update(func() {
			a.subBusy = false
			if err != nil {
				a.subNote = err.Error()
				a.logf(i18n.T("订阅失败：%s"), err)
				return
			}
			a.replaceNodes(name, nodes)
			if kept := a.subs.Item(name); kept != nil {
				kept.Updated = time.Now()
			}
			a.subNote = fmt.Sprintf(i18n.T("已更新：%d 个节点"), len(nodes))
			a.logf(i18n.T("订阅更新（%s）：%d 个节点"), name, len(nodes))
			a.saveSubs()
		})
	}()
}

func (a *app) replaceNodes(name string, nodes []core.Node) {
	for i := range nodes {
		nodes[i].Sub = name
	}

	was := a.node()
	kept := make([]core.Node, 0, len(a.nodes)+len(nodes))
	for _, node := range a.nodes {
		if node.Sub != name {
			kept = append(kept, node)
		}
	}
	a.setNodes(append(kept, nodes...))
	a.chosen = 0
	for i, node := range a.nodes {
		if sameNode(node, was) {
			a.chosen = i
			break
		}
	}
	if a.connected() && !sameNode(a.node(), was) {
		a.connect()
	}
	a.save()
}

func (a *app) saveSubs() {
	if err := sub.Save(a.subs); err != nil {
		a.fail(err.Error())
		a.logf("%s", err)
	}
}

func (a *app) autoUpdate() {
	for range time.Tick(time.Minute) {
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
			a.updateSubs(due)
		})
	}
}

func (a *app) probe() {
	if len(a.nodes) == 0 {
		return
	}
	a.logf("%s", i18n.T("开始测速"))
	gate := make(chan struct{}, probeWorkers)
	for i, node := range a.nodes {
		addr := nodeAddr(node)
		go func(i int, addr string) {
			gate <- struct{}{}
			a.pingAt(i, addr)
			<-gate
		}(i, addr)
	}
}

func (a *app) pingNode(i int) {
	if i < 0 || i >= len(a.nodes) {
		return
	}
	go a.pingAt(i, nodeAddr(a.nodes[i]))
}

func nodeAddr(node core.Node) string {
	if node.Direct() {
		return probeAddr
	}
	return net.JoinHostPort(node.Server, fmt.Sprint(node.Port))
}

func (a *app) pingAt(i int, addr string) {
	start := time.Now()
	ms := -1
	if conn, err := net.DialTimeout("tcp", addr, 3*time.Second); err == nil {
		conn.Close()
		ms = int(time.Since(start).Milliseconds())
	}
	a.update(func() {
		if i < len(a.ms) {
			a.ms[i] = ms
			a.rev++
		}
	})
}

func indexOf(list []string, want string) int {
	for i, one := range list {
		if one == want {
			return i
		}
	}
	return -1
}

func (a *app) groupNames() []string {
	key := groupsKey{rev: a.rev, lang: i18n.Lang()}
	if a.groups != nil && key == a.groupsAt {
		return a.groups
	}
	names := []string{i18n.T("全部")}
	seen := map[string]bool{}
	for _, node := range a.nodes {
		name := groupName(node)
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	if a.group != "" && !seen[a.group] {
		a.group = ""
	}
	a.groupsAt, a.groups = key, names
	return names
}

func (a *app) syncRows() {
	key := rowsKey{rev: a.rev, group: a.group}
	if order := a.list.Sort; order != nil {
		key.column, key.descending = order.Column, order.Descending
	}
	if a.shown != nil && key == a.rowsAt {
		return
	}
	shown := a.shown[:0]
	for i, node := range a.nodes {
		if a.group != "" && groupName(node) != a.group {
			continue
		}
		shown = append(shown, i)
	}
	a.shown = shown
	a.sortRows()
	a.rowsAt = key
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
	sort.SliceStable(a.shown, func(i, j int) bool {
		one, other := a.shown[i], a.shown[j]
		var less, greater bool
		if order.Column == "name" {
			x, y := strings.ToLower(a.nodes[one].Name), strings.ToLower(a.nodes[other].Name)
			less, greater = x < y, x > y
		} else {
			x, known := a.sortKey(one, order.Column)
			y, otherKnown := a.sortKey(other, order.Column)
			if !known || !otherKnown {
				return known && !otherKnown
			}
			less, greater = x < y, x > y
		}
		if order.Descending {
			return greater
		}
		return less
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

func groupName(node core.Node) string {
	if node.Sub == "" {
		return i18n.T("手动")
	}
	return node.Sub
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
	return one.Name == other.Name && one.Server == other.Server &&
		one.Port == other.Port && one.UUID == other.UUID && one.Password == other.Password
}

func (a *app) refreshAll() {
	names := make([]string, 0, len(a.subs.Items))
	for _, item := range a.subs.Items {
		names = append(names, item.Name)
	}
	a.updateSubs(names)
}

func (a *app) updateSubs(names []string) {
	if a.subBusy || len(names) == 0 {
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
	a.subBusy = true
	a.event("%s", i18n.T("正在更新订阅"))
	go func() {
		defer a.update(func() { a.subBusy = false })
		ok, failed := 0, 0
		for _, one := range jobs {
			nodes, err := sub.Fetch(one.url)
			if err != nil {
				failed++
			} else {
				ok++
			}
			a.update(func() {

				if kept := a.subs.Item(one.name); kept != nil {
					kept.Updated = time.Now()
				}
				if err != nil {
					a.logf(i18n.T("订阅失败（%s）：%s"), one.name, err)
					return
				}
				a.replaceNodes(one.name, nodes)
				a.logf(i18n.T("订阅更新（%s）：%d 个节点"), one.name, len(nodes))
			})
		}
		a.update(func() {
			a.subNote = fmt.Sprintf(i18n.T("订阅已更新：%d 个成功，%d 个失败"), ok, failed)
			a.saveSubs()
		})
	}()
}

const probeWorkers = 12

type rowsKey struct {
	rev        int
	group      string
	column     string
	descending bool
}

type groupsKey struct {
	rev  int
	lang string
}
