package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/robbled17/zeropulse/loc"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// DiskHealthReport is a real SMART / Storage Reliability snapshot.
type DiskHealthReport struct {
	DriveLetter          string           `json:"driveLetter"`
	Model                string           `json:"model"`
	SerialNumber         string           `json:"serialNumber"`
	Firmware             string           `json:"firmware"`
	MediaType            string           `json:"mediaType"` // SSD / HDD / NVMe SSD
	MediaKind            string           `json:"mediaKind"` // ssd|hdd|nvme
	BusInterface         string           `json:"busInterface"` // SATA|NVMe|USB
	InterfaceType        string           `json:"interfaceType"`
	RotationRPM          float64          `json:"rotationRPM"`
	CapacityBytes        float64          `json:"capacityBytes"`
	CapacityGB           float64          `json:"capacityGB"`
	HealthStatus         string           `json:"healthStatus"`
	OperationalStatus    string           `json:"operationalStatus"`
	StatusLevel          string           `json:"statusLevel"` // good|caution|bad|unknown
	StatusCode           string           `json:"statusCode"`  // diskStateGood|…
	StatusWordRu         string           `json:"statusWordRu"`
	StatusWordEn         string           `json:"statusWordEn"`
	StatusReason         loc.Msg          `json:"statusReason"`
	StatusReasonRu       string           `json:"statusReasonRu"`
	StatusReasonEn       string           `json:"statusReasonEn"`
	InfoNote             loc.Msg          `json:"infoNote"`
	TriggerAttrID        int              `json:"triggerAttrId"`
	TriggerAttrHex       string           `json:"triggerAttrHex"`
	ProfileID            string           `json:"profileId"`
	Verdict              string           `json:"verdict"`
	VerdictBadge         string           `json:"verdictBadge"`
	HealthPercent        float64          `json:"healthPercent"`
	HealthPercentOK      bool             `json:"healthPercentOK"`
	HealthSource         string           `json:"healthSource"`
	HealthSourceMsg      loc.Msg          `json:"healthSourceMsg"`
	HealthSourceRu       string           `json:"healthSourceRu"`
	HealthSourceEn       string           `json:"healthSourceEn"`
	TemperatureC         float64          `json:"temperatureC"`
	TempAvailable        bool             `json:"tempAvailable"`
	WearRemaining        float64          `json:"wearRemaining"`
	PowerOnHours         float64          `json:"powerOnHours"`
	PowerOnHuman         loc.Msg          `json:"powerOnHuman"`
	PowerOnHumanRu       string           `json:"powerOnHumanRu"`
	PowerOnHumanEn       string           `json:"powerOnHumanEn"`
	PowerCycles          float64          `json:"powerCycles"`
	TBWGB                float64          `json:"tbwGB"`
	HostWriteTB          float64          `json:"hostWriteTB"`
	HostReadTB           float64          `json:"hostReadTB"`
	HostWriteGB          float64          `json:"hostWriteGB"`
	HostReadGB           float64          `json:"hostReadGB"`
	ReallocatedSectors   int64            `json:"reallocatedSectors"`
	PendingSectors       int64            `json:"pendingSectors"`
	UncorrectableSectors int64            `json:"uncorrectableSectors"`
	HasCriticalErrors    bool             `json:"hasCriticalErrors"`
	ReadErrors           int64            `json:"readErrors"`
	WriteErrors          int64            `json:"writeErrors"`
	Attributes           []SmartAttribute `json:"attributes"`
	MissingFieldsRu      []string         `json:"missingFieldsRu"`
	MissingFieldsEn      []string         `json:"missingFieldsEn"`
	MissingSummaryRu     string           `json:"missingSummaryRu"`
	MissingSummaryEn     string           `json:"missingSummaryEn"`
	CheckedAt            string           `json:"checkedAt"`
	DiskIndex            int              `json:"diskIndex"`
	NeedsAdmin           bool             `json:"needsAdmin"`
	AdminBannerRu        string           `json:"adminBannerRu"`
	AdminBannerEn        string           `json:"adminBannerEn"`
	PartialReasonRu      string           `json:"partialReasonRu"`
	PartialReasonEn      string           `json:"partialReasonEn"`
	FromCache            bool             `json:"fromCache"`
	ErrorMessage         string           `json:"errorMessage"`
	OK                   bool             `json:"ok"`
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
	Name             string  `json:"name"`
	DriverVersion    string  `json:"driverVersion"`
	DedicatedMB      float64 `json:"dedicatedMB"`
	SharedMB         float64 `json:"sharedMB"`
	UsedMB           float64 `json:"usedMB"`
	Percent          float64 `json:"percent"`
	IsIntegrated     bool    `json:"isIntegrated"`
	VRAMGB           float64 `json:"vramGB"`
	TemperatureC     float64 `json:"temperatureC"`
	TempAvailable    bool    `json:"tempAvailable"`
	PowerW           float64 `json:"powerW"`
	PowerAvailable   bool    `json:"powerAvailable"`
	ClockCoreMHz     float64 `json:"clockCoreMHz"`
	ClockMemMHz      float64 `json:"clockMemMHz"`
}

type RAMDeepInfo struct {
	TotalMB         uint64  `json:"totalMB"`
	UsedMB          uint64  `json:"usedMB"`
	FreeMB          uint64  `json:"freeMB"`
	Percent         float64 `json:"percent"`
	Type            string  `json:"type"`
	SlotsUsed       int     `json:"slotsUsed"`
	SlotsTotal      int     `json:"slotsTotal"`
	SpeedMHz        int     `json:"speedMHz"`
	IsSingleChannel bool    `json:"isSingleChannel"`
	ChannelStatus   string  `json:"channelStatus"`
	ModuleModel     string  `json:"moduleModel,omitempty"`
	PartNumber      string  `json:"partNumber,omitempty"`
	Manufacturer    string  `json:"manufacturer,omitempty"`
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
	PowerScheme       string  `json:"powerScheme,omitempty"`
	CPUWatts          float64 `json:"cpuWatts,omitempty"`
	GPUWatts          float64 `json:"gpuWatts,omitempty"`
	TotalWatts        float64 `json:"totalWatts,omitempty"`
}

type staticHardware struct {
	cpuName         string
	physicalCores   int
	logicalCores    int
	frequencyMHz    float64
	gpuName         string
	gpuDriver       string
	gpuVRAM         uint64
	ramType         string
	ramSlotsUsed    int
	ramSlotsTotal   int
	ramSpeedMHz     int
	ramModuleModel  string
	ramPartNumber   string
	ramManufacturer string
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
	out.ramType, out.ramSlotsUsed, out.ramSlotsTotal, out.ramSpeedMHz,
		out.ramManufacturer, out.ramPartNumber, out.ramModuleModel = readRAMSlots()
	return out
}

func readGPUInfoDetailed() (name string, vram uint64, driver string) {
	name, vram, driver = readWMIGPU()
	if regName, regVRAM := readRegistryVRAM(name); regVRAM > vram {
		if regName != "" && !gpuNamesMatch(name, regName) {
			name = regName
			driver = ""
		}
		vram = regVRAM
	}
	if dxName, dxVRAM := readDXGIVRAM(name); dxVRAM > vram {
		if dxName != "" && !gpuNamesMatch(name, dxName) {
			name = dxName
			driver = ""
		}
		vram = dxVRAM
	}
	if known := knownDiscreteVRAMBytes(name); known > vram && vramBytesLookCapped(vram) {
		vram = known
	}
	vram = snapVRAMBytes(vram)
	return name, vram, driver
}

func readWMIGPU() (name string, vram uint64, driver string) {
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
		// AdapterRAM is a 32-bit UINT. 8 GB cards saturate near 4 GB or come back negative.
		ram := adapterRAMBytes(ramVar.Val)
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

// adapterRAMBytes interprets Win32_VideoController.AdapterRAM without treating a signed overflow as empty.
func adapterRAMBytes(val int64) uint64 {
	if val <= 0 {
		if val < 0 {
			return uint64(uint32(val))
		}
		return 0
	}
	return uint64(val)
}

func vramBytesLookCapped(b uint64) bool {
	if b == 0 {
		return true
	}
	const gib = 1024 * 1024 * 1024
	// 32-bit ceiling sits just under 4 GiB.
	return b <= 4*gib+64*1024*1024
}

func knownDiscreteVRAMBytes(name string) uint64 {
	n := strings.ToLower(name)
	const gib = 1024 * 1024 * 1024
	switch {
	case strings.Contains(n, "2060 super"):
		return 8 * gib
	default:
		return 0
	}
}

func snapVRAMBytes(b uint64) uint64 {
	if b == 0 {
		return 0
	}
	gb := float64(b) / (1024 * 1024 * 1024)
	rounded := math.Round(gb)
	if rounded >= 1 && math.Abs(gb-rounded) <= 0.2 {
		return uint64(rounded) * 1024 * 1024 * 1024
	}
	return b
}

func normGPUKey(s string) string {
	s = strings.ToLower(s)
	r := strings.NewReplacer("nvidia", "", "geforce", "", "amd", "", "radeon", "", "(r)", "", "(tm)", "", " ", "", "-", "")
	return r.Replace(s)
}

func gpuNamesMatch(a, b string) bool {
	na, nb := normGPUKey(a), normGPUKey(b)
	if na == "" || nb == "" || na == "unknowngpu" || nb == "unknowngpu" {
		return false
	}
	return strings.Contains(na, nb) || strings.Contains(nb, na)
}

func readRegistryVRAM(matchName string) (string, uint64) {
	const base = `SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}`
	root, err := registry.OpenKey(registry.LOCAL_MACHINE, base, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return "", 0
	}
	defer root.Close()
	subs, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return "", 0
	}

	var bestName string
	var best, matched uint64
	var matchedName string
	for _, sub := range subs {
		if len(sub) != 4 {
			continue
		}
		sk, err := registry.OpenKey(registry.LOCAL_MACHINE, base+`\`+sub, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		desc, _, _ := sk.GetStringValue("DriverDesc")
		mem, _, memErr := sk.GetIntegerValue("HardwareInformation.qwMemorySize")
		if memErr != nil || mem == 0 {
			if v, _, e2 := sk.GetIntegerValue("HardwareInformation.MemorySize"); e2 == nil && v > mem {
				mem = v
			}
		}
		sk.Close()
		low := strings.ToLower(desc)
		if desc == "" || strings.Contains(low, "basic display") || strings.Contains(low, "basic render") || strings.Contains(low, "remote desktop") {
			continue
		}
		if mem > best {
			best = mem
			bestName = desc
		}
		if gpuNamesMatch(desc, matchName) && mem > matched {
			matched = mem
			matchedName = desc
		}
	}
	if matched > 0 {
		return matchedName, matched
	}
	return bestName, best
}

var (
	modDXGI               = windows.NewLazySystemDLL("dxgi.dll")
	procCreateDXGIFactory = modDXGI.NewProc("CreateDXGIFactory")
	iidIDXGIFactory       = windows.GUID{Data1: 0x7b7166ec, Data2: 0x21c7, Data3: 0x44ae, Data4: [8]byte{0xb2, 0x1a, 0xc9, 0xae, 0x32, 0x1a, 0xe3, 0x69}}
)

type dxgiAdapterDesc struct {
	Description           [128]uint16
	VendorID              uint32
	DeviceID              uint32
	SubSysID              uint32
	Revision              uint32
	DedicatedVideoMemory  uint64
	DedicatedSystemMemory uint64
	SharedSystemMemory    uint64
	LuidLow               uint32
	LuidHigh              int32
}

func comRelease(obj uintptr) {
	if obj == 0 {
		return
	}
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	release := *(*uintptr)(unsafe.Pointer(vtbl + 2*unsafe.Sizeof(uintptr(0))))
	_, _, _ = syscall.SyscallN(release, obj)
}

func vtblFunc(obj uintptr, index uintptr) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	return *(*uintptr)(unsafe.Pointer(vtbl + index*unsafe.Sizeof(uintptr(0))))
}

// readDXGIVRAM reads DXGI_ADAPTER_DESC.DedicatedVideoMemory (full 64-bit, not the 4 GB WMI cap).
func readDXGIVRAM(matchName string) (string, uint64) {
	if err := procCreateDXGIFactory.Find(); err != nil {
		return "", 0
	}
	var factory uintptr
	hr, _, _ := procCreateDXGIFactory.Call(
		uintptr(unsafe.Pointer(&iidIDXGIFactory)),
		uintptr(unsafe.Pointer(&factory)),
	)
	if hr != 0 || factory == 0 {
		return "", 0
	}
	defer comRelease(factory)

	enumAdapters := vtblFunc(factory, 7) // IDXGIFactory::EnumAdapters
	var bestName, matchedName string
	var best, matched uint64
	for i := uintptr(0); i < 16; i++ {
		var adapter uintptr
		r, _, _ := syscall.SyscallN(enumAdapters, factory, i, uintptr(unsafe.Pointer(&adapter)))
		if r != 0 || adapter == 0 {
			break
		}
		var desc dxgiAdapterDesc
		getDesc := vtblFunc(adapter, 8) // IDXGIAdapter::GetDesc
		r2, _, _ := syscall.SyscallN(getDesc, adapter, uintptr(unsafe.Pointer(&desc)))
		name := strings.TrimSpace(windows.UTF16ToString(desc.Description[:]))
		mem := desc.DedicatedVideoMemory
		comRelease(adapter)
		if r2 != 0 {
			continue
		}
		low := strings.ToLower(name)
		if name == "" || strings.Contains(low, "microsoft basic") || strings.Contains(low, "remote") {
			continue
		}
		if mem > best {
			best = mem
			bestName = name
		}
		if gpuNamesMatch(name, matchName) && mem > matched {
			matched = mem
			matchedName = name
		}
	}
	if matched > 0 {
		return matchedName, matched
	}
	return bestName, best
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

func readRAMSlots() (memType string, used, total, speed int, manufacturer, partNumber, moduleModel string) {
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
		"SELECT MemoryType, SMBIOSMemoryType, Speed, Capacity, Manufacturer, PartNumber FROM Win32_PhysicalMemory")
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
	var totalCap uint64
	parts := make([]string, 0, count)
	mfgs := make([]string, 0, count)
	for i := 0; i < count; i++ {
		itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
		if err != nil {
			continue
		}
		item := itemRaw.ToIDispatch()
		smbiosVar, _ := oleutil.GetProperty(item, "SMBIOSMemoryType")
		memTypeVar, _ := oleutil.GetProperty(item, "MemoryType")
		speedVar, _ := oleutil.GetProperty(item, "Speed")
		capVar, _ := oleutil.GetProperty(item, "Capacity")
		mfgVar, _ := oleutil.GetProperty(item, "Manufacturer")
		partVar, _ := oleutil.GetProperty(item, "PartNumber")
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
		if capVar != nil {
			totalCap += variantCapacityBytes(capVar)
		}
		mfg := strings.TrimSpace(mfgVar.ToString())
		part := strings.TrimSpace(partVar.ToString())
		if mfg != "" && mfg != "<nil>" {
			mfgs = append(mfgs, mfg)
		}
		if part != "" && part != "<nil>" {
			parts = append(parts, part)
		}
	}
	memType = mapMemoryType(typeCode)
	if len(mfgs) > 0 {
		manufacturer = mfgs[0]
	}
	if len(parts) > 0 {
		partNumber = parts[0]
	}
	moduleModel = formatRAMModuleModel(manufacturer, partNumber, used, totalCap)

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

func cleanRAMManufacturer(mfg string) string {
	m := strings.TrimSpace(mfg)
	m = strings.Trim(m, "\x00")
	low := strings.ToLower(m)
	if m == "" || strings.HasPrefix(low, "0x") || low == "unknown" || low == "undefined" || low == "<nil>" {
		return ""
	}
	return m
}

func brandFromPartNumber(part string) string {
	p := strings.ToUpper(strings.TrimSpace(part))
	p = strings.ReplaceAll(p, " ", "")
	switch {
	case strings.HasPrefix(p, "KF"), strings.Contains(p, "KINGSTON"):
		if strings.Contains(p, "FURY") || strings.HasPrefix(p, "KF4") || strings.HasPrefix(p, "KF5") || strings.HasPrefix(p, "KF3") {
			return "Kingston Fury"
		}
		return "Kingston"
	case strings.HasPrefix(p, "CMK"), strings.HasPrefix(p, "CMW"), strings.HasPrefix(p, "CMH"), strings.HasPrefix(p, "CMV"), strings.HasPrefix(p, "CMT"):
		return "Corsair Vengeance"
	case strings.HasPrefix(p, "F4-"), strings.HasPrefix(p, "F5-"), strings.HasPrefix(p, "F3-"):
		return "G.Skill"
	case strings.HasPrefix(p, "BL"), strings.Contains(p, "BALLISTIX"):
		return "Crucial Ballistix"
	case strings.HasPrefix(p, "CT"):
		return "Crucial"
	case strings.Contains(p, "TEAM"), strings.HasPrefix(p, "TCD"), strings.HasPrefix(p, "TF"):
		return "TeamGroup"
	case strings.Contains(p, "ADATA"), strings.HasPrefix(p, "AX"):
		return "ADATA"
	case strings.Contains(p, "PATRIOT"):
		return "Patriot"
	default:
		return ""
	}
}

// variantCapacityBytes reads Win32_PhysicalMemory.Capacity (UINT64 bytes).
// go-ole Val is unreliable for VT_UI8 - prefer Value() with sanity bounds.
func variantCapacityBytes(v *ole.VARIANT) uint64 {
	if v == nil {
		return 0
	}
	const (
		minStick = uint64(256) * 1024 * 1024        // 256 MB
		maxStick = uint64(256) * 1024 * 1024 * 1024 // 256 GB
	)
	try := func(n uint64) uint64 {
		if n >= minStick && n <= maxStick {
			return n
		}
		return 0
	}
	switch val := v.Value().(type) {
	case uint64:
		if n := try(val); n > 0 {
			return n
		}
	case int64:
		if val > 0 {
			if n := try(uint64(val)); n > 0 {
				return n
			}
		}
	case float64:
		if val > 0 {
			if n := try(uint64(val)); n > 0 {
				return n
			}
		}
	case string:
		s := strings.TrimSpace(val)
		if s != "" {
			var parsed uint64
			if _, err := fmt.Sscanf(s, "%d", &parsed); err == nil {
				if n := try(parsed); n > 0 {
					return n
				}
			}
		}
	}
	if v.Val > 0 {
		if n := try(uint64(v.Val)); n > 0 {
			return n
		}
	}
	return 0
}

func formatRAMModuleModel(mfg, part string, sticks int, totalBytes uint64) string {
	part = strings.TrimSpace(strings.Trim(part, "\x00"))
	brand := cleanRAMManufacturer(mfg)
	if brand == "" {
		brand = brandFromPartNumber(part)
	}
	gbEach := 0
	if sticks > 0 && totalBytes > 0 {
		per := totalBytes / uint64(sticks)
		gbEach = int(per / (1024 * 1024 * 1024))
		// Round common sizes (7.x → 8) when WMI is slightly short of a full GiB.
		if rem := per % (1024 * 1024 * 1024); rem > 0 && gbEach > 0 && rem > 512*1024*1024 {
			gbEach++
		}
	}
	// Absurd per-stick sizes mean Capacity was mis-read - omit size rather than show 2×2353 ГБ.
	if gbEach > 128 || (sticks > 0 && totalBytes > 0 && gbEach == 0) {
		gbEach = 0
	}
	name := brand
	if name == "" {
		name = "RAM"
	}
	if sticks > 0 && gbEach > 0 {
		name = fmt.Sprintf("%s %d × %d ГБ", name, sticks, gbEach)
	} else if sticks > 1 {
		name = fmt.Sprintf("%s (%d модулей)", name, sticks)
	}
	if part != "" {
		return fmt.Sprintf("%s (Part: %s)", name, part)
	}
	if brand == "" && gbEach == 0 {
		return ""
	}
	return name
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
		cpuW, gpuW := estimateComponentWatts(static.cpuName, static.gpuName, cpuPct, gpuPct)
		scheme := activePowerSchemeName()
		total := cpuW + gpuW
		power.HasBattery = false
		power.DeviceName = "Настольный ПК (Сетевое питание 220V)"
		power.StatusText = "Режим Windows: " + scheme
		power.WearLabel = fmt.Sprintf("оценка: CPU %.0fW + GPU %.0fW ≈ %.0fW", cpuW, gpuW, total)
		power.PowerScheme = scheme
		power.CPUWatts = cpuW
		power.GPUWatts = gpuW
		power.TotalWatts = total
		power.PowerDrawWatts = total
		power.PluggedIn = true
		power.Percent = 100
		power.CapacitymWh = 0
		power.DesignCapacitymWh = 0
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
		RAM:   buildRAMDeepInfo(static, totalMB, usedMB, freeMB, pct),
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
		ModuleModel:     static.ramModuleModel,
		PartNumber:      static.ramPartNumber,
		Manufacturer:    static.ramManufacturer,
	}
}

const diskSMARTCacheTTL = 60 * time.Second
const diskSMARTTimeout = 5 * time.Second

// CheckDiskHealth returns cached SMART (≤60s) or refreshes in-process with a 5s timeout.
func (a *App) CheckDiskHealth(driveLetter string) DiskHealthReport {
	return a.checkDiskHealth(driveLetter, false)
}

// CheckDiskHealthForce bypasses the 60s cache.
func (a *App) CheckDiskHealthForce(driveLetter string) DiskHealthReport {
	return a.checkDiskHealth(driveLetter, true)
}

func (a *App) checkDiskHealth(driveLetter string, force bool) DiskHealthReport {
	letter := strings.ToUpper(strings.TrimSpace(driveLetter))
	letter = strings.TrimSuffix(letter, `:\`)
	letter = strings.TrimSuffix(letter, ":")
	if letter == "" {
		letter = "C"
	}
	if !force {
		if cached, ok := a.cachedDiskSMARTFresh(letter); ok {
			cached.FromCache = true
			return cached
		}
	}
	ch := make(chan DiskHealthReport, 1)
	go func() {
		ch <- a.checkDiskHealthSync(letter)
	}()
	select {
	case r := <-ch:
		return r
	case <-time.After(diskSMARTTimeout):
		if cached, ok := a.cachedSystemDiskByLetter(letter); ok {
			cached.FromCache = true
			cached.PartialReasonRu = "чтение SMART превысило 5 с - показан кэш"
			cached.PartialReasonEn = "SMART read exceeded 5s - showing cache"
			return cached
		}
		return DiskHealthReport{
			DriveLetter: letter + ":", OK: false,
			StatusLevel: DiskStatusUnknown,
			StatusWordRu: "Неизвестно", StatusWordEn: "Unknown",
			ErrorMessage: "таймаут чтения SMART (5 с)",
			PartialReasonRu: "не удалось прочитать SMART за 5 с",
			PartialReasonEn: "SMART could not be read within 5s",
		}
	}
}

func (a *App) cachedDiskSMARTFresh(letter string) (DiskHealthReport, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.diskSMARTCache == nil || a.diskSMARTCacheAt == nil {
		return DiskHealthReport{}, false
	}
	r, ok := a.diskSMARTCache[letter]
	at, okAt := a.diskSMARTCacheAt[letter]
	if !ok || !okAt || !r.OK {
		return DiskHealthReport{}, false
	}
	if time.Since(at) > diskSMARTCacheTTL {
		return DiskHealthReport{}, false
	}
	return r, true
}

func (a *App) cachedSystemDiskByLetter(letter string) (DiskHealthReport, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.diskSMARTCache == nil {
		return DiskHealthReport{}, false
	}
	r, ok := a.diskSMARTCache[letter]
	return r, ok && r.OK
}

// RefreshAllDiskHealth refreshes SMART for the given letters in parallel (5s each).
func (a *App) RefreshAllDiskHealth(letters []string) []DiskHealthReport {
	if len(letters) == 0 {
		letters = []string{"C"}
	}
	out := make([]DiskHealthReport, len(letters))
	var wg sync.WaitGroup
	for i, L := range letters {
		wg.Add(1)
		go func(i int, L string) {
			defer wg.Done()
			out[i] = a.checkDiskHealth(L, true)
		}(i, L)
	}
	wg.Wait()
	return out
}

func (a *App) checkDiskHealthSync(letter string) DiskHealthReport {
	if advancedModeEnabled() {
		if r, ok := diskReportFromHelper(letter); ok {
			a.applyDiskWatchToReport(&r)
			a.cacheDiskSMART(r)
			return r
		}
	}

	ps := fmt.Sprintf(`
$OutputEncoding = New-Object System.Text.UTF8Encoding $false
[Console]::OutputEncoding = $OutputEncoding
$ErrorActionPreference='Stop'
function Emit($h) { $h | ConvertTo-Json -Compress }
try {
  $letter = '%s'
  $ld = Get-CimInstance -ClassName Win32_LogicalDisk -Filter "DeviceID='${letter}:'" -ErrorAction Stop
  if ($null -eq $ld) { Emit @{ ErrorCode = 'fs_unsupported' }; return }
  $driveType = [int]$ld.DriveType
  if ($driveType -eq 2) { Emit @{ ErrorCode = 'removable' }; return }
  if ($driveType -eq 4) { Emit @{ ErrorCode = 'virtual' }; return }

  $needle = 'DeviceID="' + ${letter} + ':"'
  $partLink = Get-CimInstance -ClassName Win32_LogicalDiskToPartition -ErrorAction SilentlyContinue |
    Where-Object { ([string]$_.Dependent).Contains($needle) } |
    Select-Object -First 1
  $partition = $null
  if ($null -ne $partLink) {
    if ([string]$partLink.Antecedent -match 'DeviceID="([^"]+)"') { $partName = $Matches[1] } else { $partName = '' }
    if ($partName) {
      $partition = Get-CimInstance -ClassName Win32_DiskPartition | Where-Object { $_.DeviceID -eq $partName } | Select-Object -First 1
    }
  }
  if ($null -eq $partition) {
    $partition = Get-CimAssociatedInstance -InputObject $ld -ResultClassName Win32_DiskPartition -ErrorAction SilentlyContinue | Select-Object -First 1
  }

  $physDrive = $null
  if ($null -ne $partition) {
    $diskLink = Get-CimInstance -ClassName Win32_DiskDriveToDiskPartition -ErrorAction SilentlyContinue |
      Where-Object { ([string]$_.Dependent).Contains([string]$partition.DeviceID) } |
      Select-Object -First 1
    if ($null -ne $diskLink) {
      if ([string]$diskLink.Antecedent -match 'DeviceID="([^"]+)"') { $diskName = $Matches[1] } else { $diskName = '' }
      if ($diskName) {
        $physDrive = Get-CimInstance -ClassName Win32_DiskDrive | Where-Object { $_.DeviceID -eq $diskName } | Select-Object -First 1
      }
    }
    if ($null -eq $physDrive) {
      $physDrive = Get-CimAssociatedInstance -InputObject $partition -ResultClassName Win32_DiskDrive -ErrorAction SilentlyContinue | Select-Object -First 1
    }
  }
  if ($null -eq $physDrive) {
    Emit @{ ErrorCode = 'fs_unsupported'; Error = 'Physical disk not found' }
    return
  }

  $ifaceRaw = [string]$physDrive.InterfaceType
  $mediaHint = [string]$physDrive.MediaType
  $model = [string]$physDrive.Model
  $serial = [string]$physDrive.SerialNumber
  $firmware = [string]$physDrive.FirmwareRevision
  $sizeBytes = [double]$physDrive.Size
  $pnp = [string]$physDrive.PNPDeviceID
  $index = [int]$physDrive.Index

  if ($ifaceRaw -match 'USB' -or $mediaHint -match 'Removable') {
    Emit @{ ErrorCode = 'removable' }
    return
  }
  if ($model -match 'Virtual|VBOX|VMware|QEMU' -or ($ifaceRaw -match 'SCSI' -and $model -match 'Virtual')) {
    Emit @{ ErrorCode = 'virtual' }
    return
  }

  $disk = $null
  $phys = $null
  $rel = $null
  try { $disk = Get-Disk -Number $index -ErrorAction SilentlyContinue } catch {}
  try {
    if ($null -ne $disk) { $phys = $disk | Get-PhysicalDisk -ErrorAction SilentlyContinue }
    if ($null -eq $phys) { $phys = Get-PhysicalDisk -ErrorAction SilentlyContinue | Where-Object { $_.DeviceId -eq $index -or $_.FriendlyName -eq $model } | Select-Object -First 1 }
  } catch {}
  try {
    if ($null -ne $phys) { $rel = Get-StorageReliabilityCounter -PhysicalDisk $phys -ErrorAction SilentlyContinue }
  } catch {}

  $bus = $ifaceRaw
  if ($null -ne $disk) { $bus = [string]$disk.BusType }
  $media = $mediaHint
  if ($null -ne $phys -and [string]$phys.MediaType) { $media = [string]$phys.MediaType }
  $spindle = 0.0
  try { if ($null -ne $phys -and $phys.SpindleSpeed) { $spindle = [double]$phys.SpindleSpeed } } catch {}
  $iface = $bus
  $isHDD = $false
  if ($media -match 'SSD' -or $media -eq '3' -or $media -eq '4') {
    if ($bus -match 'NVMe|RAID' -or $media -eq '4') { $iface = 'NVMe' }
    elseif ($bus -match 'SATA|ATA|IDE') { $iface = 'SATA' }
    else { $iface = 'SATA' }
  } elseif ($media -match 'HDD|Fixed hard disk' -or $media -eq '2' -or ($spindle -gt 0)) {
    $iface = 'SATA'
    $isHDD = $true
    if ($bus -match 'USB') { $iface = 'USB' }
  }
  if ($bus -match 'USB') { $iface = 'USB' }
  if ($bus -match 'NVMe') { $iface = 'NVMe' }

  $health = 'Healthy'
  $op = 'OK'
  $wear = -1.0
  $temp = 0.0
  $poh = 0.0
  $readErr = [int64]0
  $writeErr = [int64]0
  $smartOK = $false

  if ($null -ne $phys) {
    $health = [string]$phys.HealthStatus
    $op = [string]$phys.OperationalStatus
  }
  if ($null -ne $rel) {
    try { $temp = [double]$rel.Temperature } catch {}
    try { if ($null -ne $rel.Wear) { $wear = [double]$rel.Wear } } catch {}
    try { if ($rel.PowerOnHours) { $poh = [double]$rel.PowerOnHours } } catch {}
    try { $readErr = [int64]$rel.ReadErrorsUncorrected } catch {}
    try { $writeErr = [int64]$rel.WriteErrorsUncorrected } catch {}
  }

  # SMART failure predict (HDD / SATA) via root\wmi
  try {
    $preds = Get-CimInstance -Namespace root\wmi -ClassName MSStorageDriver_FailurePredictStatus -ErrorAction SilentlyContinue
    foreach ($pr in @($preds)) {
      $inst = [string]$pr.InstanceName
      $pnpTail = ($pnp -split '[\\]')[-1]
      if ($pnp -and $inst -and ($inst.Contains($pnpTail) -or $inst.Contains($pnp))) {
        if ([bool]$pr.PredictFailure) {
          $health = 'Unhealthy'
          $op = 'SMART FAIL'
          $smartOK = $false
        } else {
          $smartOK = $true
          if ([string]::IsNullOrWhiteSpace($health) -or $health -eq '0') { $health = 'Healthy' }
        }
        break
      }
    }
    if (-not $smartOK -and $isHDD -and $null -ne $preds) {
      # Fallback: single-disk systems often expose one predict row
      $one = @($preds) | Select-Object -First 1
      if ($null -ne $one -and -not [bool]$one.PredictFailure) {
        $smartOK = $true
        if ([string]::IsNullOrWhiteSpace($health) -or $health -eq '0') { $health = 'Healthy' }
      }
    }
  } catch {}

  if ($isHDD -and $smartOK -and $wear -lt 0) {
    $wear = 0
    $health = 'Healthy'
  }
  if ($null -ne $phys -and [string]$phys.FriendlyName) { $model = [string]$phys.FriendlyName }
  if ($null -ne $disk -and [string]$disk.SerialNumber) { $serial = [string]$disk.SerialNumber }
  if ($sizeBytes -le 0 -and $null -ne $disk) { $sizeBytes = [double]$disk.Size }

  Emit ([ordered]@{
    Model = $model
    SerialNumber = $serial
    Firmware = $firmware
    MediaType = $media
    InterfaceType = $iface
    HealthStatus = $health
    OperationalStatus = $op
    Temperature = $temp
    Wear = $wear
    PowerOnHours = $poh
    ReadErrors = $readErr
    WriteErrors = $writeErr
    SizeBytes = $sizeBytes
    DiskIndex = $index
    BusHint = $bus
    SpindleSpeed = $spindle
    SmartOK = [bool]$smartOK
  })
} catch {
  $m = [string]$_.Exception.Message
  if ($m -match 'Access is denied|отказано') {
    Emit @{ ErrorCode = 'error'; Error = $m }
  } else {
    Emit @{ ErrorCode = 'error'; Error = $m }
  }
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

	raw := decodePowerShellUTF8(out)
	if i := strings.Index(raw, "{"); i > 0 {
		raw = raw[i:]
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		report.ErrorMessage = "Не удалось разобрать ответ Storage API"
		report.Verdict = "Недоступно"
		return report
	}
	if code, _ := parsed["ErrorCode"].(string); code != "" {
		switch code {
		case "removable":
			report.ErrorMessage = "Неприменимо / Съемный накопитель"
			report.Verdict = "Неприменимо"
		case "virtual":
			report.ErrorMessage = "Неприменимо / Виртуальный диск"
			report.Verdict = "Неприменимо"
		case "reserved":
			report.ErrorMessage = "Неприменимо / Служебный раздел"
			report.Verdict = "Неприменимо"
		case "fs_unsupported":
			report.ErrorMessage = "Неприменимо / Файловая система не поддерживается"
			report.Verdict = "Неприменимо"
		default:
			report.ErrorMessage = "Неприменимо / Съемный накопитель"
			report.Verdict = "Неприменимо"
		}
		return report
	}
	if errMsg, ok := parsed["Error"].(string); ok && errMsg != "" {
		report.ErrorMessage = errMsg
		report.Verdict = "Недоступно"
		return report
	}

	report.Model = asString(parsed["Model"])
	report.SerialNumber = asString(parsed["SerialNumber"])
	report.Firmware = asString(parsed["Firmware"])
	mediaRaw := asString(parsed["MediaType"])
	busRaw := asString(parsed["BusHint"])
	ifaceRaw := asString(parsed["InterfaceType"])
	report.MediaType = normalizeMediaType(mediaRaw)
	report.BusInterface = classifyBusInterface(busRaw, ifaceRaw)
	report.InterfaceType = report.BusInterface
	report.RotationRPM = asFloat(parsed["SpindleSpeed"])
	report.HealthStatus = asString(parsed["HealthStatus"])
	report.OperationalStatus = asString(parsed["OperationalStatus"])
	report.TemperatureC = asFloat(parsed["Temperature"])
	report.TempAvailable = report.TemperatureC > 0
	report.PowerOnHours = asFloat(parsed["PowerOnHours"])
	report.ReadErrors = asInt64(parsed["ReadErrors"])
	report.WriteErrors = asInt64(parsed["WriteErrors"])
	sizeBytes := asFloat(parsed["SizeBytes"])
	report.CapacityBytes = sizeBytes
	if sizeBytes > 0 {
		report.CapacityGB = sizeBytes / (1024 * 1024 * 1024)
	}
	report.DiskIndex = int(asFloat(parsed["DiskIndex"]))
	seek := querySeekPenalty(report.DiskIndex)
	mediaIn := MediaClassifyInput{
		Bus:               report.BusInterface,
		MSFTMediaType:     mediaRaw,
		RotationRPMHint:   int(report.RotationRPM),
		IncursSeekPenalty: seek,
	}
	media := ClassifyDiskMedia(mediaIn)
	kind := media.Kind
	if kind == DiskKindUnknown {
		kind = classifyDiskKind(report.MediaType, report.BusInterface, busRaw)
	}
	report.MediaKind = kind
	applyMediaLabels(&report, media, kind)
	preferNVMe := kind == DiskKindNVMe

	// Enrich with native NVMe / ATA SMART. Never invent 100% from Storage "Healthy".
	ioctl := readDiskSMARTIOCTL(report.DiskIndex, preferNVMe)
	if ioctl.NeedsAdmin {
		report.NeedsAdmin = true
		if !advancedModeEnabled() {
			report.AdminBannerRu = "Включите расширенный режим, чтобы видеть состояние дисков"
			report.AdminBannerEn = "Enable advanced mode to see disk health"
			report.StatusLevel = DiskStatusUnknown
			report.StatusWordRu = "Включите расширенный режим"
			report.StatusWordEn = "Enable advanced mode"
			report.Verdict = report.StatusWordRu
			report.VerdictBadge = report.StatusWordRu
		} else {
			report.AdminBannerRu = ioctl.AdminReasonRu
			report.AdminBannerEn = ioctl.AdminReasonEn
			if report.AdminBannerRu == "" {
				report.AdminBannerRu = "Помощник не отдал SMART - проверьте расширенный режим"
				report.AdminBannerEn = "Helper did not return SMART - check advanced mode"
			}
		}
	}
	smartOK := false
	evalIn := DiskHealthEvalInput{Model: report.Model, Kind: kind, SMARTReadOK: false}
	if ioctl.OK {
		smartOK = true
		evalIn.SMARTReadOK = true
		if ioctl.Firmware != "" && report.Firmware == "" {
			report.Firmware = ioctl.Firmware
		}
		if ioctl.TempOK {
			report.TemperatureC = ioctl.TemperatureC
			report.TempAvailable = true
		}
		if ioctl.PowerOnHours > 0 {
			report.PowerOnHours = ioctl.PowerOnHours
		}
		if ioctl.PowerCycles > 0 {
			report.PowerCycles = ioctl.PowerCycles
		}
		if ioctl.HostWriteGB > 0 {
			report.HostWriteGB = ioctl.HostWriteGB
			report.HostWriteTB = ioctl.HostWriteGB / 1024.0
			report.TBWGB = ioctl.HostWriteGB
		}
		if ioctl.HostReadGB > 0 {
			report.HostReadGB = ioctl.HostReadGB
			report.HostReadTB = ioctl.HostReadGB / 1024.0
		}
		report.ReallocatedSectors = ioctl.Reallocated
		report.PendingSectors = ioctl.Pending
		report.UncorrectableSectors = ioctl.Uncorrectable
		if len(ioctl.Attributes) > 0 {
			report.Attributes = ioctl.Attributes
		}
		if ioctl.Bus == "nvme" {
			kind = DiskKindNVMe
			report.MediaKind = kind
			report.MediaType = "SSD"
			report.BusInterface = "NVMe"
			report.InterfaceType = "NVMe"
			evalIn.Kind = DiskKindNVMe
			evalIn.NVMeCriticalWarn = ioctl.CriticalWarning
			evalIn.NVMeSpare = ioctl.AvailableSpare
			evalIn.NVMeSpareThresh = ioctl.AvailableSpareThresh
			evalIn.NVMePercentUsed = ioctl.PercentageUsed
			evalIn.NVMePercentUsedOK = true
			evalIn.NVMeMediaErrors = uint64(ioctl.MediaErrors)
		} else {
			evalIn.Attributes = report.Attributes
			evalIn.Reallocated = report.ReallocatedSectors
			evalIn.Pending = report.PendingSectors
			evalIn.Uncorrectable = report.UncorrectableSectors
			if ioctl.IdentifyWord217OK {
				mediaIn.IdentifyWord217 = ioctl.IdentifyWord217
				mediaIn.IdentifyWord217OK = true
				media = ClassifyDiskMedia(mediaIn)
				kind = media.Kind
				report.MediaKind = kind
				applyMediaLabels(&report, media, kind)
				evalIn.Kind = kind
			} else if kind == DiskKindUnknown {
				// Wear attrs suggest SSD, but never invent SSD without evidence.
				if findWearLifeID(report.Attributes) {
					kind = DiskKindSATASSD
					report.MediaKind = kind
					report.MediaType = "SSD"
					evalIn.Kind = kind
				} else if report.RotationRPM > 1 {
					kind = DiskKindHDD
					report.MediaKind = kind
					report.MediaType = "HDD"
					evalIn.Kind = kind
				}
			}
		}
	} else if ioctl.UnavailableRu != "" {
		report.PartialReasonRu = ioctl.UnavailableRu
		report.PartialReasonEn = ioctl.UnavailableEn
	}

	// Vendor host IO units (e.g. SMI F1/F2 = 32 MiB) override ATA LBA×512 defaults.
	if len(report.Attributes) > 0 {
		prof := detectDiskProfile(kind, report.Model, report.Attributes)
		if w, r, okW, okR := hostIOFromProfile(prof, report.Attributes); okW || okR {
			if okW {
				report.HostWriteGB = w
				report.HostWriteTB = w / 1024.0
				report.TBWGB = w
			}
			if okR {
				report.HostReadGB = r
				report.HostReadTB = r / 1024.0
			}
		}
	}
	if a != nil {
		pp, pr, okP, okR, _, _, _, _, _ := a.DiskWatchPrior(report.DriveLetter, 7)
		evalIn.PriorPending, evalIn.PriorPendingOK = pp, okP
		evalIn.PriorReallocated, evalIn.PriorReallocatedOK = pr, okR
	}
	ev := EvaluateDiskHealth(evalIn)
	applyDiskHealthEval(&report, ev)
	if a != nil {
		wear := 0.0
		if report.HealthPercentOK {
			wear = report.HealthPercent
		}
		temp := 0.0
		if report.TempAvailable {
			temp = report.TemperatureC
		}
		a.RecordDiskWatchSample(report.DriveLetter, report.PendingSectors, report.ReallocatedSectors, wear, temp, report.PowerOnHours)
		a.flushDiskWatch()
	}
	report.PowerOnHuman = humanPowerOnMsg(report.PowerOnHours)
	report.PowerOnHumanRu = humanPowerOn(report.PowerOnHours, true)
	report.PowerOnHumanEn = humanPowerOn(report.PowerOnHours, false)
	report.MissingFieldsRu, report.MissingFieldsEn, report.MissingSummaryRu, report.MissingSummaryEn =
		buildDiskMissingSummary(report, smartOK, report.NeedsAdmin)

	_ = smartOK
	report.CheckedAt = time.Now().Format(time.RFC3339)
	report.OK = true
	a.cacheDiskSMART(report)
	return report
}

// applyDiskWatchToReport re-evaluates with local trend priors and records a daily sample.
func (a *App) applyDiskWatchToReport(report *DiskHealthReport) {
	if a == nil || report == nil {
		return
	}
	pp, pr, okP, okR, _, _, _, _, _ := a.DiskWatchPrior(report.DriveLetter, 7)
	if (okP || okR) && len(report.Attributes) > 0 && report.StatusLevel != DiskStatusUnknown {
		evalIn := DiskHealthEvalInput{
			Kind: report.MediaKind, Model: report.Model, Attributes: report.Attributes,
			SMARTReadOK: true,
			Reallocated: report.ReallocatedSectors, Pending: report.PendingSectors,
			Uncorrectable: report.UncorrectableSectors,
			PriorPending: pp, PriorPendingOK: okP,
			PriorReallocated: pr, PriorReallocatedOK: okR,
		}
		// Preserve NVMe fields if present via percent used.
		if report.MediaKind == DiskKindNVMe && report.HealthPercentOK {
			used := int(100 - report.HealthPercent)
			if used < 0 {
				used = 0
			}
			evalIn.NVMePercentUsed = used
			evalIn.NVMePercentUsedOK = true
		}
		ev := EvaluateDiskHealth(evalIn)
		applyDiskHealthEval(report, ev)
	}
	wear := 0.0
	if report.HealthPercentOK {
		wear = report.HealthPercent
	}
	temp := 0.0
	if report.TempAvailable {
		temp = report.TemperatureC
	}
	a.RecordDiskWatchSample(report.DriveLetter, report.PendingSectors, report.ReallocatedSectors, wear, temp, report.PowerOnHours)
	a.flushDiskWatch()
}

func applyDiskHealthEval(report *DiskHealthReport, ev DiskHealthEval) {
	report.StatusLevel = ev.Level
	report.StatusCode = ev.StatusCode
	report.StatusWordRu = ev.StatusRu
	report.StatusWordEn = ev.StatusEn
	report.StatusReason = ev.Reason
	report.StatusReasonRu = ev.ReasonRu
	report.StatusReasonEn = ev.ReasonEn
	report.InfoNote = ev.InfoNote
	report.TriggerAttrID = ev.TriggerAttrID
	report.TriggerAttrHex = ev.TriggerAttrHex
	report.ProfileID = ev.ProfileID
	report.HealthPercent = ev.HealthPercent
	report.HealthPercentOK = ev.HealthPctOK
	report.HealthSource = ev.HealthSource
	report.HealthSourceMsg = ev.HealthSourceMsg
	report.HealthSourceRu = ev.HealthSourceRu
	report.HealthSourceEn = ev.HealthSourceEn
	if ev.HealthPctOK {
		report.WearRemaining = ev.HealthPercent
	}
	report.HasCriticalErrors = ev.Level == DiskStatusBad
	report.Verdict = ev.StatusRu
	report.VerdictBadge = ev.StatusRu
	if ev.HealthPctOK {
		report.VerdictBadge = fmt.Sprintf("%s · %.0f%%", ev.StatusRu, ev.HealthPercent)
	}
}

func findWearLifeID(attrs []SmartAttribute) bool {
	_, _, ok, _ := findWearLife("", attrs)
	return ok
}

func buildDiskMissingSummary(r DiskHealthReport, smartOK, needsAdmin bool) (ruFields, enFields []string, ruSum, enSum string) {
	add := func(have bool, ru, en string) {
		if have {
			return
		}
		ruFields = append(ruFields, ru)
		enFields = append(enFields, en)
	}
	add(r.HostWriteGB > 0, "всего записано", "total written")
	add(r.HostReadGB > 0, "всего прочитано", "total read")
	add(r.PowerCycles > 0, "число включений", "power cycles")
	add(r.PowerOnHours > 0, "время работы", "power-on time")
	add(r.TempAvailable, "температура", "temperature")
	add(r.Firmware != "", "прошивка", "firmware")
	add(r.HealthPercentOK || r.MediaKind == DiskKindHDD, "процент здоровья", "health percent")
	if len(ruFields) == 0 {
		return nil, nil, "", ""
	}
	reasonRu, reasonEn := "не поддерживается диском", "not reported by the disk"
	if needsAdmin {
		reasonRu, reasonEn = "нужны права администратора", "administrator rights required"
	} else if !smartOK {
		reasonRu, reasonEn = "не удалось прочитать SMART", "SMART could not be read"
	}
	ruSum = "Диск не сообщает: " + strings.Join(ruFields, ", ") + " (" + reasonRu + ")"
	enSum = "Disk does not report: " + strings.Join(enFields, ", ") + " (" + reasonEn + ")"
	return
}

func (a *App) cacheDiskSMART(r DiskHealthReport) {
	if a == nil {
		return
	}
	key := strings.ToUpper(strings.TrimSuffix(strings.TrimSpace(r.DriveLetter), ":"))
	if key == "" {
		return
	}
	a.mu.Lock()
	if a.diskSMARTCache == nil {
		a.diskSMARTCache = map[string]DiskHealthReport{}
	}
	if a.diskSMARTCacheAt == nil {
		a.diskSMARTCacheAt = map[string]time.Time{}
	}
	a.diskSMARTCache[key] = r
	a.diskSMARTCacheAt[key] = time.Now()
	a.mu.Unlock()
}

func (a *App) cachedSystemDiskSMART() (DiskHealthReport, bool) {
	if a == nil {
		return DiskHealthReport{}, false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.diskSMARTCache == nil {
		return DiskHealthReport{}, false
	}
	if r, ok := a.diskSMARTCache["C"]; ok && r.OK {
		return r, true
	}
	for _, r := range a.diskSMARTCache {
		if r.OK {
			return r, true
		}
	}
	return DiskHealthReport{}, false
}

func decodePowerShellUTF8(b []byte) string {
	b = bytes.TrimSpace(b)
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	return strings.TrimSpace(string(b))
}

func applyMediaLabels(report *DiskHealthReport, media MediaClassifyResult, kind string) {
	switch kind {
	case DiskKindNVMe:
		report.MediaType = "SSD"
		report.BusInterface = "NVMe"
		report.InterfaceType = "NVMe"
	case DiskKindSATASSD:
		report.MediaType = "SSD"
	case DiskKindHDD:
		report.MediaType = "HDD"
		if media.RotationRPM > 1 {
			report.RotationRPM = float64(media.RotationRPM)
		}
	default:
		report.MediaType = "тип не определён"
	}
	if media.BusLabel != "" && kind != DiskKindNVMe {
		report.BusInterface = media.BusLabel
		report.InterfaceType = media.BusLabel
	}
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
