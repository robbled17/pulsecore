//go:build !windows

package main

func getTickCount64() uint64 { return 0 }

func queryRAMChannelHints() (single, belowRated, ok bool) { return false, false, false }

func queryGPUDriverAge() (days int, vendor string, ok bool) { return 0, "", false }

func queryGameModeFlags() (gameModeOff, gameBarBgOn, ok bool) { return false, false, false }

func applyGameModeFix(actionID string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func restoreGameModeFix(prev map[string]interface{}) error { return nil }

func (a *App) systemDiskIsHDD() bool { return false }

func (a *App) gamesLikelyOnHDD() bool { return false }
