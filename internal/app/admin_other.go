//go:build !windows

package app

import "os"

func admin() bool { return os.Geteuid() == 0 }
