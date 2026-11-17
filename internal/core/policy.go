package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
	return len(r.Domain) == 0 && len(r.IP) == 0 && r.Port == "" &&
		len(r.NetworkList()) == 0 && len(r.Protocol) == 0
}

func DefaultDNS() DNS {
	return DNS{
		Remote:   "https://1.1.1.1/dns-query",
		Direct:   "223.5.5.5",
		Strategy: "prefer_ipv4",
	}
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
	set := DefaultRoutingSet()
	err := read("routing.json", &set)
	if len(set.Items) == 0 {
		set = DefaultRoutingSet()
	}
	if set.Item(set.Current) == nil {
		set.Current = set.Items[0].Name
	}
	return set, err
}

func SaveRoutingSet(set RoutingSet) error { return writePolicy("routing.json", set) }

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
	err := read("dns.json", &dns)
	return dns, err
}

func SaveDNS(dns DNS) error { return writePolicy("dns.json", dns) }

func read(name string, v any) error {
	path, err := policyPath(name)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s 解析失败：%w", name, err)
	}
	return nil
}

func writePolicy(name string, v any) error {
	path, err := policyPath(name)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func policyPath(name string) (string, error) {
	dir, err := appdir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

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
