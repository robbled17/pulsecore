package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

const (
	tempMinAge   = 24 * time.Hour
	cacheMinAge  = 12 * time.Hour
	globalMinAge = 12 * time.Hour
)

// DiskCleanResult summarizes a safe temp/cache cleanup pass.
type DiskCleanResult struct {
	OK           bool     `json:"ok"`
	DeletedFiles int      `json:"deletedFiles"`
	DeletedBytes int64    `json:"deletedBytes"`
	Skipped      int      `json:"skipped"`
	Errors       int      `json:"errors"`
	Targets      []string `json:"targets"`
	Message      string   `json:"message"`
	DryRun       bool     `json:"dryRun"`
}

// CleanCategoryInfo describes one category in the human-friendly system clean UI.
type CleanCategoryInfo struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Files     int     `json:"files"`
	Bytes     int64   `json:"bytes"`
	SizeMB    float64 `json:"sizeMB"`
	Reachable bool    `json:"reachable"`
}

// CleanScanSummary summarizes parallel scanning of all system clean categories.
type CleanScanSummary struct {
	Categories []CleanCategoryInfo `json:"categories"`
	TotalFiles int                 `json:"totalFiles"`
	TotalBytes int64               `json:"totalBytes"`
	TotalMB    float64             `json:"totalMB"`
}

type cleanRootKind int

const (
	cleanKindTemp cleanRootKind = iota
	cleanKindCache
)

type cleanRoot struct {
	path     string
	kind     cleanRootKind
	category string // "temp", "browser", "directx"
	label    string
}

// ScanSystemClean performs a fast parallel scan of clean categories without deleting anything.
func (a *App) ScanSystemClean() CleanScanSummary {
	roots := listAllowedCleanRoots()
	type rootResult struct {
		root cleanRoot
		d    int
		b    int64
		s, e int
	}

	results := make([]rootResult, len(roots))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6) // parallel worker pool

	for i, r := range roots {
		wg.Add(1)
		go func(idx int, cr cleanRoot) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if cr.path == "" {
				return
			}
			d, b, s, e := cleanAllowedRoot(cr, true)
			results[idx] = rootResult{root: cr, d: d, b: b, s: s, e: e}
		}(i, r)
	}
	wg.Wait()

	catMap := map[string]*CleanCategoryInfo{
		"temp":    {ID: "temp", Title: "Временные файлы Windows", Reachable: true},
		"browser": {ID: "browser", Title: "Кэш браузеров", Reachable: true},
		"directx": {ID: "directx", Title: "Кэш шейдеров DirectX", Reachable: true},
	}

	var totalFiles int
	var totalBytes int64

	for _, res := range results {
		cKey := res.root.category
		if cKey == "" {
			cKey = "temp"
		}
		cat, ok := catMap[cKey]
		if !ok {
			cat = catMap["temp"]
		}
		cat.Files += res.d
		cat.Bytes += int64(res.b)
		totalFiles += res.d
		totalBytes += int64(res.b)
	}

	order := []string{"temp", "browser", "directx"}
	cats := make([]CleanCategoryInfo, 0, len(order))
	for _, id := range order {
		cat := catMap[id]
		cat.SizeMB = float64(cat.Bytes) / (1024 * 1024)
		cats = append(cats, *cat)
	}

	debug.FreeOSMemory()
	return CleanScanSummary{
		Categories: cats,
		TotalFiles: totalFiles,
		TotalBytes: totalBytes,
		TotalMB:    float64(totalBytes) / (1024 * 1024),
	}
}

// CleanSystemCategories deletes allowlisted files only for the selected category IDs.
func (a *App) CleanSystemCategories(selectedCategories []string) DiskCleanResult {
	filter := make(map[string]bool)
	for _, c := range selectedCategories {
		filter[strings.ToLower(strings.TrimSpace(c))] = true
	}
	if len(filter) == 0 {
		filter["temp"] = true
		filter["browser"] = true
		filter["directx"] = true
	}

	roots := listAllowedCleanRoots()
	var selectedRoots []cleanRoot
	for _, r := range roots {
		if filter[r.category] {
			selectedRoots = append(selectedRoots, r)
		}
	}

	type rootRes struct {
		label string
		d     int
		b     int64
		s, e  int
	}
	results := make([]rootRes, len(selectedRoots))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)

	for i, r := range selectedRoots {
		wg.Add(1)
		go func(idx int, cr cleanRoot) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if cr.path == "" {
				return
			}
			d, b, s, e := cleanAllowedRoot(cr, false)
			results[idx] = rootRes{label: cr.label, d: d, b: b, s: s, e: e}
		}(i, r)
	}
	wg.Wait()

	var deleted int
	var bytes int64
	var skipped, errs int
	labels := make([]string, 0, len(selectedRoots))

	for _, res := range results {
		labels = append(labels, res.label)
		deleted += res.d
		bytes += int64(res.b)
		skipped += res.s
		errs += res.e
	}

	msg := fmt.Sprintf("Очищено файлов: %d (%.1f МБ), пропущено: %d, ошибок: %d",
		deleted, float64(bytes)/(1024*1024), skipped, errs)
	debug.FreeOSMemory()
	return DiskCleanResult{
		OK:           errs == 0,
		DeletedFiles: deleted,
		DeletedBytes: bytes,
		Skipped:      skipped,
		Errors:       errs,
		Targets:      labels,
		Message:      msg,
		DryRun:       false,
	}
}

// CleanSafeDiskCaches deletes only allowlisted temp/cache files with age guards.
// Never removes the root folders themselves; never touches Packages or system trees.
func (a *App) CleanSafeDiskCaches() DiskCleanResult {
	return a.CleanSystemCategories([]string{"temp", "browser", "directx"})
}

// PreviewSafeDiskCaches is a dry-run of CleanSafeDiskCaches (no deletes).
func (a *App) PreviewSafeDiskCaches() DiskCleanResult {
	scan := a.ScanSystemClean()
	roots := listAllowedCleanRoots()
	labels := make([]string, 0, len(roots))
	for _, r := range roots {
		labels = append(labels, r.label)
	}
	return DiskCleanResult{
		OK:           true,
		DeletedFiles: scan.TotalFiles,
		DeletedBytes: scan.TotalBytes,
		Skipped:      0,
		Errors:       0,
		Targets:      labels,
		Message:      fmt.Sprintf("К удалению файлов: %d (%.1f МБ)", scan.TotalFiles, scan.TotalMB),
		DryRun:       true,
	}
}

func listAllowedCleanRoots() []cleanRoot {
	out := make([]cleanRoot, 0, 16)

	addTemp := func(p, label string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if isForbiddenCleanRoot(abs) {
			return
		}
		out = append(out, cleanRoot{path: abs, kind: cleanKindTemp, category: "temp", label: label})
	}
	addCache := func(p, category, label string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if isForbiddenCleanRoot(abs) {
			return
		}
		if !isAllowedD3DSCache(abs) && !isAllowedBrowserCacheDir(abs) {
			return
		}
		out = append(out, cleanRoot{path: abs, kind: cleanKindCache, category: category, label: label})
	}

	if t := os.Getenv("TEMP"); t != "" {
		addTemp(t, "%TEMP%")
	}
	if t := os.Getenv("TMP"); t != "" && !samePath(t, os.Getenv("TEMP")) {
		addTemp(t, "%TMP%")
	}
	addTemp(`C:\Windows\Temp`, `C:\Windows\Temp`)

	local := os.Getenv("LOCALAPPDATA")
	if local != "" {
		addCache(filepath.Join(local, "D3DSCache"), "directx", "D3DSCache")
		for _, browser := range browserUserDataRoots(local) {
			for _, profile := range listBrowserProfiles(browser) {
				for _, leaf := range []string{"Cache", "Code Cache", "GPUCache"} {
					p := filepath.Join(profile, leaf)
					if st, err := os.Stat(p); err == nil && st.IsDir() {
						addCache(p, "browser", filepath.Base(browser)+"/"+filepath.Base(profile)+"/"+leaf)
					}
				}
			}
		}
	}
	return out
}

func browserUserDataRoots(localAppData string) []string {
	candidates := []string{
		filepath.Join(localAppData, "Google", "Chrome", "User Data"),
		filepath.Join(localAppData, "Microsoft", "Edge", "User Data"),
		filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "User Data"),
		filepath.Join(localAppData, "Chromium", "User Data"),
		filepath.Join(localAppData, "Vivaldi", "User Data"),
		filepath.Join(localAppData, "Opera Software", "Opera Stable"),
	}
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			out = append(out, c)
		}
	}
	return out
}

func listBrowserProfiles(userData string) []string {
	entries, err := os.ReadDir(userData)
	if err != nil {
		return nil
	}
	out := make([]string, 0, 4)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		ln := strings.ToLower(name)
		if ln == "default" || strings.HasPrefix(ln, "profile ") {
			out = append(out, filepath.Join(userData, name))
		}
	}
	// Opera Stable is itself a profile root
	if len(out) == 0 {
		out = append(out, userData)
	}
	return out
}

func cleanAllowedRoot(root cleanRoot, dryRun bool) (deleted int, bytes int64, skipped, errs int) {
	info, err := os.Stat(root.path)
	if err != nil || !info.IsDir() {
		return 0, 0, 1, 0
	}
	// Never delete the allowlisted folder itself - only contents.
	minAge := tempMinAge
	if root.kind == cleanKindCache {
		minAge = cacheMinAge
	}
	if minAge < globalMinAge {
		minAge = globalMinAge
	}
	cutoff := time.Now().Add(-minAge)

	_ = filepath.WalkDir(root.path, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			skipped++
			return nil
		}
		if path == root.path {
			return nil
		}
		// Do not descend into directory symlinks/junctions (WalkDir already skips
		// symlinks on some platforms; re-check explicitly for Windows reparse points).
		if d.IsDir() {
			if isSymlinkPath(path, d) {
				skipped++
				return fs.SkipDir
			}
			if isForbiddenCleanPath(path) {
				skipped++
				return fs.SkipDir
			}
			return nil
		}
		if isForbiddenCleanPath(path) {
			skipped++
			return nil
		}
		if isSymlinkPath(path, d) {
			// File symlink: only delete if resolved target stays under allowlisted root.
			if _, ok := resolveDeletePath(path, root.path); !ok {
				skipped++
				return nil
			}
		}
		fi, err := d.Info()
		if err != nil {
			skipped++
			return nil
		}
		if !fi.ModTime().Before(cutoff) {
			skipped++
			return nil
		}
		if IsProtectedSystemTarget(path) {
			skipped++
			return nil
		}
		sz := fi.Size()
		if err := safeRemoveFile(path, root.path, dryRun); err != nil {
			// Open / locked files / escapes: skip, do not escalate.
			skipped++
			if !dryRun {
				errs++
			}
			return nil
		}
		deleted++
		bytes += sz
		return nil
	})
	return deleted, bytes, skipped, errs
}

func isSymlinkPath(path string, d fs.DirEntry) bool {
	if d != nil && d.Type()&fs.ModeSymlink != 0 {
		return true
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSymlink != 0
}

// isUnderLocalPackages forbids UWP/Store package trees under LOCALAPPDATA\Packages
// (and classic AppData\Local\Packages / WindowsApps paths), including test harnesses
// that redirect LOCALAPPDATA into a temp folder.
func isUnderLocalPackages(norm string) bool {
	if norm == "" {
		return false
	}
	local := normalizeProtectPath(os.Getenv("LOCALAPPDATA"))
	if local != "" {
		root := local + `\packages`
		if norm == root || strings.HasPrefix(norm, root+`\`) {
			return true
		}
	}
	if strings.Contains(norm, `\appdata\local\packages`) {
		return true
	}
	// Store install roots / Start Menu package hosts.
	if strings.Contains(norm, `\windowsapps\`) || strings.HasSuffix(norm, `\windowsapps`) {
		return true
	}
	if strings.Contains(norm, `\microsoft\windows\start menu`) {
		return true
	}
	return false
}

// isForbiddenCleanRoot blocks cleaning entire AppData / ProgramData / Windows / PF trees.
func isForbiddenCleanRoot(path string) bool {
	norm := normalizeProtectPath(path)
	if norm == "" || IsProtectedSystemTarget(path) {
		return true
	}
	if isUnderLocalPackages(norm) {
		return true
	}
	// Bare profile AppData roots (not a specific allowlisted leaf)
	forbiddenExact := []string{
		normalizeProtectPath(os.Getenv("LOCALAPPDATA")),
		normalizeProtectPath(os.Getenv("APPDATA")),
		normalizeProtectPath(os.Getenv("ProgramData")),
		`c:\windows`,
		`c:\program files`,
		`c:\program files (x86)`,
		`c:\users`,
	}
	for _, f := range forbiddenExact {
		if f != "" && norm == f {
			return true
		}
	}
	return false
}

// isForbiddenCleanPath rejects any file under Packages or outside allowlisted trees mid-walk.
func isForbiddenCleanPath(path string) bool {
	norm := normalizeProtectPath(path)
	if isUnderLocalPackages(norm) {
		return true
	}
	if IsProtectedSystemTarget(path) {
		// Temp under Windows\Temp is allowlisted; don't treat the Temp folder contents as System32.
		if isUnderWindowsTemp(norm) || isUnderUserTemp(norm) || isAllowedBrowserCacheDir(path) || isAllowedD3DSCache(path) {
			return false
		}
		return true
	}
	return false
}

func isUnderWindowsTemp(norm string) bool {
	return strings.HasPrefix(norm, `c:\windows\temp\`) || norm == `c:\windows\temp`
}

func isUnderUserTemp(norm string) bool {
	for _, env := range []string{"TEMP", "TMP"} {
		t := normalizeProtectPath(os.Getenv(env))
		if t == "" {
			continue
		}
		if norm == t || strings.HasPrefix(norm, t+`\`) {
			return true
		}
	}
	return false
}

func isAllowedBrowserCacheDir(path string) bool {
	norm := normalizeProtectPath(path)
	base := strings.ToLower(filepath.Base(path))
	if base != "cache" && base != "code cache" && base != "gpucache" {
		// Also allow nested files: parent dir must be one of these.
		parent := strings.ToLower(filepath.Base(filepath.Dir(path)))
		if parent != "cache" && parent != "code cache" && parent != "gpucache" {
			// deeper nesting under those dirs
			if !strings.Contains(norm, `\cache\`) &&
				!strings.Contains(norm, `\code cache\`) &&
				!strings.Contains(norm, `\gpucache\`) {
				return false
			}
		}
	}
	local := normalizeProtectPath(os.Getenv("LOCALAPPDATA"))
	if local == "" || !strings.HasPrefix(norm, local+`\`) {
		return false
	}
	if isUnderLocalPackages(norm) {
		return false
	}
	// Must live under a known browser User Data (or Opera Stable) tree.
	markers := []string{
		`\google\chrome\user data\`,
		`\microsoft\edge\user data\`,
		`\bravesoftware\brave-browser\user data\`,
		`\chromium\user data\`,
		`\vivaldi\user data\`,
		`\opera software\opera stable\`,
	}
	for _, m := range markers {
		if strings.Contains(norm, m) {
			return true
		}
	}
	return false
}

func isAllowedD3DSCache(path string) bool {
	norm := normalizeProtectPath(path)
	local := normalizeProtectPath(os.Getenv("LOCALAPPDATA"))
	if local == "" {
		return false
	}
	root := local + `\d3dscache`
	return norm == root || strings.HasPrefix(norm, root+`\`)
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return normalizeProtectPath(a) == normalizeProtectPath(b)
}

// IsSafeCleanTarget is exported for unit tests: path must be an allowlisted clean target.
func IsSafeCleanTarget(path string) bool {
	if path == "" || isForbiddenCleanRoot(path) || isForbiddenCleanPath(path) {
		return false
	}
	norm := normalizeProtectPath(path)
	if isUnderWindowsTemp(norm) || isUnderUserTemp(norm) {
		return true
	}
	if isAllowedD3DSCache(path) || isAllowedBrowserCacheDir(path) {
		return true
	}
	return false
}
