package main

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/robbled17/zeropulse/loc"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// HomeState is the payload for the main «Состояние ПК» screen.
type HomeState struct {
	Score           int               `json:"score"`
	ScoreIfAllFixed int               `json:"scoreIfAllFixed"`
	Status          loc.Msg           `json:"status"`
	CheckedAgoSec   int               `json:"checkedAgoSec"`
	Problems        []HomeProblem     `json:"problems"`
	AllGood         bool              `json:"allGood"`
	Incomplete      bool              `json:"incomplete"`
	MissingNote     string            `json:"missingNote"`
	Deductions      []HealthDeduction `json:"deductions"`
	PrepareActive   bool              `json:"prepareActive"`
	PrepareRAMMB    float64           `json:"prepareRAMMB"`
}

// HomeActionResult reports outcome of a one-click fix.
type HomeActionResult struct {
	OK          bool    `json:"ok"`
	Message     loc.Msg `json:"message"`
	ScoreBefore int     `json:"scoreBefore"`
	ScoreAfter  int     `json:"scoreAfter"`
	FreedRAMMB  float64 `json:"freedRAMMB"`
	CanRevert   bool    `json:"canRevert"`
	OpenTab     string  `json:"openTab"`
}

type prepareGameState struct {
	mu         sync.Mutex
	active     bool
	sessionIDs []string
	prevPower  string
	hudWasOn   bool
	freedMB    float64
}

func (a *App) ensurePrepare() *prepareGameState {
	if a.prepareGame == nil {
		a.prepareGame = &prepareGameState{}
	}
	return a.prepareGame
}

func ecoFreedMB(res EcoApplyResult) float64 {
	if res.RAMBeforeMB > res.RAMAfterMB && res.RAMAfterMB > 0 {
		return res.RAMBeforeMB - res.RAMAfterMB
	}
	return 0
}

// GetHomeState builds the home screen from existing System Health + catalog (no new collectors).
func (a *App) GetHomeState() HomeState {
	health := a.GetSystemHealth()
	a.mu.RLock()
	m := a.metrics
	power := a.powerMode
	a.mu.RUnlock()

	sysUsed, sysOK := systemDiskUsedPct(m.Disks)
	topRAMName, topFriend, topEco := "", "", false
	if len(m.Vampires) > 0 {
		v := m.Vampires[0]
		topRAMName = v.Name
		topFriend = v.FriendlyName
		if topFriend == "" {
			topFriend = friendlyProcessName(v.Name)
		}
		topEco = !v.Frozen && !isCriticalSystemProc(v.Name) && !isEcoProtected(v.Name)
	}
	if topRAMName == "" {
		bb := a.ensureBlackBox()
		bb.mu.Lock()
		tops := append([]ProcessSnap(nil), bb.cachedTopRAM...)
		bb.mu.Unlock()
		for _, p := range tops {
			base := filepath.Base(p.Name)
			if isSelfOrWebView(base) || isCriticalSystemProc(base) {
				continue
			}
			topRAMName = p.Name
			topFriend = friendlyProcessName(p.Name)
			topEco = !isEcoProtected(base)
			break
		}
	}

	lagCulprit, lagFriend, lagEco := "", "", false
	cards := a.GetLagIncidents()
	if len(cards) > 0 {
		c := cards[0]
		lagCulprit = c.Culprit
		lagFriend = friendlyProcessName(c.Culprit)
		lagEco = c.CanEco && !c.SystemProc
	}

	diskBad, diskCaution := false, false
	diskLetter := ""
	var diskReason loc.Msg
	if smart, ok := a.cachedSystemDiskSMART(); ok {
		diskLetter = strings.TrimSuffix(smart.DriveLetter, ":")
		if diskLetter == "" {
			diskLetter = "C"
		}
		switch smart.StatusLevel {
		case DiskStatusBad:
			diskBad = true
			diskReason = smart.StatusReason
			if diskReason.Code == "" && smart.StatusCode != "" {
				diskReason = loc.M(smart.StatusCode, nil)
			}
		case DiskStatusCaution:
			diskCaution = true
			diskReason = smart.StatusReason
			if diskReason.Code == "" {
				if smart.HealthPercentOK {
					diskReason = loc.M("diskReasonHealthPct", loc.P(
						"pct", int(smart.HealthPercent+0.5),
						"source", smart.HealthSourceMsg.Code,
					))
					// Prefer embedding source text via nested code on FE; if source missing use pct only.
					if smart.HealthSourceMsg.Code == "" {
						diskReason = loc.M("diskReasonHealthPctPlain", loc.P("pct", int(smart.HealthPercent+0.5)))
					}
				} else if smart.StatusCode != "" {
					diskReason = loc.M(smart.StatusCode, nil)
				}
			}
		}
	}
	refreshHz, refreshMax, refreshDev, refreshName := 0, 0, "", ""
	if mon, ok := primaryUnderclockedMonitor(); ok {
		refreshHz, refreshMax = mon.CurrentHz, mon.MaxSupportedHz
		refreshDev, refreshName = mon.DeviceName, mon.MonitorName
	}

	in := HomeCatalogInput{
		SystemDiskUsed:   sysUsed,
		SystemDiskOK:     sysOK,
		RAMPercent:       m.RAMPercent,
		RAMOK:            m.RAMTotalMB > 0,
		TopRAMName:       topRAMName,
		TopRAMFriendly:   topFriend,
		TopRAMEcoOK:      topEco,
		LagCulprit:       lagCulprit,
		LagCulpritFriend: lagFriend,
		LagCulpritEcoOK:  lagEco,
		LagCount:         len(cards),
		StartupHighCount: countHighImpactStartup(a.GetStartupItems()),
		PowerMode:        power,
		GameActive:       a.isGameSessionActive(),
		TempCPU:          m.TemperatureC,
		TempOK:           m.TemperatureAvailable && m.TemperatureC > 0,
		HealthScore:      health.Score,
		DiskBad:          diskBad,
		DiskCaution:      diskCaution,
		DiskBadLetter:    diskLetter,
		DiskReason:       diskReason,
		RefreshHz:        refreshHz,
		RefreshHzMax:     refreshMax,
		RefreshDevice:    refreshDev,
		RefreshName:      refreshName,
	}
	probs := EvaluateHomeProblems(in, 3)
	probs = a.filterDismissedProblems(probs, sysUsed, sysOK)
	status := HomeStatusWord(health.Score)

	pg := a.ensurePrepare()
	pg.mu.Lock()
	prepOn := pg.active
	prepRAM := pg.freedMB
	pg.mu.Unlock()

	return HomeState{
		Score: health.Score, ScoreIfAllFixed: ScoreIfAllFixed(health.Score, probs),
		Status: status,
		CheckedAgoSec: 2, Problems: probs, AllGood: len(probs) == 0,
		Incomplete: health.Incomplete, MissingNote: health.MissingNote,
		Deductions: health.Deductions,
		PrepareActive: prepOn, PrepareRAMMB: prepRAM,
	}
}

// RunHomeAction executes a catalog action and returns score delta when possible.
func (a *App) RunHomeAction(actionID, actionArg string) HomeActionResult {
	before := a.GetSystemHealth().Score
	actionID = strings.TrimSpace(actionID)
	switch actionID {
	case "free_disk":
		// Open Disk Cleanup Wizard in UI — no blind 100MB loop.
		return HomeActionResult{OK: true, ScoreBefore: before, ScoreAfter: before,
			OpenTab: "disk_cleanup", Message: loc.Msg{}}
	case "set_refresh_rate":
		// Never apply Hz blindly — open the safe monitors modal for TestAndSet + 15s revert.
		_ = actionArg
		return HomeActionResult{OK: true, ScoreBefore: before, ScoreAfter: before,
			OpenTab: "monitors", Message: loc.M("homeRefreshOpenMonitors", nil)}
	case "dismiss_problem":
		id := strings.TrimSpace(actionArg)
		if id == "" {
			return HomeActionResult{OK: false, ScoreBefore: before, ScoreAfter: before,
				Message: loc.M("errGeneric", loc.P("err", "empty problem id"))}
		}
		a.DismissHomeProblem(id)
		after := a.GetSystemHealth().Score
		return HomeActionResult{OK: true, ScoreBefore: before, ScoreAfter: after,
			Message: loc.M("homeProblemDismissed", nil)}
	case "eco_process":
		res := a.ApplyEcoWithFeedback(actionArg)
		after := a.GetSystemHealth().Score
		freed := ecoFreedMB(res)
		name := friendlyProcessName(actionArg)
		msg := loc.M("ecoApplyFail", nil)
		if res.OK {
			if after > before {
				msg = loc.M("homeActEcoLimitedScore", loc.P("name", name, "before", before, "after", after))
			} else {
				msg = loc.M("homeActEcoLimited", loc.P("name", name))
			}
		} else if res.MessageEn != "" || res.MessageRu != "" {
			errText := res.MessageEn
			if errText == "" {
				errText = res.MessageRu
			}
			msg = loc.M("errGeneric", loc.P("err", errText))
		}
		return HomeActionResult{
			OK: res.OK, ScoreBefore: before, ScoreAfter: after,
			FreedRAMMB: freed, CanRevert: res.CanRevert,
			Message: msg,
		}
	case "open_startup":
		return HomeActionResult{OK: true, ScoreBefore: before, ScoreAfter: before,
			OpenTab: "startup", Message: loc.M("homeActOpenStartup", nil)}
	case "open_disk":
		return HomeActionResult{OK: true, ScoreBefore: before, ScoreAfter: before,
			OpenTab: "home", Message: loc.M("homeActOpenDisk", nil)}
	case "disk_advice":
		// Frontend opens the What-to-do panel; no toast.
		return HomeActionResult{OK: true, ScoreBefore: before, ScoreAfter: before,
			OpenTab: "home", Message: loc.Msg{}}
	case "disk_watch_on":
		letter := actionArg
		if letter == "" {
			letter = "C"
		}
		a.SetDiskWatching(letter, true)
		a.flushDiskWatch()
		return HomeActionResult{OK: true, ScoreBefore: before, ScoreAfter: before,
			Message: loc.M("diskWatchEnabled", loc.P("letter", strings.ToUpper(strings.TrimSuffix(letter, ":"))))}
	case "open_windows_backup":
		if err := a.OpenWindowsBackup(); err != nil {
			return HomeActionResult{OK: false, ScoreBefore: before, ScoreAfter: before,
				Message: loc.M("errGeneric", loc.P("err", err.Error()))}
		}
		return HomeActionResult{OK: true, ScoreBefore: before, ScoreAfter: before,
			Message: loc.M("diskAdviceBackupOpened", nil)}
	case "open_settings":
		arg := strings.TrimSpace(actionArg)
		if arg == "devmgmt.msc" {
			_ = runHiddenStart("devmgmt.msc")
			return HomeActionResult{OK: true, ScoreBefore: before, ScoreAfter: before,
				Message: loc.M("smartFixOpenedSettings", nil)}
		}
		if arg == "" {
			arg = "ms-settings:storagesense"
		}
		if err := a.openSettingsURL(arg); err != nil {
			return HomeActionResult{OK: false, ScoreBefore: before, ScoreAfter: before,
				Message: loc.M("errGeneric", loc.P("err", err.Error()))}
		}
		return HomeActionResult{OK: true, ScoreBefore: before, ScoreAfter: before,
			Message: loc.M("smartFixOpenedSettings", nil)}
	case "power_turbo":
		if err := a.SetPowerProfile("turbo"); err != nil {
			return HomeActionResult{OK: false, ScoreBefore: before, ScoreAfter: before,
				Message: loc.M("errGeneric", loc.P("err", err.Error()))}
		}
		after := a.GetSystemHealth().Score
		return HomeActionResult{OK: true, ScoreBefore: before, ScoreAfter: after,
			Message: loc.M("homeActPowerTurbo", nil)}
	case "power_silent":
		if err := a.SetPowerProfile("silent"); err != nil {
			return HomeActionResult{OK: false, ScoreBefore: before, ScoreAfter: before,
				Message: loc.M("errGeneric", loc.P("err", err.Error()))}
		}
		after := a.GetSystemHealth().Score
		return HomeActionResult{OK: true, ScoreBefore: before, ScoreAfter: after,
			Message: loc.M("homeActPowerSilent", nil)}
	case "prepare_game":
		return a.PrepareForGame()
	case "revert_prepare":
		return a.RevertPrepareForGame()
	default:
		return HomeActionResult{OK: false, ScoreBefore: before, ScoreAfter: before,
			Message: loc.M("homeActUnknown", nil)}
	}
}

// OpenDiskCleanup launches Windows Disk Cleanup for C: (never deletes files itself).
func (a *App) OpenDiskCleanup() error {
	if err := runHiddenStart("cleanmgr", "/d", "C"); err == nil {
		return nil
	}
	if a.ctx != nil {
		runtime.BrowserOpenURL(a.ctx, "ms-settings:storagesense")
		return nil
	}
	// Prefer explorer protocol handler over cmd.exe to avoid console flash.
	return runHiddenStart("explorer.exe", "ms-settings:storagesense")
}

// OpenWindowsBackup opens Windows Backup settings (no destructive actions).
func (a *App) OpenWindowsBackup() error {
	if a.ctx != nil {
		runtime.BrowserOpenURL(a.ctx, "ms-settings:backup")
		return nil
	}
	return runHiddenStart("explorer.exe", "ms-settings:backup")
}

// PrepareForGame applies Eco to background vampires, Turbo power, Mini-HUD.
func (a *App) PrepareForGame() HomeActionResult {
	before := a.GetSystemHealth().Score
	pg := a.ensurePrepare()
	pg.mu.Lock()
	if pg.active {
		freed := pg.freedMB
		pg.mu.Unlock()
		return HomeActionResult{OK: true, ScoreBefore: before, ScoreAfter: before,
			FreedRAMMB: freed, CanRevert: true,
			Message: loc.M("homeActPrepareAlready", loc.P("mb", int(freed+0.5)))}
	}
	a.mu.RLock()
	prevPower := a.powerMode
	hudWas := a.miniHUD
	a.mu.RUnlock()
	pg.prevPower = prevPower
	pg.hudWasOn = hudWas
	pg.mu.Unlock()

	var freed float64
	var sessions []string
	vamps := a.GetVampireProcesses()
	for i, v := range vamps {
		if i >= 5 {
			break
		}
		if v.Frozen || isCriticalSystemProc(v.Name) || isEcoProtected(v.Name) || isSelfOrWebView(v.Name) {
			continue
		}
		res := a.ApplyEcoWithFeedback(v.Name)
		if res.OK {
			freed += ecoFreedMB(res)
			if res.SessionID != "" {
				sessions = append(sessions, res.SessionID)
			}
		}
	}
	_ = a.SetPowerProfile("turbo")
	a.ToggleMiniHUD(true)

	pg.mu.Lock()
	pg.active = true
	pg.sessionIDs = sessions
	pg.freedMB = freed
	pg.mu.Unlock()

	after := a.GetSystemHealth().Score
	msg := loc.M("homeActPrepareDone", loc.P("mb", int(freed+0.5)))
	if after > before {
		msg = loc.M("homeActPrepareDoneScore", loc.P("mb", int(freed+0.5), "before", before, "after", after))
	}
	return HomeActionResult{
		OK: true, ScoreBefore: before, ScoreAfter: after,
		FreedRAMMB: freed, CanRevert: true,
		Message: msg,
	}
}

// RevertPrepareForGame undoes PrepareForGame.
func (a *App) RevertPrepareForGame() HomeActionResult {
	before := a.GetSystemHealth().Score
	pg := a.ensurePrepare()
	pg.mu.Lock()
	if !pg.active {
		pg.mu.Unlock()
		return HomeActionResult{OK: true, ScoreBefore: before, ScoreAfter: before,
			Message: loc.M("homeActRevertNothing", nil)}
	}
	ids := append([]string(nil), pg.sessionIDs...)
	prev := pg.prevPower
	hudWas := pg.hudWasOn
	pg.active = false
	pg.sessionIDs = nil
	pg.freedMB = 0
	pg.mu.Unlock()

	for _, id := range ids {
		_ = a.RevertEcoSession(id)
	}
	if prev == "" {
		prev = "balanced"
	}
	_ = a.SetPowerProfile(prev)
	a.ToggleMiniHUD(hudWas)

	after := a.GetSystemHealth().Score
	return HomeActionResult{OK: true, ScoreBefore: before, ScoreAfter: after, CanRevert: false,
		Message: loc.M("homeActRevertOK", nil)}
}
