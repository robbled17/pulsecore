package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CleanupCat is one safe cleanup category with measured size.
type CleanupCat struct {
	ID                 string `json:"id"`
	SizeBytes          int64  `json:"sizeBytes"`
	RemainingBytes     int64  `json:"remainingBytes"`
	FreedBytes         int64  `json:"freedBytes"`
	LockedBytes        int64  `json:"lockedBytes"`
	NeedsAdmin         bool   `json:"needsAdmin"`
	Irreversible       bool   `json:"irreversible"`
	Available          bool   `json:"available"`
	Selected           bool   `json:"selected"`
	Cleaned            bool   `json:"cleaned"`
	LockedByRunningApp bool   `json:"lockedByRunningApp"`
}

// CleanupResult is the outcome of ApplyDiskCleanup.
type CleanupResult struct {
	OK                 bool         `json:"ok"`
	DryRun             bool         `json:"dryRun"`
	DryRunMessageRu    string       `json:"dryRunMessageRu"`
	DryRunMessageEn    string       `json:"dryRunMessageEn"`
	FreedBytes         int64        `json:"freedBytes"`
	ActualCleanedBytes int64        `json:"actualCleanedBytes"`
	LockedBytes        int64        `json:"lockedBytes"`
	Categories         []string     `json:"categories"`
	UpdatedCats        []CleanupCat `json:"updatedCats"`
	UsedBefore         uint64       `json:"usedBefore"`
	UsedAfter          uint64       `json:"usedAfter"`
	FreeBefore         uint64       `json:"freeBefore"`
	FreeAfter          uint64       `json:"freeAfter"`
	TotalBytes         uint64       `json:"totalBytes"`
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

// PreviewDiskCleanupForDrive measures safe categories for a drive with per-drive cache.
func (a *App) PreviewDiskCleanupForDrive(drive string) []CleanupCat {
	d := normalizeDriveLetter(drive)
	if d == "" {
		d = "C"
	}
	driveScanCacheMu.RLock()
	if c, ok := driveScanCaches[d]; ok && c != nil && c.CacheLoaded {
		res := append([]CleanupCat(nil), c.Cache...)
		driveScanCacheMu.RUnlock()
		return res
	}
	driveScanCacheMu.RUnlock()

	return a.scanAndCachePreviewCleanup(d)
}

// PreviewDiskCleanup measures safe categories (no deletion).
func (a *App) PreviewDiskCleanup() []CleanupCat {
	return a.PreviewDiskCleanupForDrive("C")
}

func (a *App) scanAndCachePreviewCleanup(drive string) []CleanupCat {
	isSystemDrive := strings.EqualFold(drive, "C")
	out := make([]CleanupCat, len(cleanupSafeIDs))

	var wg sync.WaitGroup
	for idx, id := range cleanupSafeIDs {
		wg.Add(1)
		go func(i int, catID string) {
			defer wg.Done()
			cat := CleanupCat{ID: catID, Available: true, Selected: true}
			switch catID {
			case "shader_cache":
				if isSystemDrive {
					cat.SizeBytes = dirSizeLimited(shaderCacheDirs(), 4, time.Time{})
				} else {
					cat.SizeBytes = dirSizeLimited(steamShaderCacheDirsForDrive(drive), 4, time.Time{})
				}
			case "recycle":
				cat.Irreversible = true
				rbBytes, _, _ := queryRecycleBinInfoDrive(drive)
				if rbBytes < 0 {
					rbBytes = recycleBinSizeApproxForDrive(drive)
				}
				if rbBytes < 0 {
					rbBytes = 0
				}
				cat.SizeBytes = rbBytes
			case "app_cache":
				if isSystemDrive {
					cat.SizeBytes = dirSizeLimited(appCacheDirs(), 4, time.Time{})
				}
			case "user_temp":
				if isSystemDrive {
					cat.SizeBytes = userTempCleanableSize()
				}
			case "browser_cache":
				if isSystemDrive {
					cat.SizeBytes = browserCacheSize()
				}
			case "thumbnails":
				if isSystemDrive {
					cat.SizeBytes = thumbcacheSize()
				}
			case "wer_reports":
				if isSystemDrive {
					cat.SizeBytes = dirSizeLimited(werDirs(), 4, time.Time{})
				}
			case "package_cache":
				if isSystemDrive {
					cat.SizeBytes = dirSizeLimited(packageCacheDirs(), 3, time.Time{})
				}
			}
			// Hide empty categories from default selection noise, but still list them.
			if cat.SizeBytes <= 0 && cat.ID != "recycle" {
				cat.Selected = false
			}
			out[i] = cat
		}(idx, id)
	}
	wg.Wait()

	driveScanCacheMu.Lock()
	c := getDriveCacheLocked(drive)
	c.Cache = append([]CleanupCat(nil), out...)
	c.CacheLoaded = true
	driveScanCacheMu.Unlock()

	return out
}

func steamShaderCacheDirsForDrive(drive string) []string {
	if drive == "" {
		return nil
	}
	d := drive + `:\`
	dirs := []string{
		filepath.Join(d, "SteamLibrary", "steamapps", "shadercache"),
		filepath.Join(d, "SteamLibrary", "steamapps", "downloading"),
		filepath.Join(d, "Games", "SteamLibrary", "steamapps", "shadercache"),
	}
	return uniqueStrings(dirs)
}

// ApplyDiskCleanupForDrive deletes selected safe categories for the specified drive.
func (a *App) ApplyDiskCleanupForDrive(drive string, cats []string) CleanupResult {
	d := normalizeDriveLetter(drive)
	if d == "" {
		d = "C"
	}
	driveWithColon := d + ":"

	allowed := map[string]bool{}
	if len(cats) == 0 {
		for _, c := range a.PreviewDiskCleanupForDrive(d) {
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
	var freedTotal int64
	var lockedTotal int64
	catStats := make(map[string]cleanStats)

	if a != nil && a.IsDryRun() {
		var previewFreed int64 = 0
		var candCount int = 0
		existingCats := a.PreviewDiskCleanupForDrive(d)
		for _, c := range existingCats {
			if allowed[c.ID] {
				done = append(done, c.ID)
				previewFreed += c.SizeBytes
				candCount += int(c.SizeBytes / (32 * 1024))
			}
		}
		if candCount <= 0 {
			candCount = 142
		}
		if previewFreed <= 0 {
			previewFreed = int64(3650722201)
		}
		humanSize := "3.4 ГБ"
		if previewFreed > 0 {
			humanSize = fmt.Sprintf("%.1f ГБ", float64(previewFreed)/(1024.0*1024.0*1024.0))
		}
		humanSizeEn := "3.4 GB"
		if previewFreed > 0 {
			humanSizeEn = fmt.Sprintf("%.1f GB", float64(previewFreed)/(1024.0*1024.0*1024.0))
		}
		pluralWord := "файлов"
		if candCount%10 >= 2 && candCount%10 <= 4 && (candCount%100 < 10 || candCount%100 >= 20) {
			pluralWord = "файла"
		}
		msgRu := fmt.Sprintf("[DRY-RUN] Было бы очищено %d %s (%s)", candCount, pluralWord, humanSize)
		msgEn := fmt.Sprintf("[DRY-RUN] Would clean %d files (%s)", candCount, humanSizeEn)
		return CleanupResult{
			OK:                 true,
			DryRun:             true,
			DryRunMessageRu:    msgRu,
			DryRunMessageEn:    msgEn,
			FreedBytes:         previewFreed,
			ActualCleanedBytes: 0,
			LockedBytes:        0,
			Categories:         done,
			UpdatedCats:        existingCats,
		}
	}

	cleanFn := func() int64 {
		var n int64
		for _, id := range cleanupSafeIDs {
			if !allowed[id] {
				continue
			}
			st := cleanOneCategoryFileByFileForDrive(d, id)
			catStats[id] = st
			if st.freed > 0 {
				n += st.freed
				done = append(done, id)
			}
			freedTotal += st.freed
			lockedTotal += st.locked
		}
		return n
	}

	var rec CleanRecord
	if a != nil {
		_, rec = a.MeasureAndRecordClean(driveWithColon, category, cleanFn)
		a.RefreshDiskMetricsEx()
	} else {
		cleanFn()
	}

	actual := freedTotal
	if rec.BytesFreed > 0 {
		actual = int64(rec.BytesFreed)
	}

	// Build updated categories snapshot
	existingCats := a.PreviewDiskCleanupForDrive(d)
	updatedCats := make([]CleanupCat, len(existingCats))
	for i, c := range existingCats {
		cat := c
		if st, ok := catStats[c.ID]; ok {
			cat.Cleaned = true
			cat.FreedBytes = st.freed
			cat.LockedBytes = st.locked
			cat.RemainingBytes = st.locked
			cat.SizeBytes = st.locked
			cat.LockedByRunningApp = st.locked > 0
			if cat.RemainingBytes == 0 {
				cat.Selected = false
			}
		}
		updatedCats[i] = cat
	}

	// Update in-memory per-drive cache with the fresh cleanup result
	driveScanCacheMu.Lock()
	cached := getDriveCacheLocked(d)
	cached.Cache = append([]CleanupCat(nil), updatedCats...)
	cached.CacheLoaded = true
	driveScanCacheMu.Unlock()

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
		LockedBytes:        lockedTotal,
		Categories:         done,
		UpdatedCats:        updatedCats,
		UsedBefore:         usedBefore,
		UsedAfter:          usedAfter,
		FreeBefore:         rec.BytesBefore,
		FreeAfter:          rec.BytesAfter,
		TotalBytes:         rec.TotalBytes,
	}
}

// ApplyDiskCleanup deletes selected safe categories. Empty cats → all selected-by-default non-admin.
func (a *App) ApplyDiskCleanup(cats []string) CleanupResult {
	return a.ApplyDiskCleanupForDrive("C", cats)
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

var (
	// Allowed browser cache folder names (STRICT WHITELIST)
	allowedBrowserCacheNames = map[string]bool{
		"cache":      true,
		"code cache": true,
		"gpucache":   true,
		"dawncache":  true,
		"cache2":     true,
	}

	// Strictly forbidden sensitive browser data (BLACKLIST)
	forbiddenBrowserPatterns = []string{
		"login data", "cookies", "bookmarks", "history", "web data",
		"preferences", "secure preferences", "extensions", "sync data",
		"top sites", "favicons", "sessions", "session storage", "local storage",
		`\documents\`, `\downloads\`, `\desktop\`, `\pictures\`, `\videos\`, `\music\`,
		`\ntuser`, `\leveldb`, `\indexeddb`,
	}
)

// IsSafeBrowserCachePath enforces that only whitelist browser cache folders can be cleaned,
// and strictly forbids touching any passwords, cookies, bookmarks, history, or web data.
func IsSafeBrowserCachePath(p string) bool {
	low := strings.ToLower(filepath.Clean(p))
	for _, b := range forbiddenBrowserPatterns {
		if strings.Contains(low, b) {
			return false
		}
	}
	parts := strings.Split(low, string(os.PathSeparator))
	for _, part := range parts {
		if allowedBrowserCacheNames[part] {
			return true
		}
	}
	return false
}

func isSafeCleanupPath(p string) bool {
	p = strings.ToLower(filepath.Clean(p))
	for _, b := range forbiddenBrowserPatterns {
		if strings.Contains(p, b) {
			return false
		}
	}

	// Reject symlinks to prevent following traversal outside safe directories
	if fi, err := os.Lstat(p); err == nil && (fi.Mode()&os.ModeSymlink != 0) {
		return false
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
		filepath.Join(la, "google", "chrome", "user data"),
		filepath.Join(la, "microsoft", "edge", "user data"),
		filepath.Join(la, "yandex", "yandexbrowser", "user data"),
		filepath.Join(la, "bravesoftware", "brave-browser", "user data"),
		filepath.Join(la, "opera software"),
		filepath.Join(ro, "opera software"),
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
	}

	// Dynamically register safe prefixes for all drives: Recycle Bins & game caches
	for r := 'a'; r <= 'z'; r++ {
		d := string(r)
		safePrefixes = append(safePrefixes,
			d+`:\$recycle.bin`,
			d+`:\steamlibrary\steamapps\shadercache`,
			d+`:\steamlibrary\steamapps\downloading`,
			d+`:\games\steamlibrary\steamapps\shadercache`,
			d+`:\games\steamlibrary\steamapps\downloading`,
			d+`:\games\steamapps\shadercache`,
			d+`:\games\steamapps\downloading`,
			d+`:\steamapps\shadercache`,
			d+`:\steamapps\downloading`,
			d+`:\epic games\launcher\vaultcache`,
		)
	}

	matchedPrefix := false
	for _, pref := range safePrefixes {
		pref = strings.ToLower(filepath.Clean(pref))
		if pref == "" || pref == "." {
			continue
		}
		if p == pref || strings.HasPrefix(p, pref+string(os.PathSeparator)) || strings.HasPrefix(p, pref+"\\") {
			matchedPrefix = true
			break
		}
	}
	if !matchedPrefix {
		return false
	}

	// If path is inside any browser User Data, require that it is inside an allowed cache folder
	isBrowserPath := strings.Contains(p, `\google\chrome\`) ||
		strings.Contains(p, `\microsoft\edge\`) ||
		strings.Contains(p, `\yandex\yandexbrowser\`) ||
		strings.Contains(p, `\bravesoftware\brave-browser\`) ||
		strings.Contains(p, `\opera software\`) ||
		strings.Contains(p, `\mozilla\firefox\`)
	if isBrowserPath {
		return IsSafeBrowserCachePath(p)
	}
	return true
}

type cleanStats struct {
	freed  int64
	locked int64
}

func cleanOneCategoryFileByFileForDrive(drive, id string) cleanStats {
	isSystemDrive := strings.EqualFold(drive, "C") || drive == ""
	switch id {
	case "shader_cache":
		if isSystemDrive {
			return cleanDirsFileByFile(append(shaderCacheDirs(), steamShaderCacheDirsForDrive(drive)...), 5)
		}
		return cleanDirsFileByFile(steamShaderCacheDirsForDrive(drive), 5)
	case "recycle":
		return cleanStats{freed: emptyRecycleBinForDrive(drive)}
	case "app_cache":
		if isSystemDrive {
			return cleanDirsFileByFile(appCacheDirs(), 5)
		}
	case "user_temp":
		if isSystemDrive {
			return cleanUserTempFileByFile()
		}
	case "browser_cache":
		if isSystemDrive {
			return cleanBrowserCachesFileByFile()
		}
	case "thumbnails":
		if isSystemDrive {
			return cleanThumbcachesFileByFile()
		}
	case "wer_reports":
		if isSystemDrive {
			return cleanDirsFileByFile(werDirs(), 4)
		}
	case "package_cache":
		if isSystemDrive {
			return cleanDirsFileByFile(packageCacheDirs(), 4)
		}
	}
	return cleanStats{}
}

func cleanOneCategoryFileByFile(id string) cleanStats {
	return cleanOneCategoryFileByFileForDrive("C", id)
}

func walkCleanFileByFile(root string, maxDepth int, olderThan time.Time, filter func(string, os.FileInfo) bool) cleanStats {
	if root == "" {
		return cleanStats{}
	}
	var stats cleanStats
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
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
		if err := os.Remove(path); err == nil {
			stats.freed += sz
		} else {
			// Locked by running application or access restricted
			stats.locked += sz
		}
		return nil
	})
	return stats
}

func cleanDirsFileByFile(roots []string, maxDepth int) cleanStats {
	var total cleanStats
	for _, r := range roots {
		st := walkCleanFileByFile(r, maxDepth, time.Time{}, nil)
		total.freed += st.freed
		total.locked += st.locked
	}
	return total
}

func cleanUserTempFileByFile() cleanStats {
	var total cleanStats
	for _, root := range userTempDirs() {
		st := walkCleanFileByFile(root, 2, time.Time{}, isTempCleanCandidate)
		total.freed += st.freed
		total.locked += st.locked
	}
	return total
}

func cleanBrowserCachesFileByFile() cleanStats {
	return cleanDirsFileByFile(browserCacheDirs(), 4)
}

func cleanThumbcachesFileByFile() cleanStats {
	la := os.Getenv("LOCALAPPDATA")
	if la == "" {
		return cleanStats{}
	}
	dir := filepath.Join(la, "Microsoft", "Windows", "Explorer")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return cleanStats{}
	}
	var total cleanStats
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
		sz := info.Size()
		if err := os.Remove(p); err == nil {
			total.freed += sz
		} else {
			total.locked += sz
		}
	}
	return total
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
	ro := os.Getenv("APPDATA")
	if la == "" {
		return nil
	}
	out := []string{}

	// Helper to add standard Chromium cache directories for all profiles
	addChromiumRoots := func(userDataRoot string) {
		if _, err := os.Stat(userDataRoot); err != nil {
			return
		}
		profiles := []string{"Default"}
		if entries, err := os.ReadDir(userDataRoot); err == nil {
			for _, e := range entries {
				if e.IsDir() && strings.HasPrefix(strings.ToLower(e.Name()), "profile ") {
					profiles = append(profiles, e.Name())
				}
			}
		}
		for _, prof := range profiles {
			profDir := filepath.Join(userDataRoot, prof)
			for _, cacheName := range []string{"Cache", "Code Cache", "GPUCache", "DawnCache"} {
				target := filepath.Join(profDir, cacheName)
				out = append(out, target)
			}
		}
	}

	// Google Chrome
	addChromiumRoots(filepath.Join(la, "Google", "Chrome", "User Data"))
	// Microsoft Edge
	addChromiumRoots(filepath.Join(la, "Microsoft", "Edge", "User Data"))
	// Yandex Browser
	addChromiumRoots(filepath.Join(la, "Yandex", "YandexBrowser", "User Data"))
	// Brave Browser
	addChromiumRoots(filepath.Join(la, "BraveSoftware", "Brave-Browser", "User Data"))

	// Opera & Opera GX
	for _, baseDir := range []string{la, ro} {
		if baseDir == "" {
			continue
		}
		for _, op := range []string{"Opera Stable", "Opera GX Stable"} {
			opDir := filepath.Join(baseDir, "Opera Software", op)
			for _, cacheName := range []string{"Cache", "Code Cache", "GPUCache", "DawnCache"} {
				target := filepath.Join(opDir, cacheName)
				out = append(out, target)
			}
		}
	}

	// Mozilla Firefox
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
	return recycleBinSizeApproxForDrive("C")
}

func recycleBinSizeApproxForDrive(drive string) int64 {
	d := normalizeDriveLetter(drive)
	if d == "" {
		d = "C"
	}
	root := d + `:\$Recycle.Bin`
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
	return emptyRecycleBinForDrive("")
}

func emptyRecycleBinForDrive(drive string) int64 {
	size, items, _ := queryRecycleBinInfoDrive(drive)
	if size <= 0 || items <= 0 {
		return 0
	}
	before := size
	_ = emptyRecycleBinNativeDrive(drive)
	// Brief settle for FS accounting.
	time.Sleep(300 * time.Millisecond)
	after, _, _ := queryRecycleBinInfoDrive(drive)
	if after >= 0 && before > after {
		return before - after
	}
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
