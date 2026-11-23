package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"golang.org/x/sys/windows/registry"
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
	Name               string    `json:"name"`
	PhysicalCores      int       `json:"physicalCores"`
	LogicalCores       int       `json:"logicalCores"`
	FrequencyMHz       float64   `json:"frequencyMHz"`
	BaseFrequencyMHz   float64   `json:"baseFrequencyMHz"`
	CurrentFrequencyMHz float64  `json:"currentFrequencyMHz"`
	CacheKB            int       `json:"cacheKB"`
	Architecture       string    `json:"architecture"`
	PerCore            []float64 `json:"perCore"`
	Percent            float64   `json:"percent"`
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
	baseFrequency  float64
	cacheKB        int
	architecture   string
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
	out.architecture = "x64"
	if info, err := cpu.Info(); err == nil && len(info) > 0 {
		out.frequencyMHz = info[0].Mhz
		out.baseFrequency = info[0].Mhz
		out.cacheKB = int(info[0].CacheSize)
		if out.cpuName == "" || out.cpuName == "Unknown CPU" {
			out.cpuName = strings.TrimSpace(info[0].ModelName)
		}
		if info[0].Family != "" || info[0].Model != "" {
			out.architecture = fmt.Sprintf("Family %s · Model %s", info[0].Family, info[0].Model)
		}
	}
	if arch := readCPUArchitecture(); arch != "" {
		out.architecture = arch
	}
	out.gpuName, out.gpuVRAM, out.gpuDriver = readGPUInfoDetailed()
	out.ramType, out.ramSlotsUsed, out.ramSlotsTotal, out.ramSpeedMHz = readRAMSlots()
	return out
}

func readCPUArchitecture() string {
	ensureOLE()
	defer func() { _ = recover() }()
	locator, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return ""
	}
	defer locator.Release()
	dl, err := locator.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return ""
	}
	defer dl.Release()
	serviceRaw, err := oleutil.CallMethod(dl, "ConnectServer", nil, `root\cimv2`)
	if err != nil {
		return ""
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()
	resultRaw, err := oleutil.CallMethod(service, "ExecQuery", "SELECT Architecture, Caption, MaxClockSpeed, L2CacheSize, L3CacheSize FROM Win32_Processor")
	if err != nil {
		return ""
	}
	result := resultRaw.ToIDispatch()
	defer result.Release()
	countVar, err := oleutil.GetProperty(result, "Count")
	if err != nil {
		return ""
	}
	count := int(countVar.Val)
	countVar.Clear()
	if count < 1 {
		return ""
	}
	itemRaw, err := oleutil.CallMethod(result, "ItemIndex", 0)
	if err != nil {
		return ""
	}
	item := itemRaw.ToIDispatch()
	defer item.Release()
	archMap := map[int]string{
		0: "x86", 5: "ARM", 6: "ia64", 9: "x64", 12: "ARM64",
	}
	archCode := 9
	if v, err := oleutil.GetProperty(item, "Architecture"); err == nil {
		archCode = int(v.Val)
		v.Clear()
	}
	label := archMap[archCode]
	if label == "" {
		label = "x64"
	}
	return label
}

func gpuDiscreteRank(name string) int {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return 0
	}
	if strings.Contains(n, "microsoft basic") || strings.Contains(n, "remote desktop") {
		return -1
	}
	discrete := []string{
		"geforce", "rtx", "gtx", "quadro", "tesla", "titan",
		"radeon rx", "rx 5", "rx 6", "rx 7", "rx 4", "rx 3",
		"arc a", "arc b", "nvidia",
	}
	for _, d := range discrete {
		if strings.Contains(n, d) {
			return 3
		}
	}
	integrated := []string{
		"uhd", "iris", "hd graphics", "intel(r) graphics", "intel graphics",
		"radeon graphics", "radeon(tm) graphics", "vega ", "graphics family",
	}
	for _, d := range integrated {
		if strings.Contains(n, d) {
			return 1
		}
	}
	return 2 // unknown vendor: prefer over known iGPU when AdapterRAM ties
}

func readGPUInfoDetailed() (name string, vram uint64, driver string) {
	ensureOLE()
	name = "Unknown GPU"
	defer func() {
		if r := recover(); r != nil {
			name = "Unknown GPU"
			vram = 0
			driver = ""
		}
	}()

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

	bestRank := -2
	countVar, _ := oleutil.GetProperty(result, "Count")
	count := int(countVar.Val)
	for i := 0; i < count; i++ {
		itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
		if err != nil {
			continue
		}
		item := itemRaw.ToIDispatch()
		if item == nil {
			continue
		}
		nameVar, _ := oleutil.GetProperty(item, "Name")
		ramVar, _ := oleutil.GetProperty(item, "AdapterRAM")
		drvVar, _ := oleutil.GetProperty(item, "DriverVersion")
		n := strings.TrimSpace(nameVar.ToString())
		// AdapterRAM is a 32-bit WMI field: cards >4GB wrap to 0 / garbage.
		ram := uint64(uint32(ramVar.Val))
		if ramVar.Val < 0 {
			ram = uint64(uint32(int32(ramVar.Val)))
		}
		// Values at the 32-bit ceiling are not trustworthy for >4GB cards.
		if ram >= 0xFFFF0000 {
			ram = 0
		}
		drv := strings.TrimSpace(drvVar.ToString())
		item.Release()
		if n == "" {
			continue
		}
		rank := gpuDiscreteRank(n)
		if rank < 0 {
			continue
		}
		// Prefer discrete (even when AdapterRAM is 0 on Optimus), then higher VRAM.
		better := false
		if rank > bestRank {
			better = true
		} else if rank == bestRank && ram > vram {
			better = true
		} else if rank == bestRank && ram == vram && name == "Unknown GPU" {
			better = true
		}
		if better {
			bestRank = rank
			vram = ram
			name = n
			driver = drv
		}
	}

	// Prefer 64-bit registry qwMemorySize / nvidia-smi when AdapterRAM is missing or clamped.
	if qw := readVRAMBytesFromRegistry(name); qw > vram {
		vram = qw
	}
	if _, _, totalMB, ok := readNVIDIASMICached(); ok && totalMB > 0 {
		smiBytes := uint64(totalMB * 1024 * 1024)
		if smiBytes > vram {
			vram = smiBytes
		}
	}
	if known := knownDiscreteVRAMBytes(name); known > vram {
		vram = known
	}
	return name, vram, driver
}

func knownDiscreteVRAMBytes(name string) uint64 {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "2060 super"):
		return 8 * 1024 * 1024 * 1024
	case strings.Contains(lower, "2070 super"), strings.Contains(lower, "2070"):
		return 8 * 1024 * 1024 * 1024
	case strings.Contains(lower, "2080 super"), strings.Contains(lower, "2080 ti"):
		if strings.Contains(lower, "ti") {
			return 11 * 1024 * 1024 * 1024
		}
		return 8 * 1024 * 1024 * 1024
	case strings.Contains(lower, "2080"):
		return 8 * 1024 * 1024 * 1024
	case strings.Contains(lower, "3070 ti"), strings.Contains(lower, "3070"):
		return 8 * 1024 * 1024 * 1024
	case strings.Contains(lower, "3080 ti"):
		return 12 * 1024 * 1024 * 1024
	case strings.Contains(lower, "3080"):
		return 10 * 1024 * 1024 * 1024
	case strings.Contains(lower, "3090"):
		return 24 * 1024 * 1024 * 1024
	case strings.Contains(lower, "4070 ti"), strings.Contains(lower, "4070"):
		return 12 * 1024 * 1024 * 1024
	case strings.Contains(lower, "4080"):
		return 16 * 1024 * 1024 * 1024
	case strings.Contains(lower, "4090"):
		return 24 * 1024 * 1024 * 1024
	case strings.Contains(lower, "rx 6700 xt"), strings.Contains(lower, "rx 6750 xt"):
		return 12 * 1024 * 1024 * 1024
	case strings.Contains(lower, "rx 6800"):
		return 16 * 1024 * 1024 * 1024
	case strings.Contains(lower, "rx 7800 xt"), strings.Contains(lower, "rx 7900"):
		return 16 * 1024 * 1024 * 1024
	default:
		return 0
	}
}

// readVRAMBytesFromRegistry reads HardwareInformation.qwMemorySize (64-bit) for display adapters.
// This bypasses the Win32_VideoController AdapterRAM 32-bit clamp above 4GB.
func readVRAMBytesFromRegistry(preferName string) uint64 {
	const classPath = `SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}`
	root, err := registry.OpenKey(registry.LOCAL_MACHINE, classPath, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err != nil {
		return 0
	}
	defer root.Close()
	subs, err := root.ReadSubKeyNames(64)
	if err != nil {
		return 0
	}
	prefer := strings.ToLower(strings.TrimSpace(preferName))
	var best, matched uint64
	for _, sub := range subs {
		if len(sub) != 4 {
			continue
		}
		// Only numbered adapter slots (0000, 0001, ...).
		if sub[0] < '0' || sub[0] > '9' {
			continue
		}
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, classPath+`\`+sub, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		desc, _, _ := k.GetStringValue("DriverDesc")
		bytes := readQwMemorySize(k)
		k.Close()
		if bytes < 512*1024*1024 {
			continue
		}
		descL := strings.ToLower(strings.TrimSpace(desc))
		if prefer != "" && descL != "" && (strings.Contains(descL, prefer) || strings.Contains(prefer, descL)) {
			if bytes > matched {
				matched = bytes
			}
		}
		if bytes > best {
			best = bytes
		}
	}
	if matched > 0 {
		return matched
	}
	return best
}

func readQwMemorySize(k registry.Key) uint64 {
	// REG_QWORD
	if v, _, err := k.GetIntegerValue("HardwareInformation.qwMemorySize"); err == nil && v > 0 {
		return v
	}
	// Some drivers store it as REG_BINARY (8 bytes little-endian).
	if raw, _, err := k.GetBinaryValue("HardwareInformation.qwMemorySize"); err == nil && len(raw) >= 8 {
		return binary.LittleEndian.Uint64(raw[:8])
	}
	return 0
}

func detectIntegratedGPU(name string, dedicatedMB float64) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	// Unknown with measurable dedicated VRAM is treated as discrete for GamePulse.
	if n == "" || strings.Contains(n, "unknown") {
		return dedicatedMB <= 0
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
	// Unknown name with ≥2GB dedicated → discrete.
	if dedicatedMB >= 2048 {
		return false
	}
	return false
}

func readRAMSlots() (memType string, used, total, speed int) {
	memType = "Unknown"
	used = 0
	total = 0
	speed = 0
	_ = withSensorGate(defaultSensorTimeout, func() {
		ensureOLE()
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
	})
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

	baseFreq := static.baseFrequency
	if baseFreq <= 0 {
		baseFreq = static.frequencyMHz
	}
	curFreq := freq
	if curFreq <= 0 {
		curFreq = static.frequencyMHz
	}
	return HardwareInfo{
		CPU: CPUDeepInfo{
			Name:                static.cpuName,
			PhysicalCores:       static.physicalCores,
			LogicalCores:        static.logicalCores,
			FrequencyMHz:        curFreq,
			BaseFrequencyMHz:    baseFreq,
			CurrentFrequencyMHz: curFreq,
			CacheKB:             static.cacheKB,
			Architecture:        static.architecture,
			PerCore:             perCore,
			Percent:             cpuPct,
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
	slotsUsed, slotsTotal, _ := normalizeRAMSlots(static.ramSlotsUsed, static.ramSlotsTotal)
	single := slotsUsed == 1
	status := "✓ Двухканальный режим активен (максимальная скорость графики)."
	if single {
		status = "⚠ Память в одноканале. Встроенная графика Radeon теряет до 30-40% мощности."
	} else if slotsUsed >= 2 {
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
		SlotsUsed:       slotsUsed,
		SlotsTotal:      slotsTotal,
		SpeedMHz:        static.ramSpeedMHz,
		IsSingleChannel: single,
		ChannelStatus:   status,
	}
}

// CheckDiskHealth queries real PhysicalDisk + StorageReliabilityCounter data.
func (a *App) CheckDiskHealth(driveLetter string) DiskHealthReport {
	letter := sanitizeDriveLetter(driveLetter)

	// Storage cmdlets require PowerShell; never use -ExecutionPolicy Bypass (AV heuristic).
	// Drive letter is a single A-Z from sanitizeDriveLetter: no string-concat argv injection.
	// Force UTF-8 output encoding so non-English error strings never produce encoding artifacts.
	ps := `[Console]::OutputEncoding = [System.Text.Encoding]::UTF8;
[Console]::InputEncoding = [System.Text.Encoding]::UTF8;
$ErrorActionPreference = 'SilentlyContinue';
$letter = '` + letter + `';

$model = '';
$serial = '';
$media = 'HDD';
$iface = 'SATA';
$health = 'Healthy';
$opStatus = 'OK';
$temp = 0.0;
$wear = -1.0;
$poh = 0.0;
$startStop = 0.0;
$readErr = 0;
$writeErr = 0;
$size = 0.0;

# Primary lookup: Windows Storage API (PhysicalDisk + StorageReliabilityCounter)
try {
  $part = Get-Partition -DriveLetter $letter -ErrorAction SilentlyContinue
  if ($part) {
    $disk = $part | Get-Disk -ErrorAction SilentlyContinue
    if ($disk) {
      $size = [double]$disk.Size
      $model = [string]$disk.FriendlyName
      $serial = [string]$disk.SerialNumber
      $bus = [string]$disk.BusType
      if ($disk.HealthStatus) { $health = [string]$disk.HealthStatus }
      if ($disk.OperationalStatus) { $opStatus = [string]$disk.OperationalStatus }
      if ($bus -match 'NVMe') { $iface = 'NVMe PCIe SSD'; $media = 'SSD' }
      elseif ($bus -match 'SATA|ATA') { $iface = 'SATA' }
      elseif ($bus) { $iface = $bus }

      $phys = $disk | Get-PhysicalDisk -ErrorAction SilentlyContinue
      if ($phys) {
        if ($phys.FriendlyName) { $model = [string]$phys.FriendlyName }
        if ($phys.MediaType) { $media = [string]$phys.MediaType }
        if ($phys.HealthStatus) { $health = [string]$phys.HealthStatus }
        if ($phys.OperationalStatus) { $opStatus = [string]$phys.OperationalStatus }
        $rel = $null
        try { $rel = Get-StorageReliabilityCounter -PhysicalDisk $phys -ErrorAction SilentlyContinue } catch {}
        if ($rel) {
          if ($rel.Temperature) { $temp = [double]$rel.Temperature }
          if ($rel.Wear -ne $null) { $wear = [double]$rel.Wear }
          if ($rel.PowerOnHours) { $poh = [double]$rel.PowerOnHours }
          if ($rel.StartStopCycleCount) { $startStop = [double]$rel.StartStopCycleCount }
          if ($rel.ReadErrorsUncorrected) { $readErr = [int64]$rel.ReadErrorsUncorrected }
          if ($rel.WriteErrorsUncorrected) { $writeErr = [int64]$rel.WriteErrorsUncorrected }
        }
      }
    }
  }
} catch {}

# Fallback lookup: WMI / CIM for secondary SATA HDDs and partitions
if (-not $model -or $model -match '^\s*$') {
  try {
    $ld = Get-CimInstance Win32_LogicalDisk -Filter "DeviceID='$($letter):'" -ErrorAction SilentlyContinue
    if ($ld) {
      $size = [double]$ld.Size
      $parts = Get-CimAssociatedInstance -InputObject $ld -ResultClassName Win32_DiskPartition -ErrorAction SilentlyContinue
      foreach ($p in $parts) {
        $drives = Get-CimAssociatedInstance -InputObject $p -ResultClassName Win32_DiskDrive -ErrorAction SilentlyContinue
        foreach ($drv in $drives) {
          if ($drv.Model) { $model = [string]$drv.Model }
          if ($drv.InterfaceType) { $iface = [string]$drv.InterfaceType }
          if ($drv.Status) { $opStatus = [string]$drv.Status; $health = 'Healthy' }
          break
        }
        if ($model) { break }
      }
    }
  } catch {}
}

if (-not $model) { $model = "Disk $letter:" }
if (-not $health) { $health = 'Healthy' }
if (-not $opStatus) { $opStatus = 'OK' }

$obj = [ordered]@{
  Model = $model
  SerialNumber = $serial
  MediaType = $media
  InterfaceType = $iface
  HealthStatus = $health
  OperationalStatus = $opStatus
  Temperature = $temp
  Wear = $wear
  PowerOnHours = $poh
  StartStopCycle = $startStop
  ReadErrors = $readErr
  WriteErrors = $writeErr
  SizeBytes = $size
}
$obj | ConvertTo-Json -Compress`

	out, err := runHiddenOutput(systemPowerShell(), "-NoProfile", "-NonInteractive", "-Command", ps)
	defer debug.FreeOSMemory()
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
	sizeBytes := asFloat(parsed["SizeBytes"])
	return finalizeDiskHealthReport(report, wear, sizeBytes)
}

// finalizeDiskHealthReport applies wear/health scoring without inventing percentages
// when the Storage Reliability wear counter is missing (wear < 0).
func finalizeDiskHealthReport(report DiskHealthReport, wear, sizeBytes float64) DiskHealthReport {
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
		if sizeBytes > 0 && wear > 0 {
			report.TBWGB = (sizeBytes * (wear / 100.0)) / (1024 * 1024 * 1024)
		}
	} else {
		// Wear counter missing: never invent 50%/70%/100% health.
		report.WearRemaining = -1
		report.HealthPercent = -1
	}

	switch {
	case report.HasCriticalErrors || strings.EqualFold(report.HealthStatus, "Unhealthy"):
		report.Verdict = "Внимание"
		report.VerdictBadge = "⚠ Внимание"
	case strings.EqualFold(report.HealthStatus, "Warning"):
		report.Verdict = "Внимание"
		report.VerdictBadge = "⚠ Внимание"
	case report.WearRemaining >= 0:
		report.Verdict = fmt.Sprintf("Отличное (%.0f%%)", report.HealthPercent)
		report.VerdictBadge = fmt.Sprintf("✓ Состояние: Отличное (%.0f%%)", report.HealthPercent)
	case strings.EqualFold(report.HealthStatus, "Healthy") || strings.EqualFold(report.OperationalStatus, "OK"):
		// Disk reports healthy, wear % is not applicable (e.g. SATA HDD).
		report.Verdict = "Исправен"
		report.VerdictBadge = "✓ Исправен"
	default:
		report.Verdict = "Недоступно"
		report.VerdictBadge = "N/A"
	}
	// OK means we have a usable health verdict, not merely that a model string came back.
	report.OK = report.Verdict != "Недоступно" && report.VerdictBadge != "N/A"
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

// sanitizeDriveLetter forces a single A-Z letter for Storage queries (no injection).
func sanitizeDriveLetter(driveLetter string) string {
	letter := strings.ToUpper(strings.TrimSpace(driveLetter))
	letter = strings.TrimSuffix(letter, `:\`)
	letter = strings.TrimSuffix(letter, ":")
	if len(letter) != 1 || letter[0] < 'A' || letter[0] > 'Z' {
		return "C"
	}
	return letter
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
