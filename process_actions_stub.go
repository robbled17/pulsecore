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
