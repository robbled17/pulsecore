package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modShell32           = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteW    = modShell32.NewProc("ShellExecuteW")
	elevatedCacheOnce    bool
	elevatedCacheValue   bool
)

// isProcessElevated reports whether the current process token is elevated.
func isProcessElevated() bool {
	if elevatedCacheOnce {
		return elevatedCacheValue
	}
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		elevatedCacheOnce = true
		elevatedCacheValue = false
		return false
	}
	defer token.Close()
	elevatedCacheOnce = true
	elevatedCacheValue = token.IsElevated()
	return elevatedCacheValue
}

// IsElevated is bound for the frontend sensor tip / restart-as-admin button.
func (a *App) IsElevated() bool {
	return isProcessElevated()
}

// RestartAsAdministrator relaunches pulsecore.exe with a UAC elevation prompt, then quits.
func (a *App) RestartAsAdministrator() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	if strings.TrimSpace(exe) == "" {
		return fmt.Errorf("empty executable path")
	}
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	dir, err := windows.UTF16PtrFromString(filepath.Dir(exe))
	if err != nil {
		return err
	}
	// SW_SHOWNORMAL = 1
	r, _, callErr := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		0,
		uintptr(unsafe.Pointer(dir)),
		1,
	)
	// ShellExecute returns value > 32 on success.
	if r <= 32 {
		if callErr != nil {
			return callErr
		}
		return fmt.Errorf("ShellExecute runas failed (%d)", r)
	}
	go a.QuitApp()
	return nil
}
