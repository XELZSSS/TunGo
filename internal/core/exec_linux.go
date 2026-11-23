//go:build linux

package core

import (
	"os/exec"
	"syscall"
)

func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL, Setpgid: true}
}

func Hide(cmd *exec.Cmd) {}

func follow(cmd *exec.Cmd) {}

func stop(cmd *exec.Cmd) {
	syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
}

func kill(cmd *exec.Cmd) {
	syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
