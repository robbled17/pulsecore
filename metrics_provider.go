package main

import (
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/mem"
)

// MetricsProvider decouples raw hardware telemetry from consumption.
// It allows transparent switching between live Win32 telemetry and the Dev Mode simulator.
type MetricsProvider interface {
	GetCPULoad() (overall float64, perCore []float64)
	GetCPUTemp(cpuPct float64) (temp float64, available bool, source, sourceRu, sourceEn string)
	GetRAM() (*mem.VirtualMemoryStat, *mem.SwapMemoryStat)
	GetGPU(tab string) (gpuPct float64, vramUsedMB, vramSharedMB, vramDedicatedMB float64, temp float64, tempAvailable bool, name string)
	GetDisks(now time.Time) []DiskMetrics
	GetDiskHealth(driveLetter string) (DiskHealthReport, bool)
	GetLocalDrives() []LocalDriveInfo
	GetBattery(now time.Time) BatteryMetrics
	GetUptime() time.Duration
	GetStaticHardware() staticHardware
	GetVampires() []VampireProcess
}

// RealMetricsProvider gathers real system metrics using native Win32/WMI/NVML/SMART APIs.
type RealMetricsProvider struct {
	app *App
}

func NewRealMetricsProvider(a *App) *RealMetricsProvider {
	return &RealMetricsProvider{app: a}
}

func (r *RealMetricsProvider) GetCPULoad() (float64, []float64) {
	return sampleCPULoad()
}

func (r *RealMetricsProvider) GetCPUTemp(cpuPct float64) (float64, bool, string, string, string) {
	if r.app == nil {
		rd := readCPUTemperatureSmart()
		return rd.TempC, rd.OK, rd.Source, rd.LabelRu, rd.LabelEn
	}
	r.app.mu.RLock()
	tab := r.app.activeTab
	r.app.mu.RUnlock()
	if tab == "" {
		tab = "overview"
	}
	temp := r.app.tempCached(time.Now(), cpuPct, tab)
	r.app.mu.RLock()
	src := r.app.cachedTempSource
	ru := r.app.cachedTempLabelRu
	en := r.app.cachedTempLabelEn
	avail := temp > 0 && !r.app.tempUnavailable
	r.app.mu.RUnlock()
	return temp, avail, src, ru, en
}

func (r *RealMetricsProvider) GetRAM() (*mem.VirtualMemoryStat, *mem.SwapMemoryStat) {
	vm, _ := mem.VirtualMemory()
	sm, _ := mem.SwapMemory()
	return vm, sm
}

func (r *RealMetricsProvider) GetGPU(tab string) (float64, float64, float64, float64, float64, bool, string) {
	if r.app == nil {
		return 0, 0, 0, 0, 0, false, ""
	}
	now := time.Now()
	gpuPct, vramUsed, vramShared, vramDedicated := r.app.gpuCached(now, tab)
	gpuTemp, gpuTempOK, _, _, _, _ := r.app.gpuExtras()
	r.app.mu.RLock()
	name := r.app.gpuName
	r.app.mu.RUnlock()
	return gpuPct, vramUsed, vramShared, vramDedicated, gpuTemp, gpuTempOK, name
}

func (r *RealMetricsProvider) GetDisks(now time.Time) []DiskMetrics {
	if r.app == nil {
		return nil
	}
	r.app.mu.RLock()
	tab := r.app.activeTab
	r.app.mu.RUnlock()
	if tab == "" {
		tab = "overview"
	}
	disks := r.app.disksForTab(tab, now)
	if len(disks) == 0 {
		r.app.mu.RLock()
		cached := append([]DiskMetrics(nil), r.app.cachedDisks...)
		r.app.mu.RUnlock()
		if len(cached) > 0 {
			return cached
		}
	}
	return disks
}

func (r *RealMetricsProvider) GetDiskHealth(driveLetter string) (DiskHealthReport, bool) {
	if r.app == nil {
		return DiskHealthReport{}, false
	}
	return r.app.checkDiskHealthSync(driveLetter), true
}

func (r *RealMetricsProvider) GetLocalDrives() []LocalDriveInfo {
	return getLocalDrivesNative()
}

func (r *RealMetricsProvider) GetBattery(now time.Time) BatteryMetrics {
	if r.app == nil {
		return collectBattery()
	}
	return r.app.batteryCached(now)
}

func (r *RealMetricsProvider) GetUptime() time.Duration {
	return time.Duration(getTickCount64()) * time.Millisecond
}

func (r *RealMetricsProvider) GetStaticHardware() staticHardware {
	if r.app != nil {
		r.app.mu.RLock()
		hw := r.app.staticHW
		r.app.mu.RUnlock()
		if hw.cpuName != "" {
			return hw
		}
	}
	return collectStaticHardware()
}

func (r *RealMetricsProvider) GetVampires() []VampireProcess {
	if r.app == nil {
		return nil
	}
	return r.app.GetVampireProcesses()
}

// SimulatedDisk describes one simulated disk for the hardware constructor.
type SimulatedDisk struct {
	Letter    string  `json:"letter"`    // "C", "D", "E"
	Type      string  `json:"type"`      // "HDD", "SATA SSD", "NVMe SSD"
	Model     string  `json:"model"`     // e.g. "WDC WD10EZEX", "Samsung SSD 980 PRO"
	TotalGB   float64 `json:"totalGB"`   // 120, 240, 480, 512, 1024, 2048, 4096
	UsedPct   float64 `json:"usedPct"`   // 0-99
	HealthPct float64 `json:"healthPct"` // 1-100
	TempC     float64 `json:"tempC"`     // 25-75
}

// SimulatedMetricsProvider simulates sensors and hardware configuration.
type SimulatedMetricsProvider struct {
	mu sync.RWMutex

	// Sensors
	cpuPercent   float64
	cpuPerCore   []float64
	cpuTemp      float64
	cpuTempAvail bool

	ramTotalMB uint64
	ramUsedMB  uint64

	gpuPercent   float64
	vramUsedMB   float64
	vramTotalMB  float64
	gpuTemp      float64
	gpuTempAvail bool

	uptimeDuration time.Duration

	// Hardware Info
	cpuName       string
	physicalCores int
	logicalCores  int
	freqMHz       float64
	cpuScore      float64

	gpuName      string
	gpuVRAMMB    float64
	isIntegrated bool
	gpuScore     float64

	ramType     string
	ramSpeedMHz int

	// Laptop / Battery
	isLaptop       bool
	batteryPercent float64
	pluggedIn      bool
	wearPercent    float64
	powerDrawWatts float64

	// Disks
	disks []SimulatedDisk

	// Vampires
	vampires []VampireProcess
}

// NewSimulatedMetricsProvider initializes a realistic default simulated state.
func NewSimulatedMetricsProvider() *SimulatedMetricsProvider {
	cores := 8
	threads := 16
	perCore := make([]float64, threads)
	for i := range perCore {
		perCore[i] = 12.0
	}

	return &SimulatedMetricsProvider{
		cpuPercent:     12.0,
		cpuPerCore:     perCore,
		cpuTemp:        48.0,
		cpuTempAvail:   true,
		ramTotalMB:     32 * 1024, // 32 GB
		ramUsedMB:      6 * 1024,  // 6 GB (18.75%)
		gpuPercent:     5.0,
		vramUsedMB:     2400.0,
		vramTotalMB:    16.0 * 1024.0, // 16 GB
		gpuTemp:        42.0,
		gpuTempAvail:   true,
		uptimeDuration: 4*time.Hour + 30*time.Minute,

		cpuName:       "AMD Ryzen 7 7800X3D",
		physicalCores: cores,
		logicalCores:  threads,
		freqMHz:       4200.0,
		cpuScore:      96.0,

		gpuName:      "NVIDIA GeForce RTX 4080 Super",
		gpuVRAMMB:    16.0 * 1024.0,
		isIntegrated: false,
		gpuScore:     95.0,

		ramType:     "DDR5",
		ramSpeedMHz: 6000,

		isLaptop:       false,
		batteryPercent: 100.0,
		pluggedIn:      true,
		wearPercent:    0.0,
		powerDrawWatts: 0.0,

		disks: []SimulatedDisk{
			{
				Letter:    "C",
				Type:      "NVMe SSD",
				Model:     "Samsung SSD 980 PRO 1TB",
				TotalGB:   1024,
				UsedPct:   35.0,
				HealthPct: 100.0,
				TempC:     34.0,
			},
			{
				Letter:    "D",
				Type:      "HDD",
				Model:     "WDC WD10EZEX",
				TotalGB:   2048,
				UsedPct:   55.0,
				HealthPct: 98.0,
				TempC:     32.0,
			},
		},
		vampires: nil,
	}
}

func (s *SimulatedMetricsProvider) GetCPULoad() (float64, []float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cores := len(s.cpuPerCore)
	if cores == 0 {
		cores = s.logicalCores
		if cores <= 0 {
			cores = 8
		}
	}
	outCore := make([]float64, cores)
	for i := range outCore {
		if i < len(s.cpuPerCore) {
			outCore[i] = s.cpuPerCore[i]
		} else {
			outCore[i] = s.cpuPercent
		}
	}
	return s.cpuPercent, outCore
}

func (s *SimulatedMetricsProvider) GetCPUTemp(cpuPct float64) (float64, bool, string, string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cpuTemp, s.cpuTempAvail, "simulated", "Симулятор Dev Mode", "Dev Mode Simulator"
}

func (s *SimulatedMetricsProvider) GetRAM() (*mem.VirtualMemoryStat, *mem.SwapMemoryStat) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tot := s.ramTotalMB * 1024 * 1024
	used := s.ramUsedMB * 1024 * 1024
	pct := 0.0
	if tot > 0 {
		pct = (float64(used) / float64(tot)) * 100.0
	}
	free := tot - used
	vm := &mem.VirtualMemoryStat{
		Total:       tot,
		Available:   free,
		Used:        used,
		UsedPercent: pct,
		Free:        free,
	}
	swapTot := uint64(8 * 1024 * 1024 * 1024)
	swapUsed := uint64(float64(swapTot) * (pct / 100.0) * 0.4)
	sm := &mem.SwapMemoryStat{
		Total:       swapTot,
		Used:        swapUsed,
		Free:        swapTot - swapUsed,
		UsedPercent: pct * 0.4,
	}
	return vm, sm
}

func (s *SimulatedMetricsProvider) GetGPU(tab string) (float64, float64, float64, float64, float64, bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	vramTotal := s.vramTotalMB
	if vramTotal <= 0 {
		vramTotal = s.gpuVRAMMB
	}
	vramUsed := s.vramUsedMB
	if vramUsed <= 0 && vramTotal > 0 {
		vramUsed = vramTotal * (s.gpuPercent / 100.0)
	}
	return s.gpuPercent, vramUsed, 512.0, vramTotal, s.gpuTemp, s.gpuTempAvail, s.gpuName
}

func (s *SimulatedMetricsProvider) GetDisks(now time.Time) []DiskMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DiskMetrics, len(s.disks))
	for i, d := range s.disks {
		letter := strings.ToUpper(strings.TrimSpace(d.Letter))
		if letter == "" {
			letter = "C"
		}
		mount := letter + `:\`
		usedGB := d.TotalGB * (d.UsedPct / 100.0)
		freeGB := d.TotalGB - usedGB
		out[i] = DiskMetrics{
			Mount:     mount,
			UsedGB:    usedGB,
			TotalGB:   d.TotalGB,
			Percent:   d.UsedPct,
			FreeGB:    freeGB,
			ReadMBps:  8.4,
			WriteMBps: 3.1,
		}
	}
	return out
}

func (s *SimulatedMetricsProvider) GetDiskHealth(driveLetter string) (DiskHealthReport, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cleanLetter := strings.ToUpper(strings.TrimSpace(driveLetter))
	cleanLetter = strings.TrimSuffix(cleanLetter, `:\`)
	cleanLetter = strings.TrimSuffix(cleanLetter, ":")
	if cleanLetter == "" {
		cleanLetter = "C"
	}

	var found *SimulatedDisk
	for i := range s.disks {
		dLetter := strings.ToUpper(strings.TrimSpace(s.disks[i].Letter))
		if dLetter == cleanLetter {
			found = &s.disks[i]
			break
		}
	}
	if found == nil && len(s.disks) > 0 {
		found = &s.disks[0]
	}
	if found == nil {
		return DiskHealthReport{OK: false}, false
	}

	isHDD := strings.EqualFold(found.Type, "HDD") || strings.Contains(strings.ToUpper(found.Model), "WD10EZEX")
	isNVMe := strings.Contains(strings.ToUpper(found.Type), "NVME") || strings.Contains(strings.ToUpper(found.Model), "980")

	mediaType := "SSD"
	mediaKind := "sata_ssd"
	bus := "SATA"
	rpm := 0.0

	if isHDD {
		mediaType = "HDD"
		mediaKind = "hdd"
		bus = "SATA"
		rpm = 7200.0
	} else if isNVMe {
		mediaType = "SSD"
		mediaKind = "nvme"
		bus = "NVMe"
	}

	statusLevel := "good"
	statusCode := "diskStateGood"
	statusWordRu := "Хорошо"
	statusWordEn := "Good"
	statusReasonRu := "Накопитель в отличном техническом состоянии"
	statusReasonEn := "Drive is in good technical health"

	if found.HealthPct < 20.0 {
		statusLevel = "bad"
		statusCode = "diskStateBad"
		statusWordRu = "Критическое состояние"
		statusWordEn = "Bad"
		statusReasonRu = "Критический износ ячеек памяти или переназначенные сектора"
		statusReasonEn = "Critical memory wear or reallocated sectors"
	} else if found.HealthPct < 60.0 {
		statusLevel = "caution"
		statusCode = "diskStateCaution"
		statusWordRu = "Внимание"
		statusWordEn = "Caution"
		statusReasonRu = "Повышенный износ ресурса или нестабильные сектора"
		statusReasonEn = "Elevated resource wear or pending sectors"
	}

	return DiskHealthReport{
		DriveLetter:       cleanLetter,
		Model:             found.Model,
		SerialNumber:      "SIM-DEV-SERIAL-001",
		Firmware:          "SIM1.00",
		MediaType:         mediaType,
		MediaKind:         mediaKind,
		BusInterface:      bus,
		InterfaceType:     bus,
		RotationRPM:       rpm,
		CapacityGB:        found.TotalGB,
		CapacityBytes:     found.TotalGB * 1024 * 1024 * 1024,
		HealthStatus:      strings.ToUpper(statusLevel),
		StatusLevel:       statusLevel,
		StatusCode:        statusCode,
		StatusWordRu:      statusWordRu,
		StatusWordEn:      statusWordEn,
		StatusReasonRu:    statusReasonRu,
		StatusReasonEn:    statusReasonEn,
		HealthPercent:     found.HealthPct,
		HealthPercentOK:   true,
		HealthSource:      "Dev Mode Simulator",
		HealthSourceRu:    "Симулятор Dev Mode",
		HealthSourceEn:    "Dev Mode Simulator",
		TemperatureC:      found.TempC,
		TempAvailable:     true,
		WearRemaining:     found.HealthPct,
		PowerOnHours:      1420.0,
		CheckedAt:         time.Now().Format("15:04:05"),
		OK:                true,
	}, true
}

func (s *SimulatedMetricsProvider) GetLocalDrives() []LocalDriveInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]LocalDriveInfo, len(s.disks))
	for i, d := range s.disks {
		letter := strings.ToUpper(strings.TrimSpace(d.Letter))
		if letter == "" {
			letter = "C"
		}
		mount := letter + `:\`
		usedGB := d.TotalGB * (d.UsedPct / 100.0)
		freeGB := d.TotalGB - usedGB
		totBytes := uint64(d.TotalGB * 1024 * 1024 * 1024)
		usedBytes := uint64(usedGB * 1024 * 1024 * 1024)
		freeBytes := totBytes - usedBytes
		out[i] = LocalDriveInfo{
			Letter:      letter,
			Mount:       mount,
			Label:       d.Type,
			DriveType:   d.Type,
			TotalBytes:  totBytes,
			UsedBytes:   usedBytes,
			FreeBytes:   freeBytes,
			TotalGB:     d.TotalGB,
			UsedGB:      usedGB,
			FreeGB:      freeGB,
			UsedPercent: d.UsedPct,
		}
	}
	return out
}

func (s *SimulatedMetricsProvider) GetBattery(now time.Time) BatteryMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.isLaptop {
		return BatteryMetrics{
			Available:      false,
			Percent:        0,
			PluggedIn:      true,
			PowerDrawWatts: 0,
			StatusText:     "AC Line",
			WearPercent:    0,
		}
	}

	statusText := "Работает от сети"
	if !s.pluggedIn {
		statusText = "Работает от батареи"
	}
	designCap := 60000.0
	capMWh := designCap * (1.0 - (s.wearPercent / 100.0))
	powerDraw := s.powerDrawWatts
	if powerDraw <= 0 {
		if s.pluggedIn {
			powerDraw = 35.0
		} else {
			powerDraw = 18.5
		}
	}

	return BatteryMetrics{
		Available:         true,
		Percent:           s.batteryPercent,
		PluggedIn:         s.pluggedIn,
		PowerDrawWatts:    powerDraw,
		StatusText:        statusText,
		WearPercent:       s.wearPercent,
		DesignCapacitymWh: designCap,
		CapacitymWh:       capMWh,
		DeviceName:        "Simulated Laptop Battery",
	}
}

func (s *SimulatedMetricsProvider) GetUptime() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.uptimeDuration
}

func (s *SimulatedMetricsProvider) GetStaticHardware() staticHardware {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return staticHardware{
		cpuName:       s.cpuName,
		physicalCores: s.physicalCores,
		logicalCores:  s.logicalCores,
		frequencyMHz:  s.freqMHz,
		gpuName:       s.gpuName,
		gpuDriver:     "SIM-560.94",
		gpuVRAM:       uint64(s.gpuVRAMMB * 1024 * 1024),
		ramType:       s.ramType,
		ramSlotsUsed:  2,
		ramSpeedMHz:   s.ramSpeedMHz,
	}
}

func (s *SimulatedMetricsProvider) GetVampires() []VampireProcess {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.vampires) == 0 {
		return nil
	}
	return append([]VampireProcess(nil), s.vampires...)
}
