package diag

// EcoSample is a point-in-time metrics sample around an Eco Mode toggle.
type EcoSample struct {
	CPUPercent  float64 `json:"cpuPercent"`
	RAMPercent  float64 `json:"ramPercent"`
	GPUPercent  float64 `json:"gpuPercent"`
	ProcessCPU  float64 `json:"processCPU"`
	ProcessRSS  float64 `json:"processRSSMB"`
	AtUnixMs    int64   `json:"atUnixMs"`
}

// EcoDelta compares before/after samples for UI feedback.
type EcoDelta struct {
	PID           int32     `json:"pid"`
	Name          string    `json:"name"`
	Enabled       bool      `json:"enabled"`
	Before        EcoSample `json:"before"`
	After         EcoSample `json:"after"`
	CPUDeltaPct   float64   `json:"cpuDeltaPct"`
	ProcCPUDelta  float64   `json:"procCPUDelta"`
	RAMDeltaPct   float64   `json:"ramDeltaPct"`
	Improved      bool      `json:"improved"`
	SummaryKey    string    `json:"summaryKey"`
}

// ComputeEcoDelta builds a delta. After may be zeroed if not yet sampled.
func ComputeEcoDelta(pid int32, name string, enabled bool, before, after EcoSample) EcoDelta {
	d := EcoDelta{
		PID:     pid,
		Name:    name,
		Enabled: enabled,
		Before:  before,
		After:   after,
	}
	if after.AtUnixMs == 0 {
		d.SummaryKey = "ecoDeltaPending"
		return d
	}
	d.CPUDeltaPct = after.CPUPercent - before.CPUPercent
	d.ProcCPUDelta = after.ProcessCPU - before.ProcessCPU
	d.RAMDeltaPct = after.RAMPercent - before.RAMPercent
	if enabled {
		// Improvement = process or system CPU dropped.
		d.Improved = d.ProcCPUDelta < -0.5 || d.CPUDeltaPct < -1.0
		if d.Improved {
			d.SummaryKey = "ecoDeltaImproved"
		} else {
			d.SummaryKey = "ecoDeltaNeutral"
		}
	} else {
		d.SummaryKey = "ecoDeltaDisabled"
	}
	return d
}
