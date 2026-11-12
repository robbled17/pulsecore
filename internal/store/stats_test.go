package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.json")
	m, err := OpenStats(path)
	if err != nil {
		t.Fatal(err)
	}
	m.RecordSample(40, 50, 10, "chrome.exe", 22, true)
	m.IncEcoToggle()
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	m2, err := OpenStats(path)
	if err != nil {
		t.Fatal(err)
	}
	days := m2.LastNDays(7)
	if len(days) != 1 || days[0].LagCount != 1 || days[0].EcoToggles != 1 {
		t.Fatalf("%+v", days)
	}
	sum := BuildWeeklySummary(days)
	if !sum.OK || sum.Samples < 1 || sum.TopProcess != "chrome.exe" {
		t.Fatalf("%+v", sum)
	}
	_ = os.Remove(path)
}
