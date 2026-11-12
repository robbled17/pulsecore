package diag

import "sync"

// SnapshotRing is a fixed-capacity circular buffer of Snapshots.
type SnapshotRing struct {
	mu   sync.RWMutex
	buf  []Snapshot
	cap  int
	head int // next write index
	size int
}

// NewSnapshotRing creates a ring with the given capacity (min 1).
func NewSnapshotRing(capacity int) *SnapshotRing {
	if capacity < 1 {
		capacity = 1
	}
	return &SnapshotRing{
		buf: make([]Snapshot, capacity),
		cap: capacity,
	}
}

// Push appends a snapshot, evicting the oldest when full.
func (r *SnapshotRing) Push(s Snapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.head] = s
	r.head = (r.head + 1) % r.cap
	if r.size < r.cap {
		r.size++
	}
}

// Len returns the number of stored snapshots.
func (r *SnapshotRing) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.size
}

// Cap returns the ring capacity.
func (r *SnapshotRing) Cap() int {
	return r.cap
}

// Latest returns the most recent snapshot and ok=false if empty.
func (r *SnapshotRing) Latest() (Snapshot, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.size == 0 {
		return Snapshot{}, false
	}
	idx := r.head - 1
	if idx < 0 {
		idx = r.cap - 1
	}
	return r.buf[idx], true
}

// Slice returns snapshots in chronological order (oldest → newest).
func (r *SnapshotRing) Slice() []Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Snapshot, 0, r.size)
	if r.size == 0 {
		return out
	}
	start := r.head - r.size
	if start < 0 {
		start += r.cap
	}
	for i := 0; i < r.size; i++ {
		out = append(out, r.buf[(start+i)%r.cap])
	}
	return out
}

// WindowAround returns snapshots within ±window of center (inclusive by time).
// If the ring is empty, returns nil.
func (r *SnapshotRing) WindowAround(center Snapshot, beforeN, afterN int) (pre, post []Snapshot) {
	all := r.Slice()
	if len(all) == 0 {
		return nil, nil
	}
	// Find index of center by timestamp match (or nearest).
	idx := -1
	for i := range all {
		if all[i].At.Equal(center.At) {
			idx = i
			break
		}
	}
	if idx < 0 {
		idx = len(all) - 1
	}
	start := idx - beforeN
	if start < 0 {
		start = 0
	}
	end := idx + afterN
	if end >= len(all) {
		end = len(all) - 1
	}
	if start <= idx {
		pre = append([]Snapshot(nil), all[start:idx+1]...)
	}
	if idx < end {
		post = append([]Snapshot(nil), all[idx+1:end+1]...)
	}
	return pre, post
}

// DiskTotals sums read/write MBps across disks for a live metrics adapter.
func DiskTotals(read, write float64) (float64, float64) {
	if read < 0 {
		read = 0
	}
	if write < 0 {
		write = 0
	}
	return read, write
}
