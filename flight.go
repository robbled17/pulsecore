package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

const (
	flightSleepQuantum = 8 * time.Millisecond
	flightCritical     = 300 * time.Millisecond // only real micro-stutters
	maxStutterEvents   = 5
	criticalFreeRAMMB  = 300
)

// StutterEvent is a Flight Recorder incident explaining a micro-stutter.
type StutterEvent struct {
	Timestamp     string  `json:"timestamp"`
	JitterMs      float64 `json:"jitterMs"`
	Cause         string  `json:"cause"`
	Severity      string  `json:"severity"` // critical only in UI
	Culprit       string  `json:"culprit"`
	DiskBusy      bool    `json:"diskBusy"`
	FreeRAMMB     uint64  `json:"freeRAMMB"`
	DiskReadMBps  float64 `json:"diskReadMBps"`
	DiskWriteMBps float64 `json:"diskWriteMBps"`
}

func (a *App) startFlightRecorder() {
	go func() {
		var (
			prevIO   disk.IOCountersStat
			prevTime time.Time
			havePrev bool
		)
		for {
			select {
			case <-a.stopWorker:
				return
			default:
			}
			if a.ctx != nil {
				select {
				case <-a.ctx.Done():
					return
				default:
				}
			}

			start := time.Now()
			time.Sleep(flightSleepQuantum)
			elapsed := time.Since(start)
			overshoot := elapsed - flightSleepQuantum

			// Ignore normal Windows background jitter; only record real hitches.
			if overshoot < flightCritical {
				continue
			}

			diskBusy := false
			var readMBps, writeMBps float64
			freeMB := uint64(0)
			cause := "Зависание вызвано фоновой нагрузкой Windows"
			culprit := ""

			if vm, err := mem.VirtualMemory(); err == nil {
				freeMB = vm.Available / (1024 * 1024)
				if freeMB < criticalFreeRAMMB {
					cause = fmt.Sprintf("Зависание %.0f мс: критически мало свободной RAM", float64(overshoot)/float64(time.Millisecond))
				}
			}

			if counters, err := disk.IOCounters(); err == nil {
				now := time.Now()
				var cur disk.IOCountersStat
				found := false
				for name, c := range counters {
					ln := strings.ToLower(name)
					if strings.Contains(ln, "c:") || strings.HasSuffix(ln, "0") || strings.Contains(ln, "physicaldrive0") {
						cur = c
						found = true
						break
					}
				}
				if !found {
					for _, c := range counters {
						cur = c
						found = true
						break
					}
				}
				if found && havePrev {
					sec := now.Sub(prevTime).Seconds()
					if sec > 0 {
						readMBps = float64(cur.ReadBytes-prevIO.ReadBytes) / (1024 * 1024) / sec
						writeMBps = float64(cur.WriteBytes-prevIO.WriteBytes) / (1024 * 1024) / sec
						if readMBps < 0 {
							readMBps = 0
						}
						if writeMBps < 0 {
							writeMBps = 0
						}
						if readMBps+writeMBps >= 40 {
							diskBusy = true
							cause = fmt.Sprintf("Зависание %.0f мс вызвано активностью диска C:", float64(overshoot)/float64(time.Millisecond))
							culprit = "SearchIndexer.exe"
						}
					}
				}
				if found {
					prevIO = cur
					prevTime = now
					havePrev = true
				}
			}

			if !diskBusy && freeMB >= criticalFreeRAMMB {
				if name, _ := findHottestProcess(); name != "" && !isEcoProtected(name) {
					culprit = name
					cause = fmt.Sprintf("Зависание %.0f мс: высокая нагрузка от %s", float64(overshoot)/float64(time.Millisecond), friendlyProcessName(name))
				}
			}

			ev := StutterEvent{
				Timestamp:     time.Now().Format("15:04:05"),
				JitterMs:      float64(overshoot) / float64(time.Millisecond),
				Cause:         cause,
				Severity:      "critical",
				Culprit:       culprit,
				DiskBusy:      diskBusy,
				FreeRAMMB:     freeMB,
				DiskReadMBps:  readMBps,
				DiskWriteMBps: writeMBps,
			}
			a.pushStutterEvent(ev)
		}
	}()
}

func (a *App) pushStutterEvent(ev StutterEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stutterEvents = append([]StutterEvent{ev}, a.stutterEvents...)
	if len(a.stutterEvents) > maxStutterEvents {
		a.stutterEvents = a.stutterEvents[:maxStutterEvents]
	}
}

// GetStutterEvents returns the Flight Recorder ring buffer.
func (a *App) GetStutterEvents() []StutterEvent {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]StutterEvent(nil), a.stutterEvents...)
}

// SmoothnessStatus returns a human status for the Lag tab banner.
func (a *App) SmoothnessStatus() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()
	maxJ := 0.0
	var last *StutterEvent
	for i := range a.stutterEvents {
		if a.stutterEvents[i].JitterMs > maxJ {
			maxJ = a.stutterEvents[i].JitterMs
		}
		if a.stutterEvents[i].Severity == "critical" && last == nil {
			last = &a.stutterEvents[i]
		}
	}
	if last != nil || maxJ >= 300 {
		j := maxJ
		cause := ""
		culprit := ""
		if last != nil {
			j = last.JitterMs
			cause = last.Cause
			culprit = last.Culprit
		}
		return map[string]interface{}{
			"level":   "warn",
			"message": fmt.Sprintf("Обнаружен микролаг (+%.0f мс)", j),
			"cause":   cause,
			"culprit": culprit,
		}
	}
	return map[string]interface{}{
		"level":   "ok",
		"message": "Система работает идеально. Микрофризов в играх и программах не зафиксировано.",
	}
}
