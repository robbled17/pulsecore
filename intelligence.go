package main

import (
	"github.com/robbled17/zeropulse/internal/diag"
)

// GetIntelligenceInsights runs local offline rules over existing modules.
func (a *App) GetIntelligenceInsights() []diag.Insight {
	weekly := a.GetWeeklySummary()
	health := a.GetSystemHealth()
	leaks := a.GetMemoryLeakWatch()
	startup := a.AnalyzeStartup()
	gm := a.GetGameModeStatus()
	a.mu.RLock()
	batWear := a.metrics.Battery.WearPercent
	batAvail := a.metrics.Battery.Available
	a.mu.RUnlock()

	in := diag.IntelInput{
		CPUAvg7d:       weekly.CPUAvg,
		RAMAvg7d:       weekly.RAMAvg,
		LagCount7d:     weekly.LagCount,
		EcoToggles7d:   weekly.EcoToggles,
		HealthScore:    health.Score,
		LeakSuspects:   len(leaks),
		StartupHeavy:   startup.HeavyCount,
		GameModeActive: gm.Active,
		BatteryWear:    batWear,
		BatteryAvail:   batAvail,
	}
	return diag.EvaluateIntelligence(in)
}
