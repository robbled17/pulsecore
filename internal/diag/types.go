// Package diag holds pure diagnostic logic (ring buffer, verdicts, eco deltas,
// health scoring) with no Windows/Wails imports so Linux CI can run go test.
package diag

import "time"

// RingCapacity is ~2 minutes at 1s poll (master-spec Snapshot / Black Box).
const RingCapacity = 120

// MaxIncidents keeps recent lag incidents in memory (UI ring).
const MaxIncidents = 20

// ProcessSample is a lightweight top-N process sample for attribution.
type ProcessSample struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpuPercent"`
	RSSMB      float64 `json:"rssMB"`
	DiskMBps   float64 `json:"diskMBps"`
}

// Snapshot is a compact historical sample fed by the existing metrics pipeline.
type Snapshot struct {
	At            time.Time       `json:"at"`
	CPUPercent    float64         `json:"cpuPercent"`
	GPUPercent    float64         `json:"gpuPercent"`
	RAMPercent    float64         `json:"ramPercent"`
	RAMFreeMB     uint64          `json:"ramFreeMB"`
	SwapPercent   float64         `json:"swapPercent"`
	DiskReadMBps  float64         `json:"diskReadMBps"`
	DiskWriteMBps float64         `json:"diskWriteMBps"`
	TemperatureC  float64         `json:"temperatureC"`
	TopProcesses  []ProcessSample `json:"topProcesses"`
}

// VerdictLevel is the honesty-aware classification for a lag incident.
type VerdictLevel string

const (
	LevelOK      VerdictLevel = "ok"
	LevelWarn    VerdictLevel = "warn"
	LevelCrit    VerdictLevel = "critical"
	LevelUnknown VerdictLevel = "unknown"
)

// CauseKind identifies the primary attributed cause (i18n-friendly key).
type CauseKind string

const (
	CauseRAMPressure CauseKind = "ram_pressure"
	CauseDiskBusy    CauseKind = "disk_busy"
	CauseCPUHot      CauseKind = "cpu_hot"
	CauseGPUHot      CauseKind = "gpu_hot"
	CauseThermal     CauseKind = "thermal"
	CauseProcess     CauseKind = "process"
	CauseUnknown     CauseKind = "unknown"
)

// Verdict is the output of VerdictEngine.
type Verdict struct {
	Level      VerdictLevel `json:"level"`
	Cause      CauseKind    `json:"cause"`
	CauseKey   string       `json:"causeKey"` // i18n key
	Culprit    string       `json:"culprit"`
	Confidence float64      `json:"confidence"` // 0..1
	SummaryKey string       `json:"summaryKey"`
	DetailKey  string       `json:"detailKey"`
}

// TimelinePoint is a pre/post sample for the lag timeline UI.
type TimelinePoint struct {
	OffsetSec     float64 `json:"offsetSec"`
	CPUPercent    float64 `json:"cpuPercent"`
	GPUPercent    float64 `json:"gpuPercent"`
	RAMPercent    float64 `json:"ramPercent"`
	DiskReadMBps  float64 `json:"diskReadMBps"`
	DiskWriteMBps float64 `json:"diskWriteMBps"`
}

// Incident is a durable Black Box lag record.
type Incident struct {
	ID            string          `json:"id"`
	Timestamp     time.Time       `json:"timestamp"`
	TimestampUI   string          `json:"timestampUI"`
	JitterMs      float64         `json:"jitterMs"`
	Severity      string          `json:"severity"`
	Verdict       Verdict         `json:"verdict"`
	Culprit       string          `json:"culprit"`
	DiskBusy      bool            `json:"diskBusy"`
	FreeRAMMB     uint64          `json:"freeRAMMB"`
	DiskReadMBps  float64         `json:"diskReadMBps"`
	DiskWriteMBps float64         `json:"diskWriteMBps"`
	PreWindow     []TimelinePoint `json:"preWindow"`
	PostWindow    []TimelinePoint `json:"postWindow"`
	Source        string          `json:"source"` // flight | lagspotter | synthetic
}
