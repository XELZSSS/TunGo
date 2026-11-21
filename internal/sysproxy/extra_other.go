//go:build !windows

package sysproxy

import (
	"errors"

	"tungo/internal/i18n"
)

var errUnsupported = errors.New(i18n.T("只有 Windows 支持这一步"))

func MirrorWinHTTP() error { return errUnsupported }

func ExemptLoopback() (int, error) { return 0, errUnsupported }

func LoopbackAvailable() bool { return false }
