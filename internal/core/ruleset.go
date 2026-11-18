package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"tungo/internal/i18n"
)

const rulesetURL = "https://raw.githubusercontent.com/2dust/sing-box-rules/rule-set-"

func have(entry string, kind Kind) bool {
	k, tag := geoKindTag(entry)
	name := k + "-" + tag + ".srs"
	if kind == Xray {
		if k == "geoip" {
			name = k + "-" + tag + ".cidrs.txt"
		} else {
			name = k + "-" + tag + ".domains.txt"
		}
	}
	return ruleSetPath(name) != ""
}

func EnsureRuleSet(entry string, kind Kind) error {
	k, tag := geoKindTag(entry)
	dir := ruleSetDir()
	if dir == "" {
		return errors.New(i18n.T("找不到规则集目录"))
	}
	if have(entry, kind) {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	base := filepath.Join(dir, k+"-"+tag)
	if ruleSetPath(k+"-"+tag+".srs") == "" {
		if err := fetchRuleSet(rulesetURL+k+"/"+k+"-"+tag+".srs", base+".srs"); err != nil {
			return fmt.Errorf("%s: %w", entry, err)
		}
	}
	if kind != Xray {
		return nil
	}
	if err := decompile(ruleSetPath(k+"-"+tag+".srs"), base); err != nil {
		return fmt.Errorf(i18n.T("%s：xray 要读文本列表，反编译失败：%w"), entry, err)
	}
	if !have(entry, kind) {
		return fmt.Errorf(i18n.T("%s：反编译后没有 %s 的列表"), entry, tag)
	}
	return nil
}

func fetchRuleSet(url, path string) error {
	out, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tmp := out.Name()
	err = fetch(url, out, nil)
	out.Close()
	if err != nil {
		os.Remove(tmp)
		return err
	}
	head := make([]byte, 3)
	file, err := os.Open(tmp)
	if err != nil {
		os.Remove(tmp)
		return err
	}
	_, err = file.Read(head)
	file.Close()
	if err != nil || string(head) != "SRS" {
		os.Remove(tmp)
		return fmt.Errorf(i18n.T("%s 不是规则集"), url)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func decompile(srs, prefix string) error {
	binary, err := Binary(SingBox)
	if err != nil {
		return errors.New(i18n.T("准备 xray 规则集需要 sing-box 程序，请先安装 sing-box"))
	}
	dump := prefix + ".json"
	defer os.Remove(dump)
	cmd := exec.Command(binary, "rule-set", "decompile", "-o", dump, srs)
	prepare(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rule-set decompile: %w: %s", err, out)
	}
	data, err := os.ReadFile(dump)
	if err != nil {
		return err
	}

	var set struct {
		Rules []struct {
			Domain        []string `json:"domain"`
			DomainSuffix  []string `json:"domain_suffix"`
			DomainKeyword []string `json:"domain_keyword"`
			DomainRegex   []string `json:"domain_regex"`
			IPCIDR        []string `json:"ip_cidr"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(data, &set); err != nil {
		return err
	}
	var domains, cidrs []string
	for _, rule := range set.Rules {
		for _, one := range rule.Domain {
			domains = append(domains, "full:"+one)
		}
		for _, one := range rule.DomainSuffix {
			domains = append(domains, "domain:"+one)
		}
		for _, one := range rule.DomainKeyword {
			domains = append(domains, "keyword:"+one)
		}
		for _, one := range rule.DomainRegex {
			domains = append(domains, "regexp:"+one)
		}
		cidrs = append(cidrs, rule.IPCIDR...)
	}
	if len(domains) > 0 {
		if err := writeLines(prefix+".domains.txt", domains); err != nil {
			return err
		}
	}
	if len(cidrs) > 0 {
		if err := writeLines(prefix+".cidrs.txt", cidrs); err != nil {
			return err
		}
	}
	return nil
}

func writeLines(path string, lines []string) error {
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return fmt.Errorf(i18n.T("写 %s: %w"), filepath.Base(path), err)
	}
	return nil
}
