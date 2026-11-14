//go:build !windows

package main

// LockProcessInfo describes a process locking a file or folder.
type LockProcessInfo struct {
	PID         uint32 `json:"pid"`
	AppName     string `json:"appName"`
	ProcessName string `json:"processName"`
	ExePath     string `json:"exePath"`
}

func FindLockingProcesses(targetPath string) ([]LockProcessInfo, error) {
	return nil, nil
}

func TerminateLockingProcesses(procs []LockProcessInfo) (int, int, error) {
	return 0, 0, nil
}
