// Package store provides durable local JSON persistence under the OS app-data dir.
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const appDirName = "PulseCore"

// DayBucket aggregates one calendar day of local metrics.
type DayBucket struct {
	Date          string  `json:"date"` // YYYY-MM-DD
	Samples       int     `json:"samples"`
	CPUAvg        float64 `json:"cpuAvg"`
	RAMAvg        float64 `json:"ramAvg"`
	GPUAvg        float64 `json:"gpuAvg"`
	LagCount      int     `json:"lagCount"`
	EcoToggles    int     `json:"ecoToggles"`
	TopProcess    string  `json:"topProcess"`
	TopProcessCPU float64 `json:"topProcessCPU"`
}

// StatsFile is the on-disk weekly/local summary store.
type StatsFile struct {
	Version int         `json:"version"`
	Days    []DayBucket `json:"days"`
}

// StatsManager loads/saves stats.json under LocalAppData/PulseCore (or ~/.local/share).
type StatsManager struct {
	mu   sync.Mutex
	path string
	data StatsFile
}

// DefaultPath resolves the stats.json path for the current OS user.
func DefaultPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(base, ".pulsecore")
	} else {
		base = filepath.Join(base, appDirName)
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(base, "stats.json"), nil
}

// OpenStats loads or creates the stats file.
func OpenStats(path string) (*StatsManager, error) {
	m := &StatsManager{path: path, data: StatsFile{Version: 1, Days: []DayBucket{}}}
	b, err := os.ReadFile(path)
	if err == nil && len(b) > 0 {
		_ = json.Unmarshal(b, &m.data)
		if m.data.Version == 0 {
			m.data.Version = 1
		}
	}
	return m, nil
}

// RecordSample merges a live tick into today's bucket (running averages).
func (m *StatsManager) RecordSample(cpu, ram, gpu float64, topProc string, topCPU float64, lagHit bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	day := time.Now().Format("2006-01-02")
	idx := -1
	for i := range m.data.Days {
		if m.data.Days[i].Date == day {
			idx = i
			break
		}
	}
	if idx < 0 {
		m.data.Days = append(m.data.Days, DayBucket{Date: day})
		idx = len(m.data.Days) - 1
		// Keep ~35 days.
		if len(m.data.Days) > 35 {
			m.data.Days = m.data.Days[len(m.data.Days)-35:]
			idx = len(m.data.Days) - 1
		}
	}
	b := &m.data.Days[idx]
	n := float64(b.Samples)
	b.CPUAvg = (b.CPUAvg*n + cpu) / (n + 1)
	b.RAMAvg = (b.RAMAvg*n + ram) / (n + 1)
	b.GPUAvg = (b.GPUAvg*n + gpu) / (n + 1)
	b.Samples++
	if lagHit {
		b.LagCount++
	}
	if topCPU > b.TopProcessCPU {
		b.TopProcessCPU = topCPU
		b.TopProcess = topProc
	}
}

// IncEcoToggle increments today's eco toggle counter.
func (m *StatsManager) IncEcoToggle() {
	m.mu.Lock()
	defer m.mu.Unlock()
	day := time.Now().Format("2006-01-02")
	for i := range m.data.Days {
		if m.data.Days[i].Date == day {
			m.data.Days[i].EcoToggles++
			return
		}
	}
	m.data.Days = append(m.data.Days, DayBucket{Date: day, EcoToggles: 1})
}

// LastNDays returns up to n most recent day buckets (oldest→newest).
func (m *StatsManager) LastNDays(n int) []DayBucket {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n <= 0 || len(m.data.Days) == 0 {
		return nil
	}
	if len(m.data.Days) <= n {
		return append([]DayBucket(nil), m.data.Days...)
	}
	return append([]DayBucket(nil), m.data.Days[len(m.data.Days)-n:]...)
}

// Save writes stats.json atomically-ish.
func (m *StatsManager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}

// WeeklySummary aggregates the last 7 day buckets.
type WeeklySummary struct {
	OK           bool        `json:"ok"`
	PeriodDays   int         `json:"periodDays"`
	Samples      int         `json:"samples"`
	CPUAvg       float64     `json:"cpuAvg"`
	RAMAvg       float64     `json:"ramAvg"`
	GPUAvg       float64     `json:"gpuAvg"`
	LagCount     int         `json:"lagCount"`
	EcoToggles   int         `json:"ecoToggles"`
	TopProcess   string      `json:"topProcess"`
	Days         []DayBucket `json:"days"`
	NoteKey      string      `json:"noteKey"`
	Source       string      `json:"source"` // local | powercfg | mixed
}

// BuildWeeklySummary rolls up last 7 days.
func BuildWeeklySummary(days []DayBucket) WeeklySummary {
	s := WeeklySummary{OK: true, PeriodDays: 7, Days: days, Source: "local", NoteKey: "weeklyLocalNote"}
	if len(days) == 0 {
		s.NoteKey = "weeklyEmptyLocal"
		return s
	}
	var topCPU float64
	for _, d := range days {
		s.Samples += d.Samples
		s.LagCount += d.LagCount
		s.EcoToggles += d.EcoToggles
		s.CPUAvg += d.CPUAvg
		s.RAMAvg += d.RAMAvg
		s.GPUAvg += d.GPUAvg
		if d.TopProcessCPU >= topCPU && d.TopProcess != "" {
			topCPU = d.TopProcessCPU
			s.TopProcess = d.TopProcess
		}
	}
	n := float64(len(days))
	s.CPUAvg /= n
	s.RAMAvg /= n
	s.GPUAvg /= n
	return s
}
