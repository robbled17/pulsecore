package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSimulatedMetricsProvider(t *testing.T) {
	sim := NewSimulatedMetricsProvider()
	if sim == nil {
		t.Fatal("expected non-nil SimulatedMetricsProvider")
	}

	// 1. Initial default state
	cpuPct, perCore := sim.GetCPULoad()
	if cpuPct <= 0 || len(perCore) == 0 {
		t.Fatalf("expected positive cpuPct and non-empty perCore, got %.1f, len=%d", cpuPct, len(perCore))
	}
	temp, tempOK, src, _, _ := sim.GetCPUTemp(cpuPct)
	if !tempOK || temp <= 0 || src != "simulated" {
		t.Fatalf("expected simulated temp, got temp=%.1f, ok=%v, src=%s", temp, tempOK, src)
	}

	vm, sm := sim.GetRAM()
	if vm == nil || vm.Total == 0 || sm == nil {
		t.Fatal("expected valid virtual and swap memory from simulator")
	}

	gpuPct, _, _, vramTotal, gpuTemp, gpuTempOK, gpuName := sim.GetGPU("overview")
	if gpuPct < 0 || vramTotal <= 0 || !gpuTempOK || gpuTemp <= 0 || gpuName == "" {
		t.Fatalf("expected valid simulated GPU stats, got gpuPct=%.1f, vramTotal=%.1f, temp=%.1f, name=%s", gpuPct, vramTotal, gpuTemp, gpuName)
	}

	uptime := sim.GetUptime()
	if uptime <= 0 {
		t.Fatalf("expected positive uptime, got %v", uptime)
	}
}

func TestCustomDisksListAndWDBlue(t *testing.T) {
	app := NewApp()
	app.SetDevMode(true)

	// Configure a custom disk layout including WDC WD10EZEX (WD Blue HDD)
	disks := []SimulatedDisk{
		{
			Letter:    "C",
			Type:      "NVMe SSD",
			Model:     "Samsung SSD 980 PRO 1TB",
			TotalGB:   1024,
			UsedPct:   40.0,
			HealthPct: 100.0,
			TempC:     35.0,
		},
		{
			Letter:    "D",
			Type:      "HDD",
			Model:     "WDC WD10EZEX",
			TotalGB:   1000,
			UsedPct:   60.0,
			HealthPct: 98.0,
			TempC:     33.0,
		},
		{
			Letter:    "E",
			Type:      "SATA SSD",
			Model:     "Crucial MX500 500GB",
			TotalGB:   500,
			UsedPct:   25.0,
			HealthPct: 92.0,
			TempC:     30.0,
		},
	}
	state := app.SetSimulatedDisks(disks)
	if len(state.Disks) != 3 {
		t.Fatalf("expected 3 disks, got %d", len(state.Disks))
	}

	// Verify LocalDrives reflects all 3 drives
	localDrives := app.GetLocalDrives()
	if len(localDrives) != 3 {
		t.Fatalf("expected 3 local drives, got %d", len(localDrives))
	}

	// Verify WDC WD10EZEX (WD Blue) is strictly identified as HDD
	reportD := app.CheckDiskHealth("D")
	if !reportD.OK {
		t.Fatal("expected reportD to be OK")
	}
	if reportD.MediaType != "HDD" {
		t.Fatalf("expected MediaType HDD, got %s", reportD.MediaType)
	}
	if reportD.MediaKind != "hdd" {
		t.Fatalf("expected MediaKind hdd, got %s", reportD.MediaKind)
	}
	if reportD.RotationRPM != 7200 {
		t.Fatalf("expected 7200 RPM for HDD, got %.0f", reportD.RotationRPM)
	}
	if reportD.BusInterface != "SATA" {
		t.Fatalf("expected SATA bus, got %s", reportD.BusInterface)
	}

	// Verify NVMe SSD
	reportC := app.CheckDiskHealth("C")
	if reportC.MediaType != "SSD" || reportC.MediaKind != "nvme" || reportC.BusInterface != "NVMe" {
		t.Fatalf("expected C: to be NVMe SSD, got type=%s, kind=%s, bus=%s", reportC.MediaType, reportC.MediaKind, reportC.BusInterface)
	}

	// Verify SATA SSD
	reportE := app.CheckDiskHealth("E")
	if reportE.MediaType != "SSD" || reportE.MediaKind != "sata_ssd" || reportE.BusInterface != "SATA" {
		t.Fatalf("expected E: to be SATA SSD, got type=%s, kind=%s, bus=%s", reportE.MediaType, reportE.MediaKind, reportE.BusInterface)
	}
}

func TestGamePulseCompatibilityOnSimulatedHardware(t *testing.T) {
	app := NewApp()
	app.SetDevMode(true)

	// 1. Weak Entry Level Configuration (Celeron + UHD 630 iGPU + 4 GB RAM)
	app.SetSimulatedHardware(
		"Intel Celeron G5905", 2, 2, 3500, 24,
		"Intel UHD Graphics 630 (1 ГБ)", 1.0, true, 12,
		4,
	)

	profileWeak := app.GetSystemPowerProfile()
	if profileWeak.CPUPowerScore > 35 {
		t.Fatalf("expected low CPU score for Celeron, got %.1f", profileWeak.CPUPowerScore)
	}
	if profileWeak.GPUPowerScore > 20 {
		t.Fatalf("expected low GPU score for UHD 630, got %.1f", profileWeak.GPUPowerScore)
	}
	if !profileWeak.IsIntegrated {
		t.Fatal("expected UHD 630 to be marked as integrated")
	}

	// Predict FPS on a heavy game (Cyberpunk 2077)
	predWeak := app.PredictGameFPS("cyberpunk")
	if predWeak.UltraAvg > 20 {
		t.Fatalf("expected very low FPS on weak hardware, got %d", predWeak.UltraAvg)
	}

	// 2. High-End Top Configuration (Ryzen 7 7800X3D + RTX 4080 Super + 32 GB RAM)
	app.SetSimulatedHardware(
		"AMD Ryzen 7 7800X3D", 8, 16, 4200, 96,
		"NVIDIA GeForce RTX 4080 Super", 16.0, false, 95,
		32,
	)

	profileStrong := app.GetSystemPowerProfile()
	if profileStrong.CPUPowerScore < 90 {
		t.Fatalf("expected high CPU score for 7800X3D, got %.1f", profileStrong.CPUPowerScore)
	}
	if profileStrong.GPUPowerScore < 90 {
		t.Fatalf("expected high GPU score for 4080 Super, got %.1f", profileStrong.GPUPowerScore)
	}
	if profileStrong.IsIntegrated {
		t.Fatal("expected RTX 4080 Super to be discrete")
	}

	predStrong := app.PredictGameFPS("cyberpunk")
	if predStrong.UltraAvg < 40 {
		t.Fatalf("expected high FPS on top hardware, got %d", predStrong.UltraAvg)
	}

	// Verify that the rating significantly improved between weak and strong configs
	if predStrong.UltraAvg <= predWeak.UltraAvg*2 {
		t.Fatalf("expected strong FPS (%d) to be at least double weak FPS (%d)", predStrong.UltraAvg, predWeak.UltraAvg)
	}
}

func TestLaptopBatteryEmulation(t *testing.T) {
	app := NewApp()
	app.SetDevMode(true)

	// 1. Desktop Mode (No battery)
	app.SetSimulatedBattery(false, 0, true, 0)
	metricsDesktop := app.GetMetrics()
	if metricsDesktop.Battery.Available {
		t.Fatal("desktop must not have an available battery")
	}

	// 2. Laptop Mode: Battery discharging with 25% wear
	app.SetSimulatedBattery(true, 42.0, false, 25.0)
	metricsLaptop := app.GetMetrics()
	if !metricsLaptop.Battery.Available {
		t.Fatal("laptop must have an available battery")
	}
	if metricsLaptop.Battery.Percent != 42.0 {
		t.Fatalf("expected 42%% battery, got %.1f", metricsLaptop.Battery.Percent)
	}
	if metricsLaptop.Battery.PluggedIn {
		t.Fatal("expected discharging battery")
	}
	if metricsLaptop.Battery.WearPercent != 25.0 {
		t.Fatalf("expected 25%% wear, got %.1f", metricsLaptop.Battery.WearPercent)
	}
	if !strings.Contains(metricsLaptop.Battery.StatusText, "батареи") {
		t.Fatalf("expected discharging text, got %s", metricsLaptop.Battery.StatusText)
	}

	// 3. Laptop Mode: AC Charging
	app.SetSimulatedBattery(true, 95.0, true, 5.0)
	metricsCharging := app.GetMetrics()
	if !metricsCharging.Battery.PluggedIn {
		t.Fatal("expected charging battery")
	}
	if !strings.Contains(metricsCharging.Battery.StatusText, "сети") {
		t.Fatalf("expected plugged in text, got %s", metricsCharging.Battery.StatusText)
	}
}

func TestDryRunMode(t *testing.T) {
	app := NewApp()
	app.SetDevMode(true)
	app.SetDryRun(true)

	if !app.IsDryRun() {
		t.Fatal("expected Dry-Run to be active")
	}

	// Test 1: Disk Cleanup does NOT delete files
	tmpDir := os.TempDir()
	testFile := filepath.Join(tmpDir, "pulsecore_dryrun_test.tmp")
	_ = os.WriteFile(testFile, []byte("dry-run safety verification"), 0644)
	defer os.Remove(testFile)

	cleanRes := app.ApplyDiskCleanup([]string{"shader_cache", "user_temp"})
	if !cleanRes.DryRun {
		t.Fatal("expected cleanRes.DryRun == true")
	}
	if !strings.Contains(cleanRes.DryRunMessageRu, "[DRY-RUN]") {
		t.Fatalf("expected [DRY-RUN] in message, got %s", cleanRes.DryRunMessageRu)
	}
	if cleanRes.ActualCleanedBytes != 0 {
		t.Fatalf("actual cleaned bytes in dry-run must be 0, got %d", cleanRes.ActualCleanedBytes)
	}
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatal("dry-run must not delete files from filesystem")
	}

	// Test 2: Process Freeze does not affect process
	err := app.FreezeProcess(99999)
	if err != nil {
		t.Fatalf("dry-run FreezeProcess should return nil error, got %v", err)
	}

	// Test 3: Eco feedback in Dry-Run
	ecoRes := app.ApplyEcoPIDWithFeedback(99999)
	if !ecoRes.DryRun || !strings.Contains(ecoRes.MessageRu, "[DRY-RUN]") {
		t.Fatalf("expected DryRun eco feedback, got %+v", ecoRes)
	}

	// Test 4: Unlocker Dry-Run feedback
	unlockRes := app.UnlockFileDryRun(`C:\Games\Example\locked.dat`)
	if !unlockRes.DryRun || !strings.Contains(unlockRes.MessageRu, "[DRY-RUN]") {
		t.Fatalf("expected DryRun unlock feedback, got %+v", unlockRes)
	}
}

func TestDiagnosticReportIsolation(t *testing.T) {
	app := NewApp()

	// Capture real hardware names before turning on Dev Mode
	realHW := collectStaticHardware()
	realCPU := realHW.cpuName
	if realCPU == "" {
		realCPU = "Unknown CPU"
	}

	// Turn on Dev Mode with crazy fake values
	app.SetDevMode(true)
	app.SetSimulatedHardware(
		"Fake Ultra Alien Quantum Core i99-99999X", 128, 256, 9999, 100,
		"Fake Cyberpunk Monster RTX 9090 Super Ti", 128.0, false, 100,
		1024,
	)

	// Build Diagnostic Report
	report := app.BuildDiagnosticReport()

	// Verify report did NOT leak the simulated CPU or GPU name!
	if strings.Contains(report.CPUName, "Fake Ultra Alien") {
		t.Fatalf("DiagnosticReport leaked fake CPU name: %s", report.CPUName)
	}
	if strings.Contains(report.GPUName, "Fake Cyberpunk") {
		t.Fatalf("DiagnosticReport leaked fake GPU name: %s", report.GPUName)
	}
	if report.RAMTotalMB > 500*1024 {
		t.Fatalf("DiagnosticReport leaked fake 1024 GB RAM: %d MB", report.RAMTotalMB)
	}

	// Turning off Dev Mode resets state
	app.SetDevMode(false)
	if app.IsDevMode() {
		t.Fatal("expected Dev Mode to be false after SetDevMode(false)")
	}
}

func TestQuickPresets(t *testing.T) {
	app := NewApp()
	app.SetDevMode(true)

	// 1. All Good
	st1 := app.ApplyDevPreset("all_good")
	if st1.Sensors.CPUPercent != 5.0 || st1.Sensors.CPUTempC != 40.0 {
		t.Fatalf("all_good preset failed, got cpu=%.1f, temp=%.1f", st1.Sensors.CPUPercent, st1.Sensors.CPUTempC)
	}

	// 2. Max Load
	st2 := app.ApplyDevPreset("max_load")
	if st2.Sensors.CPUPercent < 98.0 || st2.Sensors.GPUPercent < 98.0 {
		t.Fatalf("max_load preset failed, got cpu=%.1f, gpu=%.1f", st2.Sensors.CPUPercent, st2.Sensors.GPUPercent)
	}

	// 3. Disk Full
	st3 := app.ApplyDevPreset("disk_full")
	if len(st3.Disks) == 0 || st3.Disks[0].UsedPct < 95.0 {
		t.Fatalf("disk_full preset failed, got disks=%+v", st3.Disks)
	}

	// 4. Overheat
	st4 := app.ApplyDevPreset("overheat")
	if st4.Sensors.CPUTempC < 90.0 || st4.Sensors.GPUTempC < 90.0 {
		t.Fatalf("overheat preset failed, got cpuTemp=%.1f, gpuTemp=%.1f", st4.Sensors.CPUTempC, st4.Sensors.GPUTempC)
	}

	// 5. Dying SSD
	st5 := app.ApplyDevPreset("dying_ssd")
	if len(st5.Disks) == 0 || st5.Disks[0].HealthPct > 20.0 {
		t.Fatalf("dying_ssd preset failed, got health=%.1f", st5.Disks[0].HealthPct)
	}

	// 6. Vampire Hog
	st6 := app.ApplyDevPreset("vampire_hog")
	if len(st6.Vampires) == 0 {
		t.Fatal("vampire_hog preset failed, vampires list empty")
	}

	// 7. Long Uptime
	st7 := app.ApplyDevPreset("long_uptime")
	if st7.Sensors.UptimeHours < 300.0 {
		t.Fatalf("long_uptime preset failed, got uptime hours=%.1f", st7.Sensors.UptimeHours)
	}
}
