package main

import "testing"

func TestNoiseBackgroundProcessFilterList(t *testing.T) {
	noise := []string{
		"MsMpEng.exe", "NisSrv.exe", "SecurityHealthService.exe",
		"SearchIndexer.exe", "SearchProtocolHost.exe",
		"System", "svchost.exe", "csrss.exe", "lsass.exe",
		"dwm.exe", "explorer.exe", "SmartScreen.exe",
	}
	for _, n := range noise {
		if !isNoiseBackgroundProcess(n) {
			t.Fatalf("expected noise/protected: %q", n)
		}
		if !IsProtectedSystemTarget(n) {
			t.Fatalf("expected IsProtectedSystemTarget: %q", n)
		}
	}
	ok := []string{"chrome.exe", "Discord.exe", "Game.exe", "firefox.exe"}
	for _, n := range ok {
		if isNoiseBackgroundProcess(n) {
			t.Fatalf("user process flagged as noise: %q", n)
		}
	}
}

func TestEcoNeverTargetsNoise(t *testing.T) {
	for _, n := range []string{"MsMpEng.exe", "NisSrv.exe", "SearchIndexer.exe", "svchost.exe", "System"} {
		if !isEcoProtected(n) && !isPPLProtected(n) && !IsProtectedSystemTarget(n) {
			t.Fatalf("Eco path must refuse %q", n)
		}
	}
}
