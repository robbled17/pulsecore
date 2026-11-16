package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/robbled17/zeropulse/loc"
)

// SmartFixInput aggregates signals for scenario evaluation (pure + App-filled).
type SmartFixInput struct {
	HomeCatalogInput
	StartupItems     []StartupItem
	SystemOnHDD      bool
	GamesOnHDD       bool
	RefreshHz        int
	RefreshHzMax     int
	RefreshDevice    string
	RefreshName      string
	RAMSingleChannel bool
	RAMBelowRated    bool
	GPUDriverDays    int // age in days; 0 = unknown
	GPUDriverVendor  string
	DeviceErrorCount int
	GameModeOff      bool
	GameBarBgOn      bool
	UptimeDays       int
}

func (a *App) buildSmartFixInput() SmartFixInput {
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
		case DiskStatusCaution:
			diskCaution = true
			diskReason = smart.StatusReason
		}
	}

	items := a.GetStartupItems()
	in := SmartFixInput{
		HomeCatalogInput: HomeCatalogInput{
			SystemDiskUsed: sysUsed, SystemDiskOK: sysOK,
			RAMPercent: m.RAMPercent, RAMOK: m.RAMTotalMB > 0,
			TopRAMName: topRAMName, TopRAMFriendly: topFriend, TopRAMEcoOK: topEco,
			LagCulprit: lagCulprit, LagCulpritFriend: lagFriend, LagCulpritEcoOK: lagEco,
			LagCount: len(cards), StartupHighCount: countHighImpactStartup(items),
			PowerMode: power, GameActive: a.isGameSessionActive(),
			TempCPU: m.TemperatureC, TempOK: m.TemperatureAvailable && m.TemperatureC > 0,
			HealthScore: health.Score,
			DiskBad: diskBad, DiskCaution: diskCaution, DiskBadLetter: diskLetter, DiskReason: diskReason,
		},
		StartupItems: items,
		UptimeDays:   int(a.uptimeDuration().Hours() / 24),
	}

	// Optional detectors — skip scenario on failure.
	if mon, ok := primaryUnderclockedMonitor(); ok {
		in.RefreshHz, in.RefreshHzMax = mon.CurrentHz, mon.MaxSupportedHz
		in.RefreshDevice, in.RefreshName = mon.DeviceName, mon.MonitorName
	} else if hz, max, ok := queryDisplayRefresh(); ok {
		in.RefreshHz, in.RefreshHzMax = hz, max
	}
	if single, below, ok := queryRAMChannelHints(); ok {
		in.RAMSingleChannel, in.RAMBelowRated = single, below
	}
	if days, vendor, ok := queryGPUDriverAge(); ok {
		in.GPUDriverDays, in.GPUDriverVendor = days, vendor
	}
	if n, ok := queryDeviceErrorCount(); ok {
		in.DeviceErrorCount = n
	}
	if off, bg, ok := queryGameModeFlags(); ok {
		in.GameModeOff, in.GameBarBgOn = off, bg
	}
	in.SystemOnHDD = a.systemDiskIsHDD()
	in.GamesOnHDD = a.gamesLikelyOnHDD()
	return in
}

// EvaluateSmartFixScenarios returns all matching steps (no max-3 cap).
func EvaluateSmartFixScenarios(in SmartFixInput) []SmartFixStep {
	out := make([]SmartFixStep, 0, 16)

	if in.SystemDiskOK && in.SystemDiskUsed >= 85 {
		gain := 8
		if in.SystemDiskUsed >= 90 {
			gain = 12
		}
		if in.SystemDiskUsed >= 95 {
			gain = 16
		}
		pct := int(in.SystemDiskUsed + 0.5)
		out = append(out, SmartFixStep{
			ID: "sf_disk_full", ScenarioID: "disk_full", Priority: 10,
			Title: loc.M("homeDiskFullTitle", loc.P("pct", pct)),
			Why:   loc.M("homeDiskFullWhy", nil),
			GainEstimate: gain, Selected: true,
			ActionID: "cleanup_safe", ActionArg: "shader_cache,recycle,app_cache,user_temp,browser_cache,thumbnails,wer_reports,package_cache",
			Irreversible: true, Reversible: false,
			Instruction: loc.M("sfCleanupInstruction", nil),
		})
	}

	// Per-startup disables for high-impact canDisable items.
	highN := 0
	for _, it := range in.StartupItems {
		if !it.Enabled || !it.CanDisable || it.Critical || it.Impact != "high" {
			continue
		}
		highN++
		name := it.Name
		if name == "" {
			name = filepath.Base(it.Path)
		}
		out = append(out, SmartFixStep{
			ID: "sf_startup_" + it.ID, ScenarioID: "startup_item", Priority: 40,
			Title: loc.M("sfStartupDisableTitle", loc.P("name", name)),
			Why:   loc.M("sfStartupDisableWhy", loc.P("name", name)),
			GainEstimate: 3, Selected: true,
			ActionID: "startup_disable", ActionArg: it.ID,
			Reversible: true,
		})
	}
	if highN == 0 && in.StartupHighCount >= 2 {
		out = append(out, SmartFixStep{
			ID: "sf_startup_review", ScenarioID: "startup_heavy", Priority: 40,
			Title: loc.M("homeStartupHeavyTitle", loc.P("n", in.StartupHighCount)),
			Why:   loc.M("homeStartupHeavyWhy", nil),
			GainEstimate: smartFixMin(10, in.StartupHighCount*3), Selected: true,
			ActionID: "open_settings", OpenURL: "",
			ActionArg: "startup", // FE opens startup tab
			Instruction: loc.M("sfStartupOpenHint", nil),
			Reversible: false,
		})
	}

	if in.RAMOK && in.RAMPercent >= 85 && in.TopRAMName != "" && in.TopRAMEcoOK {
		friend := in.TopRAMFriendly
		if friend == "" {
			friend = filepath.Base(in.TopRAMName)
		}
		pct := int(in.RAMPercent + 0.5)
		out = append(out, SmartFixStep{
			ID: "sf_ram", ScenarioID: "ram_pressure", Priority: 20,
			Title: loc.M("homeRamHighTitle", loc.P("pct", pct)),
			Why:   loc.M("sfRamTempWhy", loc.P("name", friend)),
			GainEstimate: 7, Selected: true,
			ActionID: "free_ram_temp", ActionArg: in.TopRAMName,
			Reversible: true,
		})
	}

	if in.LagCount > 0 && in.LagCulprit != "" && in.LagCulpritEcoOK &&
		!strings.EqualFold(in.LagCulprit, "many_processes") &&
		!strings.EqualFold(in.LagCulprit, "unknown") {
		friend := in.LagCulpritFriend
		if friend == "" {
			friend = filepath.Base(in.LagCulprit)
		}
		out = append(out, SmartFixStep{
			ID: "sf_lag", ScenarioID: "lag_culprit", Priority: 15,
			Title: loc.M("homeLagTitle", loc.P("name", friend)),
			Why:   loc.M("homeLagWhy", nil),
			GainEstimate: 6, Selected: true,
			ActionID: "eco_process", ActionArg: in.LagCulprit,
			Reversible: true,
		})
	}

	if in.TempOK && in.TempCPU >= 90 {
		temp := int(in.TempCPU + 0.5)
		out = append(out, SmartFixStep{
			ID: "sf_cpu_hot", ScenarioID: "cpu_hot", Priority: 12,
			Title: loc.M("homeCpuHotTitle", loc.P("temp", temp)),
			Why:   loc.M("homeCpuHotWhy", nil),
			GainEstimate: 4, Selected: true,
			ActionID: "power_silent", Reversible: true,
		})
	}

	if in.GameActive && (in.PowerMode == "silent" || in.PowerMode == "") {
		out = append(out, SmartFixStep{
			ID: "sf_power_game", ScenarioID: "power_silent_game", Priority: 25,
			Title: loc.M("homeGameSilentTitle", nil),
			Why:   loc.M("homeGameSilentWhy", nil),
			GainEstimate: 5, Selected: true,
			ActionID: "power_turbo", Reversible: true,
		})
	}

	if in.DiskBad {
		letter := in.DiskBadLetter
		if letter == "" {
			letter = "C"
		}
		out = append(out, SmartFixStep{
			ID: "sf_disk_bad", ScenarioID: "disk_smart_bad", Priority: 8,
			Title: loc.M("homeDiskBadTitle", loc.P("letter", letter)),
			Why:   in.DiskReason,
			GainEstimate: 5, Selected: true,
			ActionID: "disk_advice", ActionArg: letter, Reversible: false,
			Instruction: loc.M("sfDiskAdviceInstruction", loc.P("letter", letter)),
		})
	} else if in.DiskCaution {
		letter := in.DiskBadLetter
		if letter == "" {
			letter = "C"
		}
		out = append(out, SmartFixStep{
			ID: "sf_disk_caution", ScenarioID: "disk_smart_caution", Priority: 7,
			Title: loc.M("homeDiskCautionTitle", loc.P("letter", letter)),
			Why:   in.DiskReason,
			GainEstimate: 3, Selected: true,
			ActionID: "disk_advice", ActionArg: letter, Reversible: false,
			Instruction: loc.M("sfDiskAdviceInstruction", loc.P("letter", letter)),
		})
	}

	if in.SystemOnHDD {
		out = append(out, SmartFixStep{
			ID: "sf_os_hdd", ScenarioID: "os_on_hdd", Priority: 35,
			Title: loc.M("sfOsOnHddTitle", nil),
			Why:   loc.M("sfOsOnHddWhy", nil),
			GainEstimate: 2, Selected: false,
			ActionID: "open_settings", OpenURL: "ms-settings:storagesense",
			Instruction: loc.M("sfOsOnHddInstruction", nil), Reversible: false,
		})
	}
	if in.GamesOnHDD {
		out = append(out, SmartFixStep{
			ID: "sf_games_hdd", ScenarioID: "games_on_hdd", Priority: 36,
			Title: loc.M("sfGamesOnHddTitle", nil),
			Why:   loc.M("sfGamesOnHddWhy", nil),
			GainEstimate: 2, Selected: false,
			ActionID: "open_settings", OpenURL: "ms-settings:storagesense",
			Instruction: loc.M("sfGamesOnHddInstruction", nil), Reversible: false,
		})
	}

	if in.RefreshHzMax > 0 && in.RefreshHz > 0 && in.RefreshHz < in.RefreshHzMax {
		name := in.RefreshName
		if name == "" {
			name = "Monitor"
		}
		arg := fmt.Sprintf("%s|%d", in.RefreshDevice, in.RefreshHzMax)
		if in.RefreshDevice == "" {
			arg = fmt.Sprintf("%d", in.RefreshHzMax)
		}
		out = append(out, SmartFixStep{
			ID: "sf_refresh", ScenarioID: "refresh_rate", Priority: 45,
			Title: loc.M("sfRefreshTitle", loc.P("name", name, "hz", in.RefreshHz, "max", in.RefreshHzMax)),
			Why:   loc.M("sfRefreshWhy", loc.P("max", in.RefreshHzMax)),
			GainEstimate: 5, Selected: true,
			ActionID: "set_refresh_rate", ActionArg: arg,
			Instruction: loc.M("sfRefreshInstruction", loc.P("max", in.RefreshHzMax)), Reversible: false,
		})
	}

	if in.RAMSingleChannel || in.RAMBelowRated {
		out = append(out, SmartFixStep{
			ID: "sf_ram_config", ScenarioID: "ram_config", Priority: 48,
			Title: loc.M("sfRamConfigTitle", nil),
			Why:   loc.M("sfRamConfigWhy", nil),
			GainEstimate: 3, Selected: false,
			ActionID: "open_settings", OpenURL: "ms-settings:about",
			Instruction: loc.M("sfRamConfigInstruction", nil), Reversible: false,
		})
	}

	if in.GPUDriverDays >= 365 {
		out = append(out, SmartFixStep{
			ID: "sf_gpu_driver", ScenarioID: "gpu_driver_old", Priority: 42,
			Title: loc.M("sfGpuDriverTitle", loc.P("days", in.GPUDriverDays)),
			Why:   loc.M("sfGpuDriverWhy", nil),
			GainEstimate: 3, Selected: true,
			ActionID: "open_settings", OpenURL: gpuDriverURL(in.GPUDriverVendor),
			Instruction: loc.M("sfGpuDriverInstruction", nil), Reversible: false,
		})
	}

	if in.DeviceErrorCount > 0 {
		out = append(out, SmartFixStep{
			ID: "sf_device_err", ScenarioID: "device_errors", Priority: 30,
			Title: loc.M("sfDeviceErrTitle", loc.P("n", in.DeviceErrorCount)),
			Why:   loc.M("sfDeviceErrWhy", nil),
			GainEstimate: 2, Selected: true,
			ActionID: "open_settings", OpenURL: "ms-settings:devices",
			ActionArg: "devmgmt.msc",
			Instruction: loc.M("sfDeviceErrInstruction", nil), Reversible: false,
		})
	}

	if in.GameModeOff {
		out = append(out, SmartFixStep{
			ID: "sf_game_mode", ScenarioID: "game_mode_off", Priority: 50,
			Title: loc.M("sfGameModeTitle", nil),
			Why:   loc.M("sfGameModeWhy", nil),
			GainEstimate: 2, Selected: true,
			ActionID: "game_mode_on", Reversible: true,
		})
	}
	if in.GameBarBgOn {
		out = append(out, SmartFixStep{
			ID: "sf_game_bar", ScenarioID: "game_bar_bg", Priority: 51,
			Title: loc.M("sfGameBarTitle", nil),
			Why:   loc.M("sfGameBarWhy", nil),
			GainEstimate: 2, Selected: true,
			ActionID: "game_bar_bg_off", Reversible: true,
		})
	}

	if in.UptimeDays >= 14 {
		out = append(out, SmartFixStep{
			ID: "sf_uptime", ScenarioID: "long_uptime", Priority: 55,
			Title: loc.M("sfUptimeTitle", loc.P("days", in.UptimeDays)),
			Why:   loc.M("sfUptimeWhy", nil),
			GainEstimate: 2, Selected: true,
			ActionID: "open_settings", OpenURL: "ms-settings:powersleep",
			Instruction: loc.M("sfUptimeInstruction", nil), Reversible: false,
		})
	}

	// Sort by priority ascending.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Priority < out[i].Priority || (out[j].Priority == out[i].Priority && out[j].ID < out[i].ID) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func smartFixMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (a *App) uptimeDuration() time.Duration {
	if a != nil && a.IsDevMode() && a.metricsProvider != nil {
		return a.metricsProvider.GetUptime()
	}
	return time.Duration(getTickCount64()) * time.Millisecond
}

func uptimeDuration() time.Duration {
	return time.Duration(getTickCount64()) * time.Millisecond
}

func gpuDriverURL(vendor string) string {
	v := strings.ToLower(vendor)
	switch {
	case strings.Contains(v, "nvidia"):
		return "https://www.nvidia.com/Download/index.aspx"
	case strings.Contains(v, "amd"), strings.Contains(v, "ati"):
		return "https://www.amd.com/en/support"
	case strings.Contains(v, "intel"):
		return "https://www.intel.com/content/www/us/en/download-center/home.html"
	default:
		return "ms-settings:windowsupdate"
	}
}
