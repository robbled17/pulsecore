package main

import (
	"testing"

	"github.com/robbled17/zeropulse/loc"
)

func TestSmartFixPlanGainsCascade(t *testing.T) {
	in := SmartFixInput{
		HomeCatalogInput: HomeCatalogInput{
			SystemDiskOK: true, SystemDiskUsed: 96,
			HealthScore: 80,
		},
	}
	steps := EvaluateSmartFixScenarios(in)
	if len(steps) == 0 {
		t.Fatal("expected disk_full step")
	}
	probs := make([]HomeProblem, len(steps))
	for i, s := range steps {
		probs[i] = HomeProblem{ID: s.ID, Priority: s.Priority, GainEstimate: s.GainEstimate}
	}
	probs = AssignCascadedGains(probs, 80)
	sum := 0
	for _, p := range probs {
		sum += p.GainEstimate
	}
	if sum > 20 {
		t.Fatalf("cascaded gains %d exceed room 20", sum)
	}
	if ScoreIfAllFixed(80, probs) > 100 {
		t.Fatal("score if applied > 100")
	}
}

func TestSmartFixCleanupNotReversible(t *testing.T) {
	steps := EvaluateSmartFixScenarios(SmartFixInput{
		HomeCatalogInput: HomeCatalogInput{SystemDiskOK: true, SystemDiskUsed: 90, HealthScore: 70},
	})
	found := false
	for _, s := range steps {
		if s.ScenarioID == "disk_full" {
			found = true
			if s.Reversible || !s.Irreversible {
				t.Fatalf("cleanup must be irreversible: %+v", s)
			}
			if s.ActionID != "cleanup_safe" {
				t.Fatalf("want cleanup_safe, got %s", s.ActionID)
			}
		}
	}
	if !found {
		t.Fatal("disk_full missing")
	}
}

func TestSmartFixUptimeScenario(t *testing.T) {
	steps := EvaluateSmartFixScenarios(SmartFixInput{
		HomeCatalogInput: HomeCatalogInput{HealthScore: 70},
		UptimeDays:       20,
	})
	ok := false
	for _, s := range steps {
		if s.ScenarioID == "long_uptime" {
			ok = true
			if s.ActionID != "open_settings" {
				t.Fatalf("uptime should open settings, got %s", s.ActionID)
			}
		}
	}
	if !ok {
		t.Fatal("expected long_uptime")
	}
}

func TestSmartFixOnlyFound(t *testing.T) {
	steps := EvaluateSmartFixScenarios(SmartFixInput{
		HomeCatalogInput: HomeCatalogInput{HealthScore: 90},
	})
	for _, s := range steps {
		if s.ScenarioID == "disk_full" || s.ScenarioID == "cpu_hot" {
			t.Fatalf("unexpected scenario %s", s.ScenarioID)
		}
	}
}

func TestSmartFixJournalSessionID(t *testing.T) {
	id := newSmartFixSessionID()
	if id == "" {
		t.Fatal("empty id")
	}
	_ = loc.M("smartFixResultOK", loc.P("before", 1, "after", 2, "gb", 1.5, "mb", 0))
}

func TestIsSafeCleanupPath(t *testing.T) {
	if isSafeCleanupPath(`C:\Users\me\Documents\secret.txt`) {
		t.Fatal("documents must be blocked")
	}
	if isSafeCleanupPath(`C:\Users\me\Downloads\file.zip`) {
		t.Fatal("downloads must be blocked")
	}
}
