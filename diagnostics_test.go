package main

import (
	"strings"
	"testing"
)

func TestBuildSelfDiagnosticsReport_LinuxUnavailable(t *testing.T) {
	rep := buildSelfDiagnosticsReport(diagSnapshot{
		Platform: "linux",
		IsAdmin:  false,
	})
	if !rep.OK {
		t.Fatal("report should be ok even when unavailable")
	}
	if rep.Platform != "linux" {
		t.Fatalf("platform=%s", rep.Platform)
	}
	if rep.RunMode != "user" {
		t.Fatalf("runMode=%s", rep.RunMode)
	}
	wantIDs := []string{"cpu", "gpu", "ram", "drives", "registry", "runmode"}
	if len(rep.Items) != len(wantIDs) {
		t.Fatalf("items=%d want %d", len(rep.Items), len(wantIDs))
	}
	for i, id := range wantIDs {
		if rep.Items[i].ID != id {
			t.Fatalf("item[%d]=%s want %s", i, rep.Items[i].ID, id)
		}
		if id != "runmode" && rep.Items[i].Status != DiagStatusUnavailable {
			t.Fatalf("%s status=%s want unavailable", id, rep.Items[i].Status)
		}
	}
	if !strings.Contains(rep.TextReport, "platform: linux") {
		t.Fatal("text report missing platform")
	}
	if strings.Contains(rep.TextReport, "\u2014") || strings.Contains(rep.TextReport, "\u2013") {
		t.Fatal("text report must not use em/en dashes")
	}
}

func TestBuildSelfDiagnosticsReport_WindowsFixtures(t *testing.T) {
	rep := buildSelfDiagnosticsReport(diagSnapshot{
		Platform:       "windows",
		IsAdmin:        false,
		CPUName:        "AMD Ryzen 7 5800H",
		CPUPhys:        8,
		CPULogical:     16,
		CPULoad:        22.5,
		CPULoadOK:      true,
		CPUNameOK:      true,
		CPUTempOK:      false, // ACPI needs admin in this fixture
		GPUPrimary:     "NVIDIA GeForce RTX 3060 Laptop GPU",
		GPUIntegrated:  "AMD Radeon Graphics",
		GPUVRAMMB:      6144,
		GPULoad:        0,
		GPULoadOK:      false,
		GPUPrimaryOK:   true,
		GPUUsesFallback: true,
		RAMTotalMB:     16384,
		RAMSlotsUsed:   2,
		RAMSlotsTotal:  2,
		RAMSpeedMHz:    3200,
		RAMOK:          true,
		RAMSlotsOK:     true,
		Drives: []diagDrive{
			{Mount: `C:\`, TotalGB: 512, UsedGB: 200, SMARTOK: false, SMARTNote: "Access denied"},
		},
		SMARTAvailable:  false,
		SMARTNeedsAdmin: true,
		CleanRootsReachable: 3,
		CleanRootsTotal:     4,
		BackupDir:           `C:\Users\mock\AppData\Local\PulseCore\backups`,
		BackupDirOK:         true,
		RegistryEnumOK:      true,
	})

	byID := map[string]SelfDiagItem{}
	for _, it := range rep.Items {
		byID[it.ID] = it
	}

	if byID["cpu"].Status != DiagStatusAdmin {
		t.Fatalf("cpu status=%s want admin", byID["cpu"].Status)
	}
	if byID["gpu"].Status != DiagStatusFallback {
		t.Fatalf("gpu status=%s want fallback", byID["gpu"].Status)
	}
	if byID["ram"].Status != DiagStatusOK {
		t.Fatalf("ram status=%s want ok", byID["ram"].Status)
	}
	if byID["drives"].Status != DiagStatusAdmin {
		t.Fatalf("drives status=%s want admin", byID["drives"].Status)
	}
	if byID["registry"].Status != DiagStatusFallback {
		t.Fatalf("registry status=%s want fallback (partial roots)", byID["registry"].Status)
	}
	if byID["runmode"].Status != DiagStatusOK || rep.RunMode != "user" {
		t.Fatalf("runmode=%s status=%s", rep.RunMode, byID["runmode"].Status)
	}
	if !strings.Contains(byID["cpu"].Facts[0], "AMD Ryzen") {
		t.Fatalf("cpu facts missing model: %v", byID["cpu"].Facts)
	}
	// Never invent fake sensor numbers when unavailable.
	for _, f := range byID["cpu"].Facts {
		if strings.HasPrefix(f, "temp=") && f != "temp=unavailable" {
			t.Fatalf("invented temp fact: %s", f)
		}
	}
}

func TestBuildSelfDiagnosticsReport_AdminWorking(t *testing.T) {
	rep := buildSelfDiagnosticsReport(diagSnapshot{
		Platform:      "windows",
		IsAdmin:       true,
		CPUName:       "Intel Core i5-12400",
		CPUPhys:       6,
		CPULogical:    12,
		CPULoad:       10,
		CPULoadOK:     true,
		CPUNameOK:     true,
		CPUTempC:      48,
		CPUTempOK:     true,
		GPUPrimary:    "NVIDIA GeForce GTX 1660",
		GPUVRAMMB:     6144,
		GPULoad:       12,
		GPULoadOK:     true,
		GPUPrimaryOK:  true,
		RAMTotalMB:    16384,
		RAMSlotsUsed:  2,
		RAMSlotsTotal: 4,
		RAMSpeedMHz:   3200,
		RAMOK:         true,
		RAMSlotsOK:    true,
		Drives: []diagDrive{
			{Mount: `C:\`, TotalGB: 1000, UsedGB: 400, SMARTOK: true, TempC: 35, TempOK: true, SMARTNote: "OK"},
		},
		SMARTAvailable:      true,
		CleanRootsReachable: 5,
		CleanRootsTotal:     5,
		BackupDir:           `D:\PulseCore\backups`,
		BackupDirOK:         true,
		RegistryEnumOK:      true,
	})
	for _, it := range rep.Items {
		if it.Status != DiagStatusOK {
			t.Fatalf("%s status=%s want ok", it.ID, it.Status)
		}
	}
	if rep.RunMode != "admin" {
		t.Fatalf("runMode=%s", rep.RunMode)
	}
}

func TestFormatDiagTextReport_NoLongDashes(t *testing.T) {
	rep := SelfDiagnosticsReport{
		GeneratedAt: "2026-01-01T00:00:00Z",
		Platform:    "windows",
		RunMode:     "user",
		Items: []SelfDiagItem{
			{ID: "cpu", Status: DiagStatusFallback, Detail: "Fallback calculation", Facts: []string{"temp=unavailable"}},
		},
	}
	text := formatDiagTextReport(rep)
	if strings.ContainsAny(text, "\u2013\u2014") {
		t.Fatal("em/en dash in text report")
	}
	if !strings.Contains(text, "[fallback]") {
		t.Fatal("missing status marker")
	}
}
