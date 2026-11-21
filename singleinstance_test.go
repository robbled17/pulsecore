package main

import (
	"strings"
	"testing"
)

func TestAcquireSingleInstance_Primary(t *testing.T) {
	// Use a unique local mutex so CI parallel packages do not collide with a live PulseCore.
	name := `Local\PulseCoreSingleInstanceTestPrimary`
	activated := false
	res := acquireSingleInstance(name, func() bool {
		activated = true
		return true
	})
	if !res.IsPrimary {
		t.Fatalf("expected primary, got %+v", res)
	}
	if activated {
		t.Fatal("activate must not run for primary")
	}
	releaseSingleInstance()
}

func TestAcquireSingleInstance_SecondActivates(t *testing.T) {
	name := `Local\PulseCoreSingleInstanceTestSecond`
	first := acquireSingleInstance(name, nil)
	if !first.IsPrimary {
		t.Fatalf("first should be primary: %+v", first)
	}
	defer releaseSingleInstance()

	called := false
	second := acquireSingleInstance(name, func() bool {
		called = true
		return true
	})
	if second.IsPrimary {
		t.Fatalf("second must not be primary: %+v", second)
	}
	if !second.Activated || !called {
		t.Fatalf("second should activate existing: %+v called=%v", second, called)
	}
}

func TestActivateMessageID_Registered(t *testing.T) {
	id := ActivateMessageID()
	if id == 0 {
		t.Fatal("RegisterWindowMessage should return non-zero id on Windows")
	}
}

func TestSingleInstanceMutexName_Documented(t *testing.T) {
	if pulseSingleInstanceMutex != `Global\PulseCoreSingleInstance` {
		t.Fatalf("mutex name changed: %s", pulseSingleInstanceMutex)
	}
	if strings.Contains(pulseSingleInstanceMutex, "\u2013") || strings.Contains(pulseSingleInstanceMutex, "\u2014") {
		t.Fatal("mutex name must not use long dashes")
	}
}

// Hardware/OS-only checks (not automated here):
// - Live dual-launch: start pulsecore.exe twice; second must restore tray/minimized first window.
// - Live Mini-HUD hotkey inside fullscreen games (RegisterHotKey + focus rules).
