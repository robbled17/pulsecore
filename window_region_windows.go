//go:build windows

package main

import (
	"math"

	"golang.org/x/sys/windows"
)

var (
	modGdi32                  = windows.NewLazySystemDLL("gdi32.dll")
	procCreateRoundRect       = modGdi32.NewProc("CreateRoundRectRgn")
	procSetWindowRgn          = windows.NewLazySystemDLL("user32.dll").NewProc("SetWindowRgn")
	procGetClassLongPtr       = windows.NewLazySystemDLL("user32.dll").NewProc("GetClassLongPtrW")
	procSetClassLongPtr       = windows.NewLazySystemDLL("user32.dll").NewProc("SetClassLongPtrW")
	modDwmapi                 = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWindowAttribute = modDwmapi.NewProc("DwmSetWindowAttribute")
)

const (
	gclStyleCW             = ^uintptr(25) // -26 in two's complement (GCL_STYLE)
	csDropShadowCW         = 0x00020000
	dwmwaNCRenderingPolicy = 2
	dwmncrpDisabled        = 1
	dwmncrpUseWindowStyle  = 0
)

// ApplyOverlayWindowRegion clips the frameless PulseCore window to a rounded
// rectangle so Mini-HUD / game overlay do not show black corners or bleed on Windows 10/11.
// HWND is resolved only for our PID — never the foreground foreign app.
func (a *App) ApplyOverlayWindowRegion(width, height, radius int) {
	_ = a
	hwnd := appMainHWND()
	if hwnd == 0 || !hwndBelongsToUs(hwnd) || width <= 0 || height <= 0 {
		return
	}
	scale := getWindowDpiScale(hwnd)
	physW := int(math.Round(float64(width) * scale))
	physH := int(math.Round(float64(height) * scale))
	if physW < 200 {
		physW = 200
	}
	if physH < 32 {
		physH = 32
	}

	applyWindowDwmPolicy(hwnd, true)

	// Update layered ARGB buffer with zeroed background to eliminate shadows & bleed
	isFPS := (radius <= 16)
	if isFPS {
		updateFPSOverlayLayeredCapsule(hwnd, physW, physH, isWindows11())
	} else {
		updateLayeredCapsule(hwnd, physW, physH, isWindows11())
	}

	// Create capsule rounded region using exact physical dimensions
	rgn, _, _ := procCreateRoundRect.Call(0, 0, uintptr(physW+1), uintptr(physH+1), uintptr(physH), uintptr(physH))
	if rgn == 0 {
		return
	}
	_, _, _ = procSetWindowRgn.Call(uintptr(hwnd), rgn, 1)
}

// ClearOverlayWindowRegion restores a rectangular window region and removes layered style.
func (a *App) ClearOverlayWindowRegion() {
	_ = a
	hwnd := appMainHWND()
	if hwnd == 0 || !hwndBelongsToUs(hwnd) {
		return
	}
	applyWindowDwmPolicy(hwnd, false)
	clearLayeredWindow(hwnd)
	_, _, _ = procSetWindowRgn.Call(uintptr(hwnd), 0, 1)
}
