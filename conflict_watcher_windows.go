//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modUser32CW                 = windows.NewLazySystemDLL("user32.dll")
	procCWEnumWindows           = modUser32CW.NewProc("EnumWindows")
	procCWEnumChildWindows      = modUser32CW.NewProc("EnumChildWindows")
	procCWGetClassNameW         = modUser32CW.NewProc("GetClassNameW")
	procCWGetWindowTextW        = modUser32CW.NewProc("GetWindowTextW")
	procCWGetWindowTextLengthW  = modUser32CW.NewProc("GetWindowTextLengthW")
	procCWIsWindow              = modUser32CW.NewProc("IsWindow")
	procCWIsWindowVisible       = modUser32CW.NewProc("IsWindowVisible")
	procCWIsIconic              = modUser32CW.NewProc("IsIconic")
	procCWGetWindowRect         = modUser32CW.NewProc("GetWindowRect")
	procCWSetWindowPos          = modUser32CW.NewProc("SetWindowPos")
	procCWShowWindow            = modUser32CW.NewProc("ShowWindow")
	procCWSendMessageW          = modUser32CW.NewProc("SendMessageW")
	procCWPostMessageW          = modUser32CW.NewProc("PostMessageW")
	procCWCreateWindowExW       = modUser32CW.NewProc("CreateWindowExW")
	procCWRegisterClassExW      = modUser32CW.NewProc("RegisterClassExW")
	procCWDefWindowProcW        = modUser32CW.NewProc("DefWindowProcW")
	procCWDestroyWindow         = modUser32CW.NewProc("DestroyWindow")
	procCWSetWindowRgn          = modUser32CW.NewProc("SetWindowRgn")
	procCWInvalidateRect        = modUser32CW.NewProc("InvalidateRect")
	procCWGetDC                 = modUser32CW.NewProc("GetDC")
	procCWReleaseDC             = modUser32CW.NewProc("ReleaseDC")
	procCWGetMessageW           = modUser32CW.NewProc("GetMessageW")
	procCWTranslateMessage      = modUser32CW.NewProc("TranslateMessage")
	procCWDispatchMessageW      = modUser32CW.NewProc("DispatchMessageW")
	procCWPostQuitMessage       = modUser32CW.NewProc("PostQuitMessage")

	modGdi32CW                  = windows.NewLazySystemDLL("gdi32.dll")
	procCWCreateSolidBrush      = modGdi32CW.NewProc("CreateSolidBrush")
	procCWCreateRoundRectRgn    = modGdi32CW.NewProc("CreateRoundRectRgn")
	procCWCreateFontW           = modGdi32CW.NewProc("CreateFontW")
	procCWSetBkMode             = modGdi32CW.NewProc("SetBkMode")
	procCWSetTextColor          = modGdi32CW.NewProc("SetTextColor")
	procCWSelectObject          = modGdi32CW.NewProc("SelectObject")
	procCWDeleteObject          = modGdi32CW.NewProc("DeleteObject")
)

const (
	wsPopupCW              = 0x80000000
	wsVisibleCW            = 0x10000000
	wsChildCW              = 0x40000000
	wsExTopMostCW          = 0x00000008
	wsExToolWindowCW       = 0x00000080
	wsExNoActivateCW       = 0x08000000
	swpNoActivateCW        = 0x0010
	swpShowWindowCW        = 0x0040
	swpNoZOrderCW          = 0x0004
	swShowNoActivateCW     = 4
	swHideCW               = 0
	hwndTopMostCW          = ^uintptr(0) // -1
	bmClickCW              = 0x00F5
	wmPaintCW              = 0x000F
	wmCommandCW            = 0x0111
	wmDestroyCW            = 0x0002
	wmCtlColorStaticCW     = 0x0138
	wmCtlColorBtnCW        = 0x0135
	btnUnlockIDCW          = 1001
)

type rectCW struct {
	Left, Top, Right, Bottom int32
}

type pointCW struct {
	X, Y int32
}

type msgCW struct {
	Hwnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      pointCW
}

type wndClassExWCW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     windows.Handle
	HIcon         windows.Handle
	HCursor       windows.Handle
	HbrBackground windows.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       windows.Handle
}

type conflictState struct {
	sync.Mutex
	targetHWND   windows.HWND
	targetPath   string
	procs        []LockProcessInfo
	overlayHWND  windows.HWND
	lblHWND      windows.HWND
	btnHWND      windows.HWND
	lastRect     rectCW
	visible      bool
	bgBrush      windows.Handle
	font         windows.Handle
}

var globalConflict conflictState

// StartConflictWatcher begins the background monitoring of file conflict dialogs.
func (a *App) StartConflictWatcher() {
	go runConflictOverlayWindowThread()
	go a.conflictWatcherLoop()
}

func (a *App) conflictWatcherLoop() {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		a.mu.RLock()
		enabled := a.prefs.ConflictWatcherEnabled
		a.mu.RUnlock()

		if !enabled {
			globalConflict.Lock()
			if globalConflict.visible && globalConflict.overlayHWND != 0 {
				procCWShowWindow.Call(uintptr(globalConflict.overlayHWND), uintptr(swHideCW))
				globalConflict.visible = false
				globalConflict.targetHWND = 0
			}
			globalConflict.Unlock()
			continue
		}

		checkActiveConflictDialogs()
	}
}

func checkActiveConflictDialogs() {
	globalConflict.Lock()
	currentTarget := globalConflict.targetHWND
	overlay := globalConflict.overlayHWND
	globalConflict.Unlock()

	// If currently showing for a target, verify it's still alive and visible
	if currentTarget != 0 {
		r, _, _ := procCWIsWindow.Call(uintptr(currentTarget))
		rv, _, _ := procCWIsWindowVisible.Call(uintptr(currentTarget))
		if r == 0 || rv == 0 {
			// Dialog closed
			globalConflict.Lock()
			globalConflict.targetHWND = 0
			globalConflict.targetPath = ""
			globalConflict.procs = nil
			globalConflict.visible = false
			if overlay != 0 {
				procCWShowWindow.Call(uintptr(overlay), uintptr(swHideCW))
			}
			globalConflict.Unlock()
			return
		}

		// Update position if target moved
		var rc rectCW
		procCWGetWindowRect.Call(uintptr(currentTarget), uintptr(unsafe.Pointer(&rc)))
		globalConflict.Lock()
		if rc != globalConflict.lastRect && overlay != 0 {
			globalConflict.lastRect = rc
			w := rc.Right - rc.Left
			x := rc.Left
			y := rc.Bottom + 4
			procCWSetWindowPos.Call(
				uintptr(overlay),
				uintptr(hwndTopMostCW),
				uintptr(x),
				uintptr(y),
				uintptr(w),
				uintptr(64),
				uintptr(swpNoActivateCW|swpShowWindowCW),
			)
			// Adjust child button and label sizes
			if globalConflict.lblHWND != 0 && globalConflict.btnHWND != 0 {
				btnW := int32(200)
				btnH := int32(36)
				btnX := w - btnW - 14
				btnY := int32(14)
				procCWSetWindowPos.Call(uintptr(globalConflict.btnHWND), 0, uintptr(btnX), uintptr(btnY), uintptr(btnW), uintptr(btnH), swpNoZOrderCW)
				procCWSetWindowPos.Call(uintptr(globalConflict.lblHWND), 0, 16, 12, uintptr(btnX-24), 40, swpNoZOrderCW)
			}
		}
		globalConflict.Unlock()
		return
	}

	// Search for a conflict window
	foundHWND, foundTitle, childTexts := findConflictWindow()
	if foundHWND == 0 {
		return
	}

	// Resolve the locked item path
	targetPath := resolveLockedPath(foundTitle, childTexts)
	if targetPath == "" {
		return
	}

	// Query Restart Manager for blocking processes
	procs, err := FindLockingProcesses(targetPath)
	if err != nil || len(procs) == 0 {
		return
	}

	// Show docked overlay
	showDockedOverlay(foundHWND, targetPath, procs)
}

func showDockedOverlay(targetHWND windows.HWND, targetPath string, procs []LockProcessInfo) {
	globalConflict.Lock()
	defer globalConflict.Unlock()

	globalConflict.targetHWND = targetHWND
	globalConflict.targetPath = targetPath
	globalConflict.procs = procs

	if globalConflict.overlayHWND == 0 {
		return
	}

	var rc rectCW
	procCWGetWindowRect.Call(uintptr(targetHWND), uintptr(unsafe.Pointer(&rc)))
	globalConflict.lastRect = rc
	w := rc.Right - rc.Left
	x := rc.Left
	y := rc.Bottom + 4

	// Format label text
	var text string
	if len(procs) == 1 {
		text = fmt.Sprintf("🔒 Занято процессом: %s (PID %d)\nПуть: %s", procs[0].ProcessName, procs[0].PID, filepath.Base(targetPath))
	} else {
		names := make([]string, 0, len(procs))
		for _, p := range procs {
			names = append(names, p.ProcessName)
			if len(names) >= 2 {
				break
			}
		}
		text = fmt.Sprintf("🔒 Занято процессами (%d): %s...\nПуть: %s", len(procs), strings.Join(names, ", "), filepath.Base(targetPath))
	}

	// Update label text
	txtPtr, _ := windows.UTF16PtrFromString(text)
	procCWSendMessageW.Call(uintptr(globalConflict.lblHWND), 0x000C /* WM_SETTEXT */, 0, uintptr(unsafe.Pointer(txtPtr)))

	// Position and display overlay
	procCWSetWindowPos.Call(
		uintptr(globalConflict.overlayHWND),
		uintptr(hwndTopMostCW),
		uintptr(x),
		uintptr(y),
		uintptr(w),
		uintptr(64),
		uintptr(swpNoActivateCW|swpShowWindowCW),
	)

	// Update child button and label sizes
	btnW := int32(200)
	btnH := int32(36)
	btnX := w - btnW - 14
	btnY := int32(14)
	if globalConflict.btnHWND != 0 {
		procCWSetWindowPos.Call(uintptr(globalConflict.btnHWND), 0, uintptr(btnX), uintptr(btnY), uintptr(btnW), uintptr(btnH), swpNoZOrderCW)
	}
	if globalConflict.lblHWND != 0 {
		procCWSetWindowPos.Call(uintptr(globalConflict.lblHWND), 0, 16, 12, uintptr(btnX-24), 40, swpNoZOrderCW)
	}

	// Apply rounded corners region
	rgn, _, _ := procCWCreateRoundRectRgn.Call(0, 0, uintptr(w), 64, 14, 14)
	if rgn != 0 {
		procCWSetWindowRgn.Call(uintptr(globalConflict.overlayHWND), rgn, 1)
	}

	procCWShowWindow.Call(uintptr(globalConflict.overlayHWND), uintptr(swShowNoActivateCW))
	globalConflict.visible = true
}

func findConflictWindow() (windows.HWND, string, []string) {
	var target windows.HWND
	var winTitle string
	var allTexts []string

	cb := syscall.NewCallback(func(hwnd windows.HWND, lparam uintptr) uintptr {
		rv, _, _ := procCWIsWindowVisible.Call(uintptr(hwnd))
		if rv == 0 {
			return 1
		}
		ri, _, _ := procCWIsIconic.Call(uintptr(hwnd))
		if ri != 0 {
			return 1
		}

		var className [256]uint16
		_, _, _ = procCWGetClassNameW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&className[0])), 256)
		cls := windows.UTF16ToString(className[:])

		isTargetClass := cls == "OperationStatusWindow" || cls == "#32770"
		if !isTargetClass {
			return 1
		}

		var titleBuf [512]uint16
		_, _, _ = procCWGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&titleBuf[0])), 512)
		title := windows.UTF16ToString(titleBuf[:])

		var children []string
		childCb := syscall.NewCallback(func(ch windows.HWND, _ uintptr) uintptr {
			var chBuf [512]uint16
			_, _, _ = procCWGetWindowTextW.Call(uintptr(ch), uintptr(unsafe.Pointer(&chBuf[0])), 512)
			txt := strings.TrimSpace(windows.UTF16ToString(chBuf[:]))
			if txt != "" {
				children = append(children, txt)
			}
			return 1
		})
		procCWEnumChildWindows.Call(uintptr(hwnd), childCb, 0)

		fullContent := strings.ToLower(title + " " + strings.Join(children, " "))
		isConflict := strings.Contains(fullContent, "папка уже используется") ||
			strings.Contains(fullContent, "файл уже используется") ||
			strings.Contains(fullContent, "файл используется") ||
			strings.Contains(fullContent, "папка используется") ||
			strings.Contains(fullContent, "folder in use") ||
			strings.Contains(fullContent, "file in use") ||
			strings.Contains(fullContent, "действие не может быть выполнено") ||
			strings.Contains(fullContent, "action can't be completed")

		if isConflict {
			target = hwnd
			winTitle = title
			allTexts = children
			return 0
		}
		return 1
	})

	procCWEnumWindows.Call(cb, 0)
	return target, winTitle, allTexts
}

func resolveLockedPath(title string, childTexts []string) string {
	// 1. Check if any text is already an existing path
	for _, text := range append([]string{title}, childTexts...) {
		text = strings.TrimSpace(text)
		if len(text) >= 3 && text[1] == ':' {
			if _, err := os.Stat(text); err == nil {
				return text
			}
		}
	}

	// 2. Extract candidate item name (exclude common button / UI strings)
	var candidateNames []string
	for _, t := range childTexts {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		low := strings.ToLower(t)
		if strings.Contains(low, "повторить") || strings.Contains(low, "отмена") ||
			strings.Contains(low, "retry") || strings.Contains(low, "cancel") ||
			strings.Contains(low, "действие не может") || strings.Contains(low, "папка уже") ||
			strings.Contains(low, "файл уже") || strings.Contains(low, "закройте эту") ||
			strings.Contains(low, "the action can't") || strings.Contains(low, "folder in use") ||
			strings.Contains(low, "file in use") || len(t) > 100 {
			continue
		}
		t = strings.Trim(t, `"'«»`)
		if t != "" {
			candidateNames = append(candidateNames, t)
		}
	}

	if len(candidateNames) == 0 {
		return ""
	}

	// 3. Query open Explorer paths
	explorerDirs := getOpenExplorerPaths()

	for _, name := range candidateNames {
		for _, expDir := range explorerDirs {
			target := filepath.Join(expDir, name)
			if _, err := os.Stat(target); err == nil {
				return target
			}
		}
	}

	// 4. Try current working directory and user desktop / downloads
	userHome, _ := os.UserHomeDir()
	commonDirs := []string{
		userHome,
		filepath.Join(userHome, "Desktop"),
		filepath.Join(userHome, "Downloads"),
		filepath.Join(userHome, "Documents"),
	}
	for _, name := range candidateNames {
		for _, d := range commonDirs {
			target := filepath.Join(d, name)
			if _, err := os.Stat(target); err == nil {
				return target
			}
		}
	}

	return ""
}

func getOpenExplorerPaths() []string {
	cmd := RunHiddenCommand("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command",
		`[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; $s = New-Object -ComObject Shell.Application; $s.Windows() | ForEach-Object { try { $_.Document.Folder.Self.Path } catch {} }`)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(string(out), "\n")
	var paths []string
	for _, l := range lines {
		p := strings.TrimSpace(l)
		if p != "" && len(p) >= 2 && p[1] == ':' {
			paths = append(paths, p)
		}
	}
	return paths
}

func findRetryButton(targetHWND windows.HWND) windows.HWND {
	var retryBtn windows.HWND
	cb := syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		var className [256]uint16
		_, _, _ = procCWGetClassNameW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&className[0])), 256)
		cls := windows.UTF16ToString(className[:])

		var title [256]uint16
		_, _, _ = procCWGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&title[0])), 256)
		txt := strings.ToLower(windows.UTF16ToString(title[:]))

		if strings.Contains(strings.ToLower(cls), "button") || strings.Contains(txt, "повторить") || strings.Contains(txt, "retry") || strings.Contains(txt, "try again") {
			if strings.Contains(txt, "повторить") || strings.Contains(txt, "retry") || strings.Contains(txt, "try again") {
				retryBtn = hwnd
				return 0
			}
		}
		return 1
	})
	procCWEnumChildWindows.Call(uintptr(targetHWND), cb, 0)
	return retryBtn
}

// unlockAndRetry executes the termination of blocking processes and clicks Retry.
func unlockAndRetry() {
	globalConflict.Lock()
	targetHWND := globalConflict.targetHWND
	procs := globalConflict.procs
	overlay := globalConflict.overlayHWND
	globalConflict.Unlock()

	if len(procs) > 0 {
		_, _, _ = TerminateLockingProcesses(procs)
	}

	time.Sleep(200 * time.Millisecond)

	if targetHWND != 0 {
		btn := findRetryButton(targetHWND)
		if btn != 0 {
			procCWPostMessageW.Call(uintptr(btn), uintptr(bmClickCW), 0, 0)
		}
	}

	globalConflict.Lock()
	globalConflict.visible = false
	globalConflict.targetHWND = 0
	globalConflict.targetPath = ""
	globalConflict.procs = nil
	if overlay != 0 {
		procCWShowWindow.Call(uintptr(overlay), uintptr(swHideCW))
	}
	globalConflict.Unlock()
}

// runConflictOverlayWindowThread creates the native frameless glass overlay.
func runConflictOverlayWindowThread() {
	runtime.LockOSThread()

	className, _ := windows.UTF16PtrFromString("PulseCoreConflictOverlay")
	wndProcCallback := syscall.NewCallback(overlayWndProc)

	hInst := windows.Handle(0)
	bgBrush, _, _ := procCWCreateSolidBrush.Call(uintptr(0x00261b16)) // RGB(22, 27, 38) in BGR

	var wc wndClassExWCW
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.Style = 0x0002 | 0x0001
	wc.LpfnWndProc = wndProcCallback
	wc.HInstance = hInst
	wc.HbrBackground = windows.Handle(bgBrush)
	wc.LpszClassName = className

	procCWRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	wndTitle, _ := windows.UTF16PtrFromString("PulseCore Lock Overlay")
	hwnd, _, _ := procCWCreateWindowExW.Call(
		uintptr(wsExTopMostCW|wsExToolWindowCW|wsExNoActivateCW),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(wndTitle)),
		uintptr(wsPopupCW),
		0, 0, 400, 64,
		0, 0, uintptr(hInst), 0,
	)

	if hwnd == 0 {
		return
	}

	globalConflict.Lock()
	globalConflict.overlayHWND = windows.HWND(hwnd)
	globalConflict.bgBrush = windows.Handle(bgBrush)

	fontName, _ := windows.UTF16PtrFromString("Segoe UI")
	hFont, _, _ := procCWCreateFontW.Call(
		uintptr(15), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 2, 0,
		uintptr(unsafe.Pointer(fontName)),
	)
	globalConflict.font = windows.Handle(hFont)

	staticCls, _ := windows.UTF16PtrFromString("STATIC")
	lblTitle, _ := windows.UTF16PtrFromString("🔒 Занято процессом...")
	lbl, _, _ := procCWCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(staticCls)),
		uintptr(unsafe.Pointer(lblTitle)),
		uintptr(wsChildCW|wsVisibleCW),
		16, 12, 220, 40,
		hwnd, 0, uintptr(hInst), 0,
	)
	globalConflict.lblHWND = windows.HWND(lbl)
	procCWSendMessageW.Call(lbl, 0x0030 /* WM_SETFONT */, hFont, 1)

	btnCls, _ := windows.UTF16PtrFromString("BUTTON")
	btnTitle, _ := windows.UTF16PtrFromString("Завершить и удалить")
	btn, _, _ := procCWCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(btnCls)),
		uintptr(unsafe.Pointer(btnTitle)),
		uintptr(wsChildCW|wsVisibleCW|0x00000001 /* BS_DEFPUSHBUTTON */),
		240, 14, 200, 36,
		hwnd, uintptr(btnUnlockIDCW), uintptr(hInst), 0,
	)
	globalConflict.btnHWND = windows.HWND(btn)
	procCWSendMessageW.Call(btn, 0x0030 /* WM_SETFONT */, hFont, 1)

	globalConflict.Unlock()

	var m msgCW
	for {
		ret, _, _ := procCWGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if ret == 0 || int32(ret) == -1 {
			break
		}
		procCWTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procCWDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func overlayWndProc(hwnd windows.HWND, msg uint32, wparam uintptr, lparam uintptr) uintptr {
	switch msg {
	case wmCommandCW:
		id := int(wparam & 0xFFFF)
		if id == btnUnlockIDCW {
			go unlockAndRetry()
			return 0
		}
	case wmCtlColorStaticCW:
		hdc := wparam
		procCWSetBkMode.Call(hdc, 1 /* TRANSPARENT */)
		procCWSetTextColor.Call(hdc, uintptr(0x00E0F2FE))
		globalConflict.Lock()
		b := globalConflict.bgBrush
		globalConflict.Unlock()
		return uintptr(b)
	case wmDestroyCW:
		procCWPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procCWDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
	return r
}
