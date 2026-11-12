package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type trayManager struct {
	app *App

	mu          sync.Mutex
	started     bool
	lastValue   int
	lastR, lastG, lastB byte
	lastTip     string
	showTemp    bool
	stopUpdate  chan struct{}
	updateOnce  sync.Once
	closeOnce   sync.Once
}

func (a *App) startTray() {
	if a.tray == nil {
		a.tray = &trayManager{app: a, stopUpdate: make(chan struct{})}
	}
	go systray.Run(a.tray.onReady, a.tray.onExit)
}

func (t *trayManager) onReady() {
	t.mu.Lock()
	t.started = true
	t.mu.Unlock()

	systray.SetTitle("PulseCore")
	systray.SetTooltip("PulseCore")
	ico := trayIconICO(0, 56, 189, 248)
	systray.SetIcon(ico)

	lang := t.app.getLang()
	mOpen := systray.AddMenuItem(trayT(lang, "open"), "")
	mHUD := systray.AddMenuItem(trayT(lang, "hud"), "")
	systray.AddSeparator()
	mPower := systray.AddMenuItem(trayT(lang, "power"), "")
	mSilent := mPower.AddSubMenuItem(trayT(lang, "silent"), "")
	mBalanced := mPower.AddSubMenuItem(trayT(lang, "balanced"), "")
	mTurbo := mPower.AddSubMenuItem(trayT(lang, "turbo"), "")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem(trayT(lang, "quit"), "")

	systray.SetOnClick(func(menu systray.IMenu) {
		t.app.ShowWindow()
	})
	systray.SetOnRClick(func(menu systray.IMenu) {
		if menu != nil {
			menu.ShowMenu()
		}
	})
	systray.SetOnDClick(func(menu systray.IMenu) {
		t.app.ShowWindow()
	})

	mOpen.Click(func() { t.app.ShowWindow() })
	mHUD.Click(func() {
		t.app.mu.RLock()
		on := t.app.miniHUD
		t.app.mu.RUnlock()
		t.app.ToggleMiniHUD(!on)
	})
	mSilent.Click(func() { _ = t.app.SetPowerProfile("silent") })
	mBalanced.Click(func() { _ = t.app.SetPowerProfile("balanced") })
	mTurbo.Click(func() { _ = t.app.SetPowerProfile("turbo") })
	mQuit.Click(func() { t.app.QuitApp() })

	t.updateOnce.Do(func() {
		go t.updateLoop()
	})
	t.refreshNow()
}

func (t *trayManager) onExit() {
	t.closeStop()
}

func (t *trayManager) closeStop() {
	t.closeOnce.Do(func() {
		if t.stopUpdate != nil {
			close(t.stopUpdate)
		}
	})
}

func (t *trayManager) updateLoop() {
	for {
		wait := 3 * time.Second
		t.app.mu.RLock()
		hidden := t.app.hidden
		cpu := t.app.metrics.CPUPercent
		t.app.mu.RUnlock()
		if hidden {
			wait = 8 * time.Second
			if cpu < 25 {
				wait = 10 * time.Second
			}
		}
		select {
		case <-t.stopUpdate:
			return
		case <-time.After(wait):
			t.refreshNow()
		}
	}
}

func (t *trayManager) refreshNow() {
	t.app.mu.RLock()
	m := t.app.metrics
	prefs := t.app.prefs
	t.app.mu.RUnlock()

	showTemp := prefs.ShowTempOnTray
	temp := m.TemperatureC
	if !showTemp {
		temp = 0
	}
	value, r, g, b, isTemp := trayColorForTemp(temp, m.CPUPercent)

	tip := formatTrayTooltip(m, isTemp)
	t.mu.Lock()
	sameIcon := t.lastValue == value && t.lastR == r && t.lastG == g && t.lastB == b
	sameTip := t.lastTip == tip
	if !sameIcon {
		t.lastValue, t.lastR, t.lastG, t.lastB = value, r, g, b
	}
	if !sameTip {
		t.lastTip = tip
	}
	t.mu.Unlock()

	if !sameIcon {
		systray.SetIcon(trayIconICO(value, r, g, b))
	}
	if !sameTip {
		systray.SetTooltip(tip)
	}
}

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
	if a.tray != nil {
		a.tray.closeStop()
	}
	systray.Quit()
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
