//go:build darwin

package sysproxy

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

	"tungo/internal/appdir"
)

const snapshotName = "sysproxy.json"

type entry struct {
	On   bool   `json:"on"`
	Host string `json:"host"`
	Port string `json:"port"`
	URL  string `json:"url"`
}

type snapshot struct {
	Service string `json:"service"`
	Web     entry  `json:"web"`
	Secure  entry  `json:"secure"`
	Socks   entry  `json:"socks"`
	Auto    entry  `json:"auto"`
}

func Enable(addr string) error {
	service, err := activeService()
	if err != nil {
		return err
	}
	host, port, err := splitAddr(addr)
	if err != nil {
		return err
	}
	save(service)
	one := entry{On: true, Host: host, Port: port}
	return write(snapshot{Service: service, Web: one, Secure: one, Socks: one})
}

func EnablePac(url string) error {
	service, err := activeService()
	if err != nil {
		return err
	}
	save(service)
	return write(snapshot{Service: service, Auto: entry{On: true, URL: url}})
}

func clear() error {
	if restore() {
		return nil
	}
	return reset()
}

func reset() error {
	service, err := activeService()
	if err != nil {
		return err
	}
	return write(snapshot{Service: service})
}

func restore() bool {
	var saved snapshot
	if err := appdir.ReadJSON(snapshotName, &saved); err != nil {
		return false
	}
	dropSnapshot()
	if saved.Service == "" {
		return false
	}
	return write(saved) == nil
}

func dropSnapshot() {
	appdir.Remove(snapshotName)
}

func save(service string) {
	saved, err := query(service)
	if err != nil {
		return
	}
	appdir.WriteJSON(snapshotName, saved)
}

func query(service string) (snapshot, error) {
	saved := snapshot{Service: service}
	for _, one := range []struct {
		flag string
		into *entry
	}{
		{"-getwebproxy", &saved.Web},
		{"-getsecurewebproxy", &saved.Secure},
		{"-getsocksfirewallproxy", &saved.Socks},
		{"-getautoproxyurl", &saved.Auto},
	} {
		out, err := exec.Command("networksetup", one.flag, service).Output()
		if err != nil {
			return snapshot{}, fmt.Errorf("networksetup %s: %w", one.flag, err)
		}
		readFields(string(out), one.into)
	}
	return saved, nil
}

func readFields(out string, into *entry) {
	for _, line := range strings.Split(out, "\n") {
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(name) {
		case "Enabled":
			into.On = value == "Yes"
		case "Server":
			into.Host = value
		case "Port":
			into.Port = value
		case "URL":
			into.URL = value
		}
	}
}

func write(saved snapshot) error {
	for _, one := range []struct {
		set   string
		state string
		entry entry
	}{
		{"-setwebproxy", "-setwebproxystate", saved.Web},
		{"-setsecurewebproxy", "-setsecurewebproxystate", saved.Secure},
		{"-setsocksfirewallproxy", "-setsocksfirewallproxystate", saved.Socks},
	} {
		if one.entry.On && one.entry.Host != "" && one.entry.Port != "" {
			if err := run("networksetup", one.set, saved.Service, one.entry.Host, one.entry.Port); err != nil {
				return err
			}
		}
		if err := run("networksetup", one.state, saved.Service, onOff(one.entry.On)); err != nil {
			return err
		}
	}
	if saved.Auto.On && saved.Auto.URL != "" {
		if err := run("networksetup", "-setautoproxyurl", saved.Service, saved.Auto.URL); err != nil {
			return err
		}
	}
	return run("networksetup", "-setautoproxystate", saved.Service, onOff(saved.Auto.On))
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func splitAddr(addr string) (host, port string, err error) {
	host, port, err = net.SplitHostPort(addr)
	if err != nil {
		return "", "", fmt.Errorf("sysproxy: %w", err)
	}
	return host, port, nil
}

func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, out)
	}
	return nil
}

func activeService() (string, error) {
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return "", fmt.Errorf("sysproxy: %w", err)
	}
	device := ""
	for _, line := range strings.Split(string(out), "\n") {
		if name, value, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(name) == "interface" {
			device = strings.TrimSpace(value)
		}
	}
	if device == "" {
		return "", fmt.Errorf("sysproxy: no default route")
	}

	out, err = exec.Command("networksetup", "-listnetworkserviceorder").Output()
	if err != nil {
		return "", fmt.Errorf("sysproxy: %w", err)
	}

	for _, block := range strings.Split(string(out), "\n\n") {
		if !strings.Contains(block, "Device: "+device) {
			continue
		}
		name := strings.TrimSpace(strings.Split(strings.TrimSpace(block), "\n")[0])
		if _, after, ok := strings.Cut(name, ") "); ok {
			name = after
		}
		return strings.TrimSpace(name), nil
	}
	return "", fmt.Errorf("sysproxy: no network service for %s", device)
}
