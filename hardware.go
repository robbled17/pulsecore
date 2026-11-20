package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// DiskHealthReport is a real SMART / Storage Reliability snapshot.
type DiskHealthReport struct {
	DriveLetter       string  `json:"driveLetter"`
	Model             string  `json:"model"`
	SerialNumber      string  `json:"serialNumber"`
	MediaType         string  `json:"mediaType"`
	InterfaceType     string  `json:"interfaceType"`
	HealthStatus      string  `json:"healthStatus"`
	OperationalStatus string  `json:"operationalStatus"`
	Verdict           string  `json:"verdict"`
	VerdictBadge      string  `json:"verdictBadge"`
	HealthPercent     float64 `json:"healthPercent"`
	TemperatureC      float64 `json:"temperatureC"`
	WearRemaining     float64 `json:"wearRemaining"`
	PowerOnHours      float64 `json:"powerOnHours"`
	TBWGB             float64 `json:"tbwGB"`
	HasCriticalErrors bool    `json:"hasCriticalErrors"`
	ReadErrors        int64   `json:"readErrors"`
	WriteErrors       int64   `json:"writeErrors"`
	ErrorMessage      string  `json:"errorMessage"`
	OK                bool    `json:"ok"`
}

// HardwareInfo is a structured deep-dive inventory.
type HardwareInfo struct {
	CPU   CPUDeepInfo   `json:"cpu"`
	GPU   GPUDeepInfo   `json:"gpu"`
	RAM   RAMDeepInfo   `json:"ram"`
	Power PowerDeepInfo `json:"power"`
}

type CPUDeepInfo struct {
	Name          string    `json:"name"`
	PhysicalCores int       `json:"physicalCores"`
	LogicalCores  int       `json:"logicalCores"`
	FrequencyMHz  float64   `json:"frequencyMHz"`
	PerCore       []float64 `json:"perCore"`
	Percent       float64   `json:"percent"`
}

type GPUDeepInfo struct {
	Name           string  `json:"name"`
	DriverVersion  string  `json:"driverVersion"`
	DedicatedMB    float64 `json:"dedicatedMB"`
	SharedMB       float64 `json:"sharedMB"`
	UsedMB         float64 `json:"usedMB"`
	Percent        float64 `json:"percent"`
	IsIntegrated   bool    `json:"isIntegrated"`
	VRAMGB         float64 `json:"vramGB"`
}

type RAMDeepInfo struct {
	TotalMB         uint64 `json:"totalMB"`
	UsedMB          uint64 `json:"usedMB"`
	FreeMB          uint64 `json:"freeMB"`
	Percent         float64 `json:"percent"`
	Type            string  `json:"type"`
	SlotsUsed       int     `json:"slotsUsed"`
	SlotsTotal      int     `json:"slotsTotal"`
	SpeedMHz        int     `json:"speedMHz"`
	IsSingleChannel bool    `json:"isSingleChannel"`
	ChannelStatus   string  `json:"channelStatus"`
}

type PowerDeepInfo struct {
	HasBattery        bool    `json:"hasBattery"`
	WearPercent       float64 `json:"wearPercent"`
	WearLabel         string  `json:"wearLabel"`
	PluggedIn         bool    `json:"pluggedIn"`
	CapacitymWh       float64 `json:"capacitymWh"`
	DesignCapacitymWh float64 `json:"designCapacitymWh"`
	Percent           float64 `json:"percent"`
	StatusText        string  `json:"statusText"`
	PowerDrawWatts    float64 `json:"powerDrawWatts"`
	DeviceName        string  `json:"deviceName"`
}

type staticHardware struct {
	cpuName        string
	physicalCores  int
	logicalCores   int
	frequencyMHz   float64
	gpuName        string
	gpuDriver      string
	gpuVRAM        uint64
	ramType        string
	ramSlotsUsed   int
	ramSlotsTotal  int
	ramSpeedMHz    int
}

func collectStaticHardware() staticHardware {
	out := staticHardware{
		cpuName: readCPUName(),
		gpuName: "Unknown GPU",
	}
	if phys, err := cpu.Counts(false); err == nil {
		out.physicalCores = phys
	}
	if logi, err := cpu.Counts(true); err == nil {
		out.logicalCores = logi
	}
	if info, err := cpu.Info(); err == nil && len(info) > 0 {
		out.frequencyMHz = info[0].Mhz
		if out.cpuName == "" || out.cpuName == "Unknown CPU" {
			out.cpuName = strings.TrimSpace(info[0].ModelName)
		}
	}
	out.gpuName, out.gpuVRAM, out.gpuDriver = readGPUInfoDetailed()
	out.ramType, out.ramSlotsUsed, out.ramSlotsTotal, out.ramSpeedMHz = readRAMSlots()
	return out
}

func readGPUInfoDetailed() (name string, vram uint64, driver string) {
	ensureOLE()
	name = "Unknown GPU"

	locator, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return name, 0, ""
	}
	defer locator.Release()
	dl, err := locator.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return name, 0, ""
	}
	defer dl.Release()
	serviceRaw, err := oleutil.CallMethod(dl, "ConnectServer", nil, `root\cimv2`)
	if err != nil {
		return name, 0, ""
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()

	resultRaw, err := oleutil.CallMethod(service, "ExecQuery",
		"SELECT Name, AdapterRAM, DriverVersion FROM Win32_VideoController")
	if err != nil {
		return name, 0, ""
	}
	result := resultRaw.ToIDispatch()
	defer result.Release()

	countVar, _ := oleutil.GetProperty(result, "Count")
	count := int(countVar.Val)
	for i := 0; i < count; i++ {
		itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
		if err != nil {
			continue
		}
		item := itemRaw.ToIDispatch()
		nameVar, _ := oleutil.GetProperty(item, "Name")
		ramVar, _ := oleutil.GetProperty(item, "AdapterRAM")
		drvVar, _ := oleutil.GetProperty(item, "DriverVersion")
		n := strings.TrimSpace(nameVar.ToString())
		// AdapterRAM is often a signed/unsigned 32-bit WMI value; widen carefully.
		ram := uint64(uint32(ramVar.Val))
		if ramVar.Val < 0 {
			ram = uint64(uint32(int32(ramVar.Val)))
		}
		drv := strings.TrimSpace(drvVar.ToString())
		item.Release()
		if n == "" {
			continue
		}
		lower := strings.ToLower(n)
		if strings.Contains(lower, "microsoft basic") || strings.Contains(lower, "remote desktop") {
			continue
		}
		if ram >= vram {
			vram = ram
			name = n
			driver = drv
		} else if name == "Unknown GPU" {
			name = n
			driver = drv
		}
	}
	return name, vram, driver
}

func detectIntegratedGPU(name string, dedicatedMB float64) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" || strings.Contains(n, "unknown") {
		return true
	}
	// Discrete markers win even if AdapterRAM looks small.
	discrete := []string{
		"geforce", "rtx", "gtx", "quadro", "tesla", "titan",
		"radeon rx", "rx 5", "rx 6", "rx 7", "rx 4", "rx 3",
		"arc a", "arc b", "nvidia",
	}
	for _, d := range discrete {
		if strings.Contains(n, d) {
			return false
		}
	}
	integrated := []string{
		"uhd", "iris", "hd graphics", "intel(r) graphics", "intel graphics",
		"radeon graphics", "radeon(tm) graphics", "vega 3", "vega 5", "vega 6", "vega 7", "vega 8", "vega 11",
		"graphics family", "mali", "adreno", "apple m",
	}
	for _, d := range integrated {
		if strings.Contains(n, d) {
			return true
		}
	}
	// Very small AdapterRAM usually means iGPU / shared pool.
	if dedicatedMB > 0 && dedicatedMB < 1536 {
		return true
	}
	return false
}

func readRAMSlots() (memType string, used, total, speed int) {
	ensureOLE()
	memType = "Unknown"
	used = 0
	total = 0
	speed = 0

	locator, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return
	}
	defer locator.Release()
	dl, err := locator.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return
	}
	defer dl.Release()
	serviceRaw, err := oleutil.CallMethod(dl, "ConnectServer", nil, `root\cimv2`)
	if err != nil {
		return
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()

	resultRaw, err := oleutil.CallMethod(service, "ExecQuery",
		"SELECT MemoryType, SMBIOSMemoryType, Speed, Capacity FROM Win32_PhysicalMemory")
	if err != nil {
		return
	}
	result := resultRaw.ToIDispatch()
	defer result.Release()

	countVar, _ := oleutil.GetProperty(result, "Count")
	count := int(countVar.Val)
	used = count
	total = count
	if total < 2 {
		total = 2 // common dual-channel board baseline when slot count is opaque
	}
	typeCode := -1
	for i := 0; i < count; i++ {
		itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
		if err != nil {
			continue
		}
		item := itemRaw.ToIDispatch()
		smbiosVar, _ := oleutil.GetProperty(item, "SMBIOSMemoryType")
		memTypeVar, _ := oleutil.GetProperty(item, "MemoryType")
		speedVar, _ := oleutil.GetProperty(item, "Speed")
		item.Release()
		if typeCode < 0 {
			typeCode = int(smbiosVar.Val)
			if typeCode == 0 {
				typeCode = int(memTypeVar.Val)
			}
		}
		if sp := int(speedVar.Val); sp > speed {
			speed = sp
		}
	}
	memType = mapMemoryType(typeCode)

	// Try to refine total slots via Win32_PhysicalMemoryArray.
	arrRaw, err := oleutil.CallMethod(service, "ExecQuery",
		"SELECT MemoryDevices FROM Win32_PhysicalMemoryArray")
	if err == nil {
		arr := arrRaw.ToIDispatch()
		defer arr.Release()
		cVar, _ := oleutil.GetProperty(arr, "Count")
		if int(cVar.Val) > 0 {
			itemRaw, err := oleutil.CallMethod(arr, "ItemIndex", 0)
			if err == nil {
				item := itemRaw.ToIDispatch()
				devVar, _ := oleutil.GetProperty(item, "MemoryDevices")
				item.Release()
				if n := int(devVar.Val); n > 0 {
					total = n
				}
			}
		}
	}
	return
}

func mapMemoryType(code int) string {
	switch code {
	case 20:
		return "DDR"
	case 21:
		return "DDR2"
	case 24:
		return "DDR3"
	case 26:
		return "DDR4"
	case 34:
		return "DDR5"
	case 22:
		return "DDR2 FB-DIMM"
	default:
		if code > 0 {
			return fmt.Sprintf("Type %d", code)
		}
		return "Unknown"
	}
}

func buildHardwareInfo(static staticHardware, cpuPct float64, perCore []float64, gpuPct, vramUsed, vramShared, vramDedicated float64, vm *mem.VirtualMemoryStat, bat BatteryMetrics) HardwareInfo {
	vramTotal := float64(static.gpuVRAM) / (1024 * 1024)
	if vramTotal <= 0 {
		vramTotal = vramDedicated
	}
	freq := static.frequencyMHz
	// Prefer live max frequency estimate from info when available.
	if info, err := cpu.Info(); err == nil && len(info) > 0 && info[0].Mhz > 0 {
		freq = info[0].Mhz
	}

	power := PowerDeepInfo{
		HasBattery:        bat.Available,
		WearPercent:       bat.WearPercent,
		WearLabel:         bat.WearLabel,
		PluggedIn:         bat.PluggedIn,
		CapacitymWh:       bat.CapacitymWh,
		DesignCapacitymWh: bat.DesignCapacitymWh,
		Percent:           bat.Percent,
		PowerDrawWatts:    bat.PowerDrawWatts,
		StatusText:        bat.StatusText,
		DeviceName:        bat.DeviceName,
	}
	if !bat.Available {
		power.StatusText = "Стационарный ПК (Блок питания)"
		power.WearLabel = "Питание: Стандартный ATX блок питания (От сети 220V)"
		power.DeviceName = "Нет аккумулятора"
		power.PluggedIn = true
		power.Percent = 100
	}

	freeMB := uint64(0)
	usedMB := uint64(0)
	totalMB := uint64(0)
	pct := 0.0
	if vm != nil {
		freeMB = vm.Available / (1024 * 1024)
		usedMB = vm.Used / (1024 * 1024)
		totalMB = vm.Total / (1024 * 1024)
		pct = vm.UsedPercent
	}

	return HardwareInfo{
		CPU: CPUDeepInfo{
			Name:          static.cpuName,
			PhysicalCores: static.physicalCores,
			LogicalCores:  static.logicalCores,
			FrequencyMHz:  freq,
			PerCore:       perCore,
			Percent:       cpuPct,
		},
		GPU: func() GPUDeepInfo {
			integrated := detectIntegratedGPU(static.gpuName, vramTotal)
			vramGB := vramTotal / 1024.0
			if integrated {
				vramGB = 0 // no dedicated VRAM on iGPU
			}
			return GPUDeepInfo{
				Name:          static.gpuName,
				DriverVersion: static.gpuDriver,
				DedicatedMB:   vramTotal,
				SharedMB:      vramShared,
				UsedMB:        vramUsed,
				Percent:       gpuPct,
				IsIntegrated:  integrated,
				VRAMGB:        vramGB,
			}
		}(),
		RAM: buildRAMDeepInfo(static, totalMB, usedMB, freeMB, pct),
		Power: power,
	}
}

func buildRAMDeepInfo(static staticHardware, totalMB, usedMB, freeMB uint64, pct float64) RAMDeepInfo {
	single := static.ramSlotsUsed == 1
	status := "✓ Двухканальный режим активен (максимальная скорость графики)."
	if single {
		status = "⚠ Память в одноканале. Встроенная графика Radeon теряет до 30-40% мощности."
	} else if static.ramSlotsUsed >= 2 {
		status = "✓ Двухканальный режим активен (максимальная скорость графики)."
	} else {
		status = "Режим каналов памяти не определён."
		single = false
	}
	return RAMDeepInfo{
		TotalMB:         totalMB,
		UsedMB:          usedMB,
		FreeMB:          freeMB,
		Percent:         pct,
		Type:            static.ramType,
		SlotsUsed:       static.ramSlotsUsed,
		SlotsTotal:      static.ramSlotsTotal,
		SpeedMHz:        static.ramSpeedMHz,
		IsSingleChannel: single,
		ChannelStatus:   status,
	}
}

// CheckDiskHealth queries real PhysicalDisk + StorageReliabilityCounter data.
func (a *App) CheckDiskHealth(driveLetter string) DiskHealthReport {
	letter := strings.ToUpper(strings.TrimSpace(driveLetter))
	letter = strings.TrimSuffix(letter, `:\`)
	letter = strings.TrimSuffix(letter, ":")
	if letter == "" {
		letter = "C"
	}

	ps := fmt.Sprintf(`
$ErrorActionPreference='Stop'
try {
  $part = Get-Partition -DriveLetter '%s' -ErrorAction Stop
  $disk = $part | Get-Disk -ErrorAction Stop
  $phys = $disk | Get-PhysicalDisk -ErrorAction Stop
  $rel = $null
  try { $rel = Get-StorageReliabilityCounter -PhysicalDisk $phys -ErrorAction SilentlyContinue } catch {}
  $bus = [string]$disk.BusType
  $media = [string]$phys.MediaType
  $iface = $bus
  if ($media -match 'SSD' -or $media -eq '3' -or $media -eq '4') {
    if ($bus -match 'NVMe|RAID' -or $media -eq '4') { $iface = 'NVMe PCIe SSD' }
    elseif ($bus -match 'SATA|ATA') { $iface = 'SATA SSD' }
    else { $iface = 'SSD' }
  } elseif ($media -match 'HDD' -or $media -eq '2' -or $bus -match 'SATA|ATA|USB') {
    $iface = 'HDD'
  }
  $tbwGB = 0.0
  if ($rel -and $rel.WriteLatencyMax -ne $null) { }
  if ($rel) {
    try {
      $bytes = [double]$rel.FlushLatencyMax
    } catch {}
    try {
      if ($rel.PSObject.Properties.Name -contains 'Wear') { }
      # StorageReliabilityCounter exposes Read/Write errors; TBW approximated from Wear when possible.
    } catch {}
  }
  $obj = [ordered]@{
    Model = [string]$phys.FriendlyName
    SerialNumber = [string]$disk.SerialNumber
    MediaType = $media
    InterfaceType = $iface
    HealthStatus = [string]$phys.HealthStatus
    OperationalStatus = [string]$phys.OperationalStatus
    Temperature = if ($rel) { [double]$rel.Temperature } else { 0 }
    Wear = if ($rel) { [double]$rel.Wear } else { -1 }
    PowerOnHours = if ($rel -and $rel.PowerOnHours) { [double]$rel.PowerOnHours } else { 0 }
    StartStopCycle = if ($rel -and $rel.StartStopCycleCount) { [double]$rel.StartStopCycleCount } else { 0 }
    ReadErrors = if ($rel) { [int64]$rel.ReadErrorsUncorrected } else { 0 }
    WriteErrors = if ($rel) { [int64]$rel.WriteErrorsUncorrected } else { 0 }
    SizeBytes = [double]$disk.Size
  }
  $obj | ConvertTo-Json -Compress
} catch {
  @{ Error = $_.Exception.Message } | ConvertTo-Json -Compress
}
`, letter)

	out, err := runHiddenOutput("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", ps)
	report := DiskHealthReport{
		DriveLetter: letter + ":",
		OK:          false,
	}
	if err != nil {
		report.ErrorMessage = err.Error()
		report.Verdict = "Недоступно"
		return report
	}

	raw := strings.TrimSpace(string(out))
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		report.ErrorMessage = "Не удалось разобрать ответ Storage API"
		report.Verdict = "Недоступно"
		return report
	}
	if errMsg, ok := parsed["Error"].(string); ok && errMsg != "" {
		report.ErrorMessage = errMsg
		report.Verdict = "Недоступно"
		return report
	}

	report.Model = asString(parsed["Model"])
	report.SerialNumber = asString(parsed["SerialNumber"])
	report.MediaType = normalizeMediaType(asString(parsed["MediaType"]))
	report.InterfaceType = asString(parsed["InterfaceType"])
	if report.InterfaceType == "" {
		report.InterfaceType = report.MediaType
	}
	report.HealthStatus = asString(parsed["HealthStatus"])
	report.OperationalStatus = asString(parsed["OperationalStatus"])
	report.TemperatureC = asFloat(parsed["Temperature"])
	report.PowerOnHours = asFloat(parsed["PowerOnHours"])
	wear := asFloat(parsed["Wear"])
	report.ReadErrors = asInt64(parsed["ReadErrors"])
	report.WriteErrors = asInt64(parsed["WriteErrors"])
	report.HasCriticalErrors = report.ReadErrors > 0 || report.WriteErrors > 0 ||
		strings.EqualFold(report.HealthStatus, "Warning") ||
		strings.EqualFold(report.HealthStatus, "Unhealthy")

	if wear >= 0 {
		report.WearRemaining = 100 - wear
		if report.WearRemaining < 0 {
			report.WearRemaining = 0
		}
		report.HealthPercent = report.WearRemaining
		// Approximate TBW from capacity * wear fraction when size known.
		sizeBytes := asFloat(parsed["SizeBytes"])
		if sizeBytes > 0 && wear > 0 {
			report.TBWGB = (sizeBytes * (wear / 100.0)) / (1024 * 1024 * 1024)
		}
	} else if strings.EqualFold(report.HealthStatus, "Healthy") {
		report.HealthPercent = 100
		report.WearRemaining = 100
	} else if strings.EqualFold(report.HealthStatus, "Warning") {
		report.HealthPercent = 70
		report.WearRemaining = 70
	} else {
		report.HealthPercent = 50
	}

	switch {
	case report.HasCriticalErrors || strings.EqualFold(report.HealthStatus, "Unhealthy"):
		report.Verdict = "Внимание"
		report.VerdictBadge = "⚠ Внимание"
	case strings.EqualFold(report.HealthStatus, "Warning"):
		report.Verdict = "Внимание"
		report.VerdictBadge = "⚠ Внимание"
	default:
		report.Verdict = fmt.Sprintf("Отличное (%.0f%%)", report.HealthPercent)
		report.VerdictBadge = fmt.Sprintf("✓ Состояние: Отличное (%.0f%%)", report.HealthPercent)
	}
	report.OK = true
	return report
}

func normalizeMediaType(media string) string {
	m := strings.ToLower(strings.TrimSpace(media))
	switch {
	case strings.Contains(m, "nvme") || m == "4" || m == "scm":
		return "NVMe SSD"
	case strings.Contains(m, "ssd") || m == "3" || m == "solidstate":
		return "SATA SSD"
	case strings.Contains(m, "hdd") || m == "2" || strings.Contains(m, "hard"):
		return "HDD"
	default:
		if media == "" || strings.Contains(m, "unspecified") {
			return "Unknown"
		}
		return media
	}
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func asFloat(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	default:
		return 0
	}
}

func asInt64(v interface{}) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int:
		return int64(t)
	case int64:
		return t
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	default:
		return 0
	}
}
