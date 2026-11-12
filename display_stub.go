//go:build !windows

package main

// DisplayInfo describes one attached display (stub).
type DisplayInfo struct {
	DeviceName     string `json:"deviceName"`
	MonitorName    string `json:"monitorName"`
	AdapterName    string `json:"adapterName"`
	IsPrimary      bool   `json:"isPrimary"`
	CurrentWidth   int    `json:"currentWidth"`
	CurrentHeight  int    `json:"currentHeight"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	CurrentHz      int    `json:"currentHz"`
	MaxHz          int    `json:"maxHz"`
	MaxSupportedHz int    `json:"maxSupportedHz"`
	AvailableHz    []int  `json:"availableHz"`
	NeedsAttention bool   `json:"needsAttention"`
	Underclocked   bool   `json:"underclocked"`
}

type MonitorInfo = DisplayInfo

func (a *App) ListMonitors() []DisplayInfo { return nil }
func (a *App) ListDisplays() []DisplayInfo { return nil }

func (a *App) SetDisplayRefreshRate(deviceName string, hz int) map[string]interface{} {
	return map[string]interface{}{"ok": false, "message": "unsupported"}
}

func (a *App) TestAndSetRefreshRate(deviceName string, hz int) map[string]interface{} {
	return map[string]interface{}{"ok": false, "message": "unsupported"}
}

func (a *App) ConfirmRefreshRate(deviceName string, hz int) map[string]interface{} {
	return map[string]interface{}{"ok": false, "message": "unsupported"}
}

func (a *App) RevertRefreshRate(deviceName string) map[string]interface{} {
	return map[string]interface{}{"ok": false, "message": "unsupported"}
}

func queryDisplayRefresh() (cur, max int, ok bool) { return 0, 0, false }

func primaryUnderclockedMonitor() (DisplayInfo, bool) { return DisplayInfo{}, false }

func listMonitors() []DisplayInfo { return nil }
