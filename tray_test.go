//go:build windows

package main

import (
	"strings"
	"sync"
	"testing"

	"golang.org/x/sys/windows"
)

type mockTrayRecorder struct {
	mu           sync.Mutex
	addCalls     int
	modifyCalls  int
	deleteCalls  int
	destroyCalls int
	lastNID      NOTIFYICONDATAW
	destroyed    []HICON
}

func (m *mockTrayRecorder) notify(dwMessage uint32, lpData *NOTIFYICONDATAW) uintptr {
	m.mu.Lock()
	defer m.mu.Unlock()
	if lpData != nil {
		m.lastNID = *lpData
	}
	switch dwMessage {
	case NIM_ADD:
		m.addCalls++
	case NIM_MODIFY:
		m.modifyCalls++
	case NIM_DELETE:
		m.deleteCalls++
	}
	return 1
}

func (m *mockTrayRecorder) destroy(hIcon HICON) uintptr {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.destroyCalls++
	m.destroyed = append(m.destroyed, hIcon)
	return 1
}

func TestTrayInitIdempotency(t *testing.T) {
	app := &App{
		metrics: SystemMetrics{TemperatureC: 45, CPUPercent: 20},
		prefs:   appPrefs{ShowTempOnTray: true, ShowTrayTemp: true},
	}
	tm := newTrayManager(app)
	rec := &mockTrayRecorder{}
	tm.notifyIcon = rec.notify
	tm.destroyIcon = rec.destroy
	tm.hWnd = windows.HWND(0x1234) // dummy window handle
	tm.defaultHIcon = HICON(0xAAA1)

	// First initialization: must call NIM_ADD exactly once
	tm.mu.Lock()
	err := tm.initTrayLocked()
	tm.mu.Unlock()
	if err != nil {
		t.Fatalf("first initTrayLocked failed: %v", err)
	}
	if rec.addCalls != 1 {
		t.Fatalf("expected 1 NIM_ADD call, got %d", rec.addCalls)
	}
	if !tm.initialized {
		t.Fatalf("expected tm.initialized to be true")
	}
	if tm.uID != 1 {
		t.Fatalf("expected tm.uID == 1, got %d", tm.uID)
	}

	// Second initialization attempt: must be idempotent and NOT call NIM_ADD again
	tm.mu.Lock()
	err = tm.initTrayLocked()
	tm.mu.Unlock()
	if err != nil {
		t.Fatalf("second initTrayLocked failed: %v", err)
	}
	if rec.addCalls != 1 {
		t.Fatalf("expected NIM_ADD call count to remain 1, got %d", rec.addCalls)
	}

	// Destroy: must call NIM_DELETE
	tm.mu.Lock()
	tm.destroyTrayLocked()
	tm.mu.Unlock()
	if rec.deleteCalls != 1 {
		t.Fatalf("expected 1 NIM_DELETE call, got %d", rec.deleteCalls)
	}
	if tm.initialized {
		t.Fatalf("expected tm.initialized to be false after destroy")
	}
}

func TestTrayModeSwitching(t *testing.T) {
	app := &App{
		metrics: SystemMetrics{TemperatureC: 55, CPUPercent: 25},
		prefs:   appPrefs{ShowTempOnTray: true, ShowTrayTemp: true},
	}
	tm := newTrayManager(app)
	rec := &mockTrayRecorder{}
	tm.notifyIcon = rec.notify
	tm.destroyIcon = rec.destroy
	tm.hWnd = windows.HWND(0x5678)
	tm.defaultHIcon = HICON(0xDEF0)

	// Initialize tray
	tm.mu.Lock()
	_ = tm.initTrayLocked()
	tm.mu.Unlock()

	// Mode: ShowTrayTemp == true -> dynamic temp icon + tooltip
	tm.refreshNow()
	if rec.modifyCalls < 1 {
		t.Fatalf("expected at least 1 NIM_MODIFY call, got %d", rec.modifyCalls)
	}
	tm.mu.Lock()
	isDyn := tm.isDynamic
	tip := tm.lastTooltip
	currIcon := tm.currentHIcon
	tm.mu.Unlock()

	if !isDyn {
		t.Fatalf("expected dynamic icon when ShowTrayTemp is true")
	}
	if !strings.HasPrefix(tip, "PulseCore · CPU 55°C") {
		t.Fatalf("expected tooltip 'PulseCore · CPU 55°C', got %q", tip)
	}
	if currIcon == tm.defaultHIcon || currIcon == 0 {
		t.Fatalf("expected valid distinct dynamic icon handle, got %v", currIcon)
	}

	// Mode switch: ShowTrayTemp == false -> revert to standard brand icon
	app.mu.Lock()
	app.prefs.ShowTrayTemp = false
	app.prefs.ShowTempOnTray = false
	app.mu.Unlock()

	prevDynIcon := currIcon
	tm.refreshNow()

	tm.mu.Lock()
	isDyn2 := tm.isDynamic
	tip2 := tm.lastTooltip
	currIcon2 := tm.currentHIcon
	tm.mu.Unlock()

	if isDyn2 {
		t.Fatalf("expected standard (non-dynamic) icon when ShowTrayTemp is false")
	}
	if currIcon2 != tm.defaultHIcon {
		t.Fatalf("expected current icon to be defaultHIcon (0xDEF0), got %v", currIcon2)
	}
	if tip2 != "PulseCore" {
		t.Fatalf("expected tooltip 'PulseCore', got %q", tip2)
	}

	// Verify previous dynamic icon was destroyed to prevent GDI leak
	rec.mu.Lock()
	destroyedCount := rec.destroyCalls
	destroyedList := append([]HICON(nil), rec.destroyed...)
	rec.mu.Unlock()

	if destroyedCount < 1 {
		t.Fatalf("expected destroyIcon to be called for previous dynamic icon")
	}
	found := false
	for _, h := range destroyedList {
		if h == prevDynIcon {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected prevDynIcon %v to be destroyed, destroyed: %v", prevDynIcon, destroyedList)
	}
}

func TestCreateHIconFromICO(t *testing.T) {
	// Test creating icon from defaultIconBytes
	hDef, err := createHIconFromICO(defaultIconBytes, 16)
	if err != nil {
		t.Fatalf("createHIconFromICO failed on defaultIconBytes: %v", err)
	}
	if hDef == 0 {
		t.Fatalf("expected non-zero HICON from defaultIconBytes")
	}
	// Verify destruction
	procDestroyIconWin.Call(uintptr(hDef))

	// Test creating dynamic glyph icon
	glyphICO := trayIconICO(65, 234, 179, 8)
	hGlyph, err2 := createHIconFromICO(glyphICO, 16)
	if err2 != nil {
		t.Fatalf("createHIconFromICO failed on glyphICO: %v", err2)
	}
	if hGlyph == 0 {
		t.Fatalf("expected non-zero HICON from glyphICO")
	}
	procDestroyIconWin.Call(uintptr(hGlyph))
}
