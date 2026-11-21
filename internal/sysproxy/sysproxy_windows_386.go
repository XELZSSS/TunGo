//go:build windows && 386

package sysproxy

import (
	"errors"

	"tungo/internal/i18n"
)

var errUnsupported = errors.New(i18n.T("sysproxy: 32 位 Windows 不写系统代理"))

func Enable(string) error    { return errUnsupported }
func EnablePac(string) error { return errUnsupported }
func clear() error           { return errUnsupported }
func reset() error           { return errUnsupported }

func restore() bool { return false }
