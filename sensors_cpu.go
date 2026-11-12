//go:build windows

package main

import "time"

// readCPUTemperatureSmart priority:
// 1) PulseCore helper (advanced mode / PawnIO+LHM)
// 2) HWiNFO SHM / LibreHardwareMonitor WMI - reserve only if already running
// 3) ACPI thermal zone (often missing on Ryzen)
// Never invents a value.
func readCPUTemperatureSmart() externalTempReading {
	if r := readTempFromHelper(); r.OK {
		return r
	}
	if r := readTempFromHWiNFO(); r.OK {
		return r
	}
	if r := readTempFromLibreHardwareMonitor(); r.OK {
		return r
	}
	if c := readCPUTemperature(); c > 0 {
		return externalTempReading{
			TempC: c, OK: true, Source: "acpi",
			LabelRu: "", LabelEn: "", // native - no external caption
			SensorName: "ACPI",
		}
	}
	return externalTempReading{}
}

var lastProbeReset time.Time

func maybeResetExternalTempProbes() {
	if lastProbeReset.IsZero() || time.Since(lastProbeReset) > 60*time.Second {
		lastProbeReset = time.Now()
		resetExternalTempProbes()
	}
}
