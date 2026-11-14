//go:build !windows

package main

// StartConflictWatcher stub on non-Windows
func (a *App) StartConflictWatcher() {}

// DestroyLingeringConflictWindows stub on non-Windows
func DestroyLingeringConflictWindows() {}
