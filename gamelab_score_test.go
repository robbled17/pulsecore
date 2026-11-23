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
	avg, _, warn := estimatePreset(sys, req, presetMed, 0)
	if avg < 55 || avg > 95 {
		t.Fatalf("2060 Super Cyberpunk Med got %d, want ~60-85", avg)
	}
	if warn == "fpsWarnVRAM" {
		t.Fatalf("8GB must not report VRAM shortage for Cyberpunk Med")
	}
}

func TestRTX2060SuperRDR2Med(t *testing.T) {
	sys := SystemPowerProfile{
		GPUPowerScore: 55, CPUPowerScore: 65, CPUCores: 6,
		IsIntegrated: false, IsDualChannel: true, RAMGB: 16, VRAMGB: 8,
	}
	req := gameRequirements["rdr2"].Med
	avg, _, warn := estimatePreset(sys, req, presetMed, 0)
	if avg < 65 || avg > 90 {
		t.Fatalf("2060 Super RDR2 Med got %d, want ~65-80", avg)
	}
	if warn == "fpsWarnVRAM" {
		t.Fatalf("8GB must not report VRAM shortage for RDR2 Med")
	}
}

func TestUnknownVRAMNoFalseShortage(t *testing.T) {
	sys := SystemPowerProfile{
		GPUPowerScore: 55, CPUPowerScore: 65, CPUCores: 6,
		IsIntegrated: false, IsDualChannel: true, RAMGB: 16, VRAMGB: 0,
	}
	req := gameRequirements["rdr2"].Med
	_, _, warn := estimatePreset(sys, req, presetMed, 0)
	if warn == "fpsWarnVRAM" {
		t.Fatalf("unknown discrete VRAM must not invent a shortage warning")
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
