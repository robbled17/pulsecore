//go:build !windows

package main

func clearLayeredWindow(hwnd uintptr)                                                   {}
func updateLayeredCapsule(hwnd uintptr, width, height int, isWin11 bool) bool           { return true }
func updateFPSOverlayLayeredCapsule(hwnd uintptr, width, height int, isWin11 bool) bool { return true }
func GetMiniHUDPhysicalDimensions(hwnd uintptr) (int, int)                              { return 380, 44 }
func GetFPSOverlayPhysicalDimensions(hwnd uintptr) (int, int)                           { return 460, 44 }
func UpdateMiniHUD(hwnd uintptr) (int, int)                                             { return 380, 44 }
func UpdateFPSOverlay(hwnd uintptr) (int, int)                                          { return 460, 44 }
