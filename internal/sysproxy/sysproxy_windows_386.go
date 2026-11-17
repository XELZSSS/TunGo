//go:build windows && 386

package sysproxy

import "errors"

var errUnsupported = errors.New("sysproxy: 32 位 Windows 不写系统代理")

func Enable(string) error    { return errUnsupported }
func EnablePac(string) error { return errUnsupported }
func clear() error           { return errUnsupported }
