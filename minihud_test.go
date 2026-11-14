package main

import "testing"

func TestUpdateMiniHUDConsistentDimensions(t *testing.T) {
	firstW, firstH := GetMiniHUDPhysicalDimensions(0)
	if firstW <= 0 || firstH <= 0 {
		t.Fatalf("expected positive dimensions, got %dx%d", firstW, firstH)
	}

	for i := 1; i <= 10; i++ {
		w, h := GetMiniHUDPhysicalDimensions(0)
		if w != firstW || h != firstH {
			t.Fatalf("iteration %d: dimensions changed to %dx%d, want %dx%d", i, w, h, firstW, firstH)
		}
	}

	fpsW, fpsH := GetFPSOverlayPhysicalDimensions(0)
	if fpsW <= 0 || fpsH <= 0 {
		t.Fatalf("expected positive FPS overlay dimensions, got %dx%d", fpsW, fpsH)
	}
	for i := 1; i <= 10; i++ {
		w, h := GetFPSOverlayPhysicalDimensions(0)
		if w != fpsW || h != fpsH {
			t.Fatalf("iteration %d: FPS overlay dimensions changed to %dx%d, want %dx%d", i, w, h, fpsW, fpsH)
		}
	}
}
