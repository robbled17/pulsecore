package main

import (
	"sync"
	"time"
)

// DriveScanCache stores cached scan results for a specific disk drive.
type DriveScanCache struct {
	DriveLetter     string             `json:"driveLetter"`
	ScannedAt       time.Time          `json:"scannedAt"`
	Apps            []InstalledApp     `json:"apps"`
	Leftovers       []OrphanLeftover   `json:"leftovers"`
	Cache           []CleanupCat       `json:"cache"`
	System          DeepCleanupPreview `json:"system"`
	AppsLoaded      bool               `json:"appsLoaded"`
	LeftoversLoaded bool               `json:"leftoversLoaded"`
	CacheLoaded     bool               `json:"cacheLoaded"`
	SystemLoaded    bool               `json:"systemLoaded"`
}

var (
	driveScanCacheMu sync.RWMutex
	driveScanCaches  = make(map[string]*DriveScanCache)
)

func getDriveCacheLocked(drive string) *DriveScanCache {
	drive = normalizeDriveLetter(drive)
	c, ok := driveScanCaches[drive]
	if !ok || c == nil {
		c = &DriveScanCache{
			DriveLetter: drive,
			ScannedAt:   time.Now(),
		}
		driveScanCaches[drive] = c
	}
	return c
}

// InvalidateDriveScanCache clears cached results for a drive, or all drives if drive is empty.
func (a *App) InvalidateDriveScanCache(drive string) {
	driveScanCacheMu.Lock()
	defer driveScanCacheMu.Unlock()
	if drive == "" {
		driveScanCaches = make(map[string]*DriveScanCache)
		return
	}
	d := normalizeDriveLetter(drive)
	delete(driveScanCaches, d)
}

// GetDriveScanCache retrieves cached results for a drive without rescanning.
func (a *App) GetDriveScanCache(drive string) *DriveScanCache {
	driveScanCacheMu.RLock()
	defer driveScanCacheMu.RUnlock()
	d := normalizeDriveLetter(drive)
	return driveScanCaches[d]
}

// RefreshDriveScan forces a cache invalidation and fresh scan for the drive.
func (a *App) RefreshDriveScan(drive string) {
	a.InvalidateDriveScanCache(drive)
}
