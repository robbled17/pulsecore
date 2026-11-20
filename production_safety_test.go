package main

import (
	"testing"

	"github.com/shirou/gopsutil/v3/disk"
)

func TestEcoDenylistCriticalProcesses(t *testing.T) {
	protected := []string{
		"csrss.exe", "lsass.exe", "svchost.exe", "dwm.exe",
		"pulsecore.exe", "zeropulse.exe", "PulseCore.EXE",
	}
	for _, n := range protected {
		if !isEcoProtected(n) {
			t.Fatalf("expected protected: %s", n)
		}
	}
	if isEcoProtected("chrome.exe") {
		t.Fatal("chrome should not be denylisted")
	}
}

func TestEcoEmptyNameProtectedOnEnable(t *testing.T) {
	if !isEcoProtected("") {
		t.Fatal("empty name must be treated as protected for enable path")
	}
}

func TestDetectIntegratedUnknownWithVRAM(t *testing.T) {
	if detectIntegratedGPU("Unknown GPU", 8192) {
		t.Fatal("unknown + 8GB VRAM must not be classified as iGPU")
	}
	if !detectIntegratedGPU("Unknown GPU", 0) {
		t.Fatal("unknown + 0 VRAM should stay integrated/unknown")
	}
	if detectIntegratedGPU("NVIDIA GeForce RTX 3070", 100) {
		t.Fatal("RTX must be discrete even with tiny AdapterRAM")
	}
	if !detectIntegratedGPU("Intel UHD Graphics 630", 128) {
		t.Fatal("UHD must be integrated")
	}
}

func TestGPUDiscreteRankPrefersDiscrete(t *testing.T) {
	if gpuDiscreteRank("Microsoft Basic Display Adapter") >= 0 {
		t.Fatal("basic display must be rejected")
	}
	d := gpuDiscreteRank("NVIDIA GeForce RTX 3060")
	i := gpuDiscreteRank("Intel UHD Graphics 630")
	if d <= i {
		t.Fatalf("discrete rank %d should beat iGPU %d", d, i)
	}
}

func TestNonFixedPartitionFilter(t *testing.T) {
	cases := []struct {
		p    disk.PartitionStat
		skip bool
	}{
		{disk.PartitionStat{Mountpoint: `C:\`, Fstype: "NTFS", Opts: []string{"rw"}}, false},
		{disk.PartitionStat{Mountpoint: `E:\`, Fstype: "FAT32", Opts: []string{"rw", "removable"}}, true},
		{disk.PartitionStat{Mountpoint: `D:\`, Fstype: "cdfs", Opts: []string{"ro"}}, true},
		{disk.PartitionStat{Mountpoint: `\\server\share`, Fstype: "SMB", Opts: []string{"remote"}}, true},
	}
	for _, c := range cases {
		got := isNonFixedPartition(c.p)
		if got != c.skip {
			t.Fatalf("%+v: got skip=%v want %v", c.p, got, c.skip)
		}
	}
}

func TestUnknownGPUScoreUsesVRAM(t *testing.T) {
	// Mimic buildSystemPowerProfile path: unknown discrete with VRAM stays discrete.
	integrated := detectIntegratedGPU("Unknown GPU", 8192)
	if integrated {
		t.Fatal("should be discrete")
	}
	score := calcGPUPowerScore("Unknown GPU", false, 8)
	if score < 5 || score != score { // NaN check
		t.Fatalf("bad score %v", score)
	}
	cpu := calcCPUPowerScore("Mystery CPU", 8, 16, 3600)
	if cpu < 5 || cpu != cpu {
		t.Fatalf("bad cpu score %v", cpu)
	}
}
