package diag

import "time"

// RadarEntry is a process scored for the Process Radar.
type RadarEntry struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpuPercent"`
	RSSMB      float64 `json:"rssMB"`
	DiskMBps   float64 `json:"diskMBps"`
	Score      float64 `json:"score"`
	Trend      string  `json:"trend"` // up | down | flat | new
}

// LeakSuspect tracks rising RSS over time.
type LeakSuspect struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	RSSMB      float64 `json:"rssMB"`
	DeltaMB    float64 `json:"deltaMB"`
	Samples    int     `json:"samples"`
	Suspicion  float64 `json:"suspicion"` // 0..1
	SummaryKey string  `json:"summaryKey"`
}

// MemPoint is one RSS observation.
type MemPoint struct {
	At    time.Time
	RSSMB float64
}

// LeakWatch keeps short RSS histories per PID.
type LeakWatch struct {
	hist map[int32][]MemPoint
	name map[int32]string
}

// NewLeakWatch constructs an empty watcher.
func NewLeakWatch() *LeakWatch {
	return &LeakWatch{
		hist: make(map[int32][]MemPoint),
		name: make(map[int32]string),
	}
}

// Observe records RSS for a process (keeps last 30 points).
func (w *LeakWatch) Observe(pid int32, name string, rssMB float64) {
	if pid <= 0 || rssMB <= 0 {
		return
	}
	w.name[pid] = name
	pts := append(w.hist[pid], MemPoint{At: time.Now(), RSSMB: rssMB})
	if len(pts) > 30 {
		pts = pts[len(pts)-30:]
	}
	w.hist[pid] = pts
}

// Suspects returns processes with steadily rising RSS.
func (w *LeakWatch) Suspects(minSamples int, minDeltaMB float64) []LeakSuspect {
	out := make([]LeakSuspect, 0)
	for pid, pts := range w.hist {
		if len(pts) < minSamples {
			continue
		}
		first := pts[0].RSSMB
		last := pts[len(pts)-1].RSSMB
		delta := last - first
		if delta < minDeltaMB {
			continue
		}
		// Require mostly non-decreasing steps.
		up := 0
		for i := 1; i < len(pts); i++ {
			if pts[i].RSSMB >= pts[i-1].RSSMB-1 {
				up++
			}
		}
		ratio := float64(up) / float64(len(pts)-1)
		if ratio < 0.7 {
			continue
		}
		sus := delta / (first + 1)
		if sus > 1 {
			sus = 1
		}
		out = append(out, LeakSuspect{
			PID:        pid,
			Name:       w.name[pid],
			RSSMB:      last,
			DeltaMB:    delta,
			Samples:    len(pts),
			Suspicion:  sus,
			SummaryKey: "leakSuspect",
		})
	}
	return out
}

// ScoreRadar ranks processes for the radar UI.
func ScoreRadar(samples []ProcessSample, prev map[int32]float64) []RadarEntry {
	out := make([]RadarEntry, 0, len(samples))
	for _, s := range samples {
		score := s.CPUPercent*1.2 + s.RSSMB*0.02 + s.DiskMBps*2
		trend := "new"
		if prevCPU, ok := prev[s.PID]; ok {
			d := s.CPUPercent - prevCPU
			if d > 3 {
				trend = "up"
			} else if d < -3 {
				trend = "down"
			} else {
				trend = "flat"
			}
		}
		out = append(out, RadarEntry{
			PID: s.PID, Name: s.Name, CPUPercent: s.CPUPercent,
			RSSMB: s.RSSMB, DiskMBps: s.DiskMBps, Score: score, Trend: trend,
		})
	}
	// simple insertion sort by score desc
	for i := 1; i < len(out); i++ {
		j := i
		for j > 0 && out[j].Score > out[j-1].Score {
			out[j], out[j-1] = out[j-1], out[j]
			j--
		}
	}
	if len(out) > 15 {
		out = out[:15]
	}
	return out
}
