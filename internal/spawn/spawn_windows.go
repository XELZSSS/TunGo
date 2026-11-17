//go:build windows

package spawn

import (
	"os/exec"
	"syscall"
)

const noWindow = 0x08000000

func hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: noWindow}
}
