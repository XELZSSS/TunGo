package core

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"tungo/internal/appdir"
)

type Rule struct {
	Remark  string   `json:"remark,omitempty"`
	Out     string   `json:"out"`
	Domain  []string `json:"domain,omitempty"`
	IP      []string `json:"ip,omitempty"`
	Port    string   `json:"port,omitempty"`
	Network string   `json:"network,omitempty"`

	Protocol []string `json:"protocol,omitempty"`
}

var Outs = []string{"proxy", "direct", "block"}

var RuleNetworks = []string{"", "tcp", "udp", "tcp,udp"}

type Routing struct {
	Strategy string `json:"strategy"`

	Final string `json:"final,omitempty"`
	Rules []Rule `json:"rules"`
}

type SplitDNS struct {
	Domains []string `json:"domains"`
	Server  string   `json:"server"`
}

type DNS struct {
	Remote    string `json:"remote"`
	Direct    string `json:"direct"`
	Bootstrap string `json:"bootstrap"`
	Strategy  string `json:"strategy"`

	BlockAAAA bool `json:"blockAAAA"`

	Hosts map[string]string `json:"hosts,omitempty"`

	Split []SplitDNS `json:"split,omitempty"`
}

var Strategies = []string{"IPIfNonMatch", "IPOnDemand", "AsIs"}

var DNSStrategies = []string{"prefer_ipv4", "prefer_ipv6", "ipv4_only", "ipv6_only"}

var RoutingFinals = []string{"proxy", "direct"}

func (r Routing) FinalTag() string {
	if r.Final == "direct" {
		return "direct"
	}
	return "proxy"
}

func (r Rule) Ports() (ports []int, ranges []string) {
	for _, part := range strings.Split(r.Port, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if low, high, ok := strings.Cut(part, "-"); ok {
			ranges = append(ranges, strings.TrimSpace(low)+":"+strings.TrimSpace(high))
			continue
		}
		if port, err := strconv.Atoi(part); err == nil {
			ports = append(ports, port)
		}
	}
	return ports, ranges
}

func (r Rule) NetworkList() []string {
	var out []string
	for _, part := range strings.Split(r.Network, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (r Rule) Bare() bool {
	if len(r.Domain) > 0 || len(r.IP) > 0 || len(r.NetworkList()) > 0 || len(r.Protocol) > 0 {
		return false
	}
	if r.Port == "" {
		return true
	}
	ports, ranges := r.Ports()
	return len(ports) == 0 && len(ranges) == 0
}

func DefaultDNS() DNS {
	return DNS{
		Remote:    "https://1.1.1.1/dns-query",
		Direct:    "https://223.5.5.5/dns-query",
		Bootstrap: "https://8.8.8.8/dns-query",
		Strategy:  "prefer_ipv4",
	}
}

type DNSPreset struct {
	Name      string
	Remote    string
	Direct    string
	Bootstrap string
}

var DNSPresets = []DNSPreset{
	{
		Name:      "Cloudflare",
		Remote:    "https://1.1.1.1/dns-query",
		Direct:    "https://223.5.5.5/dns-query",
		Bootstrap: "https://8.8.8.8/dns-query",
	},
	{
		Name:      "Google",
		Remote:    "https://8.8.8.8/dns-query",
		Direct:    "https://223.5.5.5/dns-query",
		Bootstrap: "https://1.1.1.1/dns-query",
	},
	{
		Name:      "Quad9",
		Remote:    "https://9.9.9.9/dns-query",
		Direct:    "https://223.5.5.5/dns-query",
		Bootstrap: "https://1.1.1.1/dns-query",
	},
	{
		Name:      "AdGuard",
		Remote:    "https://94.140.14.14/dns-query",
		Direct:    "https://223.5.5.5/dns-query",
		Bootstrap: "https://1.1.1.1/dns-query",
	},
	{
		Name:      "阿里 DoH",
		Remote:    "https://223.5.5.5/dns-query",
		Direct:    "https://223.5.5.5/dns-query",
		Bootstrap: "https://8.8.8.8/dns-query",
	},
	{
		Name:      "腾讯 DoH",
		Remote:    "https://1.12.12.12/dns-query",
		Direct:    "https://1.12.12.12/dns-query",
		Bootstrap: "https://8.8.8.8/dns-query",
	},
	{
		Name:      "阿里明文",
		Remote:    "223.5.5.5",
		Direct:    "223.5.5.5",
		Bootstrap: "223.5.5.5",
	},
	{
		Name:      "腾讯明文",
		Remote:    "119.29.29.29",
		Direct:    "119.29.29.29",
		Bootstrap: "119.29.29.29",
	},
}

func DNSPresetOf(dns DNS) string {
	for _, one := range DNSPresets {
		if one.Remote == dns.Remote && one.Direct == dns.Direct && one.Bootstrap == dns.Bootstrap {
			return one.Name
		}
	}
	return ""
}

type Policy struct {
	Routing Routing
	DNS     DNS
}

func LoadPolicy() (Policy, error) {
	routing, err := LoadRouting()
	if err != nil {
		return Policy{}, err
	}
	dns, err := LoadDNS()
	if err != nil {
		return Policy{}, err
	}
	return Policy{Routing: routing, DNS: dns}, nil
}

type RoutingItem struct {
	Name    string  `json:"name"`
	Routing Routing `json:"routing"`
}

type RoutingSet struct {
	Current string        `json:"current"`
	Items   []RoutingItem `json:"items"`
}

func DefaultRoutingSet() RoutingSet {
	set := RoutingSet{Current: Routings[0]}
	for _, name := range Routings {
		set.Items = append(set.Items, RoutingItem{Name: name, Routing: PresetRouting(name)})
	}
	return set
}

func LoadRoutingSet() (RoutingSet, error) {
	var set RoutingSet
	err := appdir.ReadJSON("routing.json", &set)
	if err != nil || len(set.Items) == 0 {
		backupBrokenRouting()
		set = DefaultRoutingSet()
	}
	if set.Item(set.Current) == nil {
		set.Current = set.Items[0].Name
	}
	return set, err
}

func backupBrokenRouting() {
	dir, err := appdir.Dir()
	if err != nil {
		return
	}
	from := filepath.Join(dir, "routing.json")
	if _, err := os.Stat(from); err != nil {
		return
	}
	to := from + "." + time.Now().Format("20060102-150405") + ".bak"
	os.Rename(from, to)
}

func SaveRoutingSet(set RoutingSet) error { return appdir.WriteJSON("routing.json", set) }

func (s RoutingSet) Item(name string) *Routing {
	for i := range s.Items {
		if s.Items[i].Name == name {
			return &s.Items[i].Routing
		}
	}
	return nil
}

func (s RoutingSet) Routing() Routing {
	if routing := s.Item(s.Current); routing != nil {
		return *routing
	}
	return DefaultRouting()
}

func (s *RoutingSet) Put(name string, routing Routing) {
	if kept := s.Item(name); kept != nil {
		*kept = routing
		return
	}
	s.Items = append(s.Items, RoutingItem{Name: name, Routing: routing})
}

func (s *RoutingSet) Add(name string) {
	s.Put(name, s.Routing())
	s.Current = name
}

func (s *RoutingSet) Remove(name string) {
	if len(s.Items) < 2 {
		return
	}
	kept := make([]RoutingItem, 0, len(s.Items))
	for _, item := range s.Items {
		if item.Name != name {
			kept = append(kept, item)
		}
	}
	s.Items = kept
	if s.Current == name {
		s.Current = s.Items[0].Name
	}
}

func LoadRouting() (Routing, error) {
	set, err := LoadRoutingSet()
	return set.Routing(), err
}

func LoadDNS() (DNS, error) {
	dns := DefaultDNS()
	err := appdir.ReadJSON("dns.json", &dns)
	return dns, err
}

func SaveDNS(dns DNS) error { return appdir.WriteJSON("dns.json", dns) }

func List(values []string) string { return strings.Join(values, "\n") }

func ParseList(text string) []string {
	var out []string
	for _, field := range strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ' ' || r == '\t'
	}) {
		if field = strings.TrimSpace(field); field != "" {
			out = append(out, field)
		}
	}
	return out
}
