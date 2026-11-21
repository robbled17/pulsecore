package main

import (
	"fmt"
	"sort"
	"strings"
)

func upgradeSummary(pass PCPassport, goal, lang string) string {
	kind := trUpgrade(lang, "kindDesktop")
	if pass.IsLaptop {
		kind = trUpgrade(lang, "kindLaptop")
	}
	switch goal {
	case UpgradeGoalMaxFPS:
		return fmt.Sprintf(trUpgrade(lang, "sumFPS"), kind)
	case UpgradeGoalQuiet:
		return fmt.Sprintf(trUpgrade(lang, "sumQuiet"), kind)
	case UpgradeGoalWork:
		return fmt.Sprintf(trUpgrade(lang, "sumWork"), kind)
	default:
		return fmt.Sprintf(trUpgrade(lang, "sumBudget"), kind)
	}
}

func buildUpgradeRecommendations(pass PCPassport, goal, lang string) []UpgradeRecommendation {
	// Hard-split: force laptop path when markers say so even if IsLaptop was missed.
	if !pass.IsLaptop && looksLikeLaptopHardware(pass.CPUName, pass.GPUName, false) {
		pass.IsLaptop = true
		pass.Chassis = "laptop"
	}
	var recs []UpgradeRecommendation
	if pass.IsLaptop {
		recs = laptopRecommendations(pass, goal, lang)
	} else {
		recs = desktopRecommendations(pass, goal, lang)
	}
	if pass.IsLaptop {
		recs = filterLaptopSafeRecs(recs)
	}
	sort.SliceStable(recs, func(i, j int) bool {
		return recs[i].Priority < recs[j].Priority
	})
	if len(recs) > 3 {
		recs = recs[:3]
	}
	// Always show 2-3 concrete cards when possible.
	if len(recs) < 2 && !pass.IsLaptop {
		recs = append(recs, desktopFallback(pass, goal, lang)...)
		sort.SliceStable(recs, func(i, j int) bool {
			return recs[i].Priority < recs[j].Priority
		})
		if len(recs) > 3 {
			recs = recs[:3]
		}
	}
	return recs
}

// filterLaptopSafeRecs bans CPU/GPU replacement cards and RAM when no free slots.
func filterLaptopSafeRecs(recs []UpgradeRecommendation) []UpgradeRecommendation {
	out := make([]UpgradeRecommendation, 0, len(recs))
	for _, r := range recs {
		part := strings.ToLower(strings.TrimSpace(r.Part))
		if part == "cpu" || part == "gpu" {
			continue
		}
		out = append(out, r)
	}
	return out
}

func socketOf(pass PCPassport) string {
	socket := pass.Socket
	if socket == "" || socket == "Unknown" {
		socket = guessSocketFromCPU(pass.CPUName)
	}
	if socket == "" {
		socket = "Unknown"
	}
	return socket
}

func commonPitfalls(pass PCPassport, part, lang string) []string {
	socket := socketOf(pass)
	ramType := pass.RAMType
	if ramType == "" {
		ramType = "DDR4"
	}
	switch part {
	case "cpu":
		return []string{
			trUpgrade(lang, "pitCPU1"),
			trUpgrade(lang, "pitCPU2"),
			fmt.Sprintf(trUpgrade(lang, "pitCPU3"), socket),
		}
	case "gpu":
		return []string{
			trUpgrade(lang, "pitGPU1"),
			trUpgrade(lang, "pitGPU2"),
			trUpgrade(lang, "pitGPU3"),
		}
	case "ram":
		return []string{
			fmt.Sprintf(trUpgrade(lang, "pitRAM1"), ramType),
			trUpgrade(lang, "pitRAM2"),
			trUpgrade(lang, "pitRAM3"),
		}
	case "storage":
		return []string{
			trUpgrade(lang, "pitSSD1"),
			trUpgrade(lang, "pitSSD2"),
			trUpgrade(lang, "pitSSD3"),
		}
	case "cooling":
		return []string{
			trUpgrade(lang, "pitCool1"),
			trUpgrade(lang, "pitCool2"),
			trUpgrade(lang, "pitCool3"),
		}
	default:
		return []string{
			trUpgrade(lang, "pitOther1"),
			trUpgrade(lang, "pitOther2"),
		}
	}
}

func laptopRecommendations(pass PCPassport, goal, lang string) []UpgradeRecommendation {
	recs := make([]UpgradeRecommendation, 0, 4)
	ramType := pass.RAMType
	if ramType == "" {
		ramType = "DDR4"
	}

	// Realistic laptop upgrades only: RAM (free slots only), 2nd NVMe, cooling.
	// Never soldered CPU/GPU. Never claim free slots when Hardware shows full.
	_, _, freeSlots := normalizeRAMSlots(pass.RAMSlotsUsed, pass.RAMSlotsTotal)
	if pass.RAMSlotsFree >= 0 && pass.RAMSlotsTotal > 0 {
		freeSlots = pass.RAMSlotsFree
		if freeSlots < 0 {
			freeSlots = 0
		}
		if pass.RAMSlotsUsed+freeSlots != pass.RAMSlotsTotal && pass.RAMSlotsTotal > 0 {
			_, _, freeSlots = normalizeRAMSlots(pass.RAMSlotsUsed, pass.RAMSlotsTotal)
		}
	}
	pass.RAMSlotsFree = freeSlots
	if freeSlots > 0 {
		gain := "+15-25% FPS (iGPU)"
		if goal == UpgradeGoalWork {
			gain = trUpgrade(lang, "gainSnappier")
		}
		if goal == UpgradeGoalBudget {
			gain = trUpgrade(lang, "gainBestValue")
		}
		model := fmt.Sprintf("1x 16 GB %s (match)", ramType)
		if pass.RAMTotalGB > 0 && pass.RAMTotalGB < 12 {
			model = fmt.Sprintf("1x 8 GB %s (match)", ramType)
		}
		recs = append(recs, UpgradeRecommendation{
			ID:            "laptop-ram-dual",
			Title:         trUpgrade(lang, "titleRAMDual"),
			Part:          "ram",
			Model:         model,
			EstimatedGain: gain,
			Priority:      1,
			Body:          fmt.Sprintf(trUpgrade(lang, "bodyLaptopRAM"), ramType),
			Pitfalls:      commonPitfalls(pass, "ram", lang),
		})
	}

	{
		gain := trUpgrade(lang, "gainLoadTimes")
		if goal == UpgradeGoalWork {
			gain = trUpgrade(lang, "gainProjects")
		}
		recs = append(recs, UpgradeRecommendation{
			ID:            "laptop-m2",
			Title:         trUpgrade(lang, "titleLaptopSSD"),
			Part:          "storage",
			Model:         "NVMe 1 TB (2nd M.2)",
			EstimatedGain: gain,
			Priority:      2,
			Body:          trUpgrade(lang, "bodyLaptopSSD"),
			Pitfalls:      commonPitfalls(pass, "storage", lang),
		})
	}

	coolGain := trUpgrade(lang, "gainQuieter")
	coolBody := trUpgrade(lang, "bodyLaptopCool")
	if goal == UpgradeGoalMaxFPS {
		coolGain = trUpgrade(lang, "gainStableFPS")
		coolBody = trUpgrade(lang, "bodyLaptopCoolFPS")
	}
	if goal == UpgradeGoalQuiet {
		coolGain = trUpgrade(lang, "gainMuchQuieter")
		coolBody = trUpgrade(lang, "bodyLaptopCoolQuiet")
	}
	recs = append(recs, UpgradeRecommendation{
		ID:            "laptop-cooling",
		Title:         trUpgrade(lang, "titleLaptopCool"),
		Part:          "cooling",
		Model:         trUpgrade(lang, "modelLaptopCool"),
		EstimatedGain: coolGain,
		Priority:      3,
		Body:          coolBody,
		Pitfalls:      commonPitfalls(pass, "cooling", lang),
	})

	if len(recs) > 3 {
		recs = recs[:3]
	}
	return recs
}

func desktopRecommendations(pass PCPassport, goal, lang string) []UpgradeRecommendation {
	recs := make([]UpgradeRecommendation, 0, 6)
	socket := socketOf(pass)
	gpuModel, gpuGain := pickGPUUpgrade(pass, goal)
	cpuModel, cpuGain := pickCPUUpgrade(pass, goal, socket)

	switch goal {
	case UpgradeGoalMaxFPS:
		recs = append(recs, UpgradeRecommendation{
			ID: "desktop-gpu", Title: trUpgrade(lang, "titleGPU"), Part: "gpu",
			Model: gpuModel, EstimatedGain: gpuGain, Priority: 1,
			Body:     fmt.Sprintf(trUpgrade(lang, "bodyGPUFPS"), pass.GPUName, gpuModel, gpuGain),
			Pitfalls: commonPitfalls(pass, "gpu", lang),
		})
		recs = append(recs, UpgradeRecommendation{
			ID: "desktop-cpu", Title: trUpgrade(lang, "titleCPU"), Part: "cpu",
			Model: cpuModel, EstimatedGain: cpuGain, Priority: 2,
			Body:     fmt.Sprintf(trUpgrade(lang, "bodyCPUFPS"), socket, cpuModel, pass.CPUName),
			Pitfalls: commonPitfalls(pass, "cpu", lang),
		})
		if pass.IsSingleChannel || pass.RAMSlotsFree > 0 {
			recs = append(recs, ramRec(pass, goal, lang, 3))
		} else if pass.RAMTotalGB < 32 {
			recs = append(recs, ramRec(pass, goal, lang, 3))
		}
	case UpgradeGoalQuiet:
		recs = append(recs, UpgradeRecommendation{
			ID: "desktop-cooling", Title: trUpgrade(lang, "titleCool"), Part: "cooling",
			Model: trUpgrade(lang, "modelTowerCool"), EstimatedGain: trUpgrade(lang, "gainQuieter"), Priority: 1,
			Body:     trUpgrade(lang, "bodyDesktopCool"),
			Pitfalls: commonPitfalls(pass, "cooling", lang),
		})
		recs = append(recs, UpgradeRecommendation{
			ID: "desktop-gpu", Title: trUpgrade(lang, "titleGPUQuiet"), Part: "gpu",
			Model: gpuModel, EstimatedGain: trUpgrade(lang, "gainQuietGPU"), Priority: 2,
			Body:     fmt.Sprintf(trUpgrade(lang, "bodyGPUQuiet"), gpuModel),
			Pitfalls: commonPitfalls(pass, "gpu", lang),
		})
		recs = append(recs, UpgradeRecommendation{
			ID: "desktop-cpu", Title: trUpgrade(lang, "titleCPUQuiet"), Part: "cpu",
			Model: cpuModel, EstimatedGain: trUpgrade(lang, "gainLowerTemp"), Priority: 3,
			Body:     fmt.Sprintf(trUpgrade(lang, "bodyCPUQuiet"), socket, cpuModel),
			Pitfalls: commonPitfalls(pass, "cpu", lang),
		})
	case UpgradeGoalWork:
		recs = append(recs, ramRec(pass, goal, lang, 1))
		recs = append(recs, UpgradeRecommendation{
			ID: "desktop-cpu", Title: trUpgrade(lang, "titleCPUWork"), Part: "cpu",
			Model: cpuModel, EstimatedGain: cpuGain, Priority: 2,
			Body:     fmt.Sprintf(trUpgrade(lang, "bodyCPUWork"), socket, cpuModel),
			Pitfalls: commonPitfalls(pass, "cpu", lang),
		})
		if pass.HasFreeM2 {
			recs = append(recs, nvmeRec(pass, lang, 3))
		} else {
			recs = append(recs, UpgradeRecommendation{
				ID: "desktop-gpu", Title: trUpgrade(lang, "titleGPUWork"), Part: "gpu",
				Model: gpuModel, EstimatedGain: gpuGain, Priority: 3,
				Body:     fmt.Sprintf(trUpgrade(lang, "bodyGPUWork"), gpuModel),
				Pitfalls: commonPitfalls(pass, "gpu", lang),
			})
		}
	default: // budget
		if pass.IsSingleChannel || pass.RAMSlotsFree > 0 || pass.RAMTotalGB < 16 {
			recs = append(recs, ramRec(pass, goal, lang, 1))
		}
		recs = append(recs, UpgradeRecommendation{
			ID: "desktop-gpu", Title: trUpgrade(lang, "titleGPUBudget"), Part: "gpu",
			Model: gpuModel, EstimatedGain: gpuGain, Priority: 2,
			Body:     fmt.Sprintf(trUpgrade(lang, "bodyGPUBudget"), pass.GPUName, gpuModel),
			Pitfalls: commonPitfalls(pass, "gpu", lang),
		})
		if pass.HasFreeM2 {
			recs = append(recs, nvmeRec(pass, lang, 3))
		} else {
			recs = append(recs, UpgradeRecommendation{
				ID: "desktop-cpu", Title: trUpgrade(lang, "titleCPUBudget"), Part: "cpu",
				Model: cpuModel, EstimatedGain: cpuGain, Priority: 3,
				Body:     fmt.Sprintf(trUpgrade(lang, "bodyCPUBudget"), socket, cpuModel),
				Pitfalls: commonPitfalls(pass, "cpu", lang),
			})
		}
	}

	return recs
}

func desktopFallback(pass PCPassport, goal, lang string) []UpgradeRecommendation {
	socket := socketOf(pass)
	cpuModel, cpuGain := pickCPUUpgrade(pass, goal, socket)
	return []UpgradeRecommendation{{
		ID: "desktop-cpu-fallback", Title: trUpgrade(lang, "titleCPU"), Part: "cpu",
		Model: cpuModel, EstimatedGain: cpuGain, Priority: 2,
		Body:     fmt.Sprintf(trUpgrade(lang, "bodyCPUFPS"), socket, cpuModel, pass.CPUName),
		Pitfalls: commonPitfalls(pass, "cpu", lang),
	}}
}

func ramRec(pass PCPassport, goal, lang string, priority int) UpgradeRecommendation {
	ramType := pass.RAMType
	if ramType == "" {
		ramType = "DDR4"
	}
	model := fmt.Sprintf("2x16 GB %s dual-channel", ramType)
	gain := trUpgrade(lang, "gainRAM32")
	body := fmt.Sprintf(trUpgrade(lang, "bodyRAM"), pass.RAMSlotsFree, ramType)
	if pass.IsSingleChannel {
		model = fmt.Sprintf("1x %s match stick (dual-channel)", ramType)
		gain = "+15-30% FPS (iGPU) / snappier"
		if !strings.Contains(strings.ToLower(pass.GPUName), "intel") && !pass.GPUIntegrated {
			gain = trUpgrade(lang, "gainDualChannel")
		}
		body = fmt.Sprintf(trUpgrade(lang, "bodyRAMDual"), ramType)
	}
	if goal == UpgradeGoalWork {
		gain = "32-64 GB"
		body = fmt.Sprintf(trUpgrade(lang, "bodyRAMWork"), ramType)
	}
	if goal == UpgradeGoalBudget && pass.IsSingleChannel {
		gain = trUpgrade(lang, "gainBestValue")
	}
	return UpgradeRecommendation{
		ID: "desktop-ram", Title: trUpgrade(lang, "titleRAM"), Part: "ram",
		Model: model, EstimatedGain: gain, Priority: priority,
		Body: body, Pitfalls: commonPitfalls(pass, "ram", lang),
	}
}

func nvmeRec(pass PCPassport, lang string, priority int) UpgradeRecommendation {
	return UpgradeRecommendation{
		ID: "desktop-nvme", Title: trUpgrade(lang, "titleSSD"), Part: "storage",
		Model: "NVMe 1-2 TB", EstimatedGain: trUpgrade(lang, "gainLoadTimes"), Priority: priority,
		Body:     trUpgrade(lang, "bodyDesktopSSD"),
		Pitfalls: commonPitfalls(pass, "storage", lang),
	}
}

func pickGPUUpgrade(pass PCPassport, goal string) (model, gain string) {
	name := strings.ToLower(pass.GPUName)
	if pass.GPUIntegrated || strings.Contains(name, "uhd") || strings.Contains(name, "radeon graphics") ||
		strings.Contains(name, "vega") && !strings.Contains(name, "rx") {
		if goal == UpgradeGoalBudget {
			return "RTX 4060", "+200%+ FPS vs iGPU"
		}
		return "RTX 4060 / RX 7600", "+200%+ FPS vs iGPU"
	}
	if strings.Contains(name, "4090") || strings.Contains(name, "4080") || strings.Contains(name, "5090") || strings.Contains(name, "5080") {
		return "Keep current GPU + undervolt", "0% (already top)"
	}
	if strings.Contains(name, "4070") || strings.Contains(name, "3080") || strings.Contains(name, "7900") {
		if goal == UpgradeGoalBudget {
			return "Keep GPU / used 4070", "small / used deal"
		}
		return "RTX 5070 / 4070 Ti Super", "+25-40% FPS"
	}
	if strings.Contains(name, "3060") || strings.Contains(name, "3070") || strings.Contains(name, "2080") ||
		strings.Contains(name, "6700") || strings.Contains(name, "4060") {
		if goal == UpgradeGoalBudget {
			return "RTX 4060 Ti 16GB (used OK)", "+30-45% FPS"
		}
		if goal == UpgradeGoalQuiet {
			return "RTX 4070 Super (3-fan)", "+50-60% FPS, quieter"
		}
		if goal == UpgradeGoalWork {
			return "RTX 4070 Super 12GB", "+VRAM / export"
		}
		return "RTX 4070 Super", "+60% FPS"
	}
	if strings.Contains(name, "1650") || strings.Contains(name, "1660") || strings.Contains(name, "1050") ||
		strings.Contains(name, "5500") || strings.Contains(name, "5600") {
		return "RTX 4060", "+80-120% FPS"
	}
	if goal == UpgradeGoalBudget {
		return "RTX 4060", "+40-80% FPS"
	}
	return "RTX 4070 Super", "+40-70% FPS"
}

func pickCPUUpgrade(pass PCPassport, goal, socket string) (model, gain string) {
	socket = strings.ToUpper(socket)
	name := strings.ToLower(pass.CPUName)
	switch {
	case strings.Contains(socket, "AM4"):
		if goal == UpgradeGoalWork {
			return "Ryzen 9 5950X / 5900X", "+multi-core"
		}
		if goal == UpgradeGoalQuiet || goal == UpgradeGoalBudget {
			if strings.Contains(name, "5700") || strings.Contains(name, "5800") {
				return "Keep " + shortCPU(pass.CPUName) + " + better cooler", "quieter"
			}
			return "Ryzen 5 5600 / 5700X", "AM4 value"
		}
		// max FPS: X3D on same socket
		if strings.Contains(name, "x3d") {
			return "Keep " + shortCPU(pass.CPUName), "already gaming CPU"
		}
		return "Ryzen 7 5700X3D / 5800X3D", "+10-20% 1% lows"
	case strings.Contains(socket, "AM5"):
		if goal == UpgradeGoalWork {
			return "Ryzen 9 7900 / 9950X", "+multi-core"
		}
		if goal == UpgradeGoalBudget {
			return "Ryzen 5 7600 / 9600X", "AM5 value"
		}
		return "Ryzen 7 7800X3D / 9800X3D", "+gaming lows"
	case strings.Contains(socket, "1700"):
		if goal == UpgradeGoalWork {
			return "Core i7-14700K / i9-14900K", "+cores"
		}
		if goal == UpgradeGoalBudget {
			return "Core i5-14600K / 13600K", "LGA1700 value"
		}
		return "Core i5-14600K / i7-14700K", "+1% lows"
	case strings.Contains(socket, "1200"):
		return "Core i7-10700 / i5-11400 (used)", "same socket only"
	case strings.Contains(socket, "1851"):
		return "Core Ultra 7 (same socket)", "platform match"
	default:
		return "CPU for " + socket, "same socket"
	}
}

func shortCPU(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "AMD ", "")
	name = strings.ReplaceAll(name, "Intel ", "")
	if len(name) > 36 {
		return name[:36]
	}
	if name == "" {
		return "current CPU"
	}
	return name
}

func trUpgrade(lang, key string) string {
	if m, ok := upgradeI18n[lang]; ok {
		if v, ok2 := m[key]; ok2 && v != "" {
			return v
		}
	}
	if v, ok := upgradeI18n["en"][key]; ok {
		return v
	}
	if v, ok := upgradeI18n["ru"][key]; ok {
		return v
	}
	return key
}

// upgradeI18n holds Upgrade Advisor copy for all 7 app languages.
var upgradeI18n = map[string]map[string]string{
	"ru": {
		"kindDesktop": "ПК", "kindLaptop": "ноутбук",
		"sumFPS":    "Цель: максимум FPS. Устройство: %s. Конкретные модели под ваш сокет и текущую видеокарту.",
		"sumQuiet":  "Цель: тише и холоднее. Устройство: %s. Охлаждение, undervolt и карты с тихим кулером.",
		"sumWork":   "Цель: работа и учёба. Устройство: %s. RAM, ядра на текущем сокете и быстрый NVMe.",
		"sumBudget": "Цель: апгрейд с умом. Устройство: %s. Сначала dual-channel RAM и накопитель, потом GPU.",
		"pitCPU1":   "Обновите BIOS перед установкой более нового процессора. Старая прошивка может не узнать чип.",
		"pitCPU2":   "Проверьте VRM на плате. Слабое питание греется на мощных CPU и снижает частоту.",
		"pitCPU3":   "Сокет сейчас: %s. Новый процессор должен быть для этого сокета, иначе не встанет.",
		"pitGPU1":   "Перед покупкой GPU измерьте место в корпусе и проверьте мощность и разъёмы блока питания.",
		"pitGPU2":   "Нужен запас по ваттам и родные разъёмы (8-pin / 12VHPWR). Слабый БП переходником не спасти.",
		"pitGPU3":   "После установки поставьте свежий драйвер NVIDIA или AMD. При смене бренда помогите DDU.",
		"pitRAM1":   "Тип памяти: %s. Другое поколение (DDR4/DDR5) физически не подойдёт.",
		"pitRAM2":   "Совпадайте частоту и тайминги модулей. Разные планки часто дают сбои или падение частоты.",
		"pitRAM3":   "В BIOS включите XMP/EXPO, иначе планки могут работать медленнее паспортных.",
		"pitSSD1":   "Для NVMe нужен свободный M.2 и иногда радиатор. Без него диск может троттлить.",
		"pitSSD2":   "Клонируйте систему или поставьте Windows заново. Просто вставить диск мало для переноса ОС.",
		"pitSSD3":   "SATA SSD дешевле, но медленнее NVMe. Для больших проектов разница заметнее, чем в играх.",
		"pitCool1":  "Смените термопасту и проверьте посадку кулера. Воздух должен выходить из корпуса.",
		"pitCool2":  "Undervolt снижайте маленькими шагами и проверяйте стабильность.",
		"pitCool3":  "На ноутбуке не закрывайте вентиляцию на мягкой поверхности.",
		"pitOther1": "Перед апгрейдом сделайте резервную копию важных файлов.",
		"pitOther2": "Отключайте питание от розетки перед работой внутри корпуса.",
		"titleGPU": "Видеокарта", "titleGPUQuiet": "Тихая видеокарта", "titleGPUWork": "GPU для работы",
		"titleGPUBudget": "Бюджетная видеокарта", "titleCPU": "Процессор под сокет",
		"titleCPUQuiet": "CPU с умеренным TDP", "titleCPUWork": "Больше ядер на сокете",
		"titleCPUBudget": "CPU без смены платы", "titleRAM": "Оперативная память",
		"titleRAMDual": "RAM: dual-channel", "titleSSD": "NVMe SSD", "titleCool": "Тихое охлаждение",
		"titleLaptopSSD": "Второй NVMe", "titleLaptopCool": "Охлаждение и undervolt",
		"modelTowerCool": "Башенный кулер 120/140 мм", "modelLaptopCool": "Паста + чистка + undervolt",
		"bodyGPUFPS":     "Сейчас %s. Переход на %s даёт примерно %s в типичных играх на 1080p/1440p.",
		"bodyCPUFPS":     "Сокет %s ещё живой: %s встанет без смены платы. Сейчас у вас %s. X3D или свежий mid-range снимает CPU limit.",
		"bodyGPUQuiet":   "%s с трёхвентиляторным охлаждением и undervolt работает тише коротких «дутышей».",
		"bodyCPUQuiet":   "На сокете %s берите %s или оставьте текущий чип и поставьте хороший башенный кулер.",
		"bodyCPUWork":    "Для работы на сокете %s логичен %s: больше ядер для рендера, компиляции и виртуалок.",
		"bodyGPUWork":    "%s даёт запас VRAM для монтажа и 3D. Для чистого офиса достаточно текущей карты.",
		"bodyGPUBudget":  "Сейчас %s. %s (в том числе б/у) даёт заметный прирост без смены всей платформы.",
		"bodyCPUBudget":  "Бюджетный шаг на сокете %s: %s. Новая материнская плата съест весь бюджет.",
		"bodyRAM":        "Свободно слотов: %d. Наберите 32 ГБ %s в dual-channel одной частоты и таймингов.",
		"bodyRAMDual":    "Сейчас один канал. Вторая планка %s той же частоты включает dual-channel и ускоряет систему.",
		"bodyRAMWork":    "Для работы целитесь в 32 или 64 ГБ %s. Все модули одной частоты и таймингов.",
		"bodyDesktopSSD": "Поставьте систему и проекты на NVMe. Свободный M.2: быстрый апгрейд без замены всего ПК.",
		"bodyDesktopCool": "Башенный кулер, нормальный поток воздуха и undervolt GPU/CPU часто тише смены железа.",
		"bodyLaptopRAM":  "Вторая планка %s той же частоты включает dual-channel. На ноутбуке CPU/GPU обычно впаяны, RAM реально меняется.",
		"bodyLaptopSSD":  "Второй M.2 или замена SSD ускоряет загрузку игр и проектов. Проверьте сервисное руководство по слоту.",
		"bodyLaptopCool": "Чистка, новая паста и undervolt. Реальный апгрейд ноутбука без замены платы.",
		"bodyLaptopCoolFPS": "Охлаждение держит частоты дольше. На ноутбуке это часто даёт больше стабильного FPS, чем «разгон всего».",
		"bodyLaptopCoolQuiet": "Главный путь к тишине: чистка, паста, undervolt и тихий профиль питания. Новый GPU сюда не поставить.",
		"gainSnappier": "быстрее отклик", "gainBestValue": "лучший $/эффект", "gainLoadTimes": "быстрее загрузка",
		"gainProjects": "быстрее проекты", "gainQuieter": "тише на 5-15°C", "gainStableFPS": "стабильнее FPS",
		"gainMuchQuieter": "заметно тише", "gainQuietGPU": "тише под нагрузкой", "gainLowerTemp": "ниже температура",
		"gainRAM32": "меньше статтеров", "gainDualChannel": "+dual-channel / меньше лагов",
	},
	"en": {
		"kindDesktop": "desktop", "kindLaptop": "laptop",
		"sumFPS":    "Goal: max FPS. Device: %s. Concrete models for your socket and current GPU.",
		"sumQuiet":  "Goal: quieter and cooler. Device: %s. Cooling, undervolt and quiet GPU coolers.",
		"sumWork":   "Goal: work and study. Device: %s. RAM, cores on the current socket and fast NVMe.",
		"sumBudget": "Goal: smart upgrade. Device: %s. Dual-channel RAM and storage first, then GPU.",
		"pitCPU1":   "Update the motherboard BIOS before installing a newer CPU. Old firmware may not recognize it.",
		"pitCPU2":   "Check board VRM. Weak power delivery heats up on strong CPUs and clocks down.",
		"pitCPU3":   "Socket now: %s. The new CPU must match this socket or it will not fit.",
		"pitGPU1":   "Before buying a GPU, measure case clearance and check PSU wattage and connectors.",
		"pitGPU2":   "You need spare watts and native connectors (8-pin / 12VHPWR). Adapters will not save a weak PSU.",
		"pitGPU3":   "Install a fresh NVIDIA or AMD driver after install. Use DDU when switching brands.",
		"pitRAM1":   "Memory type: %s. Another generation (DDR4/DDR5) will not fit physically.",
		"pitRAM2":   "Match frequency and timings across modules. Mixed sticks often fail or downclock.",
		"pitRAM3":   "Enable XMP/EXPO in BIOS or sticks may run below rated speed.",
		"pitSSD1":   "NVMe needs a free M.2 slot and sometimes a heatsink or it may throttle.",
		"pitSSD2":   "Clone the OS or reinstall Windows. Inserting a drive alone does not move the system.",
		"pitSSD3":   "SATA SSDs are cheaper but slower than NVMe. The gap matters more for big projects than games.",
		"pitCool1":  "Replace thermal paste and check cooler mounting. Air should leave the case, not recirculate.",
		"pitCool2":  "Lower undervolt in small steps and test stability.",
		"pitCool3":  "On laptops keep vents clear on soft surfaces.",
		"pitOther1": "Back up important files before upgrading.",
		"pitOther2": "Unplug power before working inside the case.",
		"titleGPU": "Graphics card", "titleGPUQuiet": "Quiet graphics card", "titleGPUWork": "GPU for work",
		"titleGPUBudget": "Budget graphics card", "titleCPU": "CPU for your socket",
		"titleCPUQuiet": "Moderate-TDP CPU", "titleCPUWork": "More cores on socket",
		"titleCPUBudget": "CPU without new board", "titleRAM": "System memory",
		"titleRAMDual": "RAM: dual-channel", "titleSSD": "NVMe SSD", "titleCool": "Quiet cooling",
		"titleLaptopSSD": "Second NVMe", "titleLaptopCool": "Cooling and undervolt",
		"modelTowerCool": "120/140 mm tower cooler", "modelLaptopCool": "Paste + clean + undervolt",
		"bodyGPUFPS":     "Now %s. Moving to %s is about %s in typical 1080p/1440p games.",
		"bodyCPUFPS":     "Socket %s is still alive: %s fits without a new board. You have %s. An X3D or fresh mid-range reduces CPU limit.",
		"bodyGPUQuiet":   "%s with a triple-fan cooler and undervolt stays quieter than short blower cards.",
		"bodyCPUQuiet":   "On socket %s pick %s or keep the current chip and add a good tower cooler.",
		"bodyCPUWork":    "For work on socket %s, %s makes sense: more cores for render, compile and VMs.",
		"bodyGPUWork":    "%s adds VRAM headroom for edit and 3D. Office-only workloads can keep the current GPU.",
		"bodyGPUBudget":  "Now %s. %s (including used) gives a clear jump without replacing the whole platform.",
		"bodyCPUBudget":  "Budget step on socket %s: %s. A new motherboard would eat the whole budget.",
		"bodyRAM":        "Free slots: %d. Build 32 GB %s in dual-channel with matching frequency and timings.",
		"bodyRAMDual":    "Single channel now. A second %s stick at the same frequency enables dual-channel.",
		"bodyRAMWork":    "For work aim for 32 or 64 GB %s. All modules same frequency and timings.",
		"bodyDesktopSSD": "Put the OS and projects on NVMe. A free M.2 is a fast upgrade without replacing the PC.",
		"bodyDesktopCool": "A tower cooler, sane airflow and GPU/CPU undervolt often beat swapping hardware for noise.",
		"bodyLaptopRAM":  "A second %s stick at the same frequency enables dual-channel. Laptop CPU/GPU are usually soldered; RAM is the real upgrade.",
		"bodyLaptopSSD":  "A second M.2 or SSD swap speeds game and project load. Check the service manual for the slot.",
		"bodyLaptopCool": "Clean dust, new paste and undervolt. Real laptop upgrade without a board swap.",
		"bodyLaptopCoolFPS": "Cooling holds clocks longer. On a laptop that often means more stable FPS than pushing everything.",
		"bodyLaptopCoolQuiet": "Main path to silence: clean, paste, undervolt and a quiet power plan. You cannot drop in a new GPU.",
		"gainSnappier": "snappier UI", "gainBestValue": "best $/impact", "gainLoadTimes": "faster loads",
		"gainProjects": "faster projects", "gainQuieter": "5-15°C quieter", "gainStableFPS": "more stable FPS",
		"gainMuchQuieter": "much quieter", "gainQuietGPU": "quieter under load", "gainLowerTemp": "lower temps",
		"gainRAM32": "fewer stutters", "gainDualChannel": "+dual-channel / fewer hitches",
	},
}

func init() {
	// Mirror EN into kk/tt/de/ja/zh with light overlays where we have short UI-quality strings.
	for _, lang := range []string{"kk", "tt", "de", "ja", "zh"} {
		dst := map[string]string{}
		for k, v := range upgradeI18n["en"] {
			dst[k] = v
		}
		upgradeI18n[lang] = dst
	}
	// German overlays
	de := upgradeI18n["de"]
	de["kindDesktop"] = "PC"
	de["kindLaptop"] = "Notebook"
	de["titleGPU"] = "Grafikkarte"
	de["titleCPU"] = "CPU fuer Sockel"
	de["titleRAM"] = "Arbeitsspeicher"
	de["titleRAMDual"] = "RAM: Dual-Channel"
	de["titleSSD"] = "NVMe-SSD"
	de["titleCool"] = "Leise Kuehlung"
	de["titleLaptopSSD"] = "Zweite NVMe"
	de["titleLaptopCool"] = "Kuehlung und Undervolt"
	de["pitGPU1"] = "Vor dem GPU-Kauf Gehaeuseplatz messen und Netzteil-Leistung sowie Stecker pruefen."
	de["pitCPU1"] = "BIOS aktualisieren, bevor Sie eine neuere CPU einbauen."
	de["pitRAM2"] = "Frequenz und Timings der Module angleichen."
	// Japanese overlays
	ja := upgradeI18n["ja"]
	ja["kindDesktop"] = "デスクトップ"
	ja["kindLaptop"] = "ノートPC"
	ja["titleGPU"] = "グラフィックカード"
	ja["titleCPU"] = "ソケット対応CPU"
	ja["titleRAM"] = "メモリ"
	ja["titleRAMDual"] = "RAM: デュアルチャネル"
	ja["titleSSD"] = "NVMe SSD"
	ja["titleCool"] = "静音冷却"
	ja["titleLaptopSSD"] = "2本目のNVMe"
	ja["titleLaptopCool"] = "冷却とアンダボルト"
	ja["pitGPU1"] = "GPU購入前にケース寸法と電源の容量・コネクタを確認してください。"
	ja["pitCPU1"] = "新しいCPUの前にBIOSを更新してください。"
	ja["pitRAM2"] = "モジュールの周波数とタイミングを揃えてください。"
	// Chinese overlays
	zh := upgradeI18n["zh"]
	zh["kindDesktop"] = "台式机"
	zh["kindLaptop"] = "笔记本"
	zh["titleGPU"] = "显卡"
	zh["titleCPU"] = "同插槽处理器"
	zh["titleRAM"] = "内存"
	zh["titleRAMDual"] = "内存: 双通道"
	zh["titleSSD"] = "NVMe 固态"
	zh["titleCool"] = "静音散热"
	zh["titleLaptopSSD"] = "第二根 NVMe"
	zh["titleLaptopCool"] = "散热与降压"
	zh["pitGPU1"] = "买显卡前量好机箱空间，并检查电源功率与接口。"
	zh["pitCPU1"] = "安装更新的 CPU 前先升级主板 BIOS。"
	zh["pitRAM2"] = "内存条频率与时序要一致。"
	// Kazakh overlays (short)
	kk := upgradeI18n["kk"]
	kk["kindDesktop"] = "ПК"
	kk["kindLaptop"] = "ноутбук"
	kk["titleGPU"] = "Бейнекарта"
	kk["titleCPU"] = "Сокетке сай CPU"
	kk["titleRAM"] = "Жедел жад"
	kk["titleRAMDual"] = "RAM: dual-channel"
	kk["pitGPU1"] = "GPU сатып алмас бұрын корпусты өлшеп, БҚ қуаты мен қосқыштарын тексеріңіз."
	kk["pitCPU1"] = "Жаңа CPU алдында BIOS жаңартыңыз."
	kk["pitRAM2"] = "Модульдердің жиілігі мен таймингтерін сәйкестендіріңіз."
	// Tatar overlays (short)
	tt := upgradeI18n["tt"]
	tt["kindDesktop"] = "ПК"
	tt["kindLaptop"] = "ноутбук"
	tt["titleGPU"] = "Видеокарта"
	tt["titleCPU"] = "Сокетка туры килгән CPU"
	tt["titleRAM"] = "Оператив хәтер"
	tt["titleRAMDual"] = "RAM: dual-channel"
	tt["pitGPU1"] = "GPU алганчы корпусны үлчәгез һәм БП көчен/тоташтыргычларын тикшерегез."
	tt["pitCPU1"] = "Яңарак CPU алдыннан BIOS яңартыгыз."
	tt["pitRAM2"] = "Модульләрнең ешлыгын һәм таймингларын туры китерегез."
}
