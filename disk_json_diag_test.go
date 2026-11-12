package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/robbled17/zeropulse/loc"
)

// Prints real DiskHealthReport / DiskMetrics JSON shapes for diagnosis.
func TestDiskJSONFixturesDump(t *testing.T) {
	metrics := []DiskMetrics{
		{Mount: `C:\`, UsedGB: 200, TotalGB: 500, Percent: 40, FreeGB: 300},
		{Mount: `D:\`, UsedGB: 100, TotalGB: 1000, Percent: 10, FreeGB: 900},
	}
	b, _ := json.MarshalIndent(map[string]any{"disks": metrics}, "", "  ")
	fmt.Println("=== metrics.disks ===")
	fmt.Println(string(b))

	helperOn := DiskHealthReport{
		DriveLetter: "C:", Model: "Samsung SSD 980", MediaKind: DiskKindNVMe, MediaType: "NVMe SSD",
		StatusLevel: DiskStatusGood, StatusCode: "diskStateGood",
		StatusWordRu: "Хорошо", StatusWordEn: "Good",
		StatusReason: loc.Msg{}, HealthPercent: 98, HealthPercentOK: true,
		HealthSourceMsg: loc.M("diskHealthSrcNVMe", nil),
		TempAvailable: true, TemperatureC: 42, CapacityGB: 500, OK: true,
	}
	helperOff := DiskHealthReport{
		DriveLetter: "C:", NeedsAdmin: true,
		StatusLevel: DiskStatusUnknown, StatusCode: "diskStateUnknown",
		StatusWordRu: "Включите расширенный режим", StatusWordEn: "Enable advanced mode",
		AdminBannerRu: "Включите расширенный режим", AdminBannerEn: "Enable advanced mode",
		OK: false,
	}
	caution := DiskHealthReport{
		DriveLetter: "D:", Model: "WD HDD", MediaKind: DiskKindHDD, MediaType: "HDD",
		StatusLevel: DiskStatusCaution, StatusCode: "diskStateWarn",
		StatusWordRu: "Внимание", StatusWordEn: "Caution",
		StatusReason: loc.M("diskReasonPending", loc.P("n", 3)),
		StatusReasonRu: "Атрибут C5 (ожидающие секторы) = 3",
		StatusReasonEn: "Attribute C5 (pending sectors) = 3",
		HealthPercent: 72, HealthPercentOK: true,
		RotationRPM: 7200, TempAvailable: true, TemperatureC: 38, CapacityGB: 1000, OK: true,
	}
	noSmart := DiskHealthReport{
		DriveLetter: "E:", Model: "USB Disk", MediaKind: DiskKindUnknown,
		StatusLevel: DiskStatusUnknown, StatusCode: "diskStateUnknown",
		StatusReason: loc.M("diskReasonSmartUnread", nil),
		StatusReasonRu: "SMART не прочитан", StatusReasonEn: "SMART was not read",
		OK: true,
	}

	for _, name := range []string{"helper_on", "helper_off", "caution", "no_smart"} {
		var r DiskHealthReport
		switch name {
		case "helper_on":
			r = helperOn
		case "helper_off":
			r = helperOff
		case "caution":
			r = caution
		case "no_smart":
			r = noSmart
		}
		jb, _ := json.MarshalIndent(r, "", "  ")
		fmt.Printf("=== DiskHealthReport %s ===\n%s\n", name, string(jb))
	}
}
