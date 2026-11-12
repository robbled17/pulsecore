//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadHelperStateStripsBOM(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("PROGRAMDATA")
	t.Cleanup(func() { _ = os.Setenv("PROGRAMDATA", old) })
	_ = os.Setenv("PROGRAMDATA", dir)

	path := filepath.Join(dir, "PulseCore", "helper-state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	body := []byte("\ufeff{\n  \"enabled\": true,\n  \"installedByPulseCore\": true,\n  \"helperVersion\": \"1.0.0\",\n  \"protocolVersion\": 1\n}\n")
	if err := os.WriteFile(path, body, 0644); err != nil {
		t.Fatal(err)
	}
	st, err := loadHelperState()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !st.Enabled || !st.InstalledByPulseCore {
		t.Fatalf("parsed wrong: %+v", st)
	}
	if !advancedModeEnabled() {
		t.Fatal("advancedModeEnabled should be true after BOM strip")
	}
}
