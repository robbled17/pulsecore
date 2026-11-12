package main

import (
	"strings"
	"testing"
	"time"
)

func snapWith(mut func(*Snapshot)) Snapshot {
	s := Snapshot{
		Timestamp:      time.Now(),
		CPUPercent:     40,
		RAMPercent:     60,
		RAMTotalMB:     16384,
		RAMAvailableMB: 6000,
		HasLoadSign:    true,
		CPUFreqMHz:     3600,
		CPUTempC:       55,
	}
	if mut != nil {
		mut(&s)
	}
	return s
}

func TestVerdictCPUHog(t *testing.T) {
	s := snapWith(func(s *Snapshot) {
		s.TopCPU = []ProcessSnap{{Name: "chrome.exe", PID: 100, CPUPercent: 85}}
	})
	v := EvaluateVerdict([]Snapshot{s}, IncidentHints{HasLoadSign: true, DurationMs: 200})
	if v.Type != VerdictCPUHog {
		t.Fatalf("got %s want CPU_HOG", v.Type)
	}
	if v.Action != "eco" {
		t.Fatalf("action=%s want eco", v.Action)
	}
	if v.Culprit != "chrome.exe" {
		t.Fatalf("culprit=%s", v.Culprit)
	}
}

func TestVerdictDiskHog(t *testing.T) {
	s := snapWith(func(s *Snapshot) {
		s.DiskBusyPct = 100
		s.TopCPU = []ProcessSnap{{Name: "idle.exe", PID: 1, CPUPercent: 10}}
		s.TopDisk = []ProcessSnap{
			{Name: "Setup.exe", PID: 200, DiskMBps: 80},
			{Name: "other.exe", PID: 201, DiskMBps: 10},
		}
	})
	v := EvaluateVerdict([]Snapshot{s}, IncidentHints{HasLoadSign: true})
	if v.Type != VerdictDiskHog {
		t.Fatalf("got %s want DISK_HOG", v.Type)
	}
}

func TestVerdictMemoryPressure(t *testing.T) {
	s := snapWith(func(s *Snapshot) {
		s.RAMPercent = 96
		s.RAMAvailableMB = 200
		s.TopRAM = []ProcessSnap{{Name: "firefox.exe", PID: 300, RAMBytes: 4 << 30}}
		s.TopCPU = []ProcessSnap{{Name: "firefox.exe", PID: 300, CPUPercent: 5}}
	})
	v := EvaluateVerdict([]Snapshot{s}, IncidentHints{HasLoadSign: true})
	if v.Type != VerdictMemoryPressure {
		t.Fatalf("got %s want MEMORY_PRESSURE", v.Type)
	}
	if v.Culprit != "firefox.exe" {
		t.Fatalf("culprit=%s", v.Culprit)
	}
}

func TestVerdictThermalThrottle(t *testing.T) {
	s := snapWith(func(s *Snapshot) {
		s.CPUTempC = 90
		s.CPUFreqMHz = 2800
		s.TopCPU = []ProcessSnap{{Name: "game.exe", PID: 400, CPUPercent: 20}}
	})
	v := EvaluateVerdict([]Snapshot{s}, IncidentHints{HasLoadSign: true, PeakCPUFreq: 4000})
	if v.Type != VerdictThermalThrottle {
		t.Fatalf("got %s want THERMAL_THROTTLE", v.Type)
	}
	if v.Culprit != "CPU" {
		t.Fatalf("culprit should be CPU without strong proc evidence, got %s", v.Culprit)
	}
}

func TestVerdictUnknown(t *testing.T) {
	s := snapWith(func(s *Snapshot) {
		s.CPUPercent = 45
		s.DiskBusyPct = 20
		s.TopCPU = []ProcessSnap{{Name: "app.exe", PID: 1, CPUPercent: 30}}
	})
	v := EvaluateVerdict([]Snapshot{s}, IncidentHints{HasLoadSign: true, DurationMs: 180})
	if v.Type != VerdictUnknown {
		t.Fatalf("got %s want UNKNOWN", v.Type)
	}
	if v.Culprit != "" {
		t.Fatalf("UNKNOWN must not invent culprit, got %s", v.Culprit)
	}
	if v.Action != "none" {
		t.Fatalf("action=%s", v.Action)
	}
}

func TestVerdictCPUPriorityOverDisk(t *testing.T) {
	s := snapWith(func(s *Snapshot) {
		s.DiskBusyPct = 100
		s.TopCPU = []ProcessSnap{{Name: "render.exe", PID: 10, CPUPercent: 90}}
		s.TopDisk = []ProcessSnap{{Name: "disk.exe", PID: 11, DiskMBps: 100}}
	})
	v := EvaluateVerdict([]Snapshot{s}, IncidentHints{HasLoadSign: true})
	if v.Type != VerdictCPUHog {
		t.Fatalf("CPU must win priority, got %s", v.Type)
	}
}

func TestVerdictNoLoadSign(t *testing.T) {
	s := snapWith(func(s *Snapshot) {
		s.HasLoadSign = false
		s.CPUPercent = 5
		s.DiskBusyPct = 0
		s.RAMPercent = 40
	})
	v := EvaluateVerdict([]Snapshot{s}, IncidentHints{HasLoadSign: false, IgnoreLoad: true})
	if v.Type != VerdictUnknown {
		t.Fatalf("got %s", v.Type)
	}
	if len(v.EvidenceRaw) == 0 || v.EvidenceRaw[0] != "no_load_sign" {
		t.Fatalf("evidenceRaw=%v", v.EvidenceRaw)
	}
	for _, e := range v.Evidence {
		if e == "no_load_sign" {
			t.Fatalf("raw label leaked into human evidence: %v", v.Evidence)
		}
	}
}

func TestVerdictCPUSaturation(t *testing.T) {
	s := snapWith(func(s *Snapshot) {
		s.CPUPercent = 79.8
		s.DiskBusyPct = 1
		s.TopCPU = []ProcessSnap{
			{Name: "chrome.exe", PID: 11, CPUPercent: 28},
			{Name: "code.exe", PID: 12, CPUPercent: 22},
			{Name: "discord.exe", PID: 13, CPUPercent: 18},
		}
	})
	v := EvaluateVerdict([]Snapshot{s}, IncidentHints{HasLoadSign: true, DurationMs: 200})
	if v.Type != VerdictCPUSaturation {
		t.Fatalf("got %s want CPU_SATURATION", v.Type)
	}
	if len(v.EcoTargets) < 3 {
		t.Fatalf("eco targets=%d want >=3", len(v.EcoTargets))
	}
	if v.Action != "eco" {
		t.Fatalf("action=%s", v.Action)
	}
	for _, e := range v.Evidence {
		if strings.Contains(e, "no_matching_rule") || strings.Contains(e, "disk_busy=") || strings.Contains(e, "cpu=") {
			t.Fatalf("raw leaked into human evidence: %q", e)
		}
	}
	if len(v.EvidenceRaw) == 0 {
		t.Fatal("expected raw evidence under details")
	}
}

func TestVerdictCPUSaturationDoesNotOverrideHigher(t *testing.T) {
	// CPU_HOG wins
	s1 := snapWith(func(s *Snapshot) {
		s.CPUPercent = 90
		s.TopCPU = []ProcessSnap{{Name: "render.exe", PID: 1, CPUPercent: 88}}
	})
	if v := EvaluateVerdict([]Snapshot{s1}, IncidentHints{HasLoadSign: true}); v.Type != VerdictCPUHog {
		t.Fatalf("CPU_HOG override failed: %s", v.Type)
	}
	// DISK_HOG wins over saturation
	s2 := snapWith(func(s *Snapshot) {
		s.CPUPercent = 75
		s.DiskBusyPct = 100
		s.TopCPU = []ProcessSnap{{Name: "a.exe", PID: 1, CPUPercent: 20}}
		s.TopDisk = []ProcessSnap{{Name: "setup.exe", PID: 2, DiskMBps: 90}}
	})
	if v := EvaluateVerdict([]Snapshot{s2}, IncidentHints{HasLoadSign: true}); v.Type != VerdictDiskHog {
		t.Fatalf("DISK_HOG override failed: %s", v.Type)
	}
	// MEMORY wins over saturation
	s3 := snapWith(func(s *Snapshot) {
		s.CPUPercent = 78
		s.RAMPercent = 97
		s.RAMAvailableMB = 100
		s.TopCPU = []ProcessSnap{{Name: "a.exe", PID: 1, CPUPercent: 25}}
		s.TopRAM = []ProcessSnap{{Name: "mem.exe", PID: 3, RAMBytes: 8 << 30}}
	})
	if v := EvaluateVerdict([]Snapshot{s3}, IncidentHints{HasLoadSign: true}); v.Type != VerdictMemoryPressure {
		t.Fatalf("MEMORY override failed: %s", v.Type)
	}
}

func TestVerdictSystemProcNoAction(t *testing.T) {
	s := snapWith(func(s *Snapshot) {
		s.TopCPU = []ProcessSnap{{Name: "csrss.exe", PID: 500, CPUPercent: 88}}
	})
	v := EvaluateVerdict([]Snapshot{s}, IncidentHints{HasLoadSign: true})
	if v.Type != VerdictCPUHog {
		t.Fatalf("got %s", v.Type)
	}
	if !v.SystemProc {
		t.Fatal("expected systemProc")
	}
	if v.Action != "none" {
		t.Fatalf("system process must not get eco action, got %s", v.Action)
	}
}

func TestVerdictMissingSensors(t *testing.T) {
	s := snapWith(func(s *Snapshot) {
		s.CPUTempC = 0
		s.CPUFreqMHz = 0
		s.GPUTempC = 0
		s.DiskQueue = 0
		s.TopCPU = []ProcessSnap{{Name: "app.exe", PID: 1, CPUPercent: 10}}
	})
	v := EvaluateVerdict([]Snapshot{s}, IncidentHints{HasLoadSign: true, PeakCPUFreq: 0})
	if v.Type != VerdictUnknown {
		t.Fatalf("missing sensors must not invent THERMAL, got %s", v.Type)
	}
}

func TestVerdictIncompleteSnapshots(t *testing.T) {
	v := EvaluateVerdict(nil, IncidentHints{HasLoadSign: true, DurationMs: 200})
	if v.Type != VerdictUnknown {
		t.Fatalf("got %s", v.Type)
	}
	v2 := EvaluateVerdict([]Snapshot{{}}, IncidentHints{HasLoadSign: false})
	if v2.Type != VerdictUnknown {
		t.Fatalf("got %s", v2.Type)
	}
}

func TestIdleNoFalseIncidentsHeuristic(t *testing.T) {
	// Pure engine: idle snapshots without load → no_load_sign UNKNOWN, never CPU_HOG.
	for i := 0; i < 100; i++ {
		s := Snapshot{
			Timestamp:   time.Now(),
			CPUPercent:  3,
			RAMPercent:  45,
			HasLoadSign: false,
		}
		v := EvaluateVerdict([]Snapshot{s}, IncidentHints{HasLoadSign: false, IgnoreLoad: true})
		if v.Type != VerdictUnknown {
			t.Fatalf("iter %d: got %s", i, v.Type)
		}
		if v.Culprit != "" {
			t.Fatalf("iter %d: invented culprit %s", i, v.Culprit)
		}
	}
}
