package main

import (
	"testing"
	"time"
)

func sampleWin(cpu, ram, temp float64, tempOK bool) []healthSample {
	now := time.Now()
	out := make([]healthSample, 0, 5)
	for i := 0; i < 5; i++ {
		out = append(out, healthSample{
			At: now.Add(time.Duration(i) * time.Minute),
			CPUPct: cpu, RAMPct: ram, SwapPct: 5,
			TempC: temp, TempOK: tempOK, GPUOK: false, DiskOK: true, DiskBusy: 10,
			SystemDiskUsed: 50, SystemDiskOK: true,
		})
	}
	return out
}

func TestComputeSystemHealthGood(t *testing.T) {
	rep := computeSystemHealth(sampleWin(25, 45, 55, true), 0, 0, 0, nil)
	if rep.Score < 85 {
		t.Fatalf("score=%d want >=85, factors=%+v", rep.Score, rep.Factors)
	}
	if len(rep.Action) != 0 {
		t.Fatalf("unexpected action: %v", rep.Action)
	}
	if rep.SummaryRu == "" {
		t.Fatal("missing summary")
	}
}

func TestComputeSystemHealthHotCPU(t *testing.T) {
	rep := computeSystemHealth(sampleWin(40, 50, 96, true), 0, 0, 0, nil)
	found := false
	for _, f := range rep.Factors {
		if f.Key == "cpu_temp" && f.Status == "action" {
			found = true
			if f.Score > 45 {
				t.Fatalf("overheat factor score too high: %.0f", f.Score)
			}
		}
	}
	if !found {
		t.Fatalf("cpu_temp should be action: %+v", rep.Factors)
	}
	// Overall may stay moderate if other factors are fine - deductions must mention temp.
	hasDed := false
	for _, d := range rep.Deductions {
		if d.Key == "cpu_temp" {
			hasDed = true
		}
	}
	if !hasDed {
		t.Fatal("expected cpu_temp deduction")
	}
}

func TestComputeSystemHealthMissingTempExcluded(t *testing.T) {
	with := computeSystemHealth(sampleWin(30, 40, 55, true), 0, 0, 0, nil)
	without := computeSystemHealth(sampleWin(30, 40, 0, false), 0, 0, 0, nil)
	if !without.Incomplete {
		t.Fatal("missing temp should mark incomplete")
	}
	var tempF HealthFactor
	for _, f := range without.Factors {
		if f.Key == "cpu_temp" {
			tempF = f
		}
	}
	if tempF.Available {
		t.Fatal("temp factor must be unavailable")
	}
	if without.Score < 50 {
		t.Fatalf("excluding temp should not tank score, got %d (with=%d)", without.Score, with.Score)
	}
}

func TestComputeSystemHealthFullSystemDisk(t *testing.T) {
	win := sampleWin(20, 40, 50, true)
	for i := range win {
		win[i].SystemDiskUsed = 92
		win[i].SystemDiskOK = true
	}
	rep := computeSystemHealth(win, 0, 0, 0, nil)
	found := false
	for _, f := range rep.Factors {
		if f.Key == "system_disk" {
			found = true
			if f.Score >= 90 {
				t.Fatalf("92%% full disk must not score near 100, got %.0f", f.Score)
			}
			if f.Status != "action" && f.Status != "watch" {
				t.Fatalf("status=%s", f.Status)
			}
		}
	}
	if !found {
		t.Fatal("system_disk factor missing")
	}
	if len(rep.Deductions) == 0 {
		t.Fatal("expected deductions for full disk")
	}
}

func TestSystemDiskUsedPctPrefersC(t *testing.T) {
	used, ok := systemDiskUsedPct([]DiskMetrics{
		{Mount: "D:\\", Percent: 10, TotalGB: 500},
		{Mount: "C:\\", Percent: 92, TotalGB: 476},
	})
	if !ok || used != 92 {
		t.Fatalf("used=%v ok=%v", used, ok)
	}
}

func TestHealthGrade(t *testing.T) {
	ru, en := healthGrade(92)
	if ru == "" || en == "" {
		t.Fatal("empty grade")
	}
}

func TestApplyDiskSMARTHealthFactorNormalWear(t *testing.T) {
	base := computeSystemHealth(sampleWin(10, 20, 45, true), 0, 0, 0, nil)
	smart := DiskHealthReport{
		OK:              true,
		DriveLetter:     "C:",
		StatusLevel:     DiskStatusGood,
		HealthPercentOK: true,
		HealthPercent:   99.0,
		StatusWordRu:    "Хорошо",
		StatusWordEn:    "Good",
		HealthSourceRu:  "SATA SSD: атрибут износа ID 169 (0xA9)",
		HealthSourceEn:  "SATA SSD: wear attribute ID 169 (0xA9)",
	}
	rep := applyDiskSMARTHealthFactor(base, smart)
	for _, d := range rep.Deductions {
		if d.Key == "disk_smart" {
			t.Fatalf("expected 0 deductions for 99%% SSD health, got deduction: %+v", d)
		}
	}
}

func TestApplyDiskSMARTHealthFactorLowHealth(t *testing.T) {
	base := computeSystemHealth(sampleWin(10, 20, 45, true), 0, 0, 0, nil)
	smart := DiskHealthReport{
		OK:              true,
		DriveLetter:     "C:",
		StatusLevel:     DiskStatusGood,
		HealthPercentOK: true,
		HealthPercent:   72.0,
		StatusWordRu:    "Хорошо",
		StatusWordEn:    "Good",
	}
	rep := applyDiskSMARTHealthFactor(base, smart)
	found := false
	for _, d := range rep.Deductions {
		if d.Key == "disk_smart" {
			found = true
			if d.ActionID != "open_smart" {
				t.Fatalf("expected ActionID open_smart, got %s", d.ActionID)
			}
			if d.ActionLabelRu != "S.M.A.R.T." {
				t.Fatalf("expected ActionLabelRu S.M.A.R.T., got %s", d.ActionLabelRu)
			}
		}
	}
	if !found {
		t.Fatal("expected deduction for 72% health")
	}
}

func TestDeductionActionsSet(t *testing.T) {
	rep := computeSystemHealth(sampleWin(40, 50, 96, true), 0, 0, 0, nil)
	for _, d := range rep.Deductions {
		if d.ActionID == "" {
			t.Fatalf("deduction %s has empty ActionID", d.Key)
		}
		if d.ActionLabelRu == "" || d.ActionLabelEn == "" {
			t.Fatalf("deduction %s has missing action labels", d.Key)
		}
	}
}

func TestRunHomeActionDeductions(t *testing.T) {
	app := &App{}
	res := app.RunHomeAction("free_ram", "")
	if !res.OK {
		t.Fatalf("free_ram failed: %+v", res)
	}
	if res.FreedRAMMB <= 0 {
		t.Fatalf("expected FreedRAMMB > 0, got %f", res.FreedRAMMB)
	}

	resStartup := app.RunHomeAction("open_startup", "")
	if !resStartup.OK || resStartup.OpenTab != "lag" {
		t.Fatalf("open_startup want OpenTab lag, got %+v", resStartup)
	}

	resSmart := app.RunHomeAction("open_smart", "")
	if !resSmart.OK || resSmart.OpenTab != "disk_smart" {
		t.Fatalf("open_smart want OpenTab disk_smart, got %+v", resSmart)
	}
}
