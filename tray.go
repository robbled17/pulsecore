package main

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func formatTrayTooltip(m SystemMetrics, showTemp bool) string {
	parts := make([]string, 0, 3)
	if showTemp && m.TemperatureC > 0 {
		parts = append(parts, fmt.Sprintf("CPU %.0f°C · %.0f%%", m.TemperatureC, m.CPUPercent))
	} else {
		parts = append(parts, fmt.Sprintf("CPU %.0f%%", m.CPUPercent))
	}
	parts = append(parts, fmt.Sprintf("GPU %.0f%%", m.GPUPercent))
	parts = append(parts, fmt.Sprintf("RAM %.0f%%", m.RAMPercent))
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += " | " + parts[i]
	}
	return out
}

func trayT(lang, key string) string {
	ru := map[string]string{
		"open": "Открыть PulseCore", "hud": "Показать/скрыть мини-HUD",
		"power": "Режим питания", "silent": "Тихий", "balanced": "Сбалансированный",
		"turbo": "Турбо", "quit": "Выход",
	}
	en := map[string]string{
		"open": "Open PulseCore", "hud": "Show/hide Mini-HUD",
		"power": "Power mode", "silent": "Silent", "balanced": "Balanced",
		"turbo": "Turbo", "quit": "Exit",
	}
	if lang == "en" {
		if s, ok := en[key]; ok {
			return s
		}
	}
	if s, ok := ru[key]; ok {
		return s
	}
	return key
}

// HideToTray hides the main window and keeps monitoring alive.
func (a *App) HideToTray() {
	if a.ctx == nil {
		return
	}
	a.mu.Lock()
	a.hidden = true
	showTip := !a.prefs.TrayTipShown
	if showTip {
		p := a.prefs
		p.TrayTipShown = true
		a.prefs = p
		a.mu.Unlock()
		_ = a.savePrefs(p)
		runtime.EventsEmit(a.ctx, "ui:tray-tip", true)
	} else {
		a.mu.Unlock()
	}
	runtime.WindowHide(a.ctx)
}

// QuitApp performs a full shutdown: eco prompt path, stats flush, tray quit, Wails quit.
func (a *App) QuitApp() {
	a.mu.Lock()
	a.forceQuit = true
	a.mu.Unlock()
	if a.handleEcoOnClose() {
		// Waiting for ConfirmQuitAfterEco - keep forceQuit=true.
		return
	}
	a.finishQuit()
}

func (a *App) finishQuit() {
	a.flushStats(true)
	a.persistEcoState()
	a.stopTray()
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// beforeClose hides to tray by default; Exit from tray sets forceQuit.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	a.mu.RLock()
	force := a.forceQuit
	closeTray := a.prefs.CloseToTray
	a.mu.RUnlock()
	if force {
		return a.handleEcoOnClose()
	}
	if closeTray {
		a.HideToTray()
		return true
	}
	a.mu.Lock()
	a.forceQuit = true
	a.mu.Unlock()
	if a.handleEcoOnClose() {
		return true
	}
	return false
}
