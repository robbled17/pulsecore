package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

// InstalledApp is one user-visible installed program from Uninstall registry keys.
type InstalledApp struct {
	ID               string  `json:"id"`
	DisplayName      string  `json:"displayName"`
	DisplayVersion   string  `json:"displayVersion"`
	Publisher        string  `json:"publisher"`
	InstallLocation  string  `json:"installLocation"`
	SizeBytes        int64   `json:"sizeBytes"`
	SizeLabel        string  `json:"sizeLabel"`
	UninstallString  string  `json:"uninstallString"`
	QuietUninstall   string  `json:"quietUninstallString"`
	CanUninstall     bool    `json:"canUninstall"`
	RegistryRoot     string  `json:"registryRoot"`
	RegistryPath     string  `json:"registryPath"`
	DriveLetter      string  `json:"driveLetter"`
	LastUsedDays     int     `json:"lastUsedDays"`
	CandidateCleanup bool    `json:"candidateCleanup"`
	CandidateBadgeRu string  `json:"candidateBadgeRu,omitempty"`
	CandidateBadgeEn string  `json:"candidateBadgeEn,omitempty"`
}

// AppLeftover is a residual folder after uninstall.
type AppLeftover struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
	Exists    bool   `json:"exists"`
}

// UninstallAppResult is the outcome of UninstallWithCleanup.
type UninstallAppResult struct {
	OK           bool          `json:"ok"`
	Message      string        `json:"message"`
	Leftovers    []AppLeftover `json:"leftovers"`
	LeftoverBytes int64        `json:"leftoverBytes"`
	FreedBytes   int64         `json:"freedBytes"`
}

type uninstallHive struct {
	root registry.Key
	name string
	path string
}

var uninstallHives = []uninstallHive{
	{registry.LOCAL_MACHINE, "HKLM", `Software\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.LOCAL_MACHINE, "HKLM", `Software\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.CURRENT_USER, "HKCU", `Software\Microsoft\Windows\CurrentVersion\Uninstall`},
}

// ListInstalledApps scans Uninstall registry keys, heaviest first.
func (a *App) ListInstalledApps() []InstalledApp {
	seen := map[string]bool{}
	out := make([]InstalledApp, 0, 128)

	bamMap := scanBAMLaunchTimes()
	userAssistMap := scanUserAssistLaunchTimes()
	prefetchMap := scanPrefetchLaunchTimes()

	for _, h := range uninstallHives {
		out = append(out, scanUninstallHive(h, seen, bamMap, userAssistMap, prefetchMap)...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SizeBytes == out[j].SizeBytes {
			return strings.ToLower(out[i].DisplayName) < strings.ToLower(out[j].DisplayName)
		}
		return out[i].SizeBytes > out[j].SizeBytes
	})
	return out
}

func scanUninstallHive(h uninstallHive, seen map[string]bool, bamMap, userAssistMap, prefetchMap map[string]time.Time) []InstalledApp {
	k, err := registry.OpenKey(h.root, h.path, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil
	}
	defer k.Close()
	names, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return nil
	}
	out := make([]InstalledApp, 0, len(names))
	for _, name := range names {
		sk, err := registry.OpenKey(h.root, h.path+`\`+name, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		app, ok := readInstalledApp(sk, h, name, bamMap, userAssistMap, prefetchMap)
		sk.Close()
		if !ok {
			continue
		}
		key := strings.ToLower(app.DisplayName + "|" + app.DisplayVersion + "|" + app.Publisher)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, app)
	}
	return out
}

func readInstalledApp(k registry.Key, h uninstallHive, subName string, bamMap, userAssistMap, prefetchMap map[string]time.Time) (InstalledApp, bool) {
	displayName, _, err := k.GetStringValue("DisplayName")
	displayName = strings.TrimSpace(displayName)
	if err != nil || displayName == "" {
		return InstalledApp{}, false
	}
	if sys, _, err := k.GetIntegerValue("SystemComponent"); err == nil && sys == 1 {
		return InstalledApp{}, false
	}
	if rel, _, err := k.GetStringValue("ReleaseType"); err == nil {
		rel = strings.ToLower(strings.TrimSpace(rel))
		if rel == "update" || rel == "hotfix" || rel == "security update" {
			return InstalledApp{}, false
		}
	}
	if strings.Contains(strings.ToLower(displayName), "update for") ||
		strings.HasPrefix(strings.ToLower(displayName), "security update") {
		return InstalledApp{}, false
	}

	app := InstalledApp{
		ID:              h.name + `\` + h.path + `\` + subName,
		DisplayName:     displayName,
		RegistryRoot:    h.name,
		RegistryPath:    h.path + `\` + subName,
	}
	app.DisplayVersion, _, _ = k.GetStringValue("DisplayVersion")
	app.Publisher, _, _ = k.GetStringValue("Publisher")
	app.InstallLocation, _, _ = k.GetStringValue("InstallLocation")
	app.UninstallString, _, _ = k.GetStringValue("UninstallString")
	app.QuietUninstall, _, _ = k.GetStringValue("QuietUninstallString")
	app.UninstallString = strings.TrimSpace(app.UninstallString)
	app.QuietUninstall = strings.TrimSpace(app.QuietUninstall)
	app.CanUninstall = app.UninstallString != "" || app.QuietUninstall != ""

	if kb, _, err := k.GetIntegerValue("EstimatedSize"); err == nil && kb > 0 {
		app.SizeBytes = int64(kb) * 1024
	}
	if app.SizeBytes <= 0 && app.InstallLocation != "" {
		if info, err := os.Stat(app.InstallLocation); err == nil && info.IsDir() {
			app.SizeBytes = dirSizeBudget(app.InstallLocation, time.Now().Add(800*time.Millisecond), 4)
		}
	}
	app.SizeLabel = formatSizeLabel(app.SizeBytes)

	// Determine drive letter
	loc := strings.TrimSpace(app.InstallLocation)
	if len(loc) >= 2 && loc[1] == ':' {
		app.DriveLetter = strings.ToUpper(string(loc[0]))
	} else if len(app.UninstallString) >= 2 && app.UninstallString[1] == ':' {
		app.DriveLetter = strings.ToUpper(string(app.UninstallString[0]))
	} else {
		app.DriveLetter = "C"
	}

	// Inactivity & candidate detection based on real launch history (BAM, Prefetch, UserAssist, AppData)
	app.LastUsedDays, app.CandidateCleanup, app.CandidateBadgeRu, app.CandidateBadgeEn = getAppLastLaunchDays(app, bamMap, userAssistMap, prefetchMap)

	return app, true
}

func getFolderLatestActivity(dir string) time.Time {
	var newest time.Time
	count := 0
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		count++
		if count > 40 {
			return filepath.SkipDir
		}
		if info != nil && !info.IsDir() {
			mod := info.ModTime()
			if mod.After(newest) && mod.Before(time.Now().Add(24*time.Hour)) {
				newest = mod
			}
		}
		return nil
	})
	return newest
}

func formatSizeLabel(bytes int64) string {
	if bytes <= 0 {
		return "—"
	}
	gb := float64(bytes) / (1024 * 1024 * 1024)
	if gb >= 0.1 {
		return fmt.Sprintf("%.1f GB", gb)
	}
	mb := float64(bytes) / (1024 * 1024)
	if mb >= 1 {
		return fmt.Sprintf("%.0f MB", mb)
	}
	return fmt.Sprintf("%d KB", bytes/1024)
}

// UninstallWithCleanup runs the vendor uninstaller, then scans residual folders.
func (a *App) UninstallWithCleanup(appID string) UninstallAppResult {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return UninstallAppResult{OK: false, Message: "empty app id"}
	}
	var target *InstalledApp
	for _, app := range a.ListInstalledApps() {
		if app.ID == appID {
			cp := app
			target = &cp
			break
		}
	}
	if target == nil {
		return UninstallAppResult{OK: false, Message: "app not found"}
	}
	if !target.CanUninstall {
		return UninstallAppResult{OK: false, Message: "no uninstall string"}
	}

	quiet := strings.TrimSpace(target.QuietUninstall) != ""
	cmdLine := target.QuietUninstall
	if cmdLine == "" {
		cmdLine = target.UninstallString
	}
	beforeSize := target.SizeBytes

	exe, args := parseWindowsCommandLine(cmdLine)
	var runErr error
	if quiet {
		if exe != "" {
			runErr = RunHiddenCommand(exe, args...).Run()
		} else {
			runErr = RunHiddenCommand("cmd.exe", "/C", cmdLine).Run()
		}
	} else {
		// Interactive uninstaller — show UI and wait for completion.
		var cmd *exec.Cmd
		if exe != "" {
			cmd = exec.Command(exe, args...)
		} else {
			cmd = exec.Command("cmd.exe", "/C", cmdLine)
		}
		runErr = cmd.Run()
	}
	if runErr != nil {
		// Many uninstallers return non-zero even on success / user cancel.
		// Continue to leftover scan; UI can still clean tails.
	}

	time.Sleep(600 * time.Millisecond)
	leftovers := findAppLeftovers(target.DisplayName, target.InstallLocation)
	var leftBytes int64
	for _, l := range leftovers {
		leftBytes += l.SizeBytes
	}

	if a != nil {
		a.RefreshDiskMetricsEx()
	}

	freed := beforeSize - leftBytes
	if freed < 0 {
		freed = 0
	}
	msg := "ok"
	if runErr != nil {
		msg = runErr.Error()
	}
	return UninstallAppResult{
		OK:            true, // uninstall UI may have completed even with non-zero exit
		Message:       msg,
		Leftovers:     leftovers,
		LeftoverBytes: leftBytes,
		FreedBytes:    freed,
	}
}

// RemoveAppLeftovers deletes residual folders reported after uninstall.
func (a *App) RemoveAppLeftovers(paths []string) UninstallAppResult {
	var freed int64
	kept := make([]AppLeftover, 0)
	for _, p := range paths {
		p = filepath.Clean(strings.TrimSpace(p))
		if p == "" || (!isSafeAppLeftoverPath(p) && !isDeletableInstallPath(p)) {
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		var sz int64
		if info.IsDir() {
			sz = dirSizeBudget(p, time.Now().Add(1500*time.Millisecond), 6)
			_ = ForceDeleteAll(p)
		} else {
			sz = info.Size()
			clearReadOnlyAttr(p)
			_ = os.Chmod(p, 0o777)
			_ = os.Remove(p)
		}
		if _, err := os.Stat(p); err == nil {
			kept = append(kept, AppLeftover{Path: p, SizeBytes: sz, Exists: true})
			continue
		}
		freed += sz
	}
	if a != nil {
		a.RefreshDiskMetricsEx()
	}
	return UninstallAppResult{
		OK:            true,
		Message:       "ok",
		Leftovers:     kept,
		LeftoverBytes: leftoverSum(kept),
		FreedBytes:    freed,
	}
}

func leftoverSum(items []AppLeftover) int64 {
	var n int64
	for _, it := range items {
		n += it.SizeBytes
	}
	return n
}

func findAppLeftovers(displayName, installLocation string) []AppLeftover {
	names := candidateFolderNames(displayName)
	roots := []string{}
	if v := os.Getenv("APPDATA"); v != "" {
		roots = append(roots, v)
	}
	if v := os.Getenv("LOCALAPPDATA"); v != "" {
		roots = append(roots, v)
	}
	if v := os.Getenv("PROGRAMDATA"); v != "" {
		roots = append(roots, v)
	}
	out := make([]AppLeftover, 0)
	seen := map[string]bool{}
	add := func(p string) {
		p = filepath.Clean(p)
		key := strings.ToLower(p)
		if p == "" || seen[key] || !isSafeAppLeftoverPath(p) {
			return
		}
		info, err := os.Stat(p)
		if err != nil {
			return
		}
		seen[key] = true
		sz := info.Size()
		if info.IsDir() {
			sz = dirSizeBudget(p, time.Now().Add(600*time.Millisecond), 4)
		}
		out = append(out, AppLeftover{Path: p, SizeBytes: sz, Exists: true})
	}
	for _, root := range roots {
		for _, name := range names {
			add(filepath.Join(root, name))
		}
	}
	if loc := strings.TrimSpace(installLocation); loc != "" {
		if info, err := os.Stat(loc); err == nil && info.IsDir() {
			// Only flag install dir if it still has residual content after uninstall.
			sz := dirSizeBudget(loc, time.Now().Add(500*time.Millisecond), 3)
			if sz > 0 {
				add(loc)
			}
		}
	}
	return out
}

func candidateFolderNames(displayName string) []string {
	n := strings.TrimSpace(displayName)
	if n == "" {
		return nil
	}
	out := []string{n}
	// Strip version / bitness suffixes often used in DisplayName.
	cut := n
	for _, sep := range []string{" - ", " (", " ["} {
		if i := strings.Index(cut, sep); i > 2 {
			cut = strings.TrimSpace(cut[:i])
		}
	}
	if cut != n && cut != "" {
		out = append(out, cut)
	}
	// Publisher-less short token
	parts := strings.Fields(cut)
	if len(parts) >= 2 {
		out = append(out, parts[0]+" "+parts[1])
	}
	uniq := make([]string, 0, len(out))
	seen := map[string]bool{}
	for _, s := range out {
		s = strings.TrimSpace(s)
		key := strings.ToLower(s)
		if s == "" || seen[key] || len(s) < 3 {
			continue
		}
		seen[key] = true
		uniq = append(uniq, s)
	}
	return uniq
}

func isSafeAppLeftoverPath(p string) bool {
	p = strings.ToLower(filepath.Clean(p))
	blockedExact := []string{
		`c:\`, `c:\windows`, `c:\program files`, `c:\program files (x86)`,
		`c:\users`, `c:\programdata`,
	}
	for _, b := range blockedExact {
		if p == b {
			return false
		}
	}
	blockedContains := []string{
		`\windows\`, `\system32\`, `\syswow64\`, `\winsxs\`,
		`\microsoft\windows`, `\documents\`, `\desktop\`, `\downloads\`,
		`\$recycle.bin`, `\system volume information`, `\recovery`,
	}
	for _, b := range blockedContains {
		if strings.Contains(p, b) {
			return false
		}
	}
	vol := strings.ToLower(filepath.VolumeName(p))
	if p == vol || p == vol+`\` || len(p) <= 3 {
		return false // never delete volume root
	}
	appdata := strings.ToLower(filepath.Clean(os.Getenv("APPDATA")))
	local := strings.ToLower(filepath.Clean(os.Getenv("LOCALAPPDATA")))
	pdata := strings.ToLower(filepath.Clean(os.Getenv("PROGRAMDATA")))
	pf := strings.ToLower(filepath.Clean(os.Getenv("ProgramFiles")))
	pf86 := strings.ToLower(filepath.Clean(os.Getenv("ProgramFiles(x86)")))
	allowed := []string{appdata, local, pdata, pf, pf86}
	for _, root := range allowed {
		if root == "" || root == "." {
			continue
		}
		if p == root {
			return false // never delete the root itself
		}
		if strings.HasPrefix(p, root+string(os.PathSeparator)) {
			// Require at least one subfolder under the root.
			rel := strings.TrimPrefix(p, root+string(os.PathSeparator))
			if rel == "" || !strings.Contains(rel, string(os.PathSeparator)) && len(rel) < 3 {
				// single segment under AppData is OK (e.g. %APPDATA%\Discord)
				return len(rel) >= 3
			}
			return true
		}
	}
	// Also permit non-system folders on secondary drives (D:\, E:\) or root subfolders
	if vol != "" && strings.HasPrefix(p, vol+`\`) {
		rel := strings.TrimPrefix(p, vol+`\`)
		if rel == "windows" || rel == "users" || rel == "perflogs" || rel == "program files" || rel == "program files (x86)" || rel == "programdata" {
			return false
		}
		return len(rel) >= 3
	}
	return false
}

// parseWindowsCommandLine splits an UninstallString into exe + args.
func parseWindowsCommandLine(s string) (string, []string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	// msiexec ... keep as cmd /C handled by caller if parse fails
	var parts []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '"':
			inQuote = !inQuote
		case ch == ' ' || ch == '\t':
			if inQuote {
				cur.WriteByte(ch)
			} else if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(ch)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}


func filetimeToTime(ft uint64) time.Time {
	if ft == 0 || ft > 0x7FFFFFFFFFFFFFFF {
		return time.Time{}
	}
	if ft < 116444736000000000 {
		return time.Time{}
	}
	nsec := int64(ft-116444736000000000) * 100
	t := time.Unix(0, nsec)
	now := time.Now()
	if t.After(now.Add(48*time.Hour)) || t.Before(now.AddDate(-15, 0, 0)) {
		return time.Time{}
	}
	return t
}

func rot13(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = 'a' + (c-'a'+13)%26
		} else if c >= 'A' && c <= 'Z' {
			b[i] = 'A' + (c-'A'+13)%26
		}
	}
	return string(b)
}

func normalizeExecutablePath(p string) string {
	low := strings.ToLower(filepath.Clean(strings.TrimSpace(p)))
	if idx := strings.Index(low, `\device\harddiskvolume`); idx != -1 {
		rest := low[idx+len(`\device\harddiskvolume`):]
		digitIdx := 0
		for digitIdx < len(rest) && rest[digitIdx] >= '0' && rest[digitIdx] <= '9' {
			digitIdx++
		}
		if digitIdx < len(rest) && rest[digitIdx] == '\\' {
			low = rest[digitIdx+1:]
		}
	} else if len(low) >= 3 && low[1] == ':' && low[2] == '\\' {
		low = low[3:]
	}
	return strings.TrimSpace(low)
}

func scanBAMLaunchTimes() map[string]time.Time {
	out := make(map[string]time.Time)
	basePaths := []string{
		`SYSTEM\CurrentControlSet\Services\bam\State\UserSettings`,
		`SYSTEM\CurrentControlSet\Services\bam\UserSettings`,
	}
	for _, bp := range basePaths {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, bp, registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}
		subKeys, err := k.ReadSubKeyNames(-1)
		k.Close()
		if err != nil {
			continue
		}
		for _, sk := range subKeys {
			userK, err := registry.OpenKey(registry.LOCAL_MACHINE, bp+`\`+sk, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			valNames, err := userK.ReadValueNames(-1)
			if err == nil {
				for _, vn := range valNames {
					val, valType, err := userK.GetBinaryValue(vn)
					if err == nil && valType == registry.BINARY && len(val) >= 8 {
						ft := binary.LittleEndian.Uint64(val[:8])
						t := filetimeToTime(ft)
						if !t.IsZero() {
							norm := normalizeExecutablePath(vn)
							if prev, exists := out[norm]; !exists || t.After(prev) {
								out[norm] = t
							}
							exeBase := filepath.Base(norm)
							if prev, exists := out[exeBase]; !exists || t.After(prev) {
								out[exeBase] = t
							}
						}
					}
				}
			}
			userK.Close()
		}
	}
	return out
}

func scanUserAssistLaunchTimes() map[string]time.Time {
	out := make(map[string]time.Time)
	base := `Software\Microsoft\Windows\CurrentVersion\Explorer\UserAssist`
	k, err := registry.OpenKey(registry.CURRENT_USER, base, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return out
	}
	defer k.Close()
	subKeys, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return out
	}
	for _, sk := range subKeys {
		countPath := base + `\` + sk + `\Count`
		ck, err := registry.OpenKey(registry.CURRENT_USER, countPath, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		valNames, err := ck.ReadValueNames(-1)
		if err == nil {
			for _, vn := range valNames {
				decoded := rot13(vn)
				if !strings.HasSuffix(strings.ToLower(decoded), ".exe") && !strings.Contains(decoded, ".exe") {
					continue
				}
				val, valType, err := ck.GetBinaryValue(vn)
				if err == nil && valType == registry.BINARY && len(val) >= 68 {
					ft := binary.LittleEndian.Uint64(val[60:68])
					t := filetimeToTime(ft)
					if !t.IsZero() {
						norm := normalizeExecutablePath(decoded)
						if prev, exists := out[norm]; !exists || t.After(prev) {
							out[norm] = t
						}
						exeBase := filepath.Base(norm)
						if prev, exists := out[exeBase]; !exists || t.After(prev) {
							out[exeBase] = t
						}
					}
				}
			}
		}
		ck.Close()
	}
	return out
}

func scanPrefetchLaunchTimes() map[string]time.Time {
	out := make(map[string]time.Time)
	entries, err := os.ReadDir(`C:\Windows\Prefetch`)
	if err != nil {
		return out
	}
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if strings.HasSuffix(name, ".pf") {
			parts := strings.Split(name, "-")
			if len(parts) >= 2 {
				exeName := parts[0]
				if info, err := e.Info(); err == nil {
					mod := info.ModTime()
					if prev, ok := out[exeName]; !ok || mod.After(prev) {
						out[exeName] = mod
					}
					exeNoExt := strings.TrimSuffix(exeName, ".exe")
					if prev, ok := out[exeNoExt]; !ok || mod.After(prev) {
						out[exeNoExt] = mod
					}
				}
			}
		}
	}
	return out
}

func scanAppDataRecentMod(displayName string, installLocation string) time.Time {
	var newest time.Time
	now := time.Now()
	roots := []string{
		os.Getenv("LOCALAPPDATA"),
		os.Getenv("APPDATA"),
		filepath.Join(os.Getenv("USERPROFILE"), "Saved Games"),
		filepath.Join(os.Getenv("USERPROFILE"), "Documents"),
	}

	var keywords []string
	cleanName := strings.ToLower(strings.TrimSpace(displayName))
	if cleanName != "" {
		keywords = append(keywords, cleanName)
		words := strings.Fields(cleanName)
		if len(words) > 1 {
			keywords = append(keywords, words[0])
		}
	}
	if installLocation != "" {
		base := strings.ToLower(filepath.Base(installLocation))
		if base != "" && base != cleanName {
			keywords = append(keywords, base)
		}
	}

	for _, root := range roots {
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
			folderLower := strings.ToLower(e.Name())
			matched := false
			for _, kw := range keywords {
				if len(kw) >= 3 && (folderLower == kw || strings.Contains(folderLower, kw)) {
					matched = true
					break
				}
			}
			if matched {
				fullDir := filepath.Join(root, e.Name())
				count := 0
				_ = filepath.Walk(fullDir, func(p string, info os.FileInfo, err error) error {
					if err != nil {
						return filepath.SkipDir
					}
					count++
					if count > 30 {
						return filepath.SkipDir
					}
					if info != nil && !info.IsDir() {
						mod := info.ModTime()
						if mod.After(newest) && mod.Before(now.Add(24*time.Hour)) && mod.After(now.AddDate(-1, 0, 0)) {
							newest = mod
						}
					}
					return nil
				})
			}
		}
	}
	return newest
}

func findExecutablesInDir(dir string, maxDepth int, maxCount int) []string {
	var exes []string
	cleanDir := filepath.Clean(dir)
	baseDepth := strings.Count(cleanDir, string(os.PathSeparator))
	_ = filepath.Walk(cleanDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if len(exes) >= maxCount {
			return filepath.SkipDir
		}
		if info != nil && info.IsDir() {
			d := strings.Count(p, string(os.PathSeparator)) - baseDepth
			if d > maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if info != nil && strings.EqualFold(filepath.Ext(p), ".exe") {
			exes = append(exes, p)
		}
		return nil
	})
	return exes
}

func getAppLastLaunchDays(app InstalledApp, bamMap, userAssistMap, prefetchMap map[string]time.Time) (days int, isCandidate bool, badgeRu string, badgeEn string) {
	days = -1
	var newest time.Time
	now := time.Now()

	loc := strings.TrimSpace(app.InstallLocation)
	var candidateExes []string
	if loc != "" {
		if info, err := os.Stat(loc); err == nil && info.IsDir() {
			candidateExes = findExecutablesInDir(loc, 2, 25)
		}
	}
	if app.UninstallString != "" {
		cmd, _ := parseWindowsCommandLine(app.UninstallString)
		if strings.EqualFold(filepath.Ext(cmd), ".exe") {
			candidateExes = append(candidateExes, cmd)
		}
	}

	// 1. Check candidate executables in BAM, UserAssist, Prefetch
	for _, exe := range candidateExes {
		norm := normalizeExecutablePath(exe)
		exeBase := strings.ToLower(filepath.Base(exe))
		exeNoExt := strings.TrimSuffix(exeBase, ".exe")

		if t, ok := bamMap[norm]; ok && t.After(newest) {
			newest = t
		}
		if t, ok := userAssistMap[norm]; ok && t.After(newest) {
			newest = t
		}
		if t, ok := prefetchMap[exeBase]; ok && t.After(newest) {
			newest = t
		}
		if t, ok := prefetchMap[exeNoExt]; ok && t.After(newest) {
			newest = t
		}
	}

	// 2. Check BAM/UserAssist prefixes by install location
	if loc != "" {
		normLoc := normalizeExecutablePath(loc)
		if len(normLoc) >= 4 {
			for pathKey, t := range bamMap {
				if strings.Contains(pathKey, normLoc) && t.After(newest) {
					newest = t
				}
			}
			for pathKey, t := range userAssistMap {
				if strings.Contains(pathKey, normLoc) && t.After(newest) {
					newest = t
				}
			}
		}
	}

	// 3. Check AppData logs/saves
	if appDataTime := scanAppDataRecentMod(app.DisplayName, loc); appDataTime.After(newest) {
		newest = appDataTime
	}

	// Strict verification: only accept real, plausible timestamps
	if !newest.IsZero() && newest.Before(now.Add(24*time.Hour)) && newest.After(now.AddDate(-10, 0, 0)) {
		diffDays := int(now.Sub(newest).Hours() / 24)
		if diffDays < 0 {
			diffDays = 0
		}
		if diffDays > 365 {
			diffDays = 365
		}
		days = diffDays

		if days >= 60 && app.SizeBytes >= 2*1024*1024*1024 {
			isCandidate = true
			gb := float64(app.SizeBytes) / (1024 * 1024 * 1024)
			if days >= 365 {
				badgeRu = fmt.Sprintf("💡 Не запускалась более года — можно освободить %.1f ГБ", gb)
				badgeEn = fmt.Sprintf("💡 Not launched for over a year — can free %.1f GB", gb)
			} else {
				badgeRu = fmt.Sprintf("💡 Не запускалась %d дн. — можно освободить %.1f ГБ", days, gb)
				badgeEn = fmt.Sprintf("💡 Not launched for %d days — can free %.1f GB", days, gb)
			}
		}
	}

	return
}
