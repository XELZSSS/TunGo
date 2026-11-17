//go:build !windows && !linux

package core

import "os/exec"

func prepare(cmd *exec.Cmd) {}

func Hide(cmd *exec.Cmd) {}

func follow(cmd *exec.Cmd) {}

func stop(cmd *exec.Cmd) {}
