package main

import (
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// CPU load is sampled with GetSystemTimes and NtQuerySystemInformation.
// Neither path talks to WMI.

var procGetSystemTimes = modKernel32.NewProc("GetSystemTimes")

var (
	modNtdll                     = windows.NewLazySystemDLL("ntdll.dll")
	procNtQuerySystemInformation = modNtdll.NewProc("NtQuerySystemInformation")
)

const systemProcessorPerformanceInformation = 8

type procPerfInfo struct {
	IdleTime       int64
	KernelTime     int64
	UserTime       int64
	DpcTime        int64
	InterruptTime  int64
	InterruptCount uint32
	_              uint32
}

type cpuTick struct {
	idle, kernel, user uint64
	cores              []procPerfInfo
}

var (
	cpuSampleMu sync.Mutex
	prevCPUTick cpuTick
	haveCPUTick bool
)

func fileTimeUint(ft windows.Filetime) uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}

func readSystemTimes() (idle, kernel, user uint64, ok bool) {
	var i, k, u windows.Filetime
	r, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&i)),
		uintptr(unsafe.Pointer(&k)),
		uintptr(unsafe.Pointer(&u)),
	)
	if r == 0 {
		return 0, 0, 0, false
	}
	return fileTimeUint(i), fileTimeUint(k), fileTimeUint(u), true
}

func readPerCoreTicks() []procPerfInfo {
	n := runtime.NumCPU()
	if n < 1 {
		n = 1
	}
	buf := make([]procPerfInfo, n)
	size := uintptr(len(buf)) * unsafe.Sizeof(procPerfInfo{})
	r, _, _ := procNtQuerySystemInformation.Call(
		systemProcessorPerformanceInformation,
		uintptr(unsafe.Pointer(&buf[0])),
		size,
		0,
	)
	// STATUS_SUCCESS == 0. A mismatch means the buffer does not match the live CPU count.
	if r != 0 {
		return nil
	}
	return buf
}

// sampleCPULoad returns total and per-core busy percent since the previous call.
// The first sample only arms the delta and reports zeros.
func sampleCPULoad() (overall float64, perCore []float64) {
	idle, kernel, user, ok := readSystemTimes()
	cores := readPerCoreTicks()
	if !ok && len(cores) == 0 {
		return 0, nil
	}

	cpuSampleMu.Lock()
	defer cpuSampleMu.Unlock()

	if !haveCPUTick {
		prevCPUTick = cpuTick{idle: idle, kernel: kernel, user: user, cores: cores}
		haveCPUTick = true
		if len(cores) > 0 {
			return 0, make([]float64, len(cores))
		}
		return 0, nil
	}

	// Kernel time from GetSystemTimes includes idle time.
	idleD := idle - prevCPUTick.idle
	totalD := (kernel - prevCPUTick.kernel) + (user - prevCPUTick.user)
	if totalD > 0 && idleD < totalD {
		overall = float64(totalD-idleD) / float64(totalD) * 100
	}

	if len(cores) > 0 && len(prevCPUTick.cores) == len(cores) {
		perCore = make([]float64, len(cores))
		for i := range cores {
			prev := prevCPUTick.cores[i]
			cur := cores[i]
			idleC := cur.IdleTime - prev.IdleTime
			totalC := (cur.KernelTime - prev.KernelTime) + (cur.UserTime - prev.UserTime)
			if totalC > 0 && idleC >= 0 && idleC < totalC {
				perCore[i] = float64(totalC-idleC) / float64(totalC) * 100
			}
		}
	} else if len(cores) > 0 {
		perCore = make([]float64, len(cores))
	}

	prevCPUTick = cpuTick{idle: idle, kernel: kernel, user: user, cores: cores}
	if overall < 0 {
		overall = 0
	}
	if overall > 100 {
		overall = 100
	}
	return overall, perCore
}
