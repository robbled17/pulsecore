package main

import "testing"

func TestEvaluateHomeProblemsDiskFull(t *testing.T) {
	probs := EvaluateHomeProblems(HomeCatalogInput{
		SystemDiskUsed: 92, SystemDiskOK: true,
		RAMPercent: 40, RAMOK: true, HealthScore: 70,
	}, 3)
	if len(probs) == 0 || probs[0].ID != "disk_full" {
		t.Fatalf("want disk_full first, got %#v", probs)
	}
	if probs[0].ActionID != "free_disk" || probs[0].GainEstimate < 8 {
		t.Fatalf("action/gain: %#v", probs[0])
	}
	if probs[0].Title.Code != "homeDiskFullTitle" {
		t.Fatalf("title code: %#v", probs[0].Title)
	}
}

func TestEvaluateHomeProblemsEmptyWhenFine(t *testing.T) {
	probs := EvaluateHomeProblems(HomeCatalogInput{
		SystemDiskUsed: 50, SystemDiskOK: true,
		RAMPercent: 40, RAMOK: true, PowerMode: "balanced",
	}, 3)
	if len(probs) != 0 {
		t.Fatalf("expected no problems, got %#v", probs)
	}
}

func TestEvaluateHomeProblemsRAMEco(t *testing.T) {
	probs := EvaluateHomeProblems(HomeCatalogInput{
		SystemDiskUsed: 40, SystemDiskOK: true,
		RAMPercent: 90, RAMOK: true,
		TopRAMName: "chrome.exe", TopRAMFriendly: "Chrome", TopRAMEcoOK: true,
	}, 3)
	found := false
	for _, p := range probs {
		if p.ID == "ram_pressure" && p.ActionID == "eco_process" && p.ActionArg == "chrome.exe" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ram eco missing: %#v", probs)
	}
}

func TestEvaluateHomeProblemsCap3(t *testing.T) {
	probs := EvaluateHomeProblems(HomeCatalogInput{
		SystemDiskUsed: 95, SystemDiskOK: true,
		RAMPercent: 90, RAMOK: true, TopRAMName: "a.exe", TopRAMFriendly: "A", TopRAMEcoOK: true,
		LagCount: 2, LagCulprit: "b.exe", LagCulpritFriend: "B", LagCulpritEcoOK: true,
		StartupHighCount: 5, GameActive: true, PowerMode: "silent",
		TempOK: true, TempCPU: 95,
	}, 3)
	if len(probs) != 3 {
		t.Fatalf("len=%d", len(probs))
	}
}

func TestHomeStatusWord(t *testing.T) {
	if HomeStatusWord(90).Code != "homeStatusExcellent" {
		t.Fatal(HomeStatusWord(90))
	}
	if HomeStatusWord(70).Code != "homeStatusImprove" {
		t.Fatal(HomeStatusWord(70))
	}
	if HomeStatusWord(40).Code != "homeStatusAttention" {
		t.Fatal(HomeStatusWord(40))
	}
}
