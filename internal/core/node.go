package core

import (
	"fmt"
	"path/filepath"
	"strings"

	"tungo/internal/appdir"
	"tungo/internal/i18n"
)

var Protocols = []string{
	"direct", "socks", "http", "shadowsocks", "vmess", "vless", "trojan",
	"hysteria2", "tuic",
}

var Networks = []string{"tcp", "ws", "grpc", "http", "httpupgrade"}

var Securities = []string{"none", "tls", "reality"}

type Node struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`

	Sub    string `json:"sub,omitempty"`
	Server string `json:"server,omitempty"`
	Port   int    `json:"port,omitempty"`

	Method   string `json:"method,omitempty"`
	Password string `json:"password,omitempty"`
	Username string `json:"username,omitempty"`
	UUID     string `json:"uuid,omitempty"`
	Flow     string `json:"flow,omitempty"`
	AlterID  int    `json:"alterId,omitempty"`
	Security string `json:"security,omitempty"`

	Network string `json:"network,omitempty"`
	Path    string `json:"path,omitempty"`
	Host    string `json:"host,omitempty"`

	TLS         string   `json:"tls,omitempty"`
	ServerName  string   `json:"serverName,omitempty"`
	ALPN        []string `json:"alpn,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	Insecure    bool     `json:"insecure,omitempty"`
	PublicKey   string   `json:"publicKey,omitempty"`
	ShortID     string   `json:"shortId,omitempty"`

	ECH string `json:"ech,omitempty"`

	Pinned string `json:"pinned,omitempty"`

	Obfs         string `json:"obfs,omitempty"`
	ObfsPassword string `json:"obfsPassword,omitempty"`
	Congestion   string `json:"congestion,omitempty"`

	Detour *Node `json:"detour,omitempty"`

	Raw string `json:"raw,omitempty"`

	Role string `json:"role,omitempty"`

	Children []string `json:"children,omitempty"`

	Strategy string `json:"strategy,omitempty"`

	Members []Node `json:"-"`
}

const GroupRole = "group"

var GroupStrategies = []string{"leastPing", "fallback", "random", "roundRobin", "leastLoad"}

var strategyLabels = map[string]string{
	"leastPing":  "最低延迟",
	"fallback":   "故障转移",
	"random":     "随机",
	"roundRobin": "轮询",
	"leastLoad":  "最低负载",
}

func StrategyName(id string) string {
	if label, ok := strategyLabels[id]; ok {
		return i18n.T(label)
	}
	return i18n.T(strategyLabels[GroupStrategies[0]])
}

func (n Node) Group() bool { return n.Role == GroupRole }

func (n Node) StrategyOr() string {
	if n.Strategy == "" {
		return "leastPing"
	}
	return n.Strategy
}

func (n Node) Direct() bool { return n.Protocol == "direct" }

func (n Node) NeedsPin() bool {
	if n.Raw != "" {
		return false
	}
	if n.Pinned != "" {
		return true
	}
	for _, member := range n.Members {
		if member.NeedsPin() {
			return true
		}
	}
	return n.Detour != nil && n.Detour.NeedsPin()
}

func (k Kind) Supports(protocol string) bool {
	switch protocol {
	case "direct":
		return true
	case "hysteria2", "tuic":
		return k == SingBox
	}
	for _, known := range Protocols {
		if known == protocol {
			return true
		}
	}
	return false
}

func LoadNodes() ([]Node, string, error) {
	path, err := nodesPath()
	if err != nil {
		return nil, "", err
	}
	var nodes []Node
	if err := appdir.ReadJSON("nodes.json", &nodes); err != nil {
		return nil, path, err
	}
	return nodes, path, nil
}

func SaveNodes(nodes []Node) error { return appdir.WriteJSON("nodes.json", nodes) }

func nodesPath() (string, error) {
	d, err := appdir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "nodes.json"), nil
}

func (n Node) Summary() string {
	if n.Group() {
		return fmt.Sprintf("%s · %s · %d", i18n.T("策略组"), StrategyName(n.StrategyOr()), len(n.Children))
	}
	parts := []string{n.Protocol}
	if n.Network != "" && n.Network != "tcp" {
		parts = append(parts, n.Network)
	}
	if n.TLS != "" {
		parts = append(parts, n.TLS)
	}
	if n.Detour != nil {
		parts = append(parts, "→ "+n.Detour.Name)
	}
	if n.Raw != "" {
		parts = append(parts, i18n.T("自定义"))
	}
	return strings.Join(parts, " · ")
}
