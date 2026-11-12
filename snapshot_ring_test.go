package main

import (
	"testing"
	"time"
)

func TestSnapshotRingCapacityAndOrder(t *testing.T) {
	var r SnapshotRing
	for i := 0; i < 150; i++ {
		r.Push(Snapshot{Timestamp: time.Unix(int64(i), 0), CPUPercent: float64(i)})
	}
	if r.Len() != snapshotRingCap {
		t.Fatalf("len=%d want %d", r.Len(), snapshotRingCap)
	}
	latest, ok := r.Latest()
	if !ok || latest.CPUPercent != 149 {
		t.Fatalf("latest=%v ok=%v", latest.CPUPercent, ok)
	}
	last := r.LastN(10)
	if len(last) != 10 {
		t.Fatalf("lastN=%d", len(last))
	}
	if last[0].CPUPercent != 140 || last[9].CPUPercent != 149 {
		t.Fatalf("order broken: first=%.0f last=%.0f", last[0].CPUPercent, last[9].CPUPercent)
	}
}

func TestSnapshotRingSince(t *testing.T) {
	var r SnapshotRing
	base := time.Unix(1000, 0)
	for i := 0; i < 20; i++ {
		r.Push(Snapshot{Timestamp: base.Add(time.Duration(i) * time.Second), CPUPercent: float64(i)})
	}
	got := r.Since(base.Add(15 * time.Second))
	if len(got) != 5 {
		t.Fatalf("since len=%d want 5", len(got))
	}
}
