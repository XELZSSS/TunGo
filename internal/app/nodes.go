package app

import (
	"cmp"
	"fmt"
	"net"
	"slices"
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

func (a *app) refresh(name string) { a.pullSubs([]string{name}, false) }

func (a *app) refreshProxied(name string) { a.pullSubs([]string{name}, true) }

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
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
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
		for _, node := range a.nodes {
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
		}
	})
}

func (a *app) groupNames() []string {
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
	return names
}

func (a *app) syncRows() {
	shown := a.shown[:0]
	for i, node := range a.nodes {
		if a.group != "" && groupName(node) != a.group {
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

func groupName(node core.Node) string {
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
	return one.Name == other.Name && one.Protocol == other.Protocol && one.Role == other.Role &&
		one.Server == other.Server && one.Port == other.Port &&
		one.UUID == other.UUID && one.Password == other.Password
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
		ok, failed := 0, 0
		var lastErr error
		for _, one := range jobs {
			var nodes []core.Node
			var err error
			if proxied {
				nodes, err = sub.FetchProxied(one.url, addr)
			} else {
				nodes, err = sub.Fetch(one.url)
			}
			if err != nil {
				failed++
				lastErr = err
				a.update(func() { a.logf(i18n.T("订阅失败（%s）：%s"), one.name, err) })
				continue
			}
			ok++
			a.update(func() {
				a.replaceNodes(one.name, nodes)
				if kept := a.subs.Item(one.name); kept != nil {
					kept.Updated = time.Now()
				}
				if proxied {
					a.logf(i18n.T("订阅更新（%s，经代理）：%d 个节点"), one.name, len(nodes))
				} else {
					a.logf(i18n.T("订阅更新（%s）：%d 个节点"), one.name, len(nodes))
				}
			})
		}
		a.update(func() {
			a.subBusy = false
			switch {
			case lastErr != nil && len(jobs) == 1:
				a.subNote = lastErr.Error()
			case len(jobs) == 1:
				a.subNote = fmt.Sprintf(i18n.T("已更新：%d 个节点"), ok)
			default:
				a.subNote = fmt.Sprintf(i18n.T("订阅已更新：%d 个成功，%d 个失败"), ok, failed)
			}
			a.saveSubs()
		})
	}()
}

const probeWorkers = 12
