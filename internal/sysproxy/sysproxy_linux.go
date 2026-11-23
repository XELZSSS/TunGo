//go:build linux

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
	Host string `json:"host"`
	Port string `json:"port"`
}

type snapshot struct {
	Desktop string `json:"desktop"`
	Mode    string `json:"mode"`
	HTTP    entry  `json:"http"`
	HTTPS   entry  `json:"https"`
	Socks   entry  `json:"socks"`
	Auto    string `json:"auto"`
}

func Enable(addr string) error {
	host, port, err := splitAddr(addr)
	if err != nil {
		return err
	}
	save()
	one := entry{Host: host, Port: port}
	return write(snapshot{Desktop: desktop(), Mode: "manual", HTTP: one, HTTPS: one, Socks: one})
}

func EnablePac(url string) error {
	save()
	return write(snapshot{Desktop: desktop(), Mode: "auto", Auto: url})
}

func clear() error {
	if restore() {
		return nil
	}
	return reset()
}

func reset() error {
	return write(snapshot{Desktop: desktop(), Mode: "none"})
}

func restore() bool {
	var saved snapshot
	if err := appdir.ReadJSON(snapshotName, &saved); err != nil {
		return false
	}
	appdir.Remove(snapshotName)
	if saved.Desktop == "" {
		return false
	}
	return write(saved) == nil
}

func save() {
	saved, err := query(desktop())
	if err != nil {
		return
	}
	appdir.WriteJSON(snapshotName, saved)
}

func query(which string) (snapshot, error) {
	saved := snapshot{Desktop: which}
	switch which {
	case "gnome":
		saved.Mode = strings.Trim(gget("org.gnome.system.proxy", "mode"), "'")
		saved.Auto = strings.Trim(gget("org.gnome.system.proxy", "autoconfig-url"), "'")
		for _, one := range []struct {
			schema string
			into   *entry
		}{
			{"org.gnome.system.proxy.http", &saved.HTTP},
			{"org.gnome.system.proxy.https", &saved.HTTPS},
			{"org.gnome.system.proxy.socks", &saved.Socks},
		} {
			one.into.Host = strings.Trim(gget(one.schema, "host"), "'")
			one.into.Port = gget(one.schema, "port")
		}
	case "kde":
		saved.Mode = kget("ProxyType")
		saved.Auto = kget("Proxy Config Script")
		saved.HTTP = kdeEntry(kget("httpProxy"))
		saved.HTTPS = kdeEntry(kget("httpsProxy"))
		saved.Socks = kdeEntry(kget("socksProxy"))
	default:
		return snapshot{}, fmt.Errorf("sysproxy: %s 桌面的系统代理还不支持", which)
	}
	return saved, nil
}

func write(saved snapshot) error {
	if saved.Desktop == "" {
		return fmt.Errorf("sysproxy: 没认出当前桌面，系统代理只能手动设置")
	}
	switch saved.Desktop {
	case "gnome":
		return writeGnome(saved)
	case "kde":
		return writeKDE(saved)
	}
	return fmt.Errorf("sysproxy: %s 桌面的系统代理还不支持", saved.Desktop)
}

func writeGnome(saved snapshot) error {
	mode := saved.Mode
	if mode == "" || mode == "none" {
		return gset("org.gnome.system.proxy", "mode", "none")
	}
	if mode == "auto" {
		if err := gset("org.gnome.system.proxy", "autoconfig-url", quote(saved.Auto)); err != nil {
			return err
		}
		return gset("org.gnome.system.proxy", "mode", "auto")
	}
	for _, one := range []struct {
		schema string
		entry  entry
	}{
		{"org.gnome.system.proxy.http", saved.HTTP},
		{"org.gnome.system.proxy.https", saved.HTTPS},
		{"org.gnome.system.proxy.socks", saved.Socks},
	} {
		if err := gset(one.schema, "host", quote(one.entry.Host)); err != nil {
			return err
		}
		if err := gset(one.schema, "port", port(one.entry.Port)); err != nil {
			return err
		}
	}
	return gset("org.gnome.system.proxy", "mode", "manual")
}

func writeKDE(saved snapshot) error {
	kind := "0"
	switch saved.Mode {
	case "manual":
		kind = "1"
	case "auto":
		kind = "2"
	}
	if kind == "1" {
		for _, one := range []struct {
			key    string
			scheme string
			entry  entry
		}{
			{"httpProxy", "http", saved.HTTP},
			{"httpsProxy", "http", saved.HTTPS},
			{"socksProxy", "socks", saved.Socks},
		} {
			if err := kset(one.key, one.scheme+"://"+one.entry.Host+" "+one.entry.Port); err != nil {
				return err
			}
		}
	}
	if kind == "2" {
		if err := kset("Proxy Config Script", saved.Auto); err != nil {
			return err
		}
	}
	if err := kset("ProxyType", kind); err != nil {
		return err
	}
	kdeReload()
	return nil
}

func kdeEntry(value string) entry {
	value = strings.TrimSpace(strings.TrimPrefix(value, "http://"))
	value = strings.TrimPrefix(value, "socks://")
	value = strings.TrimPrefix(value, "https://")
	host, port, _ := strings.Cut(value, " ")
	return entry{Host: host, Port: strings.TrimSpace(port)}
}

func quote(text string) string {
	return "'" + strings.ReplaceAll(text, "'", "") + "'"
}

func port(text string) string {
	if strings.TrimSpace(text) == "" {
		return "0"
	}
	return strings.TrimSpace(text)
}

func splitAddr(addr string) (host, port string, err error) {
	host, port, err = net.SplitHostPort(addr)
	if err != nil {
		return "", "", fmt.Errorf("sysproxy: %w", err)
	}
	return host, port, nil
}

func desktop() string {
	if _, err := exec.LookPath("gsettings"); err == nil {
		return "gnome"
	}
	if kdeTool("kwriteconfig") != "" {
		return "kde"
	}
	return ""
}

func kdeTool(name string) string {
	for _, one := range []string{name + "6", name + "5", name} {
		if _, err := exec.LookPath(one); err == nil {
			return one
		}
	}
	return ""
}

func gget(schema, key string) string {
	out, err := exec.Command("gsettings", "get", schema, key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gset(schema, key, value string) error {
	out, err := exec.Command("gsettings", "set", schema, key, value).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gsettings set %s %s: %w: %s", schema, key, err, out)
	}
	return nil
}

func kget(key string) string {
	out, err := exec.Command(kdeTool("kreadconfig"), "--file", "kioslaverc", "--group", "Proxy Settings", "--key", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func kset(key, value string) error {
	out, err := exec.Command(kdeTool("kwriteconfig"), "--file", "kioslaverc", "--group", "Proxy Settings", "--key", key, value).CombinedOutput()
	if err != nil {
		return fmt.Errorf("kwriteconfig %s: %w: %s", key, err, out)
	}
	return nil
}

func kdeReload() {
	exec.Command("dbus-send", "--type=signal", "/KIO/Scheduler",
		"org.kde.KIO.Scheduler.reparseSlaveConfiguration", "string:''").Run()
}
