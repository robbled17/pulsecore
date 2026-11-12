//go:build !windows

package main

func runPawnIOSetup(setupPath string, action string) error { return nil }
func shouldSkipPawnIOInstall() bool                       { return true }
func uninstallPawnIOOwned()                                {}
