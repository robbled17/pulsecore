package main

import "testing"

func TestFinalizeDiskHealthNeverInventsWear(t *testing.T) {
	base := DiskHealthReport{
		DriveLetter:  "C:",
		Model:        "Mystery SSD",
		HealthStatus: "",
	}
	got := finalizeDiskHealthReport(base, -1, 0)
	if got.WearRemaining != -1 || got.HealthPercent != -1 {
		t.Fatalf("missing wear must stay -1, got wear=%v health=%v", got.WearRemaining, got.HealthPercent)
	}
	if got.Verdict != "Недоступно" && got.VerdictBadge != "N/A" {
		t.Fatalf("expected unavailable verdict, got %q / %q", got.Verdict, got.VerdictBadge)
	}
	if !got.OK {
		t.Fatal("model present: OK should stay true")
	}
}

func TestFinalizeDiskHealthHealthyWithoutWear(t *testing.T) {
	base := DiskHealthReport{
		DriveLetter:  "C:",
		Model:        "Samsung",
		HealthStatus: "Healthy",
	}
	got := finalizeDiskHealthReport(base, -1, 0)
	if got.WearRemaining != -1 || got.HealthPercent != -1 {
		t.Fatalf("healthy without wear must not invent percent, got wear=%v health=%v", got.WearRemaining, got.HealthPercent)
	}
	if got.Verdict != "OK" {
		t.Fatalf("expected OK verdict, got %q", got.Verdict)
	}
}

func TestFinalizeDiskHealthRealWear(t *testing.T) {
	base := DiskHealthReport{
		DriveLetter:  "C:",
		Model:        "Samsung",
		HealthStatus: "Healthy",
	}
	got := finalizeDiskHealthReport(base, 6, 1024*1024*1024*1024) // 6% wear, 1TB
	if got.WearRemaining != 94 || got.HealthPercent != 94 {
		t.Fatalf("want 94%% remaining, got wear=%v health=%v", got.WearRemaining, got.HealthPercent)
	}
	if got.TBWGB <= 0 {
		t.Fatal("expected TBW estimate from size*wear")
	}
}
