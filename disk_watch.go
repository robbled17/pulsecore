package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const diskWatchMaxDays = 90

// DiskWatchSample is one daily snapshot of key SMART counters.
type DiskWatchSample struct {
	Date         string  `json:"date"` // YYYY-MM-DD local
	Pending      int64   `json:"pending"`
	Reallocated  int64   `json:"reallocated"`
	WearPct      float64 `json:"wearPct"`
	TempMaxC     float64 `json:"tempMaxC"`
	PowerOnHours float64 `json:"powerOnHours"`
}

// DiskWatchDrive holds samples + opt-in watch flag for one letter.
type DiskWatchDrive struct {
	Watching bool              `json:"watching"`
	Samples  []DiskWatchSample `json:"samples"`
}

type diskWatchStore struct {
	Drives map[string]*DiskWatchDrive `json:"drives"`
}

type diskWatchEngine struct {
	mu     sync.Mutex
	store  diskWatchStore
	loaded bool
	dirty  bool
}

func diskWatchPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "PulseCore", "disk_watch.json"), nil
}

func (a *App) ensureDiskWatch() *diskWatchEngine {
	if a.diskWatch == nil {
		a.diskWatch = &diskWatchEngine{store: diskWatchStore{Drives: map[string]*DiskWatchDrive{}}}
	}
	return a.diskWatch
}

func (a *App) loadDiskWatch() {
	w := a.ensureDiskWatch()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.loaded {
		return
	}
	w.loaded = true
	path, err := diskWatchPath()
	if err != nil {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var file diskWatchStore
	if json.Unmarshal(raw, &file) != nil || file.Drives == nil {
		w.store.Drives = map[string]*DiskWatchDrive{}
		return
	}
	w.store = file
	w.pruneLocked()
}

func (w *diskWatchEngine) pruneLocked() {
	if w.store.Drives == nil {
		w.store.Drives = map[string]*DiskWatchDrive{}
		return
	}
	cutoff := time.Now().AddDate(0, 0, -diskWatchMaxDays).Format("2006-01-02")
	for k, d := range w.store.Drives {
		if d == nil {
			delete(w.store.Drives, k)
			continue
		}
		kept := d.Samples[:0]
		for _, s := range d.Samples {
			if s.Date >= cutoff {
				kept = append(kept, s)
			}
		}
		d.Samples = kept
	}
}

func (a *App) flushDiskWatch() {
	w := a.ensureDiskWatch()
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.dirty {
		return
	}
	path, err := diskWatchPath()
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	w.pruneLocked()
	raw, err := json.MarshalIndent(w.store, "", "  ")
	if err != nil {
		return
	}
	if os.WriteFile(path, raw, 0o644) == nil {
		w.dirty = false
	}
}

func normalizeDriveLetter(letter string) string {
	L := ""
	for _, c := range letter {
		if c >= 'A' && c <= 'Z' {
			L = string(c)
			break
		}
		if c >= 'a' && c <= 'z' {
			L = string(c - 32)
			break
		}
	}
	if L == "" {
		return "C"
	}
	return L
}

// RecordDiskWatchSample stores at most one sample per day when watching (or always for trend baseline).
// Records even when not watching so trend can work after enabling; pruning keeps ≤90 days.
func (a *App) RecordDiskWatchSample(letter string, pending, realloc int64, wear, tempMax, poh float64) {
	a.loadDiskWatch()
	w := a.ensureDiskWatch()
	w.mu.Lock()
	defer w.mu.Unlock()
	L := normalizeDriveLetter(letter)
	d := w.store.Drives[L]
	if d == nil {
		d = &DiskWatchDrive{}
		w.store.Drives[L] = d
	}
	today := time.Now().Format("2006-01-02")
	s := DiskWatchSample{
		Date: today, Pending: pending, Reallocated: realloc,
		WearPct: wear, TempMaxC: tempMax, PowerOnHours: poh,
	}
	if n := len(d.Samples); n > 0 && d.Samples[n-1].Date == today {
		prev := d.Samples[n-1]
		if tempMax > prev.TempMaxC {
			s.TempMaxC = tempMax
		} else {
			s.TempMaxC = prev.TempMaxC
		}
		d.Samples[n-1] = s
	} else {
		d.Samples = append(d.Samples, s)
	}
	w.pruneLocked()
	w.dirty = true
}

// SetDiskWatching enables/disables observation for a drive.
func (a *App) SetDiskWatching(letter string, on bool) bool {
	a.loadDiskWatch()
	w := a.ensureDiskWatch()
	w.mu.Lock()
	defer w.mu.Unlock()
	L := normalizeDriveLetter(letter)
	d := w.store.Drives[L]
	if d == nil {
		d = &DiskWatchDrive{}
		w.store.Drives[L] = d
	}
	d.Watching = on
	w.dirty = true
	return true
}

// IsDiskWatching reports whether observation is on.
func (a *App) IsDiskWatching(letter string) bool {
	a.loadDiskWatch()
	w := a.ensureDiskWatch()
	w.mu.Lock()
	defer w.mu.Unlock()
	d := w.store.Drives[normalizeDriveLetter(letter)]
	return d != nil && d.Watching
}

// DiskWatchPrior returns the oldest sample within lookbackDays (default 7) for trend.
func (a *App) DiskWatchPrior(letter string, lookbackDays int) (pending, realloc int64, okP, okR bool, daysStable int, grew bool, from, to int64, which string) {
	a.loadDiskWatch()
	w := a.ensureDiskWatch()
	w.mu.Lock()
	defer w.mu.Unlock()
	d := w.store.Drives[normalizeDriveLetter(letter)]
	if d == nil || len(d.Samples) == 0 {
		return
	}
	if lookbackDays <= 0 {
		lookbackDays = 7
	}
	cur := d.Samples[len(d.Samples)-1]
	cutoff := time.Now().AddDate(0, 0, -lookbackDays).Format("2006-01-02")
	var prior *DiskWatchSample
	for i := range d.Samples {
		s := &d.Samples[i]
		if s.Date >= cutoff && (prior == nil || s.Date < prior.Date) {
			prior = s
		}
	}
	if prior == nil {
		prior = &d.Samples[0]
	}
	pending, realloc = prior.Pending, prior.Reallocated
	okP, okR = true, true

	// Growth vs prior within window.
	if cur.Pending > prior.Pending {
		grew, from, to, which = true, prior.Pending, cur.Pending, "pending"
	} else if cur.Reallocated > prior.Reallocated {
		grew, from, to, which = true, prior.Reallocated, cur.Reallocated, "reallocated"
	}

	// Stable days: consecutive days from end with same pending+realloc.
	daysStable = 1
	for i := len(d.Samples) - 2; i >= 0; i-- {
		s := d.Samples[i]
		if s.Pending == cur.Pending && s.Reallocated == cur.Reallocated {
			daysStable++
		} else {
			break
		}
	}
	return
}

// DiskWatchInfo is UI-facing watch/trend state for one drive.
type DiskWatchInfo struct {
	Letter    string `json:"letter"`
	Watching  bool   `json:"watching"`
	TrendCode string `json:"trendCode"` // diskTrendPendingGrew|diskTrendReallocatedGrew|diskTrendStableDays|""
	From      int64  `json:"from"`
	To        int64  `json:"to"`
	Days      int    `json:"days"`
}

// GetDiskWatchInfo returns observation + trend for the What-to-do panel.
func (a *App) GetDiskWatchInfo(letter string) DiskWatchInfo {
	L := normalizeDriveLetter(letter)
	_, _, _, _, days, grew, from, to, which := a.DiskWatchPrior(L, 7)
	info := DiskWatchInfo{Letter: L, Watching: a.IsDiskWatching(L), Days: days}
	if grew {
		info.From, info.To = from, to
		if which == "reallocated" {
			info.TrendCode = "diskTrendReallocatedGrew"
		} else {
			info.TrendCode = "diskTrendPendingGrew"
		}
		return info
	}
	if days > 0 {
		info.TrendCode = "diskTrendStableDays"
	}
	return info
}
