package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

const (
	vampHoldTime   = 15 * time.Second
	vampMAWindow   = 10 // ~10s at 1 Hz poll
	vampDiskThresh = 2.0
)

type vampTrack struct {
	key       string
	name      string
	cpuRing   []float64
	diskRing  []float64
	ringIdx   int
	pids      []int32
	ecoAny    bool
	lastHot   time.Time
	lastPush  time.Time
}

func (t *vampTrack) push(cpu, disk float64, pids []int32, eco bool) {
	t.lastPush = time.Now()
	t.pids = append([]int32(nil), pids...)
	if eco {
		t.ecoAny = true
	}
	if len(t.cpuRing) < vampMAWindow {
		t.cpuRing = append(t.cpuRing, cpu)
		t.diskRing = append(t.diskRing, disk)
	} else {
		t.cpuRing[t.ringIdx] = cpu
		t.diskRing[t.ringIdx] = disk
		t.ringIdx = (t.ringIdx + 1) % vampMAWindow
	}
	if cpu >= vampireCPUPct || disk >= vampDiskThresh || eco {
		t.lastHot = time.Now()
	}
}

func (t *vampTrack) avgCPU() float64 {
	if len(t.cpuRing) == 0 {
		return 0
	}
	var s float64
	for _, v := range t.cpuRing {
		s += v
	}
	return s / float64(len(t.cpuRing))
}

func (t *vampTrack) avgDisk() float64 {
	if len(t.diskRing) == 0 {
		return 0
	}
	var s float64
	for _, v := range t.diskRing {
		s += v
	}
	return s / float64(len(t.diskRing))
}

func (t *vampTrack) visible() bool {
	if t.ecoAny {
		return true
	}
	if !t.lastHot.IsZero() && time.Since(t.lastHot) <= vampHoldTime {
		return true
	}
	return t.avgCPU() >= vampireCPUPct || t.avgDisk() >= vampDiskThresh
}

func vampReason(cpu, disk float64) (ru, en string) {
	if disk >= vampDiskThresh && disk*0.6 >= cpu*0.4 {
		return fmt.Sprintf("Нагружает диск (%.1f MB/s): фоновая индексация или запись логов", disk),
			fmt.Sprintf("Disk load (%.1f MB/s): background indexing or log writes", disk)
	}
	if cpu >= vampireCPUPct || cpu > 0 {
		return fmt.Sprintf("Загружает поток CPU (%.0f%%): фоновые вычисления в режиме простоя", cpu),
			fmt.Sprintf("CPU load (%.0f%%): background work while idle", cpu)
	}
	if disk > 0 {
		return fmt.Sprintf("Нагружает диск (%.1f MB/s): фоновая индексация или запись логов", disk),
			fmt.Sprintf("Disk load (%.1f MB/s): background indexing or log writes", disk)
	}
	return "Недавно нагружал систему — удерживаем в списке",
		"Recently busy — held in the list"
}

func (a *App) ensureVampTracks() map[string]*vampTrack {
	if a.vampTracks == nil {
		a.vampTracks = make(map[string]*vampTrack)
	}
	return a.vampTracks
}

// GetVampireProcesses finds background CPU/disk hogs with MA + hold-time anti-flicker.
func (a *App) GetVampireProcesses() []VampireProcess {
	fgPID := foregroundPID()
	procs, err := process.Processes()
	if err != nil {
		return []VampireProcess{}
	}

	type agg struct {
		name   string
		pids   []int32
		cpu    float64
		disk   float64
		ecoAny bool
	}
	byName := map[string]*agg{}
	alive := map[int32]bool{}

	for _, p := range procs {
		if int32(p.Pid) == fgPID || p.Pid == 0 {
			continue
		}
		name, err := p.Name()
		if err != nil || name == "" {
			continue
		}
		lower := strings.ToLower(filepath.Base(name))
		if isSelfOrWebView(lower) || lower == "system" || lower == "idle" {
			continue
		}

		rawCPU, err := p.CPUPercent()
		if err != nil {
			continue
		}
		cpuPct := normalizeProcessCPU(rawCPU)
		io, _ := p.IOCounters()
		diskMBps := processDiskRateMBps(p.Pid, io)
		alive[p.Pid] = true

		a.mu.RLock()
		ecoOn := a.ecoPIDs[p.Pid]
		a.mu.RUnlock()

		// Feed everything into tracks; visibility uses MA + hold.
		g := byName[lower]
		if g == nil {
			g = &agg{name: name}
			byName[lower] = g
		}
		g.pids = append(g.pids, p.Pid)
		g.cpu += cpuPct
		g.disk += diskMBps
		if ecoOn {
			g.ecoAny = true
		}
	}

	a.mu.RLock()
	ecoCopy := make([]int32, 0, len(a.ecoPIDs))
	for pid, on := range a.ecoPIDs {
		if on {
			ecoCopy = append(ecoCopy, pid)
		}
	}
	a.mu.RUnlock()
	for _, pid := range ecoCopy {
		if !processStillAlive(pid) {
			continue
		}
		n := processNameByPID(int(pid))
		if n == "" {
			continue
		}
		lower := strings.ToLower(filepath.Base(n))
		if g := byName[lower]; g != nil {
			found := false
			for _, existing := range g.pids {
				if existing == pid {
					found = true
					break
				}
			}
			if !found {
				g.pids = append(g.pids, pid)
			}
			g.ecoAny = true
			continue
		}
		byName[lower] = &agg{name: n, pids: []int32{pid}, ecoAny: true}
	}
	pruneProcIOCache(alive)

	a.mu.Lock()
	tracks := a.ensureVampTracks()
	seenKeys := map[string]bool{}
	for key, g := range byName {
		seenKeys[key] = true
		tr := tracks[key]
		if tr == nil {
			tr = &vampTrack{key: key, name: g.name}
			tracks[key] = tr
		}
		tr.key = key
		tr.name = g.name
		tr.ecoAny = g.ecoAny
		tr.push(clamp01to100(g.cpu), g.disk, g.pids, g.ecoAny)
	}
	// Drop stale tracks that vanished and aged out of hold.
	for key, tr := range tracks {
		if seenKeys[key] {
			continue
		}
		if tr.ecoAny {
			// Keep until Eco cleared / process gone.
			aliveEco := false
			for _, pid := range tr.pids {
				if processStillAlive(pid) {
					aliveEco = true
					break
				}
			}
			if !aliveEco {
				delete(tracks, key)
			}
			continue
		}
		if time.Since(tr.lastHot) > vampHoldTime && time.Since(tr.lastPush) > vampHoldTime {
			delete(tracks, key)
		}
	}

	list := make([]VampireProcess, 0, len(tracks))
	for _, tr := range tracks {
		if !tr.visible() {
			continue
		}
		cpu := clamp01to100(tr.avgCPU())
		disk := tr.avgDisk()
		if disk < 0 {
			disk = 0
		}
		// Prefer live snapshot when still hot this tick.
		if g := byName[tr.key]; g != nil {
			liveCPU := clamp01to100(g.cpu)
			liveDisk := g.disk
			if liveCPU > cpu {
				cpu = liveCPU
			}
			if liveDisk > disk {
				disk = liveDisk
			}
		}
		score := cpu*1.4 + disk*0.6
		pid := int32(0)
		if len(tr.pids) > 0 {
			pid = tr.pids[0]
		}
		ru, en := vampReason(cpu, disk)
		list = append(list, VampireProcess{
			PID:          pid,
			PIDs:         append([]int32(nil), tr.pids...),
			Count:        len(tr.pids),
			Name:         tr.name,
			FriendlyName: friendlyProcessName(tr.name),
			CPUPercent:   cpu,
			DiskMBps:     disk,
			Score:        score,
			EcoMode:      tr.ecoAny,
			Frozen:       tr.ecoAny,
			ReasonRu:     ru,
			ReasonEn:     en,
		})
	}
	a.mu.Unlock()

	sort.Slice(list, func(i, j int) bool {
		if list[i].EcoMode != list[j].EcoMode {
			return list[i].EcoMode
		}
		return list[i].Score > list[j].Score
	})
	limit := 16
	if len(list) < limit {
		limit = len(list)
	}
	out := list[:limit]

	a.mu.Lock()
	a.vampires = append([]VampireProcess(nil), out...)
	a.mu.Unlock()
	return out
}
