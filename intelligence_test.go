package main

import (
	"testing"
	"time"
)

func TestClassifyIntelScenario(t *testing.T) {
	if classifyIntelScenario(true, 10) != IntelGame {
		t.Fatal("game")
	}
	if classifyIntelScenario(false, 80) != IntelHigh {
		t.Fatal("high")
	}
	if classifyIntelScenario(false, 40) != IntelWork {
		t.Fatal("work")
	}
	if classifyIntelScenario(false, 5) != IntelIdle {
		t.Fatal("idle")
	}
}

func TestIntelligenceNeedsMinSamples(t *testing.T) {
	a := &App{}
	e := a.ensureIntel()
	e.mu.Lock()
	e.loaded = true // skip disk so prior user profile cannot pollute the test
	e.baselines = map[string]*IntelBaseline{}
	e.insights = nil
	e.mu.Unlock()
	now := time.Now()
	for i := 0; i < intelMinSamples-1; i++ {
		a.recordIntelligenceTick(now.Add(time.Duration(i)*time.Minute), 20, 50, 10, 55, true, true, false)
	}
	st := a.GetIntelligenceStatus()
	var idle IntelBaseline
	for _, b := range st.Baselines {
		if b.Scenario == "idle" || b.Scenario == "work" {
			if b.Samples > idle.Samples {
				idle = b
			}
		}
	}
	if idle.Ready {
		t.Fatalf("must not be ready before min samples: %+v", idle)
	}
	a.recordIntelligenceTick(now.Add(time.Duration(intelMinSamples)*time.Minute), 20, 50, 10, 55, true, true, false)
	st = a.GetIntelligenceStatus()
	ready := false
	for _, b := range st.Baselines {
		if b.Ready {
			ready = true
		}
	}
	if !ready {
		t.Fatal("should be ready after min samples")
	}
}

func TestIntelligenceInsightOnRAMJump(t *testing.T) {
	a := &App{}
	e := a.ensureIntel()
	e.mu.Lock()
	e.loaded = true
	e.baselines = map[string]*IntelBaseline{}
	e.insights = nil
	e.mu.Unlock()
	now := time.Now()
	for i := 0; i < intelMinSamples; i++ {
		a.recordIntelligenceTick(now.Add(time.Duration(i)*time.Minute), 15, 50, 5, 50, true, false, false)
	}
	// Large sustained-looking RAM jump vs ~50% baseline
	a.recordIntelligenceTick(now.Add(time.Duration(intelMinSamples+1)*time.Minute), 15, 85, 5, 50, true, false, false)
	st := a.GetIntelligenceStatus()
	if len(st.Insights) == 0 {
		t.Fatal("expected RAM insight")
	}
	if st.MinSamples != intelMinSamples {
		t.Fatal("min samples must be documented in status")
	}
}

func TestResetIntelligenceProfile(t *testing.T) {
	a := &App{}
	e := a.ensureIntel()
	e.mu.Lock()
	e.loaded = true
	e.baselines = map[string]*IntelBaseline{}
	e.mu.Unlock()
	now := time.Now()
	for i := 0; i < intelMinSamples; i++ {
		a.recordIntelligenceTick(now.Add(time.Duration(i)*time.Minute), 10, 40, 0, 45, true, false, false)
	}
	a.ResetIntelligenceProfile()
	st := a.GetIntelligenceStatus()
	for _, b := range st.Baselines {
		if b.Samples != 0 {
			t.Fatalf("expected reset, got %+v", b)
		}
	}
}
