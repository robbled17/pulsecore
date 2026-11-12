package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CleanRecord is one successful cleanup / uninstall measurement.
type CleanRecord struct {
	Timestamp   time.Time `json:"timestamp"`
	BytesBefore uint64    `json:"bytes_before"` // free bytes before
	BytesAfter  uint64    `json:"bytes_after"`  // free bytes after
	BytesFreed  uint64    `json:"bytes_freed"`
	TotalBytes  uint64    `json:"total_bytes"`
	TargetDrive string    `json:"target_drive"` // "C:"
	Category    string    `json:"category"`     // Cache|AppUninstall|Shaders|...
}

// CleanStats is aggregated savings for the UI.
type CleanStats struct {
	LastCleanFreed  uint64 `json:"last_clean_freed"`
	LastCleanBefore uint64 `json:"last_clean_before"` // free before last
	LastCleanAfter  uint64 `json:"last_clean_after"`  // free after last
	LastUsedBefore  uint64 `json:"last_used_before"`
	LastUsedAfter   uint64 `json:"last_used_after"`
	LastTotalBytes  uint64 `json:"last_total_bytes"`
	LastCategory    string `json:"last_category"`
	LastDrive       string `json:"last_drive"`
	TodayFreed      uint64 `json:"today_freed"`
	MonthFreed      uint64 `json:"month_freed"`
	AllTimeFreed    uint64 `json:"all_time_freed"`
}

type cleanHistoryStore struct {
	Records []CleanRecord `json:"records"`
}

type cleanHistoryEngine struct {
	mu     sync.Mutex
	store  cleanHistoryStore
	loaded bool
}

const cleanHistoryMaxRecords = 500

func cleanHistoryPath() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", err
		}
	}
	dir := filepath.Join(base, "PulseCore")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "clean_history.json"), nil
}

func (a *App) ensureCleanHistory() *cleanHistoryEngine {
	if a.cleanHistory == nil {
		a.cleanHistory = &cleanHistoryEngine{store: cleanHistoryStore{Records: nil}}
	}
	return a.cleanHistory
}

func (a *App) loadCleanHistory() {
	e := a.ensureCleanHistory()
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.loaded {
		return
	}
	e.loaded = true
	path, err := cleanHistoryPath()
	if err != nil {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		e.store.Records = nil
		return
	}
	var file cleanHistoryStore
	if json.Unmarshal(raw, &file) != nil {
		e.store.Records = nil
		return
	}
	e.store = file
}

func (a *App) saveCleanHistoryLocked(e *cleanHistoryEngine) {
	path, err := cleanHistoryPath()
	if err != nil {
		return
	}
	raw, err := json.MarshalIndent(e.store, "", "  ")
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// RecordCleanEvent measures and persists a cleanup delta (free-space based).
func (a *App) RecordCleanEvent(drive, category string, freeBefore, freeAfter, total uint64) CleanRecord {
	a.loadCleanHistory()
	e := a.ensureCleanHistory()
	e.mu.Lock()
	defer e.mu.Unlock()

	drive = normalizeDriveLetter(drive) + ":"
	category = trimCategory(category)
	var freed uint64
	if freeAfter > freeBefore {
		freed = freeAfter - freeBefore
	}
	rec := CleanRecord{
		Timestamp:   time.Now(),
		BytesBefore: freeBefore,
		BytesAfter:  freeAfter,
		BytesFreed:  freed,
		TotalBytes:  total,
		TargetDrive: drive,
		Category:    category,
	}
	if freed == 0 && freeAfter == freeBefore {
		// Still record if caller insists via separate ActualCleanedBytes path.
	}
	e.store.Records = append(e.store.Records, rec)
	if len(e.store.Records) > cleanHistoryMaxRecords {
		e.store.Records = e.store.Records[len(e.store.Records)-cleanHistoryMaxRecords:]
	}
	a.saveCleanHistoryLocked(e)
	return rec
}

// MeasureAndRecordClean snapshots free space around fn and stores history.
func (a *App) MeasureAndRecordClean(drive, category string, fn func() int64) (freedReported int64, rec CleanRecord) {
	root := driveRoot(drive)
	beforeFree, total, ok := getDiskFreeSpaceEx(root)
	if !ok {
		freedReported = fn()
		return freedReported, CleanRecord{}
	}
	freedReported = fn()
	time.Sleep(150 * time.Millisecond)
	afterFree, total2, ok2 := getDiskFreeSpaceEx(root)
	if ok2 && total2 > 0 {
		total = total2
	}
	rec = a.RecordCleanEvent(drive, category, beforeFree, afterFree, total)
	// Prefer measured free-space delta when positive; keep reported as fallback for stats UI.
	if rec.BytesFreed == 0 && freedReported > 0 {
		rec.BytesFreed = uint64(freedReported)
		// Amend last record.
		a.loadCleanHistory()
		e := a.ensureCleanHistory()
		e.mu.Lock()
		if n := len(e.store.Records); n > 0 {
			e.store.Records[n-1].BytesFreed = uint64(freedReported)
			rec = e.store.Records[n-1]
			a.saveCleanHistoryLocked(e)
		}
		e.mu.Unlock()
	}
	return freedReported, rec
}

// GetCleanStats returns aggregated cleanup savings.
func (a *App) GetCleanStats() CleanStats {
	a.loadCleanHistory()
	e := a.ensureCleanHistory()
	e.mu.Lock()
	defer e.mu.Unlock()

	out := CleanStats{}
	now := time.Now()
	y, m, d := now.Date()
	todayStart := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	monthStart := time.Date(y, m, 1, 0, 0, 0, 0, now.Location())

	for _, r := range e.store.Records {
		out.AllTimeFreed += r.BytesFreed
		if !r.Timestamp.Before(todayStart) {
			out.TodayFreed += r.BytesFreed
		}
		if !r.Timestamp.Before(monthStart) {
			out.MonthFreed += r.BytesFreed
		}
	}
	if n := len(e.store.Records); n > 0 {
		last := e.store.Records[n-1]
		out.LastCleanFreed = last.BytesFreed
		out.LastCleanBefore = last.BytesBefore
		out.LastCleanAfter = last.BytesAfter
		out.LastTotalBytes = last.TotalBytes
		out.LastCategory = last.Category
		out.LastDrive = last.TargetDrive
		if last.TotalBytes > last.BytesBefore {
			out.LastUsedBefore = last.TotalBytes - last.BytesBefore
		}
		if last.TotalBytes > last.BytesAfter {
			out.LastUsedAfter = last.TotalBytes - last.BytesAfter
		}
	}
	return out
}

func driveRoot(drive string) string {
	letter := normalizeDriveLetter(drive)
	return letter + `:\`
}

func trimCategory(c string) string {
	if c == "" {
		return "Cache"
	}
	return c
}
