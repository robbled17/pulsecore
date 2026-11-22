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
	modShell32        = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteW = modShell32.NewProc("ShellExecuteW")
)

// IsElevated is bound for the frontend sensor tip / restart-as-admin button.
// Uses the shared isProcessElevated helper in diagnostics_win.go.
func (a *App) IsElevated() bool {
	return isProcessElevated()
}

// ensureElevatedOrRelaunch returns true when the process is already elevated and
// may continue. When not elevated, it relaunches via ShellExecuteW "runas" and
// returns false so the caller exits before acquiring the single-instance mutex.
func ensureElevatedOrRelaunch() bool {
	if isProcessElevated() {
		return true
	}
	if err := relaunchElevated(); err != nil {
		println("Elevation required:", err.Error())
		return false
	}
	return false
}

// RestartAsAdministrator relaunches pulsecore.exe with a UAC elevation prompt, then quits.
func (a *App) RestartAsAdministrator() error {
	if err := relaunchElevated(); err != nil {
		return err
	}
	go a.QuitApp()
	return nil
}

func relaunchElevated() error {
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
	return nil
}
