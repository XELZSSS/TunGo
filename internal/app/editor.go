package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tungo/internal/core"
	"tungo/internal/i18n"
	"tungo/internal/sub"
)

func (a *app) overlays(c *ui.Context) {
	a.nodeForm(c)
	a.groupForm(c)
	a.subForm(c)
	a.subEditForm(c)
	a.shortcutForm(c)
	a.portForm(c)
	a.policyForms(c)
	a.ruleBaseForm(c)
	a.updateDialog(c)
}

func (a *app) nodeForm(c *ui.Context) {
	if !a.editing {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.editing, func() {
		ui.Column(c).Gap(gapBase).Width(560).Height(560).Children(func() {
			heading(c, i18n.T("节点"))
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(gapBase).Children(func() {
					ui.TextInput(c, &a.draft.Name).Label(i18n.T("备注")).Placeholder(i18n.T("备注")).Grow(1)
					ui.Row(c).Gap(gapBase).Children(func() {
						ui.Select(c, &a.draft.Protocol, core.Protocols).Label(i18n.T("协议")).Width(170)
						ui.Select(c, &a.draft.Network, append([]string{"tcp"}, core.Networks[1:]...)).Label(i18n.T("传输")).Width(150)
						ui.Select(c, &a.draft.TLS, core.Securities).Label("TLS").Width(130)
					})
					ui.Row(c).Gap(gapBase).Children(func() {
						ui.TextInput(c, &a.draft.Server).Label(i18n.T("地址")).Placeholder("example.com").Grow(1)
						ui.TextInput(c, &a.draftPort).Label(i18n.T("端口")).Placeholder("443").Width(110)
					})

					switch a.draft.Protocol {
					case "vmess", "vless", "tuic":
						ui.TextInput(c, &a.draft.UUID).Label("UUID").Placeholder("UUID").Grow(1)
					case "shadowsocks":
						ui.Row(c).Gap(gapBase).Children(func() {
							ui.TextInput(c, &a.draft.Method).Label(i18n.T("加密方式")).Placeholder("aes-256-gcm").Grow(1)
							ui.TextInput(c, &a.draft.Password).Label(i18n.T("密码")).Password().Grow(1)
						})
						ui.Row(c).Gap(gapBase).Children(func() {
							ui.TextInput(c, &a.draft.Plugin).Label(i18n.T("插件")).Placeholder("obfs-local").Grow(1)
							ui.TextInput(c, &a.draft.PluginOpts).Label(i18n.T("插件参数")).Placeholder("obfs=http;obfs-host=example.com").Grow(1)
						})
					case "socks", "http":
						ui.Row(c).Gap(gapBase).Children(func() {
							ui.TextInput(c, &a.draft.Username).Label(i18n.T("用户名")).Grow(1)
							ui.TextInput(c, &a.draft.Password).Label(i18n.T("密码")).Password().Grow(1)
						})
					}
					switch a.draft.Protocol {
					case "trojan", "hysteria2", "anytls":
						ui.TextInput(c, &a.draft.Password).Label(i18n.T("密码")).Password().Grow(1)
					case "tuic":
						ui.TextInput(c, &a.draft.Password).Label(i18n.T("密码")).Password().Grow(1)
					}

					if a.draft.Protocol == "vmess" {
						ui.Row(c).Gap(gapBase).Children(func() {
							ui.TextInput(c, &a.draft.Security).Label(i18n.T("加密")).Placeholder("auto").Grow(1)
							ui.TextInput(c, &a.alterID).Label("alterId").Placeholder("0").Width(110)
						})
					}
					if a.draft.Protocol == "vless" {
						ui.TextInput(c, &a.draft.Flow).Label("flow").Placeholder("xtls-rprx-vision").Grow(1)
					}
					if a.draft.Protocol == "hysteria2" {
						ui.Row(c).Gap(gapBase).Children(func() {
							ui.TextInput(c, &a.draft.Obfs).Label(i18n.T("混淆")).Placeholder("salamander").Grow(1)
							ui.TextInput(c, &a.draft.ObfsPassword).Label(i18n.T("混淆密码")).Grow(1)
						})
					}
					if a.draft.Protocol == "tuic" {
						ui.TextInput(c, &a.draft.Congestion).Label(i18n.T("拥塞控制")).Placeholder("bbr").Grow(1)
					}
					if core.Muxable(a.draft.Protocol) {
						ui.TextInput(c, &a.muxText).Label(i18n.T("Mux 并发（0 关闭）")).Placeholder("8").Width(200)
					}

					if a.draft.Network != "tcp" {
						ui.Row(c).Gap(gapBase).Children(func() {
							ui.TextInput(c, &a.draft.Path).Label(i18n.T("路径 / serviceName")).Placeholder("/path").Grow(1)
							ui.TextInput(c, &a.draft.Host).Label("Host").Placeholder("example.com").Grow(1)
						})
					}

					if a.draft.TLS != "none" {
						ui.Row(c).Gap(gapBase).Children(func() {
							ui.TextInput(c, &a.draft.ServerName).Label("SNI").Placeholder("example.com").Grow(1)
							ui.TextInput(c, &a.alpn).Label("ALPN").Placeholder("h2,http/1.1").Grow(1)
						})
						ui.Row(c).Gap(gapBase).Children(func() {
							ui.TextInput(c, &a.draft.Fingerprint).Label(i18n.T("指纹")).Placeholder("chrome").Grow(1)
							ui.Toggle(c, &a.draft.Insecure, i18n.T("跳过证书校验"))
						})
						ui.Row(c).Gap(gapBase).Children(func() {
							ui.Toggle(c, &a.draft.Fragment, i18n.T("TLS 分片（sing-box）"))
							ui.Toggle(c, &a.draft.RecordFragment, i18n.T("记录分片（sing-box）"))
						})
					}
					if a.draft.TLS == "reality" {
						ui.Row(c).Gap(gapBase).Children(func() {
							ui.TextInput(c, &a.draft.PublicKey).Label("publicKey").Grow(1)
							ui.TextInput(c, &a.draft.ShortID).Label("shortId").Grow(1)
						})
					}
					ui.Divider(c)
					ui.Select(c, &a.detourName, a.detourOptions()).Label(i18n.T("前置代理（先经过它再连这个节点）")).Grow(1)
					muted(c, t, i18n.T("自定义出站：填了就把这段 JSON 原样交给内核，上面的协议字段不再使用。"))
					ui.TextArea(c, &a.rawText).Height(96)
					if !a.support() {
						ui.Text(c, i18n.T("当前内核不支持这个协议，请改用 sing-box")).FontSize(fontS).TextColor(t.Danger)
					}
				})
			})
			ui.Row(c).Gap(gapBase).Justify(ui.End).Children(func() {
				if ui.Button(c, i18n.T("取消")).Clicked() {
					a.editing = false
				}
				if ui.PrimaryButton(c, i18n.T("保存")).Clicked() {
					a.commit()
				}
			})
		})
	})
}

func (a *app) detourOptions() []string {
	names := []string{i18n.T("（不前置）")}
	for _, n := range a.nodes {
		if sameNode(n, a.draftWas) || n.Direct() {
			continue
		}
		names = append(names, n.Name)
	}
	return names
}

func (a *app) support() bool {
	draft := a.draft
	draft.Detour = a.detourOf()
	return core.Carries(core.PickKind(core.Kind(a.kind)), draft)
}

func (a *app) detourOf() *core.Node {
	for _, n := range a.nodes {
		if sameNode(n, a.draftWas) || n.Direct() || n.Name != a.detourName {
			continue
		}
		n.Detour = nil
		return &n
	}
	return nil
}

func (a *app) commit() {
	port, err := strconv.Atoi(strings.TrimSpace(a.draftPort))
	if err != nil || port <= 0 || port > 65535 {
		a.fail(i18n.T("端口要是一个 1 到 65535 的数字"))
		return
	}
	a.draft.Port = port
	if id, err := strconv.Atoi(strings.TrimSpace(a.alterID)); err == nil {
		a.draft.AlterID = id
	} else {
		a.draft.AlterID = 0
	}
	if mux, err := strconv.Atoi(strings.TrimSpace(a.muxText)); err == nil {
		a.draft.Mux = mux
	} else {
		a.draft.Mux = 0
	}
	a.draft.ALPN = core.ParseList(a.alpn)
	a.draft.Detour = a.detourOf()
	a.draft.Raw = strings.TrimSpace(a.rawText)
	if a.draft.Protocol == "direct" {
		direct := directNode()
		if a.draft.Name != "" {
			direct.Name = a.draft.Name
		}
		direct.Sub = a.draft.Sub
		a.draft = direct
	}
	at := a.indexOfNode(a.draftWas)
	if a.draft.Name != "" {
		for i, node := range a.nodes {
			if i != at && node.Name == a.draft.Name {
				a.fail(i18n.T("已有同名节点"))
				return
			}
		}
	}
	if at >= 0 {
		a.nodes[at] = a.draft
	} else {
		a.appendNodes(a.draft)
		a.chosen = len(a.nodes) - 1
	}
	a.nodesChanged()
	a.save()
	a.logf(i18n.T("已保存节点：%s（%s）"), a.draft.Name, a.draft.Summary())
	a.editing = false
	a.notice = ""
	if a.connected() {
		a.connect()
	}
}

func (a *app) subForm(c *ui.Context) {
	if !a.editingSub {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.editingSub, func() {
		ui.Column(c).Gap(gapLoose).Width(660).Height(460).Children(func() {
			heading(c, i18n.T("订阅"))
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(gapTight).Children(func() {
					if len(a.subs.Items) == 0 {
						muted(c, t, i18n.T("还没有订阅"))
					}
					for i := range a.subs.Items {
						at := i
						item := a.subs.Items[i]
						label := fmt.Sprintf(i18n.T("%s（%s）"), item.Name, intervalName(item.Interval))
						if traffic := trafficName(item); traffic != "" {
							label += " · " + traffic
						}
						ui.Row(c).Gap(gapBase).AlignItems(ui.Center).Children(func() {
							ui.Text(c, label).
								FontSize(fontS).Grow(1).TextColor(t.Text)
							if ui.Button(c, i18n.T("使用")).
								Disabled(a.subs.Running() != nil && a.subs.Running().Name == item.Name).
								Clicked() {
								a.selectSub(item.Name)
							}
							if ui.Button(c, i18n.T("编辑")).Clicked() {
								a.editSub(at)
							}
							if ui.Button(c, i18n.T("更新")).Disabled(a.subBusy).Clicked() {
								a.refresh(item.Name)
							}
							if ui.Button(c, i18n.T("删除")).Clicked() {
								a.after(func() { a.removeSub(item.Name) })
							}
						})
					}
					if ui.Button(c, i18n.T("新建订阅")).Clicked() {
						a.editSub(-1)
					}
				})
			})
			if a.subNote != "" {
				muted(c, t, a.subNote)
			}
			ui.Row(c).Gap(gapBase).Justify(ui.End).Children(func() {
				if ui.Button(c, i18n.T("关闭")).Clicked() {
					a.editingSub = false
				}
			})
		})
	})
}

func (a *app) subEditForm(c *ui.Context) {
	if !a.editingSubItem {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.editingSubItem, func() {
		ui.Column(c).Gap(gapLoose).Width(560).Children(func() {
			heading(c, i18n.T("订阅"))
			ui.TextInput(c, &a.subDraft.Name).Label(i18n.T("名称")).Placeholder(i18n.T("如：机场 A")).Grow(1)
			ui.TextInput(c, &a.subDraft.URL).Label(i18n.T("订阅地址")).Placeholder("https://…").Grow(1)
			ui.TextInput(c, &a.subInterval).Label(i18n.T("自动更新间隔（分钟，0 表示不自动更新）")).Width(300)
			ui.TextInput(c, &a.subDraft.UA).Label(i18n.T("更新时的 User-Agent（留空用 TunGo）")).Placeholder("ClashMetaForAndroid/2.11.1").Grow(1)
			muted(c, t, i18n.T("更新时只替换这个订阅带来的节点，自己添加的和别的订阅的不动。"))
			ui.Row(c).Gap(gapBase).Justify(ui.End).Children(func() {
				if ui.Button(c, i18n.T("取消")).Clicked() {
					a.editingSubItem = false
				}
				if ui.PrimaryButton(c, i18n.T("保存并更新")).Clicked() {
					a.commitSub()
				}
			})
		})
	})
}

func (a *app) editSub(at int) {
	a.subAt = at
	a.subDraft = sub.Item{}
	if at >= 0 && at < len(a.subs.Items) {
		a.subDraft = a.subs.Items[at]
	}
	a.subInterval = fmt.Sprint(a.subDraft.Interval)
	a.editingSub = false
	a.editingSubItem = true
}

func (a *app) commitSub() {
	name := strings.TrimSpace(a.subDraft.Name)
	url := strings.TrimSpace(a.subDraft.URL)
	if name == "" || url == "" {
		a.fail(i18n.T("订阅要有名称和地址"))
		return
	}
	interval, err := strconv.Atoi(strings.TrimSpace(a.subInterval))
	if err != nil || interval < 0 {
		interval = 0
	}
	item := a.subDraft
	item.Name, item.URL, item.Interval = name, url, interval
	item.UA = strings.TrimSpace(item.UA)
	if old := a.subName(a.subAt); old != "" && old != name {
		a.subs.Remove(old)
		for i := range a.nodes {
			if a.nodes[i].Sub == old {
				a.nodes[i].Sub = name
			}
		}
		a.nodesChanged()
		a.save()
	}
	a.subs.Put(item)
	a.subs.Current = name
	a.editingSubItem = false
	a.editingSub = true
	a.logf(i18n.T("订阅：%s"), name)
	a.saveSubs()
	a.refresh(name)
}

func (a *app) selectSub(name string) {
	if a.subs.Item(name) == nil {
		return
	}
	a.subs.Current = name
	a.logf(i18n.T("订阅：%s"), name)
	a.saveSubs()
}

func (a *app) removeSub(name string) {
	if a.subs.Item(name) == nil {
		return
	}
	was := a.node()
	dropped := was.Sub == name
	kept := make([]core.Node, 0, len(a.nodes))
	removed := 0
	for _, node := range a.nodes {
		if node.Sub != name {
			kept = append(kept, node)
			continue
		}
		removed++
	}
	a.setNodes(kept)
	a.chosen = 0
	if at := a.indexOfNode(was); at >= 0 {
		a.chosen = at
	}
	a.subs.Remove(name)
	a.save()
	a.saveSubs()
	a.logf(i18n.T("已删除订阅：%s（%d 个节点）"), name, removed)
	if dropped && a.connected() {
		a.connect()
	}
}

func intervalName(minutes int) string {
	if minutes <= 0 {
		return i18n.T("不自动更新")
	}
	return fmt.Sprintf(i18n.T("每 %d 分钟"), minutes)
}

func trafficName(item sub.Item) string {
	parts := []string{}
	if item.Total > 0 {
		parts = append(parts, fmt.Sprintf(i18n.T("已用 %s / %s"), sizeName(item.Used), sizeName(item.Total)))
	}
	if item.Expire > 0 {
		parts = append(parts, fmt.Sprintf(i18n.T("到期 %s"), time.Unix(item.Expire, 0).Format("2006-01-02")))
	}
	return strings.Join(parts, " · ")
}

func sizeName(bytes int64) string {
	return fmt.Sprintf("%.1f GB", float64(bytes)/(1<<30))
}

func (a *app) reveal(path string) {
	if path == "" {
		return
	}
	dir := path
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		dir = filepath.Dir(path)
	}
	if err := mygo.Shell.OpenPath(dir); err != nil {
		a.logf(i18n.T("打不开目录：%s"), err)
	}
}

func (a *app) rebootAsAdmin() {
	exe, err := os.Executable()
	if err != nil {
		a.fail(err.Error())
		return
	}
	if runtime.GOOS != "windows" {
		a.hint(i18n.T("请用 sudo 重新运行"))
		return
	}
	script := fmt.Sprintf("Wait-Process -Id %d -ErrorAction SilentlyContinue; Start-Process -FilePath '%s' -Verb RunAs",
		os.Getpid(), strings.ReplaceAll(exe, "'", "''"))
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	core.Hide(cmd)
	if err := cmd.Start(); err != nil {
		a.fail(err.Error())
		a.logf(i18n.T("提不起权限：%s"), err)
		return
	}
	a.logf("%s", i18n.T("已请求以管理员身份重启"))
	a.requestQuit()
}

func (a *app) subName(at int) string {
	if at < 0 || at >= len(a.subs.Items) {
		return ""
	}
	return a.subs.Items[at].Name
}
