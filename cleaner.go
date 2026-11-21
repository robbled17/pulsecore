package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

type cleanRootKind int

const (
	cleanKindTemp cleanRootKind = iota
	cleanKindCache
)

type cleanRoot struct {
	path string
	kind cleanRootKind
	label string
}

// CleanSafeDiskCaches deletes only allowlisted temp/cache files with age guards.
// Never removes the root folders themselves; never touches Packages or system trees.
func (a *App) CleanSafeDiskCaches() DiskCleanResult {
	return a.cleanSafeDiskCaches(false)
}

// PreviewSafeDiskCaches is a dry-run of CleanSafeDiskCaches (no deletes).
func (a *App) PreviewSafeDiskCaches() DiskCleanResult {
	return a.cleanSafeDiskCaches(true)
}

func (a *App) cleanSafeDiskCaches(dryRun bool) DiskCleanResult {
	roots := listAllowedCleanRoots()
	var deleted int
	var bytes int64
	var skipped, errs int
	labels := make([]string, 0, len(roots))

	for _, root := range roots {
		if root.path == "" {
			continue
		}
		labels = append(labels, root.label)
		d, b, s, e := cleanAllowedRoot(root, dryRun)
		deleted += d
		bytes += b
		skipped += s
		errs += e
	}

	verb := "Удалено"
	if dryRun {
		verb = "К удалению"
	}
	msg := fmt.Sprintf("%s файлов: %d (%.1f МБ), пропущено: %d, ошибок: %d",
		verb, deleted, float64(bytes)/(1024*1024), skipped, errs)
	return DiskCleanResult{
		OK:           errs == 0,
		DeletedFiles: deleted,
		DeletedBytes: bytes,
		Skipped:      skipped,
		Errors:       errs,
		Targets:      labels,
		Message:      msg,
		DryRun:       dryRun,
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
		out = append(out, cleanRoot{path: abs, kind: cleanKindTemp, label: label})
	}
	addCache := func(p, label string) {
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
		out = append(out, cleanRoot{path: abs, kind: cleanKindCache, label: label})
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
		addCache(filepath.Join(local, "D3DSCache"), "D3DSCache")
		for _, browser := range browserUserDataRoots(local) {
			for _, profile := range listBrowserProfiles(browser) {
				for _, leaf := range []string{"Cache", "Code Cache", "GPUCache"} {
					p := filepath.Join(profile, leaf)
					if st, err := os.Stat(p); err == nil && st.IsDir() {
						addCache(p, filepath.Base(browser)+"/"+filepath.Base(profile)+"/"+leaf)
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

// isForbiddenCleanRoot blocks cleaning entire AppData / ProgramData / Windows / PF trees.
func isForbiddenCleanRoot(path string) bool {
	norm := normalizeProtectPath(path)
	if norm == "" || IsProtectedSystemTarget(path) {
		return true
	}
	if strings.Contains(norm, `\appdata\local\packages`) {
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
	if strings.Contains(norm, `\appdata\local\packages`) {
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
	if strings.Contains(norm, `\appdata\local\packages`) {
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
