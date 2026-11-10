package main

import "testing"

func TestClassicNeighboursNotAAA(t *testing.T) {
	g := synthesizeGame("Как достать соседа")
	if g.Category != "classic" {
		t.Fatalf("expected classic, got %s / %s", g.Category, g.Genre)
	}
	req := requirementsFor(g)
	if req.Heavy || req.Low.ReqGPUScore > 10 {
		t.Fatalf("classic should be low-spec: %+v", req.Low)
	}
	sys := SystemPowerProfile{
		GPUPowerScore: 18, CPUPowerScore: 42, CPUCores: 4,
		IsIntegrated: true, IsDualChannel: true, RAMGB: 8,
	}
	avg, _, _ := estimatePreset(sys, req.Low, presetLow, req.CapFPS)
	if avg < 60 {
		t.Fatalf("classic low FPS got %d, want 60+", avg)
	}
}

func TestHalfLifeClassic(t *testing.T) {
	g := synthesizeGame("Half-Life")
	if g.Category != "classic" {
		t.Fatalf("expected classic, got %s", g.Category)
	}
}

func TestFlatOutClassic(t *testing.T) {
	g := synthesizeGame("FlatOut")
	if g.Category != "classic" {
		t.Fatalf("expected classic, got %s", g.Category)
	}
}
