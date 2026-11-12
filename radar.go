package main

import (
	"github.com/robbled17/zeropulse/internal/diag"
)

func (a *App) ensureLeakWatch() *diag.LeakWatch {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.leakWatch == nil {
		a.leakWatch = diag.NewLeakWatch()
	}
	return a.leakWatch
}

func (a *App) observeRadarAndLeaks() {
	samples := sampleTopProcesses(12)
	lw := a.ensureLeakWatch()
	prev := make(map[int32]float64)
	a.mu.RLock()
	for pid, cpu := range a.radarPrevCPU {
		prev[pid] = cpu
	}
	a.mu.RUnlock()

	radar := diag.ScoreRadar(samples, prev)
	for _, s := range samples {
		lw.Observe(s.PID, s.Name, s.RSSMB)
	}
	next := make(map[int32]float64, len(samples))
	for _, s := range samples {
		next[s.PID] = s.CPUPercent
	}
	a.mu.Lock()
	a.radarPrevCPU = next
	a.lastRadar = radar
	a.lastLeaks = lw.Suspects(8, 80)
	a.mu.Unlock()
}

// GetProcessRadar returns scored live process radar entries.
func (a *App) GetProcessRadar() []diag.RadarEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]diag.RadarEntry(nil), a.lastRadar...)
}

// GetMemoryLeakWatch returns rising-RSS suspects.
func (a *App) GetMemoryLeakWatch() []diag.LeakSuspect {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]diag.LeakSuspect(nil), a.lastLeaks...)
}
