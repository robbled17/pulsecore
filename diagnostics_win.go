package main

import (
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// isProcessElevated reports whether the current process token is elevated (admin).
// Fail-soft: returns false on any probe error.
func isProcessElevated() bool {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}

// openUninstallKeyReadable soft-probes HKCU Uninstall (always readable for the user).
func openUninstallKeyReadable() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Uninstall`, registry.READ)
	if err != nil {
		return false
	}
	_ = k.Close()
	return true
}
