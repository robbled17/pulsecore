package main

import (
	"strings"
	"testing"
)

func TestDesktopMaxFPSConcreteCards(t *testing.T) {
	pass := PCPassport{
		Chassis: "desktop", IsLaptop: false,
		CPUName: "AMD Ryzen 7 5700X", Socket: "AM4", Chipset: "B550",
		GPUName: "NVIDIA GeForce RTX 3060", VRAMGB: 12,
		RAMType: "DDR4", RAMTotalGB: 16, RAMSlotsUsed: 2, RAMSlotsTotal: 4, RAMSlotsFree: 2,
		HasFreeM2: true,
	}
	recs := buildUpgradeRecommendations(pass, UpgradeGoalMaxFPS, "ru")
	if len(recs) < 2 || len(recs) > 3 {
		t.Fatalf("want 2-3 recs, got %d", len(recs))
	}
	foundGPU := false
	for _, r := range recs {
		if r.Model == "" || r.EstimatedGain == "" {
			t.Fatalf("rec %s missing model/gain: %+v", r.ID, r)
		}
		if r.Part == "gpu" {
			foundGPU = true
			if r.Model != "RTX 4070 Super" {
				t.Fatalf("expected RTX 4070 Super for 3060, got %s", r.Model)
			}
			if r.EstimatedGain != "+60% FPS" {
				t.Fatalf("expected +60%% FPS, got %s", r.EstimatedGain)
			}
		}
		for _, p := range r.Pitfalls {
			if containsBrokenCaseTip(p) {
				t.Fatalf("broken pitfall still present: %s", p)
			}
		}
	}
	if !foundGPU {
		t.Fatal("expected a GPU recommendation")
	}
}

func TestLaptopNoSolderedCPU(t *testing.T) {
	pass := PCPassport{
		Chassis: "laptop", IsLaptop: true,
		CPUName: "Intel Core i7-12700H", Socket: "BGA",
		GPUName: "NVIDIA GeForce RTX 3060 Laptop GPU", VRAMGB: 6,
		RAMType: "DDR4", RAMTotalGB: 8, RAMSlotsUsed: 1, RAMSlotsTotal: 2, RAMSlotsFree: 1,
		IsSingleChannel: true, HasFreeM2: true,
		Storage: []StorageDrive{{Model: "NVMe 512", InterfaceType: "NVMe", MediaType: "SSD", SizeGB: 512}},
	}
	recs := buildUpgradeRecommendations(pass, UpgradeGoalMaxFPS, "en")
	if len(recs) < 2 || len(recs) > 3 {
		t.Fatalf("want 2-3 laptop recs, got %d", len(recs))
	}
	for _, r := range recs {
		if r.Part == "cpu" || r.Part == "gpu" {
			t.Fatalf("laptop must not suggest soldered %s upgrade", r.Part)
		}
		if r.Model == "" || r.EstimatedGain == "" {
			t.Fatalf("rec missing model/gain: %+v", r)
		}
	}
}

func TestPitfallsPlainTips(t *testing.T) {
	pass := PCPassport{Socket: "AM4", RAMType: "DDR4"}
	for _, part := range []string{"gpu", "cpu", "ram"} {
		for _, p := range commonPitfalls(pass, part, "ru") {
			if containsBrokenCaseTip(p) {
				t.Fatalf("broken tip in %s: %s", part, p)
			}
		}
	}
	gpu := commonPitfalls(pass, "gpu", "ru")[0]
	if !strings.Contains(gpu, "корпус") || !strings.Contains(gpu, "питания") {
		t.Fatalf("gpu pitfall should mention case and PSU: %s", gpu)
	}
	cpu := commonPitfalls(pass, "cpu", "ru")[0]
	if !strings.Contains(cpu, "BIOS") {
		t.Fatalf("cpu pitfall should mention BIOS: %s", cpu)
	}
	ram := commonPitfalls(pass, "ram", "ru")[1]
	if !strings.Contains(strings.ToLower(ram), "частот") && !strings.Contains(ram, "тайминг") {
		t.Fatalf("ram pitfall should mention freq/timings: %s", ram)
	}
}

func containsBrokenCaseTip(s string) bool {
	low := strings.ToLower(s)
	return strings.Contains(s, "длину корпуса") || strings.Contains(s, "длина корпуса") ||
		strings.Contains(low, "case length")
}
