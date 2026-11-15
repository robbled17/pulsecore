//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/windows"
)

var (
	modUser32CW                    = windows.NewLazySystemDLL("user32.dll")
	procCWEnumWindows              = modUser32CW.NewProc("EnumWindows")
	procCWEnumChildWindows         = modUser32CW.NewProc("EnumChildWindows")
	procCWGetClassNameW            = modUser32CW.NewProc("GetClassNameW")
	procCWGetWindowTextW           = modUser32CW.NewProc("GetWindowTextW")
	procCWGetWindowThreadProcessId = modUser32CW.NewProc("GetWindowThreadProcessId")
	procCWIsWindow                 = modUser32CW.NewProc("IsWindow")
	procCWIsWindowVisible       = modUser32CW.NewProc("IsWindowVisible")
	procCWIsIconic              = modUser32CW.NewProc("IsIconic")
	procCWGetWindowRect         = modUser32CW.NewProc("GetWindowRect")
	procCWGetClientRect         = modUser32CW.NewProc("GetClientRect")
	procCWSetWindowPos          = modUser32CW.NewProc("SetWindowPos")
	procCWShowWindow            = modUser32CW.NewProc("ShowWindow")
	procCWPostMessageW          = modUser32CW.NewProc("PostMessageW")
	procCWSendMessageW          = modUser32CW.NewProc("SendMessageW")
	procCWGetDlgCtrlID          = modUser32CW.NewProc("GetDlgCtrlID")
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
	wmCommandCW            = 0x0111
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
	closeRect    rectCW
	visible      bool
	fontTitle    windows.Handle
	fontSub      windows.Handle
	fontBtn      windows.Handle
	cursorArrow  windows.Handle
	cursorHand   windows.Handle
}

var (
	globalConflict       conflictState
	dismissCooldownUntil time.Time
)

// isExcludedConflictProcess returns true for processes that must never be considered conflict lockers or displayed in the overlay.
func isExcludedConflictProcess(procName string, pid uint32) bool {
	if pid != 0 && pid == uint32(os.Getpid()) {
		return true
	}
	clean := strings.ToLower(filepath.Base(strings.TrimSpace(procName)))
	if clean == "" || clean == "pulsecore.exe" || clean == "pulsecore" {
		return true
	}
	return isProtectedSystemProcess(clean, pid) || isSelfOrWebView(clean)
}

func resetConflictState(overlay windows.HWND) {
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
}

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
	defer func() {
		if r := recover(); r != nil {
			logCrash("conflictWatcherLoop", r)
		}
	}()
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
	if time.Now().Before(dismissCooldownUntil) {
		return
	}

	globalConflict.Lock()
	currentTarget := globalConflict.targetHWND
	overlay := globalConflict.overlayHWND
	globalConflict.Unlock()

	// If currently tracking a conflict dialog, verify it's still alive and showing conflict
	if currentTarget != 0 {
		if !isWindowStillConflict(currentTarget) {
			resetConflictState(overlay)
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
			if w < 500 {
				w = 500
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
	targetPath := resolveLockedPath(foundTitle, childTexts, itemName)

	var procs []LockProcessInfo
	// Way 1: Restart Manager per files in targetPath
	if targetPath != "" {
		procs, _ = FindLockingProcesses(targetPath)
	}

	// Filter out any excluded processes from RM
	var validProcs []LockProcessInfo
	for _, p := range procs {
		if !isExcludedConflictProcess(p.ProcessName, p.PID) && !isExcludedConflictProcess(p.AppName, p.PID) {
			validProcs = append(validProcs, p)
		}
	}
	procs = validProcs

	// Way 2: Process & window matching if RM returned empty
	if len(procs) == 0 && (itemName != "" || targetPath != "") {
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
	if w < 500 {
		w = 500
	}
	globalConflict.posX = rc.Left
	globalConflict.posY = rc.Bottom + 4
	globalConflict.posW = w
	globalConflict.posH = 60

	title, sub, btn := formatConflictOverlayLabels(procs, itemName)
	globalConflict.titleText = title
	globalConflict.subText = sub
	globalConflict.btnText = btn

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

var (
	explorerStateMu       sync.Mutex
	cachedSelectedPaths   []string
	cachedOpenFolders     []string
	lastExplorerStateTime time.Time
)

func getExplorerState() ([]string, []string) {
	explorerStateMu.Lock()
	defer explorerStateMu.Unlock()

	if time.Since(lastExplorerStateTime) < 500*time.Millisecond && (len(cachedSelectedPaths) > 0 || len(cachedOpenFolders) > 0) {
		return cachedSelectedPaths, cachedOpenFolders
	}

	cmd := `[Console]::OutputEncoding=[Text.Encoding]::UTF8; $shell = New-Object -ComObject Shell.Application; $shell.Windows() | ForEach-Object { try { $_.Document.SelectedItems() | ForEach-Object { "SEL|" + $_.Path } } catch {}; try { $p = $_.Document.Folder.Self.Path; if ($p) { "DIR|" + $p } } catch {} }`
	raw, err := runHiddenOutput("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", cmd)
	if err != nil {
		return cachedSelectedPaths, cachedOpenFolders
	}

	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	var sel, dirs []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "SEL|") {
			p := strings.TrimPrefix(l, "SEL|")
			if len(p) >= 3 && p[1] == ':' {
				sel = append(sel, p)
			}
		} else if strings.HasPrefix(l, "DIR|") {
			p := strings.TrimPrefix(l, "DIR|")
			if len(p) >= 3 && p[1] == ':' {
				dirs = append(dirs, p)
			}
		}
	}
	cachedSelectedPaths = sel
	cachedOpenFolders = dirs
	lastExplorerStateTime = time.Now()
	return sel, dirs
}

func extractConflictItemName(title string, children []string) string {
	// 1. High priority: check open Explorer windows for currently selected file/folder
	sel, _ := getExplorerState()
	if len(sel) > 0 {
		return filepath.Base(sel[0])
	}

	// 2. Direct path or quotes in title or children
	for _, t := range append([]string{title}, children...) {
		t = strings.TrimSpace(t)
		if len(t) >= 3 && t[1] == ':' {
			base := filepath.Base(t)
			if !isExcludedConflictProcess(base, 0) {
				return base
			}
		}

		// Look for quoted names like "Antigravity Tools" or «Antigravity Tools»
		if start := strings.IndexAny(t, `"'«`); start >= 0 {
			rest := t[start+1:]
			if end := strings.IndexAny(rest, `"'»`); end > 0 {
				candidate := strings.TrimSpace(rest[:end])
				if candidate != "" && len(candidate) < 100 && !strings.Contains(candidate, "\n") && !isExcludedConflictProcess(candidate, 0) {
					return candidate
				}
			}
		}

		// Look for text following "повторите попытку" or "try again"
		for _, marker := range []string{"повторите попытку.", "повторите попытку:", "повторите попытку", "try again.", "try again:", "try again"} {
			if idx := strings.Index(strings.ToLower(t), marker); idx >= 0 {
				after := strings.TrimSpace(t[idx+len(marker):])
				lines := strings.Split(after, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					line = strings.Trim(line, `"'«»:,; `)
					if line != "" && !strings.HasPrefix(strings.ToLower(line), "дата") && len(line) < 100 && !isExcludedConflictProcess(line, 0) {
						return line
					}
				}
			}
		}
	}

	// 3. Child text lines
	for _, t := range children {
		lines := strings.Split(t, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			low := strings.ToLower(line)
			if strings.Contains(low, "повторить") || strings.Contains(low, "отмена") ||
				strings.Contains(low, "retry") || strings.Contains(low, "cancel") ||
				strings.Contains(low, "действие") || strings.Contains(low, "action") ||
				strings.Contains(low, "используется") || strings.Contains(low, "in use") ||
				strings.Contains(low, "закройте") || strings.Contains(low, "close") ||
				strings.Contains(low, "дата создания") || strings.Contains(low, "размер") ||
				len(line) > 80 {
				continue
			}
			line = strings.Trim(line, `"'«»:,; `)
			if line != "" && !isExcludedConflictProcess(line, 0) {
				return line
			}
		}
	}
	return "заблокированный объект"
}

func resolveLockedPath(title string, childTexts []string, itemName string) string {
	sel, dirs := getExplorerState()
	if len(sel) > 0 {
		if itemName != "" {
			for _, s := range sel {
				if strings.EqualFold(filepath.Base(s), itemName) {
					return s
				}
			}
		}
		return sel[0]
	}

	for _, text := range append([]string{title}, childTexts...) {
		text = strings.TrimSpace(text)
		if len(text) >= 3 && text[1] == ':' {
			if _, err := os.Stat(text); err == nil {
				return text
			}
		}
	}

	if itemName != "" {
		for _, expPath := range dirs {
			if strings.EqualFold(filepath.Base(expPath), itemName) {
				if _, err := os.Stat(expPath); err == nil {
					return expPath
				}
			}
			candidatePath := filepath.Join(expPath, itemName)
			if _, err := os.Stat(candidatePath); err == nil {
				return candidatePath
			}
		}
	}

	return ""
}

func matchProcessWithFolder(procName, exePath, cwd, cmdline string, windowTitles []string, folderName, folderPath string) bool {
	fName := strings.ToLower(strings.TrimSpace(folderName))
	fPath := strings.ToLower(strings.TrimSpace(folderPath))
	pName := strings.ToLower(strings.TrimSpace(procName))
	pBase := strings.TrimSuffix(pName, ".exe")

	if fName == "" && fPath == "" {
		return false
	}

	// 1. Never match protected or self
	if isExcludedConflictProcess(procName, 0) {
		return false
	}

	// 2. Exe path inside folderPath
	exeLower := strings.ToLower(exePath)
	if fPath != "" && exeLower != "" && strings.Contains(exeLower, fPath) {
		return true
	}

	// 3. CWD inside folderPath
	cwdLower := strings.ToLower(cwd)
	if fPath != "" && cwdLower != "" && strings.Contains(cwdLower, fPath) {
		return true
	}

	// 4. Cmdline contains folderPath or folderName
	cmdLower := strings.ToLower(cmdline)
	if fPath != "" && strings.Contains(cmdLower, fPath) {
		return true
	}
	if fName != "" && strings.Contains(cmdLower, fName) {
		return true
	}

	// 5. Match by folder name tokens (e.g. "Antigravity Tools" -> "antigravity", "tools")
	nameTokens := strings.Fields(fName)
	var validTokens []string
	for _, tok := range nameTokens {
		tok = strings.Trim(tok, `"'«».,;:()[]{}!?-_`)
		if len(tok) >= 3 {
			validTokens = append(validTokens, tok)
		}
	}

	// Compare process name / base
	if fName != "" {
		if pName == fName || pBase == fName {
			return true
		}
		if strings.Contains(fName, pBase) && len(pBase) >= 3 {
			return true
		}
		if strings.Contains(pBase, fName) {
			return true
		}
		for _, tok := range validTokens {
			if pBase == tok || strings.Contains(pBase, tok) || strings.Contains(tok, pBase) {
				return true
			}
		}
	}

	// 6. Window titles
	for _, t := range windowTitles {
		tLower := strings.ToLower(t)
		if fName != "" && strings.Contains(tLower, fName) {
			return true
		}
		for _, tok := range validTokens {
			if strings.Contains(tLower, tok) {
				return true
			}
		}
	}

	return false
}

func findProcessesByNameOrPath(name, dirPath string) []LockProcessInfo {
	// Build map of PID -> Window titles
	pidWindows := map[uint32][]string{}
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if rv, _, _ := procCWIsWindowVisible.Call(hwnd); rv == 0 {
			return 1
		}
		var pid uint32
		procCWGetWindowThreadProcId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid > 4 {
			var buf [512]uint16
			procCWGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 512)
			txt := strings.TrimSpace(windows.UTF16ToString(buf[:]))
			if txt != "" {
				pidWindows[pid] = append(pidWindows[pid], txt)
			}
		}
		return 1
	})
	procCWEnumWindows.Call(cb, 0)

	procs, err := process.Processes()
	if err != nil {
		return nil
	}

	myPID := int32(os.Getpid())
	var out []LockProcessInfo
	seen := map[int32]bool{}

	for _, p := range procs {
		if p.Pid <= 4 || p.Pid == myPID || seen[p.Pid] {
			continue
		}
		pName, err := p.Name()
		if err != nil || pName == "" {
			continue
		}
		if isExcludedConflictProcess(pName, uint32(p.Pid)) {
			continue
		}

		var exePath, cwd, cmdline string
		if e, err := p.Exe(); err == nil {
			exePath = e
		}
		if c, err := p.Cwd(); err == nil {
			cwd = c
		}
		if cmd, err := p.Cmdline(); err == nil {
			cmdline = cmd
		}
		wTitles := pidWindows[uint32(p.Pid)]

		if matchProcessWithFolder(pName, exePath, cwd, cmdline, wTitles, name, dirPath) {
			seen[p.Pid] = true
			out = append(out, LockProcessInfo{
				PID:         uint32(p.Pid),
				ProcessName: pName,
				AppName:     pName,
				ExePath:     exePath,
			})
		}
	}
	return out
}

func formatConflictOverlayLabels(procs []LockProcessInfo, itemName string) (titleText, subText, btnText string) {
	btnText = "Закрыть и удалить"
	cleanItem := strings.TrimSpace(itemName)
	if cleanItem == "" {
		cleanItem = "заблокированный объект"
	}

	var validProcs []LockProcessInfo
	for _, p := range procs {
		if !isExcludedConflictProcess(p.ProcessName, p.PID) && !isExcludedConflictProcess(p.AppName, p.PID) {
			validProcs = append(validProcs, p)
		}
	}

	subText = fmt.Sprintf("Объект: %s", cleanItem)

	if len(validProcs) == 1 {
		pName := validProcs[0].ProcessName
		if pName == "" {
			pName = validProcs[0].AppName
		}
		titleText = fmt.Sprintf("🔒 Держит: %s", pName)
	} else if len(validProcs) > 1 {
		pName := validProcs[0].ProcessName
		if pName == "" {
			pName = validProcs[0].AppName
		}
		titleText = fmt.Sprintf("🔒 Держит: %s и ещё %d", pName, len(validProcs)-1)
	} else {
		titleText = fmt.Sprintf("🔒 Заблокировано: %s", cleanItem)
		subText = "Папка или файл заняты процессом"
	}
	return titleText, subText, btnText
}

func isWindowStillConflict(hwnd windows.HWND) bool {
	r, _, _ := procCWIsWindow.Call(uintptr(hwnd))
	rv, _, _ := procCWIsWindowVisible.Call(uintptr(hwnd))
	if r == 0 || rv == 0 {
		return false
	}

	var titleBuf [512]uint16
	_, _, _ = procCWGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&titleBuf[0])), 512)
	title := windows.UTF16ToString(titleBuf[:])

	var children []string
	cb := syscall.NewCallback(func(ch uintptr, _ uintptr) uintptr {
		var chBuf [512]uint16
		_, _, _ = procCWGetWindowTextW.Call(ch, uintptr(unsafe.Pointer(&chBuf[0])), 512)
		txt := strings.TrimSpace(windows.UTF16ToString(chBuf[:]))
		if txt != "" {
			children = append(children, txt)
		}
		return 1
	})
	procCWEnumChildWindows.Call(uintptr(hwnd), cb, 0)

	fullContent := strings.ToLower(title + " " + strings.Join(children, " "))
	return strings.Contains(fullContent, "уже используется") ||
		strings.Contains(fullContent, "используется") ||
		strings.Contains(fullContent, "in use") ||
		strings.Contains(fullContent, "действие не может быть выполнено") ||
		strings.Contains(fullContent, "action can't be completed")
}

func findRetryButton(targetHWND windows.HWND) windows.HWND {
	var retryBtn windows.HWND
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		ctrlID, _, _ := procCWGetDlgCtrlID.Call(hwnd)
		if ctrlID == 4 { // IDRETRY
			retryBtn = windows.HWND(hwnd)
			return 0
		}

		var className [256]uint16
		_, _, _ = procCWGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&className[0])), 256)
		cls := windows.UTF16ToString(className[:])

		var title [256]uint16
		_, _, _ = procCWGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&title[0])), 256)
		txt := strings.ToLower(windows.UTF16ToString(title[:]))

		if strings.Contains(strings.ToLower(cls), "button") {
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

func terminateProcessTree(pid uint32, procName string) {
	if pid <= 4 || isExcludedConflictProcess(procName, pid) {
		return
	}

	hProc, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err == nil && hProc != 0 {
		_ = windows.TerminateProcess(hProc, 1)
		_ = windows.CloseHandle(hProc)
	}

	_ = exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid)).Run()
}

// unlockAndRetry executes the termination of blocking processes, unlocks the target path, and clicks Retry.
func unlockAndRetry() {
	defer func() {
		if r := recover(); r != nil {
			logCrash("unlockAndRetry", r)
		}
	}()
	globalConflict.Lock()
	targetHWND := globalConflict.targetHWND
	targetPath := globalConflict.targetPath
	procs := globalConflict.procs
	overlay := globalConflict.overlayHWND
	globalConflict.Unlock()

	for _, p := range procs {
		terminateProcessTree(p.PID, p.ProcessName)
	}

	if targetPath != "" {
		_, _ = UnlockPath(targetPath)
	}

	time.Sleep(350 * time.Millisecond)

	if targetHWND != 0 {
		btn := findRetryButton(targetHWND)
		if btn != 0 {
			procCWPostMessageW.Call(uintptr(btn), uintptr(bmClickCW), 0, 0)
			procCWSendMessageW.Call(uintptr(targetHWND), uintptr(wmCommandCW), uintptr(4 /* IDRETRY */), uintptr(btn))
		}
	}

	resetConflictState(overlay)
}

// runConflictOverlayWindowThread creates the native frameless overlay on a dedicated OS thread.
func runConflictOverlayWindowThread() {
	defer func() {
		if r := recover(); r != nil {
			logCrash("conflictOverlayWindowThread", r)
		}
	}()
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

		rgn, _, _ := procCWCreateRoundRectRgn.Call(0, 0, uintptr(w), uintptr(h), uintptr(h), uintptr(h))
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

		// Close button rect: w=24, h=24, margin-right=12
		closeW := int32(24)
		closeH := int32(24)
		closeR := cr.Right - 12
		closeL := closeR - closeW
		closeT := (cr.Bottom - closeH) / 2
		closeB := closeT + closeH
		closeRect := rectCW{Left: closeL, Top: closeT, Right: closeR, Bottom: closeB}

		// Button rect on right: w=196, h=36, 10px to left of close button
		btnW := int32(196)
		btnH := int32(36)
		btnR := closeL - 10
		btnL := btnR - btnW
		btnT := (cr.Bottom - btnH) / 2
		btnB := btnT + btnH
		btnRect := rectCW{Left: btnL, Top: btnT, Right: btnR, Bottom: btnB}

		globalConflict.Lock()
		globalConflict.btnRect = btnRect
		globalConflict.closeRect = closeRect
		globalConflict.Unlock()

		// 3. Draw text on left
		procCWSetBkMode.Call(hdc, 1 /* TRANSPARENT */)

		titleRect := rectCW{Left: 18, Top: 10, Right: btnL - 12, Bottom: 30}
		subRect := rectCW{Left: 18, Top: 32, Right: btnL - 12, Bottom: 50}

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

		procCWRoundRect.Call(hdc, uintptr(btnRect.Left), uintptr(btnRect.Top), uintptr(btnRect.Right), uintptr(btnRect.Bottom), uintptr(btnH), uintptr(btnH))

		procCWSelectObject.Call(hdc, oldBrush)
		procCWSelectObject.Call(hdc, oldPen)
		procCWDeleteObject.Call(btnBrush)
		procCWDeleteObject.Call(nullPen)

		// Button text: White bold
		btnPtr, _ := windows.UTF16PtrFromString(btn)
		procCWSetTextColor.Call(hdc, uintptr(0x00ffffff))
		procCWSelectObject.Call(hdc, uintptr(fBtn))
		procCWDrawTextW.Call(hdc, uintptr(unsafe.Pointer(btnPtr)), ^uintptr(0), uintptr(unsafe.Pointer(&btnRect)), 0x0001|0x0004|0x0020 /* DT_CENTER | DT_VCENTER | DT_SINGLELINE */)

		// 5. Draw Close button '✕'
		closePtr, _ := windows.UTF16PtrFromString("✕")
		procCWSetTextColor.Call(hdc, uintptr(0x0094a3b8))
		procCWDrawTextW.Call(hdc, uintptr(unsafe.Pointer(closePtr)), ^uintptr(0), uintptr(unsafe.Pointer(&closeRect)), 0x0001|0x0004|0x0020)

		procCWSelectObject.Call(hdc, oldFont)
		procCWEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0

	case wmLButtonUpCW:
		x := int32(int16(lparam & 0xFFFF))
		y := int32(int16((lparam >> 16) & 0xFFFF))
		globalConflict.Lock()
		br := globalConflict.btnRect
		cr := globalConflict.closeRect
		globalConflict.Unlock()

		if x >= cr.Left && x <= cr.Right && y >= cr.Top && y <= cr.Bottom {
			dismissCooldownUntil = time.Now().Add(2 * time.Second)
			resetConflictState(windows.HWND(hwnd))
			return 0
		}

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
		cr := globalConflict.closeRect
		hHand := globalConflict.cursorHand
		hArrow := globalConflict.cursorArrow
		globalConflict.Unlock()

		if ((pt.X >= br.Left && pt.X <= br.Right && pt.Y >= br.Top && pt.Y <= br.Bottom) ||
			(pt.X >= cr.Left && pt.X <= cr.Right && pt.Y >= cr.Top && pt.Y <= cr.Bottom)) && hHand != 0 {
			procCWSetCursor.Call(uintptr(hHand))
			return 1
		}
		if hArrow != 0 {
			procCWSetCursor.Call(uintptr(hArrow))
			return 1
		}

	case wmCloseCW:
		resetConflictState(windows.HWND(hwnd))
		return 0

	case wmDestroyCW:
		globalConflict.Lock()
		if globalConflict.fontTitle != 0 {
			procCWDeleteObject.Call(uintptr(globalConflict.fontTitle))
			globalConflict.fontTitle = 0
		}
		if globalConflict.fontSub != 0 {
			procCWDeleteObject.Call(uintptr(globalConflict.fontSub))
			globalConflict.fontSub = 0
		}
		if globalConflict.fontBtn != 0 {
			procCWDeleteObject.Call(uintptr(globalConflict.fontBtn))
			globalConflict.fontBtn = 0
		}
		globalConflict.overlayHWND = 0
		globalConflict.Unlock()
		procCWPostQuitMessage.Call(0)
		return 0
	}

	r, _, _ := procCWDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return r
}
