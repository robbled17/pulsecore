package diag

// HealthInput aggregates subsystem signals for a unified score.
type HealthInput struct {
	CPUPercent     float64
	RAMPercent     float64
	TempC          float64
	DiskPercentMax float64
	BatteryWear    float64 // 0 if N/A
	BatteryAvail   bool
	LagCountRecent int
	SleepBlocked   bool
	DiskUnhealthy  bool
}

// HealthReport is the Stage 7 System Health payload.
type HealthReport struct {
	Score      int               `json:"score"` // 0..100
	Level      string            `json:"level"` // good | fair | poor | critical
	SummaryKey string            `json:"summaryKey"`
	Factors    []HealthFactor    `json:"factors"`
	Tips       []string          `json:"tips"` // i18n keys
}

// HealthFactor explains one score component.
type HealthFactor struct {
	Key    string  `json:"key"`
	Impact int     `json:"impact"` // negative points applied
	Value  float64 `json:"value"`
}

// ComputeHealthScore builds a 0–100 health score with transparent factors.
func ComputeHealthScore(in HealthInput) HealthReport {
	score := 100
	factors := make([]HealthFactor, 0, 8)
	tips := make([]string, 0, 4)

	add := func(key string, impact int, val float64, tip string) {
		if impact <= 0 {
			return
		}
		score -= impact
		factors = append(factors, HealthFactor{Key: key, Impact: impact, Value: val})
		if tip != "" {
			tips = append(tips, tip)
		}
	}

	if in.CPUPercent >= 95 {
		add("healthCPU", 15, in.CPUPercent, "tipHealthCPU")
	} else if in.CPUPercent >= 85 {
		add("healthCPU", 8, in.CPUPercent, "tipHealthCPU")
	}
	if in.RAMPercent >= 92 {
		add("healthRAM", 18, in.RAMPercent, "tipHealthRAM")
	} else if in.RAMPercent >= 80 {
		add("healthRAM", 8, in.RAMPercent, "tipHealthRAM")
	}
	if in.TempC >= 90 {
		add("healthTemp", 20, in.TempC, "tipHealthTemp")
	} else if in.TempC >= 80 {
		add("healthTemp", 10, in.TempC, "tipHealthTemp")
	}
	if in.DiskPercentMax >= 95 {
		add("healthDisk", 15, in.DiskPercentMax, "tipHealthDisk")
	} else if in.DiskPercentMax >= 85 {
		add("healthDisk", 7, in.DiskPercentMax, "tipHealthDisk")
	}
	if in.DiskUnhealthy {
		add("healthSMART", 25, 1, "tipHealthSMART")
	}
	if in.BatteryAvail && in.BatteryWear >= 30 {
		add("healthBattery", 10, in.BatteryWear, "tipHealthBattery")
	}
	if in.LagCountRecent >= 5 {
		add("healthLags", 12, float64(in.LagCountRecent), "tipHealthLags")
	} else if in.LagCountRecent >= 2 {
		add("healthLags", 5, float64(in.LagCountRecent), "")
	}
	if in.SleepBlocked {
		add("healthSleep", 6, 1, "tipHealthSleep")
	}

	if score < 0 {
		score = 0
	}
	level := "good"
	summary := "healthGood"
	switch {
	case score < 40:
		level = "critical"
		summary = "healthCritical"
	case score < 60:
		level = "poor"
		summary = "healthPoor"
	case score < 80:
		level = "fair"
		summary = "healthFair"
	}
	return HealthReport{Score: score, Level: level, SummaryKey: summary, Factors: factors, Tips: tips}
}
