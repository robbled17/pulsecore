package main

import (
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

type procIOSample struct {
	read  uint64
	write uint64
	at    time.Time
}

type sessionHit struct {
	Name         string
	FriendlyName string
	CPUSec       float64
	DiskMB       float64
	LastSeen     time.Time
}

var (
	procIOMu    sync.Mutex
	procIOCache = map[int32]procIOSample{}
)

// normalizeProcessCPU converts gopsutil's per-core-ish CPU% into a share of
// total system capacity (0-100). One fully busy thread on a 12-thread CPU → ~8.3%.
func normalizeProcessCPU(raw float64) float64 {
	n := runtime.NumCPU()
	if n < 1 {
		n = 1
	}
	pct := raw / float64(n)
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct
}

// processDiskRateMBps returns instantaneous read+write MB/s using a per-PID delta.
// The first sample for a PID returns 0 (no fake cumulative throughput).
func processDiskRateMBps(pid int32, io *process.IOCountersStat) float64 {
	if io == nil || pid <= 0 {
		return 0
	}
	now := time.Now()
	procIOMu.Lock()
	defer procIOMu.Unlock()
	prev, ok := procIOCache[pid]
	procIOCache[pid] = procIOSample{read: io.ReadBytes, write: io.WriteBytes, at: now}
	if !ok || prev.at.IsZero() {
		return 0
	}
	dt := now.Sub(prev.at).Seconds()
	if dt < 0.05 {
		return 0
	}
	var dRead, dWrite float64
	if io.ReadBytes >= prev.read {
		dRead = float64(io.ReadBytes - prev.read)
	}
	if io.WriteBytes >= prev.write {
		dWrite = float64(io.WriteBytes - prev.write)
	}
	rate := (dRead + dWrite) / dt / (1024 * 1024)
	if rate < 0 {
		return 0
	}
	if rate > 2000 {
		return 2000
	}
	return rate
}

func pruneProcIOCache(alive map[int32]bool) {
	procIOMu.Lock()
	defer procIOMu.Unlock()
	for pid := range procIOCache {
		if !alive[pid] {
			delete(procIOCache, pid)
		}
	}
}

func clamp01to100(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
