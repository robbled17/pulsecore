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
	procCWIsWindow              = modUser32CW.NewProc("IsWindow")
	procCWIsWindowVisible       = modUser32CW.NewProc("IsWindowVisible")
	procCWIsIconic              = modUser32CW.NewProc("IsIconic")
	procCWGetWindowRect         = modUser32CW.NewProc("GetWindowRect")
	procCWGetClientRect         = modUser32CW.NewProc("GetClientRect")
	procCWSetWindowPos          = modUser32CW.NewProc("SetWindowPos")
	procCWShowWindow            = modUser32CW.NewProc("ShowWindow")
	procCWPostMessageW          = modUser32CW.NewProc("PostMessageW")
	procCWCreateWindowExW       = modUser32CW.NewProc("CreateWindowExW")
	procCWRegisterClassExW      = modUser32CW.NewProc("RegisterClassExW")
	procCWDefWindowProcW        = modUser32CW.NewProc("DefWindowProcW")
	procCWDestroyWindow         = modUser32CW.NewProc("DestroyWindow")
	procCWInvalidateRect        = modUser32CW.NewProc("InvalidateRect")
	procCWUpdateWindow          = modUser32CW.NewProc("UpdateWindow")
	procCWGetMessageW           = modUser32CW.NewProc("GetMessageW")
	procCWTranslateMessage      = modUser32CW.NewProc("TranslateMessage")
	procCWDispatchMessageW      = modUser32CW.NewProc("DispatchMessageW")
	procCWPostQuitMessage       = modUser32CW.NewProc("PostQuitMessage")
	procCWBeginPaint            = modUser32CW.NewProc("BeginPaint")
	procCWEndPaint              = modUser32CW.NewProc("EndPaint")
	procCWDrawTextW             = modUser32CW.NewProc("DrawTextW")
	procCWFillRect              = modUser32CW.NewProc("FillRect")
	procCWFrameRect             = modUser32CW.NewProc("FrameRect")
	procCWLoadCursorW           = modUser32CW.NewProc("LoadCursorW")
	procCWSetCursor             = modUser32CW.NewProc("SetCursor")
	procCWGetCursorPos          = modUser32CW.NewProc("GetCursorPos")
	procCWScreenToClient        = modUser32CW.NewProc("ScreenToClient")
	procCWGetForegroundWindow   = modUser32CW.NewProc("GetForegroundWindow")
	procCWGetWindowThreadProcId = modUser32CW.NewProc("GetWindowThreadProcessId")
	procCWGetAncestor           = modUser32CW.NewProc("GetAncestor")
	procCWGetParent             = modUser32CW.NewProc("GetParent")

	modGdi32CW                  = windows.NewLazySystemDLL("gdi32.dll")
	procCWCreateSolidBrush      = modGdi32CW.NewProc("CreateSolidBrush")
	procCWCreateRoundRectRgn    = modGdi32CW.NewProc("CreateRoundRectRgn")
	procCWSetWindowRgn          = modUser32CW.NewProc("SetWindowRgn")
	procCWCreateFontW           = modGdi32CW.NewProc("CreateFontW")
	procCWSetBkMode             = modGdi32CW.NewProc("SetBkMode")
	procCWSetTextColor          = modGdi32CW.NewProc("SetTextColor")
	procCWSelectObject          = modGdi32CW.NewProc("SelectObject")
	procCWDeleteObject          = modGdi32CW.NewProc("DeleteObject")
	procCWRoundRect             = modGdi32CW.NewProc("RoundRect")
	procCWCreatePen             = modGdi32CW.NewProc("CreatePen")

	modKernel32CW                    = windows.NewLazySystemDLL("kernel32.dll")
	procCWGetModuleHandleW           = modKernel32CW.NewProc("GetModuleHandleW")
	procCWQueryFullProcessImageNameW = modKernel32CW.NewProc("QueryFullProcessImageNameW")
)

const (
	wsPopupCW              = 0x80000000
	wsExTopMostCW          = 0x00000008
	wsExToolWindowCW       = 0x00000080
	wsExNoActivateCW       = 0x08000000
	swpNoActivateCW        = 0x0010
	swpShowWindowCW        = 0x0040
	swShowNoActivateCW     = 4
	swHideCW               = 0
	hwndTopMostCW          = ^uintptr(0) // -1
	bmClickCW              = 0x00F5
	wmPaintCW              = 0x000F
	wmLButtonUpCW          = 0x0202
	wmSetCursorCW          = 0x0020
	wmEraseBkgndCW         = 0x0014
	wmDestroyCW            = 0x0002
	wmCloseCW              = 0x0010

	wmAppCW                = 0x8000
	msgShowOverlayCW       = wmAppCW + 1
	msgHideOverlayCW       = wmAppCW + 2
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

type paintStructCW struct {
	Hdc         windows.Handle
	FErase      int32
	RcPaint     rectCW
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
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
	itemName     string
	titleText    string
	subText      string
	btnText      string
	procs        []LockProcessInfo
	overlayHWND  windows.HWND
	lastRect     rectCW
	posX         int32
	posY         int32
	posW         int32
	posH         int32
	btnRect      rectCW
	visible      bool
	fontTitle    windows.Handle
	fontSub      windows.Handle
	fontBtn      windows.Handle
	cursorArrow  windows.Handle
	cursorHand   windows.Handle
}

var globalConflict conflictState

// DestroyLingeringConflictWindows finds and destroys any previous PulseCoreConflictBar windows.
func DestroyLingeringConflictWindows() {
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var buf [256]uint16
		_, _, _ = procCWGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 256)
		if windows.UTF16ToString(buf[:]) == "PulseCoreConflictBar" {
			procCWDestroyWindow.Call(hwnd)
		}
		return 1
	})
	procCWEnumWindows.Call(cb, 0)
}

// StartConflictWatcher begins the background monitoring of file conflict dialogs.
func (a *App) StartConflictWatcher() {
	DestroyLingeringConflictWindows()
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
				procCWPostMessageW.Call(uintptr(globalConflict.overlayHWND), msgHideOverlayCW, 0, 0)
				globalConflict.visible = false
				globalConflict.targetHWND = 0
			}
			globalConflict.Unlock()
			continue
		}

		checkActiveConflictDialogs()
	}
}

// isConflictTargetFocused returns true if the foreground window is the target dialog,
// the overlay itself, a child/ancestor of the target, or belongs to the same Explorer process.
func isConflictTargetFocused(targetHWND, overlayHWND windows.HWND) bool {
	fg, _, _ := procCWGetForegroundWindow.Call()
	if fg == 0 {
		return false
	}
	fgHWND := windows.HWND(fg)
	if fgHWND == targetHWND || (overlayHWND != 0 && fgHWND == overlayHWND) {
		return true
	}
	rootTarget, _, _ := procCWGetAncestor.Call(uintptr(targetHWND), 2 /* GA_ROOT */)
	rootFg, _, _ := procCWGetAncestor.Call(fg, 2 /* GA_ROOT */)
	if rootTarget != 0 && rootTarget == rootFg {
		return true
	}
	var fgPID uint32
	procCWGetWindowThreadProcId.Call(fg, uintptr(unsafe.Pointer(&fgPID)))
	var targetPID uint32
	procCWGetWindowThreadProcId.Call(uintptr(targetHWND), uintptr(unsafe.Pointer(&targetPID)))
	if fgPID != 0 && fgPID == targetPID {
		return true
	}
	return false
}

func checkActiveConflictDialogs() {
	globalConflict.Lock()
	currentTarget := globalConflict.targetHWND
	overlay := globalConflict.overlayHWND
	globalConflict.Unlock()

	// If currently tracking a conflict dialog, verify it's still alive and visible
	if currentTarget != 0 {
		r, _, _ := procCWIsWindow.Call(uintptr(currentTarget))
		rv, _, _ := procCWIsWindowVisible.Call(uintptr(currentTarget))
		if r == 0 || rv == 0 {
			// Dialog closed or dismissed
			globalConflict.Lock()
			globalConflict.targetHWND = 0
			globalConflict.targetPath = ""
			globalConflict.itemName = ""
			globalConflict.procs = nil
			globalConflict.visible = false
			if overlay != 0 {
				procCWPostMessageW.Call(uintptr(overlay), msgHideOverlayCW, 0, 0)
			}
			globalConflict.Unlock()
			return
		}

		// Focus check: only show overlay if target or its parent Explorer window or overlay is focused
		focused := isConflictTargetFocused(currentTarget, overlay)
		globalConflict.Lock()
		if !focused {
			if globalConflict.visible && overlay != 0 {
				procCWPostMessageW.Call(uintptr(overlay), msgHideOverlayCW, 0, 0)
				globalConflict.visible = false
			}
			globalConflict.Unlock()
			return
		} else if !globalConflict.visible && overlay != 0 {
			procCWPostMessageW.Call(uintptr(overlay), msgShowOverlayCW, 0, 0)
			globalConflict.visible = true
		}

		// Update position if target moved
		var rc rectCW
		procCWGetWindowRect.Call(uintptr(currentTarget), uintptr(unsafe.Pointer(&rc)))
		if rc != globalConflict.lastRect && overlay != 0 {
			globalConflict.lastRect = rc
			w := rc.Right - rc.Left
			if w < 480 {
				w = 480
			}
			globalConflict.posX = rc.Left
			globalConflict.posY = rc.Bottom + 4
			globalConflict.posW = w
			globalConflict.posH = 60
			procCWPostMessageW.Call(uintptr(overlay), msgShowOverlayCW, 0, 0)
		}
		globalConflict.Unlock()
		return
	}

	// Search for an active conflict window
	foundHWND, foundTitle, childTexts := findConflictWindow()
	if foundHWND == 0 {
		return
	}

	// Only show overlay if the conflict dialog or Explorer is currently focused
	if !isConflictTargetFocused(foundHWND, overlay) {
		return
	}

	// Extract item name and resolve locked path
	itemName := extractConflictItemName(foundTitle, childTexts)
	targetPath := resolveLockedPath(foundTitle, childTexts)

	var procs []LockProcessInfo
	if targetPath != "" {
		procs, _ = FindLockingProcesses(targetPath)
	}
	if len(procs) == 0 && itemName != "" {
		procs = findProcessesByNameOrPath(itemName, targetPath)
	}

	// Show docked overlay
	showDockedOverlay(foundHWND, targetPath, itemName, procs)
}

func showDockedOverlay(targetHWND windows.HWND, targetPath string, itemName string, procs []LockProcessInfo) {
	globalConflict.Lock()
	defer globalConflict.Unlock()

	globalConflict.targetHWND = targetHWND
	globalConflict.targetPath = targetPath
	globalConflict.itemName = itemName
	globalConflict.procs = procs

	if globalConflict.overlayHWND == 0 {
		return
	}

	var rc rectCW
	procCWGetWindowRect.Call(uintptr(targetHWND), uintptr(unsafe.Pointer(&rc)))
	globalConflict.lastRect = rc
	w := rc.Right - rc.Left
	if w < 480 {
		w = 480
	}
	globalConflict.posX = rc.Left
	globalConflict.posY = rc.Bottom + 4
	globalConflict.posW = w
	globalConflict.posH = 60

	// Format text labels according to requirements
	if len(procs) == 1 {
		p := procs[0]
		globalConflict.titleText = fmt.Sprintf("🔒 Держит процесс: %s (PID %d)", p.ProcessName, p.PID)
		globalConflict.subText = fmt.Sprintf("Объект: %s", itemName)
		globalConflict.btnText = "Завершить и повторить"
	} else if len(procs) > 1 {
		names := make([]string, 0, len(procs))
		for _, p := range procs {
			names = append(names, p.ProcessName)
			if len(names) >= 2 {
				break
			}
		}
		globalConflict.titleText = fmt.Sprintf("🔒 Держит процессов (%d): %s", len(procs), strings.Join(names, ", "))
		globalConflict.subText = fmt.Sprintf("Объект: %s", itemName)
		globalConflict.btnText = "Завершить и повторить"
	} else {
		globalConflict.titleText = "🔒 Папка занята системным дескриптором"
		globalConflict.subText = fmt.Sprintf("Объект: %s", itemName)
		globalConflict.btnText = "Повторить попытку"
	}

	procCWPostMessageW.Call(uintptr(globalConflict.overlayHWND), msgShowOverlayCW, 0, 0)
	globalConflict.visible = true
}

func findConflictWindow() (windows.HWND, string, []string) {
	var target windows.HWND
	var winTitle string
	var allTexts []string

	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		rv, _, _ := procCWIsWindowVisible.Call(hwnd)
		if rv == 0 {
			return 1
		}
		ri, _, _ := procCWIsIconic.Call(hwnd)
		if ri != 0 {
			return 1
		}

		var className [256]uint16
		_, _, _ = procCWGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&className[0])), 256)
		cls := windows.UTF16ToString(className[:])

		isTargetClass := cls == "OperationStatusWindow" || cls == "#32770"
		if !isTargetClass {
			return 1
		}

		var titleBuf [512]uint16
		_, _, _ = procCWGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&titleBuf[0])), 512)
		title := windows.UTF16ToString(titleBuf[:])

		var children []string
		childCb := syscall.NewCallback(func(ch uintptr, _ uintptr) uintptr {
			var chBuf [512]uint16
			_, _, _ = procCWGetWindowTextW.Call(ch, uintptr(unsafe.Pointer(&chBuf[0])), 512)
			txt := strings.TrimSpace(windows.UTF16ToString(chBuf[:]))
			if txt != "" {
				children = append(children, txt)
			}
			return 1
		})
		procCWEnumChildWindows.Call(hwnd, childCb, 0)

		fullContent := strings.ToLower(title + " " + strings.Join(children, " "))
		isConflict := strings.Contains(fullContent, "уже используется") ||
			strings.Contains(fullContent, "используется") ||
			strings.Contains(fullContent, "in use") ||
			strings.Contains(fullContent, "действие не может быть выполнено") ||
			strings.Contains(fullContent, "action can't be completed")

		if isConflict {
			target = windows.HWND(hwnd)
			winTitle = title
			allTexts = children
			return 0
		}
		return 1
	})

	procCWEnumWindows.Call(cb, 0)
	return target, winTitle, allTexts
}

func extractConflictItemName(title string, children []string) string {
	for _, t := range append([]string{title}, children...) {
		t = strings.TrimSpace(t)
		if len(t) >= 3 && t[1] == ':' {
			return filepath.Base(t)
		}
	}
	for _, t := range children {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		low := strings.ToLower(t)
		if strings.Contains(low, "повторить") || strings.Contains(low, "отмена") ||
			strings.Contains(low, "retry") || strings.Contains(low, "cancel") ||
			strings.Contains(low, "действие") || strings.Contains(low, "action") ||
			strings.Contains(low, "используется") || strings.Contains(low, "in use") ||
			strings.Contains(low, "закройте") || strings.Contains(low, "close") ||
			len(t) > 80 {
			continue
		}
		t = strings.Trim(t, `"'«»`)
		if t != "" {
			return t
		}
	}
	return "заблокированный объект"
}

func resolveLockedPath(title string, childTexts []string) string {
	for _, text := range append([]string{title}, childTexts...) {
		text = strings.TrimSpace(text)
		if len(text) >= 3 && text[1] == ':' {
			if _, err := os.Stat(text); err == nil {
				return text
			}
		}
	}

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
			strings.Contains(low, "file in use") || len(t) > 80 {
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

	explorerDirs := getOpenExplorerPaths()
	for _, name := range candidateNames {
		for _, expPath := range explorerDirs {
			if strings.EqualFold(filepath.Base(expPath), name) {
				if _, err := os.Stat(expPath); err == nil {
					return expPath
				}
			}
			candidatePath := filepath.Join(expPath, name)
			if _, err := os.Stat(candidatePath); err == nil {
				return candidatePath
			}
		}
	}

	return ""
}

func findProcessesByNameOrPath(name, dirPath string) []LockProcessInfo {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snap, &entry); err != nil {
		return nil
	}

	nameLower := strings.ToLower(strings.TrimSpace(name))
	nameWithoutExt := strings.TrimSuffix(nameLower, filepath.Ext(nameLower))
	dirLower := strings.ToLower(strings.TrimSpace(dirPath))

	var out []LockProcessInfo
	for {
		procName := windows.UTF16ToString(entry.ExeFile[:])
		procNameLower := strings.ToLower(procName)

		match := false
		if nameLower != "" {
			if procNameLower == nameLower || strings.TrimSuffix(procNameLower, ".exe") == nameWithoutExt {
				match = true
			} else if strings.Contains(nameLower, procNameLower) || strings.Contains(procNameLower, nameWithoutExt) {
				match = true
			}
		}

		// Also check process image path if dirPath is provided
		if !match && dirLower != "" && entry.ProcessID > 4 {
			if hProc, err := windows.OpenProcess(0x1000 /* PROCESS_QUERY_LIMITED_INFORMATION */, false, entry.ProcessID); err == nil {
				var buf [windows.MAX_PATH]uint16
				size := uint32(len(buf))
				r, _, _ := procCWQueryFullProcessImageNameW.Call(uintptr(hProc), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
				windows.CloseHandle(hProc)
				if r != 0 && size > 0 {
					exePathLower := strings.ToLower(windows.UTF16ToString(buf[:size]))
					if strings.Contains(exePathLower, dirLower) || (nameLower != "" && strings.Contains(exePathLower, nameLower)) {
						match = true
					}
				}
			}
		}

		if match && !isProtectedSystemProcess(procName, entry.ProcessID) {
			out = append(out, LockProcessInfo{
				PID:         entry.ProcessID,
				ProcessName: procName,
				AppName:     procName,
			})
		}

		if err := windows.Process32Next(snap, &entry); err != nil {
			break
		}
	}
	return out
}

func getOpenExplorerPaths() []string {
	cmd := `[Console]::OutputEncoding=[Text.Encoding]::UTF8; $shell = New-Object -ComObject Shell.Application; $shell.Windows() | ForEach-Object { try { $_.Document.SelectedItems() | ForEach-Object { $_.Path } } catch {}; try { $_.Document.Folder.Self.Path } catch {} }`
	raw, err := runHiddenOutput("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", cmd)
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	var paths []string
	for _, l := range lines {
		p := strings.TrimSpace(l)
		if len(p) >= 3 && p[1] == ':' {
			paths = append(paths, p)
		}
	}
	return paths
}

func findRetryButton(targetHWND windows.HWND) windows.HWND {
	var retryBtn windows.HWND
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var className [256]uint16
		_, _, _ = procCWGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&className[0])), 256)
		cls := windows.UTF16ToString(className[:])

		var title [256]uint16
		_, _, _ = procCWGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&title[0])), 256)
		txt := strings.ToLower(windows.UTF16ToString(title[:]))

		if strings.Contains(strings.ToLower(cls), "button") || strings.Contains(txt, "повторить") || strings.Contains(txt, "retry") || strings.Contains(txt, "try again") {
			if strings.Contains(txt, "повторить") || strings.Contains(txt, "retry") || strings.Contains(txt, "try again") {
				retryBtn = windows.HWND(hwnd)
				return 0
			}
		}
		return 1
	})
	procCWEnumChildWindows.Call(uintptr(targetHWND), cb, 0)
	return retryBtn
}

// unlockAndRetry executes the termination of blocking processes, unlocks the target path, and clicks Retry.
func unlockAndRetry() {
	globalConflict.Lock()
	targetHWND := globalConflict.targetHWND
	targetPath := globalConflict.targetPath
	itemName := globalConflict.itemName
	procs := globalConflict.procs
	overlay := globalConflict.overlayHWND
	globalConflict.Unlock()

	if len(procs) > 0 {
		_, _, _ = TerminateLockingProcesses(procs)
	} else if targetPath != "" {
		if p2, err := FindLockingProcesses(targetPath); err == nil && len(p2) > 0 {
			_, _, _ = TerminateLockingProcesses(p2)
		}
	} else if itemName != "" {
		if p3 := findProcessesByNameOrPath(itemName, ""); len(p3) > 0 {
			_, _, _ = TerminateLockingProcesses(p3)
		}
	}

	if targetPath != "" {
		_, _ = UnlockPath(targetPath)
	}

	time.Sleep(250 * time.Millisecond)

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
	globalConflict.itemName = ""
	globalConflict.procs = nil
	if overlay != 0 {
		procCWPostMessageW.Call(uintptr(overlay), msgHideOverlayCW, 0, 0)
	}
	globalConflict.Unlock()
}

// runConflictOverlayWindowThread creates the native frameless overlay on a dedicated OS thread.
func runConflictOverlayWindowThread() {
	runtime.LockOSThread()

	hInstRaw, _, _ := procCWGetModuleHandleW.Call(0)
	hInst := windows.Handle(hInstRaw)

	className, _ := windows.UTF16PtrFromString("PulseCoreConflictBar")
	wndProcCallback := syscall.NewCallback(conflictBarWndProc)

	hCursorArrow, _, _ := procCWLoadCursorW.Call(0, 32512 /* IDC_ARROW */)
	hCursorHand, _, _ := procCWLoadCursorW.Call(0, 32649 /* IDC_HAND */)

	bgBrush, _, _ := procCWCreateSolidBrush.Call(uintptr(0x002a170f)) // #0f172a in BGR

	var wc wndClassExWCW
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.Style = 0x0002 | 0x0001
	wc.LpfnWndProc = wndProcCallback
	wc.HInstance = hInst
	wc.HCursor = windows.Handle(hCursorArrow)
	wc.HbrBackground = windows.Handle(bgBrush)
	wc.LpszClassName = className

	procCWRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	wndTitle, _ := windows.UTF16PtrFromString("PulseCore Conflict Bar")
	hwnd, _, _ := procCWCreateWindowExW.Call(
		uintptr(wsExTopMostCW|wsExToolWindowCW|wsExNoActivateCW),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(wndTitle)),
		uintptr(wsPopupCW),
		0, 0, 480, 60,
		0, 0, uintptr(hInst), 0,
	)

	if hwnd == 0 {
		return
	}

	fontName, _ := windows.UTF16PtrFromString("Segoe UI")
	fTitle, _, _ := procCWCreateFontW.Call(uintptr(15), 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, 2, 0, uintptr(unsafe.Pointer(fontName)))
	fSub, _, _ := procCWCreateFontW.Call(uintptr(13), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 2, 0, uintptr(unsafe.Pointer(fontName)))
	fBtn, _, _ := procCWCreateFontW.Call(uintptr(14), 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, 2, 0, uintptr(unsafe.Pointer(fontName)))

	globalConflict.Lock()
	globalConflict.overlayHWND = windows.HWND(hwnd)
	globalConflict.fontTitle = windows.Handle(fTitle)
	globalConflict.fontSub = windows.Handle(fSub)
	globalConflict.fontBtn = windows.Handle(fBtn)
	globalConflict.cursorArrow = windows.Handle(hCursorArrow)
	globalConflict.cursorHand = windows.Handle(hCursorHand)
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

func conflictBarWndProc(hwnd uintptr, msg uintptr, wparam uintptr, lparam uintptr) uintptr {
	switch uint32(msg) {
	case msgShowOverlayCW:
		globalConflict.Lock()
		x := globalConflict.posX
		y := globalConflict.posY
		w := globalConflict.posW
		h := globalConflict.posH
		globalConflict.Unlock()

		procCWSetWindowPos.Call(
			hwnd,
			uintptr(hwndTopMostCW),
			uintptr(x),
			uintptr(y),
			uintptr(w),
			uintptr(h),
			uintptr(swpNoActivateCW|swpShowWindowCW),
		)

		rgn, _, _ := procCWCreateRoundRectRgn.Call(0, 0, uintptr(w), uintptr(h), 12, 12)
		if rgn != 0 {
			procCWSetWindowRgn.Call(hwnd, rgn, 1)
		}

		procCWInvalidateRect.Call(hwnd, 0, 1)
		procCWUpdateWindow.Call(hwnd)
		return 0

	case msgHideOverlayCW:
		procCWShowWindow.Call(hwnd, uintptr(swHideCW))
		return 0

	case wmEraseBkgndCW:
		return 1

	case wmPaintCW:
		var ps paintStructCW
		hdc, _, _ := procCWBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		if hdc == 0 {
			return 0
		}

		var cr rectCW
		procCWGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr)))

		// 1. Dark card background: #0f172a (BGR: 0x002a170f)
		bgBrush, _, _ := procCWCreateSolidBrush.Call(uintptr(0x002a170f))
		procCWFillRect.Call(hdc, uintptr(unsafe.Pointer(&cr)), bgBrush)
		procCWDeleteObject.Call(bgBrush)

		// 2. Subtle border: #334155 (BGR: 0x00554133)
		borderBrush, _, _ := procCWCreateSolidBrush.Call(uintptr(0x00554133))
		procCWFrameRect.Call(hdc, uintptr(unsafe.Pointer(&cr)), borderBrush)
		procCWDeleteObject.Call(borderBrush)

		globalConflict.Lock()
		title := globalConflict.titleText
		sub := globalConflict.subText
		btn := globalConflict.btnText
		fTitle := globalConflict.fontTitle
		fSub := globalConflict.fontSub
		fBtn := globalConflict.fontBtn
		globalConflict.Unlock()

		// Button rect on right: w=196, h=36, margin-right=14
		btnW := int32(196)
		btnH := int32(36)
		btnR := cr.Right - 14
		btnL := btnR - btnW
		btnT := (cr.Bottom - btnH) / 2
		btnB := btnT + btnH
		btnRect := rectCW{Left: btnL, Top: btnT, Right: btnR, Bottom: btnB}

		globalConflict.Lock()
		globalConflict.btnRect = btnRect
		globalConflict.Unlock()

		// 3. Draw text on left
		procCWSetBkMode.Call(hdc, 1 /* TRANSPARENT */)

		titleRect := rectCW{Left: 18, Top: 10, Right: btnL - 14, Bottom: 30}
		subRect := rectCW{Left: 18, Top: 32, Right: btnL - 14, Bottom: 50}

		titlePtr, _ := windows.UTF16PtrFromString(title)
		procCWSetTextColor.Call(hdc, uintptr(0x00f8fafc)) // White
		oldFont, _, _ := procCWSelectObject.Call(hdc, uintptr(fTitle))
		procCWDrawTextW.Call(hdc, uintptr(unsafe.Pointer(titlePtr)), ^uintptr(0), uintptr(unsafe.Pointer(&titleRect)), 0x0000 /* DT_LEFT */)

		subPtr, _ := windows.UTF16PtrFromString(sub)
		procCWSetTextColor.Call(hdc, uintptr(0x0094a3b8)) // Slate-400
		procCWSelectObject.Call(hdc, uintptr(fSub))
		procCWDrawTextW.Call(hdc, uintptr(unsafe.Pointer(subPtr)), ^uintptr(0), uintptr(unsafe.Pointer(&subRect)), 0x0000 /* DT_LEFT */)

		// 4. Draw blue button: #3b82f6 (BGR: 0x00f6823b)
		btnBrush, _, _ := procCWCreateSolidBrush.Call(uintptr(0x00f6823b))
		nullPen, _, _ := procCWCreatePen.Call(5 /* PS_NULL */, 0, 0)
		oldBrush, _, _ := procCWSelectObject.Call(hdc, btnBrush)
		oldPen, _, _ := procCWSelectObject.Call(hdc, nullPen)

		procCWRoundRect.Call(hdc, uintptr(btnRect.Left), uintptr(btnRect.Top), uintptr(btnRect.Right), uintptr(btnRect.Bottom), 8, 8)

		procCWSelectObject.Call(hdc, oldBrush)
		procCWSelectObject.Call(hdc, oldPen)
		procCWDeleteObject.Call(btnBrush)
		procCWDeleteObject.Call(nullPen)

		// Button text: White bold
		btnPtr, _ := windows.UTF16PtrFromString(btn)
		procCWSetTextColor.Call(hdc, uintptr(0x00ffffff))
		procCWSelectObject.Call(hdc, uintptr(fBtn))
		procCWDrawTextW.Call(hdc, uintptr(unsafe.Pointer(btnPtr)), ^uintptr(0), uintptr(unsafe.Pointer(&btnRect)), 0x0001|0x0004|0x0020 /* DT_CENTER | DT_VCENTER | DT_SINGLELINE */)

		procCWSelectObject.Call(hdc, oldFont)
		procCWEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0

	case wmLButtonUpCW:
		x := int32(int16(lparam & 0xFFFF))
		y := int32(int16((lparam >> 16) & 0xFFFF))
		globalConflict.Lock()
		br := globalConflict.btnRect
		globalConflict.Unlock()

		if x >= br.Left && x <= br.Right && y >= br.Top && y <= br.Bottom {
			go unlockAndRetry()
		}
		return 0

	case wmSetCursorCW:
		var pt pointCW
		procCWGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
		procCWScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&pt)))

		globalConflict.Lock()
		br := globalConflict.btnRect
		hHand := globalConflict.cursorHand
		hArrow := globalConflict.cursorArrow
		globalConflict.Unlock()

		if pt.X >= br.Left && pt.X <= br.Right && pt.Y >= br.Top && pt.Y <= br.Bottom && hHand != 0 {
			procCWSetCursor.Call(uintptr(hHand))
			return 1
		}
		if hArrow != 0 {
			procCWSetCursor.Call(uintptr(hArrow))
			return 1
		}

	case wmCloseCW:
		procCWDestroyWindow.Call(hwnd)
		return 0

	case wmDestroyCW:
		procCWPostQuitMessage.Call(0)
		return 0
	}

	r, _, _ := procCWDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return r
}
