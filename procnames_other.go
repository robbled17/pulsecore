//go:build !windows

package main

func getProcessVersionInfo(exePath string) string {
	return ""
}
