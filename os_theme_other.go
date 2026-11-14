//go:build !windows

package main

func isWindows11() bool {
	return false
}

func (a *App) IsWindows11() bool {
	return false
}
