//go:build !windows

package main

type trayManager struct{}

func (a *App) startTray() {}

func (a *App) stopTray() {}

func (a *App) UpdateTrayIconNow() {}
