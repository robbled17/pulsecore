//go:build windows

package main

import (
	"golang.org/x/sys/windows"
)

var (
	modGdi32            = windows.NewLazySystemDLL("gdi32.dll")
	procCreateRoundRect = modGdi32.NewProc("CreateRoundRectRgn")
	procSetWindowRgn    = windows.NewLazySystemDLL("user32.dll").NewProc("SetWindowRgn")
)

// ApplyOverlayWindowRegion clips the frameless PulseCore window to a rounded
// rectangle so Mini-HUD / game overlay do not show black corners on Windows 10.
// HWND is resolved only for our PID — never the foreground foreign app.
func (a *App) ApplyOverlayWindowRegion(width, height, radius int) {
	_ = a
	hwnd := appMainHWND()
	if hwnd == 0 || !hwndBelongsToUs(hwnd) || width <= 0 || height <= 0 {
		return
	}
	if radius < 4 {
		radius = 16
	}
	rgn, _, _ := procCreateRoundRect.Call(0, 0, uintptr(width+1), uintptr(height+1), uintptr(radius*2), uintptr(radius*2))
	if rgn == 0 {
		return
	}
	_, _, _ = procSetWindowRgn.Call(uintptr(hwnd), rgn, 1)
}

// ClearOverlayWindowRegion restores a rectangular window region.
func (a *App) ClearOverlayWindowRegion() {
	_ = a
	hwnd := appMainHWND()
	if hwnd == 0 || !hwndBelongsToUs(hwnd) {
		return
	}
	_, _, _ = procSetWindowRgn.Call(uintptr(hwnd), 0, 1)
}
