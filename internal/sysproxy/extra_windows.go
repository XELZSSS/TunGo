//go:build windows

package sysproxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"tungo/internal/i18n"
)

func hidden(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

func MirrorWinHTTP() error {
	out, err := hidden(exec.Command("netsh", "winhttp", "import", "proxy", "source=ie")).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func ExemptLoopback() (int, error) {
	entries, err := os.ReadDir(filepath.Join(os.Getenv("LOCALAPPDATA"), "Packages"))
	if err != nil {
		return 0, err
	}
	exempted := 0
	for _, entry := range entries {
		if !entry.IsDir() || !strings.Contains(entry.Name(), "_") {
			continue
		}
		if err := hidden(exec.Command("CheckNetIsolation.exe", "LoopbackExempt", "-a", "-n="+entry.Name())).Run(); err != nil {
			return exempted, fmt.Errorf(i18n.T("放行 %s 失败：%w"), entry.Name(), err)
		}
		exempted++
	}
	return exempted, nil
}

func LoopbackAvailable() bool { return true }
