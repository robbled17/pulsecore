package main

import "testing"

func TestParseWindowsCommandLine(t *testing.T) {
	exe, args := parseWindowsCommandLine(`"C:\Program Files\App\uninstall.exe" /S`)
	if exe != `C:\Program Files\App\uninstall.exe` {
		t.Fatalf("exe=%q", exe)
	}
	if len(args) != 1 || args[0] != "/S" {
		t.Fatalf("args=%v", args)
	}
}

func TestFormatSizeLabel(t *testing.T) {
	if formatSizeLabel(0) != "—" {
		t.Fatal("zero")
	}
	if got := formatSizeLabel(1500 * 1024 * 1024); got == "—" {
		t.Fatalf("expected size label, got %q", got)
	}
}

func TestParseVSSUsedBytes(t *testing.T) {
	sample := "Used Shadow Copy Storage space: 1.23 GB (1,234,567,890 bytes)"
	n := parseVSSUsedBytes(sample)
	if n != 1234567890 {
		t.Fatalf("got %d", n)
	}
}
