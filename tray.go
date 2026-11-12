package main

import (
	_ "embed"
	"fmt"
	"sync"

	"github.com/energye/systray"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

var (
	trayOnce   sync.Once
	trayQuitMu sync.Mutex
	trayGone   bool
	trayTipMu  sync.Mutex
)

func startSystray(app *App) {
	trayOnce.Do(func() {
		go systray.Run(func() {
			systray.SetIcon(trayIcon)
			systray.SetTitle("PulseCore")
			systray.SetTooltip("PulseCore — close keeps running in tray")

			systray.SetOnClick(func(menu systray.IMenu) {
				app.toggleWindowVisibility()
			})
			systray.SetOnRClick(func(menu systray.IMenu) {
				if menu != nil {
					_ = menu.ShowMenu()
				}
			})

			systray.CreateMenu()

			mShow := systray.AddMenuItem("Show PulseCore", "Show main window")
			mHUD := systray.AddMenuItem("Mini-HUD", "Switch to floating Mini-HUD")
			mGame := systray.AddMenuItem("Toggle Game Mode", "Enable/disable Game Mode detector")
			systray.AddSeparator()
			mQuit := systray.AddMenuItem("Quit", "Quit PulseCore")

			mShow.Click(func() {
				app.ShowWindow()
			})
			mHUD.Click(func() {
				app.ShowWindow()
				app.ToggleMiniHUD(true)
			})
			mGame.Click(func() {
				st := app.GetGameModeStatus()
				app.SetGameModeEnabled(!st.Enabled)
			})
			mQuit.Click(func() {
				app.QuitApp()
			})
		}, func() {})
	})
}

func quitSystray() {
	trayQuitMu.Lock()
	defer trayQuitMu.Unlock()
	if trayGone {
		return
	}
	trayGone = true
	systray.Quit()
}

func (a *App) toggleWindowVisibility() {
	if a.ctx == nil {
		return
	}
	a.mu.RLock()
	hidden := a.hidden
	a.mu.RUnlock()
	if hidden {
		a.ShowWindow()
		return
	}
	a.HideToTray()
}

// updateTrayLiveStats refreshes tray tooltip with live CPU/RAM (throttled).
func (a *App) updateTrayLiveStats(cpu, ram float64) {
	cpuI := int(cpu + 0.5)
	ramI := int(ram + 0.5)
	a.mu.Lock()
	if cpuI == a.trayCPU && ramI == a.trayRAM {
		a.mu.Unlock()
		return
	}
	a.trayCPU = cpuI
	a.trayRAM = ramI
	gm := a.gameModeActive
	a.mu.Unlock()

	tip := fmt.Sprintf("PulseCore  CPU %d%%  RAM %d%%", cpuI, ramI)
	if gm {
		tip += "  [Game Mode]"
	}
	trayTipMu.Lock()
	systray.SetTooltip(tip)
	trayTipMu.Unlock()
}
