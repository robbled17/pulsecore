package main

import (
	"strings"
	"testing"
)

func TestHardwareDBScaleCalibration(t *testing.T) {
	db := GetHardwareDB()
	if db == nil || len(db.GPUs) == 0 || len(db.CPUs) == 0 {
		t.Fatalf("hardware database failed to load from embedded JSON")
	}

	// 100 = GTX 1060 / Ryzen 5 2600
	gtx1060 := ResolveGPUScore("NVIDIA GeForce GTX 1060 6GB", false, 6)
	if gtx1060 != 100 {
		t.Errorf("GTX 1060 score = %v, want 100", gtx1060)
	}

	r5_2600 := ResolveCPUScore("AMD Ryzen 5 2600 6-Core Processor", 6, 12, 3400)
	if r5_2600 != 100 {
		t.Errorf("Ryzen 5 2600 score = %v, want 100", r5_2600)
	}

	// 175 = RTX 2060 SUPER / Ryzen 5 3600
	rtx2060s := ResolveGPUScore("NVIDIA GeForce RTX 2060 SUPER", false, 8)
	if rtx2060s != 175 {
		t.Errorf("RTX 2060 SUPER score = %v, want 175", rtx2060s)
	}

	r5_3600 := ResolveCPUScore("AMD Ryzen 5 3600 6-Core Processor", 6, 12, 3600)
	if r5_3600 != 175 {
		t.Errorf("Ryzen 5 3600 score = %v, want 175", r5_3600)
	}

	// iGPU scale (10-38)
	hd4000 := ResolveGPUScore("Intel(R) HD Graphics 4000", true, 1)
	if hd4000 < 10 || hd4000 > 25 {
		t.Errorf("Intel HD 4000 score = %v, want ~10-25", hd4000)
	}

	uhd630 := ResolveGPUScore("Intel(R) UHD Graphics 630", true, 1)
	if uhd630 < 10 || uhd630 > 25 {
		t.Errorf("Intel UHD 630 score = %v, want ~10-25", uhd630)
	}

	// High-end scale (300-560)
	rtx4070ti := ResolveGPUScore("NVIDIA GeForce RTX 4070 Ti", false, 12)
	if rtx4070ti < 320 {
		t.Errorf("RTX 4070 Ti score = %v, want >= 320", rtx4070ti)
	}

	r7_7800x3d := ResolveCPUScore("AMD Ryzen 7 7800X3D 8-Core Processor", 8, 16, 4200)
	if r7_7800x3d < 320 {
		t.Errorf("Ryzen 7 7800X3D score = %v, want >= 320", r7_7800x3d)
	}

	rtx4090 := ResolveGPUScore("NVIDIA GeForce RTX 4090", false, 24)
	if rtx4090 < 490 {
		t.Errorf("RTX 4090 score = %v, want >= 490", rtx4090)
	}
}

func TestCleanHardwareString(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"NVIDIA(R) GeForce(R) RTX 2060 SUPER 8GB", "rtx 2060 super"},
		{"Intel(R) Core(TM) i5-10400F CPU @ 2.90GHz", "core i5-10400f cpu @ 2.90ghz"},
		{"AMD Radeon(TM) RX 580 8 GB", "rx 580"},
	}

	for _, c := range cases {
		got := CleanHardwareString(c.input)
		if !strings.Contains(got, c.expected) {
			t.Errorf("CleanHardwareString(%q) = %q, want containing %q", c.input, got, c.expected)
		}
	}
}

func TestAlternativeHardwareResolution(t *testing.T) {
	// GTX 1060 (100) / RX 580 (96) -> minimum threshold should be 96
	gpuScore := ResolveGPUScore("NVIDIA GeForce GTX 1060 6GB / AMD Radeon RX 580 8GB", false, 6)
	if gpuScore != 96 {
		t.Errorf("ResolveGPUScore(GTX 1060 / RX 580) = %v, want 96 (min threshold)", gpuScore)
	}

	// Ryzen 5 2600 (100) or Core i5-8400 (102) -> min threshold should be 100
	cpuScore := ResolveCPUScore("AMD Ryzen 5 2600 or Intel Core i5-8400", 6, 12, 3400)
	if cpuScore != 100 {
		t.Errorf("ResolveCPUScore(Ryzen 5 2600 or Core i5-8400) = %v, want 100", cpuScore)
	}
}

func TestUnknownHardwareFallbackNeverZero(t *testing.T) {
	unknownGPU := ResolveGPUScore("Some Future Custom Discrete GPU 16GB", false, 16)
	if unknownGPU <= 0 {
		t.Fatalf("Unknown GPU score must never be 0, got %v", unknownGPU)
	}

	unknownLaptopGPU := ResolveGPUScore("Unknown Generic Mobile GPU Laptop", false, 4)
	if unknownLaptopGPU <= 0 {
		t.Fatalf("Unknown Laptop GPU score must never be 0, got %v", unknownLaptopGPU)
	}

	unknownCPU := ResolveCPUScore("Custom Unknown 8-Core Workstation CPU", 8, 16, 3200)
	if unknownCPU <= 0 {
		t.Fatalf("Unknown CPU score must never be 0, got %v", unknownCPU)
	}
}

func TestEvaluateGameCompatibilityVerdicts(t *testing.T) {
	cyberpunkReq := GameHardwareReq{
		MinCPU:  "Core i7-6700 or Ryzen 5 1600",
		RecCPU:  "Core i7-12700 or Ryzen 7 7800X3D",
		MinGPU:  "GTX 1060 6GB / RX 580",
		RecGPU:  "RTX 2060 SUPER / RX 5700 XT",
		MinRAM:  12,
		RecRAM:  16,
		MinVRAM: 6,
		RecVRAM: 8,
	}

	// Case 1: Low-end iGPU -> FAIL (НЕ ПОТЯНЕТ)
	igpuSys := SystemHardwareInput{
		CPUName:      "Intel Core i3-6100",
		CPUCores:     2,
		CPUThreads:   4,
		CPUFreqMHz:   3700,
		GPUName:      "Intel HD Graphics 530",
		GPUVRAMGB:    1,
		IsIntegrated: true,
		SystemRAMGB:  8,
	}
	verdictFail := EvaluateGameCompatibility(igpuSys, cyberpunkReq)
	if verdictFail.VerdictCode != "fail" || verdictFail.Verdict != "НЕ ПОТЯНЕТ" {
		t.Errorf("Expected fail verdict for iGPU on Cyberpunk, got %v (%s)", verdictFail.VerdictCode, verdictFail.Verdict)
	}

	// Case 2: Target PC: Ryzen 5 3600 + RTX 2060 SUPER 8GB + 16GB RAM -> HIGH (ВЫСОКИЕ · 60+ FPS)
	targetSys := SystemHardwareInput{
		CPUName:      "AMD Ryzen 5 3600 6-Core Processor",
		CPUCores:     6,
		CPUThreads:   12,
		CPUFreqMHz:   3600,
		GPUName:      "NVIDIA GeForce RTX 2060 SUPER",
		GPUVRAMGB:    8,
		IsIntegrated: false,
		SystemRAMGB:  16,
	}
	verdictTarget := EvaluateGameCompatibility(targetSys, cyberpunkReq)
	if verdictTarget.VerdictCode != "high" && verdictTarget.VerdictCode != "min" {
		t.Errorf("Expected high or min verdict for RTX 2060S on Cyberpunk, got %v (%s)", verdictTarget.VerdictCode, verdictTarget.Verdict)
	}

	// Case 3: Monster Rig: RTX 4090 + 7800X3D + 32GB RAM -> ULTRA (УЛЬТРА · 60+ FPS)
	ultraSys := SystemHardwareInput{
		CPUName:      "AMD Ryzen 7 7800X3D",
		CPUCores:     8,
		CPUThreads:   16,
		CPUFreqMHz:   4200,
		GPUName:      "NVIDIA GeForce RTX 4090",
		GPUVRAMGB:    24,
		IsIntegrated: false,
		SystemRAMGB:  32,
	}
	verdictUltra := EvaluateGameCompatibility(ultraSys, cyberpunkReq)
	if verdictUltra.VerdictCode != "ultra" || verdictUltra.Verdict != "УЛЬТРА · 60+ FPS" {
		t.Errorf("Expected ultra verdict for RTX 4090, got %v (%s)", verdictUltra.VerdictCode, verdictUltra.Verdict)
	}
}
