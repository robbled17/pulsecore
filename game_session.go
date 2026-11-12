package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// appDevMode enables simulated game sessions (CLI --dev). Never writes fixtures into production paths by default.
var appDevMode bool

// gameBookPathOverride redirects persistence (unit tests). Empty = real user config.
var gameBookPathOverride string

const (
	maxGameSessions  = 40
	gameConfirmAfter = 2 * time.Minute
	gameEndGrace     = 45 * time.Second
)

// GameSession is one completed or active play session.
type GameSession struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`          // display name
	Exe           string  `json:"exe"`
	Start         string  `json:"start"`         // RFC3339
	End           string  `json:"end,omitempty"`
	DurationMin   float64 `json:"durationMin"`
	CPUMax        float64 `json:"cpuMax"`
	CPUAvg        float64 `json:"cpuAvg"`
	GPUMax        float64 `json:"gpuMax"`
	GPUAvg        float64 `json:"gpuAvg"`
	RAMMax        float64 `json:"ramMax"`
	TempCPUMax    float64 `json:"tempCPUMax"`
	TempCPUAvg    float64 `json:"tempCPUAvg"`
	TempGPUMax    float64 `json:"tempGPUMax"`
	TempGPUAvg    float64 `json:"tempGPUAvg"`
	Freezes       int     `json:"freezes"`
	Verdict       string  `json:"verdict"`
	Active        bool    `json:"active"`
	Synthetic     bool    `json:"synthetic,omitempty"`
	samples       int     `json:"-"`
	cpuSum        float64 `json:"-"`
	gpuSum        float64 `json:"-"`
	tempCPUSum    float64 `json:"-"`
	tempGPUSum    float64 `json:"-"`
	startTime     time.Time `json:"-"`
	lastSeen      time.Time `json:"-"`
}

// GameSessionCompare is a side-by-side of two sessions.
type GameSessionCompare struct {
	OK    bool        `json:"ok"`
	Left  GameSession `json:"left"`
	Right GameSession `json:"right"`
	Notes []string    `json:"notes"`
}

// ActiveGameStatus is the live Game Mode HUD payload.
type ActiveGameStatus struct {
	Active      bool    `json:"active"`
	Name        string  `json:"name"`
	Exe         string  `json:"exe"`
	DurationMin float64 `json:"durationMin"`
	CPUPercent  float64 `json:"cpuPercent"`
	GPUPercent  float64 `json:"gpuPercent"`
	RAMPercent  float64 `json:"ramPercent"`
	TempCPU     float64 `json:"tempCPU"`
	TempGPU     float64 `json:"tempGPU"`
	Freezes     int     `json:"freezes"`
	FPSNote     string  `json:"fpsNote"` // honest: FPS not available without injection
}

type gameBook struct {
	mu        sync.Mutex
	active    *GameSession
	pending   *gameDetectState
	history   []GameSession
	aliases   map[string]string // exe -> display name
	manual    map[string]bool   // exe forced as game
	loaded    bool
}

type gameDetectState struct {
	exe        string
	since      time.Time
	confirmed  bool
	lastAccrue time.Time
	coldSince  time.Time
}

func (a *App) ensureGameBook() *gameBook {
	if a.gameBook == nil {
		a.gameBook = &gameBook{
			aliases: map[string]string{},
			manual:  map[string]bool{},
			history: []GameSession{},
		}
	}
	return a.gameBook
}

func gameSessionsPath() (string, error) {
	if gameBookPathOverride != "" {
		return gameBookPathOverride, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "PulseCore", "game_sessions.json"), nil
}

// isSyntheticGameSession detects test/demo fixtures that must not appear in the UI.
func isSyntheticGameSession(s GameSession) bool {
	if s.Synthetic {
		return true
	}
	exe := strings.ToLower(filepath.Base(s.Exe))
	name := strings.ToLower(strings.TrimSpace(s.Name))
	if exe == "game.exe" || name == "game" || name == "game.exe" {
		return true
	}
	return false
}

func scrubGameHistory(in []GameSession) ([]GameSession, bool) {
	out := make([]GameSession, 0, len(in))
	changed := false
	for _, s := range in {
		if isSyntheticGameSession(s) {
			changed = true
			continue
		}
		out = append(out, s)
	}
	return out, changed
}

type gamePersistFile struct {
	History []GameSession     `json:"history"`
	Aliases map[string]string `json:"aliases"`
	Manual  []string          `json:"manual"`
}

func (a *App) loadGameBook() {
	gb := a.ensureGameBook()
	gb.mu.Lock()
	defer gb.mu.Unlock()
	path, err := gameSessionsPath()
	if err != nil {
		gb.loaded = true
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		gb.loaded = true
		return
	}
	var f gamePersistFile
	if json.Unmarshal(raw, &f) != nil {
		gb.loaded = true
		return
	}
	cleaned, changed := scrubGameHistory(f.History)
	gb.history = cleaned
	if f.Aliases != nil {
		gb.aliases = f.Aliases
	}
	gb.manual = map[string]bool{}
	for _, m := range f.Manual {
		gb.manual[strings.ToLower(filepath.Base(m))] = true
	}
	gb.loaded = true
	if changed {
		// Rewrite cleaned history while still holding the lock (avoid nested Lock).
		manual := make([]string, 0, len(gb.manual))
		for k, on := range gb.manual {
			if on {
				manual = append(manual, k)
			}
		}
		sort.Strings(manual)
		payload := gamePersistFile{History: cleaned, Aliases: gb.aliases, Manual: manual}
		if raw, err := json.MarshalIndent(payload, "", "  "); err == nil {
			_ = os.MkdirAll(filepath.Dir(path), 0o755)
			tmp := path + ".tmp"
			if os.WriteFile(tmp, raw, 0o644) == nil {
				_ = os.Rename(tmp, path)
			}
		}
	}
}

func (a *App) persistGameBook() {
	gb := a.ensureGameBook()
	gb.mu.Lock()
	defer gb.mu.Unlock()
	path, err := gameSessionsPath()
	if err != nil {
		return
	}
	history, _ := scrubGameHistory(gb.history)
	gb.history = history
	manual := make([]string, 0, len(gb.manual))
	for k, on := range gb.manual {
		if on {
			manual = append(manual, k)
		}
	}
	sort.Strings(manual)
	f := gamePersistFile{History: history, Aliases: gb.aliases, Manual: manual}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func (a *App) displayGameName(exe string) string {
	base := strings.ToLower(filepath.Base(exe))
	gb := a.ensureGameBook()
	gb.mu.Lock()
	defer gb.mu.Unlock()
	if n, ok := gb.aliases[base]; ok && n != "" {
		return n
	}
	return friendlyProcessName(exe)
}

// tickGameDetect confirms game load, tracks an active session, and accrues weekly minutes.
func (a *App) tickGameDetect(now time.Time, gpuPct, cpuPct float64) {
	fg := foregroundPID()
	name := ""
	if fg > 0 {
		name = processNameByPID(int(fg))
	}
	base := strings.ToLower(filepath.Base(name))
	// Never treat the unit-test fixture exe as a real game outside --dev.
	if base == "game.exe" && !appDevMode {
		base = ""
		name = ""
	}
	excluded := a.isGameExcluded(base)
	gb := a.ensureGameBook()
	gb.mu.Lock()
	manual := gb.manual[base]
	gb.mu.Unlock()
	hot := gpuPct >= highLoadGPUPct || (gpuPct >= 25 && cpuPct >= 35) || manual

	a.mu.RLock()
	ram := a.metrics.RAMPercent
	temp := a.metrics.TemperatureC
	a.mu.RUnlock()

	if a.gameDet == nil {
		a.gameDet = &gameDetectState{}
	}
	gd := a.gameDet

	if base == "" || (excluded && !manual) || !hot {
		if gd.confirmed {
			if gd.coldSince.IsZero() {
				gd.coldSince = now
			}
			if now.Sub(gd.coldSince) < gameEndGrace {
				a.sampleActiveGame(now, cpuPct, gpuPct, ram, temp)
				return
			}
			a.endActiveGame(now)
		}
		gd.exe = ""
		gd.since = time.Time{}
		gd.confirmed = false
		gd.lastAccrue = time.Time{}
		gd.coldSince = time.Time{}
		return
	}
	gd.coldSince = time.Time{}

	if gd.exe != base {
		if gd.confirmed {
			a.endActiveGame(now)
		}
		gd.exe = base
		gd.since = now
		gd.confirmed = false
		gd.lastAccrue = time.Time{}
		return
	}
	if !gd.confirmed {
		need := gameConfirmAfter
		if manual {
			need = 15 * time.Second
		}
		if now.Sub(gd.since) >= need {
			gd.confirmed = true
			gd.lastAccrue = now
			a.beginActiveGame(base, name, now)
		}
		return
	}

	a.sampleActiveGame(now, cpuPct, gpuPct, ram, temp)
	if gd.lastAccrue.IsZero() {
		gd.lastAccrue = now
		return
	}
	mins := now.Sub(gd.lastAccrue).Minutes()
	if mins >= 0.5 {
		a.recordStatsGameMinutes(gd.exe, mins)
		gd.lastAccrue = now
	}
}

func (a *App) beginActiveGame(exe, rawName string, now time.Time) {
	gb := a.ensureGameBook()
	display := a.displayGameName(rawName)
	if display == "" || display == "unknown" {
		display = rawName
	}
	gb.mu.Lock()
	gb.active = &GameSession{
		ID:        fmt.Sprintf("game-%d", now.UnixNano()),
		Name:      display,
		Exe:       exe,
		Start:     now.Format(time.RFC3339),
		Active:    true,
		startTime: now,
		lastSeen:  now,
	}
	gb.mu.Unlock()
}

func (a *App) sampleActiveGame(now time.Time, cpu, gpu, ram, tempCPU float64) {
	gb := a.ensureGameBook()
	gb.mu.Lock()
	defer gb.mu.Unlock()
	s := gb.active
	if s == nil || !s.Active {
		return
	}
	s.lastSeen = now
	s.samples++
	s.cpuSum += cpu
	s.gpuSum += gpu
	if tempCPU > 0 {
		s.tempCPUSum += tempCPU
		if tempCPU > s.TempCPUMax {
			s.TempCPUMax = tempCPU
		}
	}
	if cpu > s.CPUMax {
		s.CPUMax = cpu
	}
	if gpu > s.GPUMax {
		s.GPUMax = gpu
	}
	if ram > s.RAMMax {
		s.RAMMax = ram
	}
	s.DurationMin = now.Sub(s.startTime).Minutes()
	if s.samples > 0 {
		s.CPUAvg = s.cpuSum / float64(s.samples)
		s.GPUAvg = s.gpuSum / float64(s.samples)
		if s.tempCPUSum > 0 {
			n := float64(s.samples)
			if s.TempCPUMax > 0 {
				s.TempCPUAvg = s.tempCPUSum / n
			}
		}
	}
}

func (a *App) endActiveGame(now time.Time) {
	gb := a.ensureGameBook()
	gb.mu.Lock()
	s := gb.active
	if s == nil {
		gb.mu.Unlock()
		return
	}
	s.Active = false
	s.End = now.Format(time.RFC3339)
	s.DurationMin = now.Sub(s.startTime).Minutes()
	if s.samples > 0 {
		s.CPUAvg = s.cpuSum / float64(s.samples)
		s.GPUAvg = s.gpuSum / float64(s.samples)
		if s.tempCPUSum > 0 {
			s.TempCPUAvg = s.tempCPUSum / float64(s.samples)
		}
	}
	if s.Verdict == "" {
		s.Verdict = "UNKNOWN"
	}
	cp := *s
	gb.history = append([]GameSession{cp}, gb.history...)
	if len(gb.history) > maxGameSessions {
		gb.history = gb.history[:maxGameSessions]
	}
	gb.active = nil
	gb.mu.Unlock()
	a.persistGameBook()
}

func (a *App) noteGameFreeze(verdict string) {
	gb := a.ensureGameBook()
	gb.mu.Lock()
	defer gb.mu.Unlock()
	if gb.active == nil || !gb.active.Active {
		return
	}
	gb.active.Freezes++
	if verdict != "" && (gb.active.Verdict == "" || gb.active.Verdict == "UNKNOWN") {
		gb.active.Verdict = verdict
	}
}

// GetActiveGameStatus returns live Game Mode HUD fields (no FPS promise).
func (a *App) GetActiveGameStatus() ActiveGameStatus {
	a.mu.RLock()
	cpu := a.metrics.CPUPercent
	gpu := a.metrics.GPUPercent
	ram := a.metrics.RAMPercent
	temp := a.metrics.TemperatureC
	lang := a.language
	a.mu.RUnlock()
	gb := a.ensureGameBook()
	gb.mu.Lock()
	defer gb.mu.Unlock()
	fpsNote := "FPS недоступен без интеграции с игрой"
	if lang == "en" {
		fpsNote = "FPS is unavailable without a game integration"
	}
	if gb.active == nil || !gb.active.Active {
		return ActiveGameStatus{Active: false, FPSNote: fpsNote}
	}
	s := gb.active
	return ActiveGameStatus{
		Active:      true,
		Name:        s.Name,
		Exe:         s.Exe,
		DurationMin: s.DurationMin,
		CPUPercent:  cpu,
		GPUPercent:  gpu,
		RAMPercent:  ram,
		TempCPU:     temp,
		Freezes:     s.Freezes,
		FPSNote:     fpsNote,
	}
}

// GetGameSessions returns recent completed sessions (active first if any).
func (a *App) GetGameSessions() []GameSession {
	gb := a.ensureGameBook()
	gb.mu.Lock()
	defer gb.mu.Unlock()
	out := make([]GameSession, 0, len(gb.history)+1)
	if gb.active != nil && gb.active.Active {
		cp := *gb.active
		out = append(out, cp)
	}
	out = append(out, gb.history...)
	return out
}

// CompareGameSessions compares two stored sessions by id.
func (a *App) CompareGameSessions(leftID, rightID string) GameSessionCompare {
	sessions := a.GetGameSessions()
	var left, right *GameSession
	for i := range sessions {
		if sessions[i].ID == leftID {
			left = &sessions[i]
		}
		if sessions[i].ID == rightID {
			right = &sessions[i]
		}
	}
	if left == nil || right == nil {
		return GameSessionCompare{OK: false, Notes: []string{"session_not_found"}}
	}
	notes := []string{}
	if right.TempCPUMax > left.TempCPUMax+3 {
		notes = append(notes, fmt.Sprintf("CPU temp +%.0f°C vs previous", right.TempCPUMax-left.TempCPUMax))
	}
	if right.Freezes > left.Freezes {
		notes = append(notes, fmt.Sprintf("More freezes: %d → %d", left.Freezes, right.Freezes))
	}
	if right.GPUAvg+5 < left.GPUAvg {
		notes = append(notes, "Lower average GPU load")
	}
	if len(notes) == 0 {
		notes = append(notes, "Sessions are similar")
	}
	return GameSessionCompare{OK: true, Left: *left, Right: *right, Notes: notes}
}

// RenameGame sets a display alias for an EXE.
func (a *App) RenameGame(exe, display string) {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(exe)))
	display = strings.TrimSpace(display)
	if base == "" || display == "" {
		return
	}
	gb := a.ensureGameBook()
	gb.mu.Lock()
	gb.aliases[base] = display
	if gb.active != nil && strings.EqualFold(gb.active.Exe, base) {
		gb.active.Name = display
	}
	for i := range gb.history {
		if strings.EqualFold(gb.history[i].Exe, base) {
			gb.history[i].Name = display
		}
	}
	gb.mu.Unlock()
	a.persistGameBook()
}

// MarkManualGame forces an EXE to be treated as a game (still needs load to start session).
func (a *App) MarkManualGame(exe string, enable bool) {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(exe)))
	if base == "" {
		return
	}
	if !strings.HasSuffix(base, ".exe") {
		base += ".exe"
	}
	gb := a.ensureGameBook()
	gb.mu.Lock()
	if enable {
		gb.manual[base] = true
	} else {
		delete(gb.manual, base)
	}
	gb.mu.Unlock()
	a.persistGameBook()
}

// AddGameExclusion appends an EXE to the non-game list.
func (a *App) AddGameExclusion(exe string) {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(exe)))
	if base == "" {
		return
	}
	list := a.GetGameExclusions()
	for _, e := range list {
		if e == base {
			return
		}
	}
	a.SetGameExclusions(append(list, base))
}
