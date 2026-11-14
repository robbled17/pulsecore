//go:build windows

package main

import (
	"strconv"

	"golang.org/x/sys/windows/registry"
)

// isWindows11OrGreater returns true if the OS build number is >= 22000 (Windows 11).
func isWindows11OrGreater() bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	bStr, _, err := k.GetStringValue("CurrentBuildNumber")
	if err != nil || bStr == "" {
		bStr, _, _ = k.GetStringValue("CurrentBuild")
	}
	if bStr != "" {
		if n, err := strconv.Atoi(bStr); err == nil {
			return n >= 22000
		}
	}
	return false
}
