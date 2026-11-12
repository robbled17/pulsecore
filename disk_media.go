package main

import (
	"fmt"
	"strings"
)

// MediaClassifyInput is everything needed to classify SSD vs HDD without guessing.
type MediaClassifyInput struct {
	Bus               string // NVMe|SATA|USB|…
	IdentifyWord217   int    // ATA IDENTIFY DEVICE word 217; 0 = not available
	IdentifyWord217OK bool
	IncursSeekPenalty *bool  // StorageDeviceSeekPenaltyProperty; nil = unknown
	MSFTMediaType     string // MSFT_PhysicalDisk.MediaType (HDD/SSD/Unspecified/…)
	RotationRPMHint   int    // SpindleSpeed from MSFT if known (>1)
}

// MediaClassifyResult is the resolved media kind and display strings.
type MediaClassifyResult struct {
	Kind         string // nvme|sata_ssd|hdd|unknown
	MediaLabel   string // SSD|HDD|тип не определён (NVMe uses SSD label + Bus NVMe)
	BusLabel     string // SATA|NVMe|USB|…
	RotationRPM  int    // >1 for HDD rpm; 0 if unknown/N/A
	Source       string // identify|seek_penalty|msft|nvme|unknown
	SourceRu     string
	SourceEn     string
}

// ClassifyDiskMedia follows the documented fallback chain - never defaults to SSD.
//
//  1. NVMe bus → always SSD (kind nvme)
//  2. ATA IDENTIFY word 217 (Nominal Media Rotation Rate):
//     1 = SSD; 0x0401-0xFFFE = HDD rpm; 0 or 0xFFFF = not reported
//  3. StorageDeviceSeekPenaltyProperty: IncursSeekPenalty true = HDD, false = SSD
//  4. MSFT_PhysicalDisk MediaType (+ SpindleSpeed hint)
//  5. Otherwise unknown - UI must show «тип не определён», not «SSD»
func ClassifyDiskMedia(in MediaClassifyInput) MediaClassifyResult {
	bus := normalizeBusLabel(in.Bus)
	out := MediaClassifyResult{Kind: DiskKindUnknown, BusLabel: bus, MediaLabel: "тип не определён"}

	if bus == "NVMe" {
		out.Kind = DiskKindNVMe
		out.MediaLabel = "SSD"
		out.BusLabel = "NVMe"
		out.Source = "nvme"
		out.SourceRu = "NVMe - твердотельный накопитель"
		out.SourceEn = "NVMe - solid-state drive"
		return out
	}

	if in.IdentifyWord217OK {
		w := in.IdentifyWord217 & 0xFFFF
		switch {
		case w == 1:
			out.Kind = DiskKindSATASSD
			out.MediaLabel = "SSD"
			out.Source = "identify"
			out.SourceRu = "IDENTIFY DEVICE слово 217 = 1 (SSD)"
			out.SourceEn = "IDENTIFY DEVICE word 217 = 1 (SSD)"
			return out
		case w >= 0x0401 && w <= 0xFFFE:
			out.Kind = DiskKindHDD
			out.MediaLabel = "HDD"
			out.RotationRPM = w
			out.Source = "identify"
			out.SourceRu = fmt.Sprintf("IDENTIFY DEVICE слово 217 = %d об/мин", w)
			out.SourceEn = fmt.Sprintf("IDENTIFY DEVICE word 217 = %d RPM", w)
			return out
		}
		// 0 or 0xFFFF → not reported; fall through
	}

	if in.IncursSeekPenalty != nil {
		if *in.IncursSeekPenalty {
			out.Kind = DiskKindHDD
			out.MediaLabel = "HDD"
			out.Source = "seek_penalty"
			out.SourceRu = "SeekPenalty: IncursSeekPenalty = true (HDD)"
			out.SourceEn = "SeekPenalty: IncursSeekPenalty = true (HDD)"
			if in.RotationRPMHint > 1 {
				out.RotationRPM = in.RotationRPMHint
			}
			return out
		}
		out.Kind = DiskKindSATASSD
		out.MediaLabel = "SSD"
		out.Source = "seek_penalty"
		out.SourceRu = "SeekPenalty: IncursSeekPenalty = false (SSD)"
		out.SourceEn = "SeekPenalty: IncursSeekPenalty = false (SSD)"
		return out
	}

	msft := strings.ToLower(strings.TrimSpace(in.MSFTMediaType))
	if strings.Contains(msft, "ssd") || msft == "3" || msft == "4" || strings.Contains(msft, "solid") {
		out.Kind = DiskKindSATASSD
		out.MediaLabel = "SSD"
		out.Source = "msft"
		out.SourceRu = "MSFT_PhysicalDisk.MediaType = SSD"
		out.SourceEn = "MSFT_PhysicalDisk.MediaType = SSD"
		return out
	}
	if strings.Contains(msft, "hdd") || msft == "2" || strings.Contains(msft, "hard disk") ||
		strings.Contains(msft, "rotational") {
		out.Kind = DiskKindHDD
		out.MediaLabel = "HDD"
		out.Source = "msft"
		out.SourceRu = "MSFT_PhysicalDisk.MediaType = HDD"
		out.SourceEn = "MSFT_PhysicalDisk.MediaType = HDD"
		if in.RotationRPMHint > 1 {
			out.RotationRPM = in.RotationRPMHint
		}
		return out
	}
	if in.RotationRPMHint > 1 {
		out.Kind = DiskKindHDD
		out.MediaLabel = "HDD"
		out.RotationRPM = in.RotationRPMHint
		out.Source = "msft"
		out.SourceRu = fmt.Sprintf("SpindleSpeed = %d об/мин", in.RotationRPMHint)
		out.SourceEn = fmt.Sprintf("SpindleSpeed = %d RPM", in.RotationRPMHint)
		return out
	}

	out.Source = "unknown"
	out.SourceRu = "тип не определён - IDENTIFY/SeekPenalty/MSFT недоступны"
	out.SourceEn = "type unknown - IDENTIFY/SeekPenalty/MSFT unavailable"
	out.MediaLabel = "тип не определён"
	return out
}

func normalizeBusLabel(bus string) string {
	u := strings.ToUpper(strings.TrimSpace(bus))
	switch {
	case strings.Contains(u, "NVME"):
		return "NVMe"
	case strings.Contains(u, "USB"):
		return "USB"
	case strings.Contains(u, "SATA") || strings.Contains(u, "ATA") || u == "IDE":
		return "SATA"
	case strings.Contains(u, "SAS"):
		return "SAS"
	case u == "":
		return ""
	default:
		return bus
	}
}

// FormatMediaBadge returns the compact card badge (media only, not "SATA · SSD").
func FormatMediaBadge(mediaLabel string, rpm int, langRu bool) string {
	switch mediaLabel {
	case "SSD", "HDD":
		if mediaLabel == "HDD" && rpm > 1 {
			if langRu {
				return fmt.Sprintf("HDD · %d об/мин", rpm)
			}
			return fmt.Sprintf("HDD · %d RPM", rpm)
		}
		return mediaLabel
	case "тип не определён":
		if langRu {
			return "тип не определён"
		}
		return "type unknown"
	default:
		if mediaLabel == "" {
			if langRu {
				return "тип не определён"
			}
			return "type unknown"
		}
		return mediaLabel
	}
}
