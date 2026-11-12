package main

import (
	"sort"
	"time"

	"github.com/robbled17/zeropulse/internal/diag"
	"github.com/shirou/gopsutil/v3/process"
)

// ensure BlackBox is initialized on App.
func (a *App) ensureBlackBox() *diag.BlackBox {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.blackBox == nil {
		a.blackBox = diag.NewBlackBox()
	}
	return a.blackBox
}

func (a *App) pushLiveSnapshot(m SystemMetrics) {
	bb := a.ensureBlackBox()
	read, write := sumDiskIO(m.Disks)
	freeMB := uint64(0)
	if m.RAMTotalMB > m.RAMUsedMB {
		freeMB = m.RAMTotalMB - m.RAMUsedMB
	}
	bb.PushSnapshot(diag.Snapshot{
		At:            time.Now(),
		CPUPercent:    m.CPUPercent,
		GPUPercent:    m.GPUPercent,
		RAMPercent:    m.RAMPercent,
		RAMFreeMB:     freeMB,
		SwapPercent:   m.SwapPercent,
		DiskReadMBps:  read,
		DiskWriteMBps: write,
		TemperatureC:  m.TemperatureC,
		TopProcesses:  sampleTopProcesses(5),
	})
}

func sumDiskIO(disks []DiskMetrics) (read, write float64) {
	for _, d := range disks {
		read += d.ReadMBps
		write += d.WriteMBps
	}
	return diag.DiskTotals(read, write)
}

func sampleTopProcesses(n int) []diag.ProcessSample {
	procs, err := process.Processes()
	if err != nil || len(procs) == 0 {
		return nil
	}
	type scored struct {
		s diag.ProcessSample
	}
	tmp := make([]scored, 0, len(procs))
	for _, p := range procs {
		name, err := p.Name()
		if err != nil || name == "" {
			continue
		}
		cpuPct, _ := p.CPUPercent()
		var rssMB float64
		if mi, err := p.MemoryInfo(); err == nil && mi != nil {
			rssMB = float64(mi.RSS) / (1024 * 1024)
		}
		tmp = append(tmp, scored{s: diag.ProcessSample{
			PID:        p.Pid,
			Name:       name,
			CPUPercent: cpuPct,
			RSSMB:      rssMB,
		}})
	}
	sort.Slice(tmp, func(i, j int) bool {
		return tmp[i].s.CPUPercent > tmp[j].s.CPUPercent
	})
	if len(tmp) > n {
		tmp = tmp[:n]
	}
	out := make([]diag.ProcessSample, len(tmp))
	for i := range tmp {
		out[i] = tmp[i].s
	}
	return out
}

func stutterFromIncident(inc diag.Incident) StutterEvent {
	cause := inc.Verdict.SummaryKey
	if cause == "" {
		cause = "verdictUnknown"
	}
	sev := inc.Severity
	if sev == "" {
		sev = string(inc.Verdict.Level)
	}
	return StutterEvent{
		Timestamp:     inc.TimestampUI,
		JitterMs:      inc.JitterMs,
		Cause:         cause,
		Severity:      sev,
		Culprit:       inc.Culprit,
		DiskBusy:      inc.DiskBusy,
		FreeRAMMB:     inc.FreeRAMMB,
		DiskReadMBps:  inc.DiskReadMBps,
		DiskWriteMBps: inc.DiskWriteMBps,
		VerdictKey:    inc.Verdict.SummaryKey,
		CauseKey:      inc.Verdict.CauseKey,
		DetailKey:     inc.Verdict.DetailKey,
		Confidence:    inc.Verdict.Confidence,
		IncidentID:    inc.ID,
		Source:        inc.Source,
		PreWindow:     inc.PreWindow,
		PostWindow:    inc.PostWindow,
	}
}

// GetLagIncidents returns Black Box incidents for the timeline UI.
func (a *App) GetLagIncidents() []diag.Incident {
	return a.ensureBlackBox().Incidents()
}

// GetSnapshotRingLen returns how many historical snapshots are buffered.
func (a *App) GetSnapshotRingLen() int {
	return a.ensureBlackBox().RingLen()
}

// RecordLagFromSpotter feeds LagSpotter core-spike events into the Black Box.
func (a *App) recordLagFromSpotter(jitterProxyMs float64) {
	bb := a.ensureBlackBox()
	inc := bb.RecordHitch(jitterProxyMs, "lagspotter")
	a.pushStutterEvent(stutterFromIncident(inc))
}
