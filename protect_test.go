package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsProtectedSystemTargetProcesses(t *testing.T) {
	protected := []string{
		"explorer.exe", "StartMenuExperienceHost.exe", "ShellExperienceHost.exe",
		"SearchHost.exe", "dwm.exe", "csrss.exe", "wininit.exe", "winlogon.exe",
		"services.exe", "lsass.exe", "svchost.exe", "smss.exe",
		"MsMpEng.exe", "NisSrv.exe", "SecurityHealthService.exe",
		"SearchIndexer.exe", "SearchProtocolHost.exe",
		"ctfmon.exe", "RuntimeBroker.exe", "sihost.exe", "taskmgr.exe",
		"PulseCore.EXE", "",
	}
	for _, n := range protected {
		if !IsProtectedSystemTarget(n) {
			t.Fatalf("expected protected process: %q", n)
		}
	}
	if IsProtectedSystemTarget("chrome.exe") {
		t.Fatal("chrome.exe must not be protected")
	}
	if IsProtectedSystemTarget("notepad.exe") {
		t.Fatal("notepad.exe must not be protected")
	}
}

func TestIsProtectedSystemTargetPaths(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{`C:\Windows\System32\cmd.exe`, true},
		{`C:\Program Files\Windows Defender\MsMpEng.exe`, true},
		{`C:\Users\Bob\AppData\Local\Packages\Microsoft.Windows.StartMenuExperienceHost_cw5n1h2txyewy`, true},
		{`C:\`, true},
		{`C:\Windows`, true},
		{`C:\Program Files`, true},
		{`C:\Program Files (x86)`, true},
		{`C:\Users`, true},
		{`C:\Users\Bob`, true},
		{`C:\Users\Bob\AppData\Local\MyApp`, false},
		{`C:\Program Files\MyVendor\App`, false},
	}
	for _, c := range cases {
		got := IsProtectedSystemTarget(c.path)
		if got != c.want {
			t.Fatalf("IsProtectedSystemTarget(%q)=%v want %v", c.path, got, c.want)
		}
	}
}

func TestIsForbiddenRegistryPath(t *testing.T) {
	bad := []string{
		`HKLM\SYSTEM\CurrentControlSet\Services`,
		`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Run`,
		`HKLM\SOFTWARE\Microsoft\Windows Defender`,
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\Advanced`,
		`HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows\CurrentVersion`,
	}
	for _, p := range bad {
		if !IsForbiddenRegistryPath(p) {
			t.Fatalf("expected forbidden registry: %q", p)
		}
	}
	ok := `HKCU\Software\SomeVendor\App`
	if IsForbiddenRegistryPath(ok) {
		t.Fatalf("vendor key should be allowed: %q", ok)
	}
}

func TestIsDangerousInstallRoot(t *testing.T) {
	if !isDangerousInstallRoot(`C:\`) {
		t.Fatal("C:\\ must be dangerous")
	}
	if !isDangerousInstallRoot(`C:\Windows`) {
		t.Fatal("C:\\Windows must be dangerous")
	}
	if !isDangerousInstallRoot(`C:\Program Files`) {
		t.Fatal("Program Files must be dangerous")
	}
	if !isDangerousInstallRoot(`C:\Program Files (x86)`) {
		t.Fatal("Program Files (x86) must be dangerous")
	}
	if !isDangerousInstallRoot(`C:\Users\Alice`) {
		t.Fatal("whole profile must be dangerous")
	}
	if isDangerousInstallRoot(`C:\Program Files\Acme\Tool`) {
		t.Fatal("vendor install dir should be ok")
	}
}

func TestInstallLocationDepthAndNormalize(t *testing.T) {
	if installPathDepth(`C:\`) != 0 {
		t.Fatal("drive root depth 0")
	}
	if installPathDepth(`C:\App`) != 1 {
		t.Fatal("C:\\App depth 1")
	}
	if installPathDepth(`C:\Program Files\Acme\Tool`) != 3 {
		t.Fatal("vendor tool depth 3")
	}
	if isValidInstallLocation(``) || isValidInstallLocation(`   `) {
		t.Fatal("empty InstallLocation must be invalid")
	}
	if isValidInstallLocation(`C:\App`) {
		t.Fatal("depth 1 InstallLocation must be invalid")
	}
	if !isValidInstallLocation(`C:\Program Files\Acme\Tool`) {
		t.Fatal("depth 3 vendor path must be valid")
	}
	if !isValidInstallLocation(`C:\Games\MyGame`) {
		t.Fatal("depth 2 game path must be valid")
	}
	// Normalization: \\?\, env, dots, trailing slash
	t.Setenv("LOCALAPPDATA", `C:\Users\Bob\AppData\Local`)
	norm := normalizeProtectPath(`\\?\C:\Users\Bob\AppData\Local\Packages\X\`)
	if !strings.Contains(norm, `\appdata\local\packages`) {
		t.Fatalf("extended path normalize failed: %q", norm)
	}
	envNorm := normalizeProtectPath(`%LOCALAPPDATA%\Temp\..`)
	if envNorm != `c:\users\bob\appdata\local` {
		t.Fatalf("env+dot normalize: %q", envNorm)
	}
	if !IsProtectedSystemTarget(`C:\ProgramData\Microsoft`) {
		t.Fatal("ProgramData\\Microsoft must be protected")
	}
	if !IsProtectedSystemTarget(`C:\ProgramData\Microsoft\Windows Defender`) {
		t.Fatal("Defender under ProgramData must be protected")
	}
}

func TestResolveDeletePathRejectsEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if _, ok := resolveDeletePath(outside, root); ok {
		t.Fatal("escape outside allowlisted root must fail")
	}
	inside := filepath.Join(root, "cache.tmp")
	_ = os.WriteFile(inside, []byte("x"), 0o644)
	if _, ok := resolveDeletePath(inside, root); !ok {
		t.Fatal("file under allowlisted root must resolve")
	}
	if err := safeRemoveFile(inside, root, true); err != nil {
		t.Fatalf("dry-run remove: %v", err)
	}
	if _, err := os.Stat(inside); err != nil {
		t.Fatal("dry-run must not delete")
	}
}

func TestIsSafeCleanTargetAllowlist(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TEMP", tmp)
	t.Setenv("TMP", tmp)
	local := filepath.Join(tmp, "Local")
	_ = os.MkdirAll(filepath.Join(local, "D3DSCache"), 0o755)
	_ = os.MkdirAll(filepath.Join(local, "Google", "Chrome", "User Data", "Default", "Cache"), 0o755)
	_ = os.MkdirAll(filepath.Join(local, "Packages", "Evil"), 0o755)
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("APPDATA", filepath.Join(tmp, "Roaming"))
	t.Setenv("ProgramData", filepath.Join(tmp, "ProgramData"))

	if !IsSafeCleanTarget(tmp) {
		t.Fatal("user TEMP should be safe clean target")
	}
	if !IsSafeCleanTarget(filepath.Join(local, "D3DSCache")) {
		t.Fatal("D3DSCache should be allowed")
	}
	cache := filepath.Join(local, "Google", "Chrome", "User Data", "Default", "Cache")
	if !IsSafeCleanTarget(cache) {
		t.Fatal("browser Cache should be allowed")
	}
	if IsSafeCleanTarget(local) {
		t.Fatal("entire LocalAppData must be forbidden")
	}
	if IsSafeCleanTarget(filepath.Join(local, "Packages", "Evil")) {
		t.Fatal("Packages must be forbidden")
	}
	if IsSafeCleanTarget(`C:\Windows\System32`) {
		t.Fatal("System32 must be forbidden")
	}
	if IsSafeCleanTarget(`C:\Program Files`) {
		t.Fatal("Program Files must be forbidden")
	}
}

func TestEcoProtectedUsesGlobalGuard(t *testing.T) {
	for _, n := range []string{"explorer.exe", "MsMpEng.exe", "StartMenuExperienceHost.exe", "taskmgr.exe"} {
		if !isEcoProtected(n) {
			t.Fatalf("eco must protect %s via IsProtectedSystemTarget", n)
		}
		if !IsProtectedSystemTarget(n) {
			t.Fatalf("global guard missing %s", n)
		}
	}
}
