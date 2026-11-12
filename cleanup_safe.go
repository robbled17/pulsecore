package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CleanupCat is one safe cleanup category with measured size.
type CleanupCat struct {
	ID           string `json:"id"`
	SizeBytes    int64  `json:"sizeBytes"`
	NeedsAdmin   bool   `json:"needsAdmin"`
	Irreversible bool   `json:"irreversible"`
	Available    bool   `json:"available"`
	Selected     bool   `json:"selected"`
}

// CleanupResult is the outcome of ApplyDiskCleanup.
type CleanupResult struct {
	OK                 bool     `json:"ok"`
	FreedBytes         int64    `json:"freedBytes"`
	ActualCleanedBytes int64    `json:"actualCleanedBytes"`
	Categories         []string `json:"categories"`
	UsedBefore         uint64   `json:"usedBefore"`
	UsedAfter          uint64   `json:"usedAfter"`
	FreeBefore         uint64   `json:"freeBefore"`
	FreeAfter          uint64   `json:"freeAfter"`
	TotalBytes         uint64   `json:"totalBytes"`
}

// LargeFolder kept for API compat (UI now opens Storage Sense).
type LargeFolder struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
}

// Order matters for preview UI grouping.
var cleanupSafeIDs = []string{
	"shader_cache",
	"recycle",
	"app_cache",
	"user_temp",
	"browser_cache",
	"thumbnails",
	"wer_reports",
	"package_cache",
}

// PreviewDiskCleanup measures safe categories (no deletion).
func (a *App) PreviewDiskCleanup() []CleanupCat {
	out := make([]CleanupCat, 0, len(cleanupSafeIDs))
	for _, id := range cleanupSafeIDs {
		cat := CleanupCat{ID: id, Available: true, Selected: true}
		switch id {
		case "shader_cache":
			cat.SizeBytes = dirSizeLimited(shaderCacheDirs(), 4, time.Time{})
		case "recycle":
			cat.Irreversible = true
			cat.SizeBytes = recycleBinSizeApprox()
		case "app_cache":
			cat.SizeBytes = dirSizeLimited(appCacheDirs(), 4, time.Time{})
		case "user_temp":
			cat.SizeBytes = userTempCleanableSize()
		case "browser_cache":
			cat.SizeBytes = browserCacheSize()
		case "thumbnails":
			cat.SizeBytes = thumbcacheSize()
		case "wer_reports":
			cat.SizeBytes = dirSizeLimited(werDirs(), 4, time.Time{})
		case "package_cache":
			cat.SizeBytes = dirSizeLimited(packageCacheDirs(), 3, time.Time{})
		}
		// Hide empty categories from default selection noise, but still list them.
		if cat.SizeBytes <= 0 {
			cat.Selected = false
		}
		out = append(out, cat)
	}
	return out
}

// ApplyDiskCleanup deletes selected safe categories. Empty cats → all selected-by-default non-admin.
func (a *App) ApplyDiskCleanup(cats []string) CleanupResult {
	allowed := map[string]bool{}
	if len(cats) == 0 {
		for _, c := range a.PreviewDiskCleanup() {
			if c.Selected && !c.NeedsAdmin {
				allowed[c.ID] = true
			}
		}
		if len(allowed) == 0 {
			for _, id := range cleanupSafeIDs {
				allowed[id] = true
			}
		}
	} else {
		for _, c := range cats {
			c = strings.TrimSpace(c)
			if c != "" {
				allowed[c] = true
			}
		}
	}

	category := cleanupCategoryLabel(allowed)
	var done []string
	var freed int64
	var rec CleanRecord
	if a != nil {
		freed, rec = a.MeasureAndRecordClean("C:", category, func() int64 {
			var n int64
			done = done[:0]
			for _, id := range cleanupSafeIDs {
				if !allowed[id] {
					continue
				}
				var part int64
				switch id {
				case "shader_cache":
					part = cleanDirsAll(shaderCacheDirs(), 5)
				case "recycle":
					part = emptyRecycleBin()
				case "app_cache":
					part = cleanDirsAll(appCacheDirs(), 5)
				case "user_temp":
					part = cleanUserTemp()
				case "browser_cache":
					part = cleanBrowserCaches()
				case "thumbnails":
					part = cleanThumbcaches()
				case "wer_reports":
					part = cleanDirsAll(werDirs(), 4)
				case "package_cache":
					part = cleanDirsAll(packageCacheDirs(), 4)
				}
				if part > 0 {
					n += part
					done = append(done, id)
				}
			}
			return n
		})
		a.RefreshDiskMetricsEx()
	} else {
		for _, id := range cleanupSafeIDs {
			if !allowed[id] {
				continue
			}
			var part int64
			switch id {
			case "shader_cache":
				part = cleanDirsAll(shaderCacheDirs(), 5)
			case "recycle":
				part = emptyRecycleBin()
			case "app_cache":
				part = cleanDirsAll(appCacheDirs(), 5)
			case "user_temp":
				part = cleanUserTemp()
			case "browser_cache":
				part = cleanBrowserCaches()
			case "thumbnails":
				part = cleanThumbcaches()
			case "wer_reports":
				part = cleanDirsAll(werDirs(), 4)
			case "package_cache":
				part = cleanDirsAll(packageCacheDirs(), 4)
			}
			if part > 0 {
				freed += part
				done = append(done, id)
			}
		}
	}

	actual := freed
	if rec.BytesFreed > 0 {
		actual = int64(rec.BytesFreed)
	}
	usedBefore, usedAfter := uint64(0), uint64(0)
	if rec.TotalBytes > rec.BytesBefore {
		usedBefore = rec.TotalBytes - rec.BytesBefore
	}
	if rec.TotalBytes > rec.BytesAfter {
		usedAfter = rec.TotalBytes - rec.BytesAfter
	}
	return CleanupResult{
		OK:                 actual > 0 || len(done) > 0,
		FreedBytes:         actual,
		ActualCleanedBytes: actual,
		Categories:         done,
		UsedBefore:         usedBefore,
		UsedAfter:          usedAfter,
		FreeBefore:         rec.BytesBefore,
		FreeAfter:          rec.BytesAfter,
		TotalBytes:         rec.TotalBytes,
	}
}

func cleanupCategoryLabel(allowed map[string]bool) string {
	if allowed["shader_cache"] && len(allowed) == 1 {
		return "Shaders"
	}
	if allowed["recycle"] && len(allowed) == 1 {
		return "Recycle"
	}
	return "Cache"
}

// RefreshDiskMetrics force-updates cached volume usage (call after cleanup).
func (a *App) RefreshDiskMetrics() []DiskMetrics {
	now := time.Now()
	disks := a.collectDisks(now)
	a.mu.Lock()
	a.cachedDisks = append([]DiskMetrics(nil), disks...)
	a.metrics.Disks = append([]DiskMetrics(nil), disks...)
	a.lastDisk = time.Time{} // expire cache throttle
	a.mu.Unlock()
	return disks
}

// OpenStorageSense opens Windows Storage settings (large folders analysis).
func (a *App) OpenStorageSense() error {
	// Native Storage Sense page — fast folder breakdown on Windows 11.
	if err := runHiddenStart("cmd.exe", "/c", "start", "", "ms-settings:storagesense"); err != nil {
		return a.openSettingsURL("ms-settings:storagesense")
	}
	return nil
}

// ScanLargeFolders kept for binding compat — prefer OpenStorageSense from UI.
func (a *App) ScanLargeFolders(drive string) []LargeFolder {
	_ = drive
	_ = a.OpenStorageSense()
	return nil
}

func shaderCacheDirs() []string {
	la := os.Getenv("LOCALAPPDATA")
	if la == "" {
		return nil
	}
	return uniqueStrings([]string{
		filepath.Join(la, "NVIDIA", "DXCache"),
		filepath.Join(la, "NVIDIA", "GLCache"),
		filepath.Join(la, "D3DSCache"),
		filepath.Join(la, "AMD", "DxCache"),
		filepath.Join(la, "AMD", "GLCache"),
		filepath.Join(la, "Intel", "ShaderCache"),
	})
}

func appCacheDirs() []string {
	la := os.Getenv("LOCALAPPDATA")
	ro := os.Getenv("APPDATA")
	out := []string{}
	if ro != "" {
		out = append(out,
			filepath.Join(ro, "discord", "Cache"),
			filepath.Join(ro, "discord", "Code Cache"),
			filepath.Join(ro, "discord", "GPUCache"),
			filepath.Join(ro, "Telegram Desktop", "tdata", "user_data", "cache"),
			filepath.Join(ro, "Telegram Desktop", "tdata", "user_data", "media_cache"),
		)
	}
	if la != "" {
		out = append(out,
			filepath.Join(la, "Spotify", "Storage"),
			filepath.Join(la, "Spotify", "Data"),
		)
	}
	return uniqueStrings(out)
}

func packageCacheDirs() []string {
	out := []string{}
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		out = append(out,
			filepath.Join(la, "npm-cache"),
			filepath.Join(la, "pip", "Cache"),
			filepath.Join(la, "pypa", "pip", "Cache"),
		)
	}
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out,
			filepath.Join(home, ".npm", "_cacache"),
			filepath.Join(home, ".cache", "pip"),
			filepath.Join(home, ".cargo", "registry", "cache"),
			filepath.Join(home, "AppData", "Local", "Cargo", "registry", "cache"),
		)
	}
	return uniqueStrings(out)
}

func userTempDirs() []string {
	out := []string{}
	if t := os.TempDir(); t != "" {
		out = append(out, t)
	}
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		out = append(out, filepath.Join(la, "Temp"))
		out = append(out, filepath.Join(la, "CrashDumps"))
	}
	return uniqueStrings(out)
}

func werDirs() []string {
	out := []string{}
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		out = append(out, filepath.Join(la, "Microsoft", "Windows", "WER"))
	}
	out = append(out, `C:\ProgramData\Microsoft\Windows\WER`)
	return uniqueStrings(out)
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = filepath.Clean(s)
		if s == "" || seen[strings.ToLower(s)] {
			continue
		}
		if _, err := os.Stat(s); err != nil {
			continue // skip missing roots from preview noise
		}
		seen[strings.ToLower(s)] = true
		out = append(out, s)
	}
	return out
}

func canWriteDir(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	f := filepath.Join(dir, ".pulsecore_write_test")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		return false
	}
	_ = os.Remove(f)
	return true
}

func isSafeCleanupPath(p string) bool {
	p = strings.ToLower(filepath.Clean(p))
	blocked := []string{
		`\documents\`, `\downloads\`, `\desktop\`, `\pictures\`, `\videos\`, `\music\`,
		`\ntuser`, `\cookies\`, `\login data`, `\history`, `\leveldb`, `\indexeddb`,
		`\local storage`, `\session storage`,
	}
	for _, b := range blocked {
		if strings.Contains(p, b) {
			return false
		}
	}
	temp := strings.ToLower(filepath.Clean(os.TempDir()))
	la := strings.ToLower(filepath.Clean(os.Getenv("LOCALAPPDATA")))
	ro := strings.ToLower(filepath.Clean(os.Getenv("APPDATA")))
	home, _ := os.UserHomeDir()
	home = strings.ToLower(filepath.Clean(home))

	safePrefixes := []string{
		temp,
		filepath.Join(la, "temp"),
		filepath.Join(la, "crashdumps"),
		filepath.Join(la, "nvidia", "dxcache"),
		filepath.Join(la, "nvidia", "glcache"),
		filepath.Join(la, "d3dscache"),
		filepath.Join(la, "amd", "dxcache"),
		filepath.Join(la, "amd", "glcache"),
		filepath.Join(la, "intel", "shadercache"),
		filepath.Join(la, "spotify", "storage"),
		filepath.Join(la, "spotify", "data"),
		filepath.Join(la, "microsoft", "windows", "explorer"),
		filepath.Join(la, "microsoft", "windows", "wer"),
		filepath.Join(la, "google", "chrome", "user data", "default", "cache"),
		filepath.Join(la, "google", "chrome", "user data", "default", "code cache"),
		filepath.Join(la, "google", "chrome", "user data", "default", "gpucache"),
		filepath.Join(la, "microsoft", "edge", "user data", "default", "cache"),
		filepath.Join(la, "microsoft", "edge", "user data", "default", "code cache"),
		filepath.Join(la, "microsoft", "edge", "user data", "default", "gpucache"),
		filepath.Join(la, "mozilla", "firefox", "profiles"),
		filepath.Join(la, "npm-cache"),
		filepath.Join(la, "pip", "cache"),
		filepath.Join(la, "pypa", "pip", "cache"),
		filepath.Join(la, "cargo", "registry", "cache"),
		filepath.Join(ro, "discord", "cache"),
		filepath.Join(ro, "discord", "code cache"),
		filepath.Join(ro, "discord", "gpucache"),
		filepath.Join(ro, "telegram desktop", "tdata", "user_data", "cache"),
		filepath.Join(ro, "telegram desktop", "tdata", "user_data", "media_cache"),
		filepath.Join(home, ".npm", "_cacache"),
		filepath.Join(home, ".cache", "pip"),
		filepath.Join(home, ".cargo", "registry", "cache"),
		`c:\programdata\microsoft\windows\wer`,
		`c:\$recycle.bin`,
	}
	for _, pref := range safePrefixes {
		pref = strings.ToLower(filepath.Clean(pref))
		if pref == "" || pref == "." {
			continue
		}
		if p == pref || strings.HasPrefix(p, pref+string(os.PathSeparator)) || strings.HasPrefix(p, pref+"\\") {
			return true
		}
	}
	if strings.Contains(p, `\mozilla\firefox\profiles\`) && strings.Contains(p, `\cache2`) {
		return true
	}
	return false
}

func dirSizeLimited(roots []string, maxDepth int, olderThan time.Time) int64 {
	var total int64
	for _, root := range roots {
		total += walkClean(root, maxDepth, olderThan, false, nil)
	}
	return total
}

func userTempCleanableSize() int64 {
	var total int64
	for _, root := range userTempDirs() {
		total += walkClean(root, 2, time.Time{}, false, isTempCleanCandidate)
	}
	return total
}

func cleanUserTemp() int64 {
	var n int64
	for _, root := range userTempDirs() {
		n += walkClean(root, 2, time.Time{}, true, isTempCleanCandidate)
	}
	return n
}

func isTempCleanCandidate(path string, info os.FileInfo) bool {
	name := strings.ToLower(info.Name())
	if strings.HasSuffix(name, ".dmp") || strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".tmp") {
		return true
	}
	// Older general temp files (>1h) — skip very fresh locks.
	return time.Since(info.ModTime()) > time.Hour
}

func walkClean(root string, maxDepth int, olderThan time.Time, delete bool, filter func(string, os.FileInfo) bool) int64 {
	if root == "" {
		return 0
	}
	var total int64
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Access denied / busy dir — skip silently.
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if maxDepth >= 0 && path != root {
				rel, _ := filepath.Rel(root, path)
				depth := strings.Count(rel, string(os.PathSeparator)) + 1
				if depth > maxDepth {
					return fs.SkipDir
				}
			}
			return nil
		}
		if !isSafeCleanupPath(path) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if !olderThan.IsZero() && info.ModTime().After(olderThan) {
			return nil
		}
		if filter != nil && !filter(path, info) {
			return nil
		}
		sz := info.Size()
		if !delete {
			total += sz
			return nil
		}
		if safeRemoveFile(path) {
			total += sz
		}
		return nil
	})
	return total
}

func safeRemoveFile(path string) bool {
	err := os.Remove(path)
	if err == nil {
		return true
	}
	// Busy / access denied — skip silently.
	return false
}

func cleanDirsAll(roots []string, maxDepth int) int64 {
	var n int64
	for _, r := range roots {
		n += walkClean(r, maxDepth, time.Time{}, true, nil)
	}
	return n
}

func thumbcacheSize() int64 {
	la := os.Getenv("LOCALAPPDATA")
	if la == "" {
		return 0
	}
	dir := filepath.Join(la, "Microsoft", "Windows", "Explorer")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var n int64
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if strings.HasPrefix(name, "thumbcache_") && strings.HasSuffix(name, ".db") {
			info, err := e.Info()
			if err == nil {
				n += info.Size()
			}
		}
	}
	return n
}

func cleanThumbcaches() int64 {
	la := os.Getenv("LOCALAPPDATA")
	if la == "" {
		return 0
	}
	dir := filepath.Join(la, "Microsoft", "Windows", "Explorer")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var n int64
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if !strings.HasPrefix(name, "thumbcache_") || !strings.HasSuffix(name, ".db") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if !isSafeCleanupPath(p) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if safeRemoveFile(p) {
			n += info.Size()
		}
	}
	return n
}

func browserCacheDirs() []string {
	la := os.Getenv("LOCALAPPDATA")
	if la == "" {
		return nil
	}
	out := []string{
		filepath.Join(la, "Google", "Chrome", "User Data", "Default", "Cache"),
		filepath.Join(la, "Google", "Chrome", "User Data", "Default", "Code Cache"),
		filepath.Join(la, "Google", "Chrome", "User Data", "Default", "GPUCache"),
		filepath.Join(la, "Microsoft", "Edge", "User Data", "Default", "Cache"),
		filepath.Join(la, "Microsoft", "Edge", "User Data", "Default", "Code Cache"),
		filepath.Join(la, "Microsoft", "Edge", "User Data", "Default", "GPUCache"),
	}
	ff := filepath.Join(la, "Mozilla", "Firefox", "Profiles")
	entries, err := os.ReadDir(ff)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				out = append(out, filepath.Join(ff, e.Name(), "cache2"))
			}
		}
	}
	return uniqueStrings(out)
}

func browserCacheSize() int64 {
	return dirSizeLimited(browserCacheDirs(), 4, time.Time{})
}

func cleanBrowserCaches() int64 {
	return cleanDirsAll(browserCacheDirs(), 4)
}

func recycleBinSizeApprox() int64 {
	root := `C:\$Recycle.Bin`
	return dirSizeBudget(root, time.Now().Add(1200*time.Millisecond), 2)
}

func dirSizeBudget(root string, deadline time.Time, maxDepth int) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || time.Now().After(deadline) {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			rel, _ := filepath.Rel(root, path)
			if rel != "." && maxDepth >= 0 {
				depth := 0
				if rel != "" {
					depth = strings.Count(rel, string(os.PathSeparator)) + 1
				}
				if depth > maxDepth {
					return fs.SkipDir
				}
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

func emptyRecycleBin() int64 {
	before := recycleBinSizeApprox()
	_ = emptyRecycleBinNative()
	// Brief settle for FS accounting.
	time.Sleep(400 * time.Millisecond)
	after := recycleBinSizeApprox()
	if before > after {
		return before - after
	}
	// Native call may have succeeded even if size probe failed.
	if before > 0 {
		return before
	}
	return 0
}

func isBlockedFolderName(name string) bool {
	n := strings.ToLower(name)
	switch n {
	case "documents", "downloads", "desktop", "pictures", "videos", "music",
		"onedrive", "appdata", "system32", "syswow64", "winsxs":
		return true
	}
	return false
}
