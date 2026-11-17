package core

import (
	"net"
	"strings"
	"time"
)

const (
	tunProbe        = 100 * time.Millisecond
	tunClearTimeout = 5 * time.Second
)

func cleanTun() {
	resetTun()
	deadline := time.Now().Add(tunClearTimeout)
	for tunPresent() {
		if time.Now().After(deadline) {
			return
		}
		time.Sleep(tunProbe)
	}
}

func tunPresent() bool {
	interfaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, one := range interfaces {
		if strings.HasPrefix(one.Name, tunInterface) {
			return true
		}
	}
	return false
}

func waitTun(k *Kernel, name string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for !tunUp(name) {
		if k.exited() || time.Now().After(deadline) {
			return false
		}
		time.Sleep(tunProbe)
	}
	return true
}

func tunUp(name string) bool {
	interfaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, one := range interfaces {
		if strings.HasPrefix(one.Name, name) && one.Flags&net.FlagUp != 0 {
			return true
		}
	}
	return false
}
