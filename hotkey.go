package main

import (
	"fmt"
	goruntime "runtime"
	"strings"
	"sync"
	"unsafe"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

const (
	defaultMiniHUDHotkey = "Ctrl+Shift+M"
	hotkeyIDMiniHUD      = 0x5043 // 'PC'
	modAlt               = 0x0001
	modControl           = 0x0002
	modShift             = 0x0004
	modWin               = 0x0008
	modNoRepeat          = 0x4000
	wmHotkey             = 0x0312
	wmDestroy            = 0x0002
	wmClose              = 0x0010
	wmQuit               = 0x0012
)

var (
	procRegisterHotKey   = modUser32.NewProc("RegisterHotKey")
	procUnregisterHotKey = modUser32.NewProc("UnregisterHotKey")
	procCreateWindowExW  = modUser32.NewProc("CreateWindowExW")
	procDestroyWindow    = modUser32.NewProc("DestroyWindow")
	procGetMessageW      = modUser32.NewProc("GetMessageW")
	procTranslateMessage = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW = modUser32.NewProc("DispatchMessageW")
	procPostThreadMsg    = modUser32.NewProc("PostThreadMessageW")
	procDefWindowProcW   = modUser32.NewProc("DefWindowProcW")
	procRegisterClassExW = modUser32.NewProc("RegisterClassExW")
	procGetModuleHandleW = modKernel32.NewProc("GetModuleHandleW")

	hotkeyMu       sync.Mutex
	hotkeyChord    = defaultMiniHUDHotkey
	hotkeyMods     uint32
	hotkeyVK       uint32
	hotkeyActive   bool
	hotkeyThreadID uint32
	hotkeyHWND     uintptr
	hotkeyApp      *App
	hotkeyClassOnce sync.Once
)

type msgStruct struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// ParsedHotkey is a decoded global hotkey chord.
type ParsedHotkey struct {
	Mods   uint32
	VK     uint32
	Chord  string
	Valid  bool
	Reason string
}

// ParseHotkeyChord parses strings like "Ctrl+Shift+M" into Win32 mods + VK.
// Fail-soft: invalid input returns Valid=false with Reason (never panics).
func ParseHotkeyChord(raw string) ParsedHotkey {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ParsedHotkey{Reason: "empty"}
	}
	parts := strings.Split(s, "+")
	var mods uint32
	var key string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		low := strings.ToLower(p)
		switch low {
		case "ctrl", "control", "ctl":
			mods |= modControl
		case "shift":
			mods |= modShift
		case "alt", "menu":
			mods |= modAlt
		case "win", "meta", "cmd", "super":
			mods |= modWin
		default:
			if key != "" {
				return ParsedHotkey{Reason: "multiple keys"}
			}
			key = p
		}
	}
	if key == "" {
		return ParsedHotkey{Reason: "missing key"}
	}
	if mods == 0 {
		return ParsedHotkey{Reason: "modifier required"}
	}
	vk, ok := hotkeyKeyToVK(key)
	if !ok {
		return ParsedHotkey{Reason: "unsupported key"}
	}
	norm := normalizeHotkeyChord(mods, key)
	return ParsedHotkey{Mods: mods, VK: vk, Chord: norm, Valid: true}
}

func hotkeyKeyToVK(key string) (uint32, bool) {
	k := strings.ToUpper(strings.TrimSpace(key))
	if len(k) == 1 {
		ch := k[0]
		if ch >= 'A' && ch <= 'Z' {
			return uint32(ch), true
		}
		if ch >= '0' && ch <= '9' {
			return uint32(ch), true
		}
	}
	switch k {
	case "F1":
		return 0x70, true
	case "F2":
		return 0x71, true
	case "F3":
		return 0x72, true
	case "F4":
		return 0x73, true
	case "F5":
		return 0x74, true
	case "F6":
		return 0x75, true
	case "F7":
		return 0x76, true
	case "F8":
		return 0x77, true
	case "F9":
		return 0x78, true
	case "F10":
		return 0x79, true
	case "F11":
		return 0x7A, true
	case "F12":
		return 0x7B, true
	case "SPACE":
		return 0x20, true
	case "TAB":
		return 0x09, true
	case "INSERT", "INS":
		return 0x2D, true
	case "DELETE", "DEL":
		return 0x2E, true
	case "HOME":
		return 0x24, true
	case "END":
		return 0x23, true
	case "PGUP", "PAGEUP":
		return 0x21, true
	case "PGDN", "PAGEDOWN":
		return 0x22, true
	}
	return 0, false
}

func normalizeHotkeyChord(mods uint32, key string) string {
	parts := make([]string, 0, 4)
	if mods&modControl != 0 {
		parts = append(parts, "Ctrl")
	}
	if mods&modShift != 0 {
		parts = append(parts, "Shift")
	}
	if mods&modAlt != 0 {
		parts = append(parts, "Alt")
	}
	if mods&modWin != 0 {
		parts = append(parts, "Win")
	}
	parts = append(parts, strings.ToUpper(strings.TrimSpace(key)))
	return strings.Join(parts, "+")
}

// SetMiniHUDHotkey registers a global show/hide Mini-HUD hotkey (fail soft).
// Empty chord restores the default Ctrl+Shift+M.
func (a *App) SetMiniHUDHotkey(chord string) map[string]interface{} {
	if strings.TrimSpace(chord) == "" {
		chord = defaultMiniHUDHotkey
	}
	parsed := ParseHotkeyChord(chord)
	out := map[string]interface{}{
		"ok":     false,
		"chord":  chord,
		"active": false,
	}
	if !parsed.Valid {
		out["error"] = parsed.Reason
		out["chord"] = defaultMiniHUDHotkey
		// Still try default.
		parsed = ParseHotkeyChord(defaultMiniHUDHotkey)
		if !parsed.Valid {
			return out
		}
	}
	hotkeyMu.Lock()
	hotkeyApp = a
	hotkeyChord = parsed.Chord
	hotkeyMods = parsed.Mods
	hotkeyVK = parsed.VK
	hotkeyMu.Unlock()

	ok, errMsg := applyMiniHUDHotkey(parsed.Mods, parsed.VK)
	out["ok"] = ok
	out["chord"] = parsed.Chord
	out["active"] = ok
	if errMsg != "" {
		out["error"] = errMsg
	}
	return out
}

// GetMiniHUDHotkey returns the current chord and registration state.
func (a *App) GetMiniHUDHotkey() map[string]interface{} {
	hotkeyMu.Lock()
	defer hotkeyMu.Unlock()
	return map[string]interface{}{
		"chord":  hotkeyChord,
		"active": hotkeyActive,
		"ok":     true,
	}
}

func applyMiniHUDHotkey(mods, vk uint32) (bool, string) {
	if goruntime.GOOS != "windows" {
		return false, "hotkey requires Windows"
	}
	ensureHotkeyPump()
	hotkeyMu.Lock()
	hwnd := hotkeyHWND
	hotkeyMu.Unlock()
	if hwnd == 0 {
		// Pump not ready yet; mark desired and pump will register.
		return true, ""
	}
	_, _, _ = procUnregisterHotKey.Call(hwnd, hotkeyIDMiniHUD)
	r1, _, err := procRegisterHotKey.Call(hwnd, hotkeyIDMiniHUD, uintptr(mods|modNoRepeat), uintptr(vk))
	hotkeyMu.Lock()
	hotkeyActive = r1 != 0
	hotkeyMu.Unlock()
	if r1 == 0 {
		return false, fmt.Sprintf("register failed: %v", err)
	}
	return true, ""
}

func ensureHotkeyPump() {
	hotkeyMu.Lock()
	if hotkeyThreadID != 0 {
		hotkeyMu.Unlock()
		return
	}
	hotkeyMu.Unlock()
	go hotkeyMessagePump()
}

func hotkeyMessagePump() {
	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()

	registerHotkeyWindowClass()
	hwnd := createMessageOnlyWindow()
	if hwnd == 0 {
		return
	}
	tid := windows.GetCurrentThreadId()
	hotkeyMu.Lock()
	hotkeyHWND = hwnd
	hotkeyThreadID = tid
	mods := hotkeyMods
	vk := hotkeyVK
	if mods == 0 || vk == 0 {
		parsed := ParseHotkeyChord(defaultMiniHUDHotkey)
		mods, vk = parsed.Mods, parsed.VK
		hotkeyMods, hotkeyVK = mods, vk
		hotkeyChord = parsed.Chord
	}
	hotkeyMu.Unlock()

	_, _, _ = procUnregisterHotKey.Call(hwnd, hotkeyIDMiniHUD)
	r1, _, _ := procRegisterHotKey.Call(hwnd, hotkeyIDMiniHUD, uintptr(mods|modNoRepeat), uintptr(vk))
	hotkeyMu.Lock()
	hotkeyActive = r1 != 0
	hotkeyMu.Unlock()

	var msg msgStruct
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		if msg.Message == wmHotkey && msg.WParam == hotkeyIDMiniHUD {
			onMiniHUDHotkey()
			continue
		}
		activateID := ActivateMessageID()
		if activateID != 0 && msg.Message == activateID {
			hotkeyMu.Lock()
			app := hotkeyApp
			hotkeyMu.Unlock()
			if app != nil {
				app.HandleActivateIPC()
			}
			continue
		}
		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	_, _, _ = procUnregisterHotKey.Call(hwnd, hotkeyIDMiniHUD)
	_, _, _ = procDestroyWindow.Call(hwnd)
	hotkeyMu.Lock()
	hotkeyHWND = 0
	hotkeyThreadID = 0
	hotkeyActive = false
	hotkeyMu.Unlock()
}

func onMiniHUDHotkey() {
	hotkeyMu.Lock()
	app := hotkeyApp
	hotkeyMu.Unlock()
	if app == nil || app.ctx == nil {
		return
	}
	app.mu.RLock()
	mini := app.miniHUD
	app.mu.RUnlock()
	if mini {
		app.ToggleMiniHUD(false)
		app.ShowWindow()
		return
	}
	app.ShowWindow()
	app.ToggleMiniHUD(true)
	wailsruntime.EventsEmit(app.ctx, "ui:minihud", true)
}

func stopMiniHUDHotkey() {
	hotkeyMu.Lock()
	tid := hotkeyThreadID
	hwnd := hotkeyHWND
	hotkeyMu.Unlock()
	if hwnd != 0 {
		_, _, _ = procUnregisterHotKey.Call(hwnd, hotkeyIDMiniHUD)
	}
	if tid != 0 {
		_, _, _ = procPostThreadMsg.Call(uintptr(tid), wmQuit, 0, 0)
	}
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

func registerHotkeyWindowClass() {
	hotkeyClassOnce.Do(func() {
		className, _ := windows.UTF16PtrFromString("PulseCoreHotkeyHidden")
		hInst, _, _ := procGetModuleHandleW.Call(0)
		wc := wndClassEx{
			Size:      uint32(unsafe.Sizeof(wndClassEx{})),
			WndProc:   windows.NewCallback(hotkeyWndProc),
			Instance:  windows.Handle(hInst),
			ClassName: className,
		}
		_, _, _ = procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	})
}

func hotkeyWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	r1, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r1
}

func createMessageOnlyWindow() uintptr {
	className, _ := windows.UTF16PtrFromString("PulseCoreHotkeyHidden")
	title, _ := windows.UTF16PtrFromString("PulseCoreHotkey")
	const hwndMessage = ^uintptr(2) // HWND_MESSAGE = -3
	hInst, _, _ := procGetModuleHandleW.Call(0)
	r1, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		0,
		0, 0, 0, 0,
		hwndMessage,
		0,
		hInst,
		0,
	)
	return r1
}
