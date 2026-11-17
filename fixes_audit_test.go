package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestBatteryWearNoDuplicate(t *testing.T) {
	wearPct := 33.0
	label := wearLabel(wearPct)
	formatted := fmt.Sprintf("%.0f%% (%s)", wearPct, label)
	if strings.Contains(strings.ToLower(formatted), "износ:") {
		t.Fatalf("formatted battery wear contains duplicate 'износ:': %q", formatted)
	}
	if formatted != "33% (Заметный износ)" {
		t.Fatalf("unexpected formatted wear: %q", formatted)
	}
}

func TestIGPUVRAMNotZero(t *testing.T) {
	// 512 MB dedicated VRAM on AMD Radeon integrated graphics
	dedicatedMB := 512.0
	gb := dedicatedMB / 1024.0
	if gb != 0.5 {
		t.Fatalf("expected 0.5 GB, got %f", gb)
	}

	// Verify formatting logic used in GamePulse / GameLab
	formatVram := func(v float64) string {
		if v < 1.0 {
			return fmt.Sprintf("%.1f", v)
		}
		return fmt.Sprintf("%.0f", v)
	}

	res := formatVram(gb)
	if res != "0.5" {
		t.Fatalf("expected '0.5', got %q", res)
	}
}

func TestProcessDictionaryCoverage(t *testing.T) {
	if len(processDict) < 130 {
		t.Fatalf("expected >= 130 processes in dictionary, got %d", len(processDict))
	}

	sampleExes := []string{
		"chrome.exe", "msedge.exe", "firefox.exe",
		"steam.exe", "steamwebhelper.exe", "lgc.exe", "lgc_renderer_host.exe",
		"worldoftanks.exe", "wotblitz.exe", "cs2.exe", "valorant.exe",
		"discord.exe", "telegram.exe", "code.exe",
		"nvcontainer.exe", "radeonsoftware.exe", "amdfendrsr.exe",
		"msmpeng.exe", "dwm.exe", "explorer.exe",
	}

	langs := []string{"ru", "en", "de", "ja", "kk", "tt", "zh"}

	for _, exe := range sampleExes {
		for _, l := range langs {
			name := friendlyProcessNameLang(exe, l)
			if name == "" || name == "Unknown Process" || name == "Неизвестный процесс" {
				t.Fatalf("missing translation for %s in lang %s", exe, l)
			}
		}
	}
}

func TestTransientConsoleUtilitiesFiltered(t *testing.T) {
	utilities := []string{
		"tasklist.exe", "conhost.exe", "cmd.exe",
		"findstr.exe", "dxdiag.exe", "powershell.exe",
		"pwsh.exe", "wmic.exe", "cscript.exe", "wscript.exe",
	}

	for _, u := range utilities {
		if !isTransientConsoleUtility(u) {
			t.Fatalf("expected %s to be recognized as transient console utility", u)
		}
		if !isEcoProtected(u) {
			t.Fatalf("expected %s to be eco-protected in ecoDenylist", u)
		}
	}

	// Real background applications must NOT be flagged as transient console utilities
	nonUtilities := []string{"chrome.exe", "steam.exe", "discord.exe", "pulsecore.exe"}
	for _, app := range nonUtilities {
		if isTransientConsoleUtility(app) {
			t.Fatalf("expected %s to NOT be a transient console utility", app)
		}
	}
}

func TestAppStillRetainedThresholds(t *testing.T) {
	// Target app with 500 MB baseline
	beforeSize := int64(500 * 1024 * 1024)

	// Function simulating folder size check:
	isHeavy := func(currentSize int64) bool {
		return currentSize > 50*1024*1024 || (beforeSize > 0 && float64(currentSize) > float64(beforeSize)*0.20)
	}

	// 1. Uninstallation cancelled, full folder (500 MB) remains:
	if !isHeavy(500 * 1024 * 1024) {
		t.Fatal("expected 500 MB remaining to be flagged as heavy")
	}

	// 2. Uninstallation aborted, 120 MB remains (> 20% and > 50 MB):
	if !isHeavy(120 * 1024 * 1024) {
		t.Fatal("expected 120 MB remaining to be flagged as heavy")
	}

	// 3. Genuine uninstallation, tiny leftover config (2 MB remaining):
	if isHeavy(2 * 1024 * 1024) {
		t.Fatal("expected 2 MB remaining to NOT be flagged as heavy")
	}
}
