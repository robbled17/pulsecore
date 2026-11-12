package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/robbled17/zeropulse/loc"
)

const (
	statsMaxDays       = 90
	statsFlushInterval = time.Minute
	highLoadCPUPct     = 60.0
	highLoadGPUPct     = 45.0
)

// DayStats is one calendar day of aggregated PulseCore observations (no raw snapshots).
type DayStats struct {
	Date              string             `json:"date"` // YYYY-MM-DD local
	CPUTempMax        float64            `json:"cpuTempMax"`
	CPUTempSum        float64            `json:"cpuTempSum"`
	CPUTempN          int                `json:"cpuTempN"`
	GPUTempMax        float64            `json:"gpuTempMax"`
	GPUTempSum        float64            `json:"gpuTempSum"`
	GPUTempN          int                `json:"gpuTempN"`
	Incidents         int                `json:"incidents"`
	Culprits          map[string]int     `json:"culprits"`
	GameMinutes       map[string]float64 `json:"gameMinutes"`
	HighLoadMinutes   float64            `json:"highLoadMinutes"`
	EcoRAMFreedMB     float64            `json:"ecoRAMFreedMB"`
	EcoOptimizations  int                `json:"ecoOptimizations"`
	CPUTempAvg        float64            `json:"cpuTempAvg"`
	GPUTempAvg        float64            `json:"gpuTempAvg"`
}

// StatsStore is the on-disk weekly/daily aggregate file.
type StatsStore struct {
	Days           []DayStats `json:"days"`
	GameExclusions []string   `json:"gameExclusions"`
}

// WeeklySummary is the UI payload for the Summary tab.
type WeeklySummary struct {
	OK              bool               `json:"ok"`
	Insufficient    bool               `json:"insufficient"`
	Message         loc.Msg            `json:"message"`
	MessageRu       string             `json:"messageRu"`
	MessageEn       string             `json:"messageEn"`
	GameHours       float64            `json:"gameHours"`
	PrevGameHours   float64            `json:"prevGameHours"`
	Freezes         int                `json:"freezes"`
	PrevFreezes     int                `json:"prevFreezes"`
	CPUTempPeak     float64            `json:"cpuTempPeak"`
	PrevCPUTempPeak float64            `json:"prevCPUTempPeak"`
	GPUTempPeak     float64            `json:"gpuTempPeak"`
	PrevGPUTempPeak float64            `json:"prevGPUTempPeak"`
	StableDay       string             `json:"stableDay"`
	TopCulprit      string             `json:"topCulprit"`
	EcoRAMFreedMB   float64            `json:"ecoRAMFreedMB"`
	PrevEcoRAMMB    float64            `json:"prevEcoRAMMB"`
	EcoCount        int                `json:"ecoCount"`
	PrevEcoCount    int                `json:"prevEcoCount"`
	HighLoadMin     float64            `json:"highLoadMin"`
	PrevHighLoadMin float64            `json:"prevHighLoadMin"`
	DaysObserved    int                `json:"daysObserved"`
	Series          []DayStats         `json:"series"` // last 7 days chronological
	GameExclusions  []string           `json:"gameExclusions"`
}

type statsEngine struct {
	mu       sync.Mutex
	store    StatsStore
	dirty    bool
	lastFlush time.Time
	loaded   bool
}

func defaultGameExclusions() []string {
	return []string{
		"chrome.exe", "msedge.exe", "firefox.exe", "brave.exe", "opera.exe",
		"code.exe", "devenv.exe", "idea64.exe", "pycharm64.exe", "cursor.exe",
		"discord.exe", "slack.exe", "teams.exe", "zoom.exe",
		"explorer.exe", "dwm.exe", "pulsecore.exe",
	}
}

func statsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "PulseCore", "stats.json"), nil
}

func (a *App) ensureStats() *statsEngine {
	if a.stats == nil {
		a.stats = &statsEngine{store: StatsStore{Days: []DayStats{}, GameExclusions: defaultGameExclusions()}}
	}
	return a.stats
}

func (a *App) loadStats() {
	st := a.ensureStats()
	st.mu.Lock()
	defer st.mu.Unlock()
	path, err := statsPath()
	if err != nil {
		st.loaded = true
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		st.store.GameExclusions = defaultGameExclusions()
		st.loaded = true
		return
	}
	var file StatsStore
	if json.Unmarshal(raw, &file) != nil {
		st.store.GameExclusions = defaultGameExclusions()
		st.loaded = true
		return
	}
	if len(file.GameExclusions) == 0 {
		file.GameExclusions = defaultGameExclusions()
	}
	st.store = file
	st.pruneLocked()
	st.loaded = true
}

func (s *statsEngine) pruneLocked() {
	if len(s.store.Days) <= statsMaxDays {
		return
	}
	sort.Slice(s.store.Days, func(i, j int) bool { return s.store.Days[i].Date < s.store.Days[j].Date })
	s.store.Days = s.store.Days[len(s.store.Days)-statsMaxDays:]
}

func (a *App) flushStats(force bool) {
	st := a.ensureStats()
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.dirty && !force {
		return
	}
	if !force && !st.lastFlush.IsZero() && time.Since(st.lastFlush) < statsFlushInterval {
		return
	}
	path, err := statsPath()
	if err != nil {
		return
	}
	st.pruneLocked()
	raw, err := json.MarshalIndent(st.store, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return
	}
	st.dirty = false
	st.lastFlush = time.Now()
}

func (s *statsEngine) todayLocked() *DayStats {
	date := time.Now().Format("2006-01-02")
	for i := range s.store.Days {
		if s.store.Days[i].Date == date {
			if s.store.Days[i].Culprits == nil {
				s.store.Days[i].Culprits = map[string]int{}
			}
			if s.store.Days[i].GameMinutes == nil {
				s.store.Days[i].GameMinutes = map[string]float64{}
			}
			return &s.store.Days[i]
		}
	}
	d := DayStats{
		Date:        date,
		Culprits:    map[string]int{},
		GameMinutes: map[string]float64{},
	}
	s.store.Days = append(s.store.Days, d)
	return &s.store.Days[len(s.store.Days)-1]
}

// recordStatsTick aggregates one metrics sample into today's day stats.
func (a *App) recordStatsTick(cpuPct, gpuPct, cpuTemp, gpuTemp float64, intervalSec float64) {
	if intervalSec <= 0 {
		intervalSec = 1
	}
	st := a.ensureStats()
	st.mu.Lock()
	d := st.todayLocked()
	if cpuTemp > 0 {
		if cpuTemp > d.CPUTempMax {
			d.CPUTempMax = cpuTemp
		}
		d.CPUTempSum += cpuTemp
		d.CPUTempN++
	}
	if gpuTemp > 0 {
		if gpuTemp > d.GPUTempMax {
			d.GPUTempMax = gpuTemp
		}
		d.GPUTempSum += gpuTemp
		d.GPUTempN++
	}
	if cpuPct >= highLoadCPUPct || gpuPct >= highLoadGPUPct {
		d.HighLoadMinutes += intervalSec / 60.0
	}
	st.dirty = true
	st.mu.Unlock()
	a.flushStats(false)
}

func (a *App) recordStatsIncident(culprit string) {
	st := a.ensureStats()
	st.mu.Lock()
	d := st.todayLocked()
	d.Incidents++
	name := strings.TrimSpace(culprit)
	if name != "" && name != "many_processes" && !strings.EqualFold(name, "CPU") {
		d.Culprits[filepath.Base(name)]++
	}
	st.dirty = true
	st.mu.Unlock()
}

func (a *App) recordStatsEco(ramFreedMB float64) {
	st := a.ensureStats()
	st.mu.Lock()
	d := st.todayLocked()
	d.EcoOptimizations++
	if ramFreedMB > 0 {
		d.EcoRAMFreedMB += ramFreedMB
	}
	st.dirty = true
	st.mu.Unlock()
}

func (a *App) recordStatsGameMinutes(exe string, minutes float64) {
	if minutes <= 0 {
		return
	}
	base := strings.ToLower(filepath.Base(exe))
	if base == "" {
		return
	}
	st := a.ensureStats()
	st.mu.Lock()
	for _, ex := range st.store.GameExclusions {
		if strings.ToLower(filepath.Base(ex)) == base {
			st.mu.Unlock()
			return
		}
	}
	d := st.todayLocked()
	d.GameMinutes[base] += minutes
	st.dirty = true
	st.mu.Unlock()
}

func avgTemp(sum float64, n int) float64 {
	if n <= 0 {
		return 0
	}
	return sum / float64(n)
}

func (a *App) GetWeeklySummary() WeeklySummary {
	st := a.ensureStats()
	st.mu.Lock()
	defer st.mu.Unlock()

	sort.Slice(st.store.Days, func(i, j int) bool { return st.store.Days[i].Date < st.store.Days[j].Date })
	days := append([]DayStats(nil), st.store.Days...)
	observed := len(days)
	out := WeeklySummary{
		OK:             true,
		DaysObserved:   observed,
		GameExclusions: append([]string(nil), st.store.GameExclusions...),
	}
	if observed < 2 {
		out.Insufficient = true
		out.Message = loc.M("statsInsufficient", nil)
		out.MessageRu = "Данных пока мало, продолжаем наблюдение"
		out.MessageEn = "Not enough data yet - still observing"
		if observed == 1 {
			out.Series = days
		}
		return out
	}

	last7 := enrichDayAvgs(takeLastDays(days, 7))
	prev7 := enrichDayAvgs(takePrevDays(days, 7, 7))
	out.Series = last7

	agg := func(list []DayStats) (gameHours float64, freezes int, cpuPeak, gpuPeak, ecoRAM float64, ecoN int, highMin float64, culprits map[string]int, stable string) {
		culprits = map[string]int{}
		minInc := int(^uint(0) >> 1)
		for _, d := range list {
			for k, v := range d.GameMinutes {
				gameHours += v / 60.0
				_ = k
			}
			freezes += d.Incidents
			if d.CPUTempMax > cpuPeak {
				cpuPeak = d.CPUTempMax
			}
			if d.GPUTempMax > gpuPeak {
				gpuPeak = d.GPUTempMax
			}
			ecoRAM += d.EcoRAMFreedMB
			ecoN += d.EcoOptimizations
			highMin += d.HighLoadMinutes
			for c, n := range d.Culprits {
				culprits[c] += n
			}
			if d.Incidents < minInc {
				minInc = d.Incidents
				stable = d.Date
			}
		}
		return
	}

	g, f, cp, gp, er, en, hm, cul, stable := agg(last7)
	pg, pf, pcp, pgp, per, pen, phm, _, _ := agg(prev7)
	out.GameHours, out.PrevGameHours = g, pg
	out.Freezes, out.PrevFreezes = f, pf
	out.CPUTempPeak, out.PrevCPUTempPeak = cp, pcp
	out.GPUTempPeak, out.PrevGPUTempPeak = gp, pgp
	out.EcoRAMFreedMB, out.PrevEcoRAMMB = er, per
	out.EcoCount, out.PrevEcoCount = en, pen
	out.HighLoadMin, out.PrevHighLoadMin = hm, phm
	out.StableDay = stable
	out.TopCulprit = topCulprit(cul)
	if len(prev7) == 0 {
		out.Message = loc.M("statsNoWeekCompare", nil)
		out.MessageRu = "Сравнение с предыдущей неделей появится после накопления данных"
		out.MessageEn = "Comparison with the previous week will appear after more data is collected"
	} else {
		out.Message = loc.M("statsObserving", loc.P("days", observed))
	}
	return out
}

func takeLastDays(days []DayStats, n int) []DayStats {
	if n <= 0 || len(days) == 0 {
		return nil
	}
	if len(days) <= n {
		return append([]DayStats(nil), days...)
	}
	return append([]DayStats(nil), days[len(days)-n:]...)
}

func takePrevDays(days []DayStats, skip, n int) []DayStats {
	if len(days) <= skip {
		return nil
	}
	end := len(days) - skip
	start := end - n
	if start < 0 {
		start = 0
	}
	return append([]DayStats(nil), days[start:end]...)
}

func enrichDayAvgs(days []DayStats) []DayStats {
	out := make([]DayStats, len(days))
	for i, d := range days {
		d.CPUTempAvg = avgTemp(d.CPUTempSum, d.CPUTempN)
		d.GPUTempAvg = avgTemp(d.GPUTempSum, d.GPUTempN)
		if d.Culprits == nil {
			d.Culprits = map[string]int{}
		}
		if d.GameMinutes == nil {
			d.GameMinutes = map[string]float64{}
		}
		out[i] = d
	}
	return out
}

func topCulprit(m map[string]int) string {
	best := ""
	bestN := 0
	for k, v := range m {
		if v > bestN {
			best = k
			bestN = v
		}
	}
	return best
}

// ClearStats deletes only PulseCore weekly statistics (not prefs/eco).
func (a *App) ClearStats() error {
	st := a.ensureStats()
	st.mu.Lock()
	ex := append([]string(nil), st.store.GameExclusions...)
	if len(ex) == 0 {
		ex = defaultGameExclusions()
	}
	st.store = StatsStore{Days: []DayStats{}, GameExclusions: ex}
	st.dirty = true
	st.mu.Unlock()
	a.flushStats(true)
	return nil
}

// GetGameExclusions returns the editable non-game process list.
func (a *App) GetGameExclusions() []string {
	st := a.ensureStats()
	st.mu.Lock()
	defer st.mu.Unlock()
	return append([]string(nil), st.store.GameExclusions...)
}

// SetGameExclusions replaces the exclusion list used by game-minute accounting.
func (a *App) SetGameExclusions(list []string) {
	st := a.ensureStats()
	st.mu.Lock()
	clean := make([]string, 0, len(list))
	seen := map[string]struct{}{}
	for _, s := range list {
		b := strings.ToLower(filepath.Base(strings.TrimSpace(s)))
		if b == "" {
			continue
		}
		if !strings.HasSuffix(b, ".exe") {
			b += ".exe"
		}
		if _, ok := seen[b]; ok {
			continue
		}
		seen[b] = struct{}{}
		clean = append(clean, b)
	}
	if len(clean) == 0 {
		clean = defaultGameExclusions()
	}
	st.store.GameExclusions = clean
	st.dirty = true
	st.mu.Unlock()
	a.flushStats(true)
}
