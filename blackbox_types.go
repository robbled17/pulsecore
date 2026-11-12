package main

import "time"

// Snapshot is a point-in-time system state for the Lag Black Box.
// Optional sensors use 0 / false and Must not be treated as proven zeros in UI.
type Snapshot struct {
	Timestamp      time.Time     `json:"timestamp"`
	CPUPercent     float64       `json:"cpuPercent"`
	CPUPerCore     []float64     `json:"cpuPerCore"`
	CPUFreqMHz     float64       `json:"cpuFreqMHz"`     // 0 = unavailable
	CPUTempC       float64       `json:"cpuTempC"`       // 0 = unavailable
	GPUPercent     float64       `json:"gpuPercent"`
	GPUTempC       float64       `json:"gpuTempC"`       // 0 = unavailable
	RAMUsedMB      uint64        `json:"ramUsedMB"`
	RAMTotalMB     uint64        `json:"ramTotalMB"`
	RAMAvailableMB uint64        `json:"ramAvailableMB"`
	RAMPercent     float64       `json:"ramPercent"`
	SwapPercent    float64       `json:"swapPercent"`
	SwapUsedMB     uint64        `json:"swapUsedMB"`
	DiskBusyPct    float64       `json:"diskBusyPct"`    // physical activity heuristic, not process I/O
	DiskQueue      float64       `json:"diskQueue"`      // 0 = unavailable
	DiskReadMBps   float64       `json:"diskReadMBps"`   // physical disk counters delta
	DiskWriteMBps  float64       `json:"diskWriteMBps"`
	TopCPU         []ProcessSnap `json:"topCPU"`
	TopDisk        []ProcessSnap `json:"topDisk"`
	TopRAM         []ProcessSnap `json:"topRAM"`
	AppActive      bool          `json:"appActive"`
	GameLoad       bool          `json:"gameLoad"`
	HasLoadSign    bool          `json:"hasLoadSign"`
}

// ProcessSnap is one process (or EXE group) inside a snapshot.
type ProcessSnap struct {
	Name          string  `json:"name"`
	PID           int32   `json:"pid"`
	PIDs          []int32 `json:"pids,omitempty"`
	CPUPercent    float64 `json:"cpuPercent"` // normalized system share 0-100
	RAMBytes      uint64  `json:"ramBytes"`
	DiskMBps      float64 `json:"diskMBps"` // process I/O rate (not physical disk guarantee)
	DiskReadMBps  float64 `json:"diskReadMBps"`
	DiskWriteMBps float64 `json:"diskWriteMBps"`
}

// VerdictType classifies a diagnostic outcome.
type VerdictType string

const (
	VerdictCPUHog          VerdictType = "CPU_HOG"
	VerdictDiskHog         VerdictType = "DISK_HOG"
	VerdictMemoryPressure  VerdictType = "MEMORY_PRESSURE"
	VerdictCPUSaturation   VerdictType = "CPU_SATURATION"
	VerdictThermalThrottle VerdictType = "THERMAL_THROTTLE"
	VerdictUnknown         VerdictType = "UNKNOWN"
)

// EcoTarget is a process that may safely receive Eco Mode from an incident card.
type EcoTarget struct {
	Name       string  `json:"name"`
	PID        int32   `json:"pid"`
	CPUPercent float64 `json:"cpuPercent"`
}

// Verdict is a pure diagnosis result (no side effects).
type Verdict struct {
	Type           VerdictType `json:"type"`
	Culprit        string      `json:"culprit"`
	CulpritPID     int32       `json:"culpritPID"`
	Severity       string      `json:"severity"` // low|medium|high|critical
	Confidence     float64     `json:"confidence"` // 0-1
	TextRu         string      `json:"textRu"`
	TextEn         string      `json:"textEn"`
	Action         string      `json:"action"` // eco|none
	Evidence       []string    `json:"evidence"`       // human-readable (UI primary)
	EvidenceRaw    []string    `json:"evidenceRaw"`    // technical labels under «Подробнее»
	EcoTargets     []EcoTarget `json:"ecoTargets"`
	SystemProc     bool        `json:"systemProc"`
}

// StallKind distinguishes scheduler delay from confirmed performance problems.
type StallKind string

const (
	StallPulseCoreSchedule StallKind = "pulsecore_schedule"
	StallLikelySystem      StallKind = "likely_system"
	StallConfirmed         StallKind = "confirmed"
)

// LagIncident is one merged Black Box event with verdict + timeline hooks.
type LagIncident struct {
	ID         string     `json:"id"`
	Start      time.Time  `json:"start"`
	End        time.Time  `json:"end"`
	DurationMs float64    `json:"durationMs"`
	Kind       StallKind  `json:"kind"`
	Synthetic  bool       `json:"synthetic"`
	Verdict    Verdict    `json:"verdict"`
	Evidence   []string   `json:"evidence"`
	Before     []Snapshot `json:"before,omitempty"`
	During     []Snapshot `json:"during,omitempty"`
	After      []Snapshot `json:"after,omitempty"`
}

// IncidentCard is the compact UI payload for the Lag tab.
type IncidentCard struct {
	ID           string      `json:"id"`
	Time         string      `json:"time"`
	DurationMs   float64     `json:"durationMs"`
	Type         string      `json:"type"`
	VerdictText  string      `json:"verdictText"`
	Culprit      string      `json:"culprit"`
	CulpritPID   int32       `json:"culpritPID"`
	Confidence   float64     `json:"confidence"`
	Evidence     []string    `json:"evidence"`
	EvidenceRaw  []string    `json:"evidenceRaw"`
	EcoTargets   []EcoTarget `json:"ecoTargets"`
	CanEco       bool        `json:"canEco"`
	SystemProc   bool        `json:"systemProc"`
	Synthetic    bool        `json:"synthetic"`
	Severity     string      `json:"severity"`
}
