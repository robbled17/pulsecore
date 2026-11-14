package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const orphanLeftoverMinBytes = 100 * 1024 * 1024 // 100 MB

// OrphanLeftover is a large AppData/ProgramData folder not matching any installed app.
type OrphanLeftover struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	DriveLetter string    `json:"driveLetter"` // e.g. "C", "D", "E"
	SizeBytes   int64     `json:"sizeBytes"`
	SizeLabel   string    `json:"sizeLabel"`
	RootKind    string    `json:"rootKind"` // local|roaming|programdata|drive_root|drive_games
	LastModTime time.Time `json:"lastModTime"`
	DaysAgo     int       `json:"daysAgo"`
	AgeBadgeRu  string    `json:"ageBadgeRu"`
	AgeBadgeEn  string    `json:"ageBadgeEn"`
	AgeStatus   string    `json:"ageStatus"` // recent|safe|neutral
}

// TrashedLeftover stores information about folders safely moved to Recycle Bin.
type TrashedLeftover struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	SizeBytes  int64     `json:"sizeBytes"`
	DeleteTime time.Time `json:"deleteTime"`
}

var (
	trashedMu        sync.Mutex
	trashedLeftovers []TrashedLeftover
)

var leftoverProtectedNames = map[string]bool{
	"microsoft": true, "packages": true, "temp": true, "tmp": true,
	"nvidia": true, "amd": true, "intel": true, "d3dscache": true,
	"google": true, "mozilla": true, "programs": true, "start menu": true,
	"desktop": true, "templates": true, "cookies": true, "network": true,
	"identitynuget": true, "application data": true, "history": true,
	"recent": true, "sendto": true, "local": true, "locallow": true,
	"pulsecore": true, "package cache": true, "windows": true,
	"system32": true, "syswow64": true, "common files": true,
	"internet explorer": true, "conhost": true, "comms": true,
	"connecteddevicesplatform": true, "crashdumps": true,
	"elevateddiagnostics": true, "placeholder": true,
	"ceflog": true, "virtualstore": true, "assembly": true,
}

// latestDirModTime scans directory entries to find the newest file modification time.
func latestDirModTime(dir string, budget time.Time, maxDepth int) time.Time {
	var newest time.Time
	if info, err := os.Stat(dir); err == nil {
		newest = info.ModTime()
	}
	var walk func(p string, depth int)
	walk = func(p string, depth int) {
		if depth > maxDepth || time.Now().After(budget) {
			return
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			return
		}
		for _, e := range entries {
			if time.Now().After(budget) {
				return
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(newest) {
				newest = info.ModTime()
			}
			if e.IsDir() {
				walk(filepath.Join(p, e.Name()), depth+1)
			}
		}
	}
	walk(dir, 1)
	return newest
}

// ListOrphanLeftovers finds large leftover folders from uninstalled software with recency analysis.
func (a *App) ListOrphanLeftovers() []OrphanLeftover {
	installed := a.ListInstalledApps()
	names := installedNameSet(installed)
	out := make([]OrphanLeftover, 0, 16)
	scanItem := func(p, name, kind string) {
		if !isSafeAppLeftoverPath(p) {
			return
		}
		fi, err := os.Stat(p)
		if err != nil || !fi.IsDir() {
			return
		}
		sz := dirSizeBudget(p, time.Now().Add(700*time.Millisecond), 3)
		if sz < orphanLeftoverMinBytes {
			return
		}
		if _, err := os.Stat(p); err != nil {
			return
		}

		// Check freshness of files inside
		modTime := latestDirModTime(p, time.Now().Add(120*time.Millisecond), 2)
		daysAgo := int(time.Since(modTime).Hours() / 24)
		if daysAgo < 0 {
			daysAgo = 0
		}

		ageStatus := "neutral"
		ageBadgeRu := ""
		ageBadgeEn := ""

		if daysAgo <= 14 {
			ageStatus = "recent"
			if daysAgo == 0 {
				ageBadgeRu = "⚠️ Использовалось сегодня — проверьте перед удалением"
				ageBadgeEn = "⚠️ Used today — check before deleting"
			} else if daysAgo == 1 {
				ageBadgeRu = "⚠️ Использовалось вчера — проверьте перед удалением"
				ageBadgeEn = "⚠️ Used yesterday — check before deleting"
			} else {
				ageBadgeRu = fmt.Sprintf("⚠️ Использовалось недавно (%d дн. назад) — проверьте перед удалением", daysAgo)
				ageBadgeEn = fmt.Sprintf("⚠️ Used recently (%d days ago) — check before deleting", daysAgo)
			}
		} else if daysAgo > 30 {
			ageStatus = "safe"
			ageBadgeRu = fmt.Sprintf("✓ Не использовалось %d дн. — можно безопасно очистить", daysAgo)
			ageBadgeEn = fmt.Sprintf("✓ Not used for %d days — safe to clean", daysAgo)
		} else {
			ageBadgeRu = fmt.Sprintf("Не использовалось %d дн.", daysAgo)
			ageBadgeEn = fmt.Sprintf("Not used for %d days", daysAgo)
		}

		driveLetter := strings.ToUpper(strings.TrimSuffix(filepath.VolumeName(p), ":"))
		if driveLetter == "" {
			driveLetter = "C"
		}

		out = append(out, OrphanLeftover{
			Path:        p,
			Name:        name,
			DriveLetter: driveLetter,
			SizeBytes:   sz,
			SizeLabel:   formatSizeLabel(sz),
			RootKind:    kind,
			LastModTime: modTime,
			DaysAgo:     daysAgo,
			AgeBadgeRu:  ageBadgeRu,
			AgeBadgeEn:  ageBadgeEn,
			AgeStatus:   ageStatus,
		})
	}

	scanDir := func(root, kind string) {
		if root == "" {
			return
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			key := strings.ToLower(strings.TrimSpace(name))
			if key == "" || leftoverProtectedNames[key] || strings.HasPrefix(key, ".") || strings.HasPrefix(key, "$") {
				continue
			}
			if matchesInstalledApp(key, names) {
				continue
			}
			p := filepath.Join(root, name)
			scanItem(p, name, kind)
		}
	}

	scanDir(os.Getenv("LOCALAPPDATA"), "local")
	scanDir(os.Getenv("APPDATA"), "roaming")
	scanDir(os.Getenv("PROGRAMDATA"), "programdata")

	// Also scan local drives roots for forgotten app/game folders
	for _, d := range getLocalDrivesNative() {
		root := d.Mount
		if root == "" {
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			key := strings.ToLower(strings.TrimSpace(name))
			if key == "" || leftoverProtectedNames[key] || strings.HasPrefix(key, ".") || strings.HasPrefix(key, "$") {
				continue
			}
			switch key {
			case "windows", "users", "perflogs", "program files", "program files (x86)", "programdata",
				"system volume information", "recovery", "msocache", "config.msi", "boot", "efi":
				continue
			}
			p := filepath.Join(root, name)
			// For game hubs like D:\Games, scan subdirectories
			if key == "games" || key == "steambackup" || key == "steamlibrary" {
				subEntries, err := os.ReadDir(p)
				if err == nil {
					for _, subE := range subEntries {
						if !subE.IsDir() {
							continue
						}
						subName := subE.Name()
						subKey := strings.ToLower(strings.TrimSpace(subName))
						if subKey == "" || matchesInstalledApp(subKey, names) {
							continue
						}
						scanItem(filepath.Join(p, subName), subName, "drive_games")
					}
				}
				continue
			}
			if matchesInstalledApp(key, names) {
				continue
			}
			scanItem(p, name, "drive_root")
		}
	}

	// Sort: non-recent first, recent last; inside group - by daysAgo descending then size descending
	sort.Slice(out, func(i, j int) bool {
		iRecent := out[i].AgeStatus == "recent"
		jRecent := out[j].AgeStatus == "recent"
		if iRecent != jRecent {
			return !iRecent
		}
		if out[i].DaysAgo != out[j].DaysAgo {
			return out[i].DaysAgo > out[j].DaysAgo
		}
		return out[i].SizeBytes > out[j].SizeBytes
	})

	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

func installedNameSet(apps []InstalledApp) map[string]bool {
	out := map[string]bool{}
	for _, a := range apps {
		for _, n := range candidateFolderNames(a.DisplayName) {
			out[strings.ToLower(n)] = true
		}
		if pub := strings.ToLower(strings.TrimSpace(a.Publisher)); len(pub) >= 3 {
			out[pub] = true
			if i := strings.IndexAny(pub, " ,."); i > 2 {
				out[pub[:i]] = true
			}
		}
		if loc := strings.TrimSpace(a.InstallLocation); loc != "" {
			base := strings.ToLower(filepath.Base(loc))
			if len(base) >= 3 {
				out[base] = true
			}
		}
	}
	return out
}

func matchesInstalledApp(folderKey string, names map[string]bool) bool {
	if names[folderKey] {
		return true
	}
	for n := range names {
		if n == "" {
			continue
		}
		if strings.Contains(folderKey, n) || strings.Contains(n, folderKey) {
			if len(n) >= 4 && len(folderKey) >= 4 {
				return true
			}
		}
	}
	return false
}

// StartRemoveOrphanLeftover safely moves one orphan folder to Windows Recycle Bin.
func (a *App) StartRemoveOrphanLeftover(path string) map[string]interface{} {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || !isSafeAppLeftoverPath(path) {
		return map[string]interface{}{"started": false, "message": "unsafe path"}
	}
	label := filepath.Base(path)
	jobID := fmt.Sprintf("orf-%d", deleteJobSeq.Add(1))
	go func() {
		deleteMu.Lock()
		defer deleteMu.Unlock()
		a.emitDeleteProgress(deleteProgress{JobID: jobID, Phase: "started", Percent: 8, Label: label, Path: path, OK: true})
		var reported int64
		_, rec := a.MeasureAndRecordClean("C:", "AppLeftover", func() int64 {
			info, err := os.Stat(path)
			if err == nil && info.IsDir() {
				reported = dirSizeBudget(path, time.Now().Add(800*time.Millisecond), 5)
			}
			// Safe delete to Windows Recycle Bin with undo capability
			_ = recycleFileOrDir(path)
			return reported
		})

		// Track in trashed slice for in-app restoration
		trashedMu.Lock()
		trashedLeftovers = append(trashedLeftovers, TrashedLeftover{
			Path:       path,
			Name:       label,
			SizeBytes:  reported,
			DeleteTime: time.Now(),
		})
		trashedMu.Unlock()

		a.RefreshDiskMetricsEx()
		freed := int64(rec.BytesFreed)
		if freed <= 0 {
			freed = reported
		}

		// Re-query recycle bin size immediately
		binBytes := queryRecycleBinBytes()
		runtime.EventsEmit(a.ctx, "recycle-bin-updated", map[string]interface{}{
			"bytes":       binBytes,
			"lastTrashed": label,
			"path":        path,
		})

		a.emitDeleteProgress(deleteProgress{
			JobID: jobID, Phase: "done", Percent: 100, Label: label, Path: path,
			FreedBytes: freed, OK: true, Message: "trashed",
		})
	}()
	return map[string]interface{}{"started": true, "jobId": jobID, "name": label}
}

// RestoreLastTrashedItem restores the most recently trashed folder from Windows Recycle Bin.
func (a *App) RestoreLastTrashedItem() (map[string]interface{}, error) {
	trashedMu.Lock()
	if len(trashedLeftovers) == 0 {
		trashedMu.Unlock()
		return map[string]interface{}{"success": false, "message": "no trashed items"}, fmt.Errorf("no trashed items to restore")
	}
	last := trashedLeftovers[len(trashedLeftovers)-1]
	trashedLeftovers = trashedLeftovers[:len(trashedLeftovers)-1]
	trashedMu.Unlock()

	err := restoreFromRecycleBin(last.Name, last.Path)
	binBytes := queryRecycleBinBytes()
	if err != nil {
		return map[string]interface{}{
			"success":  false,
			"name":     last.Name,
			"path":     last.Path,
			"binBytes": binBytes,
			"message":  err.Error(),
		}, err
	}

	a.RefreshDiskMetricsEx()
	runtime.EventsEmit(a.ctx, "recycle-bin-updated", map[string]interface{}{
		"bytes":    binBytes,
		"restored": last.Name,
	})

	return map[string]interface{}{
		"success":  true,
		"name":     last.Name,
		"path":     last.Path,
		"binBytes": binBytes,
		"message":  "restored",
	}, nil
}

// OpenRecycleBin opens Windows Recycle Bin in Explorer.
func (a *App) OpenRecycleBin() error {
	return exec.Command("explorer.exe", "shell:RecycleBinFolder").Start()
}
