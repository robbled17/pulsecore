package diag

import (
	"fmt"
	"sync"
	"time"
)

// BlackBox owns the snapshot ring and lag incident journal.
type BlackBox struct {
	mu        sync.RWMutex
	ring      *SnapshotRing
	incidents []Incident
	seq       uint64
}

// NewBlackBox constructs a Black Box with the standard 120-slot ring.
func NewBlackBox() *BlackBox {
	return &BlackBox{
		ring:      NewSnapshotRing(RingCapacity),
		incidents: make([]Incident, 0, MaxIncidents),
	}
}

// PushSnapshot records a live tick into the ring.
func (b *BlackBox) PushSnapshot(s Snapshot) {
	b.ring.Push(s)
}

// RingLen returns how many snapshots are buffered.
func (b *BlackBox) RingLen() int {
	return b.ring.Len()
}

// Snapshots returns a chronological copy of the ring.
func (b *BlackBox) Snapshots() []Snapshot {
	return b.ring.Slice()
}

// RecordHitch evaluates the current context and stores an incident.
func (b *BlackBox) RecordHitch(jitterMs float64, source string) Incident {
	latest, ok := b.ring.Latest()
	if !ok {
		latest = Snapshot{At: time.Now()}
	}
	pre, post := b.ring.WindowAround(latest, 5, 3)
	verdict := Evaluate(latest, pre, jitterMs)

	inc := Incident{
		Timestamp:     time.Now(),
		TimestampUI:   time.Now().Format("15:04:05"),
		JitterMs:      jitterMs,
		Severity:      string(verdict.Level),
		Verdict:       verdict,
		Culprit:       verdict.Culprit,
		DiskBusy:      verdict.Cause == CauseDiskBusy,
		FreeRAMMB:     latest.RAMFreeMB,
		DiskReadMBps:  latest.DiskReadMBps,
		DiskWriteMBps: latest.DiskWriteMBps,
		PreWindow:     toTimeline(pre, latest.At),
		PostWindow:    toTimeline(post, latest.At),
		Source:        source,
	}
	if inc.Severity == string(LevelUnknown) {
		inc.Severity = "unknown"
	} else if inc.Severity == string(LevelOK) {
		inc.Severity = "info"
	}

	b.mu.Lock()
	b.seq++
	inc.ID = fmt.Sprintf("lag-%d-%d", time.Now().Unix(), b.seq)
	b.incidents = append([]Incident{inc}, b.incidents...)
	if len(b.incidents) > MaxIncidents {
		b.incidents = b.incidents[:MaxIncidents]
	}
	b.mu.Unlock()
	return inc
}

// Incidents returns a copy of recent incidents (newest first).
func (b *BlackBox) Incidents() []Incident {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]Incident(nil), b.incidents...)
}

// SmoothnessStatus builds the Lag-tab banner payload.
func (b *BlackBox) SmoothnessStatus() map[string]interface{} {
	b.mu.RLock()
	incs := append([]Incident(nil), b.incidents...)
	b.mu.RUnlock()
	return SmoothnessFromIncidents(incs, b.ring.Len())
}

func toTimeline(snaps []Snapshot, center time.Time) []TimelinePoint {
	out := make([]TimelinePoint, 0, len(snaps))
	for _, s := range snaps {
		out = append(out, TimelinePoint{
			OffsetSec:     s.At.Sub(center).Seconds(),
			CPUPercent:    s.CPUPercent,
			GPUPercent:    s.GPUPercent,
			RAMPercent:    s.RAMPercent,
			DiskReadMBps:  s.DiskReadMBps,
			DiskWriteMBps: s.DiskWriteMBps,
		})
	}
	return out
}
