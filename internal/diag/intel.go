package diag

// Insight is a local, offline intelligence suggestion.
type Insight struct {
	ID         string  `json:"id"`
	Severity   string  `json:"severity"` // info | warn | crit
	TitleKey   string  `json:"titleKey"`
	DetailKey  string  `json:"detailKey"`
	ActionKey  string  `json:"actionKey"`
	Confidence float64 `json:"confidence"`
}

// IntelInput feeds the local rules engine (no network).
type IntelInput struct {
	CPUAvg7d       float64
	RAMAvg7d       float64
	LagCount7d     int
	EcoToggles7d   int
	HealthScore    int
	LeakSuspects   int
	StartupHeavy   int
	GameModeActive bool
	BatteryWear    float64
	BatteryAvail   bool
}

// EvaluateIntelligence runs deterministic local profiles.
func EvaluateIntelligence(in IntelInput) []Insight {
	out := make([]Insight, 0, 8)
	add := func(id, sev, title, detail, action string, conf float64) {
		out = append(out, Insight{
			ID: id, Severity: sev, TitleKey: title, DetailKey: detail,
			ActionKey: action, Confidence: conf,
		})
	}
	if in.RAMAvg7d >= 85 {
		add("ram-pressure", "warn", "intelRAMTitle", "intelRAMDetail", "intelRAMAction", 0.8)
	}
	if in.LagCount7d >= 10 {
		add("lag-week", "warn", "intelLagTitle", "intelLagDetail", "intelLagAction", 0.75)
	}
	if in.LeakSuspects > 0 {
		add("leak", "crit", "intelLeakTitle", "intelLeakDetail", "intelLeakAction", 0.7)
	}
	if in.StartupHeavy >= 3 {
		add("startup", "info", "intelStartupTitle", "intelStartupDetail", "intelStartupAction", 0.65)
	}
	if in.HealthScore > 0 && in.HealthScore < 60 {
		add("health", "warn", "intelHealthTitle", "intelHealthDetail", "intelHealthAction", 0.7)
	}
	if in.BatteryAvail && in.BatteryWear >= 25 {
		add("battery", "info", "intelBatteryTitle", "intelBatteryDetail", "intelBatteryAction", 0.6)
	}
	if in.EcoToggles7d == 0 && in.CPUAvg7d >= 50 {
		add("eco-hint", "info", "intelEcoTitle", "intelEcoDetail", "intelEcoAction", 0.55)
	}
	if in.GameModeActive {
		add("game-on", "info", "intelGameTitle", "intelGameDetail", "intelGameAction", 0.9)
	}
	if len(out) == 0 {
		add("all-good", "info", "intelOKTitle", "intelOKDetail", "intelOKAction", 0.5)
	}
	return out
}
