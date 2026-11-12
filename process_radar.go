package main

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	radarMinSamples     = 12
	radarMaxProfiles    = 80
	radarMaxAnomalies   = 24
	radarSustainTicks   = 5
	radarCPUMult        = 4.0
	radarRAMGrowPct     = 0.85 // +85% vs baseline
	radarDiskMult       = 5.0
	radarEMA            = 0.12
	radarHoldDuration   = 10 * time.Minute
	radarMinCPUPct      = 25.0
	radarMinDiskMBps    = 12.0
	radarMinRAMMB       = 400.0
)

// ProcessScenario groups comparable conditions.
type ProcessScenario string

const (
	ScenarioIdle ProcessScenario = "idle"
	ScenarioLoad ProcessScenario = "load"
	ScenarioGame ProcessScenario = "game"
)

// ProcessAnomaly is a non-scary deviation card for Process Radar.
type ProcessAnomaly struct {
	ID           string    `json:"id"`
	Process      string    `json:"process"`
	Friendly     string    `json:"friendly"`
	Type         string    `json:"type"` // ram_spike|cpu_spike|disk_spike|orphan_load|ram_creep
	Current      float64   `json:"current"`
	Baseline     float64   `json:"baseline"`
	DeltaPct     float64   `json:"deltaPct"`
	Unit         string    `json:"unit"` // percent|mb|mbps
	DetectedAt   string    `json:"detectedAt"`
	DetectedUnix int64     `json:"detectedUnix"`
	Confidence   float64   `json:"confidence"`
	TextRu       string    `json:"textRu"`
	TextEn       string    `json:"textEn"`
	Scenario     string    `json:"scenario"`
	Hidden       bool      `json:"hidden"`
	ExpiresUnix  int64     `json:"expiresUnix"`
	SustainTicks int       `json:"-"`
	seenAt       time.Time `json:"-"`
}

type procProfile struct {
	Name     string
	Scenario ProcessScenario
	CPUEMA   float64
	RAMEMA   float64 // MB
	DiskEMA  float64
	Samples  int
	LastSeen time.Time
}

type processRadar struct {
	mu        sync.Mutex
	profiles  map[string]*procProfile // key = scenario|exe
	anomalies []ProcessAnomaly
	pending   map[string]int // sustain counters
	hidden    map[string]time.Time
}

func newProcessRadar() *processRadar {
	return &processRadar{
		profiles:  map[string]*procProfile{},
		anomalies: make([]ProcessAnomaly, 0, radarMaxAnomalies),
		pending:   map[string]int{},
		hidden:    map[string]time.Time{},
	}
}

func radarHideKey(a ProcessAnomaly) string {
	return a.Type + "|" + strings.ToLower(filepath.Base(a.Process))
}

func isNoisyDevTool(exe string) bool {
	b := strings.ToLower(filepath.Base(exe))
	switch b {
	case "code.exe", "cursor.exe", "devenv.exe", "idea64.exe", "rider64.exe",
		"pycharm64.exe", "webstorm64.exe", "clion64.exe", "goland64.exe",
		"studio64.exe", "atom.exe", "sublime_text.exe", "notepad++.exe",
		"msedgewebview2.exe", "searchhost.exe", "searchapp.exe":
		return true
	default:
		return false
	}
}

func (a *App) ensureRadar() *processRadar {
	if a.radar == nil {
		a.radar = newProcessRadar()
	}
	return a.radar
}

func radarKey(scenario ProcessScenario, exe string) string {
	return string(scenario) + "|" + strings.ToLower(filepath.Base(exe))
}

func currentScenario(game bool, cpuPct float64) ProcessScenario {
	if game {
		return ScenarioGame
	}
	if cpuPct >= 45 {
		return ScenarioLoad
	}
	return ScenarioIdle
}

// ObserveProcesses updates baselines and may raise anomalies (no single-spike reactions).
func (a *App) ObserveProcesses(snaps []ProcessSnap, scenario ProcessScenario, now time.Time) {
	if len(snaps) == 0 {
		return
	}
	r := a.ensureRadar()
	r.mu.Lock()
	defer r.mu.Unlock()

	seen := map[string]bool{}
	for _, p := range snaps {
		base := strings.ToLower(filepath.Base(p.Name))
		if base == "" || isSelfOrWebView(base) || base == "idle" || base == "system" {
			continue
		}
		key := radarKey(scenario, base)
		seen[key] = true
		ramMB := float64(p.RAMBytes) / (1024 * 1024)
		prof := r.profiles[key]
		if prof == nil {
			prof = &procProfile{Name: p.Name, Scenario: scenario}
			r.profiles[key] = prof
		}
		baseCPU, baseRAM, baseDisk := prof.CPUEMA, prof.RAMEMA, prof.DiskEMA
		ready := prof.Samples >= radarMinSamples

		// Skip typical IDE / editor noise for mild CPU spikes - keep heavy disk/RAM.
		skipMildCPU := isNoisyDevTool(base)
		cpuHot := ready && !skipMildCPU && baseCPU > 2 && p.CPUPercent >= math.Max(radarMinCPUPct, baseCPU*radarCPUMult)
		ramHot := ready && baseRAM > radarMinRAMMB && ramMB >= baseRAM*(1+radarRAMGrowPct) && (ramMB-baseRAM) >= 250
		diskHot := ready && baseDisk > 0.8 && p.DiskMBps >= math.Max(radarMinDiskMBps, baseDisk*radarDiskMult)

		// Do not let sustained spikes wash out the baseline before sustain ticks fire.
		if prof.Samples == 0 {
			prof.CPUEMA = p.CPUPercent
			prof.RAMEMA = ramMB
			prof.DiskEMA = p.DiskMBps
		} else if !cpuHot && !ramHot && !diskHot {
			prof.CPUEMA = radarEMA*p.CPUPercent + (1-radarEMA)*prof.CPUEMA
			prof.RAMEMA = radarEMA*ramMB + (1-radarEMA)*prof.RAMEMA
			prof.DiskEMA = radarEMA*p.DiskMBps + (1-radarEMA)*prof.DiskEMA
		}
		prof.Samples++
		prof.LastSeen = now
		prof.Name = p.Name

		if !ready {
			continue
		}

		if cpuHot {
			r.considerLocked(ProcessAnomaly{
				Process:  p.Name,
				Type:     "cpu_spike",
				Current:  p.CPUPercent,
				Baseline: baseCPU,
				Unit:     "percent",
				Scenario: string(scenario),
			}, now)
		}
		if ramHot {
			r.considerLocked(ProcessAnomaly{
				Process:  p.Name,
				Type:     "ram_spike",
				Current:  ramMB,
				Baseline: baseRAM,
				Unit:     "mb",
				Scenario: string(scenario),
			}, now)
		}
		if diskHot {
			r.considerLocked(ProcessAnomaly{
				Process:  p.Name,
				Type:     "disk_spike",
				Current:  p.DiskMBps,
				Baseline: baseDisk,
				Unit:     "mbps",
				Scenario: string(scenario),
			}, now)
		}
	}

	// orphan load: process still hot after user apps typically idle - soft heuristic
	if scenario == ScenarioIdle {
		for _, p := range snaps {
			base := strings.ToLower(filepath.Base(p.Name))
			if isCriticalSystemProc(base) || isSelfOrWebView(base) {
				continue
			}
			if isNoisyDevTool(base) {
				continue
			}
			if p.CPUPercent >= 35 {
				r.considerLocked(ProcessAnomaly{
					Process:  p.Name,
					Type:     "orphan_load",
					Current:  p.CPUPercent,
					Baseline: 5,
					Unit:     "percent",
					Scenario: string(scenario),
				}, now)
			}
		}
	}

	// prune stale profiles
	if len(r.profiles) > radarMaxProfiles {
		type kv struct {
			k string
			t time.Time
		}
		arr := make([]kv, 0, len(r.profiles))
		for k, p := range r.profiles {
			arr = append(arr, kv{k, p.LastSeen})
		}
		sort.Slice(arr, func(i, j int) bool { return arr[i].t.Before(arr[j].t) })
		for i := 0; i < len(arr)-radarMaxProfiles; i++ {
			delete(r.profiles, arr[i].k)
		}
	}
	_ = seen
}

func (r *processRadar) considerLocked(a ProcessAnomaly, now time.Time) {
	key := a.Type + "|" + strings.ToLower(filepath.Base(a.Process)) + "|" + a.Scenario
	hk := radarHideKey(a)
	if until, ok := r.hidden[hk]; ok && now.Before(until) {
		return
	}
	r.pending[key]++
	if r.pending[key] < radarSustainTicks {
		return
	}
	r.pending[key] = 0
	if a.Baseline > 0 {
		a.DeltaPct = (a.Current - a.Baseline) / a.Baseline * 100
	}
	a.Friendly = friendlyProcessName(a.Process)
	a.DetectedAt = now.Format("15:04")
	a.DetectedUnix = now.Unix()
	a.ExpiresUnix = now.Add(radarHoldDuration).Unix()
	a.seenAt = now
	a.ID = fmt.Sprintf("%s-%d", key, now.Unix())
	a.Confidence = clamp01to100(55+math.Min(35, math.Abs(a.DeltaPct)/4)) / 100
	a.TextRu, a.TextEn = anomalyTexts(a)

	// Refresh existing card of same type+process (hold, do not flicker away).
	for i := range r.anomalies {
		old := &r.anomalies[i]
		if strings.EqualFold(old.Process, a.Process) && old.Type == a.Type {
			a.ID = old.ID
			a.DetectedAt = old.DetectedAt
			a.DetectedUnix = old.DetectedUnix
			r.anomalies[i] = a
			return
		}
	}
	r.anomalies = append([]ProcessAnomaly{a}, r.anomalies...)
	if len(r.anomalies) > radarMaxAnomalies {
		r.anomalies = r.anomalies[:radarMaxAnomalies]
	}
}

func anomalyTexts(a ProcessAnomaly) (string, string) {
	name := friendlyProcessName(a.Process)
	switch a.Type {
	case "ram_spike":
		return fmt.Sprintf("%s занимает больше памяти: %.0f МБ", name, a.Current),
			fmt.Sprintf("%s is using more memory: %.0f MB", name, a.Current)
	case "cpu_spike":
		return fmt.Sprintf("%s сильно нагружает процессор (%.0f%%)", name, a.Current),
			fmt.Sprintf("%s is heavily loading the CPU (%.0f%%)", name, a.Current)
	case "disk_spike":
		return fmt.Sprintf("%s активно пишет на диск (%.0f МБ/с)", name, a.Current),
			fmt.Sprintf("%s is writing heavily to disk (%.0f MB/s)", name, a.Current)
	case "orphan_load":
		return fmt.Sprintf("%s грузит процессор в фоне (%.0f%%)", name, a.Current),
			fmt.Sprintf("%s is loading the CPU in the background (%.0f%%)", name, a.Current)
	case "ram_creep":
		return fmt.Sprintf("%s постепенно увеличил память до %.0f МБ", name, a.Current),
			fmt.Sprintf("%s gradually grew memory to %.0f MB", name, a.Current)
	default:
		return fmt.Sprintf("%s влияет на систему", name), fmt.Sprintf("%s is affecting the system", name)
	}
}

func (r *processRadar) pruneExpiredLocked(now time.Time) {
	kept := r.anomalies[:0]
	for _, a := range r.anomalies {
		if a.Hidden {
			continue
		}
		if a.ExpiresUnix > 0 && now.Unix() > a.ExpiresUnix {
			continue
		}
		if !a.seenAt.IsZero() && now.Sub(a.seenAt) > radarHoldDuration {
			continue
		}
		kept = append(kept, a)
	}
	r.anomalies = kept
}

// GetProcessAnomalies returns recent Process Radar cards (held ≥10 min unless hidden).
func (a *App) GetProcessAnomalies() []ProcessAnomaly {
	r := a.ensureRadar()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneExpiredLocked(time.Now())
	out := make([]ProcessAnomaly, 0, len(r.anomalies))
	for _, an := range r.anomalies {
		if an.Hidden {
			continue
		}
		out = append(out, an)
	}
	return out
}

// ClearProcessAnomalies hides current cards for the hold window (not baselines).
func (a *App) ClearProcessAnomalies() {
	r := a.ensureRadar()
	r.mu.Lock()
	now := time.Now()
	for _, an := range r.anomalies {
		r.hidden[radarHideKey(an)] = now.Add(radarHoldDuration)
	}
	r.anomalies = r.anomalies[:0]
	r.pending = map[string]int{}
	r.mu.Unlock()
}

func (a *App) isGameSessionActive() bool {
	gb := a.ensureGameBook()
	gb.mu.Lock()
	defer gb.mu.Unlock()
	return gb.active != nil && gb.active.Active
}

// observeProcessRadar feeds tops into Process Radar + Memory Leak Watch.
func (a *App) observeProcessRadar(now time.Time, cpuPct float64, topCPU, topDisk, topRAM []ProcessSnap) {
	merged := mergeProcessSnaps(topCPU, topDisk, topRAM)
	if len(merged) == 0 {
		return
	}
	sc := currentScenario(a.isGameSessionActive(), cpuPct)
	a.ObserveProcesses(merged, sc, now)
	a.ObserveMemoryGrowth(merged, now)
}

func mergeProcessSnaps(lists ...[]ProcessSnap) []ProcessSnap {
	by := map[string]ProcessSnap{}
	order := make([]string, 0, 16)
	for _, list := range lists {
		for _, p := range list {
			base := strings.ToLower(filepath.Base(p.Name))
			if base == "" {
				continue
			}
			if old, ok := by[base]; ok {
				if p.CPUPercent > old.CPUPercent {
					old.CPUPercent = p.CPUPercent
				}
				if p.DiskMBps > old.DiskMBps {
					old.DiskMBps = p.DiskMBps
				}
				if p.RAMBytes > old.RAMBytes {
					old.RAMBytes = p.RAMBytes
				}
				if p.PID != 0 {
					old.PID = p.PID
				}
				by[base] = old
				continue
			}
			by[base] = p
			order = append(order, base)
		}
	}
	out := make([]ProcessSnap, 0, len(order))
	for _, k := range order {
		out = append(out, by[k])
	}
	return out
}
