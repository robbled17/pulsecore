package main

import (
	"golang.org/x/sys/windows/registry"
)

// openUninstallKeyReadable soft-probes HKCU Uninstall (always readable for the user).
// isProcessElevated lives in admin.go (single shared definition).
func openUninstallKeyReadable() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Uninstall`, registry.READ)
	if err != nil {
		return false
	}
	_ = k.Close()
	return true
}
