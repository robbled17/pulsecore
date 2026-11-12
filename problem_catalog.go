package main

import (
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/robbled17/zeropulse/loc"
)

// HomeProblem is one diagnosed issue with a single primary action.
// Text is built on the frontend from Title/Why/Action/Detail codes + params.
type HomeProblem struct {
	ID           string  `json:"id"`
	Priority     int     `json:"priority"` // lower = more important
	Title        loc.Msg `json:"title"`
	Why          loc.Msg `json:"why"`
	ActionID     string  `json:"actionId"` // free_disk|eco_process|open_startup|power_turbo|disk_advice|none
	Action       loc.Msg `json:"action"`
	ActionArg    string  `json:"actionArg"` // process name, etc.
	GainEstimate int     `json:"gainEstimate"` // effective shown gain (cascaded)
	GainRaw      int     `json:"gainRaw"`      // standalone estimate before cascade
	GainUpTo     bool    `json:"gainUpTo"`     // show «до +N» when capped by prior fixes
	Detail       loc.Msg `json:"detail"`
	StatusLevel  string  `json:"statusLevel"` // good|caution|bad|unknown — color by code
}

// HomeCatalogInput is the pure-function input for problem evaluation (no collectors).
type HomeCatalogInput struct {
	SystemDiskUsed   float64
	SystemDiskOK     bool
	RAMPercent       float64
	RAMOK            bool
	TopRAMName       string
	TopRAMFriendly   string
	TopRAMEcoOK      bool
	LagCulprit       string
	LagCulpritFriend string
	LagCulpritEcoOK  bool
	LagCount         int
	StartupHighCount int
	PowerMode        string // silent|balanced|turbo
	GameActive       bool
	TempCPU          float64
	TempOK           bool
	HealthScore      int
	DiskBad          bool
	DiskCaution      bool
	DiskBadLetter    string
	DiskReason       loc.Msg // SMART reason code+params (bad or caution)
	RefreshHz        int
	RefreshHzMax     int
	RefreshDevice    string
	RefreshName      string
}

// EvaluateHomeProblems returns up to maxN problems sorted by priority. Empty = all good.
func EvaluateHomeProblems(in HomeCatalogInput, maxN int) []HomeProblem {
	if maxN <= 0 {
		maxN = 3
	}
	out := make([]HomeProblem, 0, 8)

	if in.DiskBad && in.DiskReason.Code != "" {
		letter := in.DiskBadLetter
		if letter == "" {
			letter = "C"
		}
		out = append(out, HomeProblem{
			ID: "disk_smart_bad", Priority: 8,
			Title:    loc.M("homeDiskBadTitle", loc.P("letter", letter)),
			Why:      in.DiskReason,
			ActionID: "disk_advice", Action: loc.M("homeActionDiskAdvice", nil),
			ActionArg: letter, GainEstimate: 5,
			Detail: in.DiskReason, StatusLevel: DiskStatusBad,
		})
	} else if in.DiskCaution && in.DiskReason.Code != "" {
		letter := in.DiskBadLetter
		if letter == "" {
			letter = "C"
		}
		out = append(out, HomeProblem{
			ID: "disk_smart_caution", Priority: 7,
			Title:    loc.M("homeDiskCautionTitle", loc.P("letter", letter)),
			Why:      in.DiskReason,
			ActionID: "disk_advice", Action: loc.M("homeActionDiskAdvice", nil),
			ActionArg: letter, GainEstimate: 3,
			Detail: in.DiskReason, StatusLevel: DiskStatusCaution,
		})
	}

	if in.SystemDiskOK && in.SystemDiskUsed >= 85 {
		gain := 8
		if in.SystemDiskUsed >= 90 {
			gain = 12
		}
		if in.SystemDiskUsed >= 95 {
			gain = 16
		}
		pct := math.Round(in.SystemDiskUsed)
		out = append(out, HomeProblem{
			ID: "disk_full", Priority: 10,
			Title:    loc.M("homeDiskFullTitle", loc.P("pct", pct)),
			Why:      loc.M("homeDiskFullWhy", nil),
			ActionID: "free_disk", Action: loc.M("homeActionFreeDisk", nil),
			GainEstimate: gain,
			Detail:       loc.M("homeDiskFullDetail", loc.P("pct", pct)),
		})
	}

	if in.RAMOK && in.RAMPercent >= 85 && in.TopRAMName != "" && in.TopRAMEcoOK {
		friend := in.TopRAMFriendly
		if friend == "" {
			friend = filepath.Base(in.TopRAMName)
		}
		pct := math.Round(in.RAMPercent)
		out = append(out, HomeProblem{
			ID: "ram_pressure", Priority: 20,
			Title:    loc.M("homeRamHighTitle", loc.P("pct", pct)),
			Why:      loc.M("homeRamHighWhy", nil),
			ActionID: "eco_process", Action: loc.M("homeActionLimitProc", loc.P("name", friend)),
			ActionArg: in.TopRAMName, GainEstimate: 7,
			Detail: loc.M("homeRamHighDetail", loc.P("name", friend)),
		})
	}

	if in.LagCount > 0 && in.LagCulprit != "" && in.LagCulpritEcoOK &&
		!strings.EqualFold(in.LagCulprit, "many_processes") &&
		!strings.EqualFold(in.LagCulprit, "unknown") {
		friend := in.LagCulpritFriend
		if friend == "" {
			friend = filepath.Base(in.LagCulprit)
		}
		out = append(out, HomeProblem{
			ID: "lag_culprit", Priority: 15,
			Title:    loc.M("homeLagTitle", loc.P("name", friend)),
			Why:      loc.M("homeLagWhy", nil),
			ActionID: "eco_process", Action: loc.M("homeActionLimitProc", loc.P("name", friend)),
			ActionArg: in.LagCulprit, GainEstimate: 6,
			Detail: loc.M("homeLagDetail", loc.P("count", in.LagCount)),
		})
	}

	if in.StartupHighCount >= 2 {
		out = append(out, HomeProblem{
			ID: "startup_heavy", Priority: 40,
			Title:    loc.M("homeStartupHeavyTitle", loc.P("n", in.StartupHighCount)),
			Why:      loc.M("homeStartupHeavyWhy", nil),
			ActionID: "open_startup", Action: loc.M("homeActionReviewStartup", nil),
			GainEstimate: int(math.Min(10, float64(in.StartupHighCount)*3)),
			Detail:       loc.M("homeStartupHeavyDetail", nil),
		})
	}

	if in.GameActive && (in.PowerMode == "silent" || in.PowerMode == "") {
		out = append(out, HomeProblem{
			ID: "power_silent_game", Priority: 25,
			Title:    loc.M("homeGameSilentTitle", nil),
			Why:      loc.M("homeGameSilentWhy", nil),
			ActionID: "power_turbo", Action: loc.M("homeActionPowerTurbo", nil),
			GainEstimate: 5,
			Detail:       loc.M("homeGameSilentDetail", nil),
		})
	}

	if in.TempOK && in.TempCPU >= 90 {
		temp := math.Round(in.TempCPU)
		out = append(out, HomeProblem{
			ID: "cpu_hot", Priority: 12,
			Title:    loc.M("homeCpuHotTitle", loc.P("temp", temp)),
			Why:      loc.M("homeCpuHotWhy", nil),
			ActionID: "power_silent", Action: loc.M("homeActionPowerSilent", nil),
			GainEstimate: 4,
			Detail:       loc.M("homeCpuHotDetail", nil),
		})
	}

	if in.RefreshHzMax > 0 && in.RefreshHz > 0 && in.RefreshHz < in.RefreshHzMax {
		name := in.RefreshName
		if name == "" {
			name = "Monitor"
		}
		arg := in.RefreshDevice + "|" + strconv.Itoa(in.RefreshHzMax)
		if in.RefreshDevice == "" {
			arg = strconv.Itoa(in.RefreshHzMax)
		}
		out = append(out, HomeProblem{
			ID: "refresh_rate", Priority: 22,
			Title: loc.M("homeRefreshTitle", loc.P("name", name, "hz", in.RefreshHz, "max", in.RefreshHzMax)),
			Why:   loc.M("homeRefreshWhy", loc.P("max", in.RefreshHzMax)),
			ActionID: "set_refresh_rate",
			Action:   loc.M("homeActionSetRefresh", loc.P("max", in.RefreshHzMax)),
			ActionArg: arg, GainEstimate: 5,
			Detail: loc.M("homeRefreshDetail", loc.P("max", in.RefreshHzMax)),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority == out[j].Priority {
			return out[i].ID < out[j].ID
		}
		return out[i].Priority < out[j].Priority
	})
	if len(out) > maxN {
		out = out[:maxN]
	}
	return AssignCascadedGains(out, in.HealthScore)
}

// AssignCascadedGains recalculates +N so each card’s gain is
// (score after this fix given prior fixes) − (score before), and the sum of
// shown gains never exceeds 100 − currentScore. When a card’s standalone
// estimate is reduced by earlier fixes, GainUpTo is set («до +N»).
// If currentScore ≥ 95, gains are cleared (UI hides +N).
func AssignCascadedGains(problems []HomeProblem, currentScore int) []HomeProblem {
	if len(problems) == 0 {
		return problems
	}
	if currentScore >= 95 {
		for i := range problems {
			problems[i].GainRaw = problems[i].GainEstimate
			problems[i].GainEstimate = 0
			problems[i].GainUpTo = false
		}
		return problems
	}
	if currentScore < 0 {
		currentScore = 0
	}
	if currentScore > 100 {
		currentScore = 100
	}
	room := 100 - currentScore
	sim := currentScore
	for i := range problems {
		raw := problems[i].GainEstimate
		if raw < 0 {
			raw = 0
		}
		problems[i].GainRaw = raw
		after := sim + raw
		if after > 100 {
			after = 100
		}
		gain := after - sim
		if gain < 0 {
			gain = 0
		}
		if gain > room {
			gain = room
		}
		problems[i].GainEstimate = gain
		problems[i].GainUpTo = raw > gain && gain > 0
		sim += gain
		if sim > 100 {
			sim = 100
		}
		room = 100 - sim
		if room < 0 {
			room = 0
		}
	}
	return problems
}

// ScoreIfAllFixed returns currentScore + sum(cascaded gains), capped at 100.
func ScoreIfAllFixed(currentScore int, problems []HomeProblem) int {
	if currentScore >= 95 {
		return currentScore
	}
	sum := currentScore
	for _, p := range problems {
		sum += p.GainEstimate
	}
	if sum > 100 {
		return 100
	}
	if sum < 0 {
		return 0
	}
	return sum
}

// HomeStatusWord maps score to a short status message code.
func HomeStatusWord(score int) loc.Msg {
	switch {
	case score >= 85:
		return loc.M("homeStatusExcellent", nil)
	case score >= 60:
		return loc.M("homeStatusImprove", nil)
	default:
		return loc.M("homeStatusAttention", nil)
	}
}
