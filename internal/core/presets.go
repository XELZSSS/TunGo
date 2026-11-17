package core

import "strings"

var Routings = []string{"绕过大陆", "黑名单", "全局"}

func DefaultRouting() Routing { return PresetRouting(Routings[0]) }

func PresetRouting(name string) Routing {
	routing := Routing{Strategy: "IPIfNonMatch", Rules: baseRules()}
	switch name {
	case "黑名单":
		routing.Rules = blackRules()
		routing.Final = "direct"
	case "绕过大陆":
		routing.Rules = mainlandRules()
	}
	return routing
}

func baseRules() []Rule {
	return []Rule{
		{Remark: "阻断 UDP 443", Out: "block", Port: "443", Network: "udp"},
		{Remark: "绕过局域网地址", Out: "direct", IP: []string{"geoip:private"}},
		{Remark: "绕过局域网域名", Out: "direct", Domain: []string{"geosite:private"}},
	}
}

func mainlandRules() []Rule {
	return append(baseRules(),
		Rule{Remark: "绕过中国公共 DNS 地址", Out: "direct", IP: chinaDNS},
		Rule{Remark: "绕过中国公共 DNS 域名", Out: "direct", Domain: []string{
			"domain:alidns.com", "domain:doh.pub", "domain:dot.pub", "domain:360.cn", "domain:onedns.net",
		}},
		Rule{Remark: "绕过中国地址", Out: "direct", IP: []string{"geoip:cn"}},
		Rule{Remark: "绕过中国域名", Out: "direct", Domain: []string{"geosite:cn"}},
	)
}

func blackRules() []Rule {
	return []Rule{
		{Remark: "绕过 bittorrent", Out: "direct", Protocol: []string{"bittorrent"}},
		{Remark: "代理 api.ip.sb", Out: "proxy", Domain: []string{"domain:api.ip.sb"}},
		{Remark: "阻断 UDP 443", Out: "block", Port: "443", Network: "udp"},
		{Remark: "绕过局域网地址", Out: "direct", IP: []string{"geoip:private"}},
		{Remark: "绕过局域网域名", Out: "direct", Domain: []string{"geosite:private"}},
		{Remark: "代理海外公共 DNS 地址", Out: "proxy", IP: foreignDNS},
		{Remark: "代理海外公共 DNS 域名", Out: "proxy", Domain: []string{
			"domain:cloudflare-dns.com", "domain:one.one.one.one", "domain:dns.google",
			"domain:adguard-dns.com", "domain:opendns.com", "domain:umbrella.com",
			"domain:quad9.net", "domain:yandex.net",
		}},

		{Remark: "代理海外域名", Out: "proxy", Domain: []string{"geosite:geolocation-!cn"}},
	}
}

var chinaDNS = []string{
	"223.5.5.5", "223.6.6.6", "2400:3200::1", "2400:3200:baba::1",
	"119.29.29.29", "1.12.12.12", "120.53.53.53", "2402:4e00::", "2402:4e00:1::",
	"180.76.76.76", "2400:da00::6666",
	"114.114.114.114", "114.114.115.115", "114.114.114.119", "114.114.115.119",
	"114.114.114.110", "114.114.115.110",
	"180.184.1.1", "180.184.2.2", "101.226.4.6", "218.30.118.6",
	"123.125.81.6", "140.207.198.6",
	"1.2.4.8", "210.2.4.8", "52.80.66.66", "117.50.22.22",
	"2400:7fc0:849e:200::4", "2404:c2c0:85d8:901::4",
	"117.50.10.10", "52.80.52.52", "2400:7fc0:849e:200::8", "2404:c2c0:85d8:901::8",
	"117.50.60.30", "52.80.60.30",
}

var foreignDNS = []string{
	"1.1.1.1", "1.0.0.1", "2606:4700:4700::1111", "2606:4700:4700::1001",
	"1.1.1.2", "1.0.0.2", "2606:4700:4700::1112", "2606:4700:4700::1002",
	"1.1.1.3", "1.0.0.3", "2606:4700:4700::1113", "2606:4700:4700::1003",
	"8.8.8.8", "8.8.4.4", "2001:4860:4860::8888", "2001:4860:4860::8844",
	"94.140.14.14", "94.140.15.15", "2a10:50c0::ad1:ff", "2a10:50c0::ad2:ff",
	"94.140.14.15", "94.140.15.16", "2a10:50c0::bad1:ff", "2a10:50c0::bad2:ff",
	"94.140.14.140", "94.140.14.141", "2a10:50c0::1:ff", "2a10:50c0::2:ff",
	"208.67.222.222", "208.67.220.220", "2620:119:35::35", "2620:119:53::53",
	"208.67.222.123", "208.67.220.123", "2620:119:35::123", "2620:119:53::123",
	"9.9.9.9", "149.112.112.112", "2620:fe::9", "2620:fe::fe",
	"9.9.9.11", "149.112.112.11", "2620:fe::11", "2620:fe::fe:11",
	"9.9.9.10", "149.112.112.10", "2620:fe::10", "2620:fe::fe:10",
	"77.88.8.8", "77.88.8.1", "2a02:6b8::feed:0ff", "2a02:6b8:0:1::feed:0ff",
	"77.88.8.88", "77.88.8.2", "2a02:6b8::feed:bad", "2a02:6b8:0:1::feed:bad",
	"77.88.8.7", "77.88.8.3", "2a02:6b8::feed:a11", "2a02:6b8:0:1::feed:a11",
}

var Regions = []string{"默认", "俄罗斯", "伊朗"}

func RegionRouting(name string) Routing {
	switch name {
	case "俄罗斯":
		return regional([]Rule{
			{Remark: "绕过俄罗斯域名", Out: "direct", Domain: []string{"geosite:category-ru"}},
			{Remark: "绕过俄罗斯地址", Out: "direct", IP: []string{"geoip:ru"}},
		})
	case "伊朗":
		return regional([]Rule{
			{Remark: "绕过伊朗域名", Out: "direct", Domain: []string{"geosite:category-ir"}},
			{Remark: "绕过伊朗地址", Out: "direct", IP: []string{"geoip:ir"}},
		})
	}
	return DefaultRouting()
}

func regional(own []Rule) Routing {
	rules := append(baseRules(), own...)
	rules = append(rules, Rule{Remark: "拦截广告", Out: "block", Domain: []string{"geosite:category-ads-all"}})
	return Routing{Strategy: "IPIfNonMatch", Rules: rules}
}

func GeoEntries(routing Routing) []string {
	var out []string
	seen := map[string]bool{}
	for _, rule := range routing.Rules {
		for _, list := range [][]string{rule.Domain, rule.IP} {
			for _, entry := range list {
				if !strings.HasPrefix(entry, "geosite:") && !strings.HasPrefix(entry, "geoip:") {
					continue
				}
				if !seen[entry] {
					seen[entry] = true
					out = append(out, entry)
				}
			}
		}
	}
	return out
}

func MissingRuleSets(routing Routing, kind Kind) []string {
	var out []string
	for _, entry := range GeoEntries(routing) {
		if !have(entry, kind) {
			out = append(out, entry)
		}
	}
	return out
}
