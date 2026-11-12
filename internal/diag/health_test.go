package diag

import "testing"

func TestComputeHealthScoreCritical(t *testing.T) {
	r := ComputeHealthScore(HealthInput{
		CPUPercent: 98, RAMPercent: 95, TempC: 95,
		DiskPercentMax: 96, DiskUnhealthy: true, LagCountRecent: 6,
	})
	if r.Score >= 40 || r.Level != "critical" {
		t.Fatalf("%+v", r)
	}
}

func TestComputeHealthScoreGood(t *testing.T) {
	r := ComputeHealthScore(HealthInput{CPUPercent: 20, RAMPercent: 40, TempC: 55, DiskPercentMax: 50})
	if r.Score != 100 || r.Level != "good" {
		t.Fatalf("%+v", r)
	}
}

func TestLeakWatchSuspect(t *testing.T) {
	w := NewLeakWatch()
	for i := 0; i < 10; i++ {
		w.Observe(42, "leak.exe", float64(100+i*20))
	}
	s := w.Suspects(5, 50)
	if len(s) != 1 || s[0].PID != 42 {
		t.Fatalf("%+v", s)
	}
}

func TestIntelligenceRules(t *testing.T) {
	ins := EvaluateIntelligence(IntelInput{RAMAvg7d: 90, LagCount7d: 12, HealthScore: 50})
	if len(ins) < 2 {
		t.Fatalf("%+v", ins)
	}
}
