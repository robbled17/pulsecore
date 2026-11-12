package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DiagnosticReport is a local exportable snapshot for support/self-help.
type DiagnosticReport struct {
	OK          bool                   `json:"ok"`
	GeneratedAt string                 `json:"generatedAt"`
	Path        string                 `json:"path"`
	SummaryKey  string                 `json:"summaryKey"`
	Sections    map[string]interface{} `json:"sections"`
}

// BuildDiagnosticReport assembles Black Box, health, weekly, startup, network into one report.
func (a *App) BuildDiagnosticReport() DiagnosticReport {
	health := a.GetSystemHealth()
	weekly := a.GetWeeklySummary()
	startup := a.AnalyzeStartup()
	netm := a.GetNetworkMonitor()
	incidents := a.GetLagIncidents()
	radar := a.GetProcessRadar()
	leaks := a.GetMemoryLeakWatch()
	intel := a.GetIntelligenceInsights()
	gm := a.GetGameModeStatus()

	sections := map[string]interface{}{
		"health":       health,
		"weekly":       weekly,
		"startup":      startup,
		"network":      netm,
		"lagIncidents": incidents,
		"radar":        radar,
		"leaks":        leaks,
		"intelligence": intel,
		"gameMode":     gm,
		"metrics":      a.GetMetrics(),
	}

	dir, err := reportDir()
	path := ""
	if err == nil {
		name := fmt.Sprintf("pulsecore-report-%s.txt", time.Now().Format("20060102-150405"))
		path = filepath.Join(dir, name)
		_ = os.WriteFile(path, []byte(renderReportText(sections)), 0o644)
	}

	return DiagnosticReport{
		OK: true, GeneratedAt: time.Now().Format(time.RFC3339),
		Path: path, SummaryKey: "diagReportReady", Sections: sections,
	}
}

func reportDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(base, ".pulsecore")
	} else {
		base = filepath.Join(base, "PulseCore", "reports")
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	return base, nil
}

func renderReportText(sections map[string]interface{}) string {
	var b strings.Builder
	b.WriteString("PulseCore Diagnostic Report\n")
	b.WriteString(time.Now().Format(time.RFC3339) + "\n\n")
	for k, v := range sections {
		b.WriteString("## " + k + "\n")
		b.WriteString(fmt.Sprintf("%+v\n\n", v))
	}
	b.WriteString("\nPrivacy: local only — not uploaded.\n")
	return b.String()
}
