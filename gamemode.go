package main

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// GameModeStatus describes Game Mode session state (separate from GamePulse FPS).
type GameModeStatus struct {
	Enabled       bool   `json:"enabled"`
	Active        bool   `json:"active"`
	GameName      string `json:"gameName"`
	GamePID       int32  `json:"gamePID"`
	StartedAt     string `json:"startedAt"`
	SessionSec    int64  `json:"sessionSec"`
	Optimizations string `json:"optimizations"`
	NoteKey       string `json:"noteKey"`
}

var gameExeHints = []string{
	"game", "steam", "epic", "origin", "battle.net", "riot", "uplay",
	"valorant", "cs2", "csgo", "dota", "fortnite", "minecraft",
	"wow", "overwatch", "league", "genshin", "elden",
}

func looksLikeGame(name string) bool {
	n := strings.ToLower(filepath.Base(name))
	if n == "" || isEcoProtected(n) {
		return false
	}
	for _, h := range gameExeHints {
		if strings.Contains(n, h) {
			return true
		}
	}
	// Common game suffix patterns.
	if strings.HasSuffix(n, "-win64-shipping.exe") || strings.HasSuffix(n, "client.exe") {
		return true
	}
	return false
}

func (a *App) detectForegroundGame() (string, int32) {
	pid := foregroundPID()
	if pid <= 0 {
		return "", 0
	}
	p, err := process.NewProcess(pid)
	if err != nil {
		return "", 0
	}
	name, err := p.Name()
	if err != nil || !looksLikeGame(name) {
		return "", 0
	}
	return name, pid
}

func (a *App) gameModeWorker() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.stopWorker:
			return
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.mu.RLock()
			enabled := a.gameModeEnabled
			a.mu.RUnlock()
			if !enabled {
				continue
			}
			name, pid := a.detectForegroundGame()
			a.mu.Lock()
			was := a.gameModeActive
			if name != "" {
				if !a.gameModeActive {
					a.gameModeActive = true
					a.gameModeName = name
					a.gameModePID = pid
					a.gameModeSince = time.Now()
				} else {
					a.gameModeName = name
					a.gameModePID = pid
				}
			} else if a.gameModeActive {
				a.gameModeActive = false
				a.gameModeName = ""
				a.gameModePID = 0
			}
			nowActive := a.gameModeActive
			st := a.gameModeStatusLocked()
			a.mu.Unlock()
			if was != nowActive && a.ctx != nil {
				runtime.EventsEmit(a.ctx, "gamemode:update", st)
			}
		}
	}
}

func (a *App) gameModeStatusLocked() GameModeStatus {
	st := GameModeStatus{
		Enabled:       a.gameModeEnabled,
		Active:        a.gameModeActive,
		GameName:      a.gameModeName,
		GamePID:       a.gameModePID,
		Optimizations: "notify",
		NoteKey:       "gameModeIdle",
	}
	if a.gameModeActive {
		st.StartedAt = a.gameModeSince.Format(time.RFC3339)
		st.SessionSec = int64(time.Since(a.gameModeSince).Seconds())
		st.NoteKey = "gameModeActive"
		st.Optimizations = "eco-skip-game;overlay-ready"
	}
	return st
}

// GetGameModeStatus returns current Game Mode session info.
func (a *App) GetGameModeStatus() GameModeStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.gameModeStatusLocked()
}

// SetGameModeEnabled toggles Game Mode detector (does not replace GamePulse).
func (a *App) SetGameModeEnabled(enable bool) GameModeStatus {
	a.mu.Lock()
	a.gameModeEnabled = enable
	if !enable {
		a.gameModeActive = false
		a.gameModeName = ""
		a.gameModePID = 0
	}
	st := a.gameModeStatusLocked()
	a.mu.Unlock()
	a.persistConfig()
	return st
}
