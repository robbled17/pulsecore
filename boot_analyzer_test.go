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
		{"Windows Hello", "C:\\Windows\\System32\\bioiso.exe", true},
		{"NGC Credential", "C:\\Windows\\System32\\ngc.exe", true},
		{"Windows Defender", "MsMpEng.exe", true},
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

func TestSortStartupItems(t *testing.T) {
	items := []BootStartupItem{
		{Name: "Telegram", Status: "disabled", IsProtected: false},
		{Name: "Realtek Audio", Status: "enabled", IsProtected: true},
		{Name: "Discord", Status: "enabled", IsProtected: false},
		{Name: "Steam", Status: "delayed", IsProtected: false},
		{Name: "NVIDIA Container", Status: "enabled", IsProtected: true},
		{Name: "AnyDesk", Status: "disabled", IsProtected: false},
		{Name: "Browser", Status: "enabled", IsProtected: false},
	}

	sortStartupItems(items)

	// Group 1: Enabled/delayed non-protected: Browser, Discord, Steam (alphabetical)
	// Group 2: Protected: NVIDIA Container, Realtek Audio (alphabetical)
	// Group 3: Disabled non-protected: AnyDesk, Telegram (alphabetical)
	expected := []string{
		"Browser",
		"Discord",
		"Steam",
		"NVIDIA Container",
		"Realtek Audio",
		"AnyDesk",
		"Telegram",
	}

	if len(items) != len(expected) {
		t.Fatalf("expected %d items, got %d", len(expected), len(items))
	}

	for i, exp := range expected {
		if items[i].Name != exp {
			t.Errorf("item[%d] = %q, expected %q", i, items[i].Name, exp)
		}
	}
}

func TestStartupItemModeSwitchingAndEnabledCount(t *testing.T) {
	// Items including Run (HKCU, HKLM) and StartupFolder
	items := []BootStartupItem{
		{ID: "reg:hkcu:Discord", Name: "Discord", Status: "enabled", Location: "registry_user"},
		{ID: "reg:hklm:Riot Vanguard", Name: "Riot Vanguard", Status: "enabled", Location: "registry_machine"},
		{ID: "startup_user:ClickMonitorDDC.lnk", Name: "ClickMonitorDDC", Status: "enabled", Location: "startup_folder"},
		{ID: "reg:hkcu:Telegram", Name: "Telegram", Status: "disabled", Location: "registry_user"},
	}

	calcEnabled := func(list []BootStartupItem) int {
		cnt := 0
		for _, it := range list {
			if it.Status == "enabled" {
				cnt++
			}
		}
		return cnt
	}

	if count := calcEnabled(items); count != 3 {
		t.Fatalf("expected 3 enabled items initially, got %d", count)
	}

	// 1. Simulate disabling StartupFolder item (ClickMonitorDDC)
	for i := range items {
		if items[i].ID == "startup_user:ClickMonitorDDC.lnk" {
			items[i].Status = "disabled"
		}
	}
	if count := calcEnabled(items); count != 2 {
		t.Fatalf("expected 2 enabled items after disabling StartupFolder item, got %d", count)
	}

	// 2. Simulate disabling HKLM Run item (Riot Vanguard)
	for i := range items {
		if items[i].ID == "reg:hklm:Riot Vanguard" {
			items[i].Status = "disabled"
		}
	}
	if count := calcEnabled(items); count != 1 {
		t.Fatalf("expected 1 enabled item after disabling Riot Vanguard, got %d", count)
	}

	// 3. Verify sorting brings disabled items to the bottom
	sortStartupItems(items)
	if items[0].Name != "Discord" || items[0].Status != "enabled" {
		t.Errorf("expected Discord to remain at top as enabled, got %+v", items[0])
	}
	for i := 1; i < len(items); i++ {
		if items[i].Status != "disabled" {
			t.Errorf("expected item[%d] to be disabled at bottom, got %+v", i, items[i])
		}
	}

	// 4. Test StartupApproved byte checks (0x02 = enabled, 0x03 = disabled)
	bytesDisabled := []byte{0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	bytesEnabled := []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if isStartupApprovedEnabled(bytesDisabled) {
		t.Errorf("expected bytesDisabled to be reported as disabled")
	}
	if !isStartupApprovedEnabled(bytesEnabled) {
		t.Errorf("expected bytesEnabled to be reported as enabled")
	}
}
