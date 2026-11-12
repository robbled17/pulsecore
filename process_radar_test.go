package main

import (
	"testing"
	"time"
)

func TestObserveProcessesRaisesCPUSpikeAfterSustain(t *testing.T) {
	a := &App{}
	now := time.Now()
	// Build baseline under load scenario (avoids orphan_load heuristic).
	for i := 0; i < radarMinSamples; i++ {
		a.ObserveProcesses([]ProcessSnap{{
			Name: "chrome.exe", PID: 10, CPUPercent: 5, RAMBytes: 200 << 20,
		}}, ScenarioLoad, now.Add(time.Duration(i)*time.Second))
	}
	// Sustained spike - must hold for radarSustainTicks
	for i := 0; i < radarSustainTicks; i++ {
		a.ObserveProcesses([]ProcessSnap{{
			Name: "chrome.exe", PID: 10, CPUPercent: 40, RAMBytes: 200 << 20,
		}}, ScenarioLoad, now.Add(time.Duration(radarMinSamples+i)*time.Second))
	}
	list := a.GetProcessAnomalies()
	if len(list) == 0 {
		t.Fatal("expected cpu_spike anomaly after sustained deviation")
	}
	found := false
	for _, an := range list {
		if an.Type == "cpu_spike" {
			found = true
			if an.TextRu == "" || an.TextEn == "" {
				t.Fatal("anomaly texts must be set")
			}
		}
	}
	if !found {
		t.Fatalf("want cpu_spike, got %+v", list)
	}
}

func TestObserveProcessesIgnoresSingleSpike(t *testing.T) {
	a := &App{}
	now := time.Now()
	for i := 0; i < radarMinSamples; i++ {
		a.ObserveProcesses([]ProcessSnap{{
			Name: "app.exe", PID: 1, CPUPercent: 4, RAMBytes: 100 << 20,
		}}, ScenarioIdle, now.Add(time.Duration(i)*time.Second))
	}
	// One-off spike only
	a.ObserveProcesses([]ProcessSnap{{
		Name: "app.exe", PID: 1, CPUPercent: 50, RAMBytes: 100 << 20,
	}}, ScenarioIdle, now.Add(20*time.Second))
	if n := len(a.GetProcessAnomalies()); n != 0 {
		t.Fatalf("single spike must not raise anomaly, got %d", n)
	}
}

func TestCurrentScenario(t *testing.T) {
	if currentScenario(true, 10) != ScenarioGame {
		t.Fatal("game wins")
	}
	if currentScenario(false, 50) != ScenarioLoad {
		t.Fatal("load")
	}
	if currentScenario(false, 10) != ScenarioIdle {
		t.Fatal("idle")
	}
}

func TestMergeProcessSnaps(t *testing.T) {
	out := mergeProcessSnaps(
		[]ProcessSnap{{Name: "a.exe", CPUPercent: 10, RAMBytes: 1}},
		[]ProcessSnap{{Name: "a.exe", DiskMBps: 5, RAMBytes: 2}, {Name: "b.exe", CPUPercent: 3}},
	)
	if len(out) != 2 {
		t.Fatalf("len=%d", len(out))
	}
	var aSnap ProcessSnap
	for _, p := range out {
		if p.Name == "a.exe" {
			aSnap = p
		}
	}
	if aSnap.CPUPercent != 10 || aSnap.DiskMBps != 5 || aSnap.RAMBytes != 2 {
		t.Fatalf("merged a.exe incorrectly: %+v", aSnap)
	}
}
