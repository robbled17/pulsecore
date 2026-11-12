package diag

const (
	criticalFreeRAMMB   = 300.0
	diskBusyMBps        = 40.0
	cpuHotPct           = 90.0
	gpuHotPct           = 95.0
	thermalHotC         = 90.0
	processHotCPU       = 25.0
	minConfidenceKnown  = 0.55
	unknownConfidence   = 0.35
)

// Evaluate builds an honest verdict from a hitch sample + surrounding context.
// When evidence is weak, LevelUnknown is returned instead of inventing a cause.
func Evaluate(hit Snapshot, pre []Snapshot, jitterMs float64) Verdict {
	v := Verdict{
		Level:      LevelUnknown,
		Cause:      CauseUnknown,
		CauseKey:   "verdictCauseUnknown",
		SummaryKey: "verdictUnknown",
		DetailKey:  "verdictUnknownDetail",
		Confidence: unknownConfidence,
	}

	if jitterMs < 300 && hit.CPUPercent < 80 && hit.RAMFreeMB > criticalFreeRAMMB {
		// Soft hitch without strong signals → still unknown, not "ideal".
		v.SummaryKey = "verdictSoftUnknown"
		return v
	}

	// RAM pressure wins when free RAM is critically low.
	if float64(hit.RAMFreeMB) < criticalFreeRAMMB {
		v.Level = LevelCrit
		v.Cause = CauseRAMPressure
		v.CauseKey = "verdictCauseRAM"
		v.SummaryKey = "verdictRAM"
		v.DetailKey = "verdictRAMDetail"
		v.Confidence = 0.9
		return v
	}

	diskTotal := hit.DiskReadMBps + hit.DiskWriteMBps
	if diskTotal >= diskBusyMBps {
		v.Level = LevelCrit
		v.Cause = CauseDiskBusy
		v.CauseKey = "verdictCauseDisk"
		v.SummaryKey = "verdictDisk"
		v.DetailKey = "verdictDiskDetail"
		v.Confidence = 0.85
		if len(hit.TopProcesses) > 0 {
			// Prefer a process with disk activity if present.
			best := hit.TopProcesses[0]
			for _, p := range hit.TopProcesses {
				if p.DiskMBps > best.DiskMBps {
					best = p
				}
			}
			if best.DiskMBps > 1 || best.Name != "" {
				v.Culprit = best.Name
			}
		}
		return v
	}

	if hit.TemperatureC >= thermalHotC {
		v.Level = LevelWarn
		v.Cause = CauseThermal
		v.CauseKey = "verdictCauseThermal"
		v.SummaryKey = "verdictThermal"
		v.DetailKey = "verdictThermalDetail"
		v.Confidence = 0.75
		return v
	}

	if hit.GPUPercent >= gpuHotPct {
		v.Level = LevelWarn
		v.Cause = CauseGPUHot
		v.CauseKey = "verdictCauseGPU"
		v.SummaryKey = "verdictGPU"
		v.DetailKey = "verdictGPUDetail"
		v.Confidence = 0.7
		return v
	}

	if hit.CPUPercent >= cpuHotPct {
		v.Level = LevelWarn
		v.Cause = CauseCPUHot
		v.CauseKey = "verdictCauseCPU"
		v.SummaryKey = "verdictCPU"
		v.DetailKey = "verdictCPUDetail"
		v.Confidence = 0.7
		if cul := hottestProcess(hit); cul != "" {
			v.Culprit = cul
			v.Cause = CauseProcess
			v.CauseKey = "verdictCauseProcess"
			v.SummaryKey = "verdictProcess"
			v.DetailKey = "verdictProcessDetail"
			v.Confidence = 0.8
		}
		return v
	}

	if cul := hottestProcess(hit); cul != "" {
		for _, p := range hit.TopProcesses {
			if p.Name == cul && p.CPUPercent >= processHotCPU {
				v.Level = LevelWarn
				v.Cause = CauseProcess
				v.CauseKey = "verdictCauseProcess"
				v.SummaryKey = "verdictProcess"
				v.DetailKey = "verdictProcessDetail"
				v.Culprit = cul
				v.Confidence = 0.65
				return v
			}
		}
	}

	// Rising CPU across pre-window without a clear culprit → unknown, not "ideal".
	if risingLoad(pre, hit) {
		v.SummaryKey = "verdictRisingUnknown"
		v.DetailKey = "verdictRisingUnknownDetail"
		v.Confidence = 0.4
		return v
	}

	if v.Confidence < minConfidenceKnown {
		return v
	}
	return v
}

func hottestProcess(hit Snapshot) string {
	bestName := ""
	bestCPU := 0.0
	for _, p := range hit.TopProcesses {
		if p.CPUPercent > bestCPU {
			bestCPU = p.CPUPercent
			bestName = p.Name
		}
	}
	return bestName
}

func risingLoad(pre []Snapshot, hit Snapshot) bool {
	if len(pre) < 2 {
		return false
	}
	first := pre[0].CPUPercent
	return hit.CPUPercent-first >= 25
}

// SmoothnessFromIncidents produces banner state. Empty incidents with a warm
// ring → "watching" (not falsely ideal). Cold start → "warmup".
func SmoothnessFromIncidents(incidents []Incident, ringLen int) map[string]interface{} {
	if len(incidents) > 0 {
		last := incidents[0]
		return map[string]interface{}{
			"level":      string(last.Verdict.Level),
			"messageKey": last.Verdict.SummaryKey,
			"causeKey":   last.Verdict.CauseKey,
			"culprit":    last.Culprit,
			"jitterMs":   last.JitterMs,
			"confidence": last.Verdict.Confidence,
		}
	}
	if ringLen < 5 {
		return map[string]interface{}{
			"level":      string(LevelUnknown),
			"messageKey": "verdictWarmup",
			"causeKey":   "verdictCauseUnknown",
		}
	}
	return map[string]interface{}{
		"level":      string(LevelOK),
		"messageKey": "verdictWatching",
		"causeKey":   "",
	}
}
