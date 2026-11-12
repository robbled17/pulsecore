package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestGameSessionLifecycle(t *testing.T) {
	prev := gameBookPathOverride
	gameBookPathOverride = filepath.Join(t.TempDir(), "game_sessions.json")
	defer func() { gameBookPathOverride = prev }()

	a := NewApp()
	a.loadGameBook()
	now := time.Now()
	a.beginActiveGame("realfps.exe", "realfps.exe", now)
	a.sampleActiveGame(now.Add(time.Minute), 40, 70, 60, 75)
	a.noteGameFreeze("CPU_HOG")
	st := a.GetActiveGameStatus()
	if !st.Active || st.Freezes != 1 {
		t.Fatalf("active=%v freezes=%d", st.Active, st.Freezes)
	}
	a.endActiveGame(now.Add(5 * time.Minute))
	list := a.GetGameSessions()
	if len(list) < 1 || list[0].Active {
		t.Fatalf("expected completed session, got %#v", list)
	}
	if list[0].Freezes != 1 {
		t.Fatalf("freezes=%d", list[0].Freezes)
	}
	if isSyntheticGameSession(list[0]) {
		t.Fatal("real session marked synthetic")
	}
}

func TestScrubSyntheticGameSessions(t *testing.T) {
	in := []GameSession{
		{ID: "1", Name: "game", Exe: "game.exe", DurationMin: 5, CPUAvg: 40, GPUAvg: 70, RAMMax: 60, Freezes: 1},
		{ID: "2", Name: "Cyberpunk", Exe: "Cyberpunk2077.exe", DurationMin: 42, CPUAvg: 55, GPUAvg: 88, RAMMax: 70},
		{ID: "3", Name: "demo", Exe: "foo.exe", Synthetic: true},
	}
	out, changed := scrubGameHistory(in)
	if !changed || len(out) != 1 || out[0].ID != "2" {
		t.Fatalf("scrub failed: changed=%v out=%#v", changed, out)
	}
}
