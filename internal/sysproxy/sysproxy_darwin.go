//go:build darwin

package sysproxy

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

func Enable(addr string) error {
	service, err := activeService()
	if err != nil {
		return err
	}
	host, port, err := splitAddr(addr)
	if err != nil {
		return err
	}
	if err := run("networksetup", "-setautoproxystate", service, "off"); err != nil {
		return err
	}
	if err := run("networksetup", "-setsocksfirewallproxy", service, host, port); err != nil {
		return err
	}
	return run("networksetup", "-setsocksfirewallproxystate", service, "on")
}

func EnablePac(url string) error {
	service, err := activeService()
	if err != nil {
		return err
	}
	if err := run("networksetup", "-setsocksfirewallproxystate", service, "off"); err != nil {
		return err
	}
	if err := run("networksetup", "-setautoproxyurl", service, url); err != nil {
		return err
	}
	return run("networksetup", "-setautoproxystate", service, "on")
}

func clear() error {
	service, err := activeService()
	if err != nil {
		return err
	}
	for _, arg := range []string{"-setsocksfirewallproxystate", "-setautoproxystate"} {
		if err := run("networksetup", arg, service, "off"); err != nil {
			return err
		}
	}
	return nil
}
func reset() error { return clear() }

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
