package main

import (
	"fmt"
	"math"
	"strings"
)

// DiskFreeSnap is free space on one volume for the download advice.
type DiskFreeSnap struct {
	Mount  string  `json:"mount"`
	FreeGB float64 `json:"freeGB"`
}

// GameFitVerdict is the Phase-4 GamePulse result: fit for THIS PC, not an FPS number.
type GameFitVerdict struct {
	Level       string `json:"level"` // excellent|ok|tight|wont|unknown
	LabelRu     string `json:"labelRu"`
	LabelEn     string `json:"labelEn"`
	ReasonRu    string `json:"reasonRu"`
	ReasonEn    string `json:"reasonEn"`
	Approximate bool   `json:"approximate"` // always true for class-based estimates
	Confident   bool   `json:"confident"`
	// Download / install advice
	StorageNeedGB  float64        `json:"storageNeedGB"`
	StorageNeedOK  bool           `json:"storageNeedOK"`
	Disks          []DiskFreeSnap `json:"disks"`
	RecommendMount string         `json:"recommendMount"`
	DownloadRu     string         `json:"downloadRu"`
	DownloadEn     string         `json:"downloadEn"`
}

// gameStorageGB is local catalog of install size (GB). Unknown → category default.
var gameStorageGB = map[string]float64{
	"cs2": 85, "dota2": 60, "gtav": 110, "gta5": 110, "rdr2": 150, "cyberpunk": 70,
	"fortnite": 40, "valorant": 25, "minecraft": 4, "genshin": 80, "roblox": 5,
	"lol": 22, "wot": 60, "rust": 35, "pubg": 40, "tanksblitz": 8,
}

func storageNeedFor(game GameInfo) (gb float64, ok bool) {
	if game.InstallSizeGB >= 0.01 {
		return math.Round(game.InstallSizeGB*100) / 100, true
	}
	id := strings.ToLower(strings.TrimSpace(game.ID))
	if v, found := gameStorageGB[id]; found && v > 0 {
		return v, true
	}
	if cid := canonicalReqID(game); cid != "" {
		if v, found := gameStorageGB[cid]; found && v > 0 {
			return v, true
		}
	}

	// Check Steam / Calibrated requirements
	data := GetCachedOrFetchGameReqs(game.SteamAppID, game.Name, game.Year, game.Category)
	if data.StorageNeedGB > 0 {
		return data.StorageNeedGB, true
	}

	// Era calibration
	if game.Year > 0 && game.Year <= 2000 {
		return 0.15, true
	}
	if game.Year > 2000 && game.Year <= 2008 {
		return 2.0, true
	}
	if game.Year > 2008 && game.Year <= 2014 {
		return 8.0, true
	}

	if isModestGame(game) {
		return 1.5, true
	}
	switch strings.ToLower(game.Category) {
	case "classic":
		return 0.5, true
	case "indie", "casual":
		return 1.5, true
	case "shooter", "moba", "esports":
		return 35, true
	case "sports", "sim":
		return 40, true
	case "aaa":
		return 40, true
	default:
		if game.Year >= 2020 {
			return 25, true
		}
		return 15, true
	}
}

// EvaluateGameFitUnified calculates the unified hardware verdict:
// - "deficit" ("Не хватит", red / bad)
// - "tight" ("Впритык", yellow / warn)
// - "great" ("Отлично", green / ok)
func EvaluateGameFitUnified(sys SystemPowerProfile, game GameInfo, req gameReqSet) (verdict, cls, labelRu, labelEn, reasonRu, reasonEn string) {
	minVRAM := req.Low.ReqVRAMGB
	recVRAM := req.High.ReqVRAMGB
	if recVRAM <= 0 {
		recVRAM = req.Med.ReqVRAMGB
	}
	minRAM := req.ReqRAMGB
	if minRAM <= 0 {
		if game.Year > 0 && game.Year <= 2008 {
			minRAM = 1
		} else if strings.EqualFold(game.Category, "classic") {
			minRAM = 1
		} else if req.Heavy {
			minRAM = 8
		} else {
			minRAM = 4
		}
	}
	recRAM := minRAM * 2
	if recRAM < 8 {
		recRAM = 8
	}
	if req.Heavy && recRAM < 16 {
		recRAM = 16
	}

	userVRAM := sys.VRAMGB
	userRAM := sys.RAMGB

	gpuMin := ratio(sys.GPUPowerScore, req.Low.ReqGPUScore)
	cpuMin := ratio(sys.CPUPowerScore, req.Low.ReqCPUScore)
	gpuRec := ratio(sys.GPUPowerScore, req.High.ReqGPUScore)

	// 1. Deficit ("Не хватит") - only for hard hardware shortages:
	if minVRAM > 0 && userVRAM+0.1 < minVRAM {
		return "deficit", "bad", "Не хватит", "Not enough",
			fmt.Sprintf("VRAM %.0f ГБ, требуется минимум %.0f ГБ", userVRAM, minVRAM),
			fmt.Sprintf("VRAM %.0f GB, minimum %.0f GB required", userVRAM, minVRAM)
	}
	if minRAM > 0 && userRAM+0.5 < minRAM {
		return "deficit", "bad", "Не хватит", "Not enough",
			fmt.Sprintf("ОЗУ %.0f ГБ, требуется минимум %.0f ГБ", userRAM, minRAM),
			fmt.Sprintf("RAM %.0f GB, minimum %.0f GB required", userRAM, minRAM)
	}
	if sys.IsIntegrated && minVRAM >= 4.0 {
		return "deficit", "bad", "Не хватит", "Not enough",
			"встроенная графика (iGPU) не подходит для тяжелых игр",
			"Integrated graphics (iGPU) is not suitable for heavy games"
	}
	if req.Low.ReqGPUScore > 0 && gpuMin < 0.50 {
		return "deficit", "bad", "Не хватит", "Not enough",
			"видеокарта существенно слабее минимальных требований",
			"GPU is significantly below minimum requirements"
	}
	if req.Low.ReqCPUScore > 0 && cpuMin < 0.50 {
		return "deficit", "bad", "Не хватит", "Not enough",
			"процессор существенно слабее минимальных требований",
			"CPU is significantly below minimum requirements"
	}

	// 2. Tight ("Впритык") - meets minimums, but below high/recommended
	if (recVRAM > 0 && userVRAM+0.1 < recVRAM) || (recRAM > 0 && userRAM+0.5 < recRAM) || (req.High.ReqGPUScore > 0 && gpuRec < 0.90) || (req.Low.ReqGPUScore > 0 && gpuMin < 0.95) {
		reasonRu := "подходит для минимальных и средних настроек"
		reasonEn := "suitable for minimum to medium settings"
		if recVRAM > 0 && userVRAM+0.1 < recVRAM {
			reasonRu = fmt.Sprintf("VRAM %.0f ГБ, для ультра рекомендуется %.0f ГБ", userVRAM, recVRAM)
			reasonEn = fmt.Sprintf("VRAM %.0f GB, recommended %.0f GB for ultra", userVRAM, recVRAM)
		} else if recRAM > 0 && userRAM+0.5 < recRAM {
			reasonRu = fmt.Sprintf("ОЗУ %.0f ГБ, рекомендуется %.0f ГБ", userRAM, recRAM)
			reasonEn = fmt.Sprintf("RAM %.0f GB, recommended %.0f GB", userRAM, recRAM)
		} else if (req.High.ReqGPUScore > 0 && gpuRec < 0.90) || (req.Low.ReqGPUScore > 0 && gpuMin < 0.95) {
			if userVRAM >= recVRAM && recVRAM > 0 && !sys.IsIntegrated {
				reasonRu = fmt.Sprintf("по мощности GPU · VRAM %.0f ГБ с запасом", userVRAM)
				reasonEn = fmt.Sprintf("by GPU performance · VRAM %.0f GB with headroom", userVRAM)
			} else {
				reasonRu = "видеокарта подходит для средних настроек (для ультра рекомендуется более мощная)"
				reasonEn = "GPU suitable for medium settings (more powerful GPU recommended for ultra)"
			}
		}
		return "tight", "warn", "Впритык", "Barely", reasonRu, reasonEn
	}

	// 3. Great ("Отлично")
	return "great", "ok", "Отлично", "Great",
		"видеокарта и процессор с запасом выше рекомендованного",
		"GPU and CPU exceed recommended specs with headroom"
}

// EvaluateGameFit compares this PC to min/rec requirement classes (no FPS guessing).
// Min ≈ Low preset scores, Rec ≈ High. Marked approximate - class tables, not a guarantee.
func EvaluateGameFit(sys SystemPowerProfile, game GameInfo, req gameReqSet, disks []DiskFreeSnap) GameFitVerdict {
	out := GameFitVerdict{
		Approximate: true,
		Level:       "unknown",
		LabelRu:     "Не удалось оценить",
		LabelEn:     "Could not assess",
	}
	min := req.Low
	if min.ReqGPUScore <= 0 {
		out.ReasonRu = "Нет данных о требованиях этой игры"
		out.ReasonEn = "No requirement data for this game"
		fillDownloadAdvice(&out, game, disks)
		return out
	}

	confident := sys.GPUPowerScore >= 5 && (sys.IsIntegrated || sys.VRAMGB > 0 || sys.GPUPowerScore >= 20)
	if sys.CPUPowerScore < 5 && sys.CPUCores <= 0 {
		confident = false
	}
	out.Confident = confident
	if !confident {
		out.ReasonRu = "Не хватает данных о CPU/GPU этого ПК для уверенной оценки"
		out.ReasonEn = "Not enough CPU/GPU data on this PC for a confident assessment"
		fillDownloadAdvice(&out, game, disks)
		return out
	}

	verdict, _, lRu, lEn, rRu, rEn := EvaluateGameFitUnified(sys, game, req)
	switch verdict {
	case "great":
		out.Level = "excellent"
	case "tight":
		out.Level = "tight"
	case "deficit":
		out.Level = "wont"
	default:
		out.Level = verdict
	}
	out.LabelRu = lRu
	out.LabelEn = lEn
	out.ReasonRu = rRu
	out.ReasonEn = rEn
	fillDownloadAdvice(&out, game, disks)
	return out
}

func ratio(have, need float64) float64 {
	if need <= 0 {
		return 2
	}
	r := have / need
	if r > 3 {
		return 3
	}
	return r
}

func fitReason(sys SystemPowerProfile, min, rec gameReq, gpuMin, cpuMin, gpuRec, cpuRec float64, vramMinFail, vramRecFail, ramTight bool, level string) (ru, en string) {
	vramHave := sys.VRAMGB
	if sys.IsIntegrated {
		vramHave = 0
	}
	switch {
	case vramMinFail || (level == "wont" && vramRecFail):
		need := min.ReqVRAMGB
		if vramRecFail {
			need = rec.ReqVRAMGB
		}
		return fmt.Sprintf("VRAM %.0f ГБ, нужно ≈%.0f ГБ", vramHave, need),
			fmt.Sprintf("VRAM %.0f GB, needs ≈%.0f GB", vramHave, need)
	case vramRecFail && (level == "tight" || level == "ok"):
		return fmt.Sprintf("VRAM %.0f ГБ, рекомендуется %.0f ГБ", vramHave, rec.ReqVRAMGB),
			fmt.Sprintf("VRAM %.0f GB, recommended %.0f GB", vramHave, rec.ReqVRAMGB)
	case gpuMin < 0.95 || (level == "wont" && gpuRec < 0.95):
		return "видеокарта слабее минимальных требований",
			"GPU is below minimum requirements"
	case cpuMin < 0.90 || (level == "wont" && cpuRec < 0.90):
		return "процессор слабее минимальных требований",
			"CPU is below minimum requirements"
	case gpuRec < 0.95 && level == "tight":
		if vramHave >= rec.ReqVRAMGB && rec.ReqVRAMGB > 0 && !sys.IsIntegrated {
			return fmt.Sprintf("по мощности GPU · VRAM %.0f ГБ с запасом", vramHave),
				fmt.Sprintf("by GPU performance · VRAM %.0f GB with headroom", vramHave)
		}
		return "видеокарта слабее рекомендованной",
			"GPU is below recommended"
	case cpuRec < 0.90 && level == "tight":
		return "процессор слабее рекомендованного",
			"CPU is below recommended"
	case ramTight:
		return fmt.Sprintf("ОЗУ %.0f ГБ - для комфорта лучше 16 ГБ", sys.RAMGB),
			fmt.Sprintf("RAM %.0f GB - 16 GB is more comfortable", sys.RAMGB)
	case level == "excellent":
		return "видеокарта и процессор с запасом выше рекомендованного",
			"GPU and CPU exceed recommended specs with headroom"
	case level == "ok":
		if math.Abs(vramHave-rec.ReqVRAMGB) < 0.6 && rec.ReqVRAMGB > 0 {
			return fmt.Sprintf("VRAM %.0f ГБ, рекомендуется %.0f ГБ · процессор подходит", vramHave, rec.ReqVRAMGB),
				fmt.Sprintf("VRAM %.0f GB, recommended %.0f GB · CPU is fine", vramHave, rec.ReqVRAMGB)
		}
		return "железо примерно на уровне рекомендованного · процессор подходит",
			"hardware roughly matches recommended · CPU is fine"
	default:
		return "оценка по классам производительности (ориентировочно)",
			"class-based performance estimate (approximate)"
	}
}

func fillDownloadAdvice(out *GameFitVerdict, game GameInfo, disks []DiskFreeSnap) {
	need, ok := storageNeedFor(game)
	out.StorageNeedGB = need
	out.StorageNeedOK = ok
	out.Disks = disks
	best := ""
	bestFree := -1.0
	for _, d := range disks {
		if d.FreeGB > bestFree {
			bestFree = d.FreeGB
			best = d.Mount
		}
	}
	out.RecommendMount = best

	fitWordRu, fitWordEn := out.LabelRu, out.LabelEn
	if out.Level == "unknown" {
		fitWordRu, fitWordEn = "неясно потянет ли ПК", "unclear if this PC can run it"
	}

	diskLineRu := formatDiskFreeRu(disks)
	diskLineEn := formatDiskFreeEn(disks)
	whereRu, whereEn := "", ""
	if best != "" && bestFree >= need {
		whereRu = fmt.Sprintf(" Лучше ставить на %s (свободно ≈%.0f ГБ).", strings.TrimSuffix(best, `:\`), bestFree)
		whereEn = fmt.Sprintf(" Prefer installing on %s (≈%.0f GB free).", strings.TrimSuffix(best, `:\`), bestFree)
	} else if best != "" && need > 0 {
		whereRu = fmt.Sprintf(" На самом свободном диске %s ≈%.0f ГБ - может не хватить.", strings.TrimSuffix(best, `:\`), bestFree)
		whereEn = fmt.Sprintf(" Freest disk %s has ≈%.0f GB - may not be enough.", strings.TrimSuffix(best, `:\`), bestFree)
	}

	needStrRu, needStrEn := "?", "?"
	if ok {
		if need < 1.0 {
			mb := need * 1024.0
			needStrRu = fmt.Sprintf("%.0f МБ", mb)
			needStrEn = fmt.Sprintf("%.0f MB", mb)
		} else {
			needStrRu = fmt.Sprintf("%.0f ГБ", need)
			needStrEn = fmt.Sprintf("%.0f GB", need)
		}
	}
	out.DownloadRu = fmt.Sprintf("Стоит ли скачивать? %s. Нужно ≈%s. Свободно: %s.%s",
		fitWordRu, needStrRu, diskLineRu, whereRu)
	out.DownloadEn = fmt.Sprintf("Worth downloading? %s. Needs ≈%s. Free: %s.%s",
		fitWordEn, needStrEn, diskLineEn, whereEn)
}

func formatDiskFreeRu(disks []DiskFreeSnap) string {
	if len(disks) == 0 {
		return "нет данных"
	}
	parts := make([]string, 0, len(disks))
	for _, d := range disks {
		letter := strings.TrimSuffix(strings.TrimSuffix(d.Mount, `:\`), ":")
		parts = append(parts, fmt.Sprintf("%s ≈%.0f ГБ", letter, d.FreeGB))
	}
	return strings.Join(parts, ", ")
}

func formatDiskFreeEn(disks []DiskFreeSnap) string {
	if len(disks) == 0 {
		return "no data"
	}
	parts := make([]string, 0, len(disks))
	for _, d := range disks {
		letter := strings.TrimSuffix(strings.TrimSuffix(d.Mount, `:\`), ":")
		parts = append(parts, fmt.Sprintf("%s ≈%.0f GB", letter, d.FreeGB))
	}
	return strings.Join(parts, ", ")
}

func diskFreeFromMetrics(disks []DiskMetrics) []DiskFreeSnap {
	out := make([]DiskFreeSnap, 0, len(disks))
	for _, d := range disks {
		m := strings.TrimSpace(d.Mount)
		if m == "" {
			continue
		}
		// Skip optical / tiny volumes
		if d.TotalGB > 0 && d.TotalGB < 8 {
			continue
		}
		out = append(out, DiskFreeSnap{Mount: m, FreeGB: d.FreeGB})
	}
	return out
}

// fitLevelToFilter maps GameFit level → UI filter ok|warn|bad.
func fitLevelToFilter(level string) string {
	switch level {
	case "excellent", "ok":
		return "ok"
	case "tight":
		return "warn"
	case "wont":
		return "bad"
	default:
		return "warn"
	}
}
