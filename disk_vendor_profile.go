package main

import "strings"

// DiskControllerProfile describes vendor-specific SMART interpretation.
type DiskControllerProfile struct {
	ID string // smi|samsung|intel|generic_ssd|hdd|nvme|unknown_ssd

	// WearRemainingID: prefer raw (0–100) as remaining life % when WearFromRaw.
	WearRemainingID int
	WearFromRaw     bool

	// HostWriteID / HostReadID with HostUnitMiB (e.g. 32 for SMI F1/F2).
	HostWriteID int
	HostReadID  int
	HostUnitMiB float64 // bytes per raw unit in MiB; 0 → classic LBA×512

	// SSD: do not escalate status solely for non-zero remapped/pending raw.
	IgnoreSectorRawForStatus bool
	// Escalate when counter grew by at least this since prior sample.
	SectorGrowthThreshold int64
	// Escalate when absolute pending/realloc exceeds this (SSD).
	SectorLargeThreshold int64

	// AttrMeaning: known IDs → semantic key for i18n / notes.
	// Unknown IDs must not drive status.
	AttrMeaning map[int]string // pending|reallocated|uncorrectable|wear|spare|host_write|host_read|temp|power_on|power_cycle|ignore
}

func smiSSDProfile() DiskControllerProfile {
	return DiskControllerProfile{
		ID:                       "smi",
		WearRemainingID:          169, // A9h Remaining Life — raw %
		WearFromRaw:              true,
		HostWriteID:              241, // F1h
		HostReadID:               242, // F2h
		HostUnitMiB:              32,
		IgnoreSectorRawForStatus: true,
		SectorGrowthThreshold:    5,   // 2→12 triggers caution
		SectorLargeThreshold:     100, // absolute large value
		AttrMeaning: map[int]string{
			5: "reallocated", 197: "pending", 198: "uncorrectable",
			169: "wear", 231: "wear", 233: "wear",
			9: "power_on", 12: "power_cycle", 194: "temp", 190: "temp",
			241: "host_write", 242: "host_read",
		},
	}
}

func detectDiskProfile(kind, model string, attrs []SmartAttribute) DiskControllerProfile {
	m := strings.ToLower(strings.TrimSpace(model))
	switch kind {
	case DiskKindHDD:
		return DiskControllerProfile{
			ID: "hdd",
			AttrMeaning: map[int]string{
				5: "reallocated", 196: "reallocated", 197: "pending", 198: "uncorrectable",
				9: "power_on", 12: "power_cycle", 194: "temp", 190: "temp",
				241: "host_write", 242: "host_read",
			},
			SectorGrowthThreshold: 1,
		}
	case DiskKindNVMe:
		return DiskControllerProfile{ID: "nvme"}
	case DiskKindSATASSD:
		if isSiliconMotionSSD(m) || looksLikeSMIAttrs(attrs) {
			return smiSSDProfile()
		}
		if strings.Contains(m, "samsung") {
			return DiskControllerProfile{
				ID: "samsung", WearRemainingID: 177, IgnoreSectorRawForStatus: true,
				SectorGrowthThreshold: 5, SectorLargeThreshold: 100,
				HostWriteID: 241, HostReadID: 242, // LBA×512 default
				AttrMeaning: map[int]string{5: "reallocated", 197: "pending", 177: "wear", 241: "host_write", 242: "host_read"},
			}
		}
		if strings.Contains(m, "intel") {
			return DiskControllerProfile{
				ID: "intel", WearRemainingID: 233, IgnoreSectorRawForStatus: true,
				SectorGrowthThreshold: 5, SectorLargeThreshold: 100,
				AttrMeaning: map[int]string{5: "reallocated", 197: "pending", 233: "wear"},
			}
		}
		// Known SSD brand but no detailed map — still ignore sector-raw panic.
		if looksLikeSSDBrand(m) {
			return DiskControllerProfile{
				ID: "generic_ssd", IgnoreSectorRawForStatus: true,
				SectorGrowthThreshold: 5, SectorLargeThreshold: 100,
				AttrMeaning: map[int]string{5: "reallocated", 197: "pending", 198: "uncorrectable"},
			}
		}
		return DiskControllerProfile{ID: "unknown_ssd", IgnoreSectorRawForStatus: true}
	default:
		return DiskControllerProfile{ID: "unknown"}
	}
}

// looksLikeSMIAttrs detects Silicon Motion ATA SMART layout (Remain Life A9 + SMI vendor block).
func looksLikeSMIAttrs(attrs []SmartAttribute) bool {
	if len(attrs) == 0 {
		return false
	}
	has169, has160, has161 := false, false, false
	for _, a := range attrs {
		switch a.ID {
		case 169:
			has169 = true
		case 160:
			has160 = true
		case 161:
			has161 = true
		}
	}
	return has169 && has160 && has161
}

func isSiliconMotionSSD(modelLower string) bool {
	if strings.Contains(modelLower, "compit") {
		return true
	}
	if strings.Contains(modelLower, "silicon motion") || strings.Contains(modelLower, "smi ") {
		return true
	}
	// Common SMI OEM sticks / Dramless SSDs sold under COMPIT and similar.
	if strings.Contains(modelLower, "sm2263") || strings.Contains(modelLower, "sm2259") || strings.Contains(modelLower, "sm2262") {
		return true
	}
	return false
}

func looksLikeSSDBrand(m string) bool {
	for _, p := range []string{"kingston", "crucial", "wd ", "western", "sandisk", "adata", "corsair", "team", "patriot", "transcend"} {
		if strings.Contains(m, p) {
			return true
		}
	}
	return false
}

func attrRawByID(attrs []SmartAttribute, id int) (uint64, bool) {
	for _, a := range attrs {
		if a.ID == id {
			return a.RawValue, true
		}
	}
	return 0, false
}

func hostIOFromProfile(p DiskControllerProfile, attrs []SmartAttribute) (writeGB, readGB float64, okW, okR bool) {
	if p.HostWriteID > 0 {
		if raw, ok := attrRawByID(attrs, p.HostWriteID); ok {
			if p.HostUnitMiB > 0 {
				writeGB = float64(raw) * p.HostUnitMiB / 1024.0
			} else {
				writeGB = float64(raw) * 512.0 / (1024 * 1024 * 1024)
			}
			okW = writeGB > 0
		}
	}
	if p.HostReadID > 0 {
		if raw, ok := attrRawByID(attrs, p.HostReadID); ok {
			if p.HostUnitMiB > 0 {
				readGB = float64(raw) * p.HostUnitMiB / 1024.0
			} else {
				readGB = float64(raw) * 512.0 / (1024 * 1024 * 1024)
			}
			okR = readGB > 0
		}
	}
	return
}

func wearLifeFromProfile(p DiskControllerProfile, attrs []SmartAttribute) (life int, id int, ok bool) {
	if p.WearRemainingID <= 0 {
		return 0, 0, false
	}
	for _, a := range attrs {
		if a.ID != p.WearRemainingID {
			continue
		}
		if p.WearFromRaw {
			v := int(a.RawValue & 0xFF)
			if v <= 0 || v > 100 {
				// Some firmwares pack % in low 16 bits.
				v = int(a.RawValue & 0xFFFF)
			}
			if v > 0 && v <= 100 {
				return v, a.ID, true
			}
		}
		if a.Current > 0 && a.Current <= 100 {
			return a.Current, a.ID, true
		}
	}
	return 0, 0, false
}
