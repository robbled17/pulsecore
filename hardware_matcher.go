package main

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	rxNoise = regexp.MustCompile(`(?i)\((?:r|tm)\)|\b(?:processor|graphics|edition|series|\d+-core|\d+-thread|desktop|vram|gddr\d*)\b`)
	rxVRAM  = regexp.MustCompile(`(?i)\b\d+\s*(?:gb|g|mb)\b`)
	rxSpace = regexp.MustCompile(`\s+`)
)

// CleanHardwareString normalizes raw hardware brand strings by stripping marketing noise.
func CleanHardwareString(raw string) string {
	s := strings.ToLower(raw)
	s = strings.ReplaceAll(s, "nvidia", " ")
	s = strings.ReplaceAll(s, "geforce", " ")
	s = strings.ReplaceAll(s, "amd", " ")
	s = strings.ReplaceAll(s, "radeon", " ")
	s = strings.ReplaceAll(s, "intel", " ")
	s = rxVRAM.ReplaceAllString(s, " ")
	s = rxNoise.ReplaceAllString(s, " ")
	s = rxSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// SplitAlternatives breaks strings containing '/', 'or', '|' into individual hardware choices.
func SplitAlternatives(raw string) []string {
	parts := regexp.MustCompile(`(?i)\s+(?:or|\|)\s+|\s*/\s*`).Split(raw, -1)
	var res []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			res = append(res, trimmed)
		}
	}
	if len(res) == 0 && strings.TrimSpace(raw) != "" {
		res = append(res, strings.TrimSpace(raw))
	}
	return res
}

// ResolveSingleGPUScore resolves a single GPU string against the DB or heuristic fallback.
func ResolveSingleGPUScore(raw string, isIntegrated bool, vramGB float64) float64 {
	db := GetHardwareDB()
	cleaned := CleanHardwareString(raw)
	lowerRaw := strings.ToLower(raw)
	isLaptop := strings.Contains(lowerRaw, "laptop") || strings.Contains(lowerRaw, "mobile") || strings.Contains(lowerRaw, "max-q")

	// 1. Exact match in DB
	if entry, ok := db.GPUs[cleaned]; ok {
		score := entry.Score
		if isLaptop && entry.Type != "laptop" {
			score *= 0.80
		}
		return math.Round(score)
	}

	// 2. Substring match in DB (longest key first)
	var bestMatch GPUEntry
	bestLen := 0
	for k, entry := range db.GPUs {
		if strings.Contains(cleaned, k) || strings.Contains(lowerRaw, k) {
			if len(k) > bestLen {
				bestLen = len(k)
				bestMatch = entry
			}
		}
	}
	if bestLen > 0 {
		score := bestMatch.Score
		if isLaptop && bestMatch.Type != "laptop" {
			score *= 0.80
		}
		return math.Round(score)
	}

	// 3. Heuristic fallback (unknown hardware never gets 0)
	score := 40.0
	switch {
	case strings.Contains(cleaned, "5090") || strings.Contains(cleaned, "4090"):
		score = 495.0
	case strings.Contains(cleaned, "5080") || strings.Contains(cleaned, "4080"):
		score = 410.0
	case strings.Contains(cleaned, "5070") || strings.Contains(cleaned, "4070"):
		score = 285.0
	case strings.Contains(cleaned, "5060") || strings.Contains(cleaned, "4060"):
		score = 198.0
	case strings.Contains(cleaned, "3090"):
		score = 330.0
	case strings.Contains(cleaned, "3080"):
		score = 290.0
	case strings.Contains(cleaned, "3070"):
		score = 235.0
	case strings.Contains(cleaned, "3060"):
		score = 168.0
	case strings.Contains(cleaned, "2080"):
		score = 212.0
	case strings.Contains(cleaned, "2070"):
		score = 182.0
	case strings.Contains(cleaned, "2060"):
		score = 152.0
	case strings.Contains(cleaned, "1660"):
		score = 115.0
	case strings.Contains(cleaned, "1650"):
		score = 74.0
	case strings.Contains(cleaned, "1080"):
		score = 165.0
	case strings.Contains(cleaned, "1070"):
		score = 135.0
	case strings.Contains(cleaned, "1060"):
		score = 100.0
	case strings.Contains(cleaned, "1050"):
		score = 66.0
	case strings.Contains(cleaned, "7900"):
		score = 410.0
	case strings.Contains(cleaned, "7800"):
		score = 328.0
	case strings.Contains(cleaned, "7700"):
		score = 278.0
	case strings.Contains(cleaned, "7600"):
		score = 196.0
	case strings.Contains(cleaned, "6900") || strings.Contains(cleaned, "6950"):
		score = 335.0
	case strings.Contains(cleaned, "6800"):
		score = 280.0
	case strings.Contains(cleaned, "6700"):
		score = 220.0
	case strings.Contains(cleaned, "6600"):
		score = 155.0
	case strings.Contains(cleaned, "5700"):
		score = 175.0
	case strings.Contains(cleaned, "5600"):
		score = 145.0
	case strings.Contains(cleaned, "580"):
		score = 96.0
	case strings.Contains(cleaned, "570"):
		score = 84.0
	case isIntegrated || strings.Contains(cleaned, "uhd") || strings.Contains(cleaned, "hd") || strings.Contains(cleaned, "vega"):
		score = 18.0
	default:
		if vramGB >= 16 {
			score = 280.0
		} else if vramGB >= 12 {
			score = 220.0
		} else if vramGB >= 8 {
			score = 140.0
		} else if vramGB >= 6 {
			score = 100.0
		} else if vramGB >= 4 {
			score = 65.0
		} else {
			score = 35.0
		}
	}

	if isLaptop {
		score *= 0.80
	}
	if isIntegrated && score > 35 {
		score = 25
	}
	if score < 10 {
		score = 10
	}
	return math.Round(score)
}

// ResolveGPUScore resolves a GPU string, splitting alternatives ('/' or 'or') and taking the minimum threshold score.
func ResolveGPUScore(raw string, isIntegrated bool, vramGB float64) float64 {
	alts := SplitAlternatives(raw)
	if len(alts) <= 1 {
		return ResolveSingleGPUScore(raw, isIntegrated, vramGB)
	}
	minScore := math.MaxFloat64
	for _, alt := range alts {
		s := ResolveSingleGPUScore(alt, isIntegrated, vramGB)
		if s > 0 && s < minScore {
			minScore = s
		}
	}
	if minScore == math.MaxFloat64 {
		return 35.0
	}
	return minScore
}

// ResolveSingleCPUScore resolves a single CPU string against the DB or heuristic fallback.
func ResolveSingleCPUScore(raw string, cores, threads int, freqMHz float64) float64 {
	db := GetHardwareDB()
	cleaned := CleanHardwareString(raw)
	lowerRaw := strings.ToLower(raw)

	// 1. Exact match in DB
	if entry, ok := db.CPUs[cleaned]; ok {
		return entry.Score
	}

	// 2. Substring match in DB (longest key first)
	var bestMatch CPUEntry
	bestLen := 0
	for k, entry := range db.CPUs {
		if strings.Contains(cleaned, k) || strings.Contains(lowerRaw, k) {
			if len(k) > bestLen {
				bestLen = len(k)
				bestMatch = entry
			}
		}
	}
	if bestLen > 0 {
		return bestMatch.Score
	}

	// 3. Heuristic fallback based on cores, threads, and frequency
	if cores <= 0 {
		// Try parsing from raw string
		if m := regexp.MustCompile(`(?i)(\d+)\s*-?\s*core`).FindStringSubmatch(raw); len(m) == 2 {
			cores, _ = strconv.Atoi(m[1])
		} else {
			cores = 4
		}
	}
	if threads <= 0 {
		threads = cores
	}

	base := 45.0
	switch {
	case cores >= 16:
		base = 280.0
	case cores >= 12:
		base = 230.0
	case cores >= 8:
		if threads >= 16 {
			base = 190.0
		} else {
			base = 150.0
		}
	case cores >= 6:
		if threads >= 12 {
			base = 140.0
		} else {
			base = 100.0
		}
	case cores >= 4:
		if threads >= 8 {
			base = 85.0
		} else {
			base = 60.0
		}
	default:
		base = 35.0
	}

	if freqMHz > 0 {
		freqFactor := freqMHz / 3400.0
		if freqFactor > 1.3 {
			freqFactor = 1.3
		}
		if freqFactor < 0.75 {
			freqFactor = 0.75
		}
		base *= freqFactor
	}

	if base < 20 {
		base = 20
	}
	return math.Round(base)
}

// ResolveCPUScore resolves a CPU string, splitting alternatives ('/' or 'or') and taking the minimum threshold score.
func ResolveCPUScore(raw string, cores, threads int, freqMHz float64) float64 {
	alts := SplitAlternatives(raw)
	if len(alts) <= 1 {
		return ResolveSingleCPUScore(raw, cores, threads, freqMHz)
	}
	minScore := math.MaxFloat64
	for _, alt := range alts {
		s := ResolveSingleCPUScore(alt, cores, threads, freqMHz)
		if s > 0 && s < minScore {
			minScore = s
		}
	}
	if minScore == math.MaxFloat64 {
		return 45.0
	}
	return minScore
}

// SystemHardwareInput represents the current system hardware specs.
type SystemHardwareInput struct {
	CPUName      string  `json:"cpuName"`
	CPUCores     int     `json:"cpuCores"`
	CPUThreads   int     `json:"cpuThreads"`
	CPUFreqMHz   float64 `json:"cpuFreqMHz"`
	GPUName      string  `json:"gpuName"`
	GPUVRAMGB    float64 `json:"gpuVramGB"`
	IsIntegrated bool    `json:"isIntegrated"`
	SystemRAMGB  float64 `json:"systemRamGB"`
}

// GameHardwareReq represents the system requirements of a target game.
type GameHardwareReq struct {
	MinCPU  string  `json:"minCpu"`
	RecCPU  string  `json:"recCpu"`
	MinGPU  string  `json:"minGpu"`
	RecGPU  string  `json:"recGpu"`
	MinRAM  float64 `json:"minRam"`
	RecRAM  float64 `json:"recRam"`
	MinVRAM float64 `json:"minVram"`
	RecVRAM float64 `json:"recVram"`
}

// HardwareCompatibilityVerdict is the 4-factor compatibility verdict.
type HardwareCompatibilityVerdict struct {
	Verdict     string  `json:"verdict"`     // "НЕ ПОТЯНЕТ", "МИНИМАЛЬНЫЕ · 30–45 FPS", "ВЫСОКИЕ · 60+ FPS", "УЛЬТРА · 60+ FPS"
	VerdictCode string  `json:"verdictCode"` // "fail", "min", "high", "ultra"
	Color       string  `json:"color"`       // "#ef4444", "#f59e0b", "#10b981", "#06b6d4"
	GPUScore    float64 `json:"gpuScore"`
	CPUScore    float64 `json:"cpuScore"`
	MinGPUScore float64 `json:"minGpuScore"`
	RecGPUScore float64 `json:"recGpuScore"`
	MinCPUScore float64 `json:"minCpuScore"`
	RecCPUScore float64 `json:"recCpuScore"`
	Bottleneck  string  `json:"bottleneck"`
	Details     string  `json:"details"`
}

// EvaluateGameCompatibility evaluates compatibility across 4 factors (GPU, CPU, RAM, VRAM).
func EvaluateGameCompatibility(sys SystemHardwareInput, req GameHardwareReq) HardwareCompatibilityVerdict {
	gpuScore := ResolveGPUScore(sys.GPUName, sys.IsIntegrated, sys.GPUVRAMGB)
	cpuScore := ResolveCPUScore(sys.CPUName, sys.CPUCores, sys.CPUThreads, sys.CPUFreqMHz)

	minGPUScore := ResolveGPUScore(req.MinGPU, false, req.MinVRAM)
	if minGPUScore <= 0 {
		minGPUScore = 80.0
	}
	recGPUScore := ResolveGPUScore(req.RecGPU, false, req.RecVRAM)
	if recGPUScore <= 0 {
		recGPUScore = math.Max(minGPUScore*1.5, 140.0)
	}

	minCPUScore := ResolveCPUScore(req.MinCPU, 4, 4, 3000)
	if minCPUScore <= 0 {
		minCPUScore = 65.0
	}
	recCPUScore := ResolveCPUScore(req.RecCPU, 6, 12, 3600)
	if recCPUScore <= 0 {
		recCPUScore = math.Max(minCPUScore*1.4, 120.0)
	}

	minRAM := req.MinRAM
	if minRAM <= 0 {
		minRAM = 8.0
	}
	recRAM := req.RecRAM
	if recRAM <= 0 {
		recRAM = 16.0
	}

	minVRAM := req.MinVRAM
	if minVRAM <= 0 {
		minVRAM = 3.0
	}
	recVRAM := req.RecVRAM
	if recVRAM <= 0 {
		recVRAM = 6.0
	}

	// 1. Check for failure (НЕ ПОТЯНЕТ)
	isFail := false
	bottleneck := "none"

	if gpuScore < minGPUScore*0.80 {
		isFail = true
		bottleneck = "gpu"
	} else if cpuScore < minCPUScore*0.80 {
		isFail = true
		bottleneck = "cpu"
	} else if sys.SystemRAMGB > 0 && sys.SystemRAMGB < minRAM*0.75 {
		isFail = true
		bottleneck = "ram"
	} else if sys.GPUVRAMGB > 0 && sys.GPUVRAMGB < minVRAM*0.70 {
		isFail = true
		bottleneck = "vram"
	}

	if isFail {
		return HardwareCompatibilityVerdict{
			Verdict:     "НЕ ПОТЯНЕТ",
			VerdictCode: "fail",
			Color:       "#ef4444",
			GPUScore:    gpuScore,
			CPUScore:    cpuScore,
			MinGPUScore: minGPUScore,
			RecGPUScore: recGPUScore,
			MinCPUScore: minCPUScore,
			RecCPUScore: recCPUScore,
			Bottleneck:  bottleneck,
			Details:     "Конфигурация ПК ниже минимальных системных требований игры",
		}
	}

	// 2. Check for Ultra (УЛЬТРА · 60+ FPS)
	isUltra := gpuScore >= recGPUScore*1.35 &&
		cpuScore >= recCPUScore*1.25 &&
		(sys.SystemRAMGB <= 0 || sys.SystemRAMGB >= recRAM) &&
		(sys.GPUVRAMGB <= 0 || sys.GPUVRAMGB >= recVRAM)

	if isUltra {
		return HardwareCompatibilityVerdict{
			Verdict:     "УЛЬТРА · 60+ FPS",
			VerdictCode: "ultra",
			Color:       "#06b6d4",
			GPUScore:    gpuScore,
			CPUScore:    cpuScore,
			MinGPUScore: minGPUScore,
			RecGPUScore: recGPUScore,
			MinCPUScore: minCPUScore,
			RecCPUScore: recCPUScore,
			Bottleneck:  "none",
			Details:     "Запас производительности достаточен для ультра-настроек графики при 60+ FPS",
		}
	}

	// 3. Check for High (ВЫСОКИЕ · 60+ FPS)
	isHigh := gpuScore >= recGPUScore*0.95 &&
		cpuScore >= recCPUScore*0.90 &&
		(sys.SystemRAMGB <= 0 || sys.SystemRAMGB >= recRAM*0.85) &&
		(sys.GPUVRAMGB <= 0 || sys.GPUVRAMGB >= recVRAM*0.85)

	if isHigh {
		return HardwareCompatibilityVerdict{
			Verdict:     "ВЫСОКИЕ · 60+ FPS",
			VerdictCode: "high",
			Color:       "#10b981",
			GPUScore:    gpuScore,
			CPUScore:    cpuScore,
			MinGPUScore: minGPUScore,
			RecGPUScore: recGPUScore,
			MinCPUScore: minCPUScore,
			RecCPUScore: recCPUScore,
			Bottleneck:  "none",
			Details:     "ПК соответствует рекомендуемым требованиям игры: комфортные высокие настройки при 60+ FPS",
		}
	}

	// 4. Default to Minimal (МИНИМАЛЬНЫЕ · 30–45 FPS)
	return HardwareCompatibilityVerdict{
		Verdict:     "МИНИМАЛЬНЫЕ · 30–45 FPS",
		VerdictCode: "min",
		Color:       "#f59e0b",
		GPUScore:    gpuScore,
		CPUScore:    cpuScore,
		MinGPUScore: minGPUScore,
		RecGPUScore: recGPUScore,
		MinCPUScore: minCPUScore,
		RecCPUScore: recCPUScore,
		Bottleneck:  "gpu/cpu",
		Details:     "ПК проходит по минимальным требованиям: рекомендуются средние или минимальные настройки",
	}
}

// EvaluateHardwareCompatibility evaluates game compatibility against system hardware.
func (a *App) EvaluateHardwareCompatibility(sys SystemHardwareInput, req GameHardwareReq) HardwareCompatibilityVerdict {
	return EvaluateGameCompatibility(sys, req)
}

