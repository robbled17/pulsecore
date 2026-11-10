package main

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modPowrProf                = windows.NewLazySystemDLL("powrprof.dll")
	procPowerSetActiveScheme   = modPowrProf.NewProc("PowerSetActiveScheme")
	procPowerGetActiveScheme   = modPowrProf.NewProc("PowerGetActiveScheme")
	procPowerWriteACValueIndex = modPowrProf.NewProc("PowerWriteACValueIndex")
	procPowerWriteDCValueIndex = modPowrProf.NewProc("PowerWriteDCValueIndex")
)

const (
	guidSchemeBalanced  = "381b4222-f694-41f0-9685-ff5bb260df2e"
	guidSchemeHighPerf  = "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c"
	guidSchemePowerSave = "a1841308-3541-4fab-bc81-f71556f20b4a"
	guidSubProcessor    = "54533251-82be-4824-96c1-47b60b740d00"
	guidProcThrottleMax = "bc5038f7-23e0-4960-96da-33abaf5935d3"
)

func mustGUID(s string) windows.GUID {
	g, err := windows.GUIDFromString("{" + s + "}")
	if err != nil {
		panic(err)
	}
	return g
}

var (
	schemeBalanced  = mustGUID(guidSchemeBalanced)
	schemeHighPerf  = mustGUID(guidSchemeHighPerf)
	schemePowerSave = mustGUID(guidSchemePowerSave)
	subProcessor    = mustGUID(guidSubProcessor)
	procThrottleMax = mustGUID(guidProcThrottleMax)
)

func powerSetActiveScheme(scheme *windows.GUID) error {
	r1, _, err := procPowerSetActiveScheme.Call(0, uintptr(unsafe.Pointer(scheme)))
	if r1 != 0 {
		if err != windows.ERROR_SUCCESS {
			return fmt.Errorf("PowerSetActiveScheme: 0x%X (%v)", r1, err)
		}
		return fmt.Errorf("PowerSetActiveScheme: 0x%X", r1)
	}
	return nil
}

// powerWriteCPUMax tries to tune processor max state. Closed/OEM-locked
// schemes often return ERROR_FILE_NOT_FOUND (0x2) — caller must soft-fail.
func powerWriteCPUMax(scheme *windows.GUID, maxPercent uint32) error {
	r1, _, err := procPowerWriteACValueIndex.Call(
		0,
		uintptr(unsafe.Pointer(scheme)),
		uintptr(unsafe.Pointer(&subProcessor)),
		uintptr(unsafe.Pointer(&procThrottleMax)),
		uintptr(maxPercent),
	)
	if r1 != 0 {
		return fmt.Errorf("PowerWriteACValueIndex: 0x%X (%v)", r1, err)
	}
	r2, _, err := procPowerWriteDCValueIndex.Call(
		0,
		uintptr(unsafe.Pointer(scheme)),
		uintptr(unsafe.Pointer(&subProcessor)),
		uintptr(unsafe.Pointer(&procThrottleMax)),
		uintptr(maxPercent),
	)
	if r2 != 0 {
		return fmt.Errorf("PowerWriteDCValueIndex: 0x%X (%v)", r2, err)
	}
	return nil
}

// setPowerProfileNative switches Windows power schemes via PowrProf.dll.
// Primary path is PowerSetActiveScheme with built-in scheme GUIDs.
// Optional CPU throttle writes are soft-failed (OEM / locked schemes).
func setPowerProfileNative(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	var (
		scheme     *windows.GUID
		normalized string
		cpuMax     uint32
	)
	switch mode {
	case "silent", "silent_laptop", "powersaver":
		normalized = "silent"
		scheme = &schemePowerSave
		cpuMax = 99
	case "turbo", "full", "full_throttle", "high":
		normalized = "turbo"
		scheme = &schemeHighPerf
		cpuMax = 100
	default:
		normalized = "balanced"
		scheme = &schemeBalanced
		cpuMax = 100
	}

	// Soft-apply optional sub-parameter. ERROR_FILE_NOT_FOUND (0x2) is common
	// on locked/OEM schemes — never abort the profile switch because of it.
	_ = powerWriteCPUMax(scheme, cpuMax)

	if err := powerSetActiveScheme(scheme); err != nil {
		return normalized, err
	}
	return normalized, nil
}
