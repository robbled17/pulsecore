//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modNtdllOS          = windows.NewLazySystemDLL("ntdll.dll")
	procRtlGetVersionOS = modNtdllOS.NewProc("RtlGetVersion")
)

type rtlOSVersionInfoW struct {
	dwOSVersionInfoSize uint32
	dwMajorVersion      uint32
	dwMinorVersion      uint32
	dwBuildNumber       uint32
	dwPlatformId        uint32
	szCSDVersion        [128]uint16
}

const (
	dwmwaNCRenderingPolicyOS      = 2
	dwmncrpDisabledOS             = 1
	dwmwaWindowCornerPreferenceOS = 33
	dwmwcpRoundOS                 = 2
	dwmwcpDoNotRoundOS            = 1
	gclStyleOS                    = ^uintptr(25) // -26 (GCL_STYLE)
	csDropShadowOS                = 0x00020000
)

// getWindowsBuildNumber returns the exact OS build number from ntdll.RtlGetVersion.
func getWindowsBuildNumber() uint32 {
	var vi rtlOSVersionInfoW
	vi.dwOSVersionInfoSize = uint32(unsafe.Sizeof(vi))
	r, _, _ := procRtlGetVersionOS.Call(uintptr(unsafe.Pointer(&vi)))
	if r == 0 && vi.dwBuildNumber > 0 {
		return vi.dwBuildNumber
	}
	return 0
}

// isWindows11 returns true if the OS build number is >= 22000 (Windows 11).
func isWindows11() bool {
	b := getWindowsBuildNumber()
	if b > 0 {
		return b >= 22000
	}
	return isWindows11OrGreater()
}

// IsWindows11 exposes the Windows 11 check to frontend Wails bindings.
func (a *App) IsWindows11() bool {
	return isWindows11()
}

// applyWindowDwmPolicy configures DWM corner preference on Win11 and strips shadows on Win10.
func applyWindowDwmPolicy(hwnd windows.HWND, isOverlay bool) {
	if hwnd == 0 {
		return
	}
	if isWindows11() {
		if isOverlay {
			corner := uint32(dwmwcpRoundOS)
			_, _, _ = procDwmSetWindowAttribute.Call(uintptr(hwnd), uintptr(dwmwaWindowCornerPreferenceOS), uintptr(unsafe.Pointer(&corner)), 4)
		}
	} else {
		// Windows 10: disable non-client rendering shadow
		policy := uint32(dwmncrpDisabledOS)
		_, _, _ = procDwmSetWindowAttribute.Call(uintptr(hwnd), uintptr(dwmwaNCRenderingPolicyOS), uintptr(unsafe.Pointer(&policy)), 4)

		// Strip CS_DROPSHADOW
		if style, _, _ := procGetClassLongPtr.Call(uintptr(hwnd), gclStyleOS); style != 0 && (style&csDropShadowOS) != 0 {
			_, _, _ = procSetClassLongPtr.Call(uintptr(hwnd), gclStyleOS, style&^csDropShadowOS)
		}
	}
}
