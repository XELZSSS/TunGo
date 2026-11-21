package core

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func geoKindTag(entry string) (kind, tag string) {
	entry = strings.TrimSpace(entry)
	if kind, tag, ok := strings.Cut(entry, ":"); ok && (kind == "geosite" || kind == "geoip") {
		return kind, tag
	}
	return "geosite", entry
}

func ruleSetDirs() []string {
	dirs := make([]string, 0, 2)
	for _, bin := range bins() {
		dirs = append(dirs, filepath.Join(bin, "srss"))
	}
	return dirs
}

func ruleSetPath(name string) string {
	for _, dir := range ruleSetDirs() {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func ruleSetDir() string {
	bin, err := binDir()
	if err != nil {
		return ""
	}
	return filepath.Join(bin, "srss")
}

type fileList struct {
	size  int64
	mod   time.Time
	lines []string
}

var lists sync.Map

func cachedList(path string, parse func([]string) []string) []string {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if one, ok := lists.Load(path); ok {
		if kept := one.(fileList); kept.size == info.Size() && kept.mod.Equal(info.ModTime()) {
			return kept.lines
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := parse(listLines(string(data)))
	lists.Store(path, fileList{size: info.Size(), mod: info.ModTime(), lines: lines})
	return lines
}

func listLines(text string) []string {
	raw := strings.Split(text, "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func domainFile(path string) []string {
	return cachedList(path, func(lines []string) []string {
		out := make([]string, 0, len(lines))
		for _, line := range lines {
			switch {
			case strings.HasPrefix(line, "full:"), strings.HasPrefix(line, "keyword:"), strings.HasPrefix(line, "regexp:"):
				out = append(out, line)
			default:
				out = append(out, "domain:"+strings.TrimPrefix(line, "domain:"))
			}
		}
		return out
	})
}

var privateCIDRs = []string{
	"::/127",
	"0.0.0.0/8",
	"10.0.0.0/8",
	"100::/64",
	"100.64.0.0/10",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"172.16.0.0/12",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"192.168.0.0/16",
	"192.31.196.0/24",
	"192.52.193.0/24",
	"192.88.99.0/24",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"2001::/23",
	"2001:db8::/32",
	"203.0.113.0/24",
	"224.0.0.0/3",
	"2620:4f:8000::/48",
	"3fff::/20",
	"5f00::/16",
	"64:ff9b::/96",
	"64:ff9b:1::/48",
	"fc00::/7",
	"fe80::/10",
	"ff00::/8",
}

func xrayGeo(entries []string) (domains []string) {
	for _, entry := range entries {
		if kind, tag := geoKindTag(entry); kind == "geosite" {
			if path := ruleSetPath("geosite-" + tag + ".domains.txt"); path != "" {
				domains = append(domains, domainFile(path)...)
			}
		}
	}
	return domains
}

func xrayGeoIPs(entries []string) (cidrs []string) {
	for _, entry := range entries {
		if kind, tag := geoKindTag(entry); kind == "geoip" {
			if tag == "private" {
				cidrs = append(cidrs, privateCIDRs...)
				continue
			}
			if path := ruleSetPath("geoip-" + tag + ".cidrs.txt"); path != "" {
				cidrs = append(cidrs, fileLines(path)...)
			}
		}
	}
	return cidrs
}
