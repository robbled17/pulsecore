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
