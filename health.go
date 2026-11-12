package main

import (
	"github.com/robbled17/zeropulse/internal/diag"
)

// GetSystemHealth returns a unified 0–100 health score.
func (a *App) GetSystemHealth() diag.HealthReport {
	a.mu.RLock()
	m := a.metrics
	lags := len(a.stutterEvents)
	a.mu.RUnlock()

	maxDisk := 0.0
	for _, d := range m.Disks {
		if d.Percent > maxDisk {
			maxDisk = d.Percent
		}
	}
	in := diag.HealthInput{
		CPUPercent:     m.CPUPercent,
		RAMPercent:     m.RAMPercent,
		TempC:          m.TemperatureC,
		DiskPercentMax: maxDisk,
		BatteryWear:    m.Battery.WearPercent,
		BatteryAvail:   m.Battery.Available,
		LagCountRecent: lags,
	}
	return diag.ComputeHealthScore(in)
}
