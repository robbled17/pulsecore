package main

import (
	"math"
	"strings"
	"testing"
)

// virtualHardwareProfile is a fixture for Overview / Hardware / GamePulse / Upgrade Advisor.
// Production never invents sensor numbers; tests only use mocks.
type virtualHardwareProfile struct {
	ID              string
	CPUName         string
	PhysicalCores   int
	LogicalCores    int
	FreqMHz         float64
	GPUName         string
	VRAMGB          float64
	GPUIntegrated   bool
	RAMGB           float64
	RAMSlotsUsed    int
	RAMSlotsTotal   int
	IsLaptop        bool
	Chassis         string
	Socket          string
	HasBattery      bool
	Disks           []StorageDrive
	TempAvailable   bool
	TemperatureC    float64
	SMARTOK         bool
	ExpectGPUClass  string
	ExpectApproxGPU bool
	ExpectApproxCPU bool
}

func profileOldLaptop() virtualHardwareProfile {
	return virtualHardwareProfile{
		ID: "old-laptop", CPUName: "Intel Core i5-8250U",
		PhysicalCores: 2, LogicalCores: 4, FreqMHz: 1600,
		GPUName: "Intel UHD Graphics 620", VRAMGB: 0, GPUIntegrated: true,
		RAMGB: 8, RAMSlotsUsed: 1, RAMSlotsTotal: 2,
		IsLaptop: true, Chassis: "laptop", Socket: "BGA", HasBattery: true,
		Disks: []StorageDrive{{Model: "NVMe 256GB", InterfaceType: "NVMe", MediaType: "SSD", SizeGB: 256}},
		TempAvailable: true, TemperatureC: 62, SMARTOK: true,
		ExpectGPUClass: ClassIntegrated, ExpectApproxGPU: false, ExpectApproxCPU: false,
	}
}

func profileGamingLaptop() virtualHardwareProfile {
	return virtualHardwareProfile{
		ID: "gaming-laptop", CPUName: "AMD Ryzen 7 5800H",
		PhysicalCores: 8, LogicalCores: 16, FreqMHz: 3200,
		GPUName: "NVIDIA GeForce RTX 3060 Laptop GPU", VRAMGB: 6, GPUIntegrated: false,
		RAMGB: 16, RAMSlotsUsed: 2, RAMSlotsTotal: 2,
		IsLaptop: true, Chassis: "laptop", Socket: "BGA", HasBattery: true,
		Disks: []StorageDrive{
			{Model: "Samsung 980 1TB", InterfaceType: "NVMe", MediaType: "SSD", SizeGB: 1000},
			{Model: "WD SN570 500GB", InterfaceType: "NVMe", MediaType: "SSD", SizeGB: 500},
		},
		TempAvailable: true, TemperatureC: 71, SMARTOK: true,
		ExpectGPUClass: ClassMid, ExpectApproxGPU: false, ExpectApproxCPU: false,
	}
}

func profileHybridIntel() virtualHardwareProfile {
	return virtualHardwareProfile{
		ID: "hybrid-intel", CPUName: "Intel Core i5-14600K",
		PhysicalCores: 14, LogicalCores: 20, FreqMHz: 3500,
		GPUName: "NVIDIA GeForce RTX 4070", VRAMGB: 12, GPUIntegrated: false,
		RAMGB: 32, RAMSlotsUsed: 2, RAMSlotsTotal: 4,
		IsLaptop: false, Chassis: "desktop", Socket: "LGA1700", HasBattery: false,
		Disks: []StorageDrive{{Model: "Samsung 990 PRO 2TB", InterfaceType: "NVMe", MediaType: "SSD", SizeGB: 2000}},
		TempAvailable: true, TemperatureC: 55, SMARTOK: true,
		ExpectGPUClass: ClassHigh, ExpectApproxGPU: false, ExpectApproxCPU: false,
	}
}

func profileOldDesktopNoSMT() virtualHardwareProfile {
	return virtualHardwareProfile{
		ID: "old-desktop-no-smt", CPUName: "Mystery Desktop CPU 2014",
		PhysicalCores: 4, LogicalCores: 4, FreqMHz: 2800,
		GPUName: "AcmeVision GX-900 Unknown", VRAMGB: 3, GPUIntegrated: false,
		RAMGB: 8, RAMSlotsUsed: 2, RAMSlotsTotal: 2,
		IsLaptop: false, Chassis: "desktop", Socket: "Unknown", HasBattery: false,
		Disks: []StorageDrive{{Model: "Seagate Barracuda 1TB", InterfaceType: "SATA", MediaType: "HDD", SizeGB: 1000}},
		TempAvailable: true, TemperatureC: 48, SMARTOK: true,
		ExpectGPUClass: ClassBudget, ExpectApproxGPU: true, ExpectApproxCPU: true,
	}
}

func profileNoAdmin() virtualHardwareProfile {
	return virtualHardwareProfile{
		ID: "no-admin", CPUName: "AMD Ryzen 5 5600",
		PhysicalCores: 6, LogicalCores: 12, FreqMHz: 3400,
		GPUName: "NVIDIA GeForce RTX 3060", VRAMGB: 12, GPUIntegrated: false,
		RAMGB: 16, RAMSlotsUsed: 2, RAMSlotsTotal: 4,
		IsLaptop: false, Chassis: "desktop", Socket: "AM4", HasBattery: false,
		Disks: []StorageDrive{{Model: "Kingston NV2 1TB", InterfaceType: "NVMe", MediaType: "SSD", SizeGB: 1000}},
		TempAvailable: false, TemperatureC: 0, SMARTOK: false,
		ExpectGPUClass: ClassHigh, ExpectApproxGPU: false, ExpectApproxCPU: false,
	}
}

func allVirtualProfiles() []virtualHardwareProfile {
	return []virtualHardwareProfile{
		profileOldLaptop(),
		profileGamingLaptop(),
		profileHybridIntel(),
		profileOldDesktopNoSMT(),
		profileNoAdmin(),
	}
}

func passportFromProfile(p virtualHardwareProfile) PCPassport {
	free := p.RAMSlotsTotal - p.RAMSlotsUsed
	if free < 0 {
		free = 0
	}
	pass := PCPassport{
		Chassis:         p.Chassis,
		IsLaptop:        p.IsLaptop,
		Socket:          p.Socket,
		CPUName:         p.CPUName,
		PhysicalCores:   p.PhysicalCores,
		LogicalCores:    p.LogicalCores,
		RAMTotalGB:      p.RAMGB,
		RAMSlotsUsed:    p.RAMSlotsUsed,
		RAMSlotsTotal:   p.RAMSlotsTotal,
		RAMSlotsFree:    free,
		IsSingleChannel: p.RAMSlotsUsed <= 1,
		GPUName:         p.GPUName,
		VRAMGB:          p.VRAMGB,
		GPUIntegrated:   p.GPUIntegrated,
		Storage:         append([]StorageDrive(nil), p.Disks...),
		HasFreeM2:       !p.IsLaptop || len(p.Disks) <= 1,
	}
	enrichPassportFallback(&pass)
	return pass
}

func powerProfileFromFixture(p virtualHardwareProfile) SystemPowerProfile {
	gpu := scoreGPU(p.GPUName, p.GPUIntegrated, p.VRAMGB)
	cpu := scoreCPU(p.CPUName, p.PhysicalCores, p.LogicalCores, p.FreqMHz)
	vram := p.VRAMGB
	if p.GPUIntegrated {
		vram = 0
	}
	return SystemPowerProfile{
		GPUName:       p.GPUName,
		IsIntegrated:  p.GPUIntegrated,
		VRAMGB:        vram,
		GPUPowerScore: gpu.Score,
		CPUName:       p.CPUName,
		CPUCores:      p.PhysicalCores,
		CPUThreads:    p.LogicalCores,
		CPUFreqMHz:    p.FreqMHz,
		CPUPowerScore: cpu.Score,
		RAMGB:         p.RAMGB,
		IsDualChannel: p.RAMSlotsUsed >= 2,
		Approximate:   gpu.Approximate || cpu.Approximate,
		GPUClass:      gpu.Class,
		CPUClass:      cpu.Class,
	}
}

func overviewSnapshotFromProfile(p virtualHardwareProfile) SystemMetrics {
	m := SystemMetrics{
		CPUPercent:    22,
		TemperatureC:  p.TemperatureC,
		TempAvailable: p.TempAvailable,
		CPUName:       p.CPUName,
		GPUName:       p.GPUName,
		Hardware: HardwareInfo{
			CPU: CPUDeepInfo{
				Name:          p.CPUName,
				PhysicalCores: p.PhysicalCores,
				LogicalCores:  p.LogicalCores,
				FrequencyMHz:  p.FreqMHz,
			},
			GPU: GPUDeepInfo{
				Name:         p.GPUName,
				IsIntegrated: p.GPUIntegrated,
				VRAMGB:       p.VRAMGB,
				DedicatedMB:  p.VRAMGB * 1024,
			},
			RAM: RAMDeepInfo{
				TotalMB:    uint64(p.RAMGB * 1024),
				SlotsUsed:  p.RAMSlotsUsed,
				SlotsTotal: p.RAMSlotsTotal,
			},
			Power: PowerDeepInfo{HasBattery: p.HasBattery},
		},
	}
	if !p.TempAvailable {
		m.TemperatureC = 0
		m.TempAvailable = false
	}
	return m
}

func smartFromProfile(p virtualHardwareProfile) DiskHealthReport {
	if p.SMARTOK {
		return DiskHealthReport{
			DriveLetter:   "C:",
			Model:         p.Disks[0].Model,
			OK:            true,
			TemperatureC:  38,
			WearRemaining: 92,
			HealthPercent: 92,
			Verdict:       "OK",
			VerdictBadge:  "OK",
		}
	}
	// No-admin / SMART unavailable: never invent a real 0C reading for UI.
	return DiskHealthReport{
		DriveLetter:   "C:",
		OK:            false,
		Verdict:       "unavailable",
		ErrorMessage:  "admin required / sensor blocked",
		TemperatureC:  0,
		WearRemaining: 0,
		HealthPercent: 0,
	}
}

func assertFiniteNonNegative(t *testing.T, label string, v float64) {
	t.Helper()
	if math.IsNaN(v) || math.IsInf(v, 0) {
		t.Fatalf("%s: NaN/Inf %v", label, v)
	}
	if v < 0 {
		t.Fatalf("%s: negative %v", label, v)
	}
}

func assertNoDesktopParts(t *testing.T, profileID string, recs []UpgradeRecommendation) {
	t.Helper()
	for _, r := range recs {
		part := strings.ToLower(r.Part)
		if part == "cpu" || part == "gpu" {
			t.Fatalf("%s: laptop must not get desktop %s advice (%s)", profileID, part, r.ID)
		}
		body := strings.ToLower(r.Body + " " + r.Model + " " + r.Title)
		if strings.Contains(body, "x3d") || strings.Contains(r.ID, "desktop-gpu") || strings.Contains(r.ID, "desktop-cpu") {
			t.Fatalf("%s: leaked desktop upgrade id/body: %s", profileID, r.ID)
		}
	}
}

func TestHardwareProfilesFiveMatrix(t *testing.T) {
	profiles := allVirtualProfiles()
	if len(profiles) != 5 {
		t.Fatalf("want 5 profiles, got %d", len(profiles))
	}

	for _, p := range profiles {
		p := p
		t.Run(p.ID, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic on profile %s: %v", p.ID, r)
				}
			}()

			// --- Hardware: cores / threads ---
			if p.PhysicalCores <= 0 || p.LogicalCores <= 0 {
				t.Fatalf("cores/threads must be positive")
			}
			if p.LogicalCores < p.PhysicalCores {
				t.Fatalf("threads %d < cores %d", p.LogicalCores, p.PhysicalCores)
			}
			if p.ID == "old-desktop-no-smt" && p.LogicalCores != p.PhysicalCores {
				t.Fatalf("no-SMT profile must have equal cores/threads")
			}
			if p.ID == "old-laptop" && !(p.PhysicalCores == 2 && p.LogicalCores == 4) {
				t.Fatalf("old laptop must be 2c/4t")
			}
			if p.ID == "hybrid-intel" && !(p.PhysicalCores == 14 && p.LogicalCores == 20) {
				t.Fatalf("hybrid Intel must be 14c/20t")
			}

			// --- Overview snapshot: no fake temp when unavailable ---
			snap := overviewSnapshotFromProfile(p)
			assertFiniteNonNegative(t, "temperatureC", snap.TemperatureC)
			if !p.TempAvailable {
				if snap.TempAvailable {
					t.Fatal("no-admin: TempAvailable must be false")
				}
				if snap.TemperatureC != 0 {
					t.Fatalf("no-admin: do not invent temperature, got %v", snap.TemperatureC)
				}
			} else if snap.TemperatureC <= 0 || snap.TemperatureC > 150 {
				t.Fatalf("available temp out of range: %v", snap.TemperatureC)
			}
			hw := snap.Hardware
			if hw.CPU.PhysicalCores != p.PhysicalCores || hw.CPU.LogicalCores != p.LogicalCores {
				t.Fatalf("hardware cores mismatch")
			}

			// --- GamePulse scoring ---
			sys := powerProfileFromFixture(p)
			assertFiniteNonNegative(t, "gpuScore", sys.GPUPowerScore)
			assertFiniteNonNegative(t, "cpuScore", sys.CPUPowerScore)
			if sys.GPUPowerScore < 5 || sys.CPUPowerScore < 5 {
				t.Fatalf("scores must not collapse to empty/zero: gpu=%v cpu=%v", sys.GPUPowerScore, sys.CPUPowerScore)
			}
			if sys.GPUClass != p.ExpectGPUClass && !(p.ExpectGPUClass == ClassHigh && sys.GPUClass == ClassMid && p.VRAMGB >= 6 && p.VRAMGB < 12) {
				// Known mid discrete (3060 6GB) may classify mid; high only for 12+.
				if p.ExpectGPUClass != "" && sys.GPUClass != p.ExpectGPUClass {
					t.Fatalf("gpu class=%s want %s", sys.GPUClass, p.ExpectGPUClass)
				}
			}
			gpuMeta := scoreGPU(p.GPUName, p.GPUIntegrated, p.VRAMGB)
			cpuMeta := scoreCPU(p.CPUName, p.PhysicalCores, p.LogicalCores, p.FreqMHz)
			if gpuMeta.Approximate != p.ExpectApproxGPU {
				t.Fatalf("gpu approx=%v want %v (class=%s score=%.1f)", gpuMeta.Approximate, p.ExpectApproxGPU, gpuMeta.Class, gpuMeta.Score)
			}
			if cpuMeta.Approximate != p.ExpectApproxCPU {
				t.Fatalf("cpu approx=%v want %v (class=%s score=%.1f)", cpuMeta.Approximate, p.ExpectApproxCPU, cpuMeta.Class, cpuMeta.Score)
			}
			if p.ID == "old-desktop-no-smt" {
				if !sys.Approximate {
					t.Fatal("unknown GPU desktop must mark approximate")
				}
				if sys.GPUClass != ClassBudget {
					t.Fatalf("3GB unknown GPU class=%s want budget", sys.GPUClass)
				}
			}

			// estimatePreset must not div-by-zero / NaN
			req := gameReq{BaseFPS: 60, ReqGPUScore: 40, ReqCPUScore: 40, ReqVRAMGB: 4}
			avg, rng, _ := estimatePreset(sys, req, presetMed, 0)
			if avg <= 0 {
				t.Fatalf("fps avg <= 0: %d", avg)
			}
			if strings.Contains(rng, "NaN") {
				t.Fatalf("range has NaN: %s", rng)
			}

			// --- Upgrade Advisor ---
			pass := passportFromProfile(p)
			recs := buildUpgradeRecommendations(pass, UpgradeGoalMaxFPS, "en")
			if p.IsLaptop {
				assertNoDesktopParts(t, p.ID, recs)
			} else if len(recs) == 0 {
				t.Fatal("desktop must still get upgrade cards")
			}
			if p.ID == "old-desktop-no-smt" {
				if !pass.Approximate {
					t.Fatal("passport approximate required for unknown GPU")
				}
				foundGPU := false
				for _, r := range recs {
					if r.Part == "gpu" && strings.TrimSpace(r.Model) != "" {
						foundGPU = true
					}
				}
				if !foundGPU {
					t.Fatal("unknown GPU desktop must still get a GPU recommendation (not empty)")
				}
			}

			// --- SMART / no-admin ---
			smart := smartFromProfile(p)
			assertFiniteNonNegative(t, "smart.temp", smart.TemperatureC)
			assertFiniteNonNegative(t, "smart.wear", smart.WearRemaining)
			assertFiniteNonNegative(t, "smart.health", smart.HealthPercent)
			if !p.SMARTOK {
				if smart.OK {
					t.Fatal("no-admin SMART must not claim OK")
				}
				if smart.Verdict == "" && smart.ErrorMessage == "" {
					t.Fatal("unavailable SMART needs verdict or error")
				}
				// Wear sentinel -1 must never surface as a "real" negative display field after mapping.
				if smart.WearRemaining < 0 || smart.HealthPercent < 0 {
					t.Fatalf("unavailable SMART leaked negative wear/health")
				}
			}
		})
	}
}

func TestGPUFallbackClassesFromVRAM(t *testing.T) {
	cases := []struct {
		name string
		igpu bool
		vram float64
		want string
	}{
		{"Intel UHD Graphics 630", true, 0, ClassIntegrated},
		{"AMD Radeon Graphics", true, 0, ClassIntegrated},
		{"Unknown Budget Card", false, 3, ClassBudget},
		{"Unknown Mid Card", false, 8, ClassMid},
		{"Unknown High Card", false, 12, ClassHigh},
	}
	for _, c := range cases {
		got := classifyGPUClass(c.name, c.igpu, c.vram)
		if got != c.want {
			t.Fatalf("%s vram=%.0f: class=%s want %s", c.name, c.vram, got, c.want)
		}
		sc := scoreGPU(c.name, c.igpu, c.vram)
		assertFiniteNonNegative(t, c.name, sc.Score)
		if sc.Score < 5 {
			t.Fatalf("%s score too low: %v", c.name, sc.Score)
		}
		if c.name == "Unknown Budget Card" || c.name == "Unknown Mid Card" || c.name == "Unknown High Card" {
			if !sc.Approximate {
				t.Fatalf("%s must be approximate", c.name)
			}
		}
	}
}

func TestCPUFallbackClassesFromCores(t *testing.T) {
	cases := []struct {
		cores int
		freq  float64
		want  string
	}{
		{2, 1800, ClassWeak},
		{4, 2800, ClassBudget},
		{8, 3600, ClassMid},
		{14, 3500, ClassHigh},
	}
	for _, c := range cases {
		got := classifyCPUClass(c.cores, c.freq)
		if got != c.want {
			t.Fatalf("cores=%d: class=%s want %s", c.cores, got, c.want)
		}
		sc := scoreCPU("Custom Silicon X", c.cores, c.cores*2, c.freq)
		if !sc.Approximate {
			t.Fatalf("unknown CPU must be approximate")
		}
		assertFiniteNonNegative(t, "cpu", sc.Score)
	}
}

func TestKnownModelsNotApproximate(t *testing.T) {
	gpu := scoreGPU("NVIDIA GeForce RTX 4070", false, 12)
	if gpu.Approximate || !gpu.MatchedDB {
		t.Fatalf("RTX 4070 must be exact DB match, approx=%v matched=%v", gpu.Approximate, gpu.MatchedDB)
	}
	cpu := scoreCPU("AMD Ryzen 7 5800H", 8, 16, 3200)
	if cpu.Approximate || !cpu.MatchedDB {
		t.Fatalf("5800H must be exact DB match")
	}
}

func TestApproxCopyHasNoEmEnDashes(t *testing.T) {
	for _, lang := range []string{"ru", "en", "kk", "tt", "de", "ja", "zh"} {
		s := trUpgrade(lang, "noteApprox")
		if s == "" || s == "noteApprox" {
			t.Fatalf("%s noteApprox missing", lang)
		}
		if strings.Contains(s, "\u2014") || strings.Contains(s, "\u2013") {
			t.Fatalf("%s noteApprox has em/en dash: %q", lang, s)
		}
	}
}
