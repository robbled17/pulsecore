package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var deleteJobSeq atomic.Uint64

type deleteProgress struct {
	JobID      string `json:"jobId"`
	Phase      string `json:"phase"` // started|uninstall|cleanup|done|error
	Percent    int    `json:"percent"`
	Label      string `json:"label"`
	Path       string `json:"path"`
	FreedBytes int64  `json:"freedBytes"`
	UsedBefore int64  `json:"usedBefore"`
	UsedAfter  int64  `json:"usedAfter"`
	OK         bool   `json:"ok"`
	Message    string `json:"message"`
	AppID      string `json:"appId"`
}

var deleteMu sync.Mutex

func (a *App) emitDeleteProgress(p deleteProgress) {
	if a == nil || a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "delete-progress", p)
}

func (a *App) emitAppDeletedSuccess(name, appID string, freed int64) {
	if a == nil || a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "app-deleted-success", map[string]interface{}{
		"name": name, "appId": appID, "freedBytes": freed,
	})
}

// OpenFolderInExplorer opens the path in Windows Explorer.
func (a *App) OpenFolderInExplorer(folderPath string) error {
	return openPathInExplorer(folderPath)
}

// StartRemovePaths deletes paths asynchronously and streams delete-progress events.
func (a *App) StartRemovePaths(paths []string, label string) map[string]interface{} {
	jobID := fmt.Sprintf("del-%d", deleteJobSeq.Add(1))
	clean := make([]string, 0, len(paths))
	for _, p := range paths {
		p = filepath.Clean(strings.TrimSpace(p))
		if p != "" && isSafeAppLeftoverPath(p) {
			clean = append(clean, p)
		}
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logCrash("StartRemoveOrphanLeftover", r)
			}
		}()
		deleteMu.Lock()
		defer deleteMu.Unlock()
		a.emitDeleteProgress(deleteProgress{JobID: jobID, Phase: "started", Percent: 5, Label: label, OK: true})
		var reported int64
		_, rec := a.MeasureAndRecordClean("C:", "AppLeftover", func() int64 {
			n := len(clean)
			for i, p := range clean {
				pct := 10
				if n > 0 {
					pct = 10 + (i * 80 / n)
				}
				a.emitDeleteProgress(deleteProgress{
					JobID: jobID, Phase: "cleanup", Percent: pct, Label: label, Path: p, OK: true,
				})
				sz := int64(0)
				if info, err := os.Stat(p); err == nil {
					if info.IsDir() {
						sz = dirSizeBudget(p, time.Now().Add(800*time.Millisecond), 5)
					} else {
						sz = info.Size()
					}
				}
				if err := ForceDeleteAll(p); err == nil {
					reported += sz
				}
			}
			return reported
		})
		a.RefreshDiskMetricsEx()
		freed := int64(rec.BytesFreed)
		if freed <= 0 {
			freed = reported
		}
		a.emitDeleteProgress(deleteProgress{
			JobID: jobID, Phase: "done", Percent: 100, Label: label,
			FreedBytes: freed, OK: true, Message: "ok",
			UsedBefore: int64(rec.TotalBytes - rec.BytesBefore),
			UsedAfter:  int64(rec.TotalBytes - rec.BytesAfter),
		})
	}()
	return map[string]interface{}{"started": true, "jobId": jobID}
}

// StartUninstallWithCleanup runs vendor uninstall + leftover wipe off the UI thread.
// For launcher-driven uninstalls (Lesta/Wargaming/Steam) a background watcher tracks
// the InstallLocation until the game is actually removed.
func (a *App) StartUninstallWithCleanup(appID string) map[string]interface{} {
	appID = strings.TrimSpace(appID)
	jobID := fmt.Sprintf("un-%d", deleteJobSeq.Add(1))
	var appName, installLoc string
	var sizeBytes int64
	for _, app := range a.ListInstalledApps() {
		if app.ID == appID {
			appName = app.DisplayName
			sizeBytes = app.SizeBytes
			installLoc = app.InstallLocation
			break
		}
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logCrash("StartUninstallWithCleanup", r)
			}
		}()
		deleteMu.Lock()
		defer deleteMu.Unlock()

		initialBytes := sizeBytes
		if loc := strings.TrimSpace(installLoc); loc != "" {
			if measured := GetFolderSize(loc); measured > initialBytes {
				initialBytes = measured
			}
			// Watch for external deletion via launcher UI (Tanks Blitz / Lesta etc.).
			a.startExternalUninstallWatch(loc, initialBytes, appName, appID)
		}

		a.emitDeleteProgress(deleteProgress{
			JobID: jobID, Phase: "started", Percent: 5, Label: appName, AppID: appID, OK: true,
		})
		a.emitDeleteProgress(deleteProgress{
			JobID: jobID, Phase: "uninstall", Percent: 20, Label: appName, AppID: appID, OK: true,
		})
		res := a.UninstallWithCleanup(appID)
		if !res.OK || res.Message == "cancelled_or_retained" {
			a.cancelExternalUninstallWatch(appID)
			a.emitDeleteProgress(deleteProgress{
				JobID: jobID, Phase: "error", Percent: 100, Label: appName, AppID: appID,
				OK: false, Message: "cancelled_or_retained", FreedBytes: 0,
			})
			return
		}

		a.emitDeleteProgress(deleteProgress{
			JobID: jobID, Phase: "cleanup", Percent: 55, Label: appName, AppID: appID, OK: true,
			Message: res.Message, Path: installLoc,
		})

		var reported int64 = res.FreedBytes
		drive := driveLetterFromPath(installLoc)
		_, rec := a.MeasureAndRecordClean(drive, "AppUninstall", func() int64 {
			paths := make([]string, 0, len(res.Leftovers))
			for _, l := range res.Leftovers {
				paths = append(paths, l.Path)
			}
			if len(paths) > 0 {
				tail := a.RemoveAppLeftovers(paths)
				reported += tail.FreedBytes
			}
			if reported <= 0 && sizeBytes > 0 && !folderStillHeavy(installLoc) {
				reported = sizeBytes
			}
			return reported
		})
		a.RefreshDiskMetricsEx()

		// Launcher unfinished: keep waiting — don't claim "0 bytes" success.
		if folderStillHeavy(installLoc) {
			a.emitDeleteProgress(deleteProgress{
				JobID: jobID, Phase: "watching", Percent: 85, Label: appName, AppID: appID,
				OK: true, Message: "waiting-launcher", Path: installLoc, FreedBytes: 0,
			})
			return
		}

		a.cancelExternalUninstallWatch(appID)
		freed := int64(rec.BytesFreed)
		if freed <= 0 {
			freed = reported
		}
		if freed <= 0 && initialBytes > 0 {
			freed = initialBytes
		}
		usedBefore := int64(0)
		usedAfter := int64(0)
		if rec.TotalBytes > rec.BytesBefore {
			usedBefore = int64(rec.TotalBytes - rec.BytesBefore)
		}
		if rec.TotalBytes > rec.BytesAfter {
			usedAfter = int64(rec.TotalBytes - rec.BytesAfter)
		}
		a.emitDeleteProgress(deleteProgress{
			JobID: jobID, Phase: "done", Percent: 100, Label: appName, AppID: appID,
			FreedBytes: freed, OK: true, Message: "ok",
			UsedBefore: usedBefore, UsedAfter: usedAfter,
		})
		a.emitAppDeletedSuccess(appName, appID, freed)
	}()
	return map[string]interface{}{
		"started": true, "jobId": jobID, "name": appName, "sizeBytes": sizeBytes,
	}
}
