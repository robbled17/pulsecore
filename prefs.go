package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// HiddenGamePref is a user-hidden GamePulse catalog entry.
type HiddenGamePref struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	PosterURL string `json:"posterUrl,omitempty"`
	Year      int    `json:"year,omitempty"`
}

// appPrefs is the persisted launch and monitoring behavior.
type appPrefs struct {
	AutoStart      bool   `json:"autoStart"`
	StartMinimized bool   `json:"startMinimized"`
	CloseToTray    bool   `json:"closeToTray"`
	TempAlerts     bool   `json:"tempAlerts"`
	ShowTempOnTray bool   `json:"showTempOnTray"`
	TrayTipShown   bool   `json:"trayTipShown"`
	// EcoExitPolicy: ask | clear | keep
	EcoExitPolicy string `json:"ecoExitPolicy"`
	// User-hidden GamePulse titles (not non-game engine rules).
	HiddenGames []HiddenGamePref `json:"hiddenGames,omitempty"`
	// MiniHUDPosX/Y are last overlay coordinates; -1 = unset.
	MiniHUDPosX int `json:"miniHudPosX"`
	MiniHUDPosY int `json:"miniHudPosY"`
	// MiniHUDOpacityPercent: 60 | 80 | 100
	MiniHUDOpacityPercent int  `json:"miniHudOpacityPercent"`
	MiniHUDAcrylic        bool `json:"miniHudAcrylic"`
	ConflictWatcherEnabled bool `json:"conflictWatcherEnabled"`
}

func defaultPrefs() appPrefs {
	return appPrefs{
		TempAlerts:             true,
		CloseToTray:            true,
		ShowTempOnTray:         true,
		ConflictWatcherEnabled: true,
		EcoExitPolicy:          "ask",
		MiniHUDPosX:            -1,
		MiniHUDPosY:            -1,
		MiniHUDOpacityPercent:  80,
		MiniHUDAcrylic:         false,
	}
}

func prefsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "PulseCore", "prefs.json"), nil
}

func (a *App) loadPrefs() {
	a.mu.Lock()
	a.prefs = defaultPrefs()
	a.mu.Unlock()

	path, err := prefsPath()
	if err != nil {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var p appPrefs
	if json.Unmarshal(raw, &p) != nil {
		return
	}
	if p.EcoExitPolicy == "" {
		p.EcoExitPolicy = "ask"
	}
	if p.MiniHUDOpacityPercent != 60 && p.MiniHUDOpacityPercent != 80 && p.MiniHUDOpacityPercent != 100 {
		p.MiniHUDOpacityPercent = 80
	}
	// If the file never had closeToTray / showTempOnTray keys (pre-tray builds),
	// apply Stage-4 defaults instead of Go zero values.
	var keys map[string]json.RawMessage
	if json.Unmarshal(raw, &keys) == nil {
		if _, ok := keys["closeToTray"]; !ok {
			p.CloseToTray = true
		}
		if _, ok := keys["showTempOnTray"]; !ok {
			p.ShowTempOnTray = true
		}
		if _, ok := keys["miniHudPosX"]; !ok {
			p.MiniHUDPosX = -1
		}
		if _, ok := keys["miniHudPosY"]; !ok {
			p.MiniHUDPosY = -1
		}
		if _, ok := keys["conflictWatcherEnabled"]; !ok {
			p.ConflictWatcherEnabled = true
		}
	}
	a.mu.Lock()
	a.prefs = p
	a.mu.Unlock()
}

// IsConflictWatcherEnabled reports whether the locked file interceptor is enabled.
func (a *App) IsConflictWatcherEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.prefs.ConflictWatcherEnabled
}

// SetConflictWatcherEnabled toggles the locked file interceptor.
func (a *App) SetConflictWatcherEnabled(enabled bool) error {
	a.mu.Lock()
	a.prefs.ConflictWatcherEnabled = enabled
	p := a.prefs
	a.mu.Unlock()
	return a.savePrefs(p)
}

func (a *App) savePrefs(p appPrefs) error {
	path, err := prefsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// GetBehaviorSettings returns launch and monitoring preferences.
func (a *App) GetBehaviorSettings() appPrefs {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.prefs
}

// SetBehaviorSettings stores the checkboxes and applies the Windows autostart entry.
func (a *App) SetBehaviorSettings(p appPrefs) error {
	if err := setWindowsAutoStart(p.AutoStart); err != nil {
		return err
	}
	a.mu.RLock()
	prev := a.prefs
	a.mu.RUnlock()
	// Preserve tip / HUD / hidden-game prefs — settings UI does not edit these.
	p.TrayTipShown = prev.TrayTipShown
	p.HiddenGames = prev.HiddenGames
	p.MiniHUDPosX = prev.MiniHUDPosX
	p.MiniHUDPosY = prev.MiniHUDPosY
	p.MiniHUDOpacityPercent = prev.MiniHUDOpacityPercent
	p.MiniHUDAcrylic = prev.MiniHUDAcrylic
	if err := a.savePrefs(p); err != nil {
		return err
	}
	a.mu.Lock()
	a.prefs = p
	a.mu.Unlock()
	return nil
}

func setWindowsAutoStart(enable bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	const name = "PulseCore"
	if !enable {
		err := key.DeleteValue(name)
		if err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return key.SetStringValue(name, `"`+exe+`"`)
}
