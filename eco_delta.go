package main

import (
	"time"

	"github.com/robbled17/zeropulse/internal/diag"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) captureEcoSample(pid int) diag.EcoSample {
	a.mu.RLock()
	m := a.metrics
	a.mu.RUnlock()
	s := diag.EcoSample{
		CPUPercent: m.CPUPercent,
		RAMPercent: m.RAMPercent,
		GPUPercent: m.GPUPercent,
		AtUnixMs:   time.Now().UnixMilli(),
	}
	if pid > 0 {
		if p, err := process.NewProcess(int32(pid)); err == nil {
			if cpu, err := p.CPUPercent(); err == nil {
				s.ProcessCPU = cpu
			}
			if mi, err := p.MemoryInfo(); err == nil && mi != nil {
				s.ProcessRSS = float64(mi.RSS) / (1024 * 1024)
			}
		}
	}
	return s
}

func (a *App) beginEcoDelta(pid int, name string, enable bool) diag.EcoSample {
	before := a.captureEcoSample(pid)
	pending := diag.ComputeEcoDelta(int32(pid), name, enable, before, diag.EcoSample{})
	a.mu.Lock()
	a.lastEcoDelta = pending
	a.mu.Unlock()
	return before
}

func (a *App) finishEcoDelta(pid int, name string, enable bool, before diag.EcoSample) {
	// Re-sample after a short settle so EcoQoS effect is visible.
	go func() {
		timer := time.NewTimer(1500 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-a.stopWorker:
			return
		}
		if a.ctx != nil {
			select {
			case <-a.ctx.Done():
				return
			default:
			}
		}
		after := a.captureEcoSample(pid)
		delta := diag.ComputeEcoDelta(int32(pid), name, enable, before, after)
		a.mu.Lock()
		a.lastEcoDelta = delta
		a.mu.Unlock()
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "eco:delta", delta)
		}
	}()
}

// GetLastEcoDelta returns the most recent Eco before/after comparison.
func (a *App) GetLastEcoDelta() diag.EcoDelta {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastEcoDelta
}
