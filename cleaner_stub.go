//go:build !windows

package main

func emptyRecycleBinNative() error { return nil }
func runElevatedPowerShell(script string) (string, error) { return "", nil }
