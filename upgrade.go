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
	Approximate     bool            `json:"approximate"`
	GPUClass        string          `json:"gpuClass,omitempty"`
	CPUClass        string          `json:"cpuClass,omitempty"`
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
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Part          string   `json:"part"` // cpu | gpu | ram | storage | cooling | other
	Model         string   `json:"model"`
	EstimatedGain string   `json:"estimatedGain"`
	Body          string   `json:"body"`
	Priority      int      `json:"priority"` // 1 = highest
	Pitfalls      []string `json:"pitfalls"`
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
	// Single source of truth with Hardware tab: prefer cached static slots.
	// Re-read only when static inventory is empty.
	if ramType == "" || slotsTotal == 0 {
		rt, used, total, speed := readRAMSlots()
		if ramType == "" {
			ramType = rt
		}
		if slotsTotal == 0 {
			slotsUsed, slotsTotal = used, total
		}
		if ramSpeed == 0 {
			ramSpeed = speed
		}
	}
	slotsUsed, slotsTotal, slotsFree := normalizeRAMSlots(slotsUsed, slotsTotal)

	hasBattery := batteryPresent()
	if refine := refineLaptopDetection(isLaptop, chassis, static.cpuName, gpuName, hasBattery); refine.isLaptop {
		isLaptop = true
		chassis = refine.chassis
		if chassisRaw == "" {
			chassisRaw = refine.raw
		}
	}

	var ramGB float64
	if vm, err := mem.VirtualMemory(); err == nil && vm != nil {
		ramGB = float64(vm.Total) / (1024 * 1024 * 1024)
	}

	vramGB := float64(gpuVRAM) / (1024 * 1024 * 1024)
	dedicatedMB := float64(gpuVRAM) / (1024 * 1024)
	integrated := detectIntegratedGPU(gpuName, dedicatedMB)
	if integrated {
		vramGB = 0
	}

	gpuScore := scoreGPU(gpuName, integrated, vramGB)
	cpuScore := scoreCPU(static.cpuName, static.physicalCores, static.logicalCores, static.frequencyMHz)
	approximate := gpuScore.Approximate || cpuScore.Approximate

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
		Approximate:     approximate,
		GPUClass:        gpuScore.Class,
		CPUClass:        cpuScore.Class,
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
	lang := a.GetLanguage()
	if pass.Approximate {
		note := trUpgrade(lang, "noteApprox")
		dup := false
		for _, n := range pass.Notes {
			if n == note {
				dup = true
				break
			}
		}
		if !dup {
			pass.Notes = append(pass.Notes, note)
		}
	}
	recs := buildUpgradeRecommendations(pass, goal, lang)
	summary := upgradeSummary(pass, goal, lang)
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

// normalizeRAMSlots clamps used/total and returns free = max(0, total-used).
// Shared by Hardware and Upgrade so both UIs agree on free slot count.
func normalizeRAMSlots(used, total int) (usedOut, totalOut, free int) {
	if used < 0 {
		used = 0
	}
	if total < 0 {
		total = 0
	}
	if total > 0 && used > total {
		used = total
	}
	free = total - used
	if free < 0 {
		free = 0
	}
	return used, total, free
}

type laptopRefine struct {
	isLaptop bool
	chassis  string
	raw      string
}

// refineLaptopDetection upgrades a false-negative desktop/unknown chassis when
// battery, mobile CPU suffixes (H/HX/U/HS), or "Laptop" GPU markers are present.
func refineLaptopDetection(already bool, chassis, cpuName, gpuName string, hasBattery bool) laptopRefine {
	if already || chassis == "laptop" {
		return laptopRefine{isLaptop: true, chassis: "laptop"}
	}
	if looksLikeLaptopHardware(cpuName, gpuName, hasBattery) {
		reason := "heuristic"
		if hasBattery {
			reason = "battery"
		} else if mobileCPUName(cpuName) {
			reason = "cpu-mobile"
		} else if mobileGPUName(gpuName) {
			reason = "gpu-laptop"
		}
		return laptopRefine{isLaptop: true, chassis: "laptop", raw: reason}
	}
	return laptopRefine{isLaptop: false, chassis: chassis}
}

func looksLikeLaptopHardware(cpuName, gpuName string, hasBattery bool) bool {
	if hasBattery {
		return true
	}
	return mobileCPUName(cpuName) || mobileGPUName(gpuName)
}

func mobileGPUName(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "laptop") ||
		strings.Contains(n, "mobile") ||
		strings.Contains(n, "max-q") ||
		strings.Contains(n, "max-p") ||
		strings.Contains(n, "maxq")
}

// mobileCPUName detects common notebook CPU product markers (H/HX/U/HS and "Laptop").
func mobileCPUName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return false
	}
	if strings.Contains(n, "laptop") || strings.Contains(n, "mobile") {
		return true
	}
	// Strip trademark noise, keep alnum tokens.
	compact := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return ' '
	}, n)
	for _, tok := range strings.Fields(compact) {
		// Intel/AMD mobile SKUs: 5800h, 12700hx, 1135g7, 7840hs, 5600u …
		if len(tok) < 4 {
			continue
		}
		hasDigit := false
		for _, r := range tok {
			if r >= '0' && r <= '9' {
				hasDigit = true
				break
			}
		}
		if !hasDigit {
			continue
		}
		if strings.HasSuffix(tok, "hx") || strings.HasSuffix(tok, "hs") ||
			strings.HasSuffix(tok, "h") || strings.HasSuffix(tok, "u") {
			return true
		}
		// Intel G-series mobile (e.g. 1135g7): digit+g+digit
		if strings.Contains(tok, "g") {
			i := strings.IndexByte(tok, 'g')
			if i > 0 && i+1 < len(tok) && tok[i-1] >= '0' && tok[i-1] <= '9' && tok[i+1] >= '0' && tok[i+1] <= '9' {
				return true
			}
		}
	}
	return false
}

func batteryPresent() bool {
	st, ok := readSystemPowerStatus()
	if !ok {
		return false
	}
	// 255 = unknown / no battery on most Windows builds.
	return st.BatteryLifePercent != 255
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

var knownChipsets = []string{
	"X870E", "X870", "B850", "X670E", "X670", "B650E", "B650", "A620",
	"X570", "B550", "A520", "B450", "X470", "B350", "X370",
	"Z890", "B860", "Z790", "B760", "H770", "Z690", "B660", "H670", "H610",
	"Z590", "B560", "H570", "H510", "Z490", "B460", "H410",
	"Z390", "B365", "B360", "H370", "Z370",
}

func matchChipsetToken(blob string) string {
	up := strings.ToUpper(blob)
	for _, chip := range knownChipsets {
		if strings.Contains(up, chip) {
			return chip
		}
	}
	return ""
}

func readChipsetHint(mobo, mfg string) string {
	if chip := matchChipsetToken(mobo + " " + mfg); chip != "" {
		return chip
	}
	// Win32_BaseBoard already fed via mobo/mfg; also scan PnP / PCI bridge names
	// (e.g. "AMD B550 Chipset", "Intel(R) Z690 Chipset").
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

	queries := []string{
		"SELECT Name, Caption FROM Win32_PnPEntity WHERE Name LIKE '%Chipset%' OR Caption LIKE '%Chipset%' OR Name LIKE '%FCH%' OR Name LIKE '%LPC%' OR Name LIKE '%PCI Express Root%' OR Name LIKE '%Host Bridge%'",
		"SELECT Name, Caption FROM Win32_IDEController",
		"SELECT Name, Caption FROM Win32_PnPEntity WHERE PNPClass='System'",
	}
	for _, q := range queries {
		resultRaw, err := oleutil.CallMethod(service, "ExecQuery", q)
		if err != nil {
			continue
		}
		result := resultRaw.ToIDispatch()
		countVar, _ := oleutil.GetProperty(result, "Count")
		count := int(countVar.Val)
		limit := count
		if limit > 80 {
			limit = 80
		}
		for i := 0; i < limit; i++ {
			itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
			if err != nil {
				continue
			}
			item := itemRaw.ToIDispatch()
			nameVar, _ := oleutil.GetProperty(item, "Name")
			capVar, _ := oleutil.GetProperty(item, "Caption")
			name := strings.TrimSpace(nameVar.ToString())
			caption := strings.TrimSpace(capVar.ToString())
			nameVar.Clear()
			capVar.Clear()
			item.Release()
			blob := name + " " + caption
			if chip := matchChipsetToken(blob); chip != "" {
				result.Release()
				return chip
			}
			up := strings.ToUpper(blob)
			if strings.Contains(up, "CHIPSET") && len(name) > 0 && len(name) < 80 {
				result.Release()
				return name
			}
		}
		result.Release()
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

// Recommendation builders live in upgrade_advice.go.
