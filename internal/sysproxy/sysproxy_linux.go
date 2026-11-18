//go:build linux

package sysproxy

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

func Enable(addr string) error {
	var host, port string
	if h, p, err := net.SplitHostPort(addr); err == nil {
		host, port = h, p
	} else {
		var ok bool
		host, port, ok = strings.Cut(addr, ":")
		if !ok {
			return fmt.Errorf("sysproxy: %q is not host:port", addr)
		}
	}
	if err := gsettings("org.gnome.system.proxy.socks", "host", host); err != nil {
		return err
	}
	if err := gsettings("org.gnome.system.proxy.socks", "port", port); err != nil {
		return err
	}
	return gsettings("org.gnome.system.proxy", "mode", "manual")
}

func EnablePac(url string) error {
	if err := gsettings("org.gnome.system.proxy", "autoconfig-url", url); err != nil {
		return err
	}
	return gsettings("org.gnome.system.proxy", "mode", "auto")
}

func clear() error { return gsettings("org.gnome.system.proxy", "mode", "none") }

func reset() error { return clear() }

func gsettings(schema, key, value string) error {
	out, err := exec.Command("gsettings", "set", schema, key, value).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gsettings set %s %s: %w: %s", schema, key, err, out)
	}
	return nil
}
