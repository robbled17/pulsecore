package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// DevCatalogItem describes a CPU or GPU model preset for Dev Mode.
type DevCatalogItem struct {
	Name         string  `json:"name"`
	Tier         string  `json:"tier"` // "entry", "mid", "high", "top", "igpu", "budget"
	Cores        int     `json:"cores,omitempty"`
	Threads      int     `json:"threads,omitempty"`
	FreqMHz      float64 `json:"freqMHz,omitempty"`
	VRAMGB       float64 `json:"vramGB,omitempty"`
	IsIntegrated bool    `json:"isIntegrated,omitempty"`
	Score        float64 `json:"score"`
}

// DevHardwareCatalog contains the catalog of ~20 CPUs and ~20 GPUs.
type DevHardwareCatalog struct {
	CPUs []DevCatalogItem `json:"cpus"`
	GPUs []DevCatalogItem `json:"gpus"`
}

// DevSensorsState holds current simulated sensor readings.
type DevSensorsState struct {
	CPUPercent  float64 `json:"cpuPercent"`
	CPUTempC    float64 `json:"cpuTempC"`
	RAMTotalGB  int     `json:"ramTotalGB"`
	RAMUsedPct  float64 `json:"ramUsedPct"`
	RAMUsedGB   float64 `json:"ramUsedGB"`
	GPUPercent  float64 `json:"gpuPercent"`
	VRAMUsedPct float64 `json:"vramUsedPct"`
	VRAMUsedGB  float64 `json:"vramUsedGB"`
	GPUTempC    float64 `json:"gpuTempC"`
	UptimeHours float64 `json:"uptimeHours"`
}

// DevHardwareState holds current virtual PC configuration.
type DevHardwareState struct {
	CPUName        string  `json:"cpuName"`
	CPUCores       int     `json:"cpuCores"`
	CPUThreads     int     `json:"cpuThreads"`
	CPUFreqMHz     float64 `json:"cpuFreqMHz"`
	CPUScore       float64 `json:"cpuScore"`
	GPUName        string  `json:"gpuName"`
	GPUVRAMGB      float64 `json:"gpuVRAMGB"`
	GPUIsIGPU      bool    `json:"gpuIsIGPU"`
	GPUScore       float64 `json:"gpuScore"`
	RAMTotalGB     int     `json:"ramTotalGB"`
	IsLaptop       bool    `json:"isLaptop"`
	BatteryPercent float64 `json:"batteryPercent"`
	BatteryPlugged bool    `json:"batteryPlugged"`
	BatteryWearPct float64 `json:"batteryWearPct"`
}

// DevModeState is the complete payload sent to frontend UI.
type DevModeState struct {
	DevMode   bool            `json:"devMode"`
	DryRun    bool            `json:"dryRun"`
	Sensors   DevSensorsState `json:"sensors"`
	Hardware  DevHardwareState `json:"hardware"`
	Disks     []SimulatedDisk `json:"disks"`
	Vampires  []VampireProcess `json:"vampires"`
}

// RawTelemetryItem is one row in the Raw Telemetry Inspector table.
type RawTelemetryItem struct {
	Metric  string `json:"metric"`
	Value   string `json:"value"`
	Source  string `json:"source"`
	Updated string `json:"updated"`
}

// UnlockDryRunResult is the feedback for testing Unlocker in Dry-Run mode.
type UnlockDryRunResult struct {
	OK           bool     `json:"ok"`
	DryRun       bool     `json:"dryRun"`
	Path         string   `json:"path"`
	ProcessCount int      `json:"processCount"`
	Processes    []string `json:"processes"`
	MessageRu    string   `json:"messageRu"`
	MessageEn    string   `json:"messageEn"`
}

var defaultDevHardwareCatalog = DevHardwareCatalog{
	CPUs: []DevCatalogItem{
		// Entry level (5)
		{Name: "Intel Celeron G5905", Tier: "entry", Cores: 2, Threads: 2, FreqMHz: 3500, Score: 24},
		{Name: "Intel Core i3-4170", Tier: "entry", Cores: 2, Threads: 4, FreqMHz: 3700, Score: 32},
		{Name: "AMD Athlon 3000G", Tier: "entry", Cores: 2, Threads: 4, FreqMHz: 3500, Score: 30},
		{Name: "Intel Core i5-4590", Tier: "entry", Cores: 4, Threads: 4, FreqMHz: 3300, Score: 40},
		{Name: "Intel Core i3-10100F", Tier: "entry", Cores: 4, Threads: 8, FreqMHz: 3600, Score: 52},
		// Mid class (7)
		{Name: "AMD Ryzen 5 2600", Tier: "mid", Cores: 6, Threads: 12, FreqMHz: 3400, Score: 58},
		{Name: "Intel Core i5-10400F", Tier: "mid", Cores: 6, Threads: 12, FreqMHz: 2900, Score: 62},
		{Name: "AMD Ryzen 5 3600", Tier: "mid", Cores: 6, Threads: 12, FreqMHz: 3600, Score: 65},
		{Name: "Intel Core i5-11400F", Tier: "mid", Cores: 6, Threads: 12, FreqMHz: 2600, Score: 68},
		{Name: "AMD Ryzen 5 5600", Tier: "mid", Cores: 6, Threads: 12, FreqMHz: 3500, Score: 75},
		{Name: "Intel Core i5-12400F", Tier: "mid", Cores: 6, Threads: 12, FreqMHz: 2500, Score: 78},
		{Name: "AMD Ryzen 7 5700X3D", Tier: "mid", Cores: 8, Threads: 16, FreqMHz: 3000, Score: 84},
		// High & Flagship (8)
		{Name: "Intel Core i5-13600KF", Tier: "high", Cores: 14, Threads: 20, FreqMHz: 3500, Score: 88},
		{Name: "AMD Ryzen 7 7700X", Tier: "high", Cores: 8, Threads: 16, FreqMHz: 4500, Score: 89},
		{Name: "Intel Core i7-13700K", Tier: "high", Cores: 16, Threads: 24, FreqMHz: 3400, Score: 92},
		{Name: "AMD Ryzen 7 7800X3D", Tier: "high", Cores: 8, Threads: 16, FreqMHz: 4200, Score: 96},
		{Name: "Intel Core i9-13900K", Tier: "high", Cores: 24, Threads: 32, FreqMHz: 3000, Score: 96},
		{Name: "AMD Ryzen 9 7950X3D", Tier: "high", Cores: 16, Threads: 32, FreqMHz: 4200, Score: 98},
		{Name: "Intel Core i9-14900K", Tier: "high", Cores: 24, Threads: 32, FreqMHz: 3200, Score: 98},
		{Name: "AMD Ryzen 7 9800X3D", Tier: "high", Cores: 8, Threads: 16, FreqMHz: 4700, Score: 100},
	},
	GPUs: []DevCatalogItem{
		// iGPU & Basic (4)
		{Name: "Intel UHD Graphics 630 (1 ГБ)", Tier: "igpu", VRAMGB: 1, IsIntegrated: true, Score: 12},
		{Name: "AMD Radeon Vega 8 (2 ГБ)", Tier: "igpu", VRAMGB: 2, IsIntegrated: true, Score: 22},
		{Name: "NVIDIA GT 1030 (2 ГБ)", Tier: "igpu", VRAMGB: 2, IsIntegrated: false, Score: 20},
		{Name: "NVIDIA GTX 750 Ti (2 ГБ)", Tier: "igpu", VRAMGB: 2, IsIntegrated: false, Score: 25},
		// Budget Gaming (6)
		{Name: "NVIDIA GTX 1050 Ti (4 ГБ)", Tier: "budget", VRAMGB: 4, IsIntegrated: false, Score: 30},
		{Name: "AMD RX 570 (4 ГБ)", Tier: "budget", VRAMGB: 4, IsIntegrated: false, Score: 38},
		{Name: "NVIDIA GTX 1060 (6 ГБ)", Tier: "budget", VRAMGB: 6, IsIntegrated: false, Score: 40},
		{Name: "NVIDIA GTX 1650 (4 ГБ)", Tier: "budget", VRAMGB: 4, IsIntegrated: false, Score: 35},
		{Name: "NVIDIA GTX 1660 Super (6 ГБ)", Tier: "budget", VRAMGB: 6, IsIntegrated: false, Score: 46},
		{Name: "AMD RX 6600 (8 ГБ)", Tier: "budget", VRAMGB: 8, IsIntegrated: false, Score: 62},
		// Mid Class (6)
		{Name: "NVIDIA RTX 2060 Super (8 ГБ)", Tier: "mid", VRAMGB: 8, IsIntegrated: false, Score: 72},
		{Name: "NVIDIA RTX 3060 (12 ГБ)", Tier: "mid", VRAMGB: 12, IsIntegrated: false, Score: 60},
		{Name: "AMD RX 6700 XT (12 ГБ)", Tier: "mid", VRAMGB: 12, IsIntegrated: false, Score: 72},
		{Name: "NVIDIA RTX 4060 (8 ГБ)", Tier: "mid", VRAMGB: 8, IsIntegrated: false, Score: 65},
		{Name: "NVIDIA RTX 3070 (8 ГБ)", Tier: "mid", VRAMGB: 8, IsIntegrated: false, Score: 76},
		{Name: "NVIDIA RTX 4070 Super (12 ГБ)", Tier: "mid", VRAMGB: 12, IsIntegrated: false, Score: 87},
		// Top & Flagship (4)
		{Name: "AMD RX 7900 XT (20 ГБ)", Tier: "top", VRAMGB: 20, IsIntegrated: false, Score: 92},
		{Name: "NVIDIA RTX 4080 Super (16 ГБ)", Tier: "top", VRAMGB: 16, IsIntegrated: false, Score: 95},
		{Name: "AMD RX 7900 XTX (24 ГБ)", Tier: "top", VRAMGB: 24, IsIntegrated: false, Score: 96},
		{Name: "NVIDIA RTX 4090 (24 ГБ)", Tier: "top", VRAMGB: 24, IsIntegrated: false, Score: 100},
	},
}

// GetDevHardwareCatalog returns the reference catalog of 20 CPUs and 20 GPUs.
func (a *App) GetDevHardwareCatalog() DevHardwareCatalog {
	return defaultDevHardwareCatalog
}

// IsDevMode returns whether Dev Mode is currently enabled.
func (a *App) IsDevMode() bool {
	if a == nil {
		return false
	}
	a.devMu.RLock()
	defer a.devMu.RUnlock()
	return a.devMode
}

// IsDryRun returns whether safe Dry-Run mode is active.
func (a *App) IsDryRun() bool {
	if a == nil {
		return false
	}
	a.devMu.RLock()
	defer a.devMu.RUnlock()
	return a.devMode && a.dryRun
}

// SetDryRun toggles the safe Dry-Run testing mode.
func (a *App) SetDryRun(enabled bool) {
	if a == nil {
		return
	}
	a.devMu.Lock()
	a.dryRun = enabled
	a.devMu.Unlock()
}

// SetDevMode enables or disables Dev Mode.
// When disabled, all fake values are immediately reset and real telemetry resumes.
func (a *App) SetDevMode(enabled bool) DevModeState {
	if a == nil {
		return DevModeState{}
	}

	a.devMu.Lock()
	a.devMode = enabled
	if enabled {
		a.dryRun = true // Safe testing by default
		if a.simProvider == nil {
			a.simProvider = NewSimulatedMetricsProvider()
		}
		a.metricsProvider = a.simProvider
	} else {
		a.dryRun = false
		if a.realProvider == nil {
			a.realProvider = NewRealMetricsProvider(a)
		}
		a.metricsProvider = a.realProvider
		// Restore real hardware names
		a.mu.Lock()
		if a.staticHW.cpuName != "" {
			a.cpuName = a.staticHW.cpuName
		}
		if a.staticHW.gpuName != "" {
			a.gpuName = a.staticHW.gpuName
			a.gpuTotalVRAM = a.staticHW.gpuVRAM
		}
		a.mu.Unlock()
	}
	a.devMu.Unlock()

	// Immediately collect to synchronize frontend snapshot
	a.collectOnce()

	return a.GetDevModeState()
}

// GetDevModeState returns the current simulator state.
func (a *App) GetDevModeState() DevModeState {
	if a == nil {
		return DevModeState{}
	}

	a.devMu.RLock()
	devOn := a.devMode
	dryRun := a.dryRun
	sim := a.simProvider
	a.devMu.RUnlock()

	if sim == nil {
		sim = NewSimulatedMetricsProvider()
	}

	sim.mu.RLock()
	defer sim.mu.RUnlock()

	ramTotGB := int(sim.ramTotalMB / 1024)
	if ramTotGB <= 0 {
		ramTotGB = 16
	}
	ramUsedPct := 0.0
	if sim.ramTotalMB > 0 {
		ramUsedPct = (float64(sim.ramUsedMB) / float64(sim.ramTotalMB)) * 100.0
	}
	ramUsedGB := float64(sim.ramUsedMB) / 1024.0

	vramTotMB := sim.vramTotalMB
	if vramTotMB <= 0 {
		vramTotMB = sim.gpuVRAMMB
	}
	vramUsedPct := 0.0
	if vramTotMB > 0 {
		vramUsedPct = (sim.vramUsedMB / vramTotMB) * 100.0
	}
	vramUsedGB := sim.vramUsedMB / 1024.0
	gpuVRAMGB := sim.gpuVRAMMB / 1024.0

	return DevModeState{
		DevMode: devOn,
		DryRun:  dryRun,
		Sensors: DevSensorsState{
			CPUPercent:  sim.cpuPercent,
			CPUTempC:    sim.cpuTemp,
			RAMTotalGB:  ramTotGB,
			RAMUsedPct:  ramUsedPct,
			RAMUsedGB:   ramUsedGB,
			GPUPercent:  sim.gpuPercent,
			VRAMUsedPct: vramUsedPct,
			VRAMUsedGB:  vramUsedGB,
			GPUTempC:    sim.gpuTemp,
			UptimeHours: sim.uptimeDuration.Hours(),
		},
		Hardware: DevHardwareState{
			CPUName:        sim.cpuName,
			CPUCores:       sim.physicalCores,
			CPUThreads:     sim.logicalCores,
			CPUFreqMHz:     sim.freqMHz,
			CPUScore:       sim.cpuScore,
			GPUName:        sim.gpuName,
			GPUVRAMGB:      gpuVRAMGB,
			GPUIsIGPU:      sim.isIntegrated,
			GPUScore:       sim.gpuScore,
			RAMTotalGB:     ramTotGB,
			IsLaptop:       sim.isLaptop,
			BatteryPercent: sim.batteryPercent,
			BatteryPlugged: sim.pluggedIn,
			BatteryWearPct: sim.wearPercent,
		},
		Disks:    append([]SimulatedDisk(nil), sim.disks...),
		Vampires: append([]VampireProcess(nil), sim.vampires...),
	}
}

// SetSimulatedSensors updates the real-time sensor simulator sliders.
func (a *App) SetSimulatedSensors(cpuPct, cpuTemp, ramTotalGB, ramUsedPct, gpuPct, vramUsedPct, gpuTemp, uptimeHours float64) DevModeState {
	if a == nil {
		return DevModeState{}
	}

	a.devMu.Lock()
	if a.simProvider == nil {
		a.simProvider = NewSimulatedMetricsProvider()
	}
	sim := a.simProvider
	a.devMu.Unlock()

	sim.mu.Lock()
	if cpuPct < 0 {
		cpuPct = 0
	}
	if cpuPct > 100 {
		cpuPct = 100
	}
	sim.cpuPercent = cpuPct

	cores := sim.logicalCores
	if cores <= 0 {
		cores = 8
	}
	sim.cpuPerCore = make([]float64, cores)
	for i := range sim.cpuPerCore {
		sim.cpuPerCore[i] = cpuPct
	}

	if cpuTemp < 30 {
		cpuTemp = 30
	}
	if cpuTemp > 105 {
		cpuTemp = 105
	}
	sim.cpuTemp = cpuTemp
	sim.cpuTempAvail = true

	if ramTotalGB < 4 {
		ramTotalGB = 4
	}
	sim.ramTotalMB = uint64(ramTotalGB * 1024)

	if ramUsedPct < 0 {
		ramUsedPct = 0
	}
	if ramUsedPct > 100 {
		ramUsedPct = 100
	}
	sim.ramUsedMB = uint64(float64(sim.ramTotalMB) * (ramUsedPct / 100.0))

	if gpuPct < 0 {
		gpuPct = 0
	}
	if gpuPct > 100 {
		gpuPct = 100
	}
	sim.gpuPercent = gpuPct

	if vramUsedPct < 0 {
		vramUsedPct = 0
	}
	if vramUsedPct > 100 {
		vramUsedPct = 100
	}
	sim.vramUsedMB = sim.gpuVRAMMB * (vramUsedPct / 100.0)

	if gpuTemp < 30 {
		gpuTemp = 30
	}
	if gpuTemp > 105 {
		gpuTemp = 105
	}
	sim.gpuTemp = gpuTemp
	sim.gpuTempAvail = true

	if uptimeHours < 0 {
		uptimeHours = 0
	}
	sim.uptimeDuration = time.Duration(uptimeHours * float64(time.Hour))
	sim.mu.Unlock()

	a.collectOnce()
	return a.GetDevModeState()
}

// SetSimulatedHardware updates the CPU, GPU, RAM, and Battery virtual hardware.
func (a *App) SetSimulatedHardware(
	cpuName string, cores, threads int, freqMHz, cpuScore float64,
	gpuName string, vramGB float64, isIntegrated bool, gpuScore float64,
	ramTotalGB int,
) DevModeState {
	if a == nil {
		return DevModeState{}
	}

	a.devMu.Lock()
	if a.simProvider == nil {
		a.simProvider = NewSimulatedMetricsProvider()
	}
	sim := a.simProvider
	a.devMu.Unlock()

	sim.mu.Lock()
	if strings.TrimSpace(cpuName) != "" {
		sim.cpuName = strings.TrimSpace(cpuName)
	}
	if cores > 0 {
		sim.physicalCores = cores
	}
	if threads > 0 {
		sim.logicalCores = threads
	}
	if freqMHz > 0 {
		sim.freqMHz = freqMHz
	}
	if cpuScore > 0 {
		sim.cpuScore = cpuScore
	} else {
		sim.cpuScore = calcCPUPowerScore(sim.cpuName, sim.physicalCores, sim.logicalCores, sim.freqMHz)
	}

	if strings.TrimSpace(gpuName) != "" {
		sim.gpuName = strings.TrimSpace(gpuName)
	}
	if vramGB > 0 {
		sim.gpuVRAMMB = vramGB * 1024.0
		sim.vramTotalMB = sim.gpuVRAMMB
	}
	sim.isIntegrated = isIntegrated
	if gpuScore > 0 {
		sim.gpuScore = gpuScore
	} else {
		sim.gpuScore = calcGPUPowerScore(sim.gpuName, isIntegrated, vramGB)
	}

	if ramTotalGB >= 4 {
		sim.ramTotalMB = uint64(ramTotalGB * 1024)
		if sim.ramUsedMB > sim.ramTotalMB {
			sim.ramUsedMB = uint64(float64(sim.ramTotalMB) * 0.5)
		}
	}
	sim.mu.Unlock()

	// Update App live references so Home and GamePulse see new simulated hardware
	a.mu.Lock()
	a.cpuName = sim.cpuName
	a.gpuName = sim.gpuName
	a.gpuTotalVRAM = uint64(sim.gpuVRAMMB * 1024 * 1024)
	a.mu.Unlock()

	a.collectOnce()
	return a.GetDevModeState()
}

// SetSimulatedDisks replaces the virtual disks list.
func (a *App) SetSimulatedDisks(disks []SimulatedDisk) DevModeState {
	if a == nil {
		return DevModeState{}
	}

	a.devMu.Lock()
	if a.simProvider == nil {
		a.simProvider = NewSimulatedMetricsProvider()
	}
	sim := a.simProvider
	a.devMu.Unlock()

	sim.mu.Lock()
	if len(disks) == 0 {
		// Keep at least one valid system volume
		sim.disks = []SimulatedDisk{
			{Letter: "C", Type: "NVMe SSD", Model: "Samsung SSD 980 PRO 1TB", TotalGB: 1024, UsedPct: 35, HealthPct: 100, TempC: 34},
		}
	} else {
		sim.disks = make([]SimulatedDisk, len(disks))
		for i, d := range disks {
			l := strings.ToUpper(strings.TrimSpace(d.Letter))
			if l == "" {
				l = fmt.Sprintf("%c", 'C'+i)
			}
			tot := d.TotalGB
			if tot <= 0 {
				tot = 512
			}
			used := d.UsedPct
			if used < 0 {
				used = 0
			}
			if used > 99 {
				used = 99
			}
			h := d.HealthPct
			if h < 1 {
				h = 1
			}
			if h > 100 {
				h = 100
			}
			t := d.TempC
			if t < 20 {
				t = 20
			}
			if t > 85 {
				t = 85
			}
			m := strings.TrimSpace(d.Model)
			if m == "" {
				if strings.EqualFold(d.Type, "HDD") {
					m = "WDC WD10EZEX-00WN4A0"
				} else if strings.Contains(strings.ToUpper(d.Type), "NVME") {
					m = "Samsung SSD 980 PRO"
				} else {
					m = "Crucial MX500 SSD"
				}
			}
			sim.disks[i] = SimulatedDisk{
				Letter:    l,
				Type:      d.Type,
				Model:     m,
				TotalGB:   tot,
				UsedPct:   used,
				HealthPct: h,
				TempC:     t,
			}
		}
	}
	sim.mu.Unlock()

	a.collectOnce()
	return a.GetDevModeState()
}

// SetSimulatedBattery configures laptop battery presence, state, and wear.
func (a *App) SetSimulatedBattery(isLaptop bool, percent float64, pluggedIn bool, wearPercent float64) DevModeState {
	if a == nil {
		return DevModeState{}
	}

	a.devMu.Lock()
	if a.simProvider == nil {
		a.simProvider = NewSimulatedMetricsProvider()
	}
	sim := a.simProvider
	a.devMu.Unlock()

	sim.mu.Lock()
	sim.isLaptop = isLaptop
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	sim.batteryPercent = percent
	sim.pluggedIn = pluggedIn
	if wearPercent < 0 {
		wearPercent = 0
	}
	if wearPercent > 80 {
		wearPercent = 80
	}
	sim.wearPercent = wearPercent
	if pluggedIn {
		sim.powerDrawWatts = 35.0
	} else {
		sim.powerDrawWatts = 18.5
	}
	sim.mu.Unlock()

	a.collectOnce()
	return a.GetDevModeState()
}

// ApplyDevPreset activates one of the pre-configured quick scenarios.
func (a *App) ApplyDevPreset(presetName string) DevModeState {
	if a == nil {
		return DevModeState{}
	}

	a.devMu.Lock()
	if a.simProvider == nil {
		a.simProvider = NewSimulatedMetricsProvider()
	}
	sim := a.simProvider
	a.devMu.Unlock()

	sim.mu.Lock()
	switch strings.ToLower(strings.TrimSpace(presetName)) {
	case "all_good", "good", "все хорошо", "всё хорошо":
		// Ideal state: 100 health score
		sim.cpuPercent = 5.0
		sim.cpuTemp = 40.0
		sim.ramTotalMB = 16 * 1024
		sim.ramUsedMB = 4 * 1024 // 25%
		sim.gpuPercent = 2.0
		sim.vramUsedMB = 1024.0
		sim.gpuTemp = 36.0
		sim.uptimeDuration = 2 * time.Hour
		sim.disks = []SimulatedDisk{
			{Letter: "C", Type: "NVMe SSD", Model: "Samsung SSD 980 PRO 1TB", TotalGB: 1024, UsedPct: 30.0, HealthPct: 100.0, TempC: 32.0},
		}
		sim.vampires = nil

	case "max_load", "ceiling", "потолок cpu / ram / gpu", "потолок":
		// Resource ceiling: 98-100% across all resources
		sim.cpuPercent = 99.0
		sim.cpuTemp = 89.0
		sim.ramTotalMB = 16 * 1024
		sim.ramUsedMB = uint64(float64(sim.ramTotalMB) * 0.99)
		sim.gpuPercent = 99.0
		sim.vramUsedMB = sim.gpuVRAMMB * 0.98
		sim.gpuTemp = 87.0
		sim.uptimeDuration = 6 * time.Hour
		sim.disks = []SimulatedDisk{
			{Letter: "C", Type: "NVMe SSD", Model: "Samsung SSD 980 PRO 1TB", TotalGB: 1024, UsedPct: 88.0, HealthPct: 98.0, TempC: 48.0},
		}

	case "disk_full", "диск почти полон":
		// System disk 96% full
		sim.cpuPercent = 14.0
		sim.cpuTemp = 46.0
		sim.ramTotalMB = 16 * 1024
		sim.ramUsedMB = 6 * 1024
		sim.disks = []SimulatedDisk{
			{Letter: "C", Type: "NVMe SSD", Model: "Samsung SSD 980 PRO 480GB", TotalGB: 480, UsedPct: 96.0, HealthPct: 95.0, TempC: 36.0},
		}

	case "overheat", "перегрев":
		// CPU 94°C, GPU 91°C
		sim.cpuPercent = 78.0
		sim.cpuTemp = 94.0
		sim.gpuPercent = 82.0
		sim.gpuTemp = 91.0

	case "dying_ssd", "умирающий ssd":
		// SMART health 15%, critical wear
		sim.disks = []SimulatedDisk{
			{Letter: "C", Type: "SATA SSD", Model: "Crucial CT480BX500SSD1", TotalGB: 480, UsedPct: 45.0, HealthPct: 15.0, TempC: 62.0},
		}

	case "vampire_hog", "вампир нагружает cpu", "вампир":
		// Background resource hog process in Lag Spotter
		sim.cpuPercent = 72.0
		sim.cpuTemp = 65.0
		sim.vampires = []VampireProcess{
			{
				PID:          8844,
				PIDs:         []int32{8844},
				Count:        1,
				Name:         "crypto_miner_fake.exe",
				FriendlyName: "Скрытый майнер / Фоновый процесс (Симуляция)",
				CPUPercent:   65.0,
				DiskMBps:     12.4,
				Score:        85.0,
				Frozen:       false,
				ReasonRu:     "Высокая фоновая нагрузка на процессор",
				ReasonEn:     "High background CPU load",
			},
		}

	case "long_uptime", "долгий аптайм":
		// 14 days without reboot
		sim.uptimeDuration = 14 * 24 * time.Hour
	}

	sim.mu.Unlock()
	a.collectOnce()
	return a.GetDevModeState()
}

// ResetDevMode restores default simulated values.
func (a *App) ResetDevMode() DevModeState {
	if a == nil {
		return DevModeState{}
	}

	a.devMu.Lock()
	a.simProvider = NewSimulatedMetricsProvider()
	a.devMu.Unlock()

	a.collectOnce()
	return a.GetDevModeState()
}

// GetRawTelemetry returns the current values from all hardware/sensors with their sources.
func (a *App) GetRawTelemetry() []RawTelemetryItem {
	if a == nil {
		return nil
	}

	nowStr := time.Now().Format("15:04:05")
	isDev := a.IsDevMode()
	sourcePrefix := "Real"
	if isDev {
		sourcePrefix = "Simulated"
	}

	a.mu.RLock()
	m := a.metrics
	cpuN := a.cpuName
	gpuN := a.gpuName
	a.mu.RUnlock()

	var items []RawTelemetryItem

	// 1. CPU Load
	cpuSrc := sourcePrefix + " (NtQuerySystemInformation)"
	if isDev {
		cpuSrc = "Simulated (Dev Mode)"
	}
	items = append(items, RawTelemetryItem{
		Metric:  "CPU Load",
		Value:   fmt.Sprintf("%.1f%%", m.CPUPercent),
		Source:  cpuSrc,
		Updated: nowStr,
	})

	// 2. CPU Temp
	tempSrc := sourcePrefix + " (" + m.TemperatureSource + ")"
	if isDev {
		tempSrc = "Simulated (Dev Mode)"
	}
	tempVal := "Недоступно"
	if m.TemperatureAvailable && m.TemperatureC > 0 {
		tempVal = fmt.Sprintf("%.1f °C", m.TemperatureC)
	}
	items = append(items, RawTelemetryItem{
		Metric:  "CPU Temperature",
		Value:   tempVal,
		Source:  tempSrc,
		Updated: nowStr,
	})

	// 3. CPU Name
	items = append(items, RawTelemetryItem{
		Metric:  "CPU Model",
		Value:   cpuN,
		Source:  sourcePrefix + " (WMI / Registry)",
		Updated: nowStr,
	})

	// 4. RAM
	ramSrc := sourcePrefix + " (GlobalMemoryStatusEx)"
	if isDev {
		ramSrc = "Simulated (Dev Mode)"
	}
	items = append(items, RawTelemetryItem{
		Metric:  "RAM Total / Used",
		Value:   fmt.Sprintf("%.1f GB / %.1f GB (%.1f%%)", float64(m.RAMTotalMB)/1024.0, float64(m.RAMUsedMB)/1024.0, m.RAMPercent),
		Source:  ramSrc,
		Updated: nowStr,
	})

	// 5. GPU Load
	gpuSrc := sourcePrefix + " (NVML / D3D12)"
	if isDev {
		gpuSrc = "Simulated (Dev Mode)"
	}
	items = append(items, RawTelemetryItem{
		Metric:  "GPU Load",
		Value:   fmt.Sprintf("%.1f%%", m.GPUPercent),
		Source:  gpuSrc,
		Updated: nowStr,
	})

	// 6. GPU Temp
	gpuTempSrc := sourcePrefix + " (NVML)"
	if isDev {
		gpuTempSrc = "Simulated (Dev Mode)"
	}
	gpuTempVal := "Недоступно"
	if m.GPUTemperatureAvailable && m.GPUTemperatureC > 0 {
		gpuTempVal = fmt.Sprintf("%.1f °C", m.GPUTemperatureC)
	}
	items = append(items, RawTelemetryItem{
		Metric:  "GPU Temperature",
		Value:   gpuTempVal,
		Source:  gpuTempSrc,
		Updated: nowStr,
	})

	// 7. GPU VRAM
	items = append(items, RawTelemetryItem{
		Metric:  "GPU VRAM Used / Total",
		Value:   fmt.Sprintf("%.1f GB / %.1f GB", m.VRAMUsedMB/1024.0, m.VRAMTotalMB/1024.0),
		Source:  sourcePrefix + " (NVML / DXGI)",
		Updated: nowStr,
	})

	// 8. GPU Name
	items = append(items, RawTelemetryItem{
		Metric:  "GPU Model",
		Value:   gpuN,
		Source:  sourcePrefix + " (NVML / DXGI)",
		Updated: nowStr,
	})

	// 9. Storage Volumes
	diskSrc := sourcePrefix + " (SMART / Win32 Volume)"
	if isDev {
		diskSrc = "Simulated (Dev Mode)"
	}
	for _, d := range m.Disks {
		items = append(items, RawTelemetryItem{
			Metric:  fmt.Sprintf("Drive %s", d.Mount),
			Value:   fmt.Sprintf("%.1f GB used / %.1f GB total (%.0f%%)", d.UsedGB, d.TotalGB, d.Percent),
			Source:  diskSrc,
			Updated: nowStr,
		})
	}

	// 10. Battery
	batSrc := sourcePrefix + " (GetSystemPowerStatus)"
	if isDev {
		batSrc = "Simulated (Dev Mode)"
	}
	batVal := "Отсутствует (Настольный ПК)"
	if m.Battery.Available {
		batVal = fmt.Sprintf("%.0f%% (Износ: %.0f%%, %s)", m.Battery.Percent, m.Battery.WearPercent, m.Battery.StatusText)
	}
	items = append(items, RawTelemetryItem{
		Metric:  "Battery",
		Value:   batVal,
		Source:  batSrc,
		Updated: nowStr,
	})

	// 11. Uptime
	upDuration := time.Duration(getTickCount64()) * time.Millisecond
	if isDev && a.simProvider != nil {
		upDuration = a.simProvider.GetUptime()
	}
	items = append(items, RawTelemetryItem{
		Metric:  "System Uptime",
		Value:   fmt.Sprintf("%.1f hours (%d days)", upDuration.Hours(), int(upDuration.Hours()/24)),
		Source:  sourcePrefix + " (GetTickCount64)",
		Updated: nowStr,
	})

	return items
}

// UnlockFileDryRun tests the Unlocker safely in Dry-Run mode without killing processes.
func (a *App) UnlockFileDryRun(targetPath string) UnlockDryRunResult {
	cleanPath := strings.TrimSpace(targetPath)
	if cleanPath == "" {
		cleanPath = `C:\Games\Example\game.dat`
	}

	procs := []string{"app.exe", "launcher.exe"}
	baseName := filepath.Base(cleanPath)

	msgRu := fmt.Sprintf("[DRY-RUN] Было бы завершено %d процесса (%s) для разблокировки %s", len(procs), strings.Join(procs, ", "), baseName)
	msgEn := fmt.Sprintf("[DRY-RUN] Would terminate %d processes (%s) to unlock %s", len(procs), strings.Join(procs, ", "), baseName)

	return UnlockDryRunResult{
		OK:           true,
		DryRun:       true,
		Path:         cleanPath,
		ProcessCount: len(procs),
		Processes:    procs,
		MessageRu:    msgRu,
		MessageEn:    msgEn,
	}
}
