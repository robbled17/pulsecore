package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// criticalWindowsProcs must never receive Eco Mode / kill suggestions.
var criticalWindowsProcs = map[string]struct{}{
	"system": {}, "idle": {}, "registry": {},
	"smss.exe": {}, "csrss.exe": {}, "wininit.exe": {},
	"services.exe": {}, "lsass.exe": {}, "svchost.exe": {},
	"winlogon.exe": {}, "fontdrvhost.exe": {}, "dwm.exe": {},
	"explorer.exe": {}, "memcompression": {},
}

func isCriticalSystemProc(name string) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(name)))
	if base == "" {
		return false
	}
	_, ok := criticalWindowsProcs[base]
	return ok
}

// IncidentHints carries detector-side context into the pure verdict engine.
type IncidentHints struct {
	DurationMs  float64
	Kind        StallKind
	HasLoadSign bool
	PeakCPUFreq float64 // historical peak in recent window; 0 = unknown
	IgnoreLoad  bool    // test hook: force no-load path
}

// EvaluateVerdict is a pure function: last snapshots + incident hints → Verdict.
// Priority: CPU_HOG > DISK_HOG > MEMORY_PRESSURE > CPU_SATURATION > THERMAL_THROTTLE > UNKNOWN.
func EvaluateVerdict(snaps []Snapshot, hints IncidentHints) Verdict {
	if hints.IgnoreLoad || (!hints.HasLoadSign && !anyLoadSign(snaps)) {
		return finishVerdict(Verdict{
			Type:       VerdictUnknown,
			Severity:   "low",
			Confidence: 0.2,
			TextRu:     "Признаки системной нагрузки отсутствуют - задержка могла быть локальной для PulseCore.",
			TextEn:     "No system load signs - the delay may have been local to PulseCore.",
			Action:     "none",
			Evidence:   []string{"Нет признаков системной нагрузки"},
			EvidenceRaw: []string{"no_load_sign"},
		})
	}

	last := latestSnap(snaps)
	topCPU := bestCPU(last)
	topDisk := bestDisk(last)
	topRAM := bestRAM(last)

	// 1. CPU_HOG
	if topCPU.CPUPercent > 80 {
		v := baseVerdict(VerdictCPUHog, topCPU, "high", 0.85)
		v.TextRu = fmt.Sprintf("%s использует более 80%% нормализованного CPU (%.0f%%).", displayName(topCPU.Name), topCPU.CPUPercent)
		v.TextEn = fmt.Sprintf("%s is using over 80%% normalized CPU (%.0f%%).", displayName(topCPU.Name), topCPU.CPUPercent)
		v.Evidence = []string{
			fmt.Sprintf("Процесс %s: %.0f%% CPU (нормализовано)", displayName(topCPU.Name), topCPU.CPUPercent),
		}
		v.EvidenceRaw = []string{fmt.Sprintf("cpu=%.1f", topCPU.CPUPercent)}
		applySystemAction(&v)
		return finishVerdict(v)
	}

	// 2. DISK_HOG
	if last.DiskBusyPct >= 99.5 && topDisk.DiskMBps > 0 {
		totalProcIO := sumDiskIO(last.TopDisk)
		share := 0.0
		if totalProcIO > 0 {
			share = topDisk.DiskMBps / totalProcIO * 100
		}
		if share > 50 || (totalProcIO == 0 && topDisk.DiskMBps >= 20) {
			v := baseVerdict(VerdictDiskHog, topDisk, "high", 0.8)
			v.TextRu = fmt.Sprintf("Диск загружен ~100%%, %s даёт основную долю Process I/O (%.0f%%).", displayName(topDisk.Name), share)
			v.TextEn = fmt.Sprintf("Disk ~100%% busy; %s accounts for most Process I/O (%.0f%%).", displayName(topDisk.Name), share)
			v.Evidence = []string{
				fmt.Sprintf("Физическая активность диска ≈ %.0f%%", last.DiskBusyPct),
				fmt.Sprintf("Доля Process I/O у %s: ≈ %.0f%%", displayName(topDisk.Name), share),
				"Process I/O ≠ гарантированный объём записи на диск",
			}
			v.EvidenceRaw = []string{
				fmt.Sprintf("disk_busy=%.0f", last.DiskBusyPct),
				fmt.Sprintf("proc_io_share=%.0f", share),
				"note:process_io_not_physical_guarantee",
			}
			applySystemAction(&v)
			return finishVerdict(v)
		}
	}

	// 3. MEMORY_PRESSURE
	memPressure := last.RAMPercent > 95
	if !memPressure && last.RAMTotalMB > 0 && last.RAMAvailableMB < 300 {
		memPressure = true
	}
	if memPressure {
		culprit := topRAM
		v := baseVerdict(VerdictMemoryPressure, culprit, "critical", 0.75)
		v.TextRu = fmt.Sprintf("Давление на память: RAM %.0f%%. Крупнейший working set - %s. Не утверждаем, что он вызвал подкачку.", last.RAMPercent, displayName(culprit.Name))
		v.TextEn = fmt.Sprintf("Memory pressure: RAM %.0f%%. Largest working set is %s. Not claiming it caused paging.", last.RAMPercent, displayName(culprit.Name))
		v.Evidence = []string{
			fmt.Sprintf("RAM занято ≈ %.0f%%", last.RAMPercent),
			fmt.Sprintf("Свободно ≈ %d МБ", last.RAMAvailableMB),
		}
		v.EvidenceRaw = []string{
			fmt.Sprintf("ram=%.1f", last.RAMPercent),
			fmt.Sprintf("avail_mb=%d", last.RAMAvailableMB),
		}
		if last.SwapPercent > 10 {
			v.Evidence = append(v.Evidence, fmt.Sprintf("Подкачка ≈ %.0f%%", last.SwapPercent))
			v.EvidenceRaw = append(v.EvidenceRaw, fmt.Sprintf("swap=%.1f", last.SwapPercent))
		}
		applySystemAction(&v)
		return finishVerdict(v)
	}

	// 4. CPU_SATURATION - many processes, no single hog
	if last.CPUPercent > 70 && topCPU.CPUPercent <= 80 {
		top3 := topNCPU(last.TopCPU, 3)
		names := make([]string, 0, len(top3))
		targets := make([]EcoTarget, 0, len(top3))
		rawParts := make([]string, 0, len(top3)+1)
		rawParts = append(rawParts, fmt.Sprintf("cpu_total=%.1f", last.CPUPercent))
		for _, p := range top3 {
			names = append(names, fmt.Sprintf("%s (%.0f%%)", displayName(p.Name), p.CPUPercent))
			rawParts = append(rawParts, fmt.Sprintf("%s=%.1f", p.Name, p.CPUPercent))
			if p.Name != "" && !isCriticalSystemProc(p.Name) && !isEcoProtected(p.Name) && !isPPLProtected(p.Name) {
				targets = append(targets, EcoTarget{Name: p.Name, PID: p.PID, CPUPercent: p.CPUPercent})
			}
		}
		v := Verdict{
			Type:       VerdictCPUSaturation,
			Culprit:    "many_processes",
			Severity:   "medium",
			Confidence: 0.7,
			TextRu:     "Процессор загружен многими процессами",
			TextEn:     "CPU is saturated by many processes",
			Evidence: []string{
				fmt.Sprintf("Суммарная загрузка CPU ≈ %.0f%%", last.CPUPercent),
				"Ни один процесс не превышает порог одиночного CPU_HOG (>80%)",
			},
			EvidenceRaw: rawParts,
			EcoTargets:  targets,
		}
		if len(names) > 0 {
			v.Evidence = append(v.Evidence, "Топ по CPU: "+strings.Join(names, ", "))
		}
		if len(targets) > 0 {
			v.Action = "eco"
			v.Culprit = targets[0].Name
			v.CulpritPID = targets[0].PID
		} else {
			v.Action = "none"
			v.SystemProc = true
		}
		return finishVerdict(v)
	}

	// 5. THERMAL_THROTTLE
	peak := hints.PeakCPUFreq
	if peak <= 0 {
		peak = peakFreq(snaps)
	}
	curFreq := last.CPUFreqMHz
	temp := last.CPUTempC
	if peak > 0 && curFreq > 0 && temp >= 85 {
		drop := (peak - curFreq) / peak * 100
		if drop > 15 {
			v := Verdict{
				Type:       VerdictThermalThrottle,
				Culprit:    "CPU",
				Severity:   "high",
				Confidence: 0.7,
				TextRu:     fmt.Sprintf("Термотроттлинг: частота упала на %.0f%% при температуре %.0f°C.", drop, temp),
				TextEn:     fmt.Sprintf("Thermal throttle: frequency dropped %.0f%% at %.0f°C.", drop, temp),
				Action:     "none",
				Evidence: []string{
					fmt.Sprintf("Падение частоты ≈ %.0f%%", drop),
					fmt.Sprintf("Температура CPU ≈ %.0f°C", temp),
					fmt.Sprintf("Частота %.0f / пик %.0f МГц", curFreq, peak),
				},
				EvidenceRaw: []string{
					fmt.Sprintf("freq_drop=%.1f", drop),
					fmt.Sprintf("temp=%.1f", temp),
					fmt.Sprintf("freq=%.0f/%.0f", curFreq, peak),
				},
			}
			if topCPU.CPUPercent >= 50 && topCPU.Name != "" {
				v.Culprit = topCPU.Name
				v.CulpritPID = topCPU.PID
				v.SystemProc = isCriticalSystemProc(topCPU.Name)
				v.Evidence = append(v.Evidence, fmt.Sprintf("Горячий процесс: %s (%.0f%%)", displayName(topCPU.Name), topCPU.CPUPercent))
				v.EvidenceRaw = append(v.EvidenceRaw, fmt.Sprintf("hot_proc=%s:%.0f", topCPU.Name, topCPU.CPUPercent))
			}
			return finishVerdict(v)
		}
	}

	// 6. UNKNOWN - honest, no invented culprit
	evHuman := []string{"Согласованных признаков конкретной причины недостаточно"}
	evRaw := []string{"no_matching_rule"}
	if last.CPUPercent > 0 {
		evHuman = append(evHuman, fmt.Sprintf("CPU ≈ %.0f%%", last.CPUPercent))
		evRaw = append(evRaw, fmt.Sprintf("cpu=%.1f", last.CPUPercent))
	}
	if last.DiskBusyPct > 0 {
		evHuman = append(evHuman, fmt.Sprintf("Активность диска ≈ %.0f%%", last.DiskBusyPct))
		evRaw = append(evRaw, fmt.Sprintf("disk_busy=%.0f", last.DiskBusyPct))
	}
	if last.RAMPercent > 0 {
		evHuman = append(evHuman, fmt.Sprintf("RAM ≈ %.0f%%", last.RAMPercent))
		evRaw = append(evRaw, fmt.Sprintf("ram=%.1f", last.RAMPercent))
	}
	if hints.DurationMs > 0 {
		evHuman = append(evHuman, fmt.Sprintf("Задержка ≈ %.0f мс", hints.DurationMs))
		evRaw = append(evRaw, fmt.Sprintf("stall_ms=%.0f", hints.DurationMs))
	}
	return finishVerdict(Verdict{
		Type:        VerdictUnknown,
		Severity:    "medium",
		Confidence:  0.35,
		TextRu:      "Причина не определена. Виновник не назначен - недостаточно согласованных признаков.",
		TextEn:      "Cause undetermined. No culprit assigned - insufficient corroborating signals.",
		Action:      "none",
		Evidence:    evHuman,
		EvidenceRaw: evRaw,
	})
}

func finishVerdict(v Verdict) Verdict {
	if len(v.EcoTargets) == 0 && v.Action == "eco" && v.Culprit != "" && v.CulpritPID > 0 && !v.SystemProc {
		v.EcoTargets = []EcoTarget{{Name: v.Culprit, PID: v.CulpritPID}}
	}
	return v
}

func topNCPU(list []ProcessSnap, n int) []ProcessSnap {
	if n <= 0 || len(list) == 0 {
		return nil
	}
	cp := append([]ProcessSnap(nil), list...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].CPUPercent > cp[j].CPUPercent })
	if n > len(cp) {
		n = len(cp)
	}
	return cp[:n]
}

func anyLoadSign(snaps []Snapshot) bool {
	for _, s := range snaps {
		if s.HasLoadSign {
			return true
		}
		if s.CPUPercent >= 35 || s.DiskBusyPct >= 40 || s.RAMPercent >= 90 {
			return true
		}
		if s.DiskReadMBps+s.DiskWriteMBps >= 15 {
			return true
		}
	}
	return false
}

func latestSnap(snaps []Snapshot) Snapshot {
	if len(snaps) == 0 {
		return Snapshot{}
	}
	return snaps[len(snaps)-1]
}

func bestCPU(s Snapshot) ProcessSnap {
	var best ProcessSnap
	for _, p := range s.TopCPU {
		if p.CPUPercent > best.CPUPercent {
			best = p
		}
	}
	return best
}

func bestDisk(s Snapshot) ProcessSnap {
	var best ProcessSnap
	for _, p := range s.TopDisk {
		if p.DiskMBps > best.DiskMBps {
			best = p
		}
	}
	return best
}

func bestRAM(s Snapshot) ProcessSnap {
	var best ProcessSnap
	for _, p := range s.TopRAM {
		if p.RAMBytes > best.RAMBytes {
			best = p
		}
	}
	return best
}

func sumDiskIO(list []ProcessSnap) float64 {
	var t float64
	for _, p := range list {
		t += p.DiskMBps
	}
	return t
}

func peakFreq(snaps []Snapshot) float64 {
	var peak float64
	for _, s := range snaps {
		if s.CPUFreqMHz > peak {
			peak = s.CPUFreqMHz
		}
	}
	return peak
}

func baseVerdict(t VerdictType, p ProcessSnap, sev string, conf float64) Verdict {
	return Verdict{
		Type:       t,
		Culprit:    p.Name,
		CulpritPID: p.PID,
		Severity:   sev,
		Confidence: conf,
		SystemProc: isCriticalSystemProc(p.Name),
		Evidence:   nil,
	}
}

func applySystemAction(v *Verdict) {
	if v.SystemProc || v.Culprit == "" || strings.EqualFold(v.Culprit, "CPU") || v.Culprit == "many_processes" {
		v.Action = "none"
		return
	}
	if isEcoProtected(v.Culprit) || isPPLProtected(v.Culprit) {
		v.Action = "none"
		v.SystemProc = true
		return
	}
	v.Action = "eco"
}

func displayName(name string) string {
	if name == "" {
		return "unknown"
	}
	return friendlyProcessName(name)
}
