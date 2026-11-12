package main

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// ProcessMini is a compact process tip for the black-box ring buffer.
type ProcessMini struct {
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpuPercent"`
	DiskMBps   float64 `json:"diskMBps"`
}

// MetricSnapshot is one second of system state kept in the lag ring buffer.
type MetricSnapshot struct {
	Timestamp    time.Time     `json:"timestamp"`
	TotalCPU     float64       `json:"totalCPU"`
	TopProcesses []ProcessMini `json:"topProcesses"`
	DiskBusyPct  float64       `json:"diskBusyPct"`
	DiskMBps     float64       `json:"diskMBps"`
	MaxCoreLoad  float64       `json:"maxCoreLoad"`
	MaxCoreID    int           `json:"maxCoreID"`
}

// RingBuffer keeps the last 60 one-second snapshots (~<2MB).
type RingBuffer struct {
	mu   sync.Mutex
	buf  [60]MetricSnapshot
	n    int
	next int
}

func (r *RingBuffer) Push(s MetricSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.next] = s
	r.next = (r.next + 1) % len(r.buf)
	if r.n < len(r.buf) {
		r.n++
	}
}

func (r *RingBuffer) Latest() (MetricSnapshot, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.n == 0 {
		return MetricSnapshot{}, false
	}
	idx := r.next - 1
	if idx < 0 {
		idx = len(r.buf) - 1
	}
	return r.buf[idx], true
}

// DetectFreeze returns a human verdict when a core pegs at 100% with heavy disk IO.
func (r *RingBuffer) DetectFreeze() string {
	snap, ok := r.Latest()
	if !ok {
		return ""
	}
	if snap.MaxCoreLoad < 99.5 {
		return ""
	}
	if snap.DiskBusyPct < 90 && snap.DiskMBps < 40 {
		return ""
	}
	culprit := "фоновый процесс"
	diskRate := snap.DiskMBps
	if len(snap.TopProcesses) > 0 {
		top := snap.TopProcesses[0]
		for _, p := range snap.TopProcesses {
			if p.DiskMBps > top.DiskMBps {
				top = p
			}
		}
		if top.Name != "" {
			culprit = top.Name
		}
		if top.DiskMBps > diskRate {
			diskRate = top.DiskMBps
		}
	}
	ts := snap.Timestamp.Format("15:04:05")
	return fmt.Sprintf(
		"В %s зафиксирован пик: %s вызвал скачок диска (%.0f MB/s), поток %d загружен на 100%%.",
		ts, friendlyProcessName(culprit), diskRate, snap.MaxCoreID,
	)
}

func diskBusyFromMBps(readMBps, writeMBps float64) (busyPct, totalMBps float64) {
	totalMBps = readMBps + writeMBps
	if totalMBps < 0 {
		totalMBps = 0
	}
	// Map ~100 MB/s aggregate to ~100% busy for consumer SATA/NVMe heuristics.
	busyPct = math.Min(100, totalMBps)
	return busyPct, totalMBps
}

func maxCore(cores []float64) (load float64, id int) {
	for i, c := range cores {
		if c > load {
			load = c
			id = i
		}
	}
	return load, id
}

func (a *App) pushLagSnapshot(cpuPct float64, cores []float64, disks []DiskMetrics, top []ProcessMini) {
	var read, write float64
	for _, d := range disks {
		read += d.ReadMBps
		write += d.WriteMBps
	}
	busy, total := diskBusyFromMBps(read, write)
	maxLoad, maxID := maxCore(cores)
	snap := MetricSnapshot{
		Timestamp:    time.Now(),
		TotalCPU:     cpuPct,
		TopProcesses: top,
		DiskBusyPct:  busy,
		DiskMBps:     total,
		MaxCoreLoad:  maxLoad,
		MaxCoreID:    maxID,
	}
	a.lagRing.Push(snap)
	if verdict := a.lagRing.DetectFreeze(); verdict != "" {
		// Route through Black Box so verdicts stay consistent (no duplicate inventing).
		a.recordLagIncident(snap.Timestamp, 320, StallConfirmed, true)
		_ = verdict
	}
}

func topFromVampires(vamps []VampireProcess, limit int) []ProcessMini {
	if limit <= 0 {
		limit = 5
	}
	out := make([]ProcessMini, 0, limit)
	for i, v := range vamps {
		if i >= limit {
			break
		}
		name := v.Name
		if strings.TrimSpace(name) == "" {
			name = v.FriendlyName
		}
		out = append(out, ProcessMini{
			Name:       name,
			CPUPercent: v.CPUPercent,
			DiskMBps:   v.DiskMBps,
		})
	}
	return out
}
