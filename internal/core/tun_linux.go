//go:build linux

package core

import "os/exec"

func resetTun() {
	cmd := exec.Command("ip", "link", "delete", tunInterface)
	Hide(cmd)
	cmd.Run()
}
