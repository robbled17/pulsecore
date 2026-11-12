package main

import (
	"testing"
	"time"
)

func TestClassifyMemoryTrackGrowth(t *testing.T) {
	now := time.Now()
	tr := &memTrack{Name: "firefox.exe"}
	start := now.Add(-30 * time.Minute)
	for i := 0; i < 10; i++ {
		mb := 400 + float64(i)*80 // 400 → 1120 over ~30 min
		tr.Points = append(tr.Points, memPoint{
			At:  start.Add(time.Duration(i) * 3 * time.Minute),
			MB:  mb,
			PID: 42,
		})
	}
	card := classifyMemoryTrack(tr, now)
	if card.Verdict != MemWatchGrowth && card.Verdict != MemWatchPossible {
		t.Fatalf("verdict=%s want growth/possible", card.Verdict)
	}
	if card.FromMB <= 0 || card.ToMB <= card.FromMB {
		t.Fatalf("from/to bad: %.1f → %.1f", card.FromMB, card.ToMB)
	}
	if card.TextRu == "" {
		t.Fatal("missing text")
	}
}

func TestClassifyMemoryTrackNeedData(t *testing.T) {
	tr := &memTrack{Name: "x.exe", Points: []memPoint{{At: time.Now(), MB: 100}}}
	card := classifyMemoryTrack(tr, time.Now())
	if card.Verdict != MemWatchNeedData {
		t.Fatalf("got %s", card.Verdict)
	}
}

func TestClassifyMemoryTrackNormalDrop(t *testing.T) {
	now := time.Now()
	tr := &memTrack{Name: "editor.exe"}
	start := now.Add(-25 * time.Minute)
	vals := []float64{200, 400, 600, 500, 300, 250, 220, 210}
	for i, mb := range vals {
		tr.Points = append(tr.Points, memPoint{
			At: start.Add(time.Duration(i) * 3 * time.Minute),
			MB: mb,
		})
	}
	card := classifyMemoryTrack(tr, now)
	if card.Verdict != MemWatchNormal && card.Verdict != MemWatchNeedData {
		// last <= first*1.35 or saw drops → normal
		if card.Verdict == MemWatchGrowth || card.Verdict == MemWatchPossible {
			t.Fatalf("volatile series should not be leak: %s", card.Verdict)
		}
	}
}

func TestObserveMemoryGrowthUpserts(t *testing.T) {
	a := &App{}
	now := time.Now()
	start := now.Add(-35 * time.Minute)
	for i := 0; i < 12; i++ {
		mb := uint64((500 + i*100) * 1024 * 1024)
		a.ObserveMemoryGrowth([]ProcessSnap{{
			Name: "chrome.exe", PID: 7, RAMBytes: mb,
		}}, start.Add(time.Duration(i)*3*time.Minute))
	}
	cards := a.GetMemoryWatchCards()
	if len(cards) == 0 {
		t.Fatal("expected memory watch card")
	}
	if cards[0].Verdict != MemWatchGrowth && cards[0].Verdict != MemWatchPossible {
		t.Fatalf("verdict=%s", cards[0].Verdict)
	}
}
