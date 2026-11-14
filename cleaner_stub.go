//go:build !windows

package main

func emptyRecycleBinNative() error { return nil }
func queryRecycleBinInfo() (int64, int64, error) { return 0, 0, nil }
func queryRecycleBinBytes() int64 { return 0 }
func runElevatedPowerShell(script string) (string, error) { return "", nil }
