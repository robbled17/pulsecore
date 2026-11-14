//go:build windows

package main

import (
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var singleInstanceMutex windows.Handle

func acquireSingleInstance() bool {
	name, _ := windows.UTF16PtrFromString(`Global\PulseCoreSingleInstanceMutex`)
	h, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		nameLocal, _ := windows.UTF16PtrFromString(`Local\PulseCoreSingleInstanceMutex`)
		h, err = windows.CreateMutex(nil, false, nameLocal)
	}
	if err == windows.ERROR_ALREADY_EXISTS {
		if h != 0 {
			_ = windows.CloseHandle(h)
		}
		bringExistingInstanceToFront()
		return false
	}
	singleInstanceMutex = h
	return true
}

func bringExistingInstanceToFront() {
	user32 := windows.NewLazySystemDLL("user32.dll")
	procEnumWindows := user32.NewProc("EnumWindows")
	procGetWindowTextW := user32.NewProc("GetWindowTextW")
	procShowWindow := user32.NewProc("ShowWindow")
	procSetForegroundWindow := user32.NewProc("SetForegroundWindow")

	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var buf [256]uint16
		procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 256)
		txt := windows.UTF16ToString(buf[:])
		if strings.Contains(txt, "PulseCore") {
			procShowWindow.Call(hwnd, 9 /* SW_RESTORE */)
			procSetForegroundWindow.Call(hwnd)
			return 0
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
}
