package main

import (
	"time"

	"github.com/robbled17/zeropulse/internal/store"
)

func (a *App) ensureStats() *store.StatsManager {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stats != nil {
		return a.stats
	}
	path, err := store.DefaultPath()
	if err != nil {
		return nil
	}
	m, err := store.OpenStats(path)
	if err != nil {
		return nil
	}
	a.stats = m
	return a.stats
}

func (a *App) recordStatsTick(m SystemMetrics, lagHit bool) {
	sm := a.ensureStats()
	if sm == nil {
		return
	}
	top := ""
	topCPU := 0.0
	if len(m.Vampires) > 0 {
		top = m.Vampires[0].Name
		topCPU = m.Vampires[0].CPUPercent
	}
	sm.RecordSample(m.CPUPercent, m.RAMPercent, m.GPUPercent, top, topCPU, lagHit)
	// Persist at most once per minute.
	a.mu.Lock()
	due := time.Since(a.statsSavedAt) > time.Minute
	if due {
		a.statsSavedAt = time.Now()
	}
	a.mu.Unlock()
	if due {
		_ = sm.Save()
	}
}

// GetWeeklySummary returns local APPDATA aggregates for the last 7 days.
func (a *App) GetWeeklySummary() store.WeeklySummary {
	sm := a.ensureStats()
	if sm == nil {
		return store.WeeklySummary{OK: false, NoteKey: "weeklyEmptyLocal", PeriodDays: 7}
	}
	return store.BuildWeeklySummary(sm.LastNDays(7))
}
