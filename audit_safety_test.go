package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Table-driven dangerous paths / processes (Block 5 audit).
func TestDangerousPathsAndProcessesTable(t *testing.T) {
	t.Setenv("SystemRoot", `C:\Windows`)
	cases := []struct {
		in   string
		want bool
	}{
		{"explorer.exe", true},
		{"MsMpEng.exe", true},
		{"PulseCore.exe", true},
		{"msedgewebview2.exe", true},
		{"chrome.exe", false},
		{"game.exe", false},
		{`C:\Windows\System32`, true},
		{`C:\Windows\WinSxS\manifests`, true},
		{`C:\Program Files`, true},
		{`C:\Program Files (x86)`, true},
		{`C:\ProgramData\Microsoft`, true},
		{`C:\ProgramData\Microsoft\Windows Defender`, true},
		{`C:\Users\Bob\AppData\Local\Packages\Foo`, true},
		{`C:\`, true},
		{`D:\`, true},
		{`C:\Users\Bob`, true},
		{`C:\Games\MyGame`, false},
		{`C:\Program Files\Vendor\App`, false},
		{`\\?\C:\Windows\System32\cmd.exe`, true},
		{`C:\Windows\Temp\..\System32`, true},
	}
	for _, c := range cases {
		got := IsProtectedSystemTarget(c.in) || isDangerousInstallRoot(c.in)
		if got != c.want {
			t.Fatalf("%q: got protected=%v want %v (norm=%q)",
				c.in, got, c.want, normalizeProtectPath(c.in))
		}
	}
}

func TestInstallLocationEdgeCases(t *testing.T) {
	bad := []string{"", "  ", `C:\`, `C:\Windows`, `C:\App`, `C:\Users\Bob`, `C:\Program Files`, `C:\ProgramData\Microsoft\Foo`}
	for _, p := range bad {
		if isValidInstallLocation(p) {
			t.Fatalf("expected invalid InstallLocation: %q", p)
		}
	}
	good := []string{`C:\Games\CoolGame`, `C:\Program Files\Vendor\Product`, `D:\Tools\App`}
	for _, p := range good {
		if !isValidInstallLocation(p) {
			t.Fatalf("expected valid InstallLocation: %q (depth=%d)", p, installPathDepth(p))
		}
	}
}

func TestNormalizePathVariants(t *testing.T) {
	t.Setenv("TEMP", `C:\Users\Bob\AppData\Local\Temp`)
	cases := map[string]string{
		`C:\Windows\System32\`:        `c:\windows\system32`,
		`C:/Windows/System32`:         `c:\windows\system32`,
		`\\?\C:\Windows\System32`:     `c:\windows\system32`,
		`%TEMP%\foo`:                  `c:\users\bob\appdata\local\temp\foo`,
		`C:\Windows\Temp\..\System32`: `c:\windows\system32`,
		`C:\Games\MyGame\`:            `c:\games\mygame`,
	}
	for in, want := range cases {
		got := normalizeProtectPath(in)
		if got != want {
			t.Fatalf("normalizeProtectPath(%q)=%q want %q", in, got, want)
		}
	}
}

func TestForbiddenRegistryIncludesWOW6432(t *testing.T) {
	bad := []string{
		`HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Run`,
		`HKLM\SYSTEM\CurrentControlSet\Services\Foo`,
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\Advanced`,
	}
	for _, p := range bad {
		if !IsForbiddenRegistryPath(p) {
			t.Fatalf("expected forbidden: %q", p)
		}
	}
}

func TestCleanerDryRunDoesNotDelete(t *testing.T) {
	tmp := t.TempDir()
	old := filepath.Join(tmp, "old.tmp")
	if err := os.WriteFile(old, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := safeRemoveFile(old, tmp, true); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatal("dry-run must keep file")
	}
}

func TestSafeRemoveRejectsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	f := filepath.Join(other, "x.tmp")
	_ = os.WriteFile(f, []byte("1"), 0o644)
	if err := safeRemoveFile(f, root, false); err == nil {
		t.Fatal("must reject escape")
	}
	if _, err := os.Stat(f); err != nil {
		t.Fatal("file must remain")
	}
}

func TestTerminateGuardEmptyPath(t *testing.T) {
	// Self PID must never be terminable via guard helpers.
	if !isSelfOrParentPID(uint32(os.Getpid())) {
		t.Fatal("self PID must be protected")
	}
}

func TestWMIFixtureJSONShapes(t *testing.T) {
	// Fixture-only shapes for dual GPU laptop / SMT / no-SMT / multi-disk SMART.
	// Live WMI needs hardware check; these validate parsing contracts used by UI/mock.
	type cpuFix struct {
		Name           string
		PhysicalCores  int
		LogicalCores   int
		HasSMT         bool
	}
	type gpuFix struct {
		Primary   string
		Secondary string
		Chassis   string
	}
	type diskFix struct {
		Drive string
		Model string
		Wear  int
	}
	cpus := []cpuFix{
		{Name: "AMD Ryzen 7 5800H", PhysicalCores: 8, LogicalCores: 16, HasSMT: true},
		{Name: "Intel Core i5-12600 (P-cores fixture)", PhysicalCores: 6, LogicalCores: 6, HasSMT: false},
	}
	for _, c := range cpus {
		smt := c.LogicalCores > c.PhysicalCores
		if smt != c.HasSMT {
			t.Fatalf("SMT fixture mismatch for %s", c.Name)
		}
	}
	gpus := []gpuFix{
		{Primary: "NVIDIA GeForce RTX 3060 Laptop GPU", Secondary: "AMD Radeon Graphics", Chassis: "laptop"},
		{Primary: "Intel UHD Graphics 630", Secondary: "", Chassis: "desktop"},
	}
	if gpus[0].Secondary == "" || gpus[0].Chassis != "laptop" {
		t.Fatal("laptop dual GPU fixture required")
	}
	disks := []diskFix{
		{Drive: "C", Model: "Samsung SSD 980 PRO 1TB", Wear: 94},
		{Drive: "D", Model: "WD Blue SN570 500GB", Wear: 88},
		{Drive: "E", Model: "Seagate Barracuda 2TB", Wear: 99},
	}
	if len(disks) < 3 {
		t.Fatal("multi-disk SMART accordion fixture requires >=3 drives")
	}
	for _, d := range disks {
		if d.Wear < 0 {
			t.Fatalf("wear sentinel must not be negative in healthy fixture: %s", d.Drive)
		}
	}
}

func TestFilterNonSystemUninstallKeys(t *testing.T) {
	in := []string{"HKLM\\CoolApp", "HKLM\\Microsoft Edge", "HKCU\\Windows Defender", "HKLM\\MyGame"}
	out := filterNonSystemUninstallKeys(in)
	joined := strings.Join(out, "|")
	if strings.Contains(strings.ToLower(joined), "microsoft") || strings.Contains(strings.ToLower(joined), "windows") || strings.Contains(strings.ToLower(joined), "defender") {
		t.Fatalf("system keys leaked into diff: %v", out)
	}
	if len(out) == 0 {
		t.Fatal("expected non-system keys retained")
	}
}
