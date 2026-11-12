package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	intelMinSamples     = 30 // documented minimum before insights
	intelEMA            = 0.08
	intelOutlierMult    = 2.8 // ignore extreme spikes when updating baseline
	intelFlushEvery     = 2 * time.Minute
	intelMaxInsights    = 12
	intelTickMin        = 20 * time.Second
)

// IntelScenario is a comparable machine context.
type IntelScenario string

const (
	IntelIdle  IntelScenario = "idle"
	IntelWork  IntelScenario = "work"
	IntelGame  IntelScenario = "game"
	IntelHigh  IntelScenario = "high"
)

// IntelBaseline is the learned typical level for one scenario.
type IntelBaseline struct {
	Scenario   string  `json:"scenario"`
	CPU        float64 `json:"cpu"`
	RAM        float64 `json:"ram"`
	GPU        float64 `json:"gpu"`
	TempCPU    float64 `json:"tempCPU"`
	Samples    int     `json:"samples"`
	Ready      bool    `json:"ready"`
	UpdatedAt  string  `json:"updatedAt"`
}

// IntelInsight is a non-causal observation vs the local baseline.
type IntelInsight struct {
	ID         string  `json:"id"`
	Scenario   string  `json:"scenario"`
	Metric     string  `json:"metric"`
	Current    float64 `json:"current"`
	Typical    float64 `json:"typical"`
	Delta      float64 `json:"delta"`
	Confidence float64 `json:"confidence"`
	TextRu     string  `json:"textRu"`
	TextEn     string  `json:"textEn"`
	DetectedAt string  `json:"detectedAt"`
}

// IntelligenceStatus is the UI payload.
type IntelligenceStatus struct {
	OK           bool            `json:"ok"`
	MinSamples   int             `json:"minSamples"`
	ObservingDays float64        `json:"observingDays"`
	Baselines    []IntelBaseline `json:"baselines"`
	Insights     []IntelInsight  `json:"insights"`
	NoteRu       string          `json:"noteRu"`
	NoteEn       string          `json:"noteEn"`
}

type intelStoreFile struct {
	StartedAt string                    `json:"startedAt"`
	Baselines map[string]IntelBaseline  `json:"baselines"`
	Exclusions []string                 `json:"exclusions"`
}

type intelligenceEngine struct {
	mu         sync.Mutex
	startedAt  time.Time
	baselines  map[string]*IntelBaseline
	exclusions map[string]bool
	insights   []IntelInsight
	lastTick   time.Time
	lastFlush  time.Time
	dirty      bool
	loaded     bool
}

func (a *App) ensureIntel() *intelligenceEngine {
	if a.intel == nil {
		a.intel = &intelligenceEngine{
			baselines:  map[string]*IntelBaseline{},
			exclusions: map[string]bool{},
			insights:   make([]IntelInsight, 0, intelMaxInsights),
			startedAt:  time.Now(),
		}
	}
	return a.intel
}

func intelPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "PulseCore", "intelligence.json"), nil
}

func (a *App) loadIntelLocked(e *intelligenceEngine) {
	if e.loaded {
		return
	}
	e.loaded = true
	path, err := intelPath()
	if err != nil {
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var f intelStoreFile
	if json.Unmarshal(b, &f) != nil {
		return
	}
	if f.StartedAt != "" {
		if t, err := time.Parse(time.RFC3339, f.StartedAt); err == nil {
			e.startedAt = t
		}
	}
	for k, v := range f.Baselines {
		cp := v
		e.baselines[k] = &cp
	}
	for _, x := range f.Exclusions {
		e.exclusions[strings.ToLower(x)] = true
	}
}

func (a *App) flushIntelLocked(e *intelligenceEngine) {
	if !e.dirty {
		return
	}
	path, err := intelPath()
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f := intelStoreFile{
		StartedAt: e.startedAt.Format(time.RFC3339),
		Baselines: map[string]IntelBaseline{},
	}
	for k, v := range e.baselines {
		f.Baselines[k] = *v
	}
	for k := range e.exclusions {
		f.Exclusions = append(f.Exclusions, k)
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return
	}
	if os.WriteFile(path, b, 0o644) == nil {
		e.dirty = false
		e.lastFlush = time.Now()
	}
}

func classifyIntelScenario(game bool, cpuPct float64) IntelScenario {
	if game {
		return IntelGame
	}
	if cpuPct >= 70 {
		return IntelHigh
	}
	if cpuPct >= 25 {
		return IntelWork
	}
	return IntelIdle
}

// recordIntelligenceTick updates local PC baselines (no cloud).
func (a *App) recordIntelligenceTick(now time.Time, cpu, ram, gpu, temp float64, tempOK, gpuOK, game bool) {
	e := a.ensureIntel()
	e.mu.Lock()
	defer e.mu.Unlock()
	a.loadIntelLocked(e)
	if !e.lastTick.IsZero() && now.Sub(e.lastTick) < intelTickMin {
		return
	}
	e.lastTick = now
	sc := classifyIntelScenario(game, cpu)
	key := string(sc)
	bl := e.baselines[key]
	if bl == nil {
		bl = &IntelBaseline{Scenario: key}
		e.baselines[key] = bl
	}
	readyBefore := bl.Samples >= intelMinSamples
	// Snapshot typical levels before EMA update so insights compare against prior baseline.
	typCPU, typRAM, typGPU, typTemp := bl.CPU, bl.RAM, bl.GPU, bl.TempCPU

	update := func(cur *float64, sample float64, ok bool) {
		if !ok {
			return
		}
		if bl.Samples == 0 || *cur == 0 {
			*cur = sample
			return
		}
		// Outlier protection: do not let a single spike rewrite the baseline.
		if sample > *cur*intelOutlierMult && *cur > 1 {
			return
		}
		*cur = intelEMA*sample + (1-intelEMA)**cur
	}
	update(&bl.CPU, cpu, true)
	update(&bl.RAM, ram, true)
	update(&bl.GPU, gpu, gpuOK)
	update(&bl.TempCPU, temp, tempOK && temp > 0)
	bl.Samples++
	bl.Ready = bl.Samples >= intelMinSamples
	bl.UpdatedAt = now.Format(time.RFC3339)
	e.dirty = true

	if readyBefore {
		a.emitIntelInsightsLocked(e, sc, cpu, ram, gpu, temp, tempOK, gpuOK, typCPU, typRAM, typGPU, typTemp, now)
	}
	if e.lastFlush.IsZero() || now.Sub(e.lastFlush) >= intelFlushEvery {
		a.flushIntelLocked(e)
	}
}

func (a *App) emitIntelInsightsLocked(e *intelligenceEngine, sc IntelScenario, cpu, ram, gpu, temp float64, tempOK, gpuOK bool, typCPU, typRAM, typGPU, typTemp float64, now time.Time) {
	type cand struct {
		metric string
		cur, typ float64
		unit string
	}
	cands := []cand{{"ram", ram, typRAM, "%"}, {"cpu", cpu, typCPU, "%"}}
	if tempOK && typTemp > 0 {
		cands = append(cands, cand{"temp_cpu", temp, typTemp, "°C"})
	}
	if gpuOK && typGPU > 0 {
		cands = append(cands, cand{"gpu", gpu, typGPU, "%"})
	}
	for _, c := range cands {
		if c.typ <= 0 {
			continue
		}
		delta := c.cur - c.typ
		// Require meaningful deviation
		rel := delta / c.typ
		absOK := false
		switch c.metric {
		case "ram":
			absOK = delta >= 18 && rel >= 0.25
		case "cpu":
			absOK = delta >= 20 && rel >= 0.45
		case "temp_cpu":
			absOK = delta >= 10
		case "gpu":
			absOK = delta >= 25 && rel >= 0.4
		}
		if !absOK {
			continue
		}
		conf := clamp01to100(55 + math.Min(35, math.Abs(rel)*40)) / 100
		textRu, textEn := intelTexts(sc, c.metric, c.cur, c.typ, delta)
		ins := IntelInsight{
			ID: fmt.Sprintf("%s-%s", sc, c.metric),
			Scenario: string(sc), Metric: c.metric,
			Current: c.cur, Typical: c.typ, Delta: delta,
			Confidence: conf, TextRu: textRu, TextEn: textEn,
			DetectedAt: now.Format("15:04:05"),
		}
		filtered := make([]IntelInsight, 0, len(e.insights)+1)
		for _, old := range e.insights {
			if old.ID == ins.ID {
				continue
			}
			filtered = append(filtered, old)
		}
		e.insights = append([]IntelInsight{ins}, filtered...)
		if len(e.insights) > intelMaxInsights {
			e.insights = e.insights[:intelMaxInsights]
		}
	}
}

func intelTexts(sc IntelScenario, metric string, cur, typ, delta float64) (string, string) {
	scRu, scEn := "сейчас", "now"
	switch sc {
	case IntelIdle:
		scRu, scEn = "в простое", "while idle"
	case IntelWork:
		scRu, scEn = "в рабочей нагрузке", "under work load"
	case IntelGame:
		scRu, scEn = "в игре", "in a game"
	case IntelHigh:
		scRu, scEn = "при высокой нагрузке", "under high load"
	}
	switch metric {
	case "ram":
		return fmt.Sprintf("Обычно компьютер использует ≈%.0f%% RAM %s. Сейчас ≈%.0f%% (Δ %+.0f).", typ, scRu, cur, delta),
			fmt.Sprintf("This PC typically uses ≈%.0f%% RAM %s. Now ≈%.0f%% (Δ %+.0f).", typ, scEn, cur, delta)
	case "cpu":
		return fmt.Sprintf("Типичная загрузка CPU %s ≈%.0f%%. Сейчас ≈%.0f%%.", scRu, typ, cur),
			fmt.Sprintf("Typical CPU %s is ≈%.0f%%. Now ≈%.0f%%.", scEn, typ, cur)
	case "temp_cpu":
		return fmt.Sprintf("Температура CPU %s обычно ≈%.0f°C. Сейчас на %.0f°C выше (≈%.0f°C).", scRu, typ, delta, cur),
			fmt.Sprintf("CPU temperature %s is usually ≈%.0f°C. Now %.0f°C higher (≈%.0f°C).", scEn, typ, delta, cur)
	case "gpu":
		return fmt.Sprintf("Типичная загрузка GPU %s ≈%.0f%%. Сейчас ≈%.0f%%.", scRu, typ, cur),
			fmt.Sprintf("Typical GPU %s is ≈%.0f%%. Now ≈%.0f%%.", scEn, typ, cur)
	default:
		return "Замечено отклонение от локального профиля.", "Deviation from the local profile observed."
	}
}

// GetIntelligenceStatus returns baselines and insights (local only).
func (a *App) GetIntelligenceStatus() IntelligenceStatus {
	e := a.ensureIntel()
	e.mu.Lock()
	defer e.mu.Unlock()
	a.loadIntelLocked(e)
	bls := make([]IntelBaseline, 0, len(e.baselines))
	for _, sc := range []string{string(IntelIdle), string(IntelWork), string(IntelGame), string(IntelHigh)} {
		if b := e.baselines[sc]; b != nil {
			bls = append(bls, *b)
		} else {
			bls = append(bls, IntelBaseline{Scenario: sc, Samples: 0, Ready: false})
		}
	}
	ins := append([]IntelInsight(nil), e.insights...)
	days := time.Since(e.startedAt).Hours() / 24
	return IntelligenceStatus{
		OK: true, MinSamples: intelMinSamples, ObservingDays: days,
		Baselines: bls, Insights: ins,
		NoteRu: fmt.Sprintf("Локальный профиль. Минимум %d замеров на сценарий до выводов. Не причинно-следственные связи.", intelMinSamples),
		NoteEn: fmt.Sprintf("Local profile only. Need ≥%d samples per scenario before insights. Not causal claims.", intelMinSamples),
	}
}

// ResetIntelligenceProfile clears learned baselines and insights.
func (a *App) ResetIntelligenceProfile() {
	e := a.ensureIntel()
	e.mu.Lock()
	defer e.mu.Unlock()
	e.baselines = map[string]*IntelBaseline{}
	e.insights = e.insights[:0]
	e.startedAt = time.Now()
	e.dirty = true
	a.flushIntelLocked(e)
}

// AddIntelligenceExclusion excludes an exe name from future app-level learning hooks.
func (a *App) AddIntelligenceExclusion(exe string) {
	exe = strings.ToLower(filepath.Base(strings.TrimSpace(exe)))
	if exe == "" {
		return
	}
	e := a.ensureIntel()
	e.mu.Lock()
	defer e.mu.Unlock()
	a.loadIntelLocked(e)
	e.exclusions[exe] = true
	e.dirty = true
	a.flushIntelLocked(e)
}
