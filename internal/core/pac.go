package core

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
)

const pacLimit = 130000

const pacBody = `function FindProxyForURL(url, host) {
  host = host.toLowerCase();
  if (isPlainHostName(host) || shExpMatch(host, "*.local") ||
      isInNet(host, "127.0.0.0", "255.0.0.0") || isInNet(host, "10.0.0.0", "255.0.0.0") ||
      isInNet(host, "172.16.0.0", "255.240.0.0") || isInNet(host, "192.168.0.0", "255.255.0.0") ||
      isInNet(host, "169.254.0.0", "255.255.0.0")) return "DIRECT";
  if (directFull[host]) return "DIRECT";
  var one = host;
  while (true) {
    if (directSuffix[one]) return "DIRECT";
    var at = one.indexOf(".");
    if (at < 0) break;
    one = one.substring(at + 1);
  }
  return "PROXY $PROXY;DIRECT;";
}
`

func PacScript(addr string, routing Routing) string {
	suffix, full := directDomains(routing)
	var out strings.Builder
	out.WriteString("var directFull = ")
	out.WriteString(jsonMap(full))
	out.WriteString(";\nvar directSuffix = ")
	out.WriteString(jsonMap(suffix))
	out.WriteString(";\n")
	out.WriteString(strings.ReplaceAll(pacBody, "$PROXY", addr))
	return out.String()
}

func jsonMap(values []string) string {
	if len(values) == 0 {
		return "{}"
	}
	var out strings.Builder
	out.WriteString("{")
	for i, value := range values {
		if i > 0 {
			out.WriteString(",")
		}
		quoted, _ := json.Marshal(value)
		out.Write(quoted)
		out.WriteString(":1")
	}
	out.WriteString("}")
	return out.String()
}

func directDomains(routing Routing) (suffix, full []string) {
	seen := map[string]bool{}
	add := func(into *[]string, value string) {
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		*into = append(*into, value)
	}
	addLine := func(line string) {
		switch {
		case strings.HasPrefix(line, "full:"):
			add(&full, strings.TrimPrefix(line, "full:"))
		case strings.HasPrefix(line, "keyword:"), strings.HasPrefix(line, "regexp:"):
		default:
			add(&suffix, strings.TrimPrefix(line, "domain:"))
		}
	}
	for _, rule := range routing.Rules {
		if rule.Out != "direct" {
			continue
		}
		for _, entry := range rule.Domain {
			if tag, ok := strings.CutPrefix(entry, "geosite:"); ok {
				path := ruleSetPath("geosite-" + tag + ".domains.txt")
				if path == "" {
					continue
				}
				for _, line := range domainFile(path) {
					addLine(line)
				}
				continue
			}
			if strings.HasPrefix(entry, "geoip:") {
				continue
			}
			addLine(entry)
		}
	}
	slices.SortStableFunc(suffix, func(one, other string) int {
		return cmp.Compare(strings.Count(one, "."), strings.Count(other, "."))
	})
	if len(suffix) > pacLimit {
		suffix = suffix[:pacLimit]
	}
	return suffix, full
}
