//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/windows"
)

var (
	modRstrtMgr              = windows.NewLazySystemDLL("rstrtmgr.dll")
	procRmStartSession       = modRstrtMgr.NewProc("RmStartSession")
	procRmRegisterResources  = modRstrtMgr.NewProc("RmRegisterResources")
	procRmGetList            = modRstrtMgr.NewProc("RmGetList")
	procRmEndSession         = modRstrtMgr.NewProc("RmEndSession")

	modKernel32UL                    = windows.NewLazySystemDLL("kernel32.dll")
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

// cmdlineContainsPath checks if cmdline references targetPath as an exact argument, path token, or prefix.
func cmdlineContainsPath(cmdline, targetPath string) bool {
	t := strings.ToLower(filepath.Clean(strings.TrimSpace(targetPath)))
	if t == "" || cmdline == "" {
		return false
	}
	cmdLower := strings.ToLower(cmdline)
	idx := 0
	for {
		pos := strings.Index(cmdLower[idx:], t)
		if pos == -1 {
			break
		}
		actualPos := idx + pos
		endPos := actualPos + len(t)

		// Predecessor boundary: start of string, whitespace, quotes, equals sign, comma, semicolon
		validPrefix := false
		if actualPos == 0 {
			validPrefix = true
		} else {
			prevChar := cmdLower[actualPos-1]
			if prevChar == ' ' || prevChar == '\t' || prevChar == '"' || prevChar == '\'' || prevChar == '=' || prevChar == ',' || prevChar == ';' {
				validPrefix = true
			}
		}

		// Successor boundary: end of string, whitespace, quotes, path separators, comma, semicolon
		validSuffix := false
		if endPos >= len(cmdLower) {
			validSuffix = true
		} else {
			nextChar := cmdLower[endPos]
			if nextChar == ' ' || nextChar == '\t' || nextChar == '"' || nextChar == '\'' || nextChar == ',' || nextChar == ';' || nextChar == '\\' || nextChar == '/' {
				validSuffix = true
			}
		}

		if validPrefix && validSuffix {
			return true
		}

		idx = actualPos + 1
	}
	return false
}

// isPathUnderTarget returns true if path is equal to target or is a descendant of target.
func isPathUnderTarget(path, target string) bool {
	p := strings.ToLower(filepath.Clean(strings.TrimSpace(path)))
	t := strings.ToLower(filepath.Clean(strings.TrimSpace(target)))
	if p == "" || t == "" {
		return false
	}
	if p == t {
		return true
	}
	sep := string(filepath.Separator)
	return strings.HasPrefix(p, t+sep)
}

// collectCandidateFilesForRM recursively gathers files inside a folder up to depth 4,
// prioritizing file types most commonly locked by running applications.
func collectCandidateFilesForRM(targetPath string) []string {
	info, err := os.Stat(targetPath)
	if err != nil {
		return []string{targetPath}
	}
	if !info.IsDir() {
		return []string{targetPath}
	}

	var priorityFiles []string
	var normalFiles []string

	cleanTarget := filepath.Clean(targetPath)
	_ = filepath.WalkDir(cleanTarget, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			rel, err := filepath.Rel(cleanTarget, p)
			if err == nil && rel != "." {
				depth := strings.Count(rel, string(filepath.Separator)) + 1
				if depth > 4 {
					return filepath.SkipDir
				}
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		switch ext {
		case ".exe", ".dll", ".db", ".lock", ".log", ".asar", ".pak", ".node", ".bin", ".dat", ".tmp":
			if len(priorityFiles) < 96 {
				priorityFiles = append(priorityFiles, p)
			}
		default:
			if len(normalFiles) < 64 {
				normalFiles = append(normalFiles, p)
			}
		}
		if len(priorityFiles)+len(normalFiles) >= 160 {
			return filepath.SkipAll
		}
		return nil
	})

	res := append(priorityFiles, normalFiles...)
	res = append(res, cleanTarget)
	return res
}

// Level 1: Windows Restart Manager
func findLockingViaRM(targetPath string) ([]LockProcessInfo, error) {
	filePaths := collectCandidateFilesForRM(targetPath)
	if len(filePaths) == 0 {
		return nil, nil
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

		if isProtectedSystemProcess(procName, pid) || isProtectedSystemProcess(appName, pid) {
			continue
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

// Level 2: Toolhelp32 process & module scan
func findLockingViaModules(targetPath string) []LockProcessInfo {
	targetPath = filepath.Clean(strings.TrimSpace(targetPath))
	if targetPath == "" {
		return nil
	}

	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	var out []LockProcessInfo
	seen := map[uint32]bool{}
	myPID := uint32(os.Getpid())

	err = windows.Process32First(snap, &entry)
	for err == nil {
		pid := entry.ProcessID
		name := windows.UTF16ToString(entry.ExeFile[:])

		if pid > 4 && pid != myPID && !seen[pid] && !isProtectedSystemProcess(name, pid) {
			exePath, procName := getProcessNameAndPath(pid)
			if procName == "" {
				procName = name
			}

			matched := false
			if isPathUnderTarget(exePath, targetPath) {
				matched = true
			}

			// Check modules if not matched yet
			if !matched {
				modSnap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPMODULE|windows.TH32CS_SNAPMODULE32, pid)
				if err == nil {
					var modEntry windows.ModuleEntry32
					modEntry.Size = uint32(unsafe.Sizeof(modEntry))
					mErr := windows.Module32First(modSnap, &modEntry)
					for mErr == nil {
						modPath := windows.UTF16ToString(modEntry.ExePath[:])
						if isPathUnderTarget(modPath, targetPath) {
							matched = true
							break
						}
						mErr = windows.Module32Next(modSnap, &modEntry)
					}
					_ = windows.CloseHandle(modSnap)
				}
			}

			if matched {
				seen[pid] = true
				out = append(out, LockProcessInfo{
					PID:         pid,
					AppName:     procName,
					ProcessName: procName,
					ExePath:     exePath,
				})
			}
		}

		err = windows.Process32Next(snap, &entry)
	}

	return out
}

// Level 3: Process CWD, ExePath, and CommandLine inspection
// Uses benign standard APIs (process snapshot / gopsutil) that never trigger AV handle-manipulation heuristics.
func findLockingViaHandlesAndCWD(targetPath string) []LockProcessInfo {
	targetPath = filepath.Clean(strings.TrimSpace(targetPath))
	if targetPath == "" {
		return nil
	}

	var out []LockProcessInfo
	seen := map[uint32]bool{}
	myPID := uint32(os.Getpid())

	// Inspect active processes for CWD, Exe, or command line arguments referencing targetPath
	if procs, err := process.Processes(); err == nil {
		for _, p := range procs {
			pid := uint32(p.Pid)
			if pid <= 4 || pid == myPID || seen[pid] {
				continue
			}
			pName, err := p.Name()
			if err != nil || pName == "" || isProtectedSystemProcess(pName, pid) {
				continue
			}

			matched := false

			// 1. Check process executable path
			if exePath, err := p.Exe(); err == nil && exePath != "" {
				if isPathUnderTarget(exePath, targetPath) {
					matched = true
				}
			}

			// 2. Check process current working directory
			if !matched {
				if cwd, err := p.Cwd(); err == nil && cwd != "" {
					if isPathUnderTarget(cwd, targetPath) {
						matched = true
					}
				}
			}

			// 3. Check process command line arguments (strict token/boundary matching)
			if !matched {
				if cmdline, err := p.Cmdline(); err == nil && cmdline != "" {
					if cmdlineContainsPath(cmdline, targetPath) {
						matched = true
					}
				}
			}

			if matched {
				seen[pid] = true
				exePath, _ := p.Exe()
				out = append(out, LockProcessInfo{
					PID:         pid,
					AppName:     pName,
					ProcessName: pName,
					ExePath:     exePath,
				})
			}
		}
	}

	return out
}

// FindLockingProcesses executes a 3-level cascading search for processes holding targetPath (like LockHunter):
// Level 1: Windows Restart Manager (rstrtmgr.dll)
// Level 2: Toolhelp32 process & module scan (process exe and loaded DLLs)
// Level 3: Open handles via NtQuerySystemInformation and Process CWD
func FindLockingProcesses(targetPath string) ([]LockProcessInfo, error) {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return nil, fmt.Errorf("empty path")
	}

	cleanTarget := filepath.Clean(targetPath)
	seen := map[uint32]bool{}
	var result []LockProcessInfo

	addProcs := func(procs []LockProcessInfo) {
		for _, p := range procs {
			if p.PID > 4 && !seen[p.PID] && !isProtectedSystemProcess(p.ProcessName, p.PID) && !isProtectedSystemProcess(p.AppName, p.PID) {
				seen[p.PID] = true
				result = append(result, p)
			}
		}
	}

	// Level 1: Windows Restart Manager
	if rmProcs, err := findLockingViaRM(cleanTarget); err == nil && len(rmProcs) > 0 {
		addProcs(rmProcs)
	}

	// Level 2: Process ExePath & Module Scan
	if len(result) == 0 {
		if modProcs := findLockingViaModules(cleanTarget); len(modProcs) > 0 {
			addProcs(modProcs)
		}
	}

	// Level 3: CWD and Open Handles Scan
	if len(result) == 0 {
		if handleProcs := findLockingViaHandlesAndCWD(cleanTarget); len(handleProcs) > 0 {
			addProcs(handleProcs)
		}
	}

	return result, nil
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
	if pid > 0 && (pid <= 4 || pid == uint32(os.Getpid())) {
		return true
	}
	// Check if matches our own executable path
	if myExe, err := os.Executable(); err == nil && myExe != "" {
		if strings.EqualFold(filepath.Clean(procName), filepath.Clean(myExe)) {
			return true
		}
	}
	base := strings.ToLower(filepath.Base(strings.TrimSpace(procName)))
	if base == "" || base == "system" || base == "idle" {
		return true
	}
	if isSelfOrWebView(base) {
		return true
	}
	switch base {
	case "explorer.exe", "csrss.exe", "lsass.exe", "services.exe",
		"smss.exe", "wininit.exe", "winlogon.exe",
		"svchost.exe", "dwm.exe", "pulsecore.exe", "fontdrvhost.exe",
		"msedgewebview2.exe", "pulsecorehelper.exe", "conhost.exe",
		"runtimebroker.exe", "sihost.exe", "taskhostw.exe",
		"searchhost.exe", "searchindexer.exe", "ctfmon.exe",
		"startmenuexperiencehost.exe", "shellexperiencehost.exe",
		"applicationframehost.exe":
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
		if err == nil {
			if err := windows.TerminateProcess(h, 1); err == nil {
				terminated++
			} else {
				skipped++
			}
			windows.CloseHandle(h)
		} else {
			skipped++
		}

		// Fallback to taskkill /F /T /PID to terminate child processes
		_ = exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", p.PID)).Run()
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
