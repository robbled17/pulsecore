package main

import "testing"

func TestMinPositiveLatency(t *testing.T) {
	if got := minPositiveLatency(-1, -1); got != -1 {
		t.Fatalf("both fail: got %v", got)
	}
	if got := minPositiveLatency(-1, 12); got != 12 {
		t.Fatalf("a fail: got %v", got)
	}
	if got := minPositiveLatency(9, -1); got != 9 {
		t.Fatalf("b fail: got %v", got)
	}
	if got := minPositiveLatency(15, 8); got != 8 {
		t.Fatalf("min: got %v", got)
	}
}

func TestMeasureHostLatencyRejectsShellMeta(t *testing.T) {
	if got := measureHostLatencyMs("8.8.8.8 & calc", 1); got != -1 {
		t.Fatalf("metachar host must be rejected, got %v", got)
	}
	if got := measureHostLatencyMs("", 1); got != -1 {
		t.Fatalf("empty host must be rejected, got %v", got)
	}
}
