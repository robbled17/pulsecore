package main

import (
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
	"pulsecore.exe": {}, "zeropulse.exe": {}, "zeropulse-gui.exe": {},
}

// Path fragments that mark Windows / Defender / AV / shell critical targets.
var protectedPathMarkers = []string{
	`\windows\system32`,
	`\windows\syswow64`,
	`\windows\winsxs`,
	`\windows\systemapps`,
	`\windows\immersivecontrolpanel`,
	`\program files\windows defender`,
	`\program files\windowsapps`,
	`\programdata\microsoft\windows defender`,
	`\programdata\microsoft\windows security health`,
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

func normalizeProtectPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, `/`, `\`)
	p = strings.ToLower(p)
	// Collapse duplicate separators
	for strings.Contains(p, `\\`) {
		p = strings.ReplaceAll(p, `\\`, `\`)
	}
	p = strings.TrimRight(p, `\`)
	// Re-add trailing slash only for bare drive
	if len(p) == 2 && p[1] == ':' {
		return p + `\`
	}
	return p
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
