package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const orphanLeftoverMinBytes = 100 * 1024 * 1024 // 100 MB

// OrphanLeftover is a large AppData/ProgramData folder not matching any installed app.
type OrphanLeftover struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
	SizeLabel string `json:"sizeLabel"`
	RootKind  string `json:"rootKind"` // local|roaming|programdata
}

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

// ListOrphanLeftovers finds large leftover folders from uninstalled software (no registry writes).
func (a *App) ListOrphanLeftovers() []OrphanLeftover {
	installed := a.ListInstalledApps()
	names := installedNameSet(installed)
	out := make([]OrphanLeftover, 0, 16)
	scan := func(root, kind string) {
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
			if key == "" || leftoverProtectedNames[key] || strings.HasPrefix(key, ".") {
				continue
			}
			if matchesInstalledApp(key, names) {
				continue
			}
			p := filepath.Join(root, name)
			if !isSafeAppLeftoverPath(p) {
				continue
			}
			sz := dirSizeBudget(p, time.Now().Add(700*time.Millisecond), 3)
			if sz < orphanLeftoverMinBytes {
				continue
			}
			out = append(out, OrphanLeftover{
				Path: p, Name: name, SizeBytes: sz,
				SizeLabel: formatSizeLabel(sz), RootKind: kind,
			})
		}
	}
	scan(os.Getenv("LOCALAPPDATA"), "local")
	scan(os.Getenv("APPDATA"), "roaming")
	scan(os.Getenv("PROGRAMDATA"), "programdata")

	// Heaviest first.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].SizeBytes > out[i].SizeBytes {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
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
			// first token of publisher
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
			// Avoid ultra-short false positives.
			if len(n) >= 4 && len(folderKey) >= 4 {
				return true
			}
		}
	}
	return false
}

// StartRemoveOrphanLeftover deletes one orphan folder asynchronously with history.
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
			_ = ForceDeleteAll(path)
			return reported
		})
		a.RefreshDiskMetricsEx()
		freed := int64(rec.BytesFreed)
		if freed <= 0 {
			freed = reported
		}
		a.emitDeleteProgress(deleteProgress{
			JobID: jobID, Phase: "done", Percent: 100, Label: label, Path: path,
			FreedBytes: freed, OK: true, Message: "ok",
		})
	}()
	return map[string]interface{}{"started": true, "jobId": jobID, "name": label}
}
