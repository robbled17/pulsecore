package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// StartupEntry is a Windows Run-key / Startup-folder item.
type StartupEntry struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	Location string `json:"location"` // hkcu | hklm | folder
	Heavy    bool   `json:"heavy"`
	Impact   string `json:"impact"` // low | medium | high
}

// StartupAnalysis summarizes boot impact.
type StartupAnalysis struct {
	OK         bool           `json:"ok"`
	Entries    []StartupEntry `json:"entries"`
	HeavyCount int            `json:"heavyCount"`
	Total      int            `json:"total"`
	SummaryKey string         `json:"summaryKey"`
	NoteKey    string         `json:"noteKey"`
}

var heavyStartupHints = []string{
	"teams", "slack", "discord", "steam", "epic", "adobe", "spotify",
	"onedrive", "dropbox", "chrome", "msedge", "skype", "zoom",
	"nvidia", "corsair", "logitech", "razer", "overwolf",
}

func startupHeavy(name, cmd string) (bool, string) {
	blob := strings.ToLower(name + " " + cmd)
	for _, h := range heavyStartupHints {
		if strings.Contains(blob, h) {
			return true, "high"
		}
	}
	if strings.Contains(blob, "update") || strings.Contains(blob, "helper") {
		return false, "medium"
	}
	return false, "low"
}

func readRunKey(root registry.Key, path, loc string) []StartupEntry {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	names, err := k.ReadValueNames(0)
	if err != nil {
		return nil
	}
	out := make([]StartupEntry, 0, len(names))
	for _, n := range names {
		val, _, err := k.GetStringValue(n)
		if err != nil {
			continue
		}
		heavy, impact := startupHeavy(n, val)
		out = append(out, StartupEntry{
			Name: n, Command: val, Location: loc, Heavy: heavy, Impact: impact,
		})
	}
	return out
}

func readStartupFolder() []StartupEntry {
	dir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	folder := filepath.Join(dir, "AppData", "Roaming", "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil
	}
	out := make([]StartupEntry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		heavy, impact := startupHeavy(name, name)
		out = append(out, StartupEntry{
			Name: name, Command: filepath.Join(folder, name), Location: "folder", Heavy: heavy, Impact: impact,
		})
	}
	return out
}

// GetStartupEntries lists Run-key and Startup-folder items.
func (a *App) GetStartupEntries() []StartupEntry {
	out := make([]StartupEntry, 0, 32)
	out = append(out, readRunKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, "hkcu")...)
	out = append(out, readRunKey(registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Run`, "hklm")...)
	out = append(out, readStartupFolder()...)
	return out
}

// AnalyzeStartup returns impact summary for Settings UI.
func (a *App) AnalyzeStartup() StartupAnalysis {
	entries := a.GetStartupEntries()
	heavy := 0
	for _, e := range entries {
		if e.Heavy {
			heavy++
		}
	}
	sum := "startupOK"
	if heavy >= 3 {
		sum = "startupHeavy"
	} else if len(entries) >= 8 {
		sum = "startupBusy"
	}
	return StartupAnalysis{
		OK: true, Entries: entries, HeavyCount: heavy, Total: len(entries),
		SummaryKey: sum, NoteKey: "startupNote",
	}
}

const pulseAutostartName = "PulseCore"

// SetAutostartEnabled writes/deletes HKCU Run key for PulseCore.
func (a *App) SetAutostartEnabled(enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !enable {
		_ = k.DeleteValue(pulseAutostartName)
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return k.SetStringValue(pulseAutostartName, fmt.Sprintf("\"%s\"", exe))
}

// IsAutostartEnabled reports whether PulseCore is in HKCU Run.
func (a *App) IsAutostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(pulseAutostartName)
	return err == nil
}
