//go:build darwin

package core

import "os/exec"

func resetTun() {
	cmd := exec.Command("ifconfig", tunInterface, "destroy")
	Hide(cmd)
	cmd.Run()
}
