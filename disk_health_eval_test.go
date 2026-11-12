package main

import (
	"strings"
	"testing"
)

func attr(id, current, worst, thresh int, raw uint64) SmartAttribute {
	en, ru := smartAttrName(id)
	return SmartAttribute{
		ID: id, Name: en, NameRu: ru,
		Current: current, Worst: worst, Threshold: thresh,
		Raw: formatSMARTRaw([]byte{
			byte(raw), byte(raw >> 8), byte(raw >> 16),
			byte(raw >> 24), byte(raw >> 32), byte(raw >> 40),
		}),
		RawValue: raw,
	}
}

func TestEvalGoodSATASSD_CompitLike(t *testing.T) {
	// COMPIT-like: healthy SSD, wear 100, no pending - must NOT be Bad
	in := DiskHealthEvalInput{
		Kind: DiskKindSATASSD, Model: "COMPIT", SMARTReadOK: true,
		Attributes: []SmartAttribute{
			attr(5, 100, 100, 10, 0),
			attr(9, 100, 100, 0, 3000),
			attr(12, 100, 100, 0, 120),
			attr(197, 100, 100, 0, 0),
			attr(198, 100, 100, 0, 0),
			attr(231, 100, 100, 10, 0), // life left 100%
			attr(241, 100, 100, 0, 5000000),
			attr(242, 100, 100, 0, 4000000),
		},
	}
	ev := EvaluateDiskHealth(in)
	if ev.Level != DiskStatusGood {
		t.Fatalf("want good, got %s reason=%s log=%s", ev.Level, ev.ReasonRu, ev.LogLine)
	}
	if !ev.HealthPctOK || ev.HealthPercent != 100 {
		t.Fatalf("want health 100%%, got ok=%v pct=%.0f src=%s", ev.HealthPctOK, ev.HealthPercent, ev.HealthSourceRu)
	}
}

func TestEvalSATASSD_WearLowCaution(t *testing.T) {
	in := DiskHealthEvalInput{
		Kind: DiskKindSATASSD, Model: "Kingston A400", SMARTReadOK: true,
		Attributes: []SmartAttribute{
			attr(5, 100, 100, 10, 0),
			attr(197, 100, 100, 0, 0),
			attr(231, 8, 8, 10, 0), // 8% life → Caution (231 not in critical list)
		},
	}
	ev := EvaluateDiskHealth(in)
	if ev.Level != DiskStatusCaution {
		t.Fatalf("want caution for low wear, got %s (%s)", ev.Level, ev.ReasonRu)
	}
	if !ev.HealthPctOK || ev.HealthPercent != 8 {
		t.Fatalf("want 8%%, got %.0f", ev.HealthPercent)
	}
}

func TestEvalSATASSD_CriticalThresholdBad(t *testing.T) {
	in := DiskHealthEvalInput{
		Kind: DiskKindSATASSD, Model: "Samsung SSD 870", SMARTReadOK: true,
		Attributes: []SmartAttribute{
			attr(5, 5, 5, 10, 0), // current 5 ≤ threshold 10 → Bad
			attr(231, 100, 100, 10, 0),
		},
	}
	ev := EvaluateDiskHealth(in)
	if ev.Level != DiskStatusBad {
		t.Fatalf("want bad, got %s", ev.Level)
	}
	if ev.TriggerAttrID != 5 {
		t.Fatalf("want trigger 5, got %d", ev.TriggerAttrID)
	}
	if ev.ReasonRu == "" {
		t.Fatal("expected reason")
	}
}

func TestEvalHDD_PendingCaution(t *testing.T) {
	in := DiskHealthEvalInput{
		Kind: DiskKindHDD, Model: "WDC WD10EZEX-08M2NA0", SMARTReadOK: true,
		Pending: 12,
		Attributes: []SmartAttribute{
			attr(5, 100, 100, 36, 0),
			attr(197, 100, 100, 0, 12), // C5 pending = 12
			attr(198, 100, 100, 0, 0),
			attr(9, 80, 80, 0, 20000),
			attr(12, 100, 100, 0, 1500),
		},
	}
	ev := EvaluateDiskHealth(in)
	if ev.Level != DiskStatusCaution {
		t.Fatalf("want caution, got %s (%s)", ev.Level, ev.ReasonRu)
	}
	if !ev.HealthPctOK {
		t.Fatal("HDD should expose health % from SMART attrs")
	}
	// 100 − min(40, 12×10) = 100 − 40 = 60
	if ev.HealthPercent != 60 {
		t.Fatalf("want health 60%%, got %.0f src=%s", ev.HealthPercent, ev.HealthSourceRu)
	}
	if ev.Reason.Code != "diskReasonPendingHuman" && !strings.Contains(ev.ReasonRu, "ожидают") && !strings.Contains(ev.ReasonRu, "pending") {
		t.Fatalf("reason should be human pending: code=%s ru=%s", ev.Reason.Code, ev.ReasonRu)
	}
	if strings.Contains(ev.HealthSourceRu, "NVMe") {
		t.Fatalf("HDD source must not say NVMe: %s", ev.HealthSourceRu)
	}
}

func TestEvalHDD_Good(t *testing.T) {
	in := DiskHealthEvalInput{
		Kind: DiskKindHDD, Model: "WDC WD10EZEX", SMARTReadOK: true,
		Attributes: []SmartAttribute{
			attr(5, 200, 200, 140, 0),
			attr(197, 200, 200, 0, 0),
			attr(198, 200, 200, 0, 0),
		},
	}
	ev := EvaluateDiskHealth(in)
	if ev.Level != DiskStatusGood {
		t.Fatalf("want good, got %s", ev.Level)
	}
	if !ev.HealthPctOK || ev.HealthPercent != 100 {
		t.Fatalf("want 100%% health, got ok=%v pct=%.0f", ev.HealthPctOK, ev.HealthPercent)
	}
	if !strings.Contains(ev.HealthSourceRu, "SMART") {
		t.Fatalf("source: %s", ev.HealthSourceRu)
	}
}

func TestEvalHDD_ReallocatedCaution(t *testing.T) {
	in := DiskHealthEvalInput{
		Kind: DiskKindHDD, Model: "WDC", SMARTReadOK: true,
		Attributes: []SmartAttribute{
			attr(5, 200, 200, 140, 3),
			attr(197, 200, 200, 0, 0),
			attr(198, 200, 200, 0, 0),
		},
	}
	ev := EvaluateDiskHealth(in)
	if ev.Level != DiskStatusCaution {
		t.Fatalf("want caution, got %s (%s)", ev.Level, ev.ReasonRu)
	}
	// 100 − min(40, 3×5) = 85
	if !ev.HealthPctOK || ev.HealthPercent != 85 {
		t.Fatalf("want 85%%, got %.0f", ev.HealthPercent)
	}
}

func TestEvalNVMe_CriticalWarningBad(t *testing.T) {
	in := DiskHealthEvalInput{
		Kind: DiskKindNVMe, Model: "NVMe", SMARTReadOK: true,
		NVMeCriticalWarn: 0x01, NVMePercentUsedOK: true, NVMePercentUsed: 1,
		NVMeSpare: 100, NVMeSpareThresh: 10,
	}
	ev := EvaluateDiskHealth(in)
	if ev.Level != DiskStatusBad {
		t.Fatalf("want bad, got %s", ev.Level)
	}
	if !ev.HealthPctOK || ev.HealthPercent != 99 {
		t.Fatalf("want 99%%, got %.0f", ev.HealthPercent)
	}
}

func TestEvalNVMe_Good99(t *testing.T) {
	in := DiskHealthEvalInput{
		Kind: DiskKindNVMe, SMARTReadOK: true,
		NVMePercentUsedOK: true, NVMePercentUsed: 1,
		NVMeSpare: 100, NVMeSpareThresh: 10,
	}
	ev := EvaluateDiskHealth(in)
	if ev.Level != DiskStatusGood {
		t.Fatalf("want good, got %s reason=%s", ev.Level, ev.ReasonRu)
	}
	if ev.HealthPercent != 99 {
		t.Fatalf("want 99, got %.0f", ev.HealthPercent)
	}
}

func TestEvalUnknown_NoSMART(t *testing.T) {
	ev := EvaluateDiskHealth(DiskHealthEvalInput{Kind: DiskKindSATASSD, SMARTReadOK: false})
	if ev.Level != DiskStatusUnknown {
		t.Fatalf("want unknown, got %s", ev.Level)
	}
}

func TestFalseBad_ReadErrorsMustNotAffect(t *testing.T) {
	// Regression: Storage API ReadErrors must not be part of EvaluateDiskHealth input
	in := DiskHealthEvalInput{
		Kind: DiskKindSATASSD, Model: "COMPIT", SMARTReadOK: true,
		Attributes: []SmartAttribute{
			attr(5, 100, 100, 10, 0),
			attr(197, 100, 100, 0, 0),
			attr(231, 100, 100, 5, 0),
		},
	}
	ev := EvaluateDiskHealth(in)
	if ev.Level == DiskStatusBad {
		t.Fatalf("healthy SSD must not be Bad: %s", ev.ReasonRu)
	}
}
