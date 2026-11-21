//go:build windows && !386

package sysproxy

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

const (
	optionConnectionOption = 75
	optionSettingsChanged  = 39
	optionRefresh          = 37
)

const (
	perConnFlags         = 1
	perConnProxyServer   = 2
	perConnProxyBypass   = 3
	perConnAutoconfigURL = 4
)

const (
	flagDirect       = 0x1
	flagProxy        = 0x2
	flagAutoProxyURL = 0x4
)

const (
	bypass       = "<local>"
	maxURLLength = 2084
)

type optionList struct {
	dwSize        uint32
	szConnection  uintptr
	dwOptionCount uint32
	dwOptionError uint32
	options       uintptr
}

type connectionOption struct {
	option uint32
	value  uint64
}

type settings struct {
	flags         uint64
	proxyServer   string
	proxyBypass   string
	autoconfigURL string
}

var (
	saveMu  sync.Mutex
	saved   settings
	savedOK bool
)

func internetSetOption(what uint32, value unsafe.Pointer, size uintptr) error {
	proc := syscall.NewLazyDLL("wininet.dll").NewProc("InternetSetOptionW")
	ok, _, err := proc.Call(0, uintptr(what), uintptr(value), size)
	if ok == 0 {
		return fmt.Errorf("InternetSetOption %d: %w", what, err)
	}
	return nil
}

func internetQueryOption(what uint32, value unsafe.Pointer, size *uint32) error {
	proc := syscall.NewLazyDLL("wininet.dll").NewProc("InternetQueryOptionW")
	ok, _, err := proc.Call(0, uintptr(what), uintptr(value), uintptr(unsafe.Pointer(size)))
	if ok == 0 {
		return fmt.Errorf("InternetQueryOption %d: %w", what, err)
	}
	return nil
}

func query() (settings, bool) {
	server := make([]uint16, maxURLLength)
	proxyBypass := make([]uint16, maxURLLength)
	autoconfigURL := make([]uint16, maxURLLength)
	options := []connectionOption{
		{option: perConnFlags},
		{option: perConnProxyServer, value: uint64(uintptr(unsafe.Pointer(&server[0])))},
		{option: perConnProxyBypass, value: uint64(uintptr(unsafe.Pointer(&proxyBypass[0])))},
		{option: perConnAutoconfigURL, value: uint64(uintptr(unsafe.Pointer(&autoconfigURL[0])))},
	}
	list := optionList{
		dwSize:        uint32(unsafe.Sizeof(optionList{})),
		dwOptionCount: uint32(len(options)),
		options:       uintptr(unsafe.Pointer(&options[0])),
	}
	size := uint32(unsafe.Sizeof(list))
	err := internetQueryOption(optionConnectionOption, unsafe.Pointer(&list), &size)
	runtime.KeepAlive(options)
	runtime.KeepAlive(server)
	runtime.KeepAlive(proxyBypass)
	runtime.KeepAlive(autoconfigURL)
	if err != nil {
		return settings{}, false
	}
	return settings{
		flags:         options[0].value,
		proxyServer:   syscall.UTF16ToString(server),
		proxyBypass:   syscall.UTF16ToString(proxyBypass),
		autoconfigURL: syscall.UTF16ToString(autoconfigURL),
	}, true
}

func write(into settings) error {
	var alive [][]uint16
	options := []connectionOption{{option: perConnFlags, value: into.flags}}
	add := func(option uint32, text string) {
		if text == "" {
			return
		}
		value, _ := syscall.UTF16FromString(text)
		alive = append(alive, value)
		options = append(options, connectionOption{option: option, value: uint64(uintptr(unsafe.Pointer(&value[0])))})
	}
	add(perConnProxyServer, into.proxyServer)
	add(perConnProxyBypass, into.proxyBypass)
	add(perConnAutoconfigURL, into.autoconfigURL)
	list := optionList{
		dwSize:        uint32(unsafe.Sizeof(optionList{})),
		dwOptionCount: uint32(len(options)),
		options:       uintptr(unsafe.Pointer(&options[0])),
	}
	err := internetSetOption(optionConnectionOption, unsafe.Pointer(&list), unsafe.Sizeof(list))
	runtime.KeepAlive(options)
	runtime.KeepAlive(alive)
	if err != nil {
		return err
	}

	if err := internetSetOption(optionSettingsChanged, nil, 0); err != nil {
		return err
	}
	return internetSetOption(optionRefresh, nil, 0)
}

func save() {
	saveMu.Lock()
	defer saveMu.Unlock()
	saved, savedOK = query()
}

func Enable(addr string) error {
	save()
	return write(settings{flags: flagDirect | flagProxy, proxyServer: addr, proxyBypass: bypass})
}

func EnablePac(url string) error {
	save()
	return write(settings{flags: flagDirect | flagAutoProxyURL, autoconfigURL: url})
}

func clear() error {
	saveMu.Lock()
	state, ok := saved, savedOK
	saveMu.Unlock()
	if !ok {
		return nil
	}
	return write(state)
}
func reset() error {
	return write(settings{flags: flagDirect})
}
