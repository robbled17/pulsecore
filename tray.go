package main

import (
	_ "embed"
	"sync"

	"github.com/energye/systray"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

var (
	trayOnce   sync.Once
	trayQuitMu sync.Mutex
	trayGone   bool
)

func startSystray(app *App) {
	trayOnce.Do(func() {
		go systray.Run(func() {
			systray.SetIcon(trayIcon)
			systray.SetTitle("PulseCore")
			systray.SetTooltip("PulseCore v1.6.1")

			systray.SetOnClick(func(menu systray.IMenu) {
				app.toggleWindowVisibility()
			})
			systray.SetOnRClick(func(menu systray.IMenu) {
				if menu != nil {
					_ = menu.ShowMenu()
				}
			})

			systray.CreateMenu()

			mShow := systray.AddMenuItem("Показать PulseCore", "Show main window")
			mHUD := systray.AddMenuItem("Включить Mini-HUD", "Switch to floating Mini-HUD")
			systray.AddSeparator()
			mQuit := systray.AddMenuItem("Выход", "Quit PulseCore")

			mShow.Click(func() {
				app.ShowWindow()
			})
			mHUD.Click(func() {
				app.ShowWindow()
				app.ToggleMiniHUD(true)
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
