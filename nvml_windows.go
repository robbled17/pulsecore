//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	nvmlSuccess           = 0
	nvmlTempGPU           = 0
	nvmlClockGraphics     = 0
	nvmlClockMem          = 2
	nvmlErrUninitialized  = 1
	nvmlErrInvalidArg     = 2
	nvmlErrNotSupported   = 3
	nvmlErrLibNotFound    = 12
)

type nvmlUtilization struct {
	GPU    uint32
	Memory uint32
}

type nvmlMemory struct {
	Total uint64
	Free  uint64
	Used  uint64
}

type nvmlSample struct {
	OK           bool
	Pct          float64
	UsedMB       float64
	TotalMB      float64
	TempC        float64
	TempOK       bool
	PowerW       float64
	PowerOK      bool
	ClockCoreMHz float64
	ClockMemMHz  float64
}

type nvmlAPI struct {
	mu       sync.Mutex
	dll      *windows.LazyDLL
	initOK   bool
	initTried bool
	failed   bool

	initFn       *windows.LazyProc
	shutdownFn   *windows.LazyProc
	countFn      *windows.LazyProc
	handleFn     *windows.LazyProc
	utilFn       *windows.LazyProc
	memFn        *windows.LazyProc
	tempFn       *windows.LazyProc
	powerFn      *windows.LazyProc
	clockFn      *windows.LazyProc
}

var nvmlGlobal = &nvmlAPI{}

func nvmlDLLCandidates() []string {
	var out []string
	roots := []string{
		os.Getenv("ProgramW6432"),
		os.Getenv("ProgramFiles"),
		os.Getenv("SystemRoot"),
	}
	for _, r := range roots {
		if r == "" {
			continue
		}
		out = append(out,
			filepath.Join(r, "NVIDIA Corporation", "NVSMI", "nvml.dll"),
			filepath.Join(r, "System32", "nvml.dll"),
		)
	}
	out = append(out, "nvml.dll")
	return out
}

func (n *nvmlAPI) ensure() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.failed {
		return false
	}
	if n.initOK {
		return true
	}
	if n.initTried && n.dll == nil {
		n.failed = true
		return false
	}
	n.initTried = true

	var dll *windows.LazyDLL
	for _, path := range nvmlDLLCandidates() {
		if path != "nvml.dll" {
			if _, err := os.Stat(path); err != nil {
				continue
			}
		}
		cand := windows.NewLazyDLL(path)
		if err := cand.Load(); err != nil {
			continue
		}
		dll = cand
		break
	}
	if dll == nil {
		n.failed = true
		return false
	}
	n.dll = dll
	n.initFn = dll.NewProc("nvmlInit_v2")
	if n.initFn.Find() != nil {
		n.initFn = dll.NewProc("nvmlInit")
	}
	n.shutdownFn = dll.NewProc("nvmlShutdown")
	n.countFn = dll.NewProc("nvmlDeviceGetCount_v2")
	if n.countFn.Find() != nil {
		n.countFn = dll.NewProc("nvmlDeviceGetCount")
	}
	n.handleFn = dll.NewProc("nvmlDeviceGetHandleByIndex_v2")
	if n.handleFn.Find() != nil {
		n.handleFn = dll.NewProc("nvmlDeviceGetHandleByIndex")
	}
	n.utilFn = dll.NewProc("nvmlDeviceGetUtilizationRates")
	n.memFn = dll.NewProc("nvmlDeviceGetMemoryInfo")
	n.tempFn = dll.NewProc("nvmlDeviceGetTemperature")
	n.powerFn = dll.NewProc("nvmlDeviceGetPowerUsage")
	n.clockFn = dll.NewProc("nvmlDeviceGetClockInfo")

	r, _, _ := n.initFn.Call()
	if r != nvmlSuccess {
		n.failed = true
		return false
	}
	n.initOK = true
	return true
}

func (n *nvmlAPI) sample() nvmlSample {
	if !n.ensure() {
		return nvmlSample{}
	}
	n.mu.Lock()
	defer n.mu.Unlock()

	var count uint32
	r, _, _ := n.countFn.Call(uintptr(unsafe.Pointer(&count)))
	if r != nvmlSuccess || count == 0 {
		return nvmlSample{}
	}
	var handle uintptr
	r, _, _ = n.handleFn.Call(0, uintptr(unsafe.Pointer(&handle)))
	if r != nvmlSuccess || handle == 0 {
		return nvmlSample{}
	}

	out := nvmlSample{OK: true}
	var util nvmlUtilization
	if r, _, _ = n.utilFn.Call(handle, uintptr(unsafe.Pointer(&util))); r == nvmlSuccess {
		out.Pct = float64(util.GPU)
	}
	var mem nvmlMemory
	if r, _, _ = n.memFn.Call(handle, uintptr(unsafe.Pointer(&mem))); r == nvmlSuccess {
		out.UsedMB = float64(mem.Used) / (1024 * 1024)
		out.TotalMB = float64(mem.Total) / (1024 * 1024)
	}
	var temp uint32
	if r, _, _ = n.tempFn.Call(handle, uintptr(nvmlTempGPU), uintptr(unsafe.Pointer(&temp))); r == nvmlSuccess {
		c := float64(temp)
		if c >= 10 && c <= 125 {
			out.TempC = c
			out.TempOK = true
		}
	}
	var mw uint32
	if r, _, _ = n.powerFn.Call(handle, uintptr(unsafe.Pointer(&mw))); r == nvmlSuccess && mw > 0 {
		out.PowerW = float64(mw) / 1000.0
		out.PowerOK = true
	}
	var clock uint32
	if r, _, _ = n.clockFn.Call(handle, uintptr(nvmlClockGraphics), uintptr(unsafe.Pointer(&clock))); r == nvmlSuccess && clock > 0 {
		out.ClockCoreMHz = float64(clock)
	}
	clock = 0
	if r, _, _ = n.clockFn.Call(handle, uintptr(nvmlClockMem), uintptr(unsafe.Pointer(&clock))); r == nvmlSuccess && clock > 0 {
		out.ClockMemMHz = float64(clock)
	}
	return out
}

func readGPUViaNVML() nvmlSample {
	return nvmlGlobal.sample()
}

// nvmlStatusString is for diagnostics only.
func nvmlStatusString() string {
	if nvmlGlobal.failed {
		return "nvml unavailable"
	}
	if nvmlGlobal.initOK {
		return "nvml ok"
	}
	return fmt.Sprintf("nvml not initialized")
}
