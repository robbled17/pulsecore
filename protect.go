package main

import (
	"os"
	"path/filepath"
	"strings"
)

// Protected process basenames (lowercase, with .exe where applicable).
// Eco Mode and the uninstaller must never kill / deprioritize these.
var protectedSystemProcesses = map[string]struct{}{
	"system": {}, "registry": {}, "idle": {}, "system idle process": {},
	"secure system": {}, "memory compression": {},
	"smss.exe": {}, "csrss.exe": {}, "wininit.exe": {}, "winlogon.exe": {},
	"services.exe": {}, "lsass.exe": {}, "lsm.exe": {}, "svchost.exe": {},
	"explorer.exe": {}, "dwm.exe": {}, "fontdrvhost.exe": {},
	"sihost.exe": {}, "taskhostw.exe": {}, "conhost.exe": {},
	"ctfmon.exe": {}, "runtimebroker.exe": {}, "taskmgr.exe": {},
	"startmenuexperiencehost.exe": {}, "shellexperiencehost.exe": {},
	"searchhost.exe": {}, "searchapp.exe": {},
	"msmpeng.exe": {}, "nissrv.exe": {}, "nisrv.exe": {},
	"securityhealthservice.exe": {}, "securityhealthsystray.exe": {},
	"securityhealthhost.exe": {}, "mssense.exe": {}, "mpcmdrun.exe": {},
	"sgrmbroker.exe": {}, "smartscreen.exe": {}, "wdavscannerservice.exe": {},
	"searchindexer.exe": {}, "searchprotocolhost.exe": {},
	"pulsecore.exe": {}, "zeropulse.exe": {}, "zeropulse-gui.exe": {},
	"msedgewebview2.exe": {},
}

// isNoiseBackgroundProcess is true for AV / critical OS processes that must
// never appear as Lag culprits or Eco Mode targets (aligns with IsProtectedSystemTarget).
func isNoiseBackgroundProcess(nameOrPath string) bool {
	return IsProtectedSystemTarget(nameOrPath)
}

// Path fragments that mark Windows / Defender / AV / shell critical targets.
var protectedPathMarkers = []string{
	`\windows\system32`,
	`\windows\syswow64`,
	`\windows\winsxs`,
	`\windows\systemapps`,
	`\windows\immersivecontrolpanel`,
	`\windows\packages`,
	`\program files\windows defender`,
	`\program files\windowsapps`,
	`\programdata\microsoft\windows defender`,
	`\programdata\microsoft\windows security health`,
	`\programdata\microsoft`,
	`\appdata\local\packages`,
	`\microsoft\windows defender`,
	`\windows defender`,
}

// Forbidden registry path prefixes (normalized lowercase, backslash).
var forbiddenRegistryPrefixes = []string{
	`hklm\system`,
	`hklm\software\microsoft\windows`,
	`hklm\software\wow6432node\microsoft\windows`,
	`hklm\software\microsoft\windows defender`,
	`hklm\software\wow6432node\microsoft\windows defender`,
	`hklm\software\policies\microsoft\windows defender`,
	`hkcu\software\microsoft\windows\currentversion\explorer`,
	`hkcu\software\classes\local settings\software\microsoft\windows\currentversion\traynotify`,
}

// Minimum InstallLocation path depth (segments after drive) for kill/delete.
const minInstallLocationDepth = 2

// IsProtectedSystemTarget reports whether nameOrPath refers to a critical
// Windows / Defender / AV / shell process or filesystem location.
func IsProtectedSystemTarget(nameOrPath string) bool {
	s := strings.TrimSpace(nameOrPath)
	if s == "" {
		return true
	}
	base := strings.ToLower(filepath.Base(s))
	if _, ok := protectedSystemProcesses[base]; ok {
		return true
	}
	// Also accept names without .exe
	if !strings.HasSuffix(base, ".exe") {
		if _, ok := protectedSystemProcesses[base+".exe"]; ok {
			return true
		}
	}
	norm := normalizeProtectPath(s)
	for _, m := range protectedPathMarkers {
		if strings.Contains(norm, m) {
			return true
		}
	}
	if isDangerousInstallRoot(norm) {
		return true
	}
	return false
}

// IsForbiddenRegistryPath blocks edits under critical Windows / Defender / Explorer keys.
func IsForbiddenRegistryPath(path string) bool {
	norm := normalizeRegistryPath(path)
	if norm == "" {
		return true
	}
	for _, p := range forbiddenRegistryPrefixes {
		if norm == p || strings.HasPrefix(norm, p+`\`) {
			return true
		}
	}
	// Defender anywhere in the path
	if strings.Contains(norm, `\windows defender`) || strings.Contains(norm, `\windowsdefender`) {
		return true
	}
	return false
}

// isDangerousInstallRoot forbids deleting whole OS / profile / Program Files roots.
func isDangerousInstallRoot(path string) bool {
	norm := normalizeProtectPath(path)
	if norm == "" {
		return true
	}
	// Drive roots: c: or c:\
	if norm == `c:\` || norm == `c:` || (len(norm) == 2 && norm[1] == ':') {
		return true
	}
	// Any bare drive letter root (d:\, e:\ …)
	if len(norm) == 3 && norm[1] == ':' && (norm[2] == '\\' || norm[2] == '/') {
		return true
	}
	dangerousExact := []string{
		`c:\windows`,
		`c:\program files`,
		`c:\program files (x86)`,
		`c:\users`,
		`c:\programdata`,
	}
	for _, d := range dangerousExact {
		if norm == d {
			return true
		}
	}
	// Whole user profile: C:\Users\<name> with no deeper path
	if strings.HasPrefix(norm, `c:\users\`) {
		rest := strings.TrimPrefix(norm, `c:\users\`)
		if rest != "" && !strings.Contains(rest, `\`) {
			return true
		}
	}
	return false
}

// installPathDepth counts path segments after the drive (or UNC share).
// C:\ → 0; C:\Windows → 1; C:\Program Files\App → 2.
func installPathDepth(path string) int {
	norm := normalizeProtectPath(path)
	if norm == "" {
		return 0
	}
	// UNC: \\server\share\rest…
	if strings.HasPrefix(norm, `\\`) {
		trim := strings.TrimPrefix(norm, `\\`)
		parts := strings.Split(trim, `\`)
		n := 0
		for _, p := range parts {
			if p != "" {
				n++
			}
		}
		// server + share = 2; depth beyond share
		if n <= 2 {
			return 0
		}
		return n - 2
	}
	// Drive path
	if len(norm) >= 2 && norm[1] == ':' {
		rest := ""
		if len(norm) > 2 {
			rest = strings.TrimPrefix(norm[2:], `\`)
		}
		if rest == "" {
			return 0
		}
		parts := strings.Split(rest, `\`)
		n := 0
		for _, p := range parts {
			if p != "" && p != "." {
				n++
			}
		}
		return n
	}
	parts := strings.Split(norm, `\`)
	n := 0
	for _, p := range parts {
		if p != "" && p != "." {
			n++
		}
	}
	return n
}

// isValidInstallLocation requires a non-empty, non-dangerous path with depth ≥ 2.
func isValidInstallLocation(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	if isDangerousInstallRoot(path) || IsProtectedSystemTarget(path) {
		return false
	}
	return installPathDepth(path) >= minInstallLocationDepth
}

// normalizeProtectPath expands env vars, strips \\?\ / UNC quirks, collapses ..,
// lowercases, and trims trailing separators (except bare drive roots).
func normalizeProtectPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	// Expand %ENV% tokens before anything else.
	p = expandWindowsEnv(p)
	p = strings.ReplaceAll(p, `/`, `\`)

	// Strip extended-length / verbatim prefixes.
	lower := strings.ToLower(p)
	switch {
	case strings.HasPrefix(lower, `\\?\unc\`):
		p = `\\` + p[len(`\\?\unc\`):]
	case strings.HasPrefix(lower, `\\?\`):
		p = p[len(`\\?\`):]
	}

	// Collapse . and .. segments (do not resolve real symlinks here).
	p = collapseDotSegments(p)

	p = strings.ToLower(p)
	for strings.Contains(p, `\\`) {
		// Keep UNC leading \\
		if strings.HasPrefix(p, `\\`) {
			rest := p[2:]
			for strings.Contains(rest, `\\`) {
				rest = strings.ReplaceAll(rest, `\\`, `\`)
			}
			p = `\\` + rest
			break
		}
		p = strings.ReplaceAll(p, `\\`, `\`)
	}
	p = strings.TrimRight(p, `\`)
	// Re-add trailing slash only for bare drive
	if len(p) == 2 && p[1] == ':' {
		return p + `\`
	}
	return p
}

func expandWindowsEnv(p string) string {
	if !strings.Contains(p, "%") {
		return p
	}
	var b strings.Builder
	for i := 0; i < len(p); {
		if p[i] == '%' {
			j := strings.IndexByte(p[i+1:], '%')
			if j >= 0 {
				name := p[i+1 : i+1+j]
				if name != "" {
					if v := os.Getenv(name); v != "" {
						b.WriteString(v)
						i += j + 2
						continue
					}
				}
			}
		}
		b.WriteByte(p[i])
		i++
	}
	return b.String()
}

func collapseDotSegments(p string) string {
	unc := false
	if strings.HasPrefix(p, `\\`) {
		unc = true
		p = p[2:]
	}
	drive := ""
	if len(p) >= 2 && p[1] == ':' {
		drive = strings.ToUpper(p[:2])
		p = p[2:]
		if strings.HasPrefix(p, `\`) {
			p = p[1:]
		}
	}
	parts := strings.Split(p, `\`)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
		default:
			out = append(out, part)
		}
	}
	joined := strings.Join(out, `\`)
	if unc {
		return `\\` + joined
	}
	if drive != "" {
		if joined == "" {
			return drive + `\`
		}
		return drive + `\` + joined
	}
	return joined
}

func normalizeRegistryPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, `/`, `\`)
	p = strings.ToLower(p)
	repl := []struct{ old, new string }{
		{`hkey_local_machine\`, `hklm\`},
		{`hkey_current_user\`, `hkcu\`},
		{`hkey_classes_root\`, `hkcr\`},
		{`computer\hklm\`, `hklm\`},
		{`computer\hkcu\`, `hkcu\`},
	}
	for _, r := range repl {
		if strings.HasPrefix(p, r.old) {
			p = r.new + strings.TrimPrefix(p, r.old)
		}
	}
	for strings.Contains(p, `\\`) {
		p = strings.ReplaceAll(p, `\\`, `\`)
	}
	return strings.TrimRight(p, `\`)
}

// resolveDeletePath normalizes and optionally EvalSymlinks; rejects escapes
// outside the allowlisted root (when root is non-empty).
func resolveDeletePath(path, allowedRoot string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}
	norm := normalizeProtectPath(path)
	if norm == "" || isDangerousInstallRoot(norm) || IsProtectedSystemTarget(path) {
		return "", false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	resolved := abs
	if r, err := filepath.EvalSymlinks(abs); err == nil && r != "" {
		resolved = r
	}
	resNorm := normalizeProtectPath(resolved)
	if resNorm == "" || isDangerousInstallRoot(resNorm) || IsProtectedSystemTarget(resolved) {
		return "", false
	}
	if allowedRoot != "" {
		rootNorm := normalizeProtectPath(allowedRoot)
		rootAbs, err := filepath.Abs(allowedRoot)
		if err != nil {
			rootAbs = allowedRoot
		}
		if r, err := filepath.EvalSymlinks(rootAbs); err == nil && r != "" {
			rootAbs = r
		}
		rootNorm = normalizeProtectPath(rootAbs)
		if rootNorm == "" {
			return "", false
		}
		if resNorm != rootNorm && !strings.HasPrefix(resNorm, rootNorm+`\`) {
			return "", false
		}
	}
	return resolved, true
}

// safeRemoveFile deletes a single file after path resolve. dryRun skips Remove.
func safeRemoveFile(path, allowedRoot string, dryRun bool) error {
	resolved, ok := resolveDeletePath(path, allowedRoot)
	if !ok {
		return os.ErrPermission
	}
	if dryRun {
		return nil
	}
	return os.Remove(resolved)
}

// safeRemoveAll deletes a directory tree after path resolve. dryRun skips RemoveAll.
func safeRemoveAll(path, allowedRoot string, dryRun bool) error {
	resolved, ok := resolveDeletePath(path, allowedRoot)
	if !ok {
		return os.ErrPermission
	}
	if dryRun {
		return nil
	}
	return os.RemoveAll(resolved)
}
