//go:build windows

package core

import (
	"log"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

const (
	createNewConsole      = 0x00000010
	createNewProcessGroup = 0x00000200
	ctrlBreakEvent        = 1
	errorAccessDenied     = 5
)

func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNewProcessGroup | createNewConsole,
	}
}

func stop(cmd *exec.Cmd) {
	pid := uintptr(cmd.Process.Pid)
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	attach := kernel32.NewProc("AttachConsole")
	detach := kernel32.NewProc("FreeConsole")
	send := kernel32.NewProc("GenerateConsoleCtrlEvent")

	ok, _, err := attach.Call(pid)
	if ok == 0 {
		if errno, isErrno := err.(syscall.Errno); !isErrno || errno != errorAccessDenied {
			return
		}

		detach.Call()
		if ok, _, _ = attach.Call(pid); ok == 0 {
			return
		}
	}
	send.Call(ctrlBreakEvent, pid)
	detach.Call()
}

func follow(cmd *exec.Cmd) {
	job, err := jobHandle()
	if err != nil {
		log.Print("no job for the kernel: ", err)
		return
	}
	handle, err := syscall.OpenProcess(processSetQuota|processTerminate|processQueryInformation,
		false, uint32(cmd.Process.Pid))
	if err != nil {
		log.Print("no handle for the kernel: ", err)
		return
	}
	defer syscall.CloseHandle(handle)
	if err := assign(job, handle); err != nil {
		log.Print("the kernel follows TunGo only by luck: ", err)
	}
}

var (
	jobOnce sync.Once
	job     syscall.Handle
	jobErr  error
)

func jobHandle() (syscall.Handle, error) {
	jobOnce.Do(func() {
		kernel32 := syscall.NewLazyDLL("kernel32.dll")
		create := kernel32.NewProc("CreateJobObjectW")
		set := kernel32.NewProc("SetInformationJobObject")

		handle, _, err := create.Call(0, 0, 0)
		if handle == 0 {
			jobErr = err
			return
		}
		limits := jobObjectExtendedLimitInformation{}
		limits.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
		if ok, _, err := set.Call(handle, jobObjectExtendedLimitInformationClass,
			uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits)); ok == 0 {
			syscall.CloseHandle(syscall.Handle(handle))
			jobErr = err
			return
		}
		job = syscall.Handle(handle)
	})
	return job, jobErr
}

func assign(job syscall.Handle, process syscall.Handle) error {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	fn := kernel32.NewProc("AssignProcessToJobObject")
	if ok, _, err := fn.Call(uintptr(job), uintptr(process)); ok == 0 {
		return err
	}
	return nil
}

const (
	jobObjectExtendedLimitInformationClass = 9
	jobObjectLimitKillOnJobClose           = 0x2000

	processSetQuota         = 0x0100
	processTerminate        = 0x0001
	processQueryInformation = 0x0400
)

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectExtendedLimitInformation struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}
