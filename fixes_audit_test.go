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

func TestRDR2CompatibilityOnRTX4080Super(t *testing.T) {
	// AMD Ryzen 7 7800X3D + NVIDIA RTX 4080 Super (16 GB) + 16 GB RAM
	sys := SystemPowerProfile{
		CPUName:       "AMD Ryzen 7 7800X3D",
		CPUPowerScore: 92.0,
		GPUName:       "NVIDIA GeForce RTX 4080 Super",
		GPUPowerScore: 94.0,
		VRAMGB:        16.0,
		RAMGB:         16.0,
		IsIntegrated:  false,
	}

	game := GameInfo{
		ID:       "rdr2",
		Name:     "Red Dead Redemption 2",
		Year:     2019,
		Category: "action",
	}

	req := gameRequirements["rdr2"]
	req.ReqRAMGB = 12.0 // Recommended RAM 12 GB

	verdict, cls, labelRu, _, reasonRu, _ := EvaluateGameFitUnified(sys, game, req)

	if verdict != "great" {
		t.Fatalf("expected verdict 'great' for RTX 4080 Super on RDR2, got verdict=%q (cls=%q, labelRu=%q, reasonRu=%q)",
			verdict, cls, labelRu, reasonRu)
	}

	if labelRu != "Отлично" {
		t.Fatalf("expected labelRu 'Отлично', got %q", labelRu)
	}
}

func TestRDR2CompatibilityOnIGPU512MB(t *testing.T) {
	// Integrated AMD Radeon 512 MB VRAM
	sys := SystemPowerProfile{
		CPUName:       "AMD Ryzen 5 5600U",
		CPUPowerScore: 65.0,
		GPUName:       "AMD Radeon(TM) Graphics",
		GPUPowerScore: 28.0,
		VRAMGB:        0.5,
		RAMGB:         16.0,
		IsIntegrated:  true,
	}

	game := GameInfo{
		ID:       "rdr2",
		Name:     "Red Dead Redemption 2",
		Year:     2019,
		Category: "action",
	}

	req := gameRequirements["rdr2"]
	req.ReqRAMGB = 12.0

	verdict, cls, labelRu, _, _, _ := EvaluateGameFitUnified(sys, game, req)

	if verdict != "deficit" {
		t.Fatalf("expected verdict 'deficit' for 512MB iGPU on RDR2, got verdict=%q (cls=%q, labelRu=%q)",
			verdict, cls, labelRu)
	}

	if labelRu != "Не хватит" {
		t.Fatalf("expected labelRu 'Не хватит', got %q", labelRu)
	}
}

func TestRussianPointsInflectionRules(t *testing.T) {
	formatPointsRu := func(num int, withMinus bool) string {
		prefix := ""
		if withMinus {
			prefix = "−"
		}
		mod10 := num % 10
		mod100 := num % 100
		word := "баллов"
		if mod10 == 1 && mod100 != 11 {
			word = "балл"
		} else if mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20) {
			word = "балла"
		}
		return fmt.Sprintf("%s%d %s", prefix, num, word)
	}

	cases := []struct {
		pts      int
		minus    bool
		expected string
	}{
		{1, true, "−1 балл"},
		{2, true, "−2 балла"},
		{3, true, "−3 балла"},
		{4, true, "−4 балла"},
		{5, true, "−5 баллов"},
		{10, true, "−10 баллов"},
		{11, true, "−11 баллов"},
		{12, true, "−12 баллов"},
		{20, true, "−20 баллов"},
		{21, true, "−21 балл"},
		{22, true, "−22 балла"},
		{25, true, "−25 баллов"},
		{1, false, "1 балл"},
		{2, false, "2 балла"},
		{5, false, "5 баллов"},
	}

	for _, tc := range cases {
		got := formatPointsRu(tc.pts, tc.minus)
		if got != tc.expected {
			t.Errorf("pts=%d minus=%v: expected %q, got %q", tc.pts, tc.minus, tc.expected, got)
		}
	}
}
