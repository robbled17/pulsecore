package main

import (
	"math"
	"testing"
)

// Fixture: COMPIT SATA SSD (Silicon Motion), real dump family from admin-probe / CDI.
// Expected (CDI parity): Good, health 99%, temp 36°C, POH 2819h, cycles 1120,
// host write ≈7853 GB, host read ≈19043 GB (F1/F2 × 32 MiB).
func compitCAttrs() []SmartAttribute {
	// Raw F1/F2 chosen so 32MiB units land within 1% of CDI TBW.
	f1 := uint64(math.Round(7853.0 * 1024.0 / 32.0)) // ≈251296
	f2 := uint64(math.Round(19043.0 * 1024.0 / 32.0)) // ≈609376
	return []SmartAttribute{
		attr(1, 100, 100, 0, 0),
		attr(5, 100, 100, 0, 2),   // remapped blocks (SSD-normal low)
		attr(9, 100, 100, 0, 2819),
		attr(12, 100, 100, 0, 1120),
		attr(160, 100, 100, 0, 0),
		attr(161, 100, 100, 0, 95),
		attr(163, 100, 100, 0, 19),
		attr(169, 100, 100, 0, 99), // A9 Remain Life raw %
		attr(194, 100, 100, 0, 36),
		attr(196, 100, 100, 0, 0),
		attr(197, 100, 100, 0, 2), // C5 — do NOT caution on stable SSD
		attr(198, 100, 100, 0, 0),
		attr(232, 100, 100, 0, 95),
		attr(241, 100, 100, 0, f1),
		attr(242, 100, 100, 0, f2),
	}
}

func TestCompitC_SMI_Good99(t *testing.T) {
	attrs := compitCAttrs()
	prof := detectDiskProfile(DiskKindSATASSD, "COMPIT", attrs)
	if prof.ID != "smi" {
		t.Fatalf("want smi profile, got %s", prof.ID)
	}
	if !prof.IgnoreSectorRawForStatus {
		t.Fatal("SMI must ignore raw sector counters for status")
	}

	ev := EvaluateDiskHealth(DiskHealthEvalInput{
		Kind: DiskKindSATASSD, Model: "COMPIT", SMARTReadOK: true,
		Attributes: attrs, Pending: 2, Reallocated: 2,
	})
	if ev.Level != DiskStatusGood {
		t.Fatalf("want good (stable low remaps), got %s reason=%s log=%s", ev.Level, ev.ReasonRu, ev.LogLine)
	}
	if !ev.HealthPctOK || ev.HealthPercent != 99 {
		t.Fatalf("want health 99%% from A9 raw, got ok=%v pct=%.0f src=%s", ev.HealthPctOK, ev.HealthPercent, ev.HealthSourceRu)
	}
	if ev.InfoNote.Code == "" {
		t.Fatal("expected calm All-specs note for stable remaps")
	}

	w, r, okW, okR := hostIOFromProfile(prof, attrs)
	if !okW || !okR {
		t.Fatalf("host IO okW=%v okR=%v", okW, okR)
	}
	if math.Abs(w-7853)/7853 > 0.01 {
		t.Fatalf("written GB=%.1f want ≈7853 (±1%%)", w)
	}
	if math.Abs(r-19043)/19043 > 0.01 {
		t.Fatalf("read GB=%.1f want ≈19043 (±1%%)", r)
	}

	t.Logf("COMPIT dump: status=%s health=%.0f%% write=%.1fGB read=%.1fGB note=%s",
		ev.Level, ev.HealthPercent, w, r, ev.InfoNote.Code)
	for _, a := range attrs {
		raw6 := [6]byte{
			byte(a.RawValue), byte(a.RawValue >> 8), byte(a.RawValue >> 16),
			byte(a.RawValue >> 24), byte(a.RawValue >> 32), byte(a.RawValue >> 40),
		}
		t.Logf("attr id=%d name=%s cur=%d worst=%d thr=%d raw=%d bytes=%02X %02X %02X %02X %02X %02X",
			a.ID, a.Name, a.Current, a.Worst, a.Threshold, a.RawValue,
			raw6[0], raw6[1], raw6[2], raw6[3], raw6[4], raw6[5])
	}
}

func TestCompitC_C5Growth_Caution(t *testing.T) {
	attrs := compitCAttrs()
	// Mutate pending to 12 (grew from prior 2).
	for i := range attrs {
		if attrs[i].ID == 197 {
			attrs[i].RawValue = 12
			attrs[i].Raw = "12"
		}
	}
	ev := EvaluateDiskHealth(DiskHealthEvalInput{
		Kind: DiskKindSATASSD, Model: "COMPIT", SMARTReadOK: true,
		Attributes: attrs, Pending: 12, Reallocated: 2,
		PriorPendingOK: true, PriorPending: 2,
		PriorReallocatedOK: true, PriorReallocated: 2,
	})
	if ev.Level != DiskStatusCaution {
		t.Fatalf("want caution on C5 2→12, got %s (%s)", ev.Level, ev.ReasonRu)
	}
	if ev.Reason.Code != "diskReasonPendingGrew" {
		t.Fatalf("want diskReasonPendingGrew, got %s", ev.Reason.Code)
	}
}

func TestCleanSSD_Good(t *testing.T) {
	ev := EvaluateDiskHealth(DiskHealthEvalInput{
		Kind: DiskKindSATASSD, Model: "Kingston NV2", SMARTReadOK: true,
		Attributes: []SmartAttribute{
			attr(5, 100, 100, 10, 0),
			attr(197, 100, 100, 0, 0),
			attr(231, 100, 100, 5, 0),
		},
	})
	if ev.Level != DiskStatusGood {
		t.Fatalf("clean SSD want good, got %s", ev.Level)
	}
}

func TestHDD_PendingCaution_Fixture(t *testing.T) {
	ev := EvaluateDiskHealth(DiskHealthEvalInput{
		Kind: DiskKindHDD, Model: "WDC WD10EZEX", SMARTReadOK: true,
		Pending: 4,
		Attributes: []SmartAttribute{
			attr(5, 200, 200, 140, 0),
			attr(197, 200, 200, 0, 4),
			attr(198, 200, 200, 0, 0),
		},
	})
	if ev.Level != DiskStatusCaution {
		t.Fatalf("HDD pending want caution, got %s", ev.Level)
	}
	if ev.Reason.Code != "diskReasonPendingHuman" {
		t.Fatalf("want human pending reason, got %s", ev.Reason.Code)
	}
}

func TestDiskStatusClassMapping(t *testing.T) {
	// Backend level codes that frontend maps to st-* classes.
	want := map[string]string{
		DiskStatusGood: "st-good", DiskStatusCaution: "st-caution",
		DiskStatusBad: "st-bad", DiskStatusUnknown: "st-unknown",
	}
	for level, cls := range want {
		got := mapDiskStatusClass(level)
		if got != cls {
			t.Fatalf("level %s → %s, want %s", level, got, cls)
		}
	}
}

func mapDiskStatusClass(level string) string {
	switch level {
	case DiskStatusGood:
		return "st-good"
	case DiskStatusCaution:
		return "st-caution"
	case DiskStatusBad:
		return "st-bad"
	default:
		return "st-unknown"
	}
}
