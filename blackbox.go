package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/mem"
)

const (
	maxLagIncidents     = 32
	incidentMergeWindow = 3 * time.Second
	pressureHold        = 3 * time.Second
)

// blackBox holds the snapshot ring and recent diagnostic incidents.
type blackBox struct {
	mu            sync.Mutex
	ring          SnapshotRing
	incidents     []LagIncident
	openID        string
	openStart     time.Time
	pendingAfter  int // snapshots still needed after open incident closes
	pressureSince time.Time
	peakFreqMHz   float64
	startedAt     time.Time
	warmupDone    bool
	lastProcSample time.Time
	cachedTopCPU  []ProcessSnap
	cachedTopDisk []ProcessSnap
	cachedTopRAM  []ProcessSnap
}

func newBlackBox() *blackBox {
	return &blackBox{
		incidents: make([]LagIncident, 0, maxLagIncidents),
		startedAt: time.Now(),
	}
}

func (a *App) ensureBlackBox() *blackBox {
	if a.bb == nil {
		a.bb = newBlackBox()
	}
	return a.bb
}

// pushBlackBoxSnapshot builds a Snapshot from the live metrics tick and stores it.
// Process tops are refreshed adaptively to avoid heavy walks every idle tick.
func (a *App) pushBlackBoxSnapshot(now time.Time, cpuPct float64, perCore []float64, vm *mem.VirtualMemoryStat, sm *mem.SwapMemoryStat, disks []DiskMetrics, gpuPct, temp float64) {
	bb := a.ensureBlackBox()

	a.mu.RLock()
	hidden := a.hidden
	tab := a.activeTab
	freq := a.staticHW.frequencyMHz
	appActive := !hidden
	a.mu.RUnlock()

	var read, write float64
	for _, d := range disks {
		read += d.ReadMBps
		write += d.WriteMBps
	}
	busy, _ := diskBusyFromMBps(read, write)

	ramUsed, ramTotal, ramAvail, ramPct := uint64(0), uint64(0), uint64(0), 0.0
	if vm != nil {
		ramUsed = vm.Used / (1024 * 1024)
		ramTotal = vm.Total / (1024 * 1024)
		ramAvail = vm.Available / (1024 * 1024)
		ramPct = vm.UsedPercent
	}
	swapPct, swapUsed := 0.0, uint64(0)
	if sm != nil {
		swapPct = sm.UsedPercent
		swapUsed = sm.Used / (1024 * 1024)
	}

	loadSign := cpuPct >= 35 || busy >= 40 || ramPct >= 90 || (read+write) >= 15
	underLoad := cpuPct >= 40 || busy >= 50 || tab == "lag"

	bb.mu.Lock()
	if freq > bb.peakFreqMHz {
		bb.peakFreqMHz = freq
	}
	needProc := bb.lastProcSample.IsZero() ||
		now.Sub(bb.lastProcSample) >= 3*time.Second ||
		(underLoad && now.Sub(bb.lastProcSample) >= time.Second)
	if hidden && !underLoad {
		needProc = bb.lastProcSample.IsZero() || now.Sub(bb.lastProcSample) >= 5*time.Second
	}
	topCPU := cloneProcs(bb.cachedTopCPU)
	topDisk := cloneProcs(bb.cachedTopDisk)
	topRAM := cloneProcs(bb.cachedTopRAM)
	bb.mu.Unlock()

	if needProc {
		c, d, r := sampleProcessTops(5, 3, 5)
		bb.mu.Lock()
		bb.cachedTopCPU = c
		bb.cachedTopDisk = d
		bb.cachedTopRAM = r
		bb.lastProcSample = now
		bb.mu.Unlock()
		topCPU, topDisk, topRAM = c, d, r
		a.observeProcessRadar(now, cpuPct, c, d, r)
	}

	gameLoad := a.isGameSessionActive()

	snap := Snapshot{
		Timestamp:      now,
		CPUPercent:     cpuPct,
		CPUPerCore:     append([]float64(nil), perCore...),
		CPUFreqMHz:     freq,
		CPUTempC:       temp,
		GPUPercent:     gpuPct,
		RAMUsedMB:      ramUsed,
		RAMTotalMB:     ramTotal,
		RAMAvailableMB: ramAvail,
		RAMPercent:     ramPct,
		SwapPercent:    swapPct,
		SwapUsedMB:     swapUsed,
		DiskBusyPct:    busy,
		DiskReadMBps:   read,
		DiskWriteMBps:  write,
		TopCPU:         topCPU,
		TopDisk:        topDisk,
		TopRAM:         topRAM,
		AppActive:      appActive,
		GameLoad:       gameLoad,
		HasLoadSign:    loadSign,
	}

	bb.mu.Lock()
	bb.ring.Push(snap)
	if !bb.warmupDone && now.Sub(bb.startedAt) >= 5*time.Second {
		bb.warmupDone = true
	}
	// Fill after-window for recently closed incidents.
	for i := range bb.incidents {
		if len(bb.incidents[i].After) >= 5 {
			continue
		}
		if snap.Timestamp.After(bb.incidents[i].End) {
			bb.incidents[i].After = append(bb.incidents[i].After, cloneSnapshot(snap))
		}
	}
	pressureHit := (busy >= 99.5 /* queue unavailable: treat sustained 100% busy */) || ramPct > 95
	var openStart time.Time
	var pressureSince time.Time
	warmup := bb.warmupDone
	if pressureHit {
		if bb.pressureSince.IsZero() {
			bb.pressureSince = now
		}
	} else {
		bb.pressureSince = time.Time{}
	}
	pressureSince = bb.pressureSince
	openStart = bb.openStart
	bb.mu.Unlock()

	// Sustained pressure incident (≥3s).
	if warmup && !pressureSince.IsZero() && now.Sub(pressureSince) >= pressureHold {
		if openStart.IsZero() || now.Sub(openStart) >= incidentMergeWindow {
			a.recordLagIncident(now, 3000, StallConfirmed, true)
			bb.mu.Lock()
			bb.pressureSince = now // reset hold window
			bb.mu.Unlock()
		}
	}

	// Keep legacy MetricSnapshot path fed for DetectFreeze compatibility.
	a.pushLagSnapshot(cpuPct, perCore, disks, topFromProcessSnaps(topCPU, 5))
}

func topFromProcessSnaps(list []ProcessSnap, limit int) []ProcessMini {
	if limit <= 0 {
		limit = 5
	}
	out := make([]ProcessMini, 0, limit)
	for i, p := range list {
		if i >= limit {
			break
		}
		out = append(out, ProcessMini{Name: p.Name, CPUPercent: p.CPUPercent, DiskMBps: p.DiskMBps})
	}
	return out
}

// recordLagIncident merges nearby stalls and runs the verdict engine.
func (a *App) recordLagIncident(when time.Time, durationMs float64, kind StallKind, loadOK bool) {
	bb := a.ensureBlackBox()
	bb.mu.Lock()
	if !bb.warmupDone {
		bb.mu.Unlock()
		return
	}
	// Merge into open / recent incident within 3s.
	if len(bb.incidents) > 0 {
		last := &bb.incidents[0]
		if when.Sub(last.End) <= incidentMergeWindow || when.Sub(last.Start) <= incidentMergeWindow {
			if when.After(last.End) {
				last.End = when
			}
			last.DurationMs += durationMs
			if kind == StallConfirmed {
				last.Kind = StallConfirmed
			}
			bb.mu.Unlock()
			return
		}
	}

	before := bb.ring.LastN(10)
	during := []Snapshot{}
	if latest, ok := bb.ring.Latest(); ok {
		during = []Snapshot{cloneSnapshot(latest)}
	}
	peak := bb.peakFreqMHz
	bb.mu.Unlock()

	hints := IncidentHints{
		DurationMs:  durationMs,
		Kind:        kind,
		HasLoadSign: loadOK,
		PeakCPUFreq: peak,
	}
	if !loadOK {
		hints.IgnoreLoad = true
	}
	verdict := EvaluateVerdict(append(append([]Snapshot{}, before...), during...), hints)

	inc := LagIncident{
		ID:         fmt.Sprintf("%d", when.UnixNano()),
		Start:      when.Add(-time.Duration(durationMs) * time.Millisecond),
		End:        when,
		DurationMs: durationMs,
		Kind:       kind,
		Verdict:    verdict,
		Evidence:   append([]string{}, verdict.Evidence...),
		Before:     before,
		During:     during,
	}

	bb.mu.Lock()
	bb.incidents = append([]LagIncident{inc}, bb.incidents...)
	if len(bb.incidents) > maxLagIncidents {
		bb.incidents = bb.incidents[:maxLagIncidents]
	}
	bb.openStart = when
	bb.mu.Unlock()

	a.recordStatsIncident(verdict.Culprit)
	a.noteGameFreeze(string(verdict.Type))

	// Mirror into stutterEvents for existing GetMetrics / UI polling path.
	a.pushStutterEvent(StutterEvent{
		Timestamp:     when.Format("15:04:05"),
		JitterMs:      durationMs,
		Cause:         verdictText(verdict, a.getLang()),
		Severity:      "critical",
		Culprit:       verdict.Culprit,
		DiskBusy:      verdict.Type == VerdictDiskHog,
		FreeRAMMB:     0,
		DiskReadMBps:  0,
		DiskWriteMBps: 0,
	})
}

func (a *App) getLang() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.language == "" {
		return "ru"
	}
	return a.language
}

func verdictText(v Verdict, lang string) string {
	if lang == "en" {
		return v.TextEn
	}
	return v.TextRu
}

// GetLagIncidents returns the last 5 Black Box incident cards for the Lag tab.
func (a *App) GetLagIncidents() []IncidentCard {
	bb := a.ensureBlackBox()
	lang := a.getLang()
	bb.mu.Lock()
	defer bb.mu.Unlock()
	n := len(bb.incidents)
	if n > 5 {
		n = 5
	}
	out := make([]IncidentCard, 0, n)
	for i := 0; i < n; i++ {
		inc := bb.incidents[i]
		targets := append([]EcoTarget(nil), inc.Verdict.EcoTargets...)
		canEco := inc.Verdict.Action == "eco" && (len(targets) > 0 || (inc.Verdict.CulpritPID > 0 && !inc.Verdict.SystemProc))
		human := append([]string{}, inc.Verdict.Evidence...)
		if len(human) == 0 {
			human = append([]string{}, inc.Evidence...)
		}
		out = append(out, IncidentCard{
			ID:          inc.ID,
			Time:        inc.Start.Format("15:04:05"),
			DurationMs:  inc.DurationMs,
			Type:        string(inc.Verdict.Type),
			VerdictText: verdictText(inc.Verdict, lang),
			Culprit:     inc.Verdict.Culprit,
			CulpritPID:  inc.Verdict.CulpritPID,
			Confidence:  inc.Verdict.Confidence,
			Evidence:    human,
			EvidenceRaw: append([]string{}, inc.Verdict.EvidenceRaw...),
			EcoTargets:  targets,
			CanEco:      canEco,
			SystemProc:  inc.Verdict.SystemProc,
			Synthetic:   inc.Synthetic,
			Severity:    inc.Verdict.Severity,
		})
	}
	return out
}

// GetIncidentTimeline returns before/during/after snapshots for charts.
func (a *App) GetIncidentTimeline(id string) map[string]interface{} {
	bb := a.ensureBlackBox()
	bb.mu.Lock()
	defer bb.mu.Unlock()
	for i := range bb.incidents {
		inc := bb.incidents[i]
		if inc.ID != id {
			continue
		}
		return map[string]interface{}{
			"id":         inc.ID,
			"start":      inc.Start.Format(time.RFC3339),
			"end":        inc.End.Format(time.RFC3339),
			"durationMs": inc.DurationMs,
			"verdict":    inc.Verdict,
			"before":     inc.Before,
			"during":     inc.During,
			"after":      inc.After,
		}
	}
	return map[string]interface{}{"error": "not_found"}
}

// BlackBoxStatus is the honesty banner for «Почему лагнуло?».
func (a *App) BlackBoxStatus() map[string]interface{} {
	bb := a.ensureBlackBox()
	bb.mu.Lock()
	n := len(bb.incidents)
	warmup := bb.warmupDone
	bb.mu.Unlock()
	if !warmup {
		return map[string]interface{}{
			"level":   "ok",
			"message": "observing",
			"count":   0,
		}
	}
	if n == 0 {
		return map[string]interface{}{
			"level":   "ok",
			"message": "no_freezes",
			"count":   0,
		}
	}
	cards := a.GetLagIncidents()
	cause := ""
	if len(cards) > 0 {
		cause = cards[0].VerdictText
	}
	return map[string]interface{}{
		"level":   "warn",
		"message": "has_incidents",
		"count":   n,
		"cause":   cause,
	}
}
