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
	at    time.Time
	size  int64
	lines []string
}

var lists sync.Map

func cachedList(path string, parse func([]string) []string) []string {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if one, ok := lists.Load(path); ok {
		if kept := one.(fileList); kept.at.Equal(info.ModTime()) && kept.size == info.Size() {
			return kept.lines
		}
	}
	lines := parse(rawLines(path))
	lists.Store(path, fileList{at: info.ModTime(), size: info.Size(), lines: lines})
	return lines
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
func rawLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
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
			if path := ruleSetPath("geoip-" + tag + ".cidrs.txt"); path != "" {
				cidrs = append(cidrs, fileLines(path)...)
			}
		}
	}
	return cidrs
}
