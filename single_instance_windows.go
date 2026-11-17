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
	name, _ := windows.UTF16PtrFromString(`Global\PulseCore_SingleInstance_Mutex`)
	h, err := windows.CreateMutex(nil, false, name)
	if err == windows.ERROR_ALREADY_EXISTS || (h != 0 && windows.GetLastError() == windows.ERROR_ALREADY_EXISTS) {
		if h != 0 {
			_ = windows.CloseHandle(h)
		}
		bringExistingInstanceToFront()
		return false
	}
	if err != nil && h == 0 {
		// If Global namespace is denied due to restricted token, fallback to Local namespace
		nameLocal, _ := windows.UTF16PtrFromString(`Local\PulseCore_SingleInstance_Mutex`)
		h, err = windows.CreateMutex(nil, false, nameLocal)
		if err == windows.ERROR_ALREADY_EXISTS || (h != 0 && windows.GetLastError() == windows.ERROR_ALREADY_EXISTS) {
			if h != 0 {
				_ = windows.CloseHandle(h)
			}
			bringExistingInstanceToFront()
			return false
		}
	}
	if h != 0 {
		singleInstanceMutex = h
	}
	return true
}

func releaseSingleInstance() {
	if singleInstanceMutex != 0 {
		_ = windows.CloseHandle(singleInstanceMutex)
		singleInstanceMutex = 0
	}
}

func isMainAppWindowTitle(txt string) bool {
	trimmed := strings.TrimSpace(txt)
	if trimmed == "PulseCore" {
		return true
	}
	if strings.HasPrefix(trimmed, "PulseCore") {
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "tray") ||
			strings.Contains(lower, "conflict") ||
			strings.Contains(lower, "hud") ||
			strings.Contains(lower, "overlay") ||
			strings.Contains(lower, "helper") {
			return false
		}
		return true
	}
	return false
}

var (
	user32DLL               = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW         = user32DLL.NewProc("FindWindowW")
	procGetWindowTextW      = user32DLL.NewProc("GetWindowTextW")
	procShowWindow          = user32DLL.NewProc("ShowWindow")
	procSetForegroundWindow = user32DLL.NewProc("SetForegroundWindow")
	procEnumWindowsSingle   = user32DLL.NewProc("EnumWindows")
)

var bringInstanceCb = syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
	var buf [256]uint16
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 256)
	txt := windows.UTF16ToString(buf[:])
	if isMainAppWindowTitle(txt) {
		procShowWindow.Call(hwnd, 9 /* SW_RESTORE */)
		procSetForegroundWindow.Call(hwnd)
		return 0
	}
	return 1
})

func bringExistingInstanceToFront() {
	// First attempt quick FindWindowW
	titlePtr, _ := windows.UTF16PtrFromString("PulseCore")
	hWnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	if hWnd != 0 {
		procShowWindow.Call(hWnd, 9 /* SW_RESTORE */)
		procSetForegroundWindow.Call(hWnd)
		return
	}
	// Fallback to EnumWindows to find any window starting with PulseCore
	procEnumWindowsSingle.Call(bringInstanceCb, 0)
}
