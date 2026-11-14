package main

import (
	"time"
)

const (
	lagTickLoaded   = 50 * time.Millisecond
	lagTickIdle     = 200 * time.Millisecond
	lagThreshLoaded = 150 * time.Millisecond
	lagThreshIdle   = 450 * time.Millisecond // avoid false positives from normal timer slack
)

// startLagDetector watches goroutine scheduling delays and confirms them with load signs.
// A PulseCore schedule delay alone is never treated as a proven global Windows freeze.
func (a *App) startLagDetector() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logCrash("lagDetector", r)
			}
		}()
		// Startup grace: window may be hidden / cold-start; skip false first incident.
		select {
		case <-a.stopWorker:
			return
		case <-time.After(5 * time.Second):
		}

		for {
			a.mu.RLock()
			cpu := a.metrics.CPUPercent
			hidden := a.hidden
			a.mu.RUnlock()

			loaded := cpu >= 40
			tick := lagTickIdle
			thresh := lagThreshIdle
			if loaded {
				tick = lagTickLoaded
				thresh = lagThreshLoaded
			}
			if hidden && !loaded {
				tick = 500 * time.Millisecond
				thresh = 800 * time.Millisecond
			}

			start := time.Now()
			timer := time.NewTimer(tick)
			select {
			case <-a.stopWorker:
				timer.Stop()
				return
			default:
			}
			if a.ctx != nil {
				select {
				case <-a.stopWorker:
					timer.Stop()
					return
				case <-a.ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			} else {
				select {
				case <-a.stopWorker:
					timer.Stop()
					return
				case <-timer.C:
				}
			}

			elapsed := time.Since(start)
			overshoot := elapsed - tick
			if overshoot < thresh {
				continue
			}
			durationMs := float64(overshoot) / float64(time.Millisecond)

			loadOK := a.currentLoadSign()
			kind := StallPulseCoreSchedule
			if loadOK {
				kind = StallLikelySystem
				// Strong corroboration → confirmed.
				a.mu.RLock()
				busyDisk := false
				for _, d := range a.metrics.Disks {
					if d.ReadMBps+d.WriteMBps >= 40 {
						busyDisk = true
						break
					}
				}
				ramHigh := a.metrics.RAMPercent >= 90
				cpuHigh := a.metrics.CPUPercent >= 70
				a.mu.RUnlock()
				if busyDisk || ramHigh || cpuHigh {
					kind = StallConfirmed
				}
			} else {
				// No load signs: keep as local schedule delay, do not raise a user incident.
				continue
			}

			a.recordLagIncident(time.Now(), durationMs, kind, loadOK)
		}
	}()
}

func (a *App) currentLoadSign() bool {
	bb := a.ensureBlackBox()
	if latest, ok := bb.ring.Latest(); ok && latest.HasLoadSign {
		return true
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.metrics.CPUPercent >= 35 {
		return true
	}
	if a.metrics.RAMPercent >= 90 {
		return true
	}
	for _, d := range a.metrics.Disks {
		if d.ReadMBps+d.WriteMBps >= 15 {
			return true
		}
	}
	return false
}
