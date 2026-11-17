package app

import "syscall"

var shell32 = syscall.NewLazyDLL("shell32.dll")

func admin() bool {
	ok, _, _ := shell32.NewProc("IsUserAnAdmin").Call()
	return ok != 0
}
