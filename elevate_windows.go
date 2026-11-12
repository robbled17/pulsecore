//go:build windows

package main

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// IsProcessElevated reports whether the current process has an elevated admin token.
func IsProcessElevated() bool {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}

// IsElevated is exposed to the frontend.
func (a *App) IsElevated() bool {
	return IsProcessElevated()
}

// RestartAsAdmin relaunches via native UAC (ShellExecute runas) - never cmd/PowerShell.
// If already elevated, returns immediately (no recursive relaunch spam).
func (a *App) RestartAsAdmin() error {
	return RelaunchAsAdmin()
}

// RelaunchAsAdmin shows the UAC dialog once, then exits the non-elevated instance.
func RelaunchAsAdmin() error {
	if IsProcessElevated() {
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return err
	}

	verbPtr, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	exePtr, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return err
	}
	cwdPtr, err := windows.UTF16PtrFromString(filepath.Dir(exePath))
	if err != nil {
		return err
	}

	// Native UAC prompt - no console windows.
	if err := windows.ShellExecute(0, verbPtr, exePtr, nil, cwdPtr, windows.SW_SHOWNORMAL); err != nil {
		return err
	}

	os.Exit(0)
	return nil
}
