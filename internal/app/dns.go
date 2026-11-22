package app

import (
	"reflect"
	"sort"
	"strings"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

func (a *app) saveDNS() {
	if !validHostLines(a.dnsHosts) {
		a.fail(i18n.T("Hosts 每行要写「名称 地址」"))
		return
	}
	if !validSplitLines(a.dnsSplit) {
		a.fail(i18n.T("按域分流每行要写「域名 服务器」"))
		return
	}
	dns := core.DNS{
		Remote:    a.dnsRemote,
		Direct:    a.dnsDirect,
		Bootstrap: a.dnsBootstrap,
		Strategy:  a.dnsStrategy,
		BlockAAAA: a.dnsBlockAAAA,
		Hosts:     parseHosts(a.dnsHosts),
		Split:     parseSplit(a.dnsSplit),
	}
	changed := !reflect.DeepEqual(a.dnsWas, dns)
	if err := core.SaveDNS(dns); err != nil {
		a.fail(err.Error())
		a.logf(i18n.T("保存 DNS 失败：%s"), err)
		return
	}
	a.dnsWas = dns
	a.logf(i18n.T("DNS 已保存：远程 %s，直连 %s，%s，屏蔽 AAAA %s，%d 条 Hosts"),
		dns.Remote, dns.Direct, dns.Strategy, onOff(dns.BlockAAAA), len(dns.Hosts))
	if changed && a.connected() {
		a.connect()
	}
}

func validHostLines(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if len(fields) != 2 {
			return false
		}
	}
	return true
}

func validSplitLines(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if len(fields) < 2 {
			return false
		}
	}
	return true
}

func onOff(on bool) string {
	if on {
		return i18n.T("开")
	}
	return i18n.T("关")
}

func finalName(tag string) string {
	if tag == "direct" {
		return i18n.T("直连")
	}
	return i18n.T("代理")
}

func parseHosts(text string) map[string]string {
	hosts := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		hosts[fields[0]] = fields[1]
	}
	if len(hosts) == 0 {
		return nil
	}
	return hosts
}

func hostLines(hosts map[string]string) []string {
	names := make([]string, 0, len(hosts))
	for name := range hosts {
		names = append(names, name+" "+hosts[name])
	}
	sort.Strings(names)
	return names
}

func parseSplit(text string) []core.SplitDNS {
	var out []core.SplitDNS
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		out = append(out, core.SplitDNS{
			Domains: core.ParseList(strings.Join(fields[:len(fields)-1], " ")),
			Server:  fields[len(fields)-1],
		})
	}
	return out
}

func splitLines(splits []core.SplitDNS) []string {
	out := make([]string, 0, len(splits))
	for _, one := range splits {
		out = append(out, strings.Join(one.Domains, ",")+" "+one.Server)
	}
	return out
}
