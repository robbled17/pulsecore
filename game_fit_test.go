package main

import "testing"

func TestEvaluateGameFit_Excellent(t *testing.T) {
	sys := SystemPowerProfile{GPUPowerScore: 85, CPUPowerScore: 70, VRAMGB: 12, RAMGB: 32, CPUCores: 8}
	req := gameRequirements["cyberpunk"]
	fit := EvaluateGameFit(sys, GameInfo{ID: "cyberpunk", Category: "aaa"}, req, nil)
	if fit.Level != "excellent" && fit.Level != "ok" {
		t.Fatalf("want excellent/ok, got %s (%s)", fit.Level, fit.ReasonRu)
	}
	if !fit.Approximate {
		t.Fatal("must be marked approximate")
	}
}

func TestEvaluateGameFit_WontOnWeakGPU(t *testing.T) {
	sys := SystemPowerProfile{GPUPowerScore: 12, CPUPowerScore: 40, VRAMGB: 2, RAMGB: 8, CPUCores: 4}
	req := gameRequirements["cyberpunk"]
	fit := EvaluateGameFit(sys, GameInfo{ID: "cyberpunk", Category: "aaa"}, req, nil)
	if fit.Level != "wont" && fit.Level != "tight" {
		t.Fatalf("want wont/tight, got %s reason=%s", fit.Level, fit.ReasonRu)
	}
}

func TestEvaluateGameFit_UnknownWhenNoGPU(t *testing.T) {
	sys := SystemPowerProfile{GPUPowerScore: 0, CPUPowerScore: 0}
	req := gameRequirements["cs2"]
	fit := EvaluateGameFit(sys, GameInfo{ID: "cs2", Category: "shooter"}, req, nil)
	if fit.Level != "unknown" {
		t.Fatalf("want unknown, got %s", fit.Level)
	}
}

func TestEvaluateGameFit_DownloadAdvice(t *testing.T) {
	sys := SystemPowerProfile{GPUPowerScore: 60, CPUPowerScore: 50, VRAMGB: 8, RAMGB: 16, CPUCores: 6}
	disks := []DiskFreeSnap{{Mount: "C:", FreeGB: 20}, {Mount: "D:", FreeGB: 200}}
	fit := EvaluateGameFit(sys, GameInfo{ID: "gtav", Category: "aaa", SteamAppID: 271590}, gameRequirements["gtav"], disks)
	if !fit.StorageNeedOK || fit.StorageNeedGB < 50 {
		t.Fatalf("storage: %+v", fit)
	}
	if fit.RecommendMount != "D:" {
		t.Fatalf("want D:, got %s", fit.RecommendMount)
	}
	if fit.DownloadRu == "" || fit.DownloadEn == "" {
		t.Fatal("missing download text")
	}
}

func TestFitLevelToFilter(t *testing.T) {
	if fitLevelToFilter("excellent") != "ok" || fitLevelToFilter("tight") != "warn" || fitLevelToFilter("wont") != "bad" {
		t.Fatal("filter mapping")
	}
}

func TestEvaluateGameFit_Pre2017ExcellentOn6GB(t *testing.T) {
	sys := SystemPowerProfile{GPUPowerScore: 55, CPUPowerScore: 45, VRAMGB: 6, RAMGB: 16, CPUCores: 6}
	req := requirementsFor(GameInfo{ID: "gtav", Name: "GTA V", Category: "aaa", Year: 2015, SteamAppID: 271590})
	fit := EvaluateGameFit(sys, GameInfo{ID: "gtav", Name: "GTA V", Category: "aaa", Year: 2015, SteamAppID: 271590}, req, nil)
	if fit.Level != "excellent" && fit.Level != "ok" {
		t.Fatalf("pre-2017 on 6GB VRAM must not be tight, got %s (%s)", fit.Level, fit.ReasonRu)
	}
	if req.High.ReqVRAMGB > 3 {
		t.Fatalf("year cap failed: High VRAM=%.1f", req.High.ReqVRAMGB)
	}
}
