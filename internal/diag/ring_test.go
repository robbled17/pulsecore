package diag

import (
	"testing"
	"time"
)

func TestSnapshotRingEviction(t *testing.T) {
	r := NewSnapshotRing(3)
	for i := 0; i < 5; i++ {
		r.Push(Snapshot{At: time.Unix(int64(i), 0), CPUPercent: float64(i)})
	}
	if r.Len() != 3 {
		t.Fatalf("len=%d want 3", r.Len())
	}
	all := r.Slice()
	if len(all) != 3 {
		t.Fatalf("slice len=%d", len(all))
	}
	if all[0].CPUPercent != 2 || all[2].CPUPercent != 4 {
		t.Fatalf("unexpected order: %+v", all)
	}
	latest, ok := r.Latest()
	if !ok || latest.CPUPercent != 4 {
		t.Fatalf("latest=%v ok=%v", latest, ok)
	}
}

func TestWindowAround(t *testing.T) {
	r := NewSnapshotRing(10)
	var center Snapshot
	for i := 0; i < 10; i++ {
		s := Snapshot{At: time.Unix(int64(i), 0), CPUPercent: float64(i * 10)}
		r.Push(s)
		if i == 7 {
			center = s
		}
	}
	pre, post := r.WindowAround(center, 2, 2)
	if len(pre) != 3 { // includes center
		t.Fatalf("pre=%d want 3", len(pre))
	}
	if len(post) != 2 {
		t.Fatalf("post=%d want 2", len(post))
	}
	if pre[len(pre)-1].CPUPercent != 70 {
		t.Fatalf("center missing from pre")
	}
}
