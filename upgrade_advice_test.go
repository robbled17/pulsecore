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

func TestLaptopNoDesktopCPUGPU(t *testing.T) {
	pass := PCPassport{
		Chassis: "laptop", IsLaptop: true,
		CPUName: "AMD Ryzen 7 5800H", Socket: "BGA",
		GPUName: "NVIDIA GeForce RTX 3060 Laptop GPU", VRAMGB: 6,
		RAMType: "DDR4", RAMTotalGB: 16, RAMSlotsUsed: 2, RAMSlotsTotal: 2, RAMSlotsFree: 0,
		IsSingleChannel: false, HasFreeM2: true,
		Storage: []StorageDrive{{Model: "NVMe 512", InterfaceType: "NVMe", MediaType: "SSD", SizeGB: 512}},
	}
	for _, goal := range []string{UpgradeGoalMaxFPS, UpgradeGoalQuiet, UpgradeGoalWork, UpgradeGoalBudget} {
		recs := buildUpgradeRecommendations(pass, goal, "en")
		if len(recs) < 2 || len(recs) > 3 {
			t.Fatalf("goal %s: want 2-3 laptop recs, got %d", goal, len(recs))
		}
		for _, r := range recs {
			if r.Part == "cpu" || r.Part == "gpu" {
				t.Fatalf("goal %s: laptop must not suggest %s (%s / %s)", goal, r.Part, r.Model, r.Title)
			}
			low := strings.ToLower(r.Model + " " + r.Title + " " + r.Body)
			if strings.Contains(low, "4070") || strings.Contains(low, "5700x3d") ||
				strings.Contains(low, "5800x3d") || strings.Contains(low, "am4") {
				t.Fatalf("goal %s: desktop part leaked into laptop rec: %+v", goal, r)
			}
			if r.Part == "ram" {
				t.Fatalf("goal %s: no free slots - must not recommend RAM, got %+v", goal, r)
			}
		}
	}
}

func TestLaptopHeuristicFromMobileMarkers(t *testing.T) {
	// Chassis wrongly reported as desktop, but CPU/GPU scream laptop.
	pass := PCPassport{
		Chassis: "desktop", IsLaptop: false,
		CPUName: "AMD Ryzen 7 5800H", Socket: "AM4",
		GPUName: "NVIDIA GeForce RTX 3060 Laptop GPU", VRAMGB: 6,
		RAMType: "DDR4", RAMTotalGB: 16, RAMSlotsUsed: 1, RAMSlotsTotal: 2, RAMSlotsFree: 1,
		HasFreeM2: true,
	}
	recs := buildUpgradeRecommendations(pass, UpgradeGoalMaxFPS, "ru")
	for _, r := range recs {
		if r.Part == "cpu" || r.Part == "gpu" {
			t.Fatalf("heuristic laptop must not get desktop %s: %+v", r.Part, r)
		}
	}
	hasRAM := false
	for _, r := range recs {
		if r.Part == "ram" {
			hasRAM = true
		}
	}
	if !hasRAM {
		t.Fatal("expected RAM rec when free slots > 0 on heuristic laptop")
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

func TestRAMSlotsFreeConsistency(t *testing.T) {
	cases := []struct {
		used, total, wantFree int
	}{
		{2, 2, 0},
		{1, 2, 1},
		{2, 4, 2},
		{0, 0, 0},
		{5, 2, 0}, // clamp used
	}
	for _, c := range cases {
		used, total, free := normalizeRAMSlots(c.used, c.total)
		if free != c.wantFree {
			t.Fatalf("normalize(%d,%d) free=%d want %d", c.used, c.total, free, c.wantFree)
		}
		hw := buildRAMDeepInfo(staticHardware{ramSlotsUsed: c.used, ramSlotsTotal: c.total}, 16384, 8000, 8000, 50)
		if hw.SlotsUsed != used || hw.SlotsTotal != total {
			t.Fatalf("hardware slots mismatch: hw=%d/%d norm=%d/%d", hw.SlotsUsed, hw.SlotsTotal, used, total)
		}
		pass := PCPassport{
			Chassis: "laptop", IsLaptop: true,
			CPUName: "AMD Ryzen 7 5800H", GPUName: "RTX 3060 Laptop GPU",
			RAMType: "DDR4", RAMTotalGB: 16,
			RAMSlotsUsed: used, RAMSlotsTotal: total, RAMSlotsFree: free,
			HasFreeM2: true,
		}
		recs := buildUpgradeRecommendations(pass, UpgradeGoalMaxFPS, "en")
		for _, r := range recs {
			if r.Part == "ram" && free == 0 {
				t.Fatalf("free=0 but RAM recommended: %+v", r)
			}
			body := strings.ToLower(r.Body)
			if strings.Contains(body, "free slots: 2") && free == 0 {
				t.Fatalf("body claims free slots: 2 when free=0: %s", r.Body)
			}
		}
		if free != (hw.SlotsTotal - hw.SlotsUsed) && hw.SlotsTotal >= hw.SlotsUsed {
			t.Fatalf("passport free %d != hardware free %d", free, hw.SlotsTotal-hw.SlotsUsed)
		}
	}
}

func TestMobileCPUNameMarkers(t *testing.T) {
	mobile := []string{
		"AMD Ryzen 7 5800H", "AMD Ryzen 9 7940HS", "Intel Core i7-12700H",
		"Intel Core i9-13900HX", "AMD Ryzen 5 5600U", "Intel Core i5-1135G7",
	}
	for _, n := range mobile {
		if !mobileCPUName(n) {
			t.Fatalf("expected mobile CPU: %s", n)
		}
	}
	desktop := []string{"AMD Ryzen 7 5700X", "AMD Ryzen 7 5700X3D", "Intel Core i7-12700K", "Intel Core i5-12400F"}
	for _, n := range desktop {
		if mobileCPUName(n) {
			t.Fatalf("desktop CPU flagged mobile: %s", n)
		}
	}
	if !mobileGPUName("NVIDIA GeForce RTX 3060 Laptop GPU") {
		t.Fatal("expected laptop GPU marker")
	}
	if mobileGPUName("NVIDIA GeForce RTX 3060") {
		t.Fatal("desktop 3060 must not be laptop GPU")
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
