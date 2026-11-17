//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractVersionInfoWin32(t *testing.T) {
	windir := os.Getenv("WINDIR")
	if windir == "" {
		windir = `C:\Windows`
	}
	target := filepath.Join(windir, "System32", "cmd.exe")
	if _, err := os.Stat(target); err != nil {
		t.Skip("cmd.exe not found")
	}

	info := getProcessVersionInfo(target)
	t.Logf("cmd.exe version info: %q", info)
	if info == "" {
		t.Errorf("expected non-empty version info for cmd.exe")
	}
}
