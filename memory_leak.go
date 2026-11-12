package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	leakMinPoints     = 6
	leakWindow        = 40 * time.Minute
	leakGrowRatio     = 1.35
	leakMinAbsMB      = 180
	leakMaxTracks     = 60
	leakMaxCards      = 16
	leakPossibleRatio = 1.8
)

// MemoryWatchVerdict classifies long-term RAM behaviour - never auto-kills.
type MemoryWatchVerdict string

const (
	MemWatchGrowth   MemoryWatchVerdict = "sustained_growth"
	MemWatchPossible MemoryWatchVerdict = "possible_leak"
	MemWatchNormal   MemoryWatchVerdict = "normal"
	MemWatchNeedData MemoryWatchVerdict = "insufficient_data"
)

// MemoryWatchCard is a Memory Leak Watch observation.
type MemoryWatchCard struct {
	Process    string             `json:"process"`
	Friendly   string             `json:"friendly"`
	FromMB     float64            `json:"fromMB"`
	ToMB       float64            `json:"toMB"`
	Minutes    float64            `json:"minutes"`
	Verdict    MemoryWatchVerdict `json:"verdict"`
	TextRu     string             `json:"textRu"`
	TextEn     string             `json:"textEn"`
	Confidence float64            `json:"confidence"`
	DetectedAt string             `json:"detectedAt"`
	PID        int32              `json:"pid"`
}

type memPoint struct {
	At  time.Time
	MB  float64
	PID int32
}

type memTrack struct {
	Name   string
	Points []memPoint
}

type memoryWatch struct {
	mu     sync.Mutex
	tracks map[string]*memTrack
	cards  []MemoryWatchCard
}

func newMemoryWatch() *memoryWatch {
	return &memoryWatch{
		tracks: map[string]*memTrack{},
		cards:  make([]MemoryWatchCard, 0, leakMaxCards),
	}
}

func (a *App) ensureMemWatch() *memoryWatch {
	if a.memWatch == nil {
		a.memWatch = newMemoryWatch()
	}
	return a.memWatch
}

// ObserveMemoryGrowth records working-set samples and classifies sustained growth.
func (a *App) ObserveMemoryGrowth(snaps []ProcessSnap, now time.Time) {
	if len(snaps) == 0 {
		return
	}
	w := a.ensureMemWatch()
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, p := range snaps {
		base := strings.ToLower(filepath.Base(p.Name))
		if base == "" || isSelfOrWebView(base) || isCriticalSystemProc(base) {
			continue
		}
		mb := float64(p.RAMBytes) / (1024 * 1024)
		if mb < 50 {
			continue
		}
		tr := w.tracks[base]
		if tr == nil {
			tr = &memTrack{Name: p.Name}
			w.tracks[base] = tr
		}
		tr.Name = p.Name
		if len(tr.Points) > 0 {
			last := tr.Points[len(tr.Points)-1]
			if last.PID != 0 && p.PID != 0 && last.PID != p.PID && now.Sub(last.At) > 2*time.Minute {
				tr.Points = nil
			}
		}
		tr.Points = append(tr.Points, memPoint{At: now, MB: mb, PID: p.PID})
		cut := now.Add(-leakWindow)
		i := 0
		for i < len(tr.Points) && tr.Points[i].At.Before(cut) {
			i++
		}
		if i > 0 {
			tr.Points = append([]memPoint(nil), tr.Points[i:]...)
		}
		if len(tr.Points) > 48 {
			tr.Points = tr.Points[len(tr.Points)-48:]
		}

		card := classifyMemoryTrack(tr, now)
		if card.Verdict == MemWatchNeedData || card.Verdict == MemWatchNormal {
			continue
		}
		w.upsertCardLocked(card)
	}

	if len(w.tracks) > leakMaxTracks {
		type kv struct {
			k string
			t time.Time
		}
		arr := make([]kv, 0, len(w.tracks))
		for k, tr := range w.tracks {
			t := time.Time{}
			if n := len(tr.Points); n > 0 {
				t = tr.Points[n-1].At
			}
			arr = append(arr, kv{k, t})
		}
		sort.Slice(arr, func(i, j int) bool { return arr[i].t.Before(arr[j].t) })
		for i := 0; i < len(arr)-leakMaxTracks; i++ {
			delete(w.tracks, arr[i].k)
		}
	}
}

func (w *memoryWatch) upsertCardLocked(card MemoryWatchCard) {
	filtered := make([]MemoryWatchCard, 0, len(w.cards)+1)
	for _, c := range w.cards {
		if strings.EqualFold(c.Process, card.Process) {
			continue
		}
		filtered = append(filtered, c)
	}
	w.cards = append([]MemoryWatchCard{card}, filtered...)
	if len(w.cards) > leakMaxCards {
		w.cards = w.cards[:leakMaxCards]
	}
}

// classifyMemoryTrack is a pure classifier for unit tests.
func classifyMemoryTrack(tr *memTrack, now time.Time) MemoryWatchCard {
	card := MemoryWatchCard{
		Process:  tr.Name,
		Friendly: friendlyProcessName(tr.Name),
		Verdict:  MemWatchNeedData,
	}
	if tr == nil || len(tr.Points) < leakMinPoints {
		card.TextRu = "Недостаточно данных для оценки роста памяти."
		card.TextEn = "Not enough data to judge memory growth."
		return card
	}
	first := tr.Points[0]
	last := tr.Points[len(tr.Points)-1]
	mins := last.At.Sub(first.At).Minutes()
	if mins < 8 {
		card.TextRu = "Недостаточно данных для оценки роста памяти."
		card.TextEn = "Not enough data to judge memory growth."
		return card
	}
	card.FromMB = first.MB
	card.ToMB = last.MB
	card.Minutes = mins
	card.PID = last.PID
	card.DetectedAt = now.Format("15:04:05")

	// Check monotonic-ish growth: majority of steps non-decreasing
	up := 0
	down := 0
	for i := 1; i < len(tr.Points); i++ {
		if tr.Points[i].MB >= tr.Points[i-1].MB*0.98 {
			up++
		}
		if tr.Points[i].MB < tr.Points[i-1].MB*0.92 {
			down++
		}
	}
	grew := last.MB >= first.MB*leakGrowRatio && (last.MB-first.MB) >= leakMinAbsMB
	if !grew {
		card.Verdict = MemWatchNormal
		card.TextRu = "Нормальное изменение потребления памяти."
		card.TextEn = "Normal memory usage change."
		card.Confidence = 0.55
		return card
	}
	if down > up/2 {
		card.Verdict = MemWatchNormal
		card.TextRu = "Память росла, но затем снижалась - похоже на нормальную работу."
		card.TextEn = "Memory rose then fell - looks like normal behaviour."
		card.Confidence = 0.5
		return card
	}

	ratio := last.MB / first.MB
	if ratio >= leakPossibleRatio && mins >= 20 && down == 0 {
		card.Verdict = MemWatchPossible
		card.Confidence = 0.72
		card.TextRu = fmt.Sprintf("%s увеличил потребление памяти с %.1f до %.1f ГБ за %.0f мин - возможная утечка.",
			card.Friendly, first.MB/1024, last.MB/1024, mins)
		card.TextEn = fmt.Sprintf("%s grew from %.1f to %.1f GB over %.0f min - possible leak.",
			card.Friendly, first.MB/1024, last.MB/1024, mins)
		return card
	}

	card.Verdict = MemWatchGrowth
	card.Confidence = 0.65
	card.TextRu = fmt.Sprintf("%s: устойчивый рост памяти с %.0f до %.0f МБ за %.0f мин.",
		card.Friendly, first.MB, last.MB, mins)
	card.TextEn = fmt.Sprintf("%s: sustained memory growth from %.0f to %.0f MB over %.0f min.",
		card.Friendly, first.MB, last.MB, mins)
	return card
}

// GetMemoryWatchCards returns current Memory Leak Watch cards.
func (a *App) GetMemoryWatchCards() []MemoryWatchCard {
	w := a.ensureMemWatch()
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]MemoryWatchCard, len(w.cards))
	copy(out, w.cards)
	return out
}
