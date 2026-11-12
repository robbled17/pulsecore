package main

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/shirou/gopsutil/v3/process"
)

// sampleProcessTops walks processes once and returns top lists by CPU / disk I/O / RAM.
// Reuses normalizeProcessCPU + processDiskRateMBps (same normalization as vampires).
// Keeps real system process names; groups by EXE while preserving PIDs.
func sampleProcessTops(limitCPU, limitDisk, limitRAM int) (topCPU, topDisk, topRAM []ProcessSnap) {
	if limitCPU <= 0 {
		limitCPU = 5
	}
	if limitDisk <= 0 {
		limitDisk = 3
	}
	if limitRAM <= 0 {
		limitRAM = 5
	}

	procs, err := process.Processes()
	if err != nil {
		return nil, nil, nil
	}

	type agg struct {
		name   string
		pids   []int32
		cpu    float64
		disk   float64
		read   float64
		write  float64
		ram    uint64
	}
	byName := map[string]*agg{}
	alive := map[int32]bool{}

	for _, p := range procs {
		if p.Pid <= 0 {
			continue
		}
		name, err := p.Name()
		if err != nil || name == "" {
			continue
		}
		lower := strings.ToLower(filepath.Base(name))
		if isSelfOrWebView(lower) || lower == "idle" {
			continue
		}

		rawCPU, err := p.CPUPercent()
		cpuPct := 0.0
		if err == nil {
			cpuPct = normalizeProcessCPU(rawCPU)
		}

		var readMBps, writeMBps, diskMBps float64
		if io, err := p.IOCounters(); err == nil && io != nil {
			diskMBps = processDiskRateMBps(p.Pid, io)
			// Split approx from combined rate using cumulative share when available.
			total := io.ReadBytes + io.WriteBytes
			if total > 0 && diskMBps > 0 {
				readMBps = diskMBps * float64(io.ReadBytes) / float64(total)
				writeMBps = diskMBps * float64(io.WriteBytes) / float64(total)
			}
		}

		var ram uint64
		if mi, err := p.MemoryInfo(); err == nil && mi != nil {
			ram = mi.RSS
		}

		alive[p.Pid] = true
		g := byName[lower]
		if g == nil {
			g = &agg{name: name}
			byName[lower] = g
		}
		g.pids = append(g.pids, p.Pid)
		g.cpu += cpuPct
		g.disk += diskMBps
		g.read += readMBps
		g.write += writeMBps
		g.ram += ram
	}
	pruneProcIOCache(alive)

	all := make([]ProcessSnap, 0, len(byName))
	for _, g := range byName {
		pid := int32(0)
		if len(g.pids) > 0 {
			pid = g.pids[0]
		}
		all = append(all, ProcessSnap{
			Name:          g.name,
			PID:           pid,
			PIDs:          append([]int32(nil), g.pids...),
			CPUPercent:    clamp01to100(g.cpu),
			RAMBytes:      g.ram,
			DiskMBps:      g.disk,
			DiskReadMBps:  g.read,
			DiskWriteMBps: g.write,
		})
	}

	byCPU := append([]ProcessSnap(nil), all...)
	sort.Slice(byCPU, func(i, j int) bool { return byCPU[i].CPUPercent > byCPU[j].CPUPercent })
	topCPU = takeProcs(byCPU, limitCPU)

	byDisk := append([]ProcessSnap(nil), all...)
	sort.Slice(byDisk, func(i, j int) bool { return byDisk[i].DiskMBps > byDisk[j].DiskMBps })
	topDisk = takeProcs(byDisk, limitDisk)

	byRAM := append([]ProcessSnap(nil), all...)
	sort.Slice(byRAM, func(i, j int) bool { return byRAM[i].RAMBytes > byRAM[j].RAMBytes })
	topRAM = takeProcs(byRAM, limitRAM)

	return topCPU, topDisk, topRAM
}

func takeProcs(in []ProcessSnap, n int) []ProcessSnap {
	if len(in) == 0 {
		return nil
	}
	if n > len(in) {
		n = len(in)
	}
	out := make([]ProcessSnap, n)
	copy(out, in[:n])
	return out
}
