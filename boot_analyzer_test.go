package main

import (
	"testing"
)

func TestParseEvent100XML(t *testing.T) {
	xmlSample := []byte(`
	<Event xmlns="http://schemas.microsoft.com/win/2004/08/events/event">
		<EventData>
			<Data Name="BootTime">32500</Data>
			<Data Name="MainPathBootTime">14200</Data>
		</EventData>
	</Event>`)

	bootSec, kernelSec, desktopSec, ok := parseEvent100XML(xmlSample)
	if !ok {
		t.Fatalf("expected ok parsing Event 100 XML")
	}

	if bootSec != 32.5 {
		t.Errorf("expected bootSec 32.5, got %v", bootSec)
	}
	if kernelSec != 14.2 {
		t.Errorf("expected kernelSec 14.2, got %v", kernelSec)
	}
	if desktopSec != 18.3 {
		t.Errorf("expected desktopSec 18.3, got %v", desktopSec)
	}
}

func TestParseEvent101XML(t *testing.T) {
	xmlSample := []byte(`
	<Event xmlns="http://schemas.microsoft.com/win/2004/08/events/event">
		<EventData>
			<Data Name="Name">C:\Program Files\Discord\Discord.exe</Data>
			<Data Name="TotalTime">4250</Data>
		</EventData>
	</Event>`)

	items := parseEvent101XML(xmlSample)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Name != "Discord.exe" {
		t.Errorf("expected Name Discord.exe, got %s", items[0].Name)
	}
	if items[0].TotalSec != 4.2 {
		t.Errorf("expected TotalSec 4.2, got %v", items[0].TotalSec)
	}
}

func TestIsProtectedStartupItem(t *testing.T) {
	cases := []struct {
		name      string
		cmd       string
		protected bool
	}{
		{"Realtek Audio Service", "C:\\Windows\\System32\\RtkAudUService64.exe", true},
		{"NVIDIA Container", "\"C:\\Program Files\\NVIDIA Corporation\\NvContainer\\nvcontainer.exe\"", true},
		{"AMD Noise Suppression", "C:\\Program Files\\AMD\\CNext\\CNext\\RadeonSoftware.exe", true},
		{"SecurityHealthSystray", "SecurityHealthSystray.exe", true},
		{"PulseCore", "C:\\PulseCore\\pulsecore.exe", true},
		{"ctfmon", "ctfmon.exe", true},
		{"Discord", "C:\\Users\\User\\AppData\\Local\\Discord\\Update.exe --processStart Discord.exe", false},
		{"Telegram", "C:\\Users\\User\\AppData\\Roaming\\Telegram Desktop\\Telegram.exe", false},
		{"Steam", "C:\\Program Files (x86)\\Steam\\steam.exe -silent", false},
	}

	for _, c := range cases {
		got := isProtectedStartupItem(c.name, c.cmd)
		if got != c.protected {
			t.Errorf("isProtectedStartupItem(%q, %q) = %v, expected %v", c.name, c.cmd, got, c.protected)
		}
	}
}

func TestIsStartupApprovedEnabled(t *testing.T) {
	enabledBytes := []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	disabledBytes := []byte{0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	if !isStartupApprovedEnabled(enabledBytes) {
		t.Errorf("expected 0x02 to be enabled")
	}
	if isStartupApprovedEnabled(disabledBytes) {
		t.Errorf("expected 0x03 to be disabled")
	}
	if !isStartupApprovedEnabled(nil) {
		t.Errorf("expected nil to be enabled by default")
	}
}
