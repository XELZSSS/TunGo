package app

import "golang.org/x/sys/windows"

var shell32 = windows.NewLazySystemDLL("shell32.dll")

func admin() bool {
	ok, _, _ := shell32.NewProc("IsUserAnAdmin").Call()
	return ok != 0
}
