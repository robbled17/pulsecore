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

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
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
	procCWIsWindowVisible          = modUser32CW.NewProc("IsWindowVisible")
	procCWIsIconic                 = modUser32CW.NewProc("IsIconic")
	procCWGetWindowRect            = modUser32CW.NewProc("GetWindowRect")
	procCWGetClientRect            = modUser32CW.NewProc("GetClientRect")
	procCWSetWindowPos             = modUser32CW.NewProc("SetWindowPos")
	procCWShowWindow               = modUser32CW.NewProc("ShowWindow")
	procCWPostMessageW             = modUser32CW.NewProc("PostMessageW")
	procCWSendMessageW             = modUser32CW.NewProc("SendMessageW")
	procCWGetDlgCtrlID             = modUser32CW.NewProc("GetDlgCtrlID")
	procCWCreateWindowExW          = modUser32CW.NewProc("CreateWindowExW")
	procCWRegisterClassExW         = modUser32CW.NewProc("RegisterClassExW")
	procCWDefWindowProcW           = modUser32CW.NewProc("DefWindowProcW")
	procCWDestroyWindow            = modUser32CW.NewProc("DestroyWindow")
	procCWInvalidateRect           = modUser32CW.NewProc("InvalidateRect")
	procCWUpdateWindow             = modUser32CW.NewProc("UpdateWindow")
	procCWGetMessageW              = modUser32CW.NewProc("GetMessageW")
	procCWTranslateMessage         = modUser32CW.NewProc("TranslateMessage")
	procCWDispatchMessageW         = modUser32CW.NewProc("DispatchMessageW")
	procCWPostQuitMessage          = modUser32CW.NewProc("PostQuitMessage")
	procCWBeginPaint               = modUser32CW.NewProc("BeginPaint")
	procCWEndPaint                 = modUser32CW.NewProc("EndPaint")
	procCWDrawTextW                = modUser32CW.NewProc("DrawTextW")
	procCWFillRect                 = modUser32CW.NewProc("FillRect")
	procCWFrameRect                = modUser32CW.NewProc("FrameRect")
	procCWLoadCursorW              = modUser32CW.NewProc("LoadCursorW")
	procCWSetCursor                = modUser32CW.NewProc("SetCursor")
	procCWGetCursorPos             = modUser32CW.NewProc("GetCursorPos")
	procCWScreenToClient           = modUser32CW.NewProc("ScreenToClient")
	procCWGetForegroundWindow      = modUser32CW.NewProc("GetForegroundWindow")
	procCWGetWindowThreadProcId    = modUser32CW.NewProc("GetWindowThreadProcessId")
	procCWGetAncestor              = modUser32CW.NewProc("GetAncestor")
	procCWGetParent                = modUser32CW.NewProc("GetParent")

	modGdi32CW               = windows.NewLazySystemDLL("gdi32.dll")
	procCWCreateSolidBrush   = modGdi32CW.NewProc("CreateSolidBrush")
	procCWCreateRoundRectRgn = modGdi32CW.NewProc("CreateRoundRectRgn")
	procCWSetWindowRgn       = modUser32CW.NewProc("SetWindowRgn")
	procCWCreateFontW        = modGdi32CW.NewProc("CreateFontW")
	procCWSetBkMode          = modGdi32CW.NewProc("SetBkMode")
	procCWSetTextColor       = modGdi32CW.NewProc("SetTextColor")
	procCWSelectObject       = modGdi32CW.NewProc("SelectObject")
	procCWDeleteObject       = modGdi32CW.NewProc("DeleteObject")
	procCWRoundRect          = modGdi32CW.NewProc("RoundRect")
	procCWCreatePen          = modGdi32CW.NewProc("CreatePen")

	modKernel32CW                    = windows.NewLazySystemDLL("kernel32.dll")
	procCWGetModuleHandleW           = modKernel32CW.NewProc("GetModuleHandleW")
	procCWQueryFullProcessImageNameW = modKernel32CW.NewProc("QueryFullProcessImageNameW")
)

const (
	wsPopupCW          = 0x80000000
	wsExTopMostCW      = 0x00000008
	wsExToolWindowCW   = 0x00000080
	wsExNoActivateCW   = 0x08000000
	swpNoActivateCW    = 0x0010
	swpShowWindowCW    = 0x0040
	swShowNoActivateCW = 4
	swHideCW           = 0
	hwndTopMostCW      = ^uintptr(0) // -1
	bmClickCW          = 0x00F5
	wmCommandCW        = 0x0111
	wmPaintCW          = 0x000F
	wmLButtonUpCW      = 0x0202
	wmSetCursorCW      = 0x0020
	wmEraseBkgndCW     = 0x0014
	wmDestroyCW        = 0x0002
	wmCloseCW          = 0x0010

	wmAppCW          = 0x8000
	msgShowOverlayCW = wmAppCW + 1
	msgHideOverlayCW = wmAppCW + 2
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

// uintPtrToUnsafe converts a foreign OS memory address (Win32 / COM / MapViewOfFile) to unsafe.Pointer.
func uintPtrToUnsafe(p uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&p))
}

// callCOM invokes a virtual method from an interface pointer using stdcall conventions.
func callCOM(obj uintptr, methodIndex int, args ...uintptr) uintptr {
	if obj == 0 {
		return 1
	}
	vtablePtr := *(*uintptr)(uintPtrToUnsafe(obj))
	methodPtr := *(*uintptr)(unsafe.Add(uintPtrToUnsafe(vtablePtr), uintptr(methodIndex)*unsafe.Sizeof(uintptr(0))))
	allArgs := append([]uintptr{obj}, args...)
	r, _, _ := syscall.SyscallN(methodPtr, allArgs...)
	return r
}

// isExcludedConflictProcess returns true for processes that must never be considered conflict lockers or displayed in the overlay.
func isExcludedConflictProcess(procName string, pid uint32) bool {
	if pid > 0 && (pid <= 4 || pid == uint32(os.Getpid())) {
		return true
	}
	clean := strings.ToLower(filepath.Base(strings.TrimSpace(procName)))
	if clean == "" || clean == "pulsecore.exe" || clean == "pulsecore" || clean == "system" || clean == "idle" {
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
var destroyLingeringCb = syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
	var buf [256]uint16
	_, _, _ = procCWGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 256)
	if windows.UTF16ToString(buf[:]) == "PulseCoreConflictBar" {
		procCWDestroyWindow.Call(hwnd)
	}
	return 1
})

func DestroyLingeringConflictWindows() {
	procCWEnumWindows.Call(destroyLingeringCb, 0)
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
		func() {
			defer func() {
				if r := recover(); r != nil {
					logCrash("conflictWatcherLoop-tick", r)
				}
			}()

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
				return
			}

			checkActiveConflictDialogs()
		}()
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

	// Universal extraction: resolve item name and target path
	itemName, targetPath := resolveConflictItemAndPath(foundTitle, childTexts)

	var procs []LockProcessInfo
	// 3-Level universal search (Restart Manager -> Toolhelp Modules -> Handles/CWD)
	if targetPath != "" {
		procs, _ = FindLockingProcesses(targetPath)
	}

	// Filter out any excluded processes
	var validProcs []LockProcessInfo
	for _, p := range procs {
		pName := p.ProcessName
		if pName == "" {
			pName = p.AppName
		}
		if !isExcludedConflictProcess(pName, p.PID) && !isExcludedConflictProcess(p.AppName, p.PID) {
			validProcs = append(validProcs, p)
		}
	}
	procs = validProcs

	// If 3-level search didn't find anything by targetPath, fallback to matching by itemName
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

// extractTextsViaUIAutomation traverses the element tree of hwnd via COM IUIAutomation (TreeScope_Subtree).
// This reads text from modern DirectUIHWND and XAML controls where Win32 GetWindowText fails.
func extractTextsViaUIAutomation(hwnd windows.HWND) []string {
	if hwnd == 0 {
		return nil
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		if oleErr, ok := err.(*ole.OleError); !ok || (oleErr.Code() != ole.S_OK && oleErr.Code() != 1 && uintptr(oleErr.Code()) != 0x80010106) {
			return nil
		}
	}
	defer ole.CoUninitialize()

	clsidCUIAutomation := ole.NewGUID("{FF48DBA4-60EF-4201-AA87-54103EEF594E}")
	iidIUIAutomation := ole.NewGUID("{30CBE57D-D9D0-452A-AB13-7AC0E1029681}")

	unk, err := ole.CreateInstance(clsidCUIAutomation, iidIUIAutomation)
	if err != nil || unk == nil {
		return nil
	}
	uia := uintptr(unsafe.Pointer(unk))
	defer callCOM(uia, 2 /* Release */)

	var rootElem uintptr
	// ElementFromHandle = 6
	if callCOM(uia, 6, uintptr(hwnd), uintptr(unsafe.Pointer(&rootElem))) != 0 || rootElem == 0 {
		return nil
	}
	defer callCOM(rootElem, 2 /* Release */)

	var trueCond uintptr
	// CreateTrueCondition = 17
	if callCOM(uia, 17, uintptr(unsafe.Pointer(&trueCond))) != 0 || trueCond == 0 {
		return nil
	}
	defer callCOM(trueCond, 2 /* Release */)

	var elemArray uintptr
	// FindAll = 6 (TreeScope_Subtree = 7)
	if callCOM(rootElem, 6, 7, trueCond, uintptr(unsafe.Pointer(&elemArray))) != 0 || elemArray == 0 {
		return nil
	}
	defer callCOM(elemArray, 2 /* Release */)

	var length int32
	// get_Length = 3
	_ = callCOM(elemArray, 3, uintptr(unsafe.Pointer(&length)))
	if length <= 0 {
		return nil
	}
	if length > 256 {
		length = 256
	}

	var texts []string
	seen := map[string]bool{}

	for i := int32(0); i < length; i++ {
		var childElem uintptr
		// GetElement = 4
		if callCOM(elemArray, 4, uintptr(i), uintptr(unsafe.Pointer(&childElem))) == 0 && childElem != 0 {
			var bstr *uint16
			// GetCurrentName = 23
			if callCOM(childElem, 23, uintptr(unsafe.Pointer(&bstr))) == 0 && bstr != nil {
				txt := strings.TrimSpace(windows.UTF16PtrToString(bstr))
				ole.SysFreeString((*int16)(unsafe.Pointer(bstr)))
				if txt != "" && !seen[txt] {
					seen[txt] = true
					texts = append(texts, txt)
				}
			}
			callCOM(childElem, 2 /* Release */)
		}
	}

	return texts
}

var (
	childTextMu         sync.Mutex
	collectedChildTexts []string
	enumChildTextsCb    = syscall.NewCallback(func(ch uintptr, _ uintptr) uintptr {
		var chBuf [512]uint16
		_, _, _ = procCWGetWindowTextW.Call(ch, uintptr(unsafe.Pointer(&chBuf[0])), 512)
		txt := strings.TrimSpace(windows.UTF16ToString(chBuf[:]))
		if txt != "" {
			collectedChildTexts = append(collectedChildTexts, txt)
		}
		return 1
	})
)

func getChildWindowTexts(parent uintptr) []string {
	childTextMu.Lock()
	defer childTextMu.Unlock()
	collectedChildTexts = collectedChildTexts[:0]
	procCWEnumChildWindows.Call(parent, enumChildTextsCb, 0)
	out := make([]string, len(collectedChildTexts))
	copy(out, collectedChildTexts)
	return out
}

var (
	conflictEnumMu     sync.Mutex
	conflictFoundHWND  windows.HWND
	conflictFoundTitle string
	conflictFoundTexts []string
	enumConflictCb     = syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
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

		// 1. Windows UI Automation (reads DirectUIHWND and XAML controls)
		uiaTexts := extractTextsViaUIAutomation(windows.HWND(hwnd))

		// 2. Standard Win32 child windows
		children := getChildWindowTexts(hwnd)

		// Merge texts without duplicates
		seen := map[string]bool{}
		var merged []string
		for _, t := range append(uiaTexts, children...) {
			if !seen[t] {
				seen[t] = true
				merged = append(merged, t)
			}
		}

		fullContent := strings.ToLower(title + " " + strings.Join(merged, " "))
		isConflict := strings.Contains(fullContent, "уже используется") ||
			strings.Contains(fullContent, "используется") ||
			strings.Contains(fullContent, "in use") ||
			strings.Contains(fullContent, "действие не может быть выполнено") ||
			strings.Contains(fullContent, "action can't be completed")

		if isConflict {
			conflictFoundHWND = windows.HWND(hwnd)
			conflictFoundTitle = title
			conflictFoundTexts = merged
			return 0
		}
		return 1
	})
)

func findConflictWindow() (windows.HWND, string, []string) {
	conflictEnumMu.Lock()
	defer conflictEnumMu.Unlock()
	conflictFoundHWND = 0
	conflictFoundTitle = ""
	conflictFoundTexts = nil
	procCWEnumWindows.Call(enumConflictCb, 0)
	return conflictFoundHWND, conflictFoundTitle, conflictFoundTexts
}

var (
	explorerStateMu       sync.Mutex
	cachedSelectedPaths   []string
	cachedOpenFolders     []string
	lastExplorerStateTime time.Time
)

// getExplorerShellState queries open Explorer windows directly in-process via COM Shell.Application.
func getExplorerShellState() (selectedPaths []string, openFolders []string) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		if oleErr, ok := err.(*ole.OleError); !ok || (oleErr.Code() != ole.S_OK && oleErr.Code() != 1 && uintptr(oleErr.Code()) != 0x80010106) {
			return nil, nil
		}
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("Shell.Application")
	if err != nil {
		return nil, nil
	}
	defer unknown.Release()

	shell, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, nil
	}
	defer shell.Release()

	windowsDisp, err := oleutil.CallMethod(shell, "Windows")
	if err != nil || windowsDisp == nil {
		return nil, nil
	}
	windowsObj := windowsDisp.ToIDispatch()
	defer windowsObj.Release()

	countVar, err := oleutil.GetProperty(windowsObj, "Count")
	if err != nil {
		return nil, nil
	}
	count := int(countVar.Val)

	for i := 0; i < count; i++ {
		wDisp, err := oleutil.CallMethod(windowsObj, "Item", i)
		if err != nil || wDisp == nil {
			continue
		}
		wObj := wDisp.ToIDispatch()

		// 1. LocationURL
		if locVar, err := oleutil.GetProperty(wObj, "LocationURL"); err == nil {
			loc := locVar.ToString()
			if strings.HasPrefix(loc, "file:///") {
				p := strings.TrimPrefix(loc, "file:///")
				p = filepath.FromSlash(strings.ReplaceAll(p, "%20", " "))
				if len(p) >= 3 && p[1] == ':' {
					openFolders = append(openFolders, filepath.Clean(p))
				}
			}
		}

		// 2. Document
		if docDisp, err := oleutil.GetProperty(wObj, "Document"); err == nil && docDisp != nil && docDisp.Val != 0 {
			docObj := docDisp.ToIDispatch()
			if folderDisp, err := oleutil.GetProperty(docObj, "Folder"); err == nil && folderDisp != nil && folderDisp.Val != 0 {
				folderObj := folderDisp.ToIDispatch()
				if selfDisp, err := oleutil.GetProperty(folderObj, "Self"); err == nil && selfDisp != nil && selfDisp.Val != 0 {
					selfObj := selfDisp.ToIDispatch()
					if pathVar, err := oleutil.GetProperty(selfObj, "Path"); err == nil {
						p := pathVar.ToString()
						if len(p) >= 3 && p[1] == ':' {
							openFolders = append(openFolders, filepath.Clean(p))
						}
					}
					selfObj.Release()
				}
				folderObj.Release()
			}

			if selDisp, err := oleutil.CallMethod(docObj, "SelectedItems"); err == nil && selDisp != nil && selDisp.Val != 0 {
				selObj := selDisp.ToIDispatch()
				if selCountVar, err := oleutil.GetProperty(selObj, "Count"); err == nil {
					selCount := int(selCountVar.Val)
					for j := 0; j < selCount; j++ {
						if itemDisp, err := oleutil.CallMethod(selObj, "Item", j); err == nil && itemDisp != nil && itemDisp.Val != 0 {
							itemObj := itemDisp.ToIDispatch()
							if pVar, err := oleutil.GetProperty(itemObj, "Path"); err == nil {
								p := pVar.ToString()
								if len(p) >= 3 && p[1] == ':' {
									selectedPaths = append(selectedPaths, filepath.Clean(p))
								}
							}
							itemObj.Release()
						}
					}
				}
				selObj.Release()
			}
			docObj.Release()
		}
		wObj.Release()
	}

	return selectedPaths, openFolders
}

func getExplorerPowershellState() (selectedPaths []string, openFolders []string) {
	cmd := `[Console]::OutputEncoding=[Text.Encoding]::UTF8; $shell = New-Object -ComObject Shell.Application; $shell.Windows() | ForEach-Object { try { $_.Document.SelectedItems() | ForEach-Object { "SEL|" + $_.Path } } catch {}; try { $p = $_.Document.Folder.Self.Path; if ($p) { "DIR|" + $p } } catch {} }`
	raw, err := runHiddenOutput("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", cmd)
	if err != nil {
		return nil, nil
	}

	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "SEL|") {
			p := strings.TrimPrefix(l, "SEL|")
			if len(p) >= 3 && p[1] == ':' {
				selectedPaths = append(selectedPaths, filepath.Clean(p))
			}
		} else if strings.HasPrefix(l, "DIR|") {
			p := strings.TrimPrefix(l, "DIR|")
			if len(p) >= 3 && p[1] == ':' {
				openFolders = append(openFolders, filepath.Clean(p))
			}
		}
	}
	return selectedPaths, openFolders
}

func getExplorerState() ([]string, []string) {
	explorerStateMu.Lock()
	defer explorerStateMu.Unlock()

	if time.Since(lastExplorerStateTime) < 500*time.Millisecond && (len(cachedSelectedPaths) > 0 || len(cachedOpenFolders) > 0) {
		return cachedSelectedPaths, cachedOpenFolders
	}

	sel, dirs := getExplorerShellState()
	if len(sel) == 0 && len(dirs) == 0 {
		sel, dirs = getExplorerPowershellState()
	}

	cachedSelectedPaths = sel
	cachedOpenFolders = dirs
	lastExplorerStateTime = time.Now()
	return sel, dirs
}

// resolveConflictItemAndPath determines the target name and full path of the locked item.
func resolveConflictItemAndPath(title string, children []string) (string, string) {
	selectedPaths, openFolders := getExplorerState()

	// 1. If Explorer has a selected item, check if its name matches dialog texts
	if len(selectedPaths) > 0 {
		for _, sel := range selectedPaths {
			base := filepath.Base(sel)
			if !isExcludedConflictProcess(base, 0) {
				for _, t := range append([]string{title}, children...) {
					if strings.Contains(strings.ToLower(t), strings.ToLower(base)) {
						return base, sel
					}
				}
			}
		}
		// If only one item is selected, use it directly
		for _, sel := range selectedPaths {
			base := filepath.Base(sel)
			if !isExcludedConflictProcess(base, 0) {
				return base, sel
			}
		}
	}

	// 2. Direct path in title or children
	for _, t := range append([]string{title}, children...) {
		t = strings.TrimSpace(t)
		if len(t) >= 3 && t[1] == ':' {
			if _, err := os.Stat(t); err == nil {
				return filepath.Base(t), t
			}
		}
	}

	// 3. Extract candidate item name from dialog text
	itemName := extractConflictItemName(title, children)

	// 4. Match candidate item against open Explorer directories
	if itemName != "" && itemName != "выбранный объект" {
		for _, folder := range openFolders {
			if strings.EqualFold(filepath.Base(folder), itemName) {
				if _, err := os.Stat(folder); err == nil {
					return itemName, folder
				}
			}
			candidate := filepath.Join(folder, itemName)
			if _, err := os.Stat(candidate); err == nil {
				return itemName, candidate
			}
		}
	}

	return itemName, ""
}

func extractConflictItemName(title string, children []string) string {
	// 1. Look for quoted names like "Antigravity Tools" or «Antigravity Tools»
	for _, t := range append([]string{title}, children...) {
		t = strings.TrimSpace(t)
		if start := strings.IndexAny(t, `"'«`); start >= 0 {
			rest := t[start+1:]
			if end := strings.IndexAny(rest, `"'»`); end > 0 {
				candidate := strings.TrimSpace(rest[:end])
				if candidate != "" && len(candidate) < 100 && !strings.Contains(candidate, "\n") && !isExcludedConflictProcess(candidate, 0) {
					return candidate
				}
			}
		}
	}

	// 2. Look for text following "повторите попытку" or "try again"
	for _, t := range append([]string{title}, children...) {
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
	return "выбранный объект"
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

var (
	pidWinMu         sync.Mutex
	pidWinMap        map[uint32][]string
	enumPidWindowsCb = syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if rv, _, _ := procCWIsWindowVisible.Call(hwnd); rv == 0 {
			return 1
		}
		var pid uint32
		procCWGetWindowThreadProcId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid > 4 {
			var buf [512]uint16
			procCWGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 512)
			txt := strings.TrimSpace(windows.UTF16ToString(buf[:]))
			if txt != "" && pidWinMap != nil {
				pidWinMap[pid] = append(pidWinMap[pid], txt)
			}
		}
		return 1
	})
)

func findProcessesByNameOrPath(name, dirPath string) []LockProcessInfo {
	pidWinMu.Lock()
	pidWinMap = make(map[uint32][]string)
	procCWEnumWindows.Call(enumPidWindowsCb, 0)
	pidWindows := pidWinMap
	pidWinMap = nil
	pidWinMu.Unlock()

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
		cleanItem = "выбранный объект"
	}

	var validProcs []LockProcessInfo
	for _, p := range procs {
		pName := p.ProcessName
		if pName == "" {
			pName = p.AppName
		}
		if !isExcludedConflictProcess(pName, p.PID) && !isExcludedConflictProcess(p.AppName, p.PID) {
			validProcs = append(validProcs, p)
		}
	}

	subText = fmt.Sprintf("Объект: %s", cleanItem)

	if len(validProcs) == 0 {
		titleText = "🔒 Держит: Процесс не определён"
	} else if len(validProcs) == 1 {
		pName := validProcs[0].ProcessName
		if pName == "" {
			pName = validProcs[0].AppName
		}
		titleText = fmt.Sprintf("🔒 Держит: %s", pName)
	} else {
		mainProc := validProcs[0].ProcessName
		if mainProc == "" {
			mainProc = validProcs[0].AppName
		}
		titleText = fmt.Sprintf("🔒 Держит: %s (+%d)", mainProc, len(validProcs)-1)
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

	uiaTexts := extractTextsViaUIAutomation(hwnd)
	children := getChildWindowTexts(uintptr(hwnd))

	fullContent := strings.ToLower(title + " " + strings.Join(append(uiaTexts, children...), " "))
	return strings.Contains(fullContent, "уже используется") ||
		strings.Contains(fullContent, "используется") ||
		strings.Contains(fullContent, "in use") ||
		strings.Contains(fullContent, "действие не может быть выполнено") ||
		strings.Contains(fullContent, "action can't be completed")
}

var (
	retryBtnMu     sync.Mutex
	foundRetryBtn  windows.HWND
	enumRetryBtnCb = syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		ctrlID, _, _ := procCWGetDlgCtrlID.Call(hwnd)
		if ctrlID == 4 { // IDRETRY = 4
			foundRetryBtn = windows.HWND(hwnd)
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
				foundRetryBtn = windows.HWND(hwnd)
				return 0
			}
		}
		return 1
	})
)

func findRetryButton(targetHWND windows.HWND) windows.HWND {
	retryBtnMu.Lock()
	defer retryBtnMu.Unlock()
	foundRetryBtn = 0
	procCWEnumChildWindows.Call(uintptr(targetHWND), enumRetryBtnCb, 0)
	return foundRetryBtn
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
		return 0
	}

	r, _, _ := procCWDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return r
}
