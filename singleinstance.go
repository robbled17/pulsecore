package main

import (
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	pulseSingleInstanceMutex = `Global\PulseCoreSingleInstance`
	pulseActivateEventName   = `Global\PulseCoreActivateEvent`
	pulseActivateMsgName     = "PulseCore_Activate_v1"
	pulseWindowTitle         = "PulseCore"
)

var (
	procFindWindowW      = modUser32.NewProc("FindWindowW")
	procSetForegroundWnd = modUser32.NewProc("SetForegroundWindow")
	procShowWindow       = modUser32.NewProc("ShowWindow")
	procIsIconic         = modUser32.NewProc("IsIconic")
	procPostMessageW     = modUser32.NewProc("PostMessageW")
	procRegisterWindowMsg = modUser32.NewProc("RegisterWindowMessageW")

	singleInstanceMu     sync.Mutex
	singleInstanceHandle windows.Handle
	activateEventHandle  windows.Handle
	activateMsgID        uint32
	activateListenerApp  *App
	activateStop         chan struct{}
)

const (
	swRestore = 9
	swShow    = 5
)

// SingleInstanceResult describes the outcome of ensureSingleInstance.
type SingleInstanceResult struct {
	IsPrimary bool
	Activated bool // second instance activated existing window
	Error     string
}

// ensureSingleInstance acquires Global\PulseCoreSingleInstance.
// If another instance owns the mutex, it activates that window and returns IsPrimary=false.
// Fail-soft: mutex/API errors allow the process to continue as primary so first launch is never blocked.
func ensureSingleInstance() SingleInstanceResult {
	if runtime.GOOS != "windows" {
		return SingleInstanceResult{IsPrimary: true}
	}
	return acquireSingleInstance(pulseSingleInstanceMutex, activateExistingPulseCore)
}

// acquireSingleInstance is the testable core (mutex name + activate callback injectable).
func acquireSingleInstance(mutexName string, activate func() bool) SingleInstanceResult {
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return SingleInstanceResult{IsPrimary: true, Error: err.Error()}
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if handle == 0 {
		// Fail soft: do not block first launch.
		msg := "CreateMutex failed"
		if err != nil {
			msg = err.Error()
		}
		return SingleInstanceResult{IsPrimary: true, Error: msg}
	}

	if err == windows.ERROR_ALREADY_EXISTS {
		_ = windows.CloseHandle(handle)
		activated := false
		if activate != nil {
			activated = activate()
		}
		return SingleInstanceResult{IsPrimary: false, Activated: activated}
	}

	singleInstanceMu.Lock()
	singleInstanceHandle = handle
	singleInstanceMu.Unlock()
	_ = ensureActivateMessageID()
	_ = ensureActivateEvent()
	return SingleInstanceResult{IsPrimary: true}
}

func releaseSingleInstance() {
	singleInstanceMu.Lock()
	h := singleInstanceHandle
	singleInstanceHandle = 0
	ev := activateEventHandle
	activateEventHandle = 0
	stop := activateStop
	activateStop = nil
	singleInstanceMu.Unlock()
	if stop != nil {
		close(stop)
	}
	if ev != 0 {
		_ = windows.CloseHandle(ev)
	}
	if h != 0 {
		_ = windows.CloseHandle(h)
	}
}

func ensureActivateEvent() windows.Handle {
	singleInstanceMu.Lock()
	if activateEventHandle != 0 {
		h := activateEventHandle
		singleInstanceMu.Unlock()
		return h
	}
	singleInstanceMu.Unlock()
	name, err := windows.UTF16PtrFromString(pulseActivateEventName)
	if err != nil {
		return 0
	}
	h, err := windows.CreateEvent(nil, 0, 0, name)
	if h == 0 {
		return 0
	}
	// If the event already existed, CreateEvent still returns a usable handle.
	_ = err
	singleInstanceMu.Lock()
	activateEventHandle = h
	singleInstanceMu.Unlock()
	return h
}

// startActivateListener waits on the named activate event and restores the primary window.
func (a *App) startActivateListener() {
	if a == nil {
		return
	}
	h := ensureActivateEvent()
	if h == 0 {
		return
	}
	singleInstanceMu.Lock()
	activateListenerApp = a
	if activateStop == nil {
		activateStop = make(chan struct{})
	}
	stop := activateStop
	singleInstanceMu.Unlock()
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			// Wait up to 500ms so shutdown can close the loop promptly.
			ev, err := windows.WaitForSingleObject(h, 500)
			if err != nil {
				continue
			}
			if ev == windows.WAIT_OBJECT_0 {
				singleInstanceMu.Lock()
				app := activateListenerApp
				singleInstanceMu.Unlock()
				if app != nil {
					app.HandleActivateIPC()
				}
			}
		}
	}()
}

func signalActivateEvent() bool {
	name, err := windows.UTF16PtrFromString(pulseActivateEventName)
	if err != nil {
		return false
	}
	h, err := windows.OpenEvent(windows.EVENT_MODIFY_STATE, false, name)
	if err != nil || h == 0 {
		return false
	}
	defer windows.CloseHandle(h)
	return windows.SetEvent(h) == nil
}

func ensureActivateMessageID() uint32 {
	if activateMsgID != 0 {
		return activateMsgID
	}
	name, err := windows.UTF16PtrFromString(pulseActivateMsgName)
	if err != nil {
		return 0
	}
	r1, _, _ := procRegisterWindowMsg.Call(uintptr(unsafe.Pointer(name)))
	activateMsgID = uint32(r1)
	return activateMsgID
}

// ActivateMessageID returns the registered window message used for second-launch IPC.
func ActivateMessageID() uint32 {
	return ensureActivateMessageID()
}

func activateExistingPulseCore() bool {
	signaled := signalActivateEvent()
	hwnd := findPulseCoreWindow()
	if hwnd != 0 {
		msg := ensureActivateMessageID()
		if msg != 0 {
			_, _, _ = procPostMessageW.Call(hwnd, uintptr(msg), 0, 0)
		}
		_ = restoreAndFocusWindow(hwnd)
		return true
	}
	return signaled
}

func findPulseCoreWindow() uintptr {
	title, err := windows.UTF16PtrFromString(pulseWindowTitle)
	if err != nil {
		return 0
	}
	r1, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
	return r1
}

func restoreAndFocusWindow(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	iconic, _, _ := procIsIconic.Call(hwnd)
	if iconic != 0 {
		_, _, _ = procShowWindow.Call(hwnd, swRestore)
	} else {
		_, _, _ = procShowWindow.Call(hwnd, swShow)
	}
	// Best-effort foreground; Windows may still deny focus steals.
	_, _, _ = procSetForegroundWnd.Call(hwnd)
	return true
}

// HandleActivateIPC restores the main window when a second launch posts the activate message.
// Safe to call from the hotkey pump or other Win32 message handlers.
func (a *App) HandleActivateIPC() {
	if a == nil {
		return
	}
	a.ShowWindow()
}
