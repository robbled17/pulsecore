package main

import (
	"fmt"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/shirou/gopsutil/v3/mem"
)

// Upgrade goals accepted by GetUpgradeAdvice.
const (
	UpgradeGoalMaxFPS   = "max_fps"
	UpgradeGoalQuiet    = "quiet_cool"
	UpgradeGoalWork     = "work"
	UpgradeGoalBudget   = "budget"
)

// PCPassport is a static hardware inventory for Upgrade Advisor.
type PCPassport struct {
	Chassis         string          `json:"chassis"` // desktop | laptop | unknown
	ChassisRaw      string          `json:"chassisRaw"`
	IsLaptop        bool            `json:"isLaptop"`
	Motherboard     string          `json:"motherboard"`
	Manufacturer    string          `json:"manufacturer"`
	Chipset         string          `json:"chipset"`
	Socket          string          `json:"socket"`
	CPUName         string          `json:"cpuName"`
	PhysicalCores   int             `json:"physicalCores"`
	LogicalCores    int             `json:"logicalCores"`
	RAMType         string          `json:"ramType"`
	RAMSpeedMHz     int             `json:"ramSpeedMHz"`
	RAMTotalGB      float64         `json:"ramTotalGB"`
	RAMSlotsUsed    int             `json:"ramSlotsUsed"`
	RAMSlotsTotal   int             `json:"ramSlotsTotal"`
	RAMSlotsFree    int             `json:"ramSlotsFree"`
	IsSingleChannel bool            `json:"isSingleChannel"`
	GPUName         string          `json:"gpuName"`
	VRAMGB          float64         `json:"vramGB"`
	GPUIntegrated   bool            `json:"gpuIntegrated"`
	Storage         []StorageDrive  `json:"storage"`
	HasFreeM2       bool            `json:"hasFreeM2"`
	Notes           []string        `json:"notes"`
}

// StorageDrive is one physical disk for the passport.
type StorageDrive struct {
	Model         string  `json:"model"`
	InterfaceType string  `json:"interfaceType"` // NVMe | SATA | USB | Unknown
	MediaType     string  `json:"mediaType"`     // SSD | HDD | Unknown
	SizeGB        float64 `json:"sizeGB"`
}

// UpgradeAdvice is a goal-driven recommendation pack.
type UpgradeAdvice struct {
	Goal           string               `json:"goal"`
	Chassis        string               `json:"chassis"`
	IsLaptop       bool                 `json:"isLaptop"`
	Passport       PCPassport           `json:"passport"`
	Recommendations []UpgradeRecommendation `json:"recommendations"`
	Summary        string               `json:"summary"`
}

// UpgradeRecommendation is one upgrade path with mandatory pitfalls.
type UpgradeRecommendation struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Part     string   `json:"part"` // cpu | gpu | ram | storage | cooling | other
	Body     string   `json:"body"`
	Priority int      `json:"priority"` // 1 = highest
	Pitfalls []string `json:"pitfalls"`
}

// GetPCPassport builds a chassis-aware hardware passport via WMI/SMBIOS.
func (a *App) GetPCPassport() PCPassport {
	a.mu.RLock()
	static := a.staticHW
	gpuName := a.gpuName
	gpuVRAM := a.gpuTotalVRAM
	a.mu.RUnlock()

	if static.cpuName == "" {
		static = collectStaticHardware()
	}
	if gpuName == "" {
		gpuName = static.gpuName
	}
	if gpuVRAM == 0 {
		gpuVRAM = static.gpuVRAM
	}

	chassis, chassisRaw, isLaptop := detectChassis()
	mobo, mfg := readMotherboard()
	socket := readCPUSocket()
	chipset := readChipsetHint(mobo, mfg)
	storage := readStorageDrives()
	ramType := static.ramType
	slotsUsed := static.ramSlotsUsed
	slotsTotal := static.ramSlotsTotal
	ramSpeed := static.ramSpeedMHz
	if ramType == "" || slotsTotal == 0 {
		ramType, slotsUsed, slotsTotal, ramSpeed = readRAMSlots()
	}
	slotsFree := slotsTotal - slotsUsed
	if slotsFree < 0 {
		slotsFree = 0
	}

	var ramGB float64
	if vm, err := mem.VirtualMemory(); err == nil && vm != nil {
		ramGB = float64(vm.Total) / (1024 * 1024 * 1024)
	}

	vramGB := float64(gpuVRAM) / (1024 * 1024 * 1024)
	dedicatedMB := float64(gpuVRAM) / (1024 * 1024)
	integrated := detectIntegratedGPU(gpuName, dedicatedMB)

	notes := make([]string, 0, 4)
	if isLaptop {
		notes = append(notes, "Ноутбук: процессор и видеокарта обычно впаяны. Менять их нельзя без замены всей платы.")
	}
	if slotsUsed <= 1 && slotsTotal >= 2 {
		notes = append(notes, "Сейчас один модуль RAM. Второй слот даст двухканальный режим и заметный прирост.")
	}
	nvmeCount := 0
	for _, d := range storage {
		if strings.EqualFold(d.InterfaceType, "NVMe") {
			nvmeCount++
		}
	}
	hasFreeM2 := false
	if isLaptop {
		hasFreeM2 = nvmeCount <= 1 // heuristic: many laptops have a second M.2
		if hasFreeM2 {
			notes = append(notes, "Часто есть второй слот M.2. Проверьте крышку снизу или сервисную документацию.")
		}
	} else if nvmeCount <= 1 {
		hasFreeM2 = true
	}

	return PCPassport{
		Chassis:         chassis,
		ChassisRaw:      chassisRaw,
		IsLaptop:        isLaptop,
		Motherboard:     mobo,
		Manufacturer:    mfg,
		Chipset:         chipset,
		Socket:          socket,
		CPUName:         static.cpuName,
		PhysicalCores:   static.physicalCores,
		LogicalCores:    static.logicalCores,
		RAMType:         ramType,
		RAMSpeedMHz:     ramSpeed,
		RAMTotalGB:      ramGB,
		RAMSlotsUsed:    slotsUsed,
		RAMSlotsTotal:   slotsTotal,
		RAMSlotsFree:    slotsFree,
		IsSingleChannel: slotsUsed <= 1,
		GPUName:         gpuName,
		VRAMGB:          vramGB,
		GPUIntegrated:   integrated,
		Storage:         storage,
		HasFreeM2:       hasFreeM2,
		Notes:           notes,
	}
}

// GetUpgradeAdvice returns local recommendations for a goal.
func (a *App) GetUpgradeAdvice(goal string) UpgradeAdvice {
	goal = strings.ToLower(strings.TrimSpace(goal))
	switch goal {
	case UpgradeGoalMaxFPS, "fps", "gaming":
		goal = UpgradeGoalMaxFPS
	case UpgradeGoalQuiet, "quiet", "cool", "silent":
		goal = UpgradeGoalQuiet
	case UpgradeGoalWork, "office", "workstation":
		goal = UpgradeGoalWork
	case UpgradeGoalBudget, "cheap", "save":
		goal = UpgradeGoalBudget
	default:
		goal = UpgradeGoalMaxFPS
	}

	pass := a.GetPCPassport()
	recs := buildUpgradeRecommendations(pass, goal)
	summary := upgradeSummary(pass, goal)
	return UpgradeAdvice{
		Goal:            goal,
		Chassis:         pass.Chassis,
		IsLaptop:        pass.IsLaptop,
		Passport:        pass,
		Recommendations: recs,
		Summary:         summary,
	}
}

func detectChassis() (chassis, raw string, isLaptop bool) {
	ensureOLE()
	defer func() { _ = recover() }()
	chassis = "unknown"
	raw = ""

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

	// Prefer Win32_SystemEnclosure ChassisTypes (SMBIOS).
	resultRaw, err := oleutil.CallMethod(service, "ExecQuery", "SELECT ChassisTypes FROM Win32_SystemEnclosure")
	if err == nil {
		result := resultRaw.ToIDispatch()
		defer result.Release()
		countVar, _ := oleutil.GetProperty(result, "Count")
		count := int(countVar.Val)
		countVar.Clear()
		for i := 0; i < count; i++ {
			itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
			if err != nil {
				continue
			}
			item := itemRaw.ToIDispatch()
			typesVar, err := oleutil.GetProperty(item, "ChassisTypes")
			item.Release()
			if err != nil {
				continue
			}
			codes := parseChassisTypes(typesVar)
			typesVar.Clear()
			if len(codes) == 0 {
				continue
			}
			raw = joinInts(codes)
			chassis, isLaptop = classifyChassis(codes)
			return
		}
	}

	// Fallback: Win32_ComputerSystem PCSystemType (2 = mobile).
	csRaw, err := oleutil.CallMethod(service, "ExecQuery", "SELECT PCSystemType, Model FROM Win32_ComputerSystem")
	if err != nil {
		return
	}
	cs := csRaw.ToIDispatch()
	defer cs.Release()
	cVar, _ := oleutil.GetProperty(cs, "Count")
	if int(cVar.Val) < 1 {
		return
	}
	itemRaw, err := oleutil.CallMethod(cs, "ItemIndex", 0)
	if err != nil {
		return
	}
	item := itemRaw.ToIDispatch()
	defer item.Release()
	typeVar, _ := oleutil.GetProperty(item, "PCSystemType")
	modelVar, _ := oleutil.GetProperty(item, "Model")
	pcType := int(typeVar.Val)
	typeVar.Clear()
	model := strings.TrimSpace(modelVar.ToString())
	modelVar.Clear()
	raw = fmt.Sprintf("PCSystemType=%d", pcType)
	if model != "" {
		raw += " " + model
	}
	if pcType == 2 {
		chassis = "laptop"
		isLaptop = true
	} else if pcType == 1 {
		chassis = "desktop"
		isLaptop = false
	}
	return
}

func parseChassisTypes(v *ole.VARIANT) []int {
	if v == nil {
		return nil
	}
	// ChassisTypes is often a SAFEARRAY of integers.
	arr := v.ToArray()
	if arr == nil {
		if v.Val != 0 {
			return []int{int(v.Val)}
		}
		return nil
	}
	vals := arr.ToValueArray()
	out := make([]int, 0, len(vals))
	for _, x := range vals {
		switch t := x.(type) {
		case int16:
			out = append(out, int(t))
		case int32:
			out = append(out, int(t))
		case int64:
			out = append(out, int(t))
		case int:
			out = append(out, t)
		case uint16:
			out = append(out, int(t))
		case uint32:
			out = append(out, int(t))
		case float64:
			out = append(out, int(t))
		}
	}
	return out
}

func classifyChassis(codes []int) (string, bool) {
	laptopCodes := map[int]bool{
		8: true, 9: true, 10: true, 11: true, 12: true, 14: true,
		30: true, 31: true, 32: true,
	}
	desktopCodes := map[int]bool{
		3: true, 4: true, 5: true, 6: true, 7: true, 13: true, 15: true,
		16: true, 34: true, 35: true,
	}
	for _, c := range codes {
		if laptopCodes[c] {
			return "laptop", true
		}
	}
	for _, c := range codes {
		if desktopCodes[c] {
			return "desktop", false
		}
	}
	return "unknown", false
}

func joinInts(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = fmt.Sprintf("%d", x)
	}
	return strings.Join(parts, ",")
}

func readMotherboard() (product, manufacturer string) {
	ensureOLE()
	defer func() { _ = recover() }()
	product = "Unknown board"
	manufacturer = ""
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
		"SELECT Product, Manufacturer FROM Win32_BaseBoard")
	if err != nil {
		return
	}
	result := resultRaw.ToIDispatch()
	defer result.Release()
	countVar, _ := oleutil.GetProperty(result, "Count")
	if int(countVar.Val) < 1 {
		return
	}
	itemRaw, err := oleutil.CallMethod(result, "ItemIndex", 0)
	if err != nil {
		return
	}
	item := itemRaw.ToIDispatch()
	defer item.Release()
	pVar, _ := oleutil.GetProperty(item, "Product")
	mVar, _ := oleutil.GetProperty(item, "Manufacturer")
	product = strings.TrimSpace(pVar.ToString())
	manufacturer = strings.TrimSpace(mVar.ToString())
	pVar.Clear()
	mVar.Clear()
	if product == "" {
		product = "Unknown board"
	}
	return
}

func readCPUSocket() string {
	ensureOLE()
	defer func() { _ = recover() }()
	locator, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return guessSocketFromCPU(readCPUName())
	}
	defer locator.Release()
	dl, err := locator.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return guessSocketFromCPU(readCPUName())
	}
	defer dl.Release()
	serviceRaw, err := oleutil.CallMethod(dl, "ConnectServer", nil, `root\cimv2`)
	if err != nil {
		return guessSocketFromCPU(readCPUName())
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()
	resultRaw, err := oleutil.CallMethod(service, "ExecQuery",
		"SELECT SocketDesignation, Name FROM Win32_Processor")
	if err != nil {
		return guessSocketFromCPU(readCPUName())
	}
	result := resultRaw.ToIDispatch()
	defer result.Release()
	countVar, _ := oleutil.GetProperty(result, "Count")
	if int(countVar.Val) < 1 {
		return guessSocketFromCPU(readCPUName())
	}
	itemRaw, err := oleutil.CallMethod(result, "ItemIndex", 0)
	if err != nil {
		return guessSocketFromCPU(readCPUName())
	}
	item := itemRaw.ToIDispatch()
	defer item.Release()
	sockVar, _ := oleutil.GetProperty(item, "SocketDesignation")
	nameVar, _ := oleutil.GetProperty(item, "Name")
	sock := strings.TrimSpace(sockVar.ToString())
	name := strings.TrimSpace(nameVar.ToString())
	sockVar.Clear()
	nameVar.Clear()
	guessed := guessSocketFromCPU(name)
	if sock == "" || strings.EqualFold(sock, "CPU 0") || strings.EqualFold(sock, "SOCKET 0") {
		return guessed
	}
	// Prefer a normalized guess when designation is opaque (e.g. "U3E1").
	if guessed != "Unknown" && !looksLikeRealSocket(sock) {
		return guessed
	}
	return sock
}

func looksLikeRealSocket(s string) bool {
	u := strings.ToUpper(s)
	return strings.Contains(u, "AM4") || strings.Contains(u, "AM5") ||
		strings.Contains(u, "LGA") || strings.Contains(u, "TR4") ||
		strings.Contains(u, "sTRX") || strings.Contains(u, "FP")
}

func guessSocketFromCPU(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "ryzen") && (strings.Contains(n, "9000") || strings.Contains(n, "8000") || strings.Contains(n, "7000")):
		return "AM5"
	case strings.Contains(n, "ryzen") && (strings.Contains(n, "5000") || strings.Contains(n, "3000") || strings.Contains(n, "4000") || strings.Contains(n, "2000") || strings.Contains(n, "1000")):
		return "AM4"
	case strings.Contains(n, "core ultra") || strings.Contains(n, "core™ ultra"):
		return "LGA1851"
	case strings.Contains(n, "14th") || strings.Contains(n, "i9-14") || strings.Contains(n, "i7-14") || strings.Contains(n, "i5-14") ||
		strings.Contains(n, "13th") || strings.Contains(n, "i9-13") || strings.Contains(n, "i7-13") || strings.Contains(n, "i5-13") ||
		strings.Contains(n, "12th") || strings.Contains(n, "i9-12") || strings.Contains(n, "i7-12") || strings.Contains(n, "i5-12"):
		return "LGA1700"
	case strings.Contains(n, "11th") || strings.Contains(n, "10th") || strings.Contains(n, "i9-10") || strings.Contains(n, "i7-10") || strings.Contains(n, "i5-10") ||
		strings.Contains(n, "i9-11") || strings.Contains(n, "i7-11") || strings.Contains(n, "i5-11"):
		return "LGA1200"
	case strings.Contains(n, "i9-9") || strings.Contains(n, "i7-9") || strings.Contains(n, "i5-9") || strings.Contains(n, "i9-8") || strings.Contains(n, "i7-8") || strings.Contains(n, "i5-8"):
		return "LGA1151"
	default:
		return "Unknown"
	}
}

func readChipsetHint(mobo, mfg string) string {
	blob := strings.ToUpper(mobo + " " + mfg)
	for _, chip := range []string{
		"X870", "B850", "X670", "B650", "A620", "X570", "B550", "A520", "B450", "X470",
		"Z890", "B860", "Z790", "B760", "H770", "Z690", "B660", "H670", "Z590", "B560", "H510",
		"Z490", "B460", "H410",
	} {
		if strings.Contains(blob, chip) {
			return chip
		}
	}
	// Try Win32_IDEController / PnP for AMD/Intel chipset strings.
	ensureOLE()
	defer func() { _ = recover() }()
	locator, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return "Unknown"
	}
	defer locator.Release()
	dl, err := locator.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return "Unknown"
	}
	defer dl.Release()
	serviceRaw, err := oleutil.CallMethod(dl, "ConnectServer", nil, `root\cimv2`)
	if err != nil {
		return "Unknown"
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()
	resultRaw, err := oleutil.CallMethod(service, "ExecQuery",
		"SELECT Name FROM Win32_PnPEntity WHERE Name LIKE '%Chipset%' OR Name LIKE '%Bridge%'")
	if err != nil {
		return "Unknown"
	}
	result := resultRaw.ToIDispatch()
	defer result.Release()
	countVar, _ := oleutil.GetProperty(result, "Count")
	count := int(countVar.Val)
	limit := count
	if limit > 40 {
		limit = 40
	}
	for i := 0; i < limit; i++ {
		itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
		if err != nil {
			continue
		}
		item := itemRaw.ToIDispatch()
		nameVar, _ := oleutil.GetProperty(item, "Name")
		name := strings.TrimSpace(nameVar.ToString())
		nameVar.Clear()
		item.Release()
		up := strings.ToUpper(name)
		for _, chip := range []string{
			"X870", "B850", "X670", "B650", "A620", "X570", "B550", "A520",
			"Z890", "B860", "Z790", "B760", "Z690", "B660", "Z590", "B560",
		} {
			if strings.Contains(up, chip) {
				return chip
			}
		}
		if strings.Contains(up, "CHIPSET") && len(name) < 80 {
			return name
		}
	}
	return "Unknown"
}

func readStorageDrives() []StorageDrive {
	ensureOLE()
	out := make([]StorageDrive, 0, 4)
	defer func() { _ = recover() }()
	locator, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return out
	}
	defer locator.Release()
	dl, err := locator.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return out
	}
	defer dl.Release()
	serviceRaw, err := oleutil.CallMethod(dl, "ConnectServer", nil, `root\cimv2`)
	if err != nil {
		return out
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()
	resultRaw, err := oleutil.CallMethod(service, "ExecQuery",
		"SELECT Model, InterfaceType, MediaType, Size FROM Win32_DiskDrive")
	if err != nil {
		return out
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
		modelVar, _ := oleutil.GetProperty(item, "Model")
		ifaceVar, _ := oleutil.GetProperty(item, "InterfaceType")
		mediaVar, _ := oleutil.GetProperty(item, "MediaType")
		sizeVar, _ := oleutil.GetProperty(item, "Size")
		model := strings.TrimSpace(modelVar.ToString())
		iface := strings.TrimSpace(ifaceVar.ToString())
		media := strings.TrimSpace(mediaVar.ToString())
		size := float64(uint64(sizeVar.Val)) / (1024 * 1024 * 1024)
		modelVar.Clear()
		ifaceVar.Clear()
		mediaVar.Clear()
		sizeVar.Clear()
		item.Release()
		if model == "" {
			continue
		}
		out = append(out, StorageDrive{
			Model:         model,
			InterfaceType: normalizeStorageIface(iface, model),
			MediaType:     normalizeDiskMediaType(media, model),
			SizeGB:        size,
		})
	}
	return out
}

func normalizeStorageIface(iface, model string) string {
	u := strings.ToUpper(iface + " " + model)
	switch {
	case strings.Contains(u, "NVME") || strings.Contains(u, "NVM EXPRESS"):
		return "NVMe"
	case strings.Contains(u, "SCSI") && (strings.Contains(u, "SSD") || strings.Contains(strings.ToUpper(model), "NVME")):
		// Many NVMe drives report SCSI in Win32_DiskDrive.
		if strings.Contains(strings.ToUpper(model), "NVME") {
			return "NVMe"
		}
		return "SATA"
	case strings.Contains(u, "IDE") || strings.Contains(u, "SATA") || strings.Contains(u, "ATA"):
		return "SATA"
	case strings.Contains(u, "USB"):
		return "USB"
	default:
		if strings.Contains(strings.ToUpper(model), "NVME") {
			return "NVMe"
		}
		return "Unknown"
	}
}

func normalizeDiskMediaType(media, model string) string {
	u := strings.ToUpper(media + " " + model)
	switch {
	case strings.Contains(u, "SSD") || strings.Contains(u, "SOLID") || strings.Contains(u, "NVME"):
		return "SSD"
	case strings.Contains(u, "HDD") || strings.Contains(u, "FIXED HARD"):
		return "HDD"
	default:
		return "Unknown"
	}
}

func upgradeSummary(pass PCPassport, goal string) string {
	kind := "ПК"
	if pass.IsLaptop {
		kind = "ноутбук"
	}
	switch goal {
	case UpgradeGoalMaxFPS:
		return fmt.Sprintf("Цель: максимум FPS. Тип устройства: %s. Смотрите видеокарту, процессор под текущий сокет и память в двухканале.", kind)
	case UpgradeGoalQuiet:
		return fmt.Sprintf("Цель: тише и холоднее. Тип устройства: %s. Упор на охлаждение, undervolt и отказ от шумных топовых решений.", kind)
	case UpgradeGoalWork:
		return fmt.Sprintf("Цель: работа и учёба. Тип устройства: %s. Важны ядра, объём RAM и быстрый NVMe.", kind)
	default:
		return fmt.Sprintf("Цель: апгрейд с умом за небольшие деньги. Тип устройства: %s. Сначала слоты RAM и накопитель.", kind)
	}
}

func buildUpgradeRecommendations(pass PCPassport, goal string) []UpgradeRecommendation {
	if pass.IsLaptop {
		return laptopRecommendations(pass, goal)
	}
	return desktopRecommendations(pass, goal)
}

func commonPitfalls(pass PCPassport, part string) []string {
	base := []string{}
	switch part {
	case "cpu":
		base = []string{
			"Обновите BIOS материнской платы до установки нового процессора. Старая прошивка может не узнать чип.",
			"Проверьте питание VRM на плате. Слабая система питания греется на мощных CPU и снижает частоту.",
			fmt.Sprintf("Сокет сейчас: %s. Новый процессор должен быть именно для этого сокета, иначе не встанет.", pass.Socket),
		}
	case "gpu":
		base = []string{
			"Измерьте длину корпуса и карты. Длинная видеокарта может не влезть или упереться в диски.",
			"Блок питания: запас по ваттам и нужные разъёмы (8-pin / 12VHPWR). Старый кабель переходником не спасает слабый БП.",
			"После установки поставьте свежий драйвер с сайта NVIDIA или AMD и удалите старый через DDU при смене бренда.",
		}
	case "ram":
		base = []string{
			fmt.Sprintf("Тип памяти: %s. Планки другого поколения (DDR4/DDR5) физически не подойдут.", pass.RAMType),
			"Берите комплект из двух одинаковых модулей. Разный объём или частота часто даёт сбои.",
			"В BIOS включите XMP/EXPO, иначе планки могут работать на меньшей частоте.",
		}
	case "storage":
		base = []string{
			"Для NVMe нужен свободный слот M.2 и иногда отдельный радиатор. Без него диск может троттлить.",
			"Клонируйте систему или поставьте Windows заново. Просто вставить диск недостаточно для переноса ОС.",
			"SATA SSD дешевле, но медленнее NVMe. Для игр разница меньше, чем для больших проектов.",
		}
	case "cooling":
		base = []string{
			"Смените термопасту и проверьте посадку кулера. Воздух должен выходить из корпуса, а не крутиться внутри.",
			"Undervolt в BIOS или через фирменную утилиту снижает нагрев. Делайте маленькими шагами и тестируйте стабильность.",
			"На ноутбуке следите, чтобы вентиляционные отверстия не были закрыты на мягкой поверхности.",
		}
	default:
		base = []string{
			"Перед апгрейдом сделайте резервную копию важных файлов.",
			"Отключайте питание от розетки и снимайте статику перед работой внутри корпуса.",
		}
	}
	return base
}

func laptopRecommendations(pass PCPassport, goal string) []UpgradeRecommendation {
	recs := make([]UpgradeRecommendation, 0, 6)

	recs = append(recs, UpgradeRecommendation{
		ID:       "laptop-soldered",
		Title:    "Честно про впаянное железо",
		Part:     "other",
		Priority: 1,
		Body:     "В большинстве ноутбуков CPU и GPU впаяны в плату. Их нельзя заменить отдельно. Реальный апгрейд: RAM (если есть слот), SSD и обслуживание охлаждения.",
		Pitfalls: []string{
			"Не покупайте «мобильный» процессор или видеокарту отдельно. Они не встанут в ваш ноутбук.",
			"Сервисная замена всей материнской платы почти всегда дороже нового устройства.",
			"Проверьте сервисное руководство по модели: какие слоты реально доступны пользователю.",
		},
	})

	if pass.RAMSlotsFree > 0 || pass.IsSingleChannel {
		body := "Добавьте второй модуль той же частоты и типа, чтобы включить dual-channel. Это часто даёт больше FPS в играх на встроенной графике и отзывчивость в браузере."
		if goal == UpgradeGoalBudget {
			body = "Самый выгодный шаг: вторая планка RAM под dual-channel. Дешевле новой видеокарты и сразу заметно."
		}
		recs = append(recs, UpgradeRecommendation{
			ID:       "laptop-ram-dual",
			Title:    "RAM: dual-channel",
			Part:     "ram",
			Priority: 2,
			Body:     body,
			Pitfalls: commonPitfalls(pass, "ram"),
		})
	}

	if pass.HasFreeM2 || len(pass.Storage) > 0 {
		body := "Поставьте или добавьте NVMe SSD во второй M.2, если слот свободен. Игры и Windows будут открываться быстрее."
		if goal == UpgradeGoalWork {
			body = "Рабочим задачам нужен быстрый NVMe: проекты, Docker, IDE и браузер с кучей вкладок любят быстрый диск."
		}
		recs = append(recs, UpgradeRecommendation{
			ID:       "laptop-m2",
			Title:    "Второй M.2 / замена SSD",
			Part:     "storage",
			Priority: 3,
			Body:     body,
			Pitfalls: commonPitfalls(pass, "storage"),
		})
	}

	coolBody := "Почистите пыль, смените термопасту и при необходимости сделайте undervolt. Тише и холоднее без покупки нового железа."
	if goal == UpgradeGoalQuiet {
		coolBody = "Главный путь к тишине на ноутбуке: чистка, новая паста, undervolt и тихий профиль питания. Новый GPU сюда не поставить."
	}
	if goal == UpgradeGoalMaxFPS {
		coolBody = "Охлаждение и undervolt держат частоты дольше. На ноутбуке это часто даёт больше стабильного FPS, чем попытка «разогнать всё»."
	}
	recs = append(recs, UpgradeRecommendation{
		ID:       "laptop-cooling",
		Title:    "Охлаждение и undervolt",
		Part:     "cooling",
		Priority: 4,
		Body:     coolBody,
		Pitfalls: commonPitfalls(pass, "cooling"),
	})

	return recs
}

func desktopRecommendations(pass PCPassport, goal string) []UpgradeRecommendation {
	recs := make([]UpgradeRecommendation, 0, 8)
	socket := pass.Socket
	if socket == "" || socket == "Unknown" {
		socket = guessSocketFromCPU(pass.CPUName)
	}

	cpuBody := fmt.Sprintf("Для сокета %s берите актуальный процессор того же поколения платы. Не меняйте платформу целиком, если сокет ещё живой.", socket)
	switch goal {
	case UpgradeGoalMaxFPS:
		cpuBody = fmt.Sprintf("Максимум FPS: возьмите лучший игровой CPU на сокете %s (высокое однопоточное ускорение). Видеокарта всё равно важнее, но слабый процессор её ограничивает.", socket)
	case UpgradeGoalWork:
		cpuBody = fmt.Sprintf("Для работы берите больше ядер на сокете %s. Рендер, компиляция и виртуалки любят много потоков.", socket)
	case UpgradeGoalQuiet:
		cpuBody = fmt.Sprintf("Тихий ПК: процессор среднего TDP на сокете %s плюс хороший башенный кулер. Топовые чипы с высоким потреблением будут шуметь.", socket)
	case UpgradeGoalBudget:
		cpuBody = fmt.Sprintf("Бюджет: б/у или mid-range CPU строго под сокет %s. Смена материнской платы съест весь бюджет.", socket)
	}
	recs = append(recs, UpgradeRecommendation{
		ID:       "desktop-cpu",
		Title:    "Процессор под текущий сокет",
		Part:     "cpu",
		Priority: 2,
		Body:     cpuBody,
		Pitfalls: commonPitfalls(pass, "cpu"),
	})

	gpuBody := "Сбалансированная видеокарта под разрешение монитора. Для 1080p хватит mid-range, для 1440p нужен запас по VRAM."
	switch goal {
	case UpgradeGoalMaxFPS:
		gpuBody = "Главный прирост FPS даёт видеокарта. Берите модель с запасом VRAM под ваше разрешение и трассировку лучей, если она нужна."
	case UpgradeGoalQuiet:
		gpuBody = "Берите карту с хорошим трёхвентиляторным охлаждением и умеренным TDP. Короткий двухслотовый «дутыш» часто орёт на 100%."
	case UpgradeGoalWork:
		gpuBody = "Для монтажа и 3D смотрите объём VRAM. Для офиса и браузера достаточно текущей карты, если это не встроенная графика."
	case UpgradeGoalBudget:
		gpuBody = "Бюджетная дискретная карта заметно обгоняет встроенную графику. Не гонитесь за топовым номером, берите свежий mid-range."
	}
	if pass.GPUIntegrated {
		gpuBody = "Сейчас встроенная графика. Дискретная карта даст главный скачок в играх. Подберите модель под блок питания и длину корпуса."
	}
	recs = append(recs, UpgradeRecommendation{
		ID:       "desktop-gpu",
		Title:    "Сбалансированная видеокарта",
		Part:     "gpu",
		Priority: 1,
		Body:     gpuBody,
		Pitfalls: commonPitfalls(pass, "gpu"),
	})

	if pass.RAMSlotsFree > 0 || pass.IsSingleChannel || pass.RAMTotalGB < 16 {
		ramBody := fmt.Sprintf("Свободно слотов: %d. Добавьте планки %s до 32 ГБ в двухканале для игр и браузера.", pass.RAMSlotsFree, pass.RAMType)
		if goal == UpgradeGoalWork {
			ramBody = fmt.Sprintf("Для работы целитесь в 32 или 64 ГБ %s. Все модули одной частоты.", pass.RAMType)
		}
		if goal == UpgradeGoalBudget && pass.IsSingleChannel {
			ramBody = "Самый дешёвый буст: вторая планка той же памяти для dual-channel."
		}
		recs = append(recs, UpgradeRecommendation{
			ID:       "desktop-ram",
			Title:    "Оперативная память",
			Part:     "ram",
			Priority: 3,
			Body:     ramBody,
			Pitfalls: commonPitfalls(pass, "ram"),
		})
	}

	if pass.HasFreeM2 {
		recs = append(recs, UpgradeRecommendation{
			ID:       "desktop-nvme",
			Title:    "NVMe SSD",
			Part:     "storage",
			Priority: 4,
			Body:     "Поставьте игры и систему на NVMe. Если слот M.2 свободен, это быстрый апгрейд без замены всего ПК.",
			Pitfalls: commonPitfalls(pass, "storage"),
		})
	}

	if goal == UpgradeGoalQuiet {
		recs = append(recs, UpgradeRecommendation{
			ID:       "desktop-cooling",
			Title:    "Тихое охлаждение",
			Part:     "cooling",
			Priority: 2,
			Body:     "Башенный кулер, нормальный поток воздуха в корпусе и undervolt GPU/CPU. Часто тише, чем замена железа.",
			Pitfalls: commonPitfalls(pass, "cooling"),
		})
	}

	return recs
}
