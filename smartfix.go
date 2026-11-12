package main

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/robbled17/zeropulse/loc"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SmartFixStep is one selectable fix in the plan UI.
type SmartFixStep struct {
	ID           string  `json:"id"`
	ScenarioID   string  `json:"scenarioId"`
	Title        loc.Msg `json:"title"`
	Why          loc.Msg `json:"why"`
	GainEstimate int     `json:"gainEstimate"`
	GainRaw      int     `json:"gainRaw"`
	GainUpTo     bool    `json:"gainUpTo"`
	Reversible   bool    `json:"reversible"`
	Irreversible bool    `json:"irreversible"`
	NeedsAdmin   bool    `json:"needsAdmin"`
	Selected     bool    `json:"selected"`
	ActionID     string  `json:"actionId"`
	ActionArg    string  `json:"actionArg"`
	OpenURL      string  `json:"openUrl"`
	Instruction  loc.Msg `json:"instruction"`
	Priority     int     `json:"priority"`
}

// SmartFixPlan is the checkbox plan shown before apply.
type SmartFixPlan struct {
	Steps              []SmartFixStep   `json:"steps"`
	ScoreNow           int              `json:"scoreNow"`
	ScoreIfApplied     int              `json:"scoreIfApplied"`
	TotalGain          int              `json:"totalGain"`
	CleanupPreview     []CleanupCat     `json:"cleanupPreview"`
	HasCleanup         bool             `json:"hasCleanup"`
	Devices            []PnPDeviceError `json:"devices"`
	Displays           []MonitorInfo    `json:"displays"`
	StartupCandidates  []StartupItem    `json:"startupCandidates"`
}

// SmartFixAppliedStep is one step outcome in the result screen.
type SmartFixAppliedStep struct {
	StepID     string  `json:"stepId"`
	ScenarioID string  `json:"scenarioId"`
	OK         bool    `json:"ok"`
	Reversible bool    `json:"reversible"`
	Title      loc.Msg `json:"title"`
	Detail     loc.Msg `json:"detail"`
	OpenTab    string  `json:"openTab,omitempty"`
}

// SmartFixResult is returned after ApplySmartFix / RevertSmartFix.
type SmartFixResult struct {
	OK          bool                  `json:"ok"`
	SessionID   string                `json:"sessionId"`
	ScoreBefore int                   `json:"scoreBefore"`
	ScoreAfter  int                   `json:"scoreAfter"`
	FreedBytes  int64                 `json:"freedBytes"`
	FreedRAMMB  float64               `json:"freedRAMMB"`
	Applied     []SmartFixAppliedStep `json:"applied"`
	CanRevert   bool                  `json:"canRevert"`
	Message     loc.Msg               `json:"message"`
	OpenTab     string                `json:"openTab"`
}

// GetSmartFixPlan builds the full fix plan (all found scenarios, no card cap).
func (a *App) GetSmartFixPlan() SmartFixPlan {
	in := a.buildSmartFixInput()
	steps := EvaluateSmartFixScenarios(in)
	score := in.HealthScore
	// Cascade gains like home cards.
	asProblems := make([]HomeProblem, len(steps))
	for i, s := range steps {
		asProblems[i] = HomeProblem{
			ID: s.ID, Priority: s.Priority, GainEstimate: s.GainEstimate,
		}
	}
	asProblems = AssignCascadedGains(asProblems, score)
	for i := range steps {
		steps[i].GainEstimate = asProblems[i].GainEstimate
		steps[i].GainRaw = asProblems[i].GainRaw
		steps[i].GainUpTo = asProblems[i].GainUpTo
	}
	total := 0
	for _, s := range steps {
		total += s.GainEstimate
	}
	plan := SmartFixPlan{
		Steps:          steps,
		ScoreNow:       score,
		ScoreIfApplied: ScoreIfAllFixed(score, asProblems),
		TotalGain:      total,
	}
	for _, s := range steps {
		if s.ActionID == "cleanup_safe" {
			plan.HasCleanup = true
			break
		}
	}
	if plan.HasCleanup {
		plan.CleanupPreview = a.PreviewDiskCleanup()
	}
	plan.Devices = a.ListPnPDeviceErrors()
	plan.Displays = a.ListMonitors()
	plan.StartupCandidates = startupCandidatesForFix(in.StartupItems)
	return plan
}

func startupCandidatesForFix(items []StartupItem) []StartupItem {
	out := make([]StartupItem, 0, 8)
	for _, it := range items {
		if !it.Enabled || !it.CanDisable || it.Critical || it.Impact != "high" {
			continue
		}
		out = append(out, it)
	}
	return out
}

// ApplySmartFix applies selected step IDs, journals changes, returns result.
func (a *App) ApplySmartFix(ids []string) SmartFixResult {
	before := a.GetSystemHealth().Score
	plan := a.GetSmartFixPlan()
	want := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			want[id] = true
		}
	}
	if len(want) == 0 {
		// Default: all selected steps.
		for _, s := range plan.Steps {
			if s.Selected {
				want[s.ID] = true
			}
		}
	}

	sessionID := newSmartFixSessionID()
	sess := SmartFixJournalSession{
		ID: sessionID, ScoreBefore: before,
	}
	applied := make([]SmartFixAppliedStep, 0, len(plan.Steps))
	var freedBytes int64
	var freedRAM float64
	anyOK := false
	openTab := ""

	for _, step := range plan.Steps {
		if !want[step.ID] {
			continue
		}
		js, appStep, bytes, ram := a.applySmartFixStep(step)
		sess.Steps = append(sess.Steps, js)
		applied = append(applied, appStep)
		if appStep.OK {
			anyOK = true
			freedBytes += bytes
			freedRAM += ram
		}
		if appStep.OpenTab != "" && openTab == "" {
			openTab = appStep.OpenTab
		}
	}

	after := a.GetSystemHealth().Score
	sess.ScoreAfter = after
	sess.FreedBytes = freedBytes
	sess.FreedRAMMB = freedRAM
	a.appendSmartFixSession(sess)
	a.flushSmartFixJournal()

	canRevert := false
	for _, st := range sess.Steps {
		if st.OK && st.Reversible {
			canRevert = true
			break
		}
	}

	msg := loc.M("smartFixResultOK", loc.P(
		"before", before, "after", after,
		"gb", round1(float64(freedBytes)/(1024*1024*1024)),
		"mb", int(freedRAM+0.5),
	))
	if !anyOK {
		msg = loc.M("smartFixResultNone", nil)
	}
	return SmartFixResult{
		OK: anyOK, SessionID: sessionID,
		ScoreBefore: before, ScoreAfter: after,
		FreedBytes: freedBytes, FreedRAMMB: freedRAM,
		Applied: applied, CanRevert: canRevert, Message: msg,
		OpenTab: openTab,
	}
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

func (a *App) applySmartFixStep(step SmartFixStep) (SmartFixJournalStep, SmartFixAppliedStep, int64, float64) {
	js := SmartFixJournalStep{
		StepID: step.ID, ScenarioID: step.ScenarioID,
		ActionID: step.ActionID, ActionArg: step.ActionArg,
		Reversible: step.Reversible && !step.Irreversible,
	}
	out := SmartFixAppliedStep{
		StepID: step.ID, ScenarioID: step.ScenarioID,
		Title: step.Title, Reversible: js.Reversible,
	}
	var freedBytes int64
	var freedRAM float64

	switch step.ActionID {
	case "cleanup_safe":
		js.Kind = "cleanup"
		js.Reversible = false
		out.Reversible = false
		cats := strings.Split(step.ActionArg, ",")
		if len(cats) == 1 && cats[0] == "" {
			cats = nil // all safe non-admin
		}
		res := a.ApplyDiskCleanup(cats)
		js.OK = res.OK || res.ActualCleanedBytes > 0
		out.OK = js.OK
		freedBytes = res.ActualCleanedBytes
		if freedBytes <= 0 {
			freedBytes = res.FreedBytes
		}
		js.AfterJSON = mustJSON(map[string]interface{}{
			"freedBytes": freedBytes, "actualCleanedBytes": res.ActualCleanedBytes, "cats": res.Categories,
		})
		if js.OK {
			out.Detail = loc.M("smartFixCleanupDone", loc.P("gb", round1(float64(freedBytes)/(1024*1024*1024))))
		} else {
			out.Detail = loc.M("smartFixStepFail", nil)
		}

	case "startup_disable":
		js.Kind = "startup"
		items := a.GetStartupItems()
		var prevCmd string
		for _, it := range items {
			if it.ID == step.ActionArg {
				prevCmd = it.Command
				if prevCmd == "" {
					prevCmd = it.Path
				}
				break
			}
		}
		js.PrevJSON = mustJSON(map[string]string{"id": step.ActionArg, "command": prevCmd, "enabled": "true"})
		res := a.SetStartupItemEnabled(step.ActionArg, false)
		js.OK = res.OK
		out.OK = res.OK
		if res.OK {
			out.Detail = loc.M("smartFixStartupDisabled", loc.P("name", filepath.Base(step.ActionArg)))
		} else {
			out.Detail = loc.M("errGeneric", loc.P("err", res.MessageEn))
		}

	case "eco_process", "free_ram_temp":
		js.Kind = "eco"
		res := a.ApplyEcoWithFeedback(step.ActionArg)
		js.OK = res.OK
		out.OK = res.OK
		js.PrevJSON = mustJSON(map[string]string{"sessionId": res.SessionID, "name": step.ActionArg})
		js.AfterJSON = js.PrevJSON
		if res.SessionID != "" {
			js.Reversible = true
			out.Reversible = true
		}
		freedRAM = ecoFreedMB(res)
		if res.OK {
			code := "smartFixEcoDone"
			if step.ActionID == "free_ram_temp" {
				code = "smartFixRamTempDone"
			}
			out.Detail = loc.M(code, loc.P("name", friendlyProcessName(step.ActionArg), "mb", int(freedRAM+0.5)))
		} else {
			out.Detail = loc.M("smartFixStepFail", nil)
		}

	case "power_silent", "power_turbo":
		js.Kind = "power"
		a.mu.RLock()
		prev := a.powerMode
		a.mu.RUnlock()
		js.PrevJSON = mustJSON(map[string]string{"power": prev})
		mode := "silent"
		if step.ActionID == "power_turbo" {
			mode = "turbo"
		}
		err := a.SetPowerProfile(mode)
		js.OK = err == nil
		out.OK = err == nil
		js.Reversible = err == nil
		out.Reversible = js.Reversible
		if err == nil {
			out.Detail = loc.M("smartFixPowerDone", loc.P("mode", mode))
		} else {
			out.Detail = loc.M("errGeneric", loc.P("err", err.Error()))
		}

	case "game_mode_on", "game_bar_bg_off":
		js.Kind = "game_mode"
		prev, err := applyGameModeFix(step.ActionID)
		js.PrevJSON = mustJSON(prev)
		js.OK = err == nil
		out.OK = err == nil
		js.Reversible = err == nil
		out.Reversible = js.Reversible
		if err == nil {
			out.Detail = loc.M("smartFixGameModeDone", nil)
		} else {
			out.Detail = loc.M("errGeneric", loc.P("err", err.Error()))
		}

	case "set_refresh_rate":
		// Safe path: open monitors modal (TestAndSet + 15s auto-revert), never blind apply.
		js.Kind = "display"
		js.Reversible = false
		out.Reversible = false
		js.OK = true
		out.OK = true
		out.OpenTab = "monitors"
		out.Detail = loc.M("homeRefreshOpenMonitors", nil)

	case "disk_advice":
		js.Kind = "settings"
		js.OK = true
		out.OK = true
		js.Reversible = false
		out.Reversible = false
		out.Detail = loc.M("smartFixDiskAdviceHint", loc.P("letter", step.ActionArg))

	case "open_settings":
		js.Kind = "settings"
		js.Reversible = false
		out.Reversible = false
		if step.ActionArg == "startup" {
			js.OK = true
			out.OK = true
			out.Detail = loc.M("sfStartupOpenHint", nil)
			break
		}
		if step.ActionArg == "devmgmt.msc" {
			err := runHiddenStart("devmgmt.msc")
			js.OK = err == nil
			out.OK = err == nil
			out.Detail = step.Instruction
			if out.Detail.Code == "" {
				out.Detail = loc.M("smartFixOpenedSettings", nil)
			}
			break
		}
		url := step.OpenURL
		if url == "" {
			url = step.ActionArg
		}
		err := a.openSettingsURL(url)
		js.OK = err == nil
		out.OK = err == nil
		out.Detail = step.Instruction
		if out.Detail.Code == "" {
			out.Detail = loc.M("smartFixOpenedSettings", nil)
		}

	default:
		js.OK = false
		out.OK = false
		out.Detail = loc.M("homeActUnknown", nil)
	}
	return js, out, freedBytes, freedRAM
}

func parseRefreshArg(arg string) (device string, hz int) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", 0
	}
	// format: deviceName|hz  or just hz
	if i := strings.LastIndex(arg, "|"); i >= 0 {
		device = strings.TrimSpace(arg[:i])
		fmtS := strings.TrimSpace(arg[i+1:])
		var n int
		for _, ch := range fmtS {
			if ch < '0' || ch > '9' {
				break
			}
			n = n*10 + int(ch-'0')
		}
		return device, n
	}
	var n int
	for _, ch := range arg {
		if ch < '0' || ch > '9' {
			break
		}
		n = n*10 + int(ch-'0')
	}
	return "", n
}

func (a *App) openSettingsURL(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil
	}
	if a.ctx != nil {
		runtime.BrowserOpenURL(a.ctx, url)
		return nil
	}
	return runHiddenStart("explorer.exe", url)
}

// RevertSmartFix undoes reversible steps of a session (newest first within session).
func (a *App) RevertSmartFix(sessionID string) SmartFixResult {
	before := a.GetSystemHealth().Score
	sess, ok := a.getSmartFixSession(sessionID)
	if !ok || sess.Reverted {
		return SmartFixResult{OK: false, ScoreBefore: before, ScoreAfter: before,
			Message: loc.M("smartFixRevertMissing", nil)}
	}
	applied := make([]SmartFixAppliedStep, 0, len(sess.Steps))
	any := false
	for i := len(sess.Steps) - 1; i >= 0; i-- {
		st := sess.Steps[i]
		if !st.OK || !st.Reversible {
			continue
		}
		okStep, detail := a.revertSmartFixStep(st)
		applied = append(applied, SmartFixAppliedStep{
			StepID: st.StepID, ScenarioID: st.ScenarioID,
			OK: okStep, Reversible: false,
			Title:  loc.M("smartFixRevertStep", loc.P("id", st.ScenarioID)),
			Detail: detail,
		})
		if okStep {
			any = true
		}
	}
	if any {
		a.markSmartFixSessionReverted(sessionID)
		a.flushSmartFixJournal()
	}
	after := a.GetSystemHealth().Score
	msg := loc.M("smartFixRevertOK", loc.P("before", before, "after", after))
	if !any {
		msg = loc.M("smartFixRevertNone", nil)
	}
	return SmartFixResult{
		OK: any, SessionID: sessionID,
		ScoreBefore: before, ScoreAfter: after,
		Applied: applied, CanRevert: false, Message: msg,
	}
}

func (a *App) revertSmartFixStep(st SmartFixJournalStep) (bool, loc.Msg) {
	switch st.Kind {
	case "startup":
		var prev struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal([]byte(st.PrevJSON), &prev)
		if prev.ID == "" {
			prev.ID = st.ActionArg
		}
		res := a.SetStartupItemEnabled(prev.ID, true)
		if res.OK {
			return true, loc.M("smartFixStartupRestored", nil)
		}
		return false, loc.M("errGeneric", loc.P("err", res.MessageEn))
	case "eco":
		var prev struct {
			SessionID string `json:"sessionId"`
		}
		_ = json.Unmarshal([]byte(st.PrevJSON), &prev)
		if prev.SessionID == "" {
			return false, loc.M("smartFixRevertNone", nil)
		}
		res := a.RevertEcoSession(prev.SessionID)
		return res.OK, loc.M("smartFixEcoReverted", nil)
	case "power":
		var prev struct {
			Power string `json:"power"`
		}
		_ = json.Unmarshal([]byte(st.PrevJSON), &prev)
		if prev.Power == "" {
			prev.Power = "balanced"
		}
		err := a.SetPowerProfile(prev.Power)
		if err != nil {
			return false, loc.M("errGeneric", loc.P("err", err.Error()))
		}
		return true, loc.M("smartFixPowerRestored", loc.P("mode", prev.Power))
	case "game_mode":
		var prev map[string]interface{}
		_ = json.Unmarshal([]byte(st.PrevJSON), &prev)
		if err := restoreGameModeFix(prev); err != nil {
			return false, loc.M("errGeneric", loc.P("err", err.Error()))
		}
		return true, loc.M("smartFixGameModeRestored", nil)
	default:
		return false, loc.M("smartFixRevertNone", nil)
	}
}

func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
