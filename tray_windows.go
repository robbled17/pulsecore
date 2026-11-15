//go:build windows

package main

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

//go:embed build/windows/icon.ico
var defaultIconBytes []byte

type HICON uintptr

type NOTIFYICONDATAW struct {
	cbSize            uint32
	_                 uint32 // 64-bit alignment padding
	hWnd              windows.HWND
	uID               uint32
	uFlags            uint32
	uCallbackMessage  uint32
	_                 uint32 // 64-bit alignment padding
	hIcon             HICON
	szTip             [128]uint16
	dwState           uint32
	dwStateMask       uint32
	szInfo            [256]uint16
	uTimeoutOrVersion uint32
	szInfoTitle       [64]uint16
	dwInfoFlags       uint32
	guidItem          windows.GUID
	hBalloonIcon      HICON
}

type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     windows.Handle
	hIcon         HICON
	hCursor       windows.Handle
	hbrBackground windows.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       HICON
}

type POINT struct {
	X int32
	Y int32
}

type MSG struct {
	HWnd    windows.HWND
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

const (
	NIM_ADD        = 0x00000000
	NIM_MODIFY     = 0x00000001
	NIM_DELETE     = 0x00000002
	NIM_SETFOCUS   = 0x00000003
	NIM_SETVERSION = 0x00000004

	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004
	NIF_STATE   = 0x00000008
	NIF_INFO    = 0x00000010
	NIF_GUID    = 0x00000020

	WM_DESTROY       = 0x0002
	WM_CLOSE         = 0x0010
	WM_COMMAND       = 0x0111
	WM_LBUTTONUP     = 0x0202
	WM_RBUTTONUP     = 0x0205
	WM_LBUTTONDBLCLK = 0x0203
	WM_CONTEXTMENU   = 0x007B
	WM_NULL          = 0x0000
	WM_USER          = 0x0400
	WM_TRAYICON      = WM_USER + 1024

	TPM_RIGHTBUTTON = 0x0002
	TPM_BOTTOMALIGN = 0x0020

	MF_STRING    = 0x0000
	MF_SEPARATOR = 0x0800
	MF_POPUP     = 0x0010

	trayMenuOpen     = 1001
	trayMenuHUD      = 1002
	trayMenuSilent   = 1003
	trayMenuBalanced = 1004
	trayMenuTurbo    = 1005
	trayMenuQuit     = 1006
)

var (
	modShell32Win            = windows.NewLazySystemDLL("shell32.dll")
	procShellNotifyIconW     = modShell32Win.NewProc("Shell_NotifyIconW")

	modUser32Win             = windows.NewLazySystemDLL("user32.dll")
	procCreateIconFromRes    = modUser32Win.NewProc("CreateIconFromResourceEx")
	procDestroyIconWin       = modUser32Win.NewProc("DestroyIcon")
	procRegisterClassExWin   = modUser32Win.NewProc("RegisterClassExW")
	procCreateWindowExWin    = modUser32Win.NewProc("CreateWindowExW")
	procDefWindowProcWin     = modUser32Win.NewProc("DefWindowProcW")
	procDestroyWindowWin     = modUser32Win.NewProc("DestroyWindow")
	procGetMessageWin        = modUser32Win.NewProc("GetMessageW")
	procTranslateMessageWin  = modUser32Win.NewProc("TranslateMessage")
	procDispatchMessageWin   = modUser32Win.NewProc("DispatchMessageW")
	procPostQuitMessageWin   = modUser32Win.NewProc("PostQuitMessage")
	procPostMessageWin       = modUser32Win.NewProc("PostMessageW")
	procCreatePopupMenuWin   = modUser32Win.NewProc("CreatePopupMenu")
	procAppendMenuWin        = modUser32Win.NewProc("AppendMenuW")
	procTrackPopupMenuWin    = modUser32Win.NewProc("TrackPopupMenu")
	procDestroyMenuWin       = modUser32Win.NewProc("DestroyMenu")
	procSetForegroundWin     = modUser32Win.NewProc("SetForegroundWindow")
	procGetCursorPosWin      = modUser32Win.NewProc("GetCursorPos")
	procRegisterWindowMsgWin = modUser32Win.NewProc("RegisterWindowMessageW")
	procLoadCursorWin        = modUser32Win.NewProc("LoadCursorW")

	modKernel32Win           = windows.NewLazySystemDLL("kernel32.dll")
	procGetModuleHandleWin   = modKernel32Win.NewProc("GetModuleHandleW")
)

var (
	globalTrayManager   *trayManager
	globalTrayManagerMu sync.RWMutex
)

func getGlobalTrayManager() *trayManager {
	globalTrayManagerMu.RLock()
	defer globalTrayManagerMu.RUnlock()
	return globalTrayManager
}

func setGlobalTrayManager(tm *trayManager) {
	globalTrayManagerMu.Lock()
	globalTrayManager = tm
	globalTrayManagerMu.Unlock()
}

type shellNotifyFunc func(dwMessage uint32, lpData *NOTIFYICONDATAW) uintptr
type destroyIconFunc func(hIcon HICON) uintptr

var defaultShellNotifyIcon = func(dwMessage uint32, lpData *NOTIFYICONDATAW) uintptr {
	r, _, _ := procShellNotifyIconW.Call(uintptr(dwMessage), uintptr(unsafe.Pointer(lpData)))
	return r
}

var defaultDestroyIcon = func(hIcon HICON) uintptr {
	r, _, _ := procDestroyIconWin.Call(uintptr(hIcon))
	return r
}

type trayManager struct {
	app *App

	mu          sync.Mutex
	initialized bool
	hWnd        windows.HWND
	uID         uint32

	defaultHIcon HICON
	currentHIcon HICON
	isDynamic    bool
	lastTooltip  string

	taskbarCreatedMsg uint32

	stopUpdate chan struct{}
	closeOnce  sync.Once
	updateOnce sync.Once
	readyChan  chan struct{}

	notifyIcon  shellNotifyFunc
	destroyIcon destroyIconFunc
}

func newTrayManager(a *App) *trayManager {
	return &trayManager{
		app:         a,
		uID:         1,
		stopUpdate:  make(chan struct{}),
		readyChan:   make(chan struct{}),
		notifyIcon:  defaultShellNotifyIcon,
		destroyIcon: defaultDestroyIcon,
	}
}

func (a *App) startTray() {
	if a.tray != nil {
		return
	}
	tm := newTrayManager(a)
	a.tray = tm
	setGlobalTrayManager(tm)

	go tm.runWindowThread()

	go func() {
		select {
		case <-tm.readyChan:
		case <-time.After(5 * time.Second):
		}
		tm.updateOnce.Do(func() {
			go tm.updateLoop()
		})
		tm.refreshNow()
	}()
}

func (a *App) stopTray() {
	if a.tray != nil {
		a.tray.stop()
		setGlobalTrayManager(nil)
		a.tray = nil
	}
}

// UpdateTrayIconNow immediately refreshes the system tray icon and tooltip.
func (a *App) UpdateTrayIconNow() {
	if a.tray != nil {
		a.tray.refreshNow()
	}
}

func (t *trayManager) stop() {
	t.closeOnce.Do(func() {
		if t.stopUpdate != nil {
			close(t.stopUpdate)
		}
		t.mu.Lock()
		t.destroyTrayLocked()
		hWnd := t.hWnd
		t.hWnd = 0
		t.mu.Unlock()

		if hWnd != 0 {
			procPostMessageWin.Call(uintptr(hWnd), WM_CLOSE, 0, 0)
		}
	})
}

func (t *trayManager) runWindowThread() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hInstRaw, _, _ := procGetModuleHandleWin.Call(0)
	hInst := windows.Handle(hInstRaw)

	className, _ := windows.UTF16PtrFromString("PulseCoreTrayHelperClass")
	wndProcCb := syscall.NewCallback(trayWndProc)

	hCursorArrow, _, _ := procLoadCursorWin.Call(0, 32512 /* IDC_ARROW */)

	var wc wndClassExW
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	wc.style = 0
	wc.lpfnWndProc = wndProcCb
	wc.hInstance = hInst
	wc.hCursor = windows.Handle(hCursorArrow)
	wc.lpszClassName = className

	procRegisterClassExWin.Call(uintptr(unsafe.Pointer(&wc)))

	wndTitle, _ := windows.UTF16PtrFromString("PulseCore Tray Window")
	hwnd, _, _ := procCreateWindowExWin.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(wndTitle)),
		0x80000000, // WS_POPUP
		0, 0, 0, 0,
		0,
		0,
		uintptr(hInst),
		0,
	)

	if hwnd == 0 {
		close(t.readyChan)
		return
	}

	taskbarStr, _ := windows.UTF16PtrFromString("TaskbarCreated")
	tMsg, _, _ := procRegisterWindowMsgWin.Call(uintptr(unsafe.Pointer(taskbarStr)))

	t.mu.Lock()
	t.hWnd = windows.HWND(hwnd)
	t.taskbarCreatedMsg = uint32(tMsg)

	// Load default brand icon (16x16)
	if hDef, err := createHIconFromICO(defaultIconBytes, 16); err == nil && hDef != 0 {
		t.defaultHIcon = hDef
	} else {
		// Fallback to pulse glyph ICO
		fallbackICO := trayIconICO(0, 56, 189, 248)
		if hFallback, err2 := createHIconFromICO(fallbackICO, 16); err2 == nil && hFallback != 0 {
			t.defaultHIcon = hFallback
		}
	}

	_ = t.initTrayLocked()
	t.mu.Unlock()

	close(t.readyChan)

	var msg MSG
	for {
		r, _, _ := procGetMessageWin.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if r == 0 || int32(r) == -1 {
			break
		}
		procTranslateMessageWin.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageWin.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func trayWndProc(hWnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	tm := getGlobalTrayManager()
	if tm == nil {
		r, _, _ := procDefWindowProcWin.Call(hWnd, uintptr(msg), wParam, lParam)
		return r
	}

	if msg == tm.taskbarCreatedMsg && tm.taskbarCreatedMsg != 0 {
		tm.reAddTrayIcon()
		return 0
	}

	switch msg {
	case WM_TRAYICON:
		switch lParam {
		case WM_LBUTTONUP, WM_LBUTTONDBLCLK:
			if tm.app != nil {
				go tm.app.ShowWindow()
			}
			return 0
		case WM_RBUTTONUP, WM_CONTEXTMENU:
			tm.showContextMenu(windows.HWND(hWnd))
			return 0
		}
		return 0

	case WM_COMMAND:
		id := uint32(wParam & 0xFFFF)
		tm.handleMenuCommand(id)
		return 0

	case WM_CLOSE:
		procDestroyWindowWin.Call(hWnd)
		return 0

	case WM_DESTROY:
		procPostQuitMessageWin.Call(0)
		return 0
	}

	r, _, _ := procDefWindowProcWin.Call(hWnd, uintptr(msg), wParam, lParam)
	return r
}

func (t *trayManager) showContextMenu(hWnd windows.HWND) {
	if t.app == nil {
		return
	}
	lang := t.app.getLang()

	var pt POINT
	procGetCursorPosWin.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWin.Call(uintptr(hWnd))

	hMenu, _, _ := procCreatePopupMenuWin.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenuWin.Call(hMenu)

	hPower, _, _ := procCreatePopupMenuWin.Call()
	procAppendMenuWin.Call(hPower, MF_STRING, trayMenuSilent, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(trayT(lang, "silent")))))
	procAppendMenuWin.Call(hPower, MF_STRING, trayMenuBalanced, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(trayT(lang, "balanced")))))
	procAppendMenuWin.Call(hPower, MF_STRING, trayMenuTurbo, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(trayT(lang, "turbo")))))

	procAppendMenuWin.Call(hMenu, MF_STRING, trayMenuOpen, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(trayT(lang, "open")))))
	procAppendMenuWin.Call(hMenu, MF_STRING, trayMenuHUD, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(trayT(lang, "hud")))))
	procAppendMenuWin.Call(hMenu, MF_SEPARATOR, 0, 0)
	procAppendMenuWin.Call(hMenu, MF_POPUP, hPower, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(trayT(lang, "power")))))
	procAppendMenuWin.Call(hMenu, MF_SEPARATOR, 0, 0)
	procAppendMenuWin.Call(hMenu, MF_STRING, trayMenuQuit, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(trayT(lang, "quit")))))

	procTrackPopupMenuWin.Call(hMenu, TPM_RIGHTBUTTON|TPM_BOTTOMALIGN, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(hWnd), 0)
	procPostMessageWin.Call(uintptr(hWnd), WM_NULL, 0, 0)
}

func (t *trayManager) handleMenuCommand(id uint32) {
	if t.app == nil {
		return
	}
	switch id {
	case trayMenuOpen:
		go t.app.ShowWindow()
	case trayMenuHUD:
		go func() {
			t.app.mu.RLock()
			on := t.app.miniHUD
			t.app.mu.RUnlock()
			t.app.ToggleMiniHUD(!on)
		}()
	case trayMenuSilent:
		go func() { _ = t.app.SetPowerProfile("silent") }()
	case trayMenuBalanced:
		go func() { _ = t.app.SetPowerProfile("balanced") }()
	case trayMenuTurbo:
		go func() { _ = t.app.SetPowerProfile("turbo") }()
	case trayMenuQuit:
		go t.app.QuitApp()
	}
}

func (t *trayManager) reAddTrayIcon() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.hWnd == 0 {
		return
	}
	var nid NOTIFYICONDATAW
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	nid.hWnd = t.hWnd
	nid.uID = t.uID
	nid.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP
	nid.uCallbackMessage = WM_TRAYICON
	nid.hIcon = t.currentHIcon
	setTip(&nid, t.lastTooltip)

	t.notifyIcon(NIM_ADD, &nid)
	t.initialized = true
}

func (t *trayManager) initTrayLocked() error {
	if t.initialized {
		return nil
	}
	if t.hWnd == 0 {
		return fmt.Errorf("tray window not ready")
	}

	var nid NOTIFYICONDATAW
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	nid.hWnd = t.hWnd
	nid.uID = t.uID
	nid.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP
	nid.uCallbackMessage = WM_TRAYICON
	nid.hIcon = t.defaultHIcon
	setTip(&nid, "PulseCore")

	ret := t.notifyIcon(NIM_ADD, &nid)
	if ret == 0 {
		return fmt.Errorf("Shell_NotifyIconW(NIM_ADD) failed")
	}

	t.initialized = true
	t.currentHIcon = t.defaultHIcon
	t.isDynamic = false
	t.lastTooltip = "PulseCore"
	return nil
}

func (t *trayManager) destroyTrayLocked() {
	if !t.initialized {
		return
	}
	var nid NOTIFYICONDATAW
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	nid.hWnd = t.hWnd
	nid.uID = t.uID
	t.notifyIcon(NIM_DELETE, &nid)
	t.initialized = false

	if t.isDynamic && t.currentHIcon != 0 && t.currentHIcon != t.defaultHIcon {
		t.destroyIcon(t.currentHIcon)
		t.currentHIcon = 0
	}
	if t.defaultHIcon != 0 {
		t.destroyIcon(t.defaultHIcon)
		t.defaultHIcon = 0
	}
}

func (t *trayManager) updateLoop() {
	for {
		wait := 3 * time.Second
		t.app.mu.RLock()
		hidden := t.app.hidden
		cpu := t.app.metrics.CPUPercent
		t.app.mu.RUnlock()
		if hidden {
			wait = 8 * time.Second
			if cpu < 25 {
				wait = 10 * time.Second
			}
		}
		select {
		case <-t.stopUpdate:
			return
		case <-time.After(wait):
			t.refreshNow()
		}
	}
}

func (t *trayManager) refreshNow() {
	if t.app == nil {
		return
	}
	t.app.mu.RLock()
	m := t.app.metrics
	prefs := t.app.prefs
	t.app.mu.RUnlock()

	showTemp := prefs.ShowTempOnTray || prefs.ShowTrayTemp

	var newHIcon HICON
	var isDynamic bool
	var tip string

	if showTemp {
		temp := m.TemperatureC
		val, r, g, b, hasTemp := trayColorForTemp(temp, m.CPUPercent)
		icoBytes := trayIconICO(val, r, g, b)
		hIcon, err := createHIconFromICO(icoBytes, 16)
		if err == nil && hIcon != 0 {
			newHIcon = hIcon
			isDynamic = true
		} else {
			newHIcon = t.defaultHIcon
			isDynamic = false
		}
		if hasTemp && temp > 0 {
			tip = fmt.Sprintf("PulseCore · CPU %.0f°C", temp)
		} else {
			tip = fmt.Sprintf("PulseCore · CPU %.0f%%", m.CPUPercent)
		}
	} else {
		newHIcon = t.defaultHIcon
		isDynamic = false
		tip = "PulseCore"
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.initialized || t.hWnd == 0 {
		if isDynamic && newHIcon != 0 && newHIcon != t.defaultHIcon {
			t.destroyIcon(newHIcon)
		}
		return
	}

	sameIcon := (t.currentHIcon == newHIcon)
	sameTip := (t.lastTooltip == tip)

	if sameIcon && sameTip {
		if isDynamic && newHIcon != 0 && newHIcon != t.currentHIcon && newHIcon != t.defaultHIcon {
			t.destroyIcon(newHIcon)
		}
		return
	}

	var nid NOTIFYICONDATAW
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	nid.hWnd = t.hWnd
	nid.uID = t.uID
	nid.uFlags = NIF_ICON | NIF_TIP
	nid.hIcon = newHIcon
	setTip(&nid, tip)

	t.notifyIcon(NIM_MODIFY, &nid)

	oldHIcon := t.currentHIcon
	wasDynamic := t.isDynamic

	t.currentHIcon = newHIcon
	t.isDynamic = isDynamic
	t.lastTooltip = tip

	// Strictly destroy previous dynamic icon immediately after NIM_MODIFY to prevent GDI leak
	if wasDynamic && oldHIcon != 0 && oldHIcon != t.defaultHIcon && oldHIcon != newHIcon {
		t.destroyIcon(oldHIcon)
	}
}

func setTip(nid *NOTIFYICONDATAW, tip string) {
	u16, err := windows.UTF16FromString(tip)
	if err != nil {
		return
	}
	n := copy(nid.szTip[:len(nid.szTip)-1], u16)
	nid.szTip[n] = 0
}

func createHIconFromICO(ico []byte, desiredSize int) (HICON, error) {
	if len(ico) < 22 {
		return 0, fmt.Errorf("ico too short: %d", len(ico))
	}
	count := int(binary.LittleEndian.Uint16(ico[4:6]))
	if count == 0 {
		return 0, fmt.Errorf("no icons in file")
	}

	bestOffset := 0
	bestSize := 0
	bestDiff := 9999

	for i := 0; i < count; i++ {
		entryOff := 6 + i*16
		if entryOff+16 > len(ico) {
			break
		}
		w := int(ico[entryOff])
		if w == 0 {
			w = 256
		}
		diff := w - desiredSize
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			bestDiff = diff
			bestSize = int(binary.LittleEndian.Uint32(ico[entryOff+8 : entryOff+12]))
			bestOffset = int(binary.LittleEndian.Uint32(ico[entryOff+12 : entryOff+16]))
		}
	}

	if bestOffset == 0 || bestOffset+bestSize > len(ico) {
		return 0, fmt.Errorf("invalid icon offset")
	}

	rawBits := ico[bestOffset : bestOffset+bestSize]
	r, _, err := procCreateIconFromRes.Call(
		uintptr(unsafe.Pointer(&rawBits[0])),
		uintptr(len(rawBits)),
		1, // TRUE (icon)
		0x00030000,
		uintptr(desiredSize),
		uintptr(desiredSize),
		0, // LR_DEFAULTCOLOR
	)
	if r == 0 {
		return 0, fmt.Errorf("CreateIconFromResourceEx failed: %v", err)
	}
	return HICON(r), nil
}
