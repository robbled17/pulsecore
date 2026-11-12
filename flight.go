package main

import (
	"time"

	"github.com/robbled17/zeropulse/internal/diag"
)

const (
	flightSleepQuantum = 8 * time.Millisecond
	flightCritical     = 300 * time.Millisecond // only real micro-stutters
	maxStutterEvents   = 12
	criticalFreeRAMMB  = 300
)

// StutterEvent is a Flight Recorder / Black Box incident explaining a micro-stutter.
type StutterEvent struct {
	Timestamp     string               `json:"timestamp"`
	JitterMs      float64              `json:"jitterMs"`
	Cause         string               `json:"cause"` // i18n key or legacy text
	Severity      string               `json:"severity"`
	Culprit       string               `json:"culprit"`
	DiskBusy      bool                 `json:"diskBusy"`
	FreeRAMMB     uint64               `json:"freeRAMMB"`
	DiskReadMBps  float64              `json:"diskReadMBps"`
	DiskWriteMBps float64              `json:"diskWriteMBps"`
	VerdictKey    string               `json:"verdictKey"`
	CauseKey      string               `json:"causeKey"`
	DetailKey     string               `json:"detailKey"`
	Confidence    float64              `json:"confidence"`
	IncidentID    string               `json:"incidentId"`
	Source        string               `json:"source"`
	PreWindow     []diag.TimelinePoint `json:"preWindow"`
	PostWindow    []diag.TimelinePoint `json:"postWindow"`
}

func (a *App) startFlightRecorder() {
	go func() {
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

			jitterMs := float64(overshoot) / float64(time.Millisecond)
			bb := a.ensureBlackBox()
			inc := bb.RecordHitch(jitterMs, "flight")
			a.pushStutterEvent(stutterFromIncident(inc))
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
// Prefers Black Box honesty (UNKNOWN / warmup / watching) over a false "ideal".
func (a *App) SmoothnessStatus() map[string]interface{} {
	return a.ensureBlackBox().SmoothnessStatus()
}
