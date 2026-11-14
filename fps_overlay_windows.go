//go:build windows

package main

import (
	"golang.org/x/sys/windows"
)

// updateFPSOverlayLayeredCapsule updates the F12 FPS overlay with clean ARGB graphics.
func updateFPSOverlayLayeredCapsule(hwnd windows.HWND, width, height int, isWin11 bool) bool {
	return updateLayeredCapsule(hwnd, width, height, isWin11)
}
