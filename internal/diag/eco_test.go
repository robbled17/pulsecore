package diag

import "testing"

func TestComputeEcoDeltaImproved(t *testing.T) {
	before := EcoSample{CPUPercent: 40, ProcessCPU: 18, AtUnixMs: 1}
	after := EcoSample{CPUPercent: 32, ProcessCPU: 4, AtUnixMs: 2}
	d := ComputeEcoDelta(1, "chrome.exe", true, before, after)
	if !d.Improved || d.SummaryKey != "ecoDeltaImproved" {
		t.Fatalf("%+v", d)
	}
}

func TestComputeEcoDeltaPending(t *testing.T) {
	d := ComputeEcoDelta(1, "x", true, EcoSample{AtUnixMs: 1}, EcoSample{})
	if d.SummaryKey != "ecoDeltaPending" {
		t.Fatalf("%+v", d)
	}
}
