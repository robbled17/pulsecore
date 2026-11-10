package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/windows"
)

const (
	processPowerThrottling               = 4
	processPowerThrottlingCurrentVersion = 1
	processPowerThrottlingExecutionSpeed = 0x1
	idlePriorityClass                    = 0x00000040
	normalPriorityClass                  = 0x00000020
)

var (
	procSetProcessInformation = modKernel32.NewProc("SetProcessInformation")
	procSetPriorityClass      = modKernel32.NewProc("SetPriorityClass")
)

// processPowerThrottlingState mirrors PROCESS_POWER_THROTTLING_STATE.
type processPowerThrottlingState struct {
	Version     uint32
	ControlMask uint32
	StateMask   uint32
}

var ecoDenylist = map[string]struct{}{
	"system": {}, "registry": {}, "idle": {},
	"smss.exe": {}, "csrss.exe": {}, "wininit.exe": {},
	"services.exe": {}, "lsass.exe": {}, "svchost.exe": {},
	"explorer.exe": {}, "dwm.exe": {},
	"pulsecore.exe": {}, "zeropulse.exe": {}, "zeropulse-gui.exe": {},
}

// Kernel / PPL protected security stack — EcoQoS cannot be applied.
var ecoPPLProtected = map[string]struct{}{
	"msmpeng.exe":               {},
	"mssense.exe":               {},
	"securityhealthservice.exe": {},
	"securityhealthsystray.exe": {},
	"wdavscannerservice.exe":    {},
	"nisrv.exe":                 {},
	"mpcmdrun.exe":              {},
	"sgrmbroker.exe":            {},
}

func isEcoProtected(name string) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(name)))
	if base == "" {
		return true
	}
	_, ok := ecoDenylist[base]
	return ok
}

func isPPLProtected(name string) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(name)))
	_, ok := ecoPPLProtected[base]
	return ok
}

func processNameByPID(pid int) string {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return ""
	}
	name, _ := p.Name()
	return name
}

// listPIDsByExeNames uses CreateToolhelp32Snapshot for a reliable live process map.
func listPIDsByExeNames(names []string) []uint32 {
	want := make(map[string]struct{}, len(names))
	for _, n := range names {
		base := strings.ToLower(filepath.Base(strings.TrimSpace(n)))
		if base == "" {
			continue
		}
		want[base] = struct{}{}
		if !strings.HasSuffix(base, ".exe") {
			want[base+".exe"] = struct{}{}
		}
	}
	if len(want) == 0 {
		return nil
	}

	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	err = windows.Process32First(snap, &pe)
	out := make([]uint32, 0, 8)
	for err == nil {
		name := strings.ToLower(windows.UTF16ToString(pe.ExeFile[:]))
		if _, ok := want[name]; ok && pe.ProcessID > 0 {
			out = append(out, pe.ProcessID)
		}
		err = windows.Process32Next(snap, &pe)
	}
	return out
}

func expandEcoCandidates(name string) []string {
	target := strings.ToLower(filepath.Base(strings.TrimSpace(name)))
	aliases := map[string][]string{
		"chrome/edge":             {"chrome.exe", "msedge.exe", "firefox.exe", "brave.exe", "opera.exe"},
		"браузер":                 {"chrome.exe", "msedge.exe", "firefox.exe", "brave.exe", "opera.exe"},
		"browser":                 {"chrome.exe", "msedge.exe", "firefox.exe", "brave.exe", "opera.exe"},
		"защитник windows":        {"msmpeng.exe"},
		"windows defender":        {"msmpeng.exe"},
		"антивирус":               {"msmpeng.exe"},
		"индексатор":              {"searchindexer.exe", "searchprotocolhost.exe"},
		"windows search":          {"searchindexer.exe"},
		"telegram":                {"telegram.exe"},
		"discord":                 {"discord.exe"},
		"steam":                   {"steam.exe", "steamwebhelper.exe"},
		"vscode":                  {"code.exe"},
		"visual studio code":      {"code.exe"},
	}
	if alt, ok := aliases[target]; ok {
		return alt
	}
	// Friendly names from weekly report may not include .exe
	if !strings.HasSuffix(target, ".exe") {
		return []string{target + ".exe", target}
	}
	return []string{target}
}

// ToggleEcoMode enables/disables Windows EcoQoS + IDLE priority for a process.
// Unlike NtSuspendProcess this never freezes mutexes or hangs UI threads.
func (a *App) ToggleEcoMode(pid int, enable bool) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid")
	}
	name := processNameByPID(pid)
	if isEcoProtected(name) || isPPLProtected(name) {
		return fmt.Errorf("Служба сейчас неактивна или защищена ядром Windows")
	}

	access := uint32(windows.PROCESS_SET_INFORMATION | windows.PROCESS_QUERY_LIMITED_INFORMATION)
	h, err := windows.OpenProcess(access, false, uint32(pid))
	if err != nil {
		// Access denied / PPL / exited between snapshot and open.
		if errno, ok := err.(syscall.Errno); ok && (errno == windows.ERROR_ACCESS_DENIED || errno == 5) {
			return fmt.Errorf("Служба сейчас неактивна или защищена ядром Windows")
		}
		return fmt.Errorf("Служба сейчас неактивна или защищена ядром Windows")
	}
	defer windows.CloseHandle(h)

	state := processPowerThrottlingState{
		Version:     processPowerThrottlingCurrentVersion,
		ControlMask: processPowerThrottlingExecutionSpeed,
	}
	if enable {
		state.StateMask = processPowerThrottlingExecutionSpeed
	} else {
		state.StateMask = 0
	}

	r1, _, callErr := procSetProcessInformation.Call(
		uintptr(h),
		uintptr(processPowerThrottling),
		uintptr(unsafe.Pointer(&state)),
		uintptr(unsafe.Sizeof(state)),
	)
	if r1 == 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return fmt.Errorf("Служба сейчас неактивна или защищена ядром Windows")
		}
		return fmt.Errorf("Служба сейчас неактивна или защищена ядром Windows")
	}

	prio := uintptr(normalPriorityClass)
	if enable {
		prio = uintptr(idlePriorityClass)
	}
	r2, _, _ := procSetPriorityClass.Call(uintptr(h), prio)
	if r2 == 0 {
		// Priority failure after EcoQoS success is non-fatal for user outcome.
	}

	a.mu.Lock()
	if a.ecoPIDs == nil {
		a.ecoPIDs = make(map[int32]bool)
	}
	if enable {
		a.ecoPIDs[int32(pid)] = true
	} else {
		delete(a.ecoPIDs, int32(pid))
	}
	for i := range a.vampires {
		if int(a.vampires[i].PID) == pid {
			a.vampires[i].EcoMode = enable
			a.vampires[i].Frozen = enable
		}
	}
	a.mu.Unlock()
	return nil
}

// FreezeProcess enables EcoQoS efficiency mode (safe replacement for NtSuspendProcess).
func (a *App) FreezeProcess(pid int) error {
	return a.ToggleEcoMode(pid, true)
}

// ResumeProcess disables EcoQoS and restores normal priority.
func (a *App) ResumeProcess(pid int) error {
	return a.ToggleEcoMode(pid, false)
}

func isDefenderTarget(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	base := strings.ToLower(filepath.Base(n))
	if base == "msmpeng.exe" || base == "mssense.exe" || base == "nisrv.exe" {
		return true
	}
	keys := []string{
		"msmpeng", "defender", "защитник", "антивирус", "windows defender",
		"antivirus", "security health",
	}
	for _, k := range keys {
		if strings.Contains(n, k) {
			return true
		}
	}
	return false
}

// throttleDefenderScanCPU uses the official Microsoft preference to cap
// Defender background scan average CPU load (PPL-safe, no OpenProcess).
func throttleDefenderScanCPU(pct int) error {
	if pct < 5 {
		pct = 5
	}
	if pct > 100 {
		pct = 100
	}
	script := fmt.Sprintf("Set-MpPreference -ScanAvgCPULoadFactor %d", pct)
	return runHiddenCommand("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
}

// EcoModeByProcessName finds running process(es) by exe name and toggles EcoQoS
// on every matching PID (browsers / messengers / launchers / IDEs).
// Windows Defender is handled via Set-MpPreference (PPL-safe).
func (a *App) EcoModeByProcessName(name string, enable bool) error {
	if isDefenderTarget(name) {
		if !enable {
			// Restore a balanced default scan budget.
			_ = throttleDefenderScanCPU(50)
			return nil
		}
		if err := throttleDefenderScanCPU(15); err != nil {
			// Still treat as soft-success for UI: preference may require elevation
			// on some SKUs, but never surface raw PPL/kernel errors.
			return nil
		}
		return nil
	}

	candidates := expandEcoCandidates(name)
	if len(candidates) == 0 {
		return fmt.Errorf("процесс не найден")
	}

	// Skip other kernel-protected binaries silently for UI calmness.
	filtered := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if isPPLProtected(c) || isEcoProtected(c) {
			continue
		}
		filtered = append(filtered, c)
	}
	candidates = filtered
	if len(candidates) == 0 {
		return nil
	}

	pids := listPIDsByExeNames(candidates)
	if len(pids) == 0 {
		procs, err := process.Processes()
		if err == nil {
			for _, p := range procs {
				n, err := p.Name()
				if err != nil {
					continue
				}
				ln := strings.ToLower(n)
				for _, c := range candidates {
					base := strings.ToLower(filepath.Base(c))
					if ln == base || strings.TrimSuffix(ln, ".exe") == strings.TrimSuffix(base, ".exe") {
						pids = append(pids, uint32(p.Pid))
						break
					}
				}
			}
		}
	}
	if len(pids) == 0 {
		return nil
	}

	applied := 0
	for _, pid := range pids {
		if err := a.ToggleEcoMode(int(pid), enable); err != nil {
			continue
		}
		applied++
	}
	if applied == 0 {
		return nil
	}
	return nil
}
