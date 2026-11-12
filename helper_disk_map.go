//go:build windows

package main

import (
	"strconv"
	"strings"
	"time"
)

// diskReportFromHelper maps helper SMART payload into DiskHealthReport.
func diskReportFromHelper(letter string) (DiskHealthReport, bool) {
	resp, err := helperGet()
	if err != nil || !resp.OK {
		return DiskHealthReport{}, false
	}
	want := strings.ToUpper(strings.TrimSpace(letter))
	want = strings.TrimSuffix(want, ":")
	var hd *HelperDisk
	for i := range resp.Disks {
		L := strings.ToUpper(strings.TrimSuffix(strings.TrimSpace(resp.Disks[i].Letter), ":"))
		if L == want {
			hd = &resp.Disks[i]
			break
		}
	}
	if hd == nil {
		return DiskHealthReport{}, false
	}

	report := DiskHealthReport{
		DriveLetter:   want + ":",
		OK:            true,
		Model:         hd.Model,
		SerialNumber:  hd.Serial,
		InterfaceType: hd.Interface,
		BusInterface:  normalizeBusLabel(hd.Interface),
		CheckedAt:     time.Now().Format(time.RFC3339),
		NeedsAdmin:    false,
	}
	if hd.TempOK && hd.TempC != nil && *hd.TempC > 0 {
		report.TemperatureC = *hd.TempC
		report.TempAvailable = true
	}
	if hd.PowerOnHours != nil && *hd.PowerOnHours > 0 {
		report.PowerOnHours = *hd.PowerOnHours
	}
	if hd.TotalWritesBytes != nil && *hd.TotalWritesBytes > 0 {
		gb := *hd.TotalWritesBytes / (1024 * 1024 * 1024)
		report.HostWriteGB = gb
		report.HostWriteTB = gb / 1024
		report.TBWGB = gb
	}

	attrs := make([]SmartAttribute, 0, len(hd.Attrs))
	for _, a := range hd.Attrs {
		rawVal := parseSMARTRawDecimal(a.Raw)
		attrs = append(attrs, SmartAttribute{
			ID: a.ID, Name: a.Name,
			Current: a.Current, Worst: a.Worst, Threshold: a.Threshold, Raw: a.Raw,
			RawValue: rawVal,
		})
	}
	report.Attributes = attrs
	report.ReallocatedSectors = attrRawOr(attrs, 5, 0)
	report.PendingSectors = attrRawOr(attrs, 197, 0)
	report.UncorrectableSectors = attrRawOr(attrs, 198, 0)

	mediaIn := MediaClassifyInput{Bus: hd.Interface}
	if hd.IdentifyWord217 != nil {
		mediaIn.IdentifyWord217 = *hd.IdentifyWord217
		mediaIn.IdentifyWord217OK = true
	}
	if hd.RotationRPM != nil && *hd.RotationRPM > 1 {
		mediaIn.RotationRPMHint = *hd.RotationRPM
	}
	// Prefer explicit helper mediaKind / IsSSD when IDENTIFY word missing.
	media := ClassifyDiskMedia(mediaIn)
	if media.Kind == DiskKindUnknown {
		switch strings.ToLower(strings.TrimSpace(hd.MediaKind)) {
		case "nvme":
			media.Kind = DiskKindNVMe
			media.MediaLabel = "SSD"
			media.BusLabel = "NVMe"
			media.Source = "helper"
		case "ssd":
			media.Kind = DiskKindSATASSD
			media.MediaLabel = "SSD"
			media.Source = "helper"
		case "hdd":
			media.Kind = DiskKindHDD
			media.MediaLabel = "HDD"
			media.Source = "helper"
			if hd.RotationRPM != nil && *hd.RotationRPM > 1 {
				media.RotationRPM = *hd.RotationRPM
			}
		default:
			if hd.IsNVMe {
				media.Kind = DiskKindNVMe
				media.MediaLabel = "SSD"
				media.BusLabel = "NVMe"
			} else if hd.IsSSD != nil {
				if *hd.IsSSD {
					media.Kind = DiskKindSATASSD
					media.MediaLabel = "SSD"
				} else {
					media.Kind = DiskKindHDD
					media.MediaLabel = "HDD"
					if hd.RotationRPM != nil && *hd.RotationRPM > 1 {
						media.RotationRPM = *hd.RotationRPM
					}
				}
				media.Source = "helper_isssd"
			}
		}
	}
	if media.BusLabel == "" {
		media.BusLabel = report.BusInterface
	}
	report.MediaKind = media.Kind
	report.MediaType = media.MediaLabel
	report.BusInterface = media.BusLabel
	report.InterfaceType = media.BusLabel
	if media.RotationRPM > 1 {
		report.RotationRPM = float64(media.RotationRPM)
	} else if hd.RotationRPM != nil && *hd.RotationRPM > 1 {
		report.RotationRPM = float64(*hd.RotationRPM)
	}

	evalIn := DiskHealthEvalInput{
		Kind: media.Kind, Model: report.Model, Attributes: attrs,
		SMARTReadOK: len(attrs) > 0 || (hd.HealthOK && hd.HealthPercent != nil),
		Reallocated: report.ReallocatedSectors,
		Pending:     report.PendingSectors,
		Uncorrectable: report.UncorrectableSectors,
	}
	// Helper Life % only for SSD/NVMe.
	if media.Kind != DiskKindHDD && hd.HealthOK && hd.HealthPercent != nil {
		// Feed as wear via synthetic attr if no wear attrs present.
		if !findWearLifeID(attrs) && media.Kind == DiskKindSATASSD {
			attrs = append(attrs, SmartAttribute{ID: 231, Name: "Life", Current: int(*hd.HealthPercent), RawValue: uint64(*hd.HealthPercent)})
			evalIn.Attributes = attrs
			report.Attributes = attrs
		}
		if media.Kind == DiskKindNVMe {
			used := 100 - int(*hd.HealthPercent)
			if used < 0 {
				used = 0
			}
			evalIn.NVMePercentUsed = used
			evalIn.NVMePercentUsedOK = true
		}
	}

	prof := detectDiskProfile(media.Kind, report.Model, attrs)
	if w, r, okW, okR := hostIOFromProfile(prof, attrs); okW || okR {
		if okW {
			report.HostWriteGB = w
			report.HostWriteTB = w / 1024.0
			report.TBWGB = w
		}
		if okR {
			report.HostReadGB = r
			report.HostReadTB = r / 1024.0
		}
	}
	ev := EvaluateDiskHealth(evalIn)
	applyDiskHealthEval(&report, ev)
	if report.StatusWordRu == "" {
		switch report.StatusLevel {
		case DiskStatusGood:
			report.StatusCode = "diskStateGood"
			report.StatusWordRu, report.StatusWordEn = "Хорошо", "Good"
		case DiskStatusCaution:
			report.StatusCode = "diskStateWarn"
			report.StatusWordRu, report.StatusWordEn = "Внимание", "Caution"
		case DiskStatusBad:
			report.StatusCode = "diskStateBad"
			report.StatusWordRu, report.StatusWordEn = "Плохо", "Bad"
		default:
			report.StatusCode = "diskStateUnknown"
			report.StatusWordRu, report.StatusWordEn = "Неизвестно", "Unknown"
		}
	}
	report.PowerOnHuman = humanPowerOnMsg(report.PowerOnHours)
	report.PowerOnHumanRu = humanPowerOn(report.PowerOnHours, true)
	report.PowerOnHumanEn = humanPowerOn(report.PowerOnHours, false)
	return report, true
}

func parseSMARTRawDecimal(raw string) uint64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	// Prefer leading integer (CrystalDiskInfo-style raw).
	n, err := strconv.ParseUint(strings.Fields(raw)[0], 10, 64)
	if err == nil {
		return n
	}
	return 0
}
