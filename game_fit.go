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
	Level        string `json:"level"` // excellent|ok|tight|wont|unknown
	LabelRu      string `json:"labelRu"`
	LabelEn      string `json:"labelEn"`
	ReasonRu     string `json:"reasonRu"`
	ReasonEn     string `json:"reasonEn"`
	Approximate  bool   `json:"approximate"`  // always true for class-based estimates
	Confident    bool   `json:"confident"`
	// Download / install advice
	StorageNeedGB   float64        `json:"storageNeedGB"`
	StorageNeedOK   bool           `json:"storageNeedOK"`
	Disks           []DiskFreeSnap `json:"disks"`
	RecommendMount  string         `json:"recommendMount"`
	DownloadRu      string         `json:"downloadRu"`
	DownloadEn      string         `json:"downloadEn"`
}

// gameStorageGB is local catalog of install size (GB). Unknown → category default.
var gameStorageGB = map[string]float64{
	"cs2": 85, "dota2": 60, "gtav": 110, "gta5": 110, "rdr2": 150, "cyberpunk": 70,
	"fortnite": 40, "valorant": 25, "minecraft": 4, "genshin": 80, "roblox": 5,
	"lol": 22, "wot": 60, "rust": 35, "pubg": 40, "tanksblitz": 8,
}

func storageNeedFor(game GameInfo) (gb float64, ok bool) {
	id := strings.ToLower(strings.TrimSpace(game.ID))
	if v, found := gameStorageGB[id]; found && v > 0 {
		return v, true
	}
	if id := canonicalReqID(game); id != "" {
		if v, found := gameStorageGB[id]; found && v > 0 {
			return v, true
		}
	}
	switch strings.ToLower(game.Category) {
	case "classic":
		return 2, true
	case "indie":
		return 5, true
	case "shooter", "moba", "esports":
		return 35, true
	case "sports", "sim":
		return 40, true
	case "aaa":
		return 70, true
	default:
		if game.Year > 0 && game.Year < 2010 {
			return 3, true
		}
		if game.Year >= 2020 {
			return 60, true
		}
		return 25, true
	}
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
	rec := req.High
	if rec.ReqGPUScore <= 0 {
		rec = req.Med
	}
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

	gpuMin := ratio(sys.GPUPowerScore, min.ReqGPUScore)
	cpuMin := ratio(sys.CPUPowerScore, min.ReqCPUScore)
	gpuRec := ratio(sys.GPUPowerScore, rec.ReqGPUScore)
	cpuRec := ratio(sys.CPUPowerScore, rec.ReqCPUScore)
	vramMinFail := vramShortfall(sys, min.ReqVRAMGB)
	vramRecFail := vramShortfall(sys, rec.ReqVRAMGB)
	ramTight := sys.RAMGB > 0 && sys.RAMGB+0.5 < 8 && (req.Heavy || rec.ReqVRAMGB >= 6)

	meetsMin := gpuMin >= 0.95 && cpuMin >= 0.90 && !vramMinFail
	meetsRec := gpuRec >= 0.95 && cpuRec >= 0.90 && !vramRecFail
	headroom := gpuRec >= 1.25 && cpuRec >= 1.15 && !vramRecFail

	switch {
	case meetsRec && headroom:
		out.Level = "excellent"
		out.LabelRu, out.LabelEn = "Отлично", "Great"
	case meetsRec:
		out.Level = "ok"
		out.LabelRu, out.LabelEn = "Пойдёт", "Will run"
	case meetsMin:
		out.Level = "tight"
		out.LabelRu, out.LabelEn = "Впритык", "Barely"
	default:
		out.Level = "wont"
		out.LabelRu, out.LabelEn = "Не потянет", "Won't run well"
	}

	// Pre-2017 titles on mid+ VRAM must not be marked «Впритык» when the card clearly has spare memory.
	if game.Year > 0 && game.Year <= 2016 && sys.VRAMGB >= 4 && !vramRecFail && meetsMin {
		if meetsRec || sys.VRAMGB+0.1 >= rec.ReqVRAMGB {
			out.Level = "excellent"
			out.LabelRu, out.LabelEn = "Отлично", "Great"
		}
	}

	out.ReasonRu, out.ReasonEn = fitReason(sys, min, rec, gpuMin, cpuMin, gpuRec, cpuRec, vramMinFail, vramRecFail, ramTight, out.Level)
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
		needStrRu = fmt.Sprintf("%.0f", need)
		needStrEn = fmt.Sprintf("%.0f", need)
	}
	out.DownloadRu = fmt.Sprintf("Стоит ли скачивать? %s. Нужно ≈%s ГБ. Свободно: %s.%s",
		fitWordRu, needStrRu, diskLineRu, whereRu)
	out.DownloadEn = fmt.Sprintf("Worth downloading? %s. Needs ≈%s GB. Free: %s.%s",
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
