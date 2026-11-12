package main

import (
	"sync"
	"time"
)

const snapshotRingCap = 120

// SnapshotRing is a preallocated ring of system snapshots (no disk persistence).
type SnapshotRing struct {
	mu   sync.Mutex
	buf  [snapshotRingCap]Snapshot
	n    int
	next int
}

func (r *SnapshotRing) Push(s Snapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.next] = s
	r.next = (r.next + 1) % snapshotRingCap
	if r.n < snapshotRingCap {
		r.n++
	}
}

func (r *SnapshotRing) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.n
}

func (r *SnapshotRing) Latest() (Snapshot, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.n == 0 {
		return Snapshot{}, false
	}
	idx := r.next - 1
	if idx < 0 {
		idx = snapshotRingCap - 1
	}
	return r.buf[idx], true
}

// LastN returns up to n most recent snapshots in chronological order.
func (r *SnapshotRing) LastN(n int) []Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n <= 0 || r.n == 0 {
		return nil
	}
	if n > r.n {
		n = r.n
	}
	out := make([]Snapshot, n)
	for i := 0; i < n; i++ {
		idx := r.next - n + i
		if idx < 0 {
			idx += snapshotRingCap
		}
		out[i] = cloneSnapshot(r.buf[idx])
	}
	return out
}

// Since returns snapshots with Timestamp >= t (chronological).
func (r *SnapshotRing) Since(t time.Time) []Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.n == 0 {
		return nil
	}
	out := make([]Snapshot, 0, r.n)
	start := r.next - r.n
	if start < 0 {
		start += snapshotRingCap
	}
	for i := 0; i < r.n; i++ {
		idx := (start + i) % snapshotRingCap
		if !r.buf[idx].Timestamp.Before(t) {
			out = append(out, cloneSnapshot(r.buf[idx]))
		}
	}
	return out
}

func cloneSnapshot(s Snapshot) Snapshot {
	out := s
	if s.CPUPerCore != nil {
		out.CPUPerCore = append([]float64(nil), s.CPUPerCore...)
	}
	out.TopCPU = cloneProcs(s.TopCPU)
	out.TopDisk = cloneProcs(s.TopDisk)
	out.TopRAM = cloneProcs(s.TopRAM)
	return out
}

func cloneProcs(in []ProcessSnap) []ProcessSnap {
	if len(in) == 0 {
		return nil
	}
	out := make([]ProcessSnap, len(in))
	copy(out, in)
	for i := range out {
		if in[i].PIDs != nil {
			out[i].PIDs = append([]int32(nil), in[i].PIDs...)
		}
	}
	return out
}
