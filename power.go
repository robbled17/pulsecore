package main

import (
	"fmt"
	"strings"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Processor / PCI power setting GUIDs (Microsoft documented scheme aliases).
const (
	guidProcThrottleMin = "893dee8e-2bef-41e0-89c6-b55d0929964c" // PROCTHROTTLEMIN
	guidCPMinCores      = "0cc5b647-c1df-4637-891a-dec35c318583" // CPMINCORES (core parking floor)
	guidSubPCIExpress   = "501a4d13-42af-4429-9fd1-a821f2c433cd"
	guidPCILinkASPM     = "ee12f906-d277-404b-b6da-e5fa1a576df5" // Link State Power Management
)

var (
	modWinmm              = windows.NewLazySystemDLL("winmm.dll")
	procTimeBeginPeriod   = modWinmm.NewProc("timeBeginPeriod")
	procTimeEndPeriod     = modWinmm.NewProc("timeEndPeriod")
	gameTimerPeriodActive atomic.Bool

	procThrottleMin = mustGUID(guidProcThrottleMin)
	cpMinCores      = mustGUID(guidCPMinCores)
	subPCIExpress   = mustGUID(guidSubPCIExpress)
	pciLinkASPM     = mustGUID(guidPCILinkASPM)
)

func setGameTimerPeriod(enable bool) {
	if enable {
		if gameTimerPeriodActive.CompareAndSwap(false, true) {
			_, _, _ = procTimeBeginPeriod.Call(1)
		}
		return
	}
	if gameTimerPeriodActive.CompareAndSwap(true, false) {
		_, _, _ = procTimeEndPeriod.Call(1)
	}
}

func powerWriteIndex(scheme, subgroup, setting *windows.GUID, value uint32) {
	if scheme == nil || subgroup == nil || setting == nil {
		return
	}
	_, _, _ = procPowerWriteACValueIndex.Call(
		0,
		uintptr(unsafe.Pointer(scheme)),
		uintptr(unsafe.Pointer(subgroup)),
		uintptr(unsafe.Pointer(setting)),
		uintptr(value),
	)
	_, _, _ = procPowerWriteDCValueIndex.Call(
		0,
		uintptr(unsafe.Pointer(scheme)),
		uintptr(unsafe.Pointer(subgroup)),
		uintptr(unsafe.Pointer(setting)),
		uintptr(value),
	)
}

// applyTurboSchemeTuning locks the High Performance scheme for stable frametimes:
// PROCTHROTTLEMIN 100, CPMINCORES 100 (no core parking delay), PCIe ASPM Off.
func applyTurboSchemeTuning(scheme *windows.GUID) {
	if scheme == nil {
		return
	}
	_ = powerWriteCPUMax(scheme, 100)
	powerWriteIndex(scheme, &subProcessor, &procThrottleMin, 100)
	powerWriteIndex(scheme, &subProcessor, &cpMinCores, 100)
	powerWriteIndex(scheme, &subPCIExpress, &pciLinkASPM, 0) // Off
}

// applyBalancedSchemeTuning restores sane Balanced defaults (soft-fail on OEM locks).
func applyBalancedSchemeTuning(scheme *windows.GUID) {
	if scheme == nil {
		return
	}
	_ = powerWriteCPUMax(scheme, 100)
	powerWriteIndex(scheme, &subProcessor, &procThrottleMin, 5)
	powerWriteIndex(scheme, &subProcessor, &cpMinCores, 10)
	powerWriteIndex(scheme, &subPCIExpress, &pciLinkASPM, 2) // Moderate power savings
}

func powercfgSetActive(guid string) {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return
	}
	_ = runHiddenCommand("powercfg", "/setactive", guid)
}

// setPowerProfileNative switches Silent / Balanced / Turbo via PowrProf + powercfg.
// Turbo also enables winmm timeBeginPeriod(1) for ~1 ms timer resolution.
func setPowerProfileNative(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	var (
		scheme     *windows.GUID
		normalized string
		schemeGUID string
	)
	switch mode {
	case "silent", "silent_laptop", "powersaver":
		normalized = "silent"
		scheme = &schemePowerSave
		schemeGUID = guidSchemePowerSave
		setGameTimerPeriod(false)
	case "turbo", "full", "full_throttle", "high":
		normalized = "turbo"
		scheme = &schemeHighPerf
		schemeGUID = guidSchemeHighPerf
		applyTurboSchemeTuning(scheme)
		setGameTimerPeriod(true)
	default:
		normalized = "balanced"
		scheme = &schemeBalanced
		schemeGUID = guidSchemeBalanced
		applyBalancedSchemeTuning(scheme)
		setGameTimerPeriod(false)
	}

	if err := powerSetActiveScheme(scheme); err != nil {
		// Fallback identical to `powercfg /setactive <GUID>`.
		powercfgSetActive(schemeGUID)
		if err2 := powerSetActiveScheme(scheme); err2 != nil {
			return normalized, fmt.Errorf("%v; powercfg fallback also failed: %v", err, err2)
		}
	} else {
		// Keep powercfg path warm for tools that only read the CLI-active scheme.
		powercfgSetActive(schemeGUID)
	}
	return normalized, nil
}
