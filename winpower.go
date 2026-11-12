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

func activePowerSchemeName() string {
	var pguid *windows.GUID
	r1, _, _ := procPowerGetActiveScheme.Call(0, uintptr(unsafe.Pointer(&pguid)))
	if r1 != 0 || pguid == nil {
		return "Сбалансированная"
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(pguid)))
	g := *pguid
	switch {
	case g == schemeHighPerf:
		return "Высокая производительность"
	case g == schemePowerSave:
		return "Экономия энергии"
	case g == schemeBalanced:
		return "Сбалансированная"
	default:
		return "Сбалансированная"
	}
}

func estimateComponentWatts(cpuName, gpuName string, cpuPct, gpuPct float64) (cpuW, gpuW float64) {
	cpuTDP := 65.0
	cl := strings.ToLower(cpuName)
	switch {
	case strings.Contains(cl, "ryzen 9"), strings.Contains(cl, "i9-"), strings.Contains(cl, "ultra 9"):
		cpuTDP = 120
	case strings.Contains(cl, "ryzen 7"), strings.Contains(cl, "i7-"), strings.Contains(cl, "ultra 7"):
		cpuTDP = 95
	case strings.Contains(cl, "ryzen 5"), strings.Contains(cl, "i5-"), strings.Contains(cl, "ultra 5"):
		cpuTDP = 65
	case strings.Contains(cl, "ryzen 3"), strings.Contains(cl, "i3-"):
		cpuTDP = 45
	case strings.Contains(cl, "athlon"), strings.Contains(cl, "celeron"), strings.Contains(cl, "pentium"):
		cpuTDP = 35
	}
	if strings.Contains(cl, "u") && (strings.Contains(cl, "laptop") || strings.Contains(cl, "mobile")) {
		cpuTDP *= 0.55
	}
	load := cpuPct / 100.0
	if load < 0 {
		load = 0
	}
	if load > 1 {
		load = 1
	}
	cpuW = 12 + cpuTDP*load*0.9

	gpuTDP := 75.0
	gl := strings.ToLower(gpuName)
	switch {
	case strings.Contains(gl, "4090"):
		gpuTDP = 450
	case strings.Contains(gl, "4080"):
		gpuTDP = 320
	case strings.Contains(gl, "4070"):
		gpuTDP = 200
	case strings.Contains(gl, "4060"):
		gpuTDP = 115
	case strings.Contains(gl, "3090"):
		gpuTDP = 350
	case strings.Contains(gl, "3080"):
		gpuTDP = 320
	case strings.Contains(gl, "3070"):
		gpuTDP = 220
	case strings.Contains(gl, "3060"):
		gpuTDP = 170
	case strings.Contains(gl, "2060"):
		gpuTDP = 160
	case strings.Contains(gl, "1660"):
		gpuTDP = 120
	case strings.Contains(gl, "1650"):
		gpuTDP = 75
	case strings.Contains(gl, "1050"):
		gpuTDP = 75
	case strings.Contains(gl, "radeon") && strings.Contains(gl, "7800"):
		gpuTDP = 263
	case strings.Contains(gl, "radeon") && strings.Contains(gl, "7600"):
		gpuTDP = 165
	case strings.Contains(gl, "radeon") && strings.Contains(gl, "6600"):
		gpuTDP = 132
	case strings.Contains(gl, "uhd"), strings.Contains(gl, "iris"), strings.Contains(gl, "vega"), strings.Contains(gl, "graphics"):
		gpuTDP = 15
	}
	gLoad := gpuPct / 100.0
	if gLoad < 0 {
		gLoad = 0
	}
	if gLoad > 1 {
		gLoad = 1
	}
	gpuW = 8 + gpuTDP*gLoad
	return cpuW, gpuW
}

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
// schemes often return ERROR_FILE_NOT_FOUND (0x2) - caller must soft-fail.
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

// Power scheme switching lives in power.go (Turbo tuning + timeBeginPeriod).
