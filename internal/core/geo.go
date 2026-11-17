package core

import (
	"os"
	"path/filepath"
	"strings"
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

func xrayGeo(entries []string) (domains []string) {
	for _, entry := range entries {
		kind, tag := geoKindTag(entry)
		if kind != "geosite" {
			continue
		}
		path := ruleSetPath("geosite-" + tag + ".domains.txt")
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			switch line = strings.TrimSpace(line); {
			case line == "":
			case strings.HasPrefix(line, "full:"), strings.HasPrefix(line, "keyword:"),
				strings.HasPrefix(line, "regexp:"):
				domains = append(domains, line)
			default:
				domains = append(domains, "domain:"+strings.TrimPrefix(line, "domain:"))
			}
		}
	}
	return domains
}

func xrayGeoIPs(entries []string) (cidrs []string) {
	for _, entry := range entries {
		kind, tag := geoKindTag(entry)
		if kind != "geoip" {
			continue
		}
		path := ruleSetPath("geoip-" + tag + ".cidrs.txt")
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				cidrs = append(cidrs, line)
			}
		}
	}
	return cidrs
}
