package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

func (a *app) reveal(path string) {
	if path == "" {
		return
	}
	dir := path
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		dir = filepath.Dir(path)
	}
	if err := mygo.Shell.OpenPath(dir); err != nil {
		a.logf(i18n.T("打不开目录：%s"), err)
	}
}

func (a *app) rebootAsAdmin() {
	exe, err := os.Executable()
	if err != nil {
		a.fail(err.Error())
		return
	}
	if runtime.GOOS != "windows" {
		a.hint(i18n.T("请用 sudo 重新运行"))
		return
	}
	script := fmt.Sprintf("Wait-Process -Id %d -ErrorAction SilentlyContinue; Start-Process -FilePath '%s' -Verb RunAs",
		os.Getpid(), strings.ReplaceAll(exe, "'", "''"))
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	core.Hide(cmd)
	if err := cmd.Start(); err != nil {
		a.fail(err.Error())
		a.logf(i18n.T("提不起权限：%s"), err)
		return
	}
	a.logf("%s", i18n.T("已请求以管理员身份重启"))
	a.requestQuit()
}

func (a *app) subName(at int) string {
	if at < 0 || at >= len(a.subs.Items) {
		return ""
	}
	return a.subs.Items[at].Name
}
