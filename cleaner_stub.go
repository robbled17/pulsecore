//go:build !windows

package main

import "time"

func emptyRecycleBinNative() error { return nil }
func emptyRecycleBinNativeDrive(drive string) error { return nil }
func queryRecycleBinInfo() (int64, int64, error) { return 0, 0, nil }
func queryRecycleBinInfoDrive(drive string) (int64, int64, error) { return 0, 0, nil }
func queryRecycleBinBytes() int64 { return 0 }
func runElevatedPowerShell(script string) (string, error) { return "", nil }
func fastDirSizeWin32(root string, deadline time.Time, maxDepth int) int64 {
	return dirSizeBudget(root, deadline, maxDepth)
}
