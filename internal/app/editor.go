package app

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

func (a *app) overlays(c *ui.Context) {
	a.nodeForm(c)
	a.groupForm(c)
	a.subForm(c)
	a.subEditForm(c)
	a.shortcutForm(c)
	a.portForm(c)
	a.policyForms(c)
	a.mirrorForm(c)
	a.restoreForm(c)
	a.updateDialog(c)
}

func (a *app) nodeForm(c *ui.Context) {
	if !a.editing {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.editing, func() {
		ui.Column(c).Gap(gapBase).Width(dialogBase).Height(dialogBody).Children(func() {
			heading(c, i18n.T("节点"))
			ui.Scroll(c).Grow(1).Padding(0, scrollGutter, 0, 0).Children(func() {
				ui.Column(c).Gap(gapBase).PaddingY(padYDense).Children(func() {
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
						ui.Row(c).Gap(gapBase).Children(func() {
							ui.TextInput(c, &a.draft.ECH).Label(i18n.T("ECH 配置")).Placeholder("-----BEGIN ECH CONFIGS-----").Grow(1)
							ui.TextInput(c, &a.draft.Pinned).Label(i18n.T("证书 pin（SHA256，仅 xray）")).Grow(1)
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
	if err := core.Unsupported(core.PickKind(core.Kind(a.kind)), a.draft); err != nil {
		a.fail(err.Error())
		return
	}
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
	if a.connected() {
		a.connect()
	}
}
