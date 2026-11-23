package main

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Hardware class labels for unknown CPU/GPU fallback (GamePulse + Upgrade Advisor).
const (
	ClassIntegrated = "integrated"
	ClassWeak       = "weak"
	ClassBudget     = "budget"
	ClassMid        = "mid"
	ClassHigh       = "high"
)

// scoredHardware is a power score plus whether it came from the named model DB.
type scoredHardware struct {
	Score       float64
	Approximate bool
	Class       string
	MatchedDB   bool
}

// classifyGPUClass maps VRAM + iGPU keywords to a coarse performance class.
// Spec tiers: integrated (no dedicated VRAM / iGPU keywords), budget 2-4 GB,
// mid 6-8 GB, high 12+ GB. Values between tiers round to the nearer band.
func classifyGPUClass(name string, integrated bool, vramGB float64) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if integrated || looksLikeIntegratedGPUName(n) {
		return ClassIntegrated
	}
	if vramGB <= 0 {
		if n == "" || strings.Contains(n, "unknown") {
			return ClassIntegrated
		}
		// Named discrete with unknown VRAM: treat as mid until sensors fill in.
		return ClassMid
	}
	switch {
	case vramGB >= 12:
		return ClassHigh
	case vramGB >= 5.5:
		return ClassMid
	case vramGB >= 2:
		return ClassBudget
	default:
		// <2 GB dedicated: near-integrated discrete / old entry card.
		return ClassWeak
	}
}

func looksLikeIntegratedGPUName(n string) bool {
	if n == "" {
		return false
	}
	markers := []string{
		"uhd", "iris", "hd graphics", "uhd graphics",
		"radeon graphics", "radeon(tm) graphics",
		"vega 3", "vega 5", "vega 6", "vega 7", "vega 8", "vega 11",
		"vega3", "vega5", "vega6", "vega7", "vega8", "vega11",
	}
	for _, m := range markers {
		if strings.Contains(n, m) {
			// Discrete RX Vega / Arc etc. are not iGPU.
			if strings.Contains(n, "rx ") || strings.Contains(n, "geforce") ||
				strings.Contains(n, "rtx") || strings.Contains(n, "gtx") ||
				strings.Contains(n, "quadro") || strings.Contains(n, "arc a") {
				return false
			}
			return true
		}
	}
	return false
}

// classifyCPUClass maps physical cores (+ light frequency hint) to a class.
func classifyCPUClass(cores int, freqMHz float64) string {
	if cores <= 0 {
		cores = 4
	}
	switch {
	case cores >= 12:
		return ClassHigh
	case cores >= 6:
		return ClassMid
	case cores >= 4:
		if freqMHz > 0 && freqMHz < 2200 {
			return ClassWeak
		}
		return ClassBudget
	default:
		return ClassWeak
	}
}

func gpuClassBaseScore(class string) float64 {
	switch class {
	case ClassHigh:
		return 78
	case ClassMid:
		return 58
	case ClassBudget:
		return 38
	case ClassWeak:
		return 24
	default: // integrated
		return 16
	}
}

func cpuClassBaseScore(class string) float64 {
	switch class {
	case ClassHigh:
		return 82
	case ClassMid:
		return 66
	case ClassBudget:
		return 48
	default: // weak
		return 30
	}
}

// matchGPUDatabase returns a named-model score when the GPU is in the ~30-model DB.
func matchGPUDatabase(n string) (score float64, ok bool) {
	switch {
	case strings.Contains(n, "5090"):
		return 100, true
	case strings.Contains(n, "5080"):
		return 96, true
	case strings.Contains(n, "5070"):
		return 88, true
	case strings.Contains(n, "4090"):
		return 100, true
	case strings.Contains(n, "4080"):
		return 94, true
	case strings.Contains(n, "4070 ti"):
		return 88, true
	case strings.Contains(n, "4070"):
		return 85, true
	case strings.Contains(n, "4060 ti"):
		return 72, true
	case strings.Contains(n, "4060"):
		return 65, true
	case strings.Contains(n, "3090"):
		return 90, true
	case strings.Contains(n, "3080"):
		return 84, true
	case strings.Contains(n, "3070"):
		return 76, true
	case strings.Contains(n, "3060 ti"):
		return 68, true
	case strings.Contains(n, "3060"):
		return 60, true
	case strings.Contains(n, "2060 super"):
		return 80, true
	case strings.Contains(n, "2070 super"):
		return 84, true
	case strings.Contains(n, "2070"):
		return 80, true
	case strings.Contains(n, "2080"):
		return 88, true
	case strings.Contains(n, "2060"):
		return 70, true
	case strings.Contains(n, "1660"):
		return 42, true
	case strings.Contains(n, "1650"):
		return 34, true
	case strings.Contains(n, "1050"):
		return 26, true
	case strings.Contains(n, "rx 7900"):
		return 95, true
	case strings.Contains(n, "rx 7800"):
		return 88, true
	case strings.Contains(n, "rx 7700"):
		return 80, true
	case strings.Contains(n, "rx 7600"):
		return 70, true
	case strings.Contains(n, "rx 6800"):
		return 82, true
	case strings.Contains(n, "rx 6700"):
		return 72, true
	case strings.Contains(n, "rx 6600"):
		return 62, true
	case strings.Contains(n, "rx 580"):
		return 38, true
	case strings.Contains(n, "arc a770"):
		return 64, true
	case strings.Contains(n, "arc a750"):
		return 58, true
	case strings.Contains(n, "arc a580"):
		return 48, true
	case strings.Contains(n, "vega 11") || strings.Contains(n, "vega11"):
		return 26, true
	case strings.Contains(n, "vega 8") || strings.Contains(n, "vega8"):
		return 22, true
	case strings.Contains(n, "vega 7") || strings.Contains(n, "vega7"):
		return 20, true
	case strings.Contains(n, "vega 6") || strings.Contains(n, "vega6"):
		return 19, true
	case strings.Contains(n, "vega 5") || strings.Contains(n, "vega5"):
		return 18, true
	case strings.Contains(n, "vega 3") || strings.Contains(n, "vega3"):
		return 12, true
	case strings.Contains(n, "iris xe"):
		return 22, true
	case strings.Contains(n, "iris"):
		return 18, true
	case strings.Contains(n, "uhd"):
		return 12, true
	case strings.Contains(n, "radeon graphics") || strings.Contains(n, "radeon(tm) graphics"):
		return 20, true
	default:
		return 0, false
	}
}

// matchCPUDatabase returns a named-model base when the CPU is in the model DB.
func matchCPUDatabase(n string) (base float64, ok bool) {
	switch {
	case strings.Contains(n, "ryzen ai 9") || strings.Contains(n, "ultra 9"):
		return 92, true
	case strings.Contains(n, "ultra 7") || strings.Contains(n, "ryzen ai 7"):
		return 84, true
	case strings.Contains(n, "ultra 5") || strings.Contains(n, "ryzen ai 5"):
		return 74, true
	case strings.Contains(n, "i9-14") || strings.Contains(n, "i9-13") || strings.Contains(n, "ryzen 9 7") || strings.Contains(n, "ryzen 9 9"):
		return 90, true
	case strings.Contains(n, "i7-14") || strings.Contains(n, "i7-13") || strings.Contains(n, "ryzen 7 7") || strings.Contains(n, "ryzen 7 9"):
		return 82, true
	case strings.Contains(n, "i5-14") || strings.Contains(n, "i5-13") || strings.Contains(n, "ryzen 5 7") || strings.Contains(n, "ryzen 5 9"):
		return 72, true
	case strings.Contains(n, "i9") || strings.Contains(n, "ryzen 9"):
		return 85, true
	case strings.Contains(n, "i7") || strings.Contains(n, "ryzen 7"):
		return 75, true
	case strings.Contains(n, "i5") || strings.Contains(n, "ryzen 5"):
		return 65, true
	case strings.Contains(n, "4300u") || strings.Contains(n, "4500u") || strings.Contains(n, "5500u"):
		return 42, true
	case strings.Contains(n, "i3") || strings.Contains(n, "ryzen 3"):
		return 45, true
	case strings.Contains(n, "celeron") || strings.Contains(n, "pentium") || strings.Contains(n, "athlon"):
		return 28, true
	default:
		return 0, false
	}
}

// chipNumberGPUHint is a weak heuristic for unmatched RTX/GTX/RX names.
// Always treated as approximate (not an exact DB hit).
func chipNumberGPUHint(n string) (score float64, ok bool) {
	m := regexp.MustCompile(`(?:rtx|gtx|rx)\s*([0-9]{3,4})`).FindStringSubmatch(n)
	if len(m) != 2 {
		return 0, false
	}
	num, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	switch {
	case num >= 4000:
		return 80, true
	case num >= 3000:
		return 70, true
	case num >= 2000:
		return 55, true
	case num >= 1600:
		return 40, true
	case num >= 1000:
		return 30, true
	case num >= 600:
		return 65, true
	case num >= 500:
		return 50, true
	default:
		return 35, true
	}
}

// scoreGPU returns GamePulse GPU power with DB match or VRAM-class fallback.
func scoreGPU(name string, integrated bool, vramGB float64) scoredHardware {
	n := strings.ToLower(strings.TrimSpace(name))
	class := classifyGPUClass(name, integrated, vramGB)
	approx := false
	matched := false
	score := 40.0

	if db, ok := matchGPUDatabase(n); ok {
		score = db
		matched = true
	} else if n == "" || strings.Contains(n, "unknown") {
		score = gpuClassBaseScore(class)
		approx = true
	} else if hint, ok := chipNumberGPUHint(n); ok {
		// Named chip family but not in exact DB: blend chip hint with VRAM class.
		base := gpuClassBaseScore(class)
		score = (hint*0.55 + base*0.45)
		approx = true
	} else {
		score = gpuClassBaseScore(class)
		approx = true
	}

	if integrated || class == ClassIntegrated {
		if score > 35 {
			score = 28
		}
		score *= 0.95
		if !matched {
			approx = true
			class = ClassIntegrated
		}
	} else {
		switch {
		case vramGB >= 12:
			score *= 1.04
		case vramGB >= 8:
			score *= 1.02
		case vramGB > 0 && vramGB < 4:
			score *= 0.92
		case vramGB > 0 && vramGB < 2:
			score *= 0.85
		}
	}

	if strings.Contains(n, "laptop") || strings.Contains(n, "mobile") ||
		strings.Contains(n, "max-q") || strings.Contains(n, "max-p") {
		score *= 0.90
	}

	out := clampScore(score)
	if math.IsNaN(out) || out < 0 {
		out = 5
	}
	return scoredHardware{Score: out, Approximate: approx, Class: class, MatchedDB: matched}
}

// scoreCPU returns GamePulse CPU power with DB match or core/freq fallback.
func scoreCPU(name string, cores, threads int, freqMHz float64) scoredHardware {
	n := strings.ToLower(strings.TrimSpace(name))
	if cores <= 0 {
		cores = 4
	}
	if threads <= 0 {
		threads = cores
	}
	class := classifyCPUClass(cores, freqMHz)
	approx := false
	matched := false
	base := 55.0

	if db, ok := matchCPUDatabase(n); ok {
		base = db
		matched = true
	} else {
		base = cpuClassBaseScore(class)
		approx = true
		// Empty / unknown names always approximate.
		if n == "" || strings.Contains(n, "unknown") {
			approx = true
		}
	}

	coreFactor := float64(cores) / 6.0
	if coreFactor > 1.35 {
		coreFactor = 1.35
	}
	if coreFactor < 0.45 {
		coreFactor = 0.45
	}

	threadBonus := 1.0 + math.Min(0.12, float64(threads-cores)*0.02)

	freqFactor := 1.0
	if freqMHz > 0 {
		freqFactor = freqMHz / 3200.0
		if freqFactor > 1.25 {
			freqFactor = 1.25
		}
		if freqFactor < 0.7 {
			freqFactor = 0.7
		}
	}

	// Low-power U-series / thin-laptop penalty.
	if regexp.MustCompile(`(?i)(\d{4}u\b|ryzen\s+[3579]\s+\d{4}u)`).MatchString(n) ||
		(strings.Contains(n, "u") && (strings.Contains(n, "ryzen") || strings.Contains(n, "core"))) {
		base *= 0.88
	}

	score := base * (0.55 + 0.45*coreFactor) * threadBonus * freqFactor
	out := clampScore(score)
	if math.IsNaN(out) || out < 0 {
		out = 5
	}
	return scoredHardware{Score: out, Approximate: approx, Class: class, MatchedDB: matched}
}

// calcGPUPowerScore keeps the historic API; scoring details live in scoreGPU.
func calcGPUPowerScore(name string, integrated bool, vramGB float64) float64 {
	return scoreGPU(name, integrated, vramGB).Score
}

// calcCPUPowerScore keeps the historic API; scoring details live in scoreCPU.
func calcCPUPowerScore(name string, cores, threads int, freqMHz float64) float64 {
	return scoreCPU(name, cores, threads, freqMHz).Score
}
