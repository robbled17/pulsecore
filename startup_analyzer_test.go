package main

import (
	"testing"
)

func TestExtractExePath(t *testing.T) {
	if got := extractExePath(`"C:\Program Files\App\app.exe" --flag`); got != `C:\Program Files\App\app.exe` {
		t.Fatalf("quoted: %q", got)
	}
	if got := extractExePath(`C:\Tools\a.exe /silent`); got != `C:\Tools\a.exe` {
		t.Fatalf("bare: %q", got)
	}
}

func TestGuessStartupImpact(t *testing.T) {
	if guessStartupImpact("discord update helper") != "high" && guessStartupImpact("discord update helper") != "medium" {
		// Discord is messenger → medium in category table.
		if classify, _, impact := classifyStartupCategory("discord update helper"); classify != "messenger" || impact == "" {
			t.Fatalf("discord category/impact unexpected: %s %s", classify, impact)
		}
	}
	if got := guessStartupImpact("steamclient"); got != "high" {
		t.Fatalf("steam launcher should be high, got %q", got)
	}
}

func TestParseStartupCommand(t *testing.T) {
	p, ok := parseStartupCommand(`"C:\Program Files\App\app.exe" --flag`)
	if !ok || p != `C:\Program Files\App\app.exe` {
		t.Fatalf("quoted: %q ok=%v", p, ok)
	}
	p, ok = parseStartupCommand(`C:\Tools\a.exe /silent`)
	if !ok || p != `C:\Tools\a.exe` {
		t.Fatalf("bare exe: %q ok=%v", p, ok)
	}
}

func TestMergeStartupDuplicates(t *testing.T) {
	out := mergeStartupDuplicates([]StartupItem{
		{ID: "hkcu_run|steam", Name: "Steam", Path: `C:\Steam\steam.exe`, Enabled: true, CanDisable: true, Location: "hkcu_run"},
		{ID: "startup_user|steam.lnk", Name: "Steam", Path: `C:\Steam\steam.exe`, Enabled: true, CanDisable: true, Location: "startup_user"},
	})
	if len(out) != 1 {
		t.Fatalf("expected 1 after merge, got %d", len(out))
	}
}

func TestIsCriticalStartup(t *testing.T) {
	if !isCriticalStartup("securityhealth", "securityhealthsystray.exe", `c:\windows\system32\securityhealthsystray.exe`) {
		t.Fatal("security health must be critical")
	}
	if isCriticalStartup("myapp", "myapp.exe", `c:\users\me\myapp.exe`) {
		t.Fatal("user app must not be critical")
	}
}

func TestEnrichStartupItemBlocksCritical(t *testing.T) {
	it := StartupItem{
		Name: "SecurityHealth", Command: `C:\Windows\System32\SecurityHealthSystray.exe`,
		Path: `C:\Windows\System32\SecurityHealthSystray.exe`, Location: "hkcu_run", CanDisable: true, Enabled: true,
	}
	enrichStartupItem(&it)
	if !it.Critical || it.CanDisable {
		t.Fatalf("critical=%v canDisable=%v", it.Critical, it.CanDisable)
	}
}

func TestEnrichStartupItemHKLM(t *testing.T) {
	it := StartupItem{Name: "Foo", Command: `C:\Foo\foo.exe`, Path: `C:\Foo\foo.exe`, Location: "hklm_run", CanDisable: true}
	enrichStartupItem(&it)
	if it.CanDisable {
		t.Fatal("HKLM must not be disableable without admin")
	}
}
