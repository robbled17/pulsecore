//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modGdi32                   = windows.NewLazySystemDLL("gdi32.dll")
	procCreateRoundRect        = modGdi32.NewProc("CreateRoundRectRgn")
	procSetWindowRgn           = windows.NewLazySystemDLL("user32.dll").NewProc("SetWindowRgn")
	procGetClassLongPtr        = windows.NewLazySystemDLL("user32.dll").NewProc("GetClassLongPtrW")
	procSetClassLongPtr        = windows.NewLazySystemDLL("user32.dll").NewProc("SetClassLongPtrW")
	modDwmapi                  = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWindowAttribute  = modDwmapi.NewProc("DwmSetWindowAttribute")
)

const (
	gclStyleCW             = ^uintptr(25) // -26 in two's complement (GCL_STYLE)
	csDropShadowCW         = 0x00020000
	dwmwaNCRenderingPolicy = 2
	dwmncrpDisabled        = 1
	dwmncrpUseWindowStyle  = 0
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
	// On Windows 10: disable DWM non-client rendering policy to remove outer window shadow completely
	policy := uint32(dwmncrpDisabled)
	_, _, _ = procDwmSetWindowAttribute.Call(uintptr(hwnd), uintptr(dwmwaNCRenderingPolicy), uintptr(unsafe.Pointer(&policy)), 4)

	// Strip CS_DROPSHADOW to eliminate dark gray rectangular border artifacts on Win10
	if style, _, _ := procGetClassLongPtr.Call(uintptr(hwnd), gclStyleCW); style != 0 && (style&csDropShadowCW) != 0 {
		_, _, _ = procSetClassLongPtr.Call(uintptr(hwnd), gclStyleCW, style&^csDropShadowCW)
	}

	// Create capsule rounded region
	rgn, _, _ := procCreateRoundRect.Call(0, 0, uintptr(width), uintptr(height), uintptr(height), uintptr(height))
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
	policy := uint32(dwmncrpUseWindowStyle)
	_, _, _ = procDwmSetWindowAttribute.Call(uintptr(hwnd), uintptr(dwmwaNCRenderingPolicy), uintptr(unsafe.Pointer(&policy)), 4)
	_, _, _ = procSetWindowRgn.Call(uintptr(hwnd), 0, 1)
}
