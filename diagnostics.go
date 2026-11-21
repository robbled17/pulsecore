package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// Diag status codes shown in the self-diagnostics modal.
const (
	DiagStatusOK          = "ok"          // Работает
	DiagStatusFallback    = "fallback"    // Резервный расчет
	DiagStatusAdmin       = "admin"       // Нужны права администратора
	DiagStatusUnavailable = "unavailable" // Недоступно (non-Windows / missing)
)

// SelfDiagItem is one subsystem check in the self-diagnostics report.
type SelfDiagItem struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Status string   `json:"status"`
	Detail string   `json:"detail"`
	Facts  []string `json:"facts"`
}

// SelfDiagnosticsReport is the structured probe result for About → Check sensors.
type SelfDiagnosticsReport struct {
	OK          bool           `json:"ok"`
	GeneratedAt string         `json:"generatedAt"`
	Platform    string         `json:"platform"`
	RunMode     string         `json:"runMode"` // "admin" | "user"
	Items       []SelfDiagItem `json:"items"`
	TextReport  string         `json:"textReport"`
}

// diagSnapshot is a fixture-friendly inventory used by the report builder.
// Live probes fill it; unit tests pass mocks without touching WMI/registry.
type diagSnapshot struct {
	Platform string
	IsAdmin  bool

	CPUName     string
	CPUPhys     int
	CPULogical  int
	CPULoad     float64
	CPUTempC    float64
	CPUTempOK   bool
	CPULoadOK   bool
	CPUNameOK   bool

	GPUPrimary     string
	GPUIntegrated  string
	GPUVRAMMB      float64
	GPULoad        float64
	GPULoadOK      bool
	GPUPrimaryOK   bool
	GPUUsesFallback bool

	RAMTotalMB  uint64
	RAMSlotsUsed int
	RAMSlotsTotal int
	RAMSpeedMHz int
	RAMOK       bool
	RAMSlotsOK  bool

	Drives []diagDrive
	SMARTAvailable bool
	SMARTNeedsAdmin bool

	CleanRootsReachable int
	CleanRootsTotal     int
	BackupDir           string
	BackupDirOK         bool
	RegistryEnumOK      bool
}

type diagDrive struct {
	Mount  string
	TotalGB float64
	UsedGB  float64
	SMARTOK bool
	SMARTNote string
	TempC  float64
	TempOK bool
}

// RunSelfDiagnostics probes current PC subsystems and returns structured status.
// On non-Windows it returns graceful unavailable items (no panic).
func (a *App) RunSelfDiagnostics() SelfDiagnosticsReport {
	snap := collectDiagSnapshot(a)
	return buildSelfDiagnosticsReport(snap)
}

func collectDiagSnapshot(a *App) diagSnapshot {
	snap := diagSnapshot{
		Platform: runtime.GOOS,
		IsAdmin:  isProcessElevated(),
	}

	if runtime.GOOS != "windows" {
		return snap
	}

	// CPU
	snap.CPUName = strings.TrimSpace(readCPUName())
	snap.CPUNameOK = snap.CPUName != "" && !strings.EqualFold(snap.CPUName, "Unknown CPU")
	if phys, err := cpu.Counts(false); err == nil {
		snap.CPUPhys = phys
	}
	if logi, err := cpu.Counts(true); err == nil {
		snap.CPULogical = logi
	}
	if pct, err := cpu.Percent(0, false); err == nil && len(pct) > 0 {
		snap.CPULoad = pct[0]
		snap.CPULoadOK = true
	}
	temp := readCPUTemperature()
	if temp > 0 {
		snap.CPUTempC = temp
		snap.CPUTempOK = true
	}

	// GPU from live metrics / static HW when available
	if a != nil {
		a.mu.RLock()
		m := a.metrics
		static := a.staticHW
		a.mu.RUnlock()
		snap.GPUPrimary = strings.TrimSpace(m.GPUName)
		if snap.GPUPrimary == "" {
			snap.GPUPrimary = strings.TrimSpace(static.gpuName)
		}
		snap.GPUVRAMMB = m.VRAMTotalMB
		if snap.GPUVRAMMB <= 0 && static.gpuVRAM > 0 {
			snap.GPUVRAMMB = float64(static.gpuVRAM) / (1024 * 1024)
		}
		snap.GPULoad = m.GPUPercent
		snap.GPULoadOK = m.GPUAvailable && m.GPUPercent >= 0
		snap.GPUUsesFallback = !m.GPUAvailable && snap.GPUPrimary != "" && !strings.EqualFold(snap.GPUPrimary, "Unknown GPU")
		snap.GPUPrimaryOK = snap.GPUPrimary != "" && !strings.EqualFold(snap.GPUPrimary, "Unknown GPU")
		if detectIntegratedGPU(snap.GPUPrimary, snap.GPUVRAMMB) {
			snap.GPUIntegrated = snap.GPUPrimary
		} else if name, _, _ := readGPUInfoDetailed(); name != "" && detectIntegratedGPU(name, 0) {
			snap.GPUIntegrated = name
		}
	} else {
		name, vram, _ := readGPUInfoDetailed()
		snap.GPUPrimary = name
		snap.GPUVRAMMB = float64(vram) / (1024 * 1024)
		snap.GPUPrimaryOK = name != "" && !strings.EqualFold(name, "Unknown GPU")
	}

	// RAM
	if vm, err := mem.VirtualMemory(); err == nil && vm != nil {
		snap.RAMTotalMB = vm.Total / (1024 * 1024)
		snap.RAMOK = snap.RAMTotalMB > 0
	}
	ramType, used, total, speed := readRAMSlots()
	_ = ramType
	snap.RAMSlotsUsed = used
	snap.RAMSlotsTotal = total
	snap.RAMSpeedMHz = speed
	snap.RAMSlotsOK = total > 0 || used > 0

	// Drives + SMART (light probe of system volume only to stay fast)
	parts, err := disk.Partitions(false)
	if err == nil {
		for _, p := range parts {
			if isNonFixedPartition(p) {
				continue
			}
			usage, uerr := disk.Usage(p.Mountpoint)
			if uerr != nil || usage == nil {
				continue
			}
			d := diagDrive{
				Mount:   p.Mountpoint,
				TotalGB: float64(usage.Total) / (1024 * 1024 * 1024),
				UsedGB:  float64(usage.Used) / (1024 * 1024 * 1024),
			}
			letter := sanitizeDriveLetter(p.Mountpoint)
			if letter != "" && a != nil && (letter == "C" || len(snap.Drives) == 0) {
				rep := a.CheckDiskHealth(letter)
				if rep.OK {
					d.SMARTOK = true
					snap.SMARTAvailable = true
					if rep.TemperatureC > 0 {
						d.TempC = rep.TemperatureC
						d.TempOK = true
					}
					d.SMARTNote = strings.TrimSpace(rep.Verdict)
				} else {
					d.SMARTNote = strings.TrimSpace(rep.ErrorMessage)
					if d.SMARTNote == "" {
						d.SMARTNote = strings.TrimSpace(rep.Verdict)
					}
					low := strings.ToLower(d.SMARTNote)
					if strings.Contains(low, "access") || strings.Contains(low, "denied") ||
						strings.Contains(low, "admin") || strings.Contains(low, "privilege") {
						snap.SMARTNeedsAdmin = true
					}
				}
			}
			snap.Drives = append(snap.Drives, d)
			if len(snap.Drives) >= 8 {
				break
			}
		}
	}

	// Cleaner allowlist + backup dir
	roots := listAllowedCleanRoots()
	snap.CleanRootsTotal = len(roots)
	for _, r := range roots {
		if st, err := os.Stat(r.path); err == nil && st.IsDir() {
			snap.CleanRootsReachable++
		}
	}
	snap.BackupDir = pulseBackupsDir()
	if st, err := os.Stat(snap.BackupDir); err == nil && st.IsDir() {
		snap.BackupDirOK = true
	}
	snap.RegistryEnumOK = probeProgramRegistryReachable()

	return snap
}

func buildSelfDiagnosticsReport(snap diagSnapshot) SelfDiagnosticsReport {
	now := time.Now().Format(time.RFC3339)
	runMode := "user"
	if snap.IsAdmin {
		runMode = "admin"
	}

	items := make([]SelfDiagItem, 0, 6)

	// Non-Windows: all sensors unavailable, no panic.
	if snap.Platform != "" && snap.Platform != "windows" {
		items = append(items,
			SelfDiagItem{ID: "cpu", Title: "CPU", Status: DiagStatusUnavailable, Detail: "Sensors require Windows", Facts: []string{"platform=" + snap.Platform}},
			SelfDiagItem{ID: "gpu", Title: "GPU", Status: DiagStatusUnavailable, Detail: "Sensors require Windows", Facts: []string{"platform=" + snap.Platform}},
			SelfDiagItem{ID: "ram", Title: "RAM", Status: DiagStatusUnavailable, Detail: "Sensors require Windows", Facts: []string{"platform=" + snap.Platform}},
			SelfDiagItem{ID: "drives", Title: "Drives + SMART", Status: DiagStatusUnavailable, Detail: "Sensors require Windows", Facts: []string{"platform=" + snap.Platform}},
			SelfDiagItem{ID: "registry", Title: "Program registry + cleaner", Status: DiagStatusUnavailable, Detail: "Windows paths only", Facts: []string{"platform=" + snap.Platform}},
			SelfDiagItem{ID: "runmode", Title: "Run mode", Status: DiagStatusOK, Detail: "Normal user (non-Windows)", Facts: []string{"mode=user", "platform=" + snap.Platform}},
		)
		rep := SelfDiagnosticsReport{
			OK:          true,
			GeneratedAt: now,
			Platform:    snap.Platform,
			RunMode:     "user",
			Items:       items,
		}
		rep.TextReport = formatDiagTextReport(rep)
		return rep
	}

	// CPU
	{
		facts := []string{}
		if snap.CPUName != "" {
			facts = append(facts, "model="+snap.CPUName)
		}
		if snap.CPUPhys > 0 || snap.CPULogical > 0 {
			facts = append(facts, fmt.Sprintf("cores=%d/%d", snap.CPUPhys, snap.CPULogical))
		}
		if snap.CPULoadOK {
			facts = append(facts, fmt.Sprintf("load=%.1f%%", snap.CPULoad))
		}
		status := DiagStatusOK
		detail := "Working"
		if !snap.CPUNameOK && !snap.CPULoadOK {
			status = DiagStatusUnavailable
			detail = "CPU sensors unavailable"
		} else if !snap.CPUTempOK {
			if snap.IsAdmin {
				status = DiagStatusFallback
				detail = "Fallback calculation (ACPI temp unavailable)"
			} else {
				status = DiagStatusAdmin
				detail = "Needs administrator rights for ACPI temperature"
			}
			facts = append(facts, "temp=unavailable")
		} else {
			facts = append(facts, fmt.Sprintf("temp=%.0fC", snap.CPUTempC))
		}
		items = append(items, SelfDiagItem{ID: "cpu", Title: "CPU", Status: status, Detail: detail, Facts: facts})
	}

	// GPU
	{
		facts := []string{}
		if snap.GPUPrimary != "" {
			facts = append(facts, "primary="+snap.GPUPrimary)
		}
		if snap.GPUIntegrated != "" && snap.GPUIntegrated != snap.GPUPrimary {
			facts = append(facts, "integrated="+snap.GPUIntegrated)
		}
		if snap.GPUVRAMMB > 0 {
			facts = append(facts, fmt.Sprintf("vram=%.0fMB", snap.GPUVRAMMB))
		}
		status := DiagStatusOK
		detail := "Working"
		if !snap.GPUPrimaryOK {
			status = DiagStatusUnavailable
			detail = "GPU not detected"
		} else if snap.GPUUsesFallback || !snap.GPULoadOK {
			status = DiagStatusFallback
			detail = "Fallback calculation (vendor load counter missing)"
			facts = append(facts, "load=fallback")
		} else {
			facts = append(facts, fmt.Sprintf("load=%.1f%%", snap.GPULoad))
		}
		items = append(items, SelfDiagItem{ID: "gpu", Title: "GPU", Status: status, Detail: detail, Facts: facts})
	}

	// RAM
	{
		facts := []string{}
		if snap.RAMTotalMB > 0 {
			facts = append(facts, fmt.Sprintf("capacity=%dMB", snap.RAMTotalMB))
		}
		if snap.RAMSlotsTotal > 0 || snap.RAMSlotsUsed > 0 {
			facts = append(facts, fmt.Sprintf("slots=%d/%d", snap.RAMSlotsUsed, snap.RAMSlotsTotal))
		}
		if snap.RAMSpeedMHz > 0 {
			facts = append(facts, fmt.Sprintf("freq=%dMHz", snap.RAMSpeedMHz))
		}
		status := DiagStatusOK
		detail := "Working"
		if !snap.RAMOK {
			status = DiagStatusUnavailable
			detail = "RAM capacity unavailable"
		} else if !snap.RAMSlotsOK {
			status = DiagStatusFallback
			detail = "Fallback calculation (SMBIOS slots unavailable)"
		}
		items = append(items, SelfDiagItem{ID: "ram", Title: "RAM", Status: status, Detail: detail, Facts: facts})
	}

	// Drives + SMART
	{
		facts := []string{}
		for _, d := range snap.Drives {
			line := fmt.Sprintf("%s %.0f/%.0fGB", d.Mount, d.UsedGB, d.TotalGB)
			if d.TempOK {
				line += fmt.Sprintf(" temp=%.0fC", d.TempC)
			}
			if d.SMARTNote != "" {
				line += " smart=" + trimDiagNote(d.SMARTNote)
			}
			facts = append(facts, line)
		}
		status := DiagStatusOK
		detail := "Working"
		if len(snap.Drives) == 0 {
			status = DiagStatusUnavailable
			detail = "No fixed drives listed"
		} else if snap.SMARTNeedsAdmin && !snap.SMARTAvailable {
			status = DiagStatusAdmin
			detail = "Needs administrator rights for SMART"
		} else if !snap.SMARTAvailable {
			status = DiagStatusFallback
			detail = "Fallback calculation (SMART counters unavailable)"
		}
		items = append(items, SelfDiagItem{ID: "drives", Title: "Drives + SMART", Status: status, Detail: detail, Facts: facts})
	}

	// Program registry + cleaner allowlist
	{
		facts := []string{
			fmt.Sprintf("cleaner_roots=%d/%d", snap.CleanRootsReachable, snap.CleanRootsTotal),
		}
		if snap.BackupDir != "" {
			facts = append(facts, "backups="+snap.BackupDir)
		}
		status := DiagStatusOK
		detail := "Working"
		if !snap.RegistryEnumOK && snap.CleanRootsReachable == 0 {
			status = DiagStatusUnavailable
			detail = "Registry and cleaner paths unreachable"
		} else if !snap.RegistryEnumOK || !snap.BackupDirOK || snap.CleanRootsReachable < snap.CleanRootsTotal {
			status = DiagStatusFallback
			detail = "Fallback calculation (some paths unreachable)"
			if !snap.RegistryEnumOK {
				facts = append(facts, "registry=limited")
			}
			if !snap.BackupDirOK {
				facts = append(facts, "backups=missing")
			}
		} else {
			facts = append(facts, "registry=ok")
		}
		items = append(items, SelfDiagItem{ID: "registry", Title: "Program registry + cleaner", Status: status, Detail: detail, Facts: facts})
	}

	// Run mode
	{
		detail := "Normal user"
		facts := []string{"mode=user"}
		if snap.IsAdmin {
			detail = "Administrator"
			facts = []string{"mode=admin"}
		}
		items = append(items, SelfDiagItem{
			ID:     "runmode",
			Title:  "Run mode",
			Status: DiagStatusOK,
			Detail: detail,
			Facts:  facts,
		})
	}

	rep := SelfDiagnosticsReport{
		OK:          true,
		GeneratedAt: now,
		Platform:    "windows",
		RunMode:     runMode,
		Items:       items,
	}
	rep.TextReport = formatDiagTextReport(rep)
	return rep
}

func formatDiagTextReport(rep SelfDiagnosticsReport) string {
	var b strings.Builder
	b.WriteString("PulseCore self-diagnostics\n")
	b.WriteString("generated: " + rep.GeneratedAt + "\n")
	b.WriteString("platform: " + rep.Platform + "\n")
	b.WriteString("run_mode: " + rep.RunMode + "\n")
	for _, it := range rep.Items {
		b.WriteString("- " + it.ID + " [" + it.Status + "] " + it.Detail)
		if len(it.Facts) > 0 {
			b.WriteString(" | " + strings.Join(it.Facts, "; "))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func trimDiagNote(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > 80 {
		return s[:77] + "..."
	}
	return s
}

func probeProgramRegistryReachable() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	// Soft probe: Uninstall keys are readable for current user without admin.
	// Avoid inventing app lists; just confirm the hive opens.
	return openUninstallKeyReadable()
}

func pulseLocalAppDataDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		if cfg, err := os.UserConfigDir(); err == nil {
			base = cfg
		} else {
			base = os.TempDir()
		}
	}
	return filepath.Join(base, "PulseCore")
}
