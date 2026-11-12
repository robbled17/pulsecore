package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanHistoryAggregates(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	a := &App{}
	now := time.Now()
	a.RecordCleanEvent("C:", "Cache", 100, 150, 1000)
	a.RecordCleanEvent("C:", "AppUninstall", 150, 250, 1000)
	st := a.GetCleanStats()
	if st.AllTimeFreed != 150 {
		t.Fatalf("all time=%d", st.AllTimeFreed)
	}
	if st.TodayFreed != 150 {
		t.Fatalf("today=%d", st.TodayFreed)
	}
	if st.LastCleanFreed != 100 {
		t.Fatalf("last=%d", st.LastCleanFreed)
	}
	if st.LastUsedBefore != 850 || st.LastUsedAfter != 750 {
		t.Fatalf("used before/after=%d/%d", st.LastUsedBefore, st.LastUsedAfter)
	}
	path := filepath.Join(dir, "PulseCore", "clean_history.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("json missing: %v", err)
	}
	_ = now
}
