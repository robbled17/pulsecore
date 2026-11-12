package main

import (
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
	for _, h := range uninstallHives {
		out = append(out, scanUninstallHive(h, seen)...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SizeBytes == out[j].SizeBytes {
			return strings.ToLower(out[i].DisplayName) < strings.ToLower(out[j].DisplayName)
		}
		return out[i].SizeBytes > out[j].SizeBytes
	})
	return out
}

func scanUninstallHive(h uninstallHive, seen map[string]bool) []InstalledApp {
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
		app, ok := readInstalledApp(sk, h, name)
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

func readInstalledApp(k registry.Key, h uninstallHive, subName string) (InstalledApp, bool) {
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
	return app, true
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
	}
	for _, b := range blockedContains {
		if strings.Contains(p, b) {
			return false
		}
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
