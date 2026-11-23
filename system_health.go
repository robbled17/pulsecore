package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	healthHistoryMax   = 48 // ~ half day at 15 min samples, or ring of snapshots
	healthSampleEvery  = 30 * time.Second
	healthPersistEvery = 2 * time.Minute
)

// HealthFactor is one documented contributor to the 0-100 score.
type HealthFactor struct {
	Key      string  `json:"key"`
	LabelRu  string  `json:"labelRu"`
	LabelEn  string  `json:"labelEn"`
	Score    float64 `json:"score"` // 0-100 for this factor
	Weight   float64 `json:"weight"`
	Available bool   `json:"available"`
	Status   string  `json:"status"` // ok|watch|action|unknown
	DetailRu string  `json:"detailRu"`
	DetailEn string  `json:"detailEn"`
}

// HealthHistoryPoint is a past overall score for the history strip.
type HealthHistoryPoint struct {
	At    string `json:"at"`
	Score int    `json:"score"`
}

// HealthDeduction explains how many points a factor cost vs a perfect 100.
type HealthDeduction struct {
	Key           string `json:"key"`
	LabelRu       string `json:"labelRu"`
	LabelEn       string `json:"labelEn"`
	Points        int    `json:"points"` // points lost from overall score
	DetailRu      string `json:"detailRu"`
	DetailEn      string `json:"detailEn"`
	ActionID      string `json:"actionId"`
	ActionLabelRu string `json:"actionLabelRu"`
	ActionLabelEn string `json:"actionLabelEn"`
}

// SystemHealthReport is the UI payload for «Здоровье системы».
type SystemHealthReport struct {
	Score       int                  `json:"score"`
	Incomplete  bool                 `json:"incomplete"`
	GradeRu     string               `json:"gradeRu"`
	GradeEn     string               `json:"gradeEn"`
	SummaryRu   string               `json:"summaryRu"`
	SummaryEn   string               `json:"summaryEn"`
	Factors     []HealthFactor       `json:"factors"`
	Deductions  []HealthDeduction    `json:"deductions"`
	OK          []string             `json:"ok"`
	Watch       []string             `json:"watch"`
	Action      []string             `json:"action"`
	History     []HealthHistoryPoint `json:"history"`
	ComputedAt  string               `json:"computedAt"`
	MissingNote string               `json:"missingNote"`
}

type healthSample struct {
	At             time.Time
	CPUPct         float64
	RAMPct         float64
	SwapPct        float64
	GPUPct         float64
	TempC          float64
	TempOK         bool
	GPUOK          bool
	DiskBusy       float64
	DiskOK         bool
	SystemDiskUsed float64 // % used on Windows system volume (C:)
	SystemDiskOK   bool
}

type healthStoreFile struct {
	History []HealthHistoryPoint `json:"history"`
}

type healthEngine struct {
	mu         sync.Mutex
	window     []healthSample
	history    []HealthHistoryPoint
	lastSample time.Time
	lastScore  int
	lastFlush  time.Time
	loaded     bool
	dirty      bool
}

func (a *App) ensureHealth() *healthEngine {
	if a.health == nil {
		a.health = &healthEngine{
			window:  make([]healthSample, 0, 40),
			history: make([]HealthHistoryPoint, 0, healthHistoryMax),
		}
	}
	return a.health
}

func healthPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "PulseCore", "health_history.json"), nil
}

func (a *App) loadHealthLocked(h *healthEngine) {
	if h.loaded {
		return
	}
	h.loaded = true
	path, err := healthPath()
	if err != nil {
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var f healthStoreFile
	if json.Unmarshal(b, &f) != nil {
		return
	}
	if len(f.History) > 0 {
		h.history = f.History
		if len(h.history) > healthHistoryMax {
			h.history = h.history[len(h.history)-healthHistoryMax:]
		}
	}
}

func (a *App) flushHealthLocked(h *healthEngine) {
	if !h.dirty {
		return
	}
	path, err := healthPath()
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	b, err := json.MarshalIndent(healthStoreFile{History: h.history}, "", "  ")
	if err != nil {
		return
	}
	if os.WriteFile(path, b, 0o644) == nil {
		h.dirty = false
		h.lastFlush = time.Now()
	}
}

// recordHealthTick stores a light sample for System Health (no extra collectors).
func (a *App) recordHealthTick(now time.Time, cpuPct, ramPct, swapPct, gpuPct, tempC, diskBusy, systemDiskUsed float64, tempOK, gpuOK, diskOK, systemDiskOK bool) {
	h := a.ensureHealth()
	h.mu.Lock()
	defer h.mu.Unlock()
	a.loadHealthLocked(h)
	if !h.lastSample.IsZero() && now.Sub(h.lastSample) < healthSampleEvery {
		return
	}
	h.lastSample = now
	h.window = append(h.window, healthSample{
		At: now, CPUPct: cpuPct, RAMPct: ramPct, SwapPct: swapPct,
		GPUPct: gpuPct, TempC: tempC, TempOK: tempOK, GPUOK: gpuOK,
		DiskBusy: diskBusy, DiskOK: diskOK,
		SystemDiskUsed: systemDiskUsed, SystemDiskOK: systemDiskOK,
	})
	cut := now.Add(-15 * time.Minute)
	i := 0
	for i < len(h.window) && h.window[i].At.Before(cut) {
		i++
	}
	if i > 0 {
		h.window = append([]healthSample(nil), h.window[i:]...)
	}
}

// GetSystemHealth builds the current health report from recent samples + soft signals.
func (a *App) GetSystemHealth() SystemHealthReport {
	h := a.ensureHealth()
	h.mu.Lock()
	a.loadHealthLocked(h)
	win := append([]healthSample(nil), h.window...)
	hist := append([]HealthHistoryPoint(nil), h.history...)
	h.mu.Unlock()

	a.mu.RLock()
	m := a.metrics
	tempMissing := a.tempUnavailable
	gpuMissing := a.nvidiaMissing
	a.mu.RUnlock()

	memCards := a.GetMemoryWatchCards()
	leakN := 0
	for _, c := range memCards {
		if c.Verdict == MemWatchGrowth || c.Verdict == MemWatchPossible {
			leakN++
		}
	}
	incidents := 0
	if cards := a.GetLagIncidents(); len(cards) > 0 {
		incidents = len(cards)
		if incidents > 8 {
			incidents = 8
		}
	}
	startupHigh := countHighImpactStartup(a.GetStartupItems())
	sysUsed, sysOK := systemDiskUsedPct(m.Disks)

	// Seed window from live metrics if still empty
	if len(win) == 0 {
		win = []healthSample{{
			At: time.Now(), CPUPct: m.CPUPercent, RAMPct: m.RAMPercent, SwapPct: m.SwapPercent,
			GPUPct: m.GPUPercent, TempC: m.TemperatureC, TempOK: m.TemperatureAvailable && m.TemperatureC > 0 && !tempMissing,
			GPUOK: !gpuMissing && m.GPUName != "",
			DiskBusy: diskBusyAvg(m.Disks), DiskOK: len(m.Disks) > 0,
			SystemDiskUsed: sysUsed, SystemDiskOK: sysOK,
		}}
	}

	rep := computeSystemHealth(win, leakN, incidents, startupHigh, hist)
	if smart, ok := a.cachedSystemDiskSMART(); ok {
		rep = applyDiskSMARTHealthFactor(rep, smart)
	}
	rep.ComputedAt = time.Now().Format("15:04:05")

	// Persist score into history occasionally
	h.mu.Lock()
	if h.lastScore != rep.Score || h.lastFlush.IsZero() || time.Since(h.lastFlush) >= healthPersistEvery {
		pt := HealthHistoryPoint{At: time.Now().Format("15:04"), Score: rep.Score}
		if len(h.history) == 0 || h.history[len(h.history)-1].Score != rep.Score ||
			time.Since(h.lastFlush) >= healthPersistEvery {
			h.history = append(h.history, pt)
			if len(h.history) > healthHistoryMax {
				h.history = h.history[len(h.history)-healthHistoryMax:]
			}
			h.dirty = true
			h.lastScore = rep.Score
			a.flushHealthLocked(h)
		}
	}
	rep.History = append([]HealthHistoryPoint(nil), h.history...)
	h.mu.Unlock()
	return rep
}

func diskBusyAvg(disks []DiskMetrics) float64 {
	if len(disks) == 0 {
		return 0
	}
	var sum float64
	for _, d := range disks {
		sum += d.ReadMBps + d.WriteMBps
	}
	busy, _ := diskBusyFromMBps(sum/2, sum/2) // rough - prefer real busy when available
	_ = busy
	// Approximate activity 0-100 from MB/s: 0→0, 40+→~80
	rate := sum
	if rate <= 0 {
		return 0
	}
	return math.Min(100, rate*2.5)
}

func systemDiskUsedPct(disks []DiskMetrics) (used float64, ok bool) {
	for _, d := range disks {
		m := strings.ToUpper(strings.TrimSpace(d.Mount))
		if strings.HasPrefix(m, "C:") || m == "C" {
			if d.Percent > 0 || d.TotalGB > 0 {
				return d.Percent, true
			}
		}
	}
	// Fallback: first fixed volume with capacity
	for _, d := range disks {
		if d.TotalGB >= 20 {
			return d.Percent, true
		}
	}
	return 0, false
}

func countHighImpactStartup(items []StartupItem) int {
	n := 0
	for _, it := range items {
		if !it.Enabled || it.Critical {
			continue
		}
		if it.Impact == "high" || it.Category == "launcher" || it.Category == "antivirus" {
			n++
		}
	}
	return n
}

// computeSystemHealth is pure scoring for unit tests.
// Missing factors are excluded from the weighted average (never treated as 0).
func computeSystemHealth(win []healthSample, leakN, incidents, startupHigh int, hist []HealthHistoryPoint) SystemHealthReport {
	avg := func(get func(healthSample) (float64, bool)) (float64, bool, float64) {
		var sum, peak float64
		n := 0
		for _, s := range win {
			v, ok := get(s)
			if !ok {
				continue
			}
			sum += v
			if v > peak {
				peak = v
			}
			n++
		}
		if n == 0 {
			return 0, false, 0
		}
		return sum / float64(n), true, peak
	}

	cpuAvg, cpuOK, cpuPeak := avg(func(s healthSample) (float64, bool) { return s.CPUPct, true })
	ramAvg, ramOK, ramPeak := avg(func(s healthSample) (float64, bool) { return s.RAMPct, true })
	swapAvg, swapOK, _ := avg(func(s healthSample) (float64, bool) { return s.SwapPct, true })
	tempAvg, tempOK, tempPeak := avg(func(s healthSample) (float64, bool) {
		if !s.TempOK || s.TempC <= 0 {
			return 0, false
		}
		return s.TempC, true
	})
	gpuAvg, gpuOK, gpuPeak := avg(func(s healthSample) (float64, bool) {
		if !s.GPUOK {
			return 0, false
		}
		return s.GPUPct, true
	})
	diskAvg, diskOK, diskPeak := avg(func(s healthSample) (float64, bool) {
		if !s.DiskOK {
			return 0, false
		}
		return s.DiskBusy, true
	})
	sysUsed, sysOK, _ := avg(func(s healthSample) (float64, bool) {
		if !s.SystemDiskOK {
			return 0, false
		}
		return s.SystemDiskUsed, true
	})

	factors := make([]HealthFactor, 0, 12)

	// CPU load - weight 1.2. High sustained load lowers score; short peaks less severe.
	if cpuOK {
		score := 100.0
		if cpuAvg > 55 {
			score -= (cpuAvg - 55) * 0.9
		}
		if cpuPeak > 92 {
			score -= 8
		}
		status, dru, den := "ok", "Средняя нагрузка CPU в норме.", "Average CPU load looks fine."
		if score < 70 {
			status, dru, den = "watch", fmt.Sprintf("CPU держит высокую среднюю нагрузку (≈%.0f%%).", cpuAvg), fmt.Sprintf("CPU average load is high (≈%.0f%%).", cpuAvg)
		}
		if score < 45 {
			status, dru, den = "action", fmt.Sprintf("CPU перегружен (среднее ≈%.0f%%, пик ≈%.0f%%).", cpuAvg, cpuPeak), fmt.Sprintf("CPU overloaded (avg ≈%.0f%%, peak ≈%.0f%%).", cpuAvg, cpuPeak)
		}
		factors = append(factors, HealthFactor{
			Key: "cpu_load", LabelRu: "Нагрузка CPU", LabelEn: "CPU load",
			Score: clamp01to100(score), Weight: 1.2, Available: true, Status: status,
			DetailRu: dru, DetailEn: den,
		})
	} else {
		factors = append(factors, missingFactor("cpu_load", "Нагрузка CPU", "CPU load", 1.2))
	}

	// CPU thermal - weight 1.4
	if tempOK {
		score := 100.0
		if tempAvg > 75 {
			score -= (tempAvg - 75) * 2.2
		}
		if tempPeak > 90 {
			score -= 15
		}
		status, dru, den := "ok", fmt.Sprintf("Температура CPU ≈%.0f°C.", tempAvg), fmt.Sprintf("CPU temperature ≈%.0f°C.", tempAvg)
		if score < 70 {
			status, dru, den = "watch", fmt.Sprintf("CPU теплее обычного (≈%.0f°C, пик %.0f°C).", tempAvg, tempPeak), fmt.Sprintf("CPU warmer than usual (≈%.0f°C, peak %.0f°C).", tempAvg, tempPeak)
		}
		if score < 45 || tempPeak >= 95 {
			status, dru, den = "action", fmt.Sprintf("Перегрев CPU (пик %.0f°C).", tempPeak), fmt.Sprintf("CPU overheating (peak %.0f°C).", tempPeak)
			score = math.Min(score, 40)
		}
		factors = append(factors, HealthFactor{
			Key: "cpu_temp", LabelRu: "Температура CPU", LabelEn: "CPU temperature",
			Score: clamp01to100(score), Weight: 1.4, Available: true, Status: status,
			DetailRu: dru, DetailEn: den,
		})
	} else {
		factors = append(factors, missingFactor("cpu_temp", "Температура CPU", "CPU temperature", 1.4))
	}

	// RAM - weight 1.3
	if ramOK {
		score := 100.0
		if ramAvg > 70 {
			score -= (ramAvg - 70) * 1.4
		}
		if ramPeak > 92 {
			score -= 12
		}
		if swapOK && swapAvg > 40 {
			score -= (swapAvg - 40) * 0.6
		}
		status, dru, den := "ok", fmt.Sprintf("RAM ≈%.0f%% - запас есть.", ramAvg), fmt.Sprintf("RAM ≈%.0f%% - headroom OK.", ramAvg)
		if score < 70 {
			status, dru, den = "watch", fmt.Sprintf("Память загружена (≈%.0f%%).", ramAvg), fmt.Sprintf("Memory is busy (≈%.0f%%).", ramAvg)
		}
		if score < 45 {
			status, dru, den = "action", fmt.Sprintf("Давление на RAM (≈%.0f%%, пик %.0f%%).", ramAvg, ramPeak), fmt.Sprintf("Memory pressure (≈%.0f%%, peak %.0f%%).", ramAvg, ramPeak)
		}
		factors = append(factors, HealthFactor{
			Key: "ram", LabelRu: "Оперативная память", LabelEn: "Memory",
			Score: clamp01to100(score), Weight: 1.3, Available: true, Status: status,
			DetailRu: dru, DetailEn: den,
		})
	} else {
		factors = append(factors, missingFactor("ram", "Оперативная память", "Memory", 1.3))
	}

	// GPU - weight 0.9 (optional)
	if gpuOK {
		score := 100.0
		if gpuAvg > 85 {
			score -= (gpuAvg - 85) * 1.2
		}
		if gpuPeak > 98 {
			score -= 6
		}
		status, dru, den := "ok", "GPU без признаков перегруза.", "GPU shows no overload signs."
		if score < 70 {
			status, dru, den = "watch", fmt.Sprintf("GPU часто высоконагружен (≈%.0f%%).", gpuAvg), fmt.Sprintf("GPU often heavily loaded (≈%.0f%%).", gpuAvg)
		}
		factors = append(factors, HealthFactor{
			Key: "gpu", LabelRu: "Видеокарта", LabelEn: "GPU",
			Score: clamp01to100(score), Weight: 0.9, Available: true, Status: status,
			DetailRu: dru, DetailEn: den,
		})
	} else {
		factors = append(factors, missingFactor("gpu", "Видеокарта", "GPU", 0.9))
	}

	// Disk activity - weight 0.7 (IO pressure, not free space)
	if diskOK {
		score := 100.0
		if diskAvg > 40 {
			score -= (diskAvg - 40) * 0.8
		}
		if diskPeak > 85 {
			score -= 10
		}
		status, dru, den := "ok", "Дисковая активность умеренная.", "Disk activity is moderate."
		if score < 70 {
			status, dru, den = "watch", "Диск часто занят - возможны подлагивания.", "Disk is often busy - may cause stutters."
		}
		if score < 45 {
			status, dru, den = "action", "Диск сильно нагружен.", "Disk is heavily loaded."
		}
		factors = append(factors, HealthFactor{
			Key: "disk_activity", LabelRu: "Активность диска", LabelEn: "Disk activity",
			Score: clamp01to100(score), Weight: 0.7, Available: true, Status: status,
			DetailRu: dru, DetailEn: den,
		})
	} else {
		factors = append(factors, missingFactor("disk_activity", "Активность диска", "Disk activity", 0.7))
	}

	// Free space on system disk - weight 1.5 (was wrongly ignored → 100/100 with 92% full)
	if sysOK {
		score := 100.0
		freePct := 100 - sysUsed
		if sysUsed >= 85 {
			score -= (sysUsed - 85) * 3.5
		}
		if sysUsed >= 90 {
			score -= 15
		}
		if sysUsed >= 95 {
			score -= 20
		}
		status, dru, den := "ok",
			fmt.Sprintf("На системном диске свободно ≈%.0f%%.", freePct),
			fmt.Sprintf("System disk has ≈%.0f%% free.", freePct)
		if sysUsed >= 85 {
			status, dru, den = "watch",
				fmt.Sprintf("Системный диск заполнен на %.0f%% - Windows может тормозить.", sysUsed),
				fmt.Sprintf("System disk is %.0f%% full - Windows may slow down.", sysUsed)
		}
		if sysUsed >= 90 {
			status, dru, den = "action",
				fmt.Sprintf("Системный диск почти полный (%.0f%%). Нужно освободить место.", sysUsed),
				fmt.Sprintf("System disk is nearly full (%.0f%%). Free up space.", sysUsed)
		}
		factors = append(factors, HealthFactor{
			Key: "system_disk", LabelRu: "Свободное место (диск Windows)", LabelEn: "Free space (Windows disk)",
			Score: clamp01to100(score), Weight: 1.5, Available: true, Status: status,
			DetailRu: dru, DetailEn: den,
		})
	} else {
		factors = append(factors, missingFactor("system_disk", "Свободное место (диск Windows)", "Free space (Windows disk)", 1.5))
	}

	// Recent lag incidents - weight 1.0
	lagScore := 100.0
	lagStatus, lru, len_ := "ok", "Недавних подтверждённых лагов нет.", "No recent confirmed stutters."
	if incidents > 0 {
		lagScore -= float64(incidents) * 10
		lagStatus = "watch"
		lru = fmt.Sprintf("Недавние фризы/лаги: %d.", incidents)
		len_ = fmt.Sprintf("Recent freezes/stutters: %d.", incidents)
	}
	if incidents >= 4 {
		lagStatus = "action"
	}
	factors = append(factors, HealthFactor{
		Key: "recent_lags", LabelRu: "Недавние лаги", LabelEn: "Recent stutters",
		Score: clamp01to100(lagScore), Weight: 1.0, Available: true, Status: lagStatus,
		DetailRu: lru, DetailEn: len_,
	})

	// Startup clutter - weight 0.8
	startScore := 100.0
	stStatus, sru2, sen2 := "ok", "Автозагрузка не перегружена тяжёлыми программами.", "Startup is not overloaded with heavy apps."
	if startupHigh > 0 {
		startScore -= float64(startupHigh) * 12
		stStatus = "watch"
		sru2 = fmt.Sprintf("В автозагрузке %d программ(ы) с высоким влиянием.", startupHigh)
		sen2 = fmt.Sprintf("%d high-impact startup program(s).", startupHigh)
	}
	if startupHigh >= 4 {
		stStatus = "action"
	}
	factors = append(factors, HealthFactor{
		Key: "startup", LabelRu: "Автозагрузка", LabelEn: "Startup",
		Score: clamp01to100(startScore), Weight: 0.8, Available: true, Status: stStatus,
		DetailRu: sru2, DetailEn: sen2,
	})

	// Soft signals: memory watch - weight 0.6
	soft := 100.0
	softStatus, sru, sen := "ok", "Долгосрочного роста памяти не видно.", "No long-term memory growth seen."
	if leakN > 0 {
		soft -= float64(leakN) * 12
		softStatus = "watch"
		sru = fmt.Sprintf("Наблюдается рост памяти у %d процесс(ов).", leakN)
		sen = fmt.Sprintf("Memory growth watched on %d process(es).", leakN)
	}
	if soft < 45 {
		softStatus = "action"
	}
	factors = append(factors, HealthFactor{
		Key: "signals", LabelRu: "Наблюдение за памятью", LabelEn: "Memory watch",
		Score: clamp01to100(soft), Weight: 0.6, Available: true, Status: softStatus,
		DetailRu: sru, DetailEn: sen,
	})

	var wSum, sSum float64
	missing := 0
	ok, watch, action := []string{}, []string{}, []string{}
	for _, f := range factors {
		if !f.Available {
			missing++
			continue
		}
		wSum += f.Weight
		sSum += f.Score * f.Weight
		label := f.LabelRu
		switch f.Status {
		case "ok":
			ok = append(ok, label)
		case "watch":
			watch = append(watch, label)
		case "action":
			action = append(action, label)
		}
	}
	score := 0
	incomplete := missing > 0 || wSum <= 0
	if wSum > 0 {
		score = int(math.Round(sSum / wSum))
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	deductions := buildDeductions(factors, score, wSum)
	if len(deductions) > 0 {
		sumPts := 0
		for _, d := range deductions {
			sumPts += d.Points
		}
		if sumPts > 0 && score != 100-sumPts {
			score = 100 - sumPts
		}
	}

	gradeRu, gradeEn := healthGrade(score)
	sumRu := fmt.Sprintf("%d/100 - %s", score, gradeRu)
	sumEn := fmt.Sprintf("%d/100 - %s", score, gradeEn)
	note := ""
	if incomplete {
		note = "Часть метрик недоступна - они не входят в расчёт и не считаются нулём."
	}

	return SystemHealthReport{
		Score: score, Incomplete: incomplete,
		GradeRu: gradeRu, GradeEn: gradeEn,
		SummaryRu: sumRu, SummaryEn: sumEn,
		Factors: factors, Deductions: deductions,
		OK: ok, Watch: watch, Action: action,
		History: hist, MissingNote: note,
	}
}

func missingFactor(key, ru, en string, w float64) HealthFactor {
	return HealthFactor{
		Key: key, LabelRu: ru, LabelEn: en, Weight: w, Available: false,
		Status: "unknown", Score: 0,
		DetailRu: "Данные недоступны - фактор не влияет на оценку.",
		DetailEn: "Data unavailable - factor excluded from score.",
	}
}

// applyDiskSMARTHealthFactor adds SMART health of the system disk into the overall score.
func applyDiskSMARTHealthFactor(rep SystemHealthReport, smart DiskHealthReport) SystemHealthReport {
	if !smart.OK || smart.StatusLevel == DiskStatusUnknown || smart.StatusLevel == "" {
		return rep
	}
	status := "ok"
	factorScore := 100.0
	if smart.HealthPercentOK {
		if smart.StatusLevel == DiskStatusGood && smart.HealthPercent >= 80.0 {
			factorScore = 100.0
		} else {
			factorScore = smart.HealthPercent
			if factorScore < 80.0 && status == "ok" {
				status = "watch"
			}
		}
	}
	switch smart.StatusLevel {
	case DiskStatusCaution:
		status = "watch"
		if factorScore > 85 {
			factorScore = 85
		}
	case DiskStatusBad:
		status = "action"
		if factorScore > 40 {
			factorScore = 40
		}
	}
	detailRu := smart.StatusWordRu
	detailEn := smart.StatusWordEn
	if smart.StatusReasonRu != "" {
		detailRu += " - " + smart.StatusReasonRu
		detailEn += " - " + smart.StatusReasonEn
	}
	if smart.HealthPercentOK {
		detailRu += fmt.Sprintf(" · здоровье %.0f%% (%s)", smart.HealthPercent, smart.HealthSourceRu)
		detailEn += fmt.Sprintf(" · health %.0f%% (%s)", smart.HealthPercent, smart.HealthSourceEn)
	}
	f := HealthFactor{
		Key: "disk_smart", LabelRu: "Здоровье диска", LabelEn: "Disk health",
		Score: clamp01to100(factorScore), Weight: 1.2, Available: true, Status: status,
		DetailRu: detailRu, DetailEn: detailEn,
	}
	out := make([]HealthFactor, 0, len(rep.Factors)+1)
	replaced := false
	for _, x := range rep.Factors {
		if x.Key == "disk_smart" {
			out = append(out, f)
			replaced = true
			continue
		}
		out = append(out, x)
	}
	if !replaced {
		out = append(out, f)
	}

	var wSum, sSum float64
	ok, watch, action := []string{}, []string{}, []string{}
	missing := 0
	for _, x := range out {
		if !x.Available {
			missing++
			continue
		}
		wSum += x.Weight
		sSum += x.Score * x.Weight
		switch x.Status {
		case "ok":
			ok = append(ok, x.LabelRu)
		case "watch":
			watch = append(watch, x.LabelRu)
		case "action":
			action = append(action, x.LabelRu)
		}
	}
	overall := rep.Score
	if wSum > 0 {
		overall = int(math.Round(sSum / wSum))
	}
	if overall < 0 {
		overall = 0
	}
	if overall > 100 {
		overall = 100
	}
	gradeRu, gradeEn := healthGrade(overall)
	rep.Factors = out
	rep.Score = overall
	rep.Incomplete = missing > 0 || wSum <= 0
	rep.GradeRu, rep.GradeEn = gradeRu, gradeEn
	rep.SummaryRu = fmt.Sprintf("%d/100 - %s", overall, gradeRu)
	rep.SummaryEn = fmt.Sprintf("%d/100 - %s", overall, gradeEn)
	rep.OK, rep.Watch, rep.Action = ok, watch, action
	rep.Deductions = buildDeductions(out, overall, wSum)
	if len(rep.Deductions) > 0 {
		sumPts := 0
		for _, d := range rep.Deductions {
			sumPts += d.Points
		}
		if sumPts > 0 && overall != 100-sumPts {
			overall = 100 - sumPts
			rep.Score = overall
			gradeRu, gradeEn = healthGrade(overall)
			rep.GradeRu, rep.GradeEn = gradeRu, gradeEn
			rep.SummaryRu = fmt.Sprintf("%d/100 - %s", overall, gradeRu)
			rep.SummaryEn = fmt.Sprintf("%d/100 - %s", overall, gradeEn)
		}
	}
	return rep
}

func buildDeductions(factors []HealthFactor, score int, wSum float64) []HealthDeduction {
	totalLost := 100 - score
	if wSum <= 0 || totalLost <= 0 {
		return nil
	}

	type factorLoss struct {
		f        HealthFactor
		rawLoss  float64
		detailRu string
		detailEn string
	}

	var losses []factorLoss
	for _, f := range factors {
		if !f.Available || f.Score >= 99.5 {
			continue
		}
		raw := (100.0 - f.Score) * f.Weight
		if raw <= 0 {
			continue
		}

		dru := f.DetailRu
		den := f.DetailEn

		switch f.Key {
		case "ram":
			dru = strings.TrimSuffix(f.DetailRu, " - запас есть.")
			dru = strings.TrimPrefix(dru, "RAM ")
			dru = "занято " + dru
			den = "usage: " + strings.TrimSuffix(f.DetailEn, " - headroom OK.")
		case "cpu_load":
			dru = strings.ReplaceAll(f.DetailRu, "Средняя нагрузка CPU в норме.", "повышенная нагрузка CPU")
			den = strings.ReplaceAll(f.DetailEn, "Average CPU load looks fine.", "elevated CPU load")
		case "cpu_temp":
			dru = strings.ReplaceAll(f.DetailRu, "Температура CPU ", "нагрев CPU ")
			den = strings.ReplaceAll(f.DetailEn, "CPU temperature ", "CPU temp ")
		}

		losses = append(losses, factorLoss{
			f: f, rawLoss: raw, detailRu: dru, detailEn: den,
		})
	}

	if len(losses) == 0 {
		return nil
	}

	sort.Slice(losses, func(i, j int) bool {
		return losses[i].rawLoss > losses[j].rawLoss
	})

	// Each deduction shown must represent at least 1 point lost.
	// Therefore we display at most totalLost items.
	if len(losses) > totalLost {
		losses = losses[:totalLost]
	}

	var subRaw float64
	for _, fl := range losses {
		subRaw += fl.rawLoss
	}
	if subRaw <= 0 {
		return nil
	}

	type lossAlloc struct {
		fl    factorLoss
		pts   int
		fract float64
		idx   int
	}

	allocs := make([]lossAlloc, len(losses))
	allocated := 0
	extra := totalLost - len(losses) // remaining points to distribute proportionally

	for i, fl := range losses {
		exact := float64(extra) * (fl.rawLoss / subRaw)
		floor := int(exact)
		allocs[i] = lossAlloc{
			fl:    fl,
			pts:   1 + floor, // guaranteed at least 1 point
			fract: exact - float64(floor),
			idx:   i,
		}
		allocated += 1 + floor
	}

	rem := totalLost - allocated
	if rem > 0 {
		sort.Slice(allocs, func(i, j int) bool {
			return allocs[i].fract > allocs[j].fract
		})
		for i := 0; i < rem && i < len(allocs); i++ {
			allocs[i].pts++
		}
		sort.Slice(allocs, func(i, j int) bool {
			return allocs[i].idx < allocs[j].idx
		})
	}

	deductions := make([]HealthDeduction, len(allocs))
	for i, a := range allocs {
		actID, actRu, actEn := deductionAction(a.fl.f.Key)
		deductions[i] = HealthDeduction{
			Key:           a.fl.f.Key,
			LabelRu:       a.fl.f.LabelRu,
			LabelEn:       a.fl.f.LabelEn,
			Points:        a.pts,
			DetailRu:      a.fl.detailRu,
			DetailEn:      a.fl.detailEn,
			ActionID:      actID,
			ActionLabelRu: actRu,
			ActionLabelEn: actEn,
		}
	}

	return deductions
}

func deductionAction(key string) (actionID, labelRu, labelEn string) {
	switch key {
	case "system_disk":
		return "free_disk", "Очистить", "Clean up"
	case "ram", "signals":
		return "free_ram", "Освободить", "Free up"
	case "cpu_load":
		return "eco_mode", "Ограничить", "Throttle"
	case "startup":
		return "open_startup", "Настроить", "Configure"
	case "cpu_temp":
		return "inspect_cpu", "Проверить", "Inspect"
	case "gpu":
		return "inspect_gpu", "Проверить", "Inspect"
	case "disk_activity":
		return "inspect_disk", "Проверить", "Inspect"
	case "recent_lags":
		return "open_lag", "Лаги", "Stutters"
	case "disk_smart":
		return "open_smart", "Проверить здоровье диска", "Check drive health"
	default:
		return "", "", ""
	}
}

func healthGrade(score int) (string, string) {
	switch {
	case score >= 85:
		return "состояние хорошее", "condition is good"
	case score >= 70:
		return "состояние нормальное", "condition is fair"
	case score >= 50:
		return "есть замечания", "needs attention"
	default:
		return "требуется вмешательство", "needs intervention"
	}
}
