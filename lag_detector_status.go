package main

import (
	"time"
)

// GetLagDetectorStatus powers the live «Почему лагнуло?» idle / incident cards.
func (a *App) GetLagDetectorStatus() map[string]interface{} {
	bb := a.ensureBlackBox()
	lang := a.getLang()
	now := time.Now()
	hourAgo := now.Add(-time.Hour)

	bb.mu.Lock()
	warmup := bb.warmupDone
	incidents := append([]LagIncident(nil), bb.incidents...)
	bb.mu.Unlock()

	recent := make([]map[string]interface{}, 0, 5)
	for _, inc := range incidents {
		if inc.Start.Before(hourAgo) {
			continue
		}
		recent = append(recent, map[string]interface{}{
			"id":          inc.ID,
			"time":        inc.Start.Format("15:04:05"),
			"type":        string(inc.Verdict.Type),
			"verdictText": verdictText(inc.Verdict, lang),
			"culprit":     inc.Verdict.Culprit,
			"culpritPID":  inc.Verdict.CulpritPID,
			"severity":    inc.Verdict.Severity,
			"synthetic":   inc.Synthetic,
		})
		if len(recent) >= 5 {
			break
		}
	}

	gameDrops := 0
	// Honest: without game injection we cannot count FPS sub-60 frames.
	// Approximate with non-synthetic micro-freeze incidents during the last hour.
	for _, inc := range incidents {
		if inc.Synthetic || inc.Start.Before(hourAgo) {
			continue
		}
		gameDrops++
	}

	status := "monitoring"
	if !warmup {
		status = "warming"
	} else if len(recent) > 0 {
		status = "incidents"
	}

	return map[string]interface{}{
		"ok":               true,
		"status":           status,
		"bufferActive":     true,
		"windowSec":        60,
		"warmupDone":       warmup,
		"incidentsHour":    len(recent),
		"gameDropsBelow60": gameDrops,
		"recent":           recent,
	}
}
