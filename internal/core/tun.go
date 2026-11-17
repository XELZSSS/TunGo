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
	for {
		if present, _ := tunState(tunInterface); !present || time.Now().After(deadline) {
			return
		}
		time.Sleep(tunProbe)
	}
}

func waitTun(k *Kernel, name string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if _, up := tunState(name); up {
			return true
		}
		if k.exited() || time.Now().After(deadline) {
			return false
		}
		time.Sleep(tunProbe)
	}
}

func tunState(name string) (present, up bool) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return false, false
	}
	for _, one := range interfaces {
		if !strings.HasPrefix(one.Name, name) {
			continue
		}
		present = true
		if one.Flags&net.FlagUp != 0 {
			up = true
		}
	}
	return present, up
}
