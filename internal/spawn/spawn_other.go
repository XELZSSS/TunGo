//go:build !windows

package spawn

import "os/exec"

func hide(cmd *exec.Cmd) {}
