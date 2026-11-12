package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	externalWatchInterval = 1500 * time.Millisecond
	externalWatchTimeout  = 4 * time.Minute
	externalGoneThreshold = int64(50 * 1024 * 1024) // < 50 MB ≈ removed
)

// GetFolderSize measures directory size with a generous budget (large game trees).
func GetFolderSize(dirPath string) int64 {
	dirPath = filepath.Clean(strings.TrimSpace(dirPath))
	if dirPath == "" || dirPath == "." {
		return 0
	}
	info, err := os.Stat(dirPath)
	if err != nil {
		return 0
	}
	if !info.IsDir() {
		return info.Size()
	}
	return dirSizeBudget(dirPath, time.Now().Add(3*time.Second), 10)
}

var externalWatchMu sync.Mutex
var externalWatchCancel = map[string]chan struct{}{}

func (a *App) cancelExternalUninstallWatch(appID string) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return
	}
	externalWatchMu.Lock()
	ch, ok := externalWatchCancel[appID]
	if ok {
		delete(externalWatchCancel, appID)
	}
	externalWatchMu.Unlock()
	if ok && ch != nil {
		select {
		case <-ch:
		default:
			close(ch)
		}
	}
}

// startExternalUninstallWatch polls InstallLocation until the game folder is gone
// or shrunk by >80% / below 50 MB (typical Lesta/Wargaming/Steam launcher uninstall).
func (a *App) startExternalUninstallWatch(folderPath string, expectedFreed int64, appName, appID string) {
	folderPath = filepath.Clean(strings.TrimSpace(folderPath))
	appID = strings.TrimSpace(appID)
	appName = strings.TrimSpace(appName)
	if a == nil || folderPath == "" || appID == "" {
		return
	}
	if expectedFreed < 0 {
		expectedFreed = 0
	}

	a.cancelExternalUninstallWatch(appID)
	cancel := make(chan struct{})
	externalWatchMu.Lock()
	externalWatchCancel[appID] = cancel
	externalWatchMu.Unlock()

	baseline := expectedFreed
	if baseline <= 0 {
		baseline = GetFolderSize(folderPath)
	}

	go func(folderPath string, expectedFreed, baseline int64, appName, appID string, cancel <-chan struct{}) {
		ticker := time.NewTicker(externalWatchInterval)
		defer ticker.Stop()
		timeout := time.After(externalWatchTimeout)
		defer a.cancelExternalUninstallWatch(appID)

		for {
			select {
			case <-cancel:
				return
			case <-timeout:
				return
			case <-ticker.C:
				if externalFolderGoneOrTiny(folderPath, baseline) {
					a.onExternalAppDeleted(folderPath, expectedFreed, baseline, appName, appID)
					return
				}
			}
		}
	}(folderPath, expectedFreed, baseline, appName, appID, cancel)
}

func externalFolderGoneOrTiny(folderPath string, baseline int64) bool {
	info, err := os.Stat(folderPath)
	if err != nil {
		return os.IsNotExist(err)
	}
	if !info.IsDir() {
		return false
	}
	sz := GetFolderSize(folderPath)
	if sz < externalGoneThreshold {
		return true
	}
	if baseline > externalGoneThreshold && sz < baseline/5 { // >80% gone
		return true
	}
	return false
}

func (a *App) onExternalAppDeleted(folderPath string, expectedFreed, baseline int64, appName, appID string) {
	if a == nil {
		return
	}
	freed := expectedFreed
	if freed <= 0 {
		freed = baseline
	}
	if freed <= 0 {
		freed = GetFolderSize(folderPath)
	}
	// Prefer measured free-space delta; fall back to expected folder weight.
	drive := driveLetterFromPath(folderPath)
	_, rec := a.MeasureAndRecordClean(drive, "AppUninstall", func() int64 {
		return freed
	})
	actual := int64(rec.BytesFreed)
	if actual <= 0 {
		actual = freed
	}
	a.RefreshDiskMetricsEx()
	a.emitExternalAppDeleted(appName, appID, actual)
	a.emitDeleteProgress(deleteProgress{
		JobID:      "ext-" + appID,
		Phase:      "done",
		Percent:    100,
		Label:      appName,
		AppID:      appID,
		FreedBytes: actual,
		OK:         true,
		Message:    "external",
		Path:       folderPath,
	})
	a.emitAppDeletedSuccess(appName, appID, actual)
}

func (a *App) emitExternalAppDeleted(name, appID string, freed int64) {
	if a == nil || a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "external-app-deleted", map[string]interface{}{
		"name":         name,
		"appId":        appID,
		"freed_bytes":  freed,
		"freedBytes":   freed,
	})
}

func driveLetterFromPath(p string) string {
	p = strings.TrimSpace(p)
	if len(p) >= 2 && p[1] == ':' {
		return strings.ToUpper(p[:1])
	}
	return "C"
}

// folderStillHeavy reports whether InstallLocation is still a large tree after uninstall.
func folderStillHeavy(folderPath string) bool {
	folderPath = filepath.Clean(strings.TrimSpace(folderPath))
	if folderPath == "" {
		return false
	}
	if _, err := os.Stat(folderPath); err != nil {
		return false
	}
	return GetFolderSize(folderPath) >= externalGoneThreshold
}
