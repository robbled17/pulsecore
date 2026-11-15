//go:build !windows

package main

// BootDiagnostics dummy structure for non-Windows platforms.
type BootDiagnostics struct {
	EventLogAvailable  bool                  `json:"eventLogAvailable"`
	BootTimeSec        float64               `json:"bootTimeSec"`
	KernelSec          float64               `json:"kernelSec"`
	DesktopSec         float64               `json:"desktopSec"`
	BootRating         string                `json:"bootRating"`
	FastStartupWarning bool                  `json:"fastStartupWarning"`
	FastStartupDays    int                   `json:"fastStartupDays"`
	HiberbootEnabled   bool                  `json:"hiberbootEnabled"`
	UptimeHours        float64               `json:"uptimeHours"`
	Degraders          []BootDegradationItem `json:"degraders"`
	StartupItems       []BootStartupItem     `json:"startupItems"`
}

type BootDegradationItem struct {
	Name     string  `json:"name"`
	TotalSec float64 `json:"totalSec"`
}

type BootStartupItem struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Command     string  `json:"command"`
	Location    string  `json:"location"`
	Status      string  `json:"status"`
	IsProtected bool    `json:"isProtected"`
	DelaySec    int     `json:"delaySec"`
	DegradeSec  float64 `json:"degradeSec"`
}

func (a *App) GetBootDiagnostics() (BootDiagnostics, error) {
	return BootDiagnostics{BootRating: "unknown"}, nil
}

func (a *App) SetStartupItemMode(id string, mode string) error {
	return nil
}

func (a *App) DisableFastStartup() error {
	return nil
}

func (a *App) RebootSystemNow() error {
	return nil
}
