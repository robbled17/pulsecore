package main

import "testing"

func TestClassifyDiskMedia_NVMe(t *testing.T) {
	r := ClassifyDiskMedia(MediaClassifyInput{Bus: "NVMe"})
	if r.Kind != DiskKindNVMe || r.MediaLabel != "SSD" {
		t.Fatalf("%+v", r)
	}
}

func TestClassifyDiskMedia_IdentifySSD(t *testing.T) {
	r := ClassifyDiskMedia(MediaClassifyInput{IdentifyWord217OK: true, IdentifyWord217: 1, Bus: "SATA"})
	if r.Kind != DiskKindSATASSD || r.MediaLabel != "SSD" {
		t.Fatalf("%+v", r)
	}
}

func TestClassifyDiskMedia_IdentifyHDD7200(t *testing.T) {
	r := ClassifyDiskMedia(MediaClassifyInput{IdentifyWord217OK: true, IdentifyWord217: 7200, Bus: "SATA"})
	if r.Kind != DiskKindHDD || r.RotationRPM != 7200 || r.MediaLabel != "HDD" {
		t.Fatalf("%+v", r)
	}
}

func TestClassifyDiskMedia_SeekPenalty(t *testing.T) {
	tr, fa := true, false
	hdd := ClassifyDiskMedia(MediaClassifyInput{IncursSeekPenalty: &tr, Bus: "SATA"})
	if hdd.Kind != DiskKindHDD {
		t.Fatalf("seek true → HDD: %+v", hdd)
	}
	ssd := ClassifyDiskMedia(MediaClassifyInput{IncursSeekPenalty: &fa, Bus: "SATA"})
	if ssd.Kind != DiskKindSATASSD {
		t.Fatalf("seek false → SSD: %+v", ssd)
	}
}

func TestClassifyDiskMedia_UnknownNoSSDDefault(t *testing.T) {
	r := ClassifyDiskMedia(MediaClassifyInput{Bus: "SATA"})
	if r.Kind != DiskKindUnknown || r.MediaLabel != "тип не определён" {
		t.Fatalf("must not default SSD: %+v", r)
	}
	if FormatMediaBadge(r.MediaLabel, 0, true) == "SSD" {
		t.Fatal("badge must not be SSD")
	}
}

func TestClassifyDiskMedia_MSFT(t *testing.T) {
	r := ClassifyDiskMedia(MediaClassifyInput{MSFTMediaType: "HDD", RotationRPMHint: 7200, Bus: "SATA"})
	if r.Kind != DiskKindHDD || r.RotationRPM != 7200 {
		t.Fatalf("%+v", r)
	}
}
