//go:build !windows

package main

import "fmt"

func killProcessNative(pid int) error {
	return fmt.Errorf("unsupported on non-windows")
}

func restartProcessNative(pid int) error {
	return fmt.Errorf("unsupported on non-windows")
}

func sampleGPUTopProcesses() []HardwareProcessItem {
	return nil
}

func sampleOverallGPUUsage(dedicatedMB float64) (pct, usedMB, sharedMB, totalDedicatedMB float64) {
	return 0, 0, 0, dedicatedMB
}
