//go:build windows && (amd64 || arm64)

package sysproxy

import (
	"fmt"
	"runtime"
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

const bypass = "<local>"

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

func internetSetOption(what uint32, value unsafe.Pointer, size uintptr) error {
	proc := syscall.NewLazyDLL("wininet.dll").NewProc("InternetSetOptionW")
	ok, _, err := proc.Call(0, uintptr(what), uintptr(value), size)
	if ok == 0 {
		return fmt.Errorf("InternetSetOption %d: %w", what, err)
	}
	return nil
}

func write(addr, url string) error {
	bypassUTF16, _ := syscall.UTF16FromString(bypass)
	var addrUTF16, urlUTF16 []uint16
	var options []connectionOption
	switch {
	case url != "":
		urlUTF16, _ = syscall.UTF16FromString(url)
		options = []connectionOption{
			{option: perConnFlags, value: uint64(flagDirect | flagAutoProxyURL)},
			{option: perConnAutoconfigURL, value: uint64(uintptr(unsafe.Pointer(&urlUTF16[0])))},
		}
	case addr != "":
		addrUTF16, _ = syscall.UTF16FromString(addr)
		options = []connectionOption{
			{option: perConnFlags, value: uint64(flagDirect | flagProxy)},
			{option: perConnProxyServer, value: uint64(uintptr(unsafe.Pointer(&addrUTF16[0])))},
			{option: perConnProxyBypass, value: uint64(uintptr(unsafe.Pointer(&bypassUTF16[0])))},
		}
	default:
		options = []connectionOption{
			{option: perConnFlags, value: uint64(flagDirect)},
		}
	}
	list := optionList{
		dwSize:        uint32(unsafe.Sizeof(optionList{})),
		dwOptionCount: uint32(len(options)),
		options:       uintptr(unsafe.Pointer(&options[0])),
	}
	err := internetSetOption(optionConnectionOption, unsafe.Pointer(&list), unsafe.Sizeof(list))
	runtime.KeepAlive(options)
	runtime.KeepAlive(addrUTF16)
	runtime.KeepAlive(urlUTF16)
	runtime.KeepAlive(bypassUTF16)
	if err != nil {
		return err
	}

	if err := internetSetOption(optionSettingsChanged, nil, 0); err != nil {
		return err
	}
	return internetSetOption(optionRefresh, nil, 0)
}

func Enable(addr string) error { return write(addr, "") }

func EnablePac(url string) error { return write("", url) }

func clear() error { return write("", "") }
