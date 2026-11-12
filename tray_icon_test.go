package main

import "testing"

func TestTrayIconICOSize(t *testing.T) {
	ico := trayIconICO(45, 34, 197, 94)
	if len(ico) < 100 {
		t.Fatalf("ico too small: %d", len(ico))
	}
	// ICONDIR type = 1
	if ico[2] != 1 || ico[3] != 0 {
		t.Fatalf("bad ico type")
	}
}

func TestTrayColorForTemp(t *testing.T) {
	v, r, g, b, temp := trayColorForTemp(45, 10)
	if !temp || v != 45 || r != 34 {
		t.Fatalf("green temp failed: %v %v %v %v %v", v, r, g, b, temp)
	}
	v, r, _, _, temp = trayColorForTemp(90, 10)
	if !temp || v != 90 || r != 239 {
		t.Fatalf("red temp failed")
	}
	v, _, _, _, temp = trayColorForTemp(0, 33)
	if temp || v != 33 {
		t.Fatalf("cpu fallback failed: %v %v", v, temp)
	}
}

func TestFormatTrayTooltip(t *testing.T) {
	m := SystemMetrics{TemperatureC: 45, CPUPercent: 13, GPUPercent: 4, RAMPercent: 77}
	tip := formatTrayTooltip(m, true)
	if tip != "CPU 45°C · 13% | GPU 4% | RAM 77%" {
		t.Fatalf("tip=%q", tip)
	}
}
