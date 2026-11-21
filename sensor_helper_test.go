package main

import (
	"strings"
	"testing"
)

func TestEmbeddedSensorHelperPresent(t *testing.T) {
	ver := sensorHelperVersion()
	if ver == "" {
		t.Fatal("embedded VERSION empty")
	}
	if !strings.HasPrefix(ver, "1.") {
		t.Fatalf("unexpected helper version %q", ver)
	}
	if len(embeddedSensorHelperEXE) < 1024 {
		t.Fatalf("embedded helper too small: %d bytes", len(embeddedSensorHelperEXE))
	}
	// PE MZ header for Windows exe
	if embeddedSensorHelperEXE[0] != 'M' || embeddedSensorHelperEXE[1] != 'Z' {
		t.Fatal("embedded helper is not a PE executable")
	}
}

func TestSensorHelperCacheTTLPositive(t *testing.T) {
	if sensorHelperCacheTTL <= 0 {
		t.Fatal("cache TTL must be positive")
	}
}
