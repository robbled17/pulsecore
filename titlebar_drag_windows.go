//go:build windows

package main

import (
	"sync"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

var (
	modUser32TB          = windows.NewLazySystemDLL("user32.dll")
	procReleaseCapture   = modUser32TB.NewProc("ReleaseCapture")
	procSendMessageW     = modUser32TB.NewProc("SendMessageW")
	procGetCursorPos     = modUser32TB.NewProc("GetCursorPos")
	procGetWindowRect    = modUser32TB.NewProc("GetWindowRect")
	procMonitorFromPoint = modUser32TB.NewProc("MonitorFromPoint")
	procGetMonitorInfoW  = modUser32TB.NewProc("GetMonitorInfoW")
	procGetSystemMetrics = modUser32TB.NewProc("GetSystemMetrics")
	procEnumWindows      = modUser32TB.NewProc("EnumWindows")
	procIsWindowVisible  = modUser32TB.NewProc("IsWindowVisible")
	procGetWindowThread  = modUser32TB.NewProc("GetWindowThreadProcessId")
	procGetAncestor      = modUser32TB.NewProc("GetAncestor")
	procIsWindow         = modUser32TB.NewProc("IsWindow")
	procGetForeground    = modUser32TB.NewProc("GetForegroundWindow")
)

const (
	wmNCLButtonDown         = 0x00A1
	htCaption               = 0x0002
	monitorDefaultToNearest = 2
	smCXScreen              = 0
	smCYScreen              = 1
	gaRoot                  = 2
)

type pointTB struct {
	X, Y int32
}

type rectTB struct {
	Left, Top, Right, Bottom int32
}

type monitorInfoTB struct {
	CbSize    uint32
	RcMonitor rectTB
	RcWork    rectTB
	DwFlags   uint32
}

var (
	cachedMainHWNDMu sync.Mutex
	cachedMainHWND   windows.HWND
)

func hwndBelongsToUs(hwnd windows.HWND) bool {
	if hwnd == 0 {
		return false
	}
	alive, _, _ := procIsWindow.Call(uintptr(hwnd))
	if alive == 0 {
		return false
	}
	var wpid uint32
	_, _, _ = procGetWindowThread.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&wpid)))
	return wpid == windows.GetCurrentProcessId()
}

var (
	enumFoundHWND   windows.HWND
	enumFoundHWNDMu sync.Mutex
)

var enumMainHWNDCb = windows.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
	vis, _, _ := procIsWindowVisible.Call(hwnd)
	if vis == 0 {
		return 1
	}
	var wpid uint32
	_, _, _ = procGetWindowThread.Call(hwnd, uintptr(unsafe.Pointer(&wpid)))
	if wpid != windows.GetCurrentProcessId() {
		return 1
	}
	root, _, _ := procGetAncestor.Call(hwnd, gaRoot)
	if root != 0 {
		enumFoundHWND = windows.HWND(root)
	} else {
		enumFoundHWND = windows.HWND(hwnd)
	}
	return 0
})

func findOurMainHWND() windows.HWND {
	enumFoundHWNDMu.Lock()
	defer enumFoundHWNDMu.Unlock()
	enumFoundHWND = 0
	_, _, _ = procEnumWindows.Call(enumMainHWNDCb, 0)
	return enumFoundHWND
}

func appMainHWND() windows.HWND {
	cachedMainHWNDMu.Lock()
	defer cachedMainHWNDMu.Unlock()
	if hwndBelongsToUs(cachedMainHWND) {
		return cachedMainHWND
	}
	if fg, _, _ := procGetForeground.Call(); fg != 0 {
		root, _, _ := procGetAncestor.Call(fg, gaRoot)
		cand := windows.HWND(fg)
		if root != 0 {
			cand = windows.HWND(root)
		}
		if hwndBelongsToUs(cand) {
			cachedMainHWND = cand
			return cand
		}
	}
	found := findOurMainHWND()
	cachedMainHWND = found
	return found
}

func invalidateMainHWNDCache() {
	cachedMainHWNDMu.Lock()
	cachedMainHWND = 0
	cachedMainHWNDMu.Unlock()
}

// TitlebarDragBegin starts a native caption drag. If maximised, restores first and
// keeps the cursor's relative X on the titlebar (work-area / multi-monitor aware).
func (a *App) TitlebarDragBegin() {
	if a.ctx == nil {
		return
	}
	hwnd := appMainHWND()
	if hwnd == 0 || !hwndBelongsToUs(hwnd) {
		return
	}

	var pt pointTB
	_, _, _ = procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	maximised := runtime.WindowIsMaximised(a.ctx)
	if maximised {
		var wr rectTB
		_, _, _ = procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&wr)))
		winW := int(wr.Right - wr.Left)
		frac := 0.5
		if winW > 0 {
			frac = float64(int(pt.X)-int(wr.Left)) / float64(winW)
			if frac < 0 {
				frac = 0
			}
			if frac > 1 {
				frac = 1
			}
		}

		runtime.WindowUnmaximise(a.ctx)

		rw, rh := mainWindowW, mainWindowH
		if w, h := runtime.WindowGetSize(a.ctx); w > 0 && h > 0 {
			rw, rh = w, h
		}
		newX := int(pt.X) - int(float64(rw)*frac)
		newY := int(pt.Y) - 18
		newX, newY = clampWindowToWorkArea(pt, newX, newY, rw, rh)
		runtime.WindowSetPosition(a.ctx, newX, newY)
	}

	_, _, _ = procReleaseCapture.Call()
	_, _, _ = procSendMessageW.Call(uintptr(hwnd), wmNCLButtonDown, htCaption, 0)
}

// TitlebarToggleMaximise toggles maximised state (double-click on caption).
func (a *App) TitlebarToggleMaximise() {
	if a.ctx == nil {
		return
	}
	runtime.WindowToggleMaximise(a.ctx)
}

// WindowMaximisedState is exposed for frontend state sync.
func (a *App) WindowMaximisedState() bool {
	if a.ctx == nil {
		return false
	}
	return runtime.WindowIsMaximised(a.ctx)
}

func clampWindowToWorkArea(pt pointTB, x, y, w, h int) (int, int) {
	ptVal := uintptr(uint32(pt.X)) | (uintptr(uint32(pt.Y)) << 32)
	mon, _, _ := procMonitorFromPoint.Call(ptVal, monitorDefaultToNearest)
	work := rectTB{Left: 0, Top: 0, Right: int32(getSysMetric(smCXScreen)), Bottom: int32(getSysMetric(smCYScreen))}
	if mon != 0 {
		var mi monitorInfoTB
		mi.CbSize = uint32(unsafe.Sizeof(mi))
		r, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi)))
		if r != 0 {
			work = mi.RcWork
		}
	}
	if x < int(work.Left) {
		x = int(work.Left)
	}
	if y < int(work.Top) {
		y = int(work.Top)
	}
	if x+w > int(work.Right) {
		x = int(work.Right) - w
	}
	if y+h > int(work.Bottom) {
		y = int(work.Bottom) - h
	}
	return x, y
}

func getSysMetric(idx int) int {
	r, _, _ := procGetSystemMetrics.Call(uintptr(idx))
	return int(r)
}
