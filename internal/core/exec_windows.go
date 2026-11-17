//go:build windows

package core

import (
	"log"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NEW_CONSOLE,
	}
}

func Hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
}

func stop(cmd *exec.Cmd) {
	pid := uint32(cmd.Process.Pid)
	console := windows.NewLazySystemDLL("kernel32.dll")
	attach := console.NewProc("AttachConsole")
	detach := console.NewProc("FreeConsole")
	ok, _, err := attach.Call(uintptr(pid))
	if ok == 0 {
		if err != windows.ERROR_ACCESS_DENIED {
			return
		}
		detach.Call()
		if ok, _, _ = attach.Call(uintptr(pid)); ok == 0 {
			return
		}
	}
	windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, pid)
	detach.Call()
}

func follow(cmd *exec.Cmd) {
	job, err := jobHandle()
	if err != nil {
		log.Print("no job for the kernel: ", err)
		return
	}
	handle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_INFORMATION, false, uint32(cmd.Process.Pid))
	if err != nil {
		log.Print("no handle for the kernel: ", err)
		return
	}
	defer windows.CloseHandle(handle)
	if err := windows.AssignProcessToJobObject(job, handle); err != nil {
		log.Print("the kernel follows TunGo only by luck: ", err)
	}
}

var (
	jobOnce sync.Once
	job     windows.Handle
	jobErr  error
)

func jobHandle() (windows.Handle, error) {
	jobOnce.Do(func() {
		handle, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			jobErr = err
			return
		}
		limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		_, err = windows.SetInformationJobObject(handle, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)))
		if err != nil {
			windows.CloseHandle(handle)
			jobErr = err
			return
		}
		job = handle
	})
	return job, jobErr
}
