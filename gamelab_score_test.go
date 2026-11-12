package main

import "testing"

func TestVegaCyberpunkLow(t *testing.T) {
	sys := SystemPowerProfile{
		GPUPowerScore: 18, CPUPowerScore: 42, CPUCores: 4,
		IsIntegrated: true, IsDualChannel: true, RAMGB: 8, VRAMGB: 0,
	}
	req := gameRequirements["cyberpunk"].Low
	avg, _, _ := estimatePreset(sys, req, presetLow, 0)
	if avg < 14 || avg > 24 {
		t.Fatalf("Vega dual-channel Cyberpunk Low got %d, want ~15-22", avg)
	}
}

func TestRTX4070CyberpunkHigh(t *testing.T) {
	sys := SystemPowerProfile{
		GPUPowerScore: 85, CPUPowerScore: 78, CPUCores: 8,
		IsIntegrated: false, IsDualChannel: true, RAMGB: 32, VRAMGB: 12,
	}
	req := gameRequirements["cyberpunk"].High
	avg, _, _ := estimatePreset(sys, req, presetHigh, 0)
	if avg < 60 || avg > 95 {
		t.Fatalf("RTX 4070 Cyberpunk High got %d, want ~60-80+", avg)
	}
}

func TestRTX2060SuperCyberpunkMed(t *testing.T) {
	sys := SystemPowerProfile{
		GPUPowerScore: 55, CPUPowerScore: 70, CPUCores: 6,
		IsIntegrated: false, IsDualChannel: true, RAMGB: 16, VRAMGB: 8,
	}
	req := gameRequirements["cyberpunk"].Med
	avg, _, _ := estimatePreset(sys, req, presetMed, 0)
	if avg < 35 || avg > 70 {
		t.Fatalf("2060 Super Cyberpunk Med got %d, want mid-tier", avg)
	}
}

func TestRTX2060SuperScoreAndGTAV(t *testing.T) {
	score := calcGPUPowerScore("NVIDIA GeForce RTX 2060 SUPER", false, 8)
	if score < 70 || score > 75 {
		t.Fatalf("2060 Super GPUPowerScore = %v, want 70-75", score)
	}
	sys := SystemPowerProfile{
		GPUName: "NVIDIA GeForce RTX 2060 SUPER", GPUPowerScore: score, CPUPowerScore: 70,
		CPUCores: 6, IsIntegrated: false, IsDualChannel: true, RAMGB: 16, VRAMGB: 8,
	}
	req := requirementsFor(GameInfo{ID: "gta5", Name: "Grand Theft Auto V", SteamAppID: 271590, Category: "aaa"})
	high, highRange, highWarn := estimatePreset(sys, req.High, presetHigh, 0)
	ultra, ultraRange, ultraWarn := estimatePreset(sys, req.Ultra, presetUltra, 0)
	if highWarn != "" || ultraWarn != "" {
		t.Fatalf("unexpected VRAM warning high=%q ultra=%q", highWarn, ultraWarn)
	}
	if high < 95 || high > 130 {
		t.Fatalf("GTA V High avg %d (%s), want 95-130", high, highRange)
	}
	if ultra < 95 || ultra > 130 {
		t.Fatalf("GTA V Ultra avg %d (%s), want 95-130", ultra, ultraRange)
	}
}

func TestAdapterRAM32BitCap(t *testing.T) {
	// 8 GiB truncated to 32 bits is 0; a signed 4 GiB-1 reading must not become empty.
	if adapterRAMBytes(0) != 0 {
		t.Fatal("zero")
	}
	if got := adapterRAMBytes(-1); got != 0xFFFFFFFF {
		t.Fatalf("signed cap = %d", got)
	}
	if !vramBytesLookCapped(4*1024*1024*1024 - 1) {
		t.Fatal("4GB-1 should look capped")
	}
	if got := knownDiscreteVRAMBytes("NVIDIA GeForce RTX 2060 SUPER"); got != 8*1024*1024*1024 {
		t.Fatalf("known vram %d", got)
	}
}

func TestIntegratedSingleChannelPenalty(t *testing.T) {
	dual := SystemPowerProfile{
		GPUPowerScore: 18, CPUPowerScore: 42, CPUCores: 4,
		IsIntegrated: true, IsDualChannel: true, RAMGB: 8,
	}
	single := dual
	single.IsDualChannel = false
	req := gameRequirements["cyberpunk"].Low
	a, _, _ := estimatePreset(dual, req, presetLow, 0)
	b, _, _ := estimatePreset(single, req, presetLow, 0)
	if b >= a {
		t.Fatalf("single-channel should be slower: dual=%d single=%d", a, b)
	}
}
