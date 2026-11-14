package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CleanRecord is one successful cleanup / uninstall measurement.
type CleanRecord struct {
	Timestamp    time.Time `json:"timestamp"`
	BytesBefore  uint64    `json:"bytes_before"` // free bytes before
	BytesAfter   uint64    `json:"bytes_after"`  // free bytes after
	BytesFreed   uint64    `json:"bytes_freed"`
	DeletedBytes uint64    `json:"deletedBytes"`
	TotalBytes   uint64    `json:"total_bytes"`
	TargetDrive  string    `json:"target_drive"` // "C:"
	Category     string    `json:"category"`     // Cache|AppUninstall|Shaders|...
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

// chooseFreedBytes prefers bytes actually deleted over an NTFS free-space delta.
// A zero delta from filesystem caching must not wipe a real file-size measurement.
func chooseFreedBytes(freeBefore, freeAfter, deleted uint64) uint64 {
	if deleted > 0 {
		return deleted
	}
	if freeAfter > freeBefore {
		return freeAfter - freeBefore
	}
	return 0
}

// RecordCleanEvent measures and persists a cleanup delta (free-space based).
func (a *App) RecordCleanEvent(drive, category string, freeBefore, freeAfter, total uint64) CleanRecord {
	return a.recordCleanEvent(drive, category, freeBefore, freeAfter, total, 0)
}

func (a *App) recordCleanEvent(drive, category string, freeBefore, freeAfter, total, deleted uint64) CleanRecord {
	a.loadCleanHistory()
	e := a.ensureCleanHistory()
	e.mu.Lock()
	defer e.mu.Unlock()

	drive = normalizeDriveLetter(drive) + ":"
	category = trimCategory(category)
	freed := chooseFreedBytes(freeBefore, freeAfter, deleted)
	rec := CleanRecord{
		Timestamp:    time.Now(),
		BytesBefore:  freeBefore,
		BytesAfter:   freeAfter,
		BytesFreed:   freed,
		DeletedBytes: freed,
		TotalBytes:   total,
		TargetDrive:  drive,
		Category:     category,
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
		var deleted uint64
		if freedReported > 0 {
			deleted = uint64(freedReported)
		}
		rec = a.recordCleanEvent(drive, category, 0, 0, 0, deleted)
		return freedReported, rec
	}
	freedReported = fn()
	time.Sleep(150 * time.Millisecond)
	afterFree, total2, ok2 := getDiskFreeSpaceEx(root)
	if ok2 && total2 > 0 {
		total = total2
	}
	var deleted uint64
	if freedReported > 0 {
		deleted = uint64(freedReported)
	}
	rec = a.recordCleanEvent(drive, category, beforeFree, afterFree, total, deleted)
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
		freed := r.DeletedBytes
		if r.BytesFreed > freed {
			freed = r.BytesFreed
		}
		out.AllTimeFreed += freed
		if !r.Timestamp.Before(todayStart) {
			out.TodayFreed += freed
		}
		if !r.Timestamp.Before(monthStart) {
			out.MonthFreed += freed
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

// FormatBytesSmart formats bytes into KB, MB, or GB with exact thresholds.
func FormatBytesSmart(bytes uint64, withSign bool) string {
	sign := ""
	if withSign {
		sign = "+"
	}
	if bytes == 0 {
		return sign + "0 МБ"
	}
	if bytes < 1024*1024 {
		kb := (bytes + 512) / 1024
		return fmt.Sprintf("%s%d КБ", sign, kb)
	}
	if bytes < 1024*1024*1024 {
		mb := float64(bytes) / (1024 * 1024)
		if mb >= 10 {
			return fmt.Sprintf("%s%.0f МБ", sign, mb)
		}
		return fmt.Sprintf("%s%.1f МБ", sign, mb)
	}
	gb := float64(bytes) / (1024 * 1024 * 1024)
	return fmt.Sprintf("%s%.1f ГБ", sign, gb)
}
