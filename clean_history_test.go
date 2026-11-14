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

func TestChooseFreedBytesPrefersCounted(t *testing.T) {
	const counted = uint64(145 * 1024 * 1024)
	if got := chooseFreedBytes(1000, 1000, counted); got != counted {
		t.Fatalf("zero disk delta dropped counted bytes: %d", got)
	}
	if got := chooseFreedBytes(1000, 5000, counted); got != counted {
		t.Fatalf("disk delta must not replace counted bytes: %d", got)
	}
	if got := chooseFreedBytes(1000, 1800, 0); got != 800 {
		t.Fatalf("fallback delta: %d", got)
	}
}

func TestDeletedBytesLandInStats(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	a := &App{}
	const counted = uint64(145 * 1024 * 1024)
	rec := a.recordCleanEvent("C:", "Cache", 5_000_000, 5_000_000, 20_000_000, counted)
	if rec.DeletedBytes != counted || rec.BytesFreed != counted {
		t.Fatalf("record=%+v", rec)
	}
	st := a.GetCleanStats()
	if st.TodayFreed != counted || st.AllTimeFreed != counted {
		t.Fatalf("stats today=%d all=%d", st.TodayFreed, st.AllTimeFreed)
	}
}
