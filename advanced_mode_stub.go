//go:build !windows

package main

import "github.com/robbled17/zeropulse/loc"

func advancedModeEnabled() bool { return false }

func diskReportFromHelper(letter string) (DiskHealthReport, bool) {
	return DiskHealthReport{}, false
}

func (a *App) GetAdvancedModeState() AdvancedModeState {
	return AdvancedModeState{Status: "off", StatusRu: "выключен", StatusEn: "off"}
}

func (a *App) EnableAdvancedMode(consentPawnIO bool) AdvancedModeActionResult {
	return AdvancedModeActionResult{
		OK:      false,
		Message: loc.M("advWindowsOnly", nil),
		MessageRu: "Только Windows",
		MessageEn: "Windows only",
		State: a.GetAdvancedModeState(),
	}
}

func (a *App) DisableAdvancedMode() AdvancedModeActionResult {
	return AdvancedModeActionResult{OK: true, State: a.GetAdvancedModeState()}
}

func (a *App) GetHelperDiagnostics() string {
	return "Windows only"
}

func (a *App) RestartHelper() AdvancedModeActionResult {
	return AdvancedModeActionResult{
		OK:      false,
		Message: loc.M("advWindowsOnly", nil),
		MessageRu: "Только Windows",
		MessageEn: "Windows only",
		State: a.GetAdvancedModeState(),
	}
}
