//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modRstrtMgr              = windows.NewLazySystemDLL("rstrtmgr.dll")
	procRmStartSession       = modRstrtMgr.NewProc("RmStartSession")
	procRmRegisterResources  = modRstrtMgr.NewProc("RmRegisterResources")
	procRmGetList            = modRstrtMgr.NewProc("RmGetList")
	procRmEndSession         = modRstrtMgr.NewProc("RmEndSession")

	modKernel32UL              = windows.NewLazySystemDLL("kernel32.dll")
	procULQueryFullProcessImageNameW = modKernel32UL.NewProc("QueryFullProcessImageNameW")
)

type rmUniqueProcess struct {
	DwProcessId      uint32
	ProcessStartTime windows.Filetime
}

type rmProcessInfo struct {
	Process             rmUniqueProcess
	StrAppName          [256]uint16
	StrServiceShortName [64]uint16
	ApplicationType     uint32
	AppStatus           uint32
	TSSessionId         uint32
	BRestartable        int32
}

// LockProcessInfo describes a process locking a file or folder.
type LockProcessInfo struct {
	PID         uint32 `json:"pid"`
	AppName     string `json:"appName"`
	ProcessName string `json:"processName"`
	ExePath     string `json:"exePath"`
}

// FindLockingProcesses uses Windows Restart Manager API to identify blocking processes.
func FindLockingProcesses(targetPath string) ([]LockProcessInfo, error) {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return nil, fmt.Errorf("empty path")
	}

	// Collect file paths to register
	var filePaths []string
	info, err := os.Stat(targetPath)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		// Collect top-level files and executables/dlls (limit to 64 to keep fast)
		entries, err := os.ReadDir(targetPath)
		if err == nil {
			for _, entry := range entries {
				full := filepath.Join(targetPath, entry.Name())
				filePaths = append(filePaths, full)
				if len(filePaths) >= 64 {
					break
				}
			}
		}
		// Also register the folder itself
		filePaths = append(filePaths, targetPath)
	} else {
		filePaths = append(filePaths, targetPath)
	}

	var sessionHandle uint32
	var sessionKey [32]uint16
	r, _, _ := procRmStartSession.Call(
		uintptr(unsafe.Pointer(&sessionHandle)),
		0,
		uintptr(unsafe.Pointer(&sessionKey[0])),
	)
	if r != 0 {
		return nil, fmt.Errorf("RmStartSession error: %d", r)
	}
	defer procRmEndSession.Call(uintptr(sessionHandle))

	// Convert file paths to UTF-16 pointers
	filePtrs := make([]*uint16, 0, len(filePaths))
	for _, p := range filePaths {
		ptr, err := windows.UTF16PtrFromString(p)
		if err == nil {
			filePtrs = append(filePtrs, ptr)
		}
	}
	if len(filePtrs) == 0 {
		return nil, nil
	}

	r, _, _ = procRmRegisterResources.Call(
		uintptr(sessionHandle),
		uintptr(len(filePtrs)),
		uintptr(unsafe.Pointer(&filePtrs[0])),
		0,
		0,
		0,
		0,
	)
	if r != 0 {
		return nil, fmt.Errorf("RmRegisterResources error: %d", r)
	}

	var nProcInfoNeeded uint32
	var nProcInfo uint32 = 16
	procInfos := make([]rmProcessInfo, nProcInfo)
	var rebootReasons uint32

	r, _, _ = procRmGetList.Call(
		uintptr(sessionHandle),
		uintptr(unsafe.Pointer(&nProcInfoNeeded)),
		uintptr(unsafe.Pointer(&nProcInfo)),
		uintptr(unsafe.Pointer(&procInfos[0])),
		uintptr(unsafe.Pointer(&rebootReasons)),
	)

	// ERROR_MORE_DATA = 234
	if r == 234 && nProcInfoNeeded > nProcInfo {
		nProcInfo = nProcInfoNeeded
		procInfos = make([]rmProcessInfo, nProcInfo)
		r, _, _ = procRmGetList.Call(
			uintptr(sessionHandle),
			uintptr(unsafe.Pointer(&nProcInfoNeeded)),
			uintptr(unsafe.Pointer(&nProcInfo)),
			uintptr(unsafe.Pointer(&procInfos[0])),
			uintptr(unsafe.Pointer(&rebootReasons)),
		)
	}

	if r != 0 && r != 234 {
		return nil, fmt.Errorf("RmGetList error: %d", r)
	}

	out := make([]LockProcessInfo, 0, nProcInfo)
	seen := map[uint32]bool{}

	for i := 0; i < int(nProcInfo); i++ {
		pi := procInfos[i]
		pid := pi.Process.DwProcessId
		if pid <= 4 || seen[pid] {
			continue
		}
		seen[pid] = true

		appName := windows.UTF16ToString(pi.StrAppName[:])
		exePath, procName := getProcessNameAndPath(pid)
		if procName == "" {
			procName = appName
		}
		if appName == "" {
			appName = procName
		}

		out = append(out, LockProcessInfo{
			PID:         pid,
			AppName:     appName,
			ProcessName: procName,
			ExePath:     exePath,
		})
	}

	return out, nil
}

func getProcessNameAndPath(pid uint32) (exePath string, procName string) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", ""
	}
	defer windows.CloseHandle(h)

	var buf [1024]uint16
	size := uint32(len(buf))
	r, _, _ := procULQueryFullProcessImageNameW.Call(
		uintptr(h),
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r != 0 && size > 0 {
		exePath = windows.UTF16ToString(buf[:size])
		procName = filepath.Base(exePath)
	}
	return
}

// isProtectedSystemProcess returns true for OS core processes that must never be terminated.
func isProtectedSystemProcess(procName string, pid uint32) bool {
	if pid <= 4 {
		return true
	}
	low := strings.ToLower(strings.TrimSpace(procName))
	switch low {
	case "explorer.exe", "csrss.exe", "lsass.exe", "services.exe",
		"smss.exe", "wininit.exe", "winlogon.exe", "system",
		"svchost.exe", "dwm.exe", "pulsecore.exe", "fontdrvhost.exe":
		return true
	}
	return false
}

// TerminateLockingProcesses terminates non-critical blocking processes.
func TerminateLockingProcesses(procs []LockProcessInfo) (terminated int, skipped int, err error) {
	for _, p := range procs {
		if isProtectedSystemProcess(p.ProcessName, p.PID) {
			skipped++
			continue
		}

		h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, p.PID)
		if err != nil {
			skipped++
			continue
		}
		if err := windows.TerminateProcess(h, 1); err == nil {
			terminated++
		} else {
			skipped++
		}
		windows.CloseHandle(h)
	}
	return terminated, skipped, nil
}

// UnlockPath finds locking processes for targetPath and terminates them.
func UnlockPath(targetPath string) (int, error) {
	procs, err := FindLockingProcesses(targetPath)
	if err != nil {
		return 0, err
	}
	if len(procs) == 0 {
		return 0, nil
	}
	term, _, err := TerminateLockingProcesses(procs)
	return term, err
}

